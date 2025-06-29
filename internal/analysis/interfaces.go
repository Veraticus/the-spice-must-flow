package analysis

import (
	"context"
)

// Service performs AI-powered transaction analysis.

// SessionStore manages analysis session persistence.
type SessionStore interface {
	// Create creates a new analysis session.
	Create(ctx context.Context, session *Session) error
	// Get retrieves an analysis session by ID.
	Get(ctx context.Context, sessionID string) (*Session, error)
	// Update updates an existing analysis session.
	Update(ctx context.Context, session *Session) error
}

// ReportValidator validates and corrects analysis reports from LLM output.
type ReportValidator interface {
	// Validate checks if the report data is valid and well-formed.
	Validate(data []byte) (*Report, error)
	// ExtractError identifies the problematic section of malformed JSON.
	ExtractError(data []byte, err error) (section string, line int, column int)
}

// FixApplier applies recommended fixes to resolve identified issues.
type FixApplier interface {
	// ApplyPatternFixes creates or updates pattern rules based on suggestions.
	ApplyPatternFixes(ctx context.Context, patterns []SuggestedPattern) error
	// ApplyCategoryFixes updates transaction categories based on fixes.
	ApplyCategoryFixes(ctx context.Context, fixes []Fix) error
	// ApplyRecategorizations moves transactions to their suggested categories.
	ApplyRecategorizations(ctx context.Context, issues []Issue) error
}

// ReportFormatter formats analysis reports for display.
type ReportFormatter interface {
	// FormatSummary creates a high-level summary of the analysis report.
	FormatSummary(report *Report) string
	// FormatIssue formats a single issue for detailed display.
	FormatIssue(issue Issue) string
	// FormatInteractive creates an interactive menu for report navigation.
	FormatInteractive(report *Report) string
}

// TransactionLoader retrieves transactions for analysis.

// CategoryLoader retrieves categories for analysis.

// PatternLoader retrieves pattern rules for analysis.

// ReportStore manages analysis report persistence.
type ReportStore interface {
	// SaveReport stores an analysis report.
	SaveReport(ctx context.Context, report *Report) error
	// GetReport retrieves a report by ID.
	GetReport(ctx context.Context, reportID string) (*Report, error)
}

// LLMAdapter provides LLM-specific operations for analysis.

// PromptBuilder constructs prompts for LLM analysis.
type PromptBuilder interface {
	// BuildAnalysisPrompt creates the main analysis prompt.
	BuildAnalysisPrompt(data PromptData) (string, error)
	// BuildCorrectionPrompt creates a prompt to fix validation errors.
	BuildCorrectionPrompt(data CorrectionPromptData) (string, error)
}

// ProgressReporter provides progress updates during analysis.
