package tui

import (
	"context"
	"fmt"

	"github.com/Veraticus/the-spice-must-flow/internal/engine"
	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/service"
	tea "github.com/charmbracelet/bubbletea"
)

// Prompter implements engine.Prompter with a rich TUI.
type Prompter struct {
	program    *tea.Program
	resultChan chan promptResult
	errorChan  chan error
	model      Model
	stats      service.CompletionStats
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

	model := newModel(cfg)
	p := &Prompter{
		model:      model,
		resultChan: make(chan promptResult, 100), // Buffer for batch operations
		errorChan:  make(chan error, 1),
		stats:      service.CompletionStats{},
	}

	p.program = tea.NewProgram(
		model,
		tea.WithContext(ctx),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	// Store channels in model for communication
	p.model.resultChan = p.resultChan
	p.model.errorChan = p.errorChan

	return p, nil
}

// Start initializes and runs the TUI program.
func (p *Prompter) Start() error {
	if _, err := p.program.Run(); err != nil {
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
	close(p.resultChan)
	close(p.errorChan)
}

// Model returns the underlying model for testing.
func (p *Prompter) Model() Model {
	return p.model
}
