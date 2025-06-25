package tui

import (
	"context"
	"fmt"
	"os"

	"github.com/Veraticus/the-spice-must-flow/internal/engine"
	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/service"
	tea "github.com/charmbracelet/bubbletea"
)

// Prompter implements engine.Prompter with a rich TUI.
type Prompter struct {
	program       *tea.Program
	resultChan    chan promptResult
	errorChan     chan error
	readyCallback func()
	model         Model
	stats         service.CompletionStats
}

type promptResult struct {
	err            error
	direction      model.TransactionDirection
	classification model.Classification
}

// Ensure we implement the interface.
var _ engine.Prompter = (*Prompter)(nil)

// New creates a TUI prompter that can replace the CLI prompter.
func New(ctx context.Context, opts ...Option) (engine.Prompter, error) {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	// Create prompter first
	p := &Prompter{
		resultChan: make(chan promptResult, 100), // Buffer for batch operations
		errorChan:  make(chan error, 1),
		stats:      service.CompletionStats{},
	}

	// Create model with channels
	model := newModel(cfg)
	model.resultChan = p.resultChan
	model.errorChan = p.errorChan
	p.model = model

	// Create program
	p.program = tea.NewProgram(
		&p.model,
		tea.WithContext(ctx),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	// Set ready callback after program is created
	model.readyCallback = func() {
		if p.readyCallback != nil {
			p.readyCallback()
		}
	}

	return p, nil
}

// Start initializes and runs the TUI program.
func (p *Prompter) Start() error {
	// Ensure terminal is restored on exit
	// The tea.Program handles this internally, but we'll make sure
	// by using WithoutSignalHandler and handling cleanup ourselves
	_, err := p.program.Run()
	if err != nil {
		// Ensure we exit alt screen mode even on error
		// Ignore write errors as this is best-effort cleanup
		_, _ = os.Stdout.Write([]byte("\033[?1049l")) // Exit alternate screen
		_, _ = os.Stdout.Write([]byte("\033[?25h"))   // Show cursor
		return fmt.Errorf("failed to run TUI: %w", err)
	}
	return nil
}

// ConfirmClassification implements engine.Prompter.
func (p *Prompter) ConfirmClassification(ctx context.Context, pending model.PendingClassification) (model.Classification, error) {
	// Send pending classification to TUI
	p.program.Send(classificationRequestMsg{
		pending: pending,
		single:  true,
	})

	// Wait for user interaction
	select {
	case result := <-p.resultChan:
		if result.err != nil {
			return model.Classification{}, result.err
		}

		// Update stats
		p.stats.TotalTransactions++
		switch result.classification.Status {
		case model.StatusClassifiedByAI:
			p.stats.AutoClassified++
		case model.StatusUserModified:
			p.stats.UserClassified++
		}

		return result.classification, nil
	case <-ctx.Done():
		return model.Classification{}, ctx.Err()
	}
}

// BatchConfirmClassifications implements engine.Prompter.
func (p *Prompter) BatchConfirmClassifications(ctx context.Context, pending []model.PendingClassification) ([]model.Classification, error) {
	if len(pending) == 0 {
		return []model.Classification{}, nil
	}

	// Send batch to TUI
	p.program.Send(batchClassificationRequestMsg{
		pending: pending,
	})

	// Collect results
	var results []model.Classification
	for i := 0; i < len(pending); i++ {
		select {
		case result := <-p.resultChan:
			if result.err != nil {
				return nil, result.err
			}

			// Update stats
			p.stats.TotalTransactions++
			switch result.classification.Status {
			case model.StatusClassifiedByAI:
				p.stats.AutoClassified++
			case model.StatusUserModified:
				p.stats.UserClassified++
			}

			results = append(results, result.classification)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return results, nil
}

// ConfirmTransactionDirection implements engine.Prompter.
func (p *Prompter) ConfirmTransactionDirection(ctx context.Context, pending engine.PendingDirection) (model.TransactionDirection, error) {
	// Send direction confirmation request to TUI
	p.program.Send(directionRequestMsg{
		pending: pending,
	})

	// Wait for user interaction
	select {
	case result := <-p.resultChan:
		if result.err != nil {
			return "", result.err
		}
		return result.direction, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// GetCompletionStats implements engine.Prompter.
func (p *Prompter) GetCompletionStats() service.CompletionStats {
	return p.stats
}

// Shutdown gracefully stops the TUI.
func (p *Prompter) Shutdown() {
	if p.program != nil {
		p.program.Quit()
	}
	// Don't close channels here as they might already be closed
	// The goroutines using them will handle their lifecycle
}

// Model returns the underlying model for testing.
func (p *Prompter) Model() Model {
	return p.model
}

// SetReadyCallback sets a function to be called when the TUI is ready.
func (p *Prompter) SetReadyCallback(callback func()) {
	p.readyCallback = callback
}

// ShowMessage displays a message to the user.
func (p *Prompter) ShowMessage(msg string) {
	p.program.Send(showMessageMsg{message: msg})
}

// Quit gracefully shuts down the TUI.
func (p *Prompter) Quit() {
	p.program.Quit()
}
