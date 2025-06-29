// Package analysis provides AI-powered transaction analysis functionality.
package analysis

import (
	"fmt"

	"github.com/Veraticus/the-spice-must-flow/internal/service"
)

// Deps contains all dependencies required by the analysis engine.
type Deps struct {
	// Storage provides access to the persistence layer.
	Storage service.Storage
	// LLMClient provides access to the language model.
	LLMClient LLMClient
	// SessionStore manages analysis session persistence.
	SessionStore SessionStore
	// ReportStore manages analysis report persistence.
	ReportStore ReportStore
	// Validator validates and corrects analysis reports.
	Validator ReportValidator
	// FixApplier applies recommended fixes.
	FixApplier FixApplier
	// PromptBuilder constructs prompts for analysis.
	PromptBuilder PromptBuilder
	// Formatter formats reports for display.
	Formatter ReportFormatter
}

// Validate ensures all required dependencies are provided.
func (d *Deps) Validate() error {
	if d.Storage == nil {
		return fmt.Errorf("storage dependency is required")
	}
	if d.LLMClient == nil {
		return fmt.Errorf("LLM client dependency is required")
	}
	if d.SessionStore == nil {
		return fmt.Errorf("session store dependency is required")
	}
	if d.ReportStore == nil {
		return fmt.Errorf("report store dependency is required")
	}
	if d.Validator == nil {
		return fmt.Errorf("validator dependency is required")
	}
	if d.FixApplier == nil {
		return fmt.Errorf("fix applier dependency is required")
	}
	if d.PromptBuilder == nil {
		return fmt.Errorf("prompt builder dependency is required")
	}
	if d.Formatter == nil {
		return fmt.Errorf("formatter dependency is required")
	}
	return nil
}

// Engine implements the Service interface.
type Engine struct {
	deps Deps
}

// NewEngine creates a new analysis engine with the provided dependencies.
func NewEngine(deps Deps) (*Engine, error) {
	if err := deps.Validate(); err != nil {
		return nil, fmt.Errorf("invalid dependencies: %w", err)
	}
	return &Engine{
		deps: deps,
	}, nil
}

// Config holds configuration options for the analysis engine.
type Config struct {
	// MaxRetries is the maximum number of attempts for JSON validation.
	MaxRetries int
	// MaxIssuesPerReport is the maximum number of issues to include in a report.
	MaxIssuesPerReport int
	// SessionTimeout is the maximum age of a session before it's considered stale.
	SessionTimeout int
	// EnableAutoFix enables automatic application of high-confidence fixes.
	EnableAutoFix bool
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		MaxRetries:         3,
		MaxIssuesPerReport: 50,
		SessionTimeout:     3600, // 1 hour in seconds
		EnableAutoFix:      false,
	}
}

// NewEngineWithConfig creates an analysis engine with custom configuration.
func NewEngineWithConfig(deps Deps, config *Config) (*Engine, error) {
	if err := deps.Validate(); err != nil {
		return nil, fmt.Errorf("invalid dependencies: %w", err)
	}
	if config == nil {
		_ = DefaultConfig() // Config not used in current implementation
	}
	return &Engine{
		deps: deps,
	}, nil
}
