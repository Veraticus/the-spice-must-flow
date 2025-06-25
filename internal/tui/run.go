package tui

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/engine"
	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/service"
)

// ClassificationConfig holds the configuration for running classification with TUI.
type ClassificationConfig struct {
	Storage        service.Storage
	Classifier     engine.Classifier
	MinAmount      *float64
	MaxAmount      *float64
	StartDate      string
	EndDate        string
	Categories     []model.Category
	Patterns       []model.CheckPattern
	AccountIDs     []string
	OnlyUnreviewed bool
}

// RunClassification runs the classification engine with TUI interface.
func RunClassification(ctx context.Context, cfg ClassificationConfig) error {
	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Create a context that cancels on signal
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Set up terminal cleanup on any exit
	cleanupTerminal := func() {
		// Restore terminal to normal state
		// We use os.Stdout.Write to avoid linter complaints about fmt.Print
		// Ignore errors as this is best-effort cleanup
		_, _ = os.Stdout.Write([]byte("\033[?1049l")) // Exit alternate screen
		_, _ = os.Stdout.Write([]byte("\033[?25h"))   // Show cursor
		_, _ = os.Stdout.Write([]byte("\033[m"))      // Reset colors
		_, _ = os.Stdout.Write([]byte("\033[?1000l")) // Disable mouse
	}
	defer cleanupTerminal()

	// Handle signals
	go func() {
		<-sigChan
		cleanupTerminal()
		cancel()
	}()

	// Validate configuration
	if cfg.Storage == nil {
		return fmt.Errorf("storage is required")
	}
	if cfg.Classifier == nil {
		return fmt.Errorf("classifier is required")
	}

	// Load categories if not provided
	if len(cfg.Categories) == 0 {
		categories, err := cfg.Storage.GetCategories(ctx)
		if err != nil {
			return fmt.Errorf("failed to load categories: %w", err)
		}
		cfg.Categories = categories
	}

	// Load patterns if not provided
	if len(cfg.Patterns) == 0 {
		patterns, err := cfg.Storage.GetActiveCheckPatterns(ctx)
		if err != nil {
			return fmt.Errorf("failed to load patterns: %w", err)
		}
		cfg.Patterns = patterns
	}

	// Validate we have categories
	if len(cfg.Categories) == 0 {
		return fmt.Errorf("no categories found - please run 'spice categories' first")
	}

	// Create TUI with all dependencies
	tuiPrompter, err := New(ctx,
		WithStorage(cfg.Storage),
		WithClassifier(cfg.Classifier),
		WithCategories(cfg.Categories),
		WithPatterns(cfg.Patterns),
	)
	if err != nil {
		return fmt.Errorf("failed to create TUI: %w", err)
	}

	// Channel to signal when TUI is ready
	ready := make(chan struct{})
	errChan := make(chan error, 1)

	// Ensure we only close ready channel once
	var readyOnce sync.Once

	// Start TUI in background
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Set ready callback (type assert to access concrete type)
		if p, ok := tuiPrompter.(*Prompter); ok {
			p.SetReadyCallback(func() {
				readyOnce.Do(func() {
					close(ready)
				})
			})
			if startErr := p.Start(); startErr != nil {
				errChan <- fmt.Errorf("TUI error: %w", startErr)
			}
		} else {
			errChan <- fmt.Errorf("TUI prompter is not the expected type")
		}
	}()

	// Wait for TUI to be ready or error
	select {
	case <-ready:
		// TUI is ready, continue
	case tuiErr := <-errChan:
		return tuiErr
	case <-ctx.Done():
		return ctx.Err()
	}

	// Get transactions to classify
	// Note: The engine will handle fetching transactions based on the fromDate
	// This is just to check if there are any transactions at all
	var fromDate *time.Time
	if cfg.StartDate != "" {
		parsed, parseErr := time.Parse("2006-01-02", cfg.StartDate)
		if parseErr != nil {
			return fmt.Errorf("invalid start date: %w", parseErr)
		}
		fromDate = &parsed
	}

	transactions, err := cfg.Storage.GetTransactionsToClassify(ctx, fromDate)
	if err != nil {
		return fmt.Errorf("failed to get transactions: %w", err)
	}

	if len(transactions) == 0 {
		// No transactions to classify
		if p, ok := tuiPrompter.(*Prompter); ok {
			p.ShowMessage("No transactions need classification")
			p.Quit()
		}
		wg.Wait()
		return nil
	}

	// Create classification engine
	classificationEngine := engine.New(cfg.Storage, cfg.Classifier, tuiPrompter)

	// Run classification
	if err := classificationEngine.ClassifyTransactions(ctx, fromDate); err != nil {
		return fmt.Errorf("classification failed: %w", err)
	}

	// Wait for TUI to finish
	wg.Wait()

	// Check for any TUI errors
	select {
	case err := <-errChan:
		return err
	default:
		return nil
	}
}
