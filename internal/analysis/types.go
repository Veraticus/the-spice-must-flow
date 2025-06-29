package analysis

import (
	"fmt"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
)

// Status represents the current state of an analysis session.
type Status string

const (
	// StatusPending indicates the analysis has not started.
	StatusPending Status = "pending"
	// StatusInProgress indicates the analysis is currently running.
	StatusInProgress Status = "in_progress"
	// StatusValidating indicates the analysis is validating LLM output.
	StatusValidating Status = "validating"
	// StatusCompleted indicates the analysis finished successfully.
	StatusCompleted Status = "completed"
	// StatusFailed indicates the analysis failed after all retries.
	StatusFailed Status = "failed"
)

// IssueSeverity represents the severity level of an identified issue.
type IssueSeverity string

const (
	// SeverityCritical indicates an issue requiring immediate attention.
	SeverityCritical IssueSeverity = "critical"
	// SeverityHigh indicates a significant issue that should be addressed soon.
	SeverityHigh IssueSeverity = "high"
	// SeverityMedium indicates a moderate issue that can be scheduled for resolution.
	SeverityMedium IssueSeverity = "medium"
	// SeverityLow indicates a minor issue or optimization opportunity.
	SeverityLow IssueSeverity = "low"
)

// IssueType categorizes the type of issue identified.
type IssueType string

const (
	// IssueTypeMiscategorized indicates transactions assigned to wrong categories.
	IssueTypeMiscategorized IssueType = "miscategorized"
	// IssueTypeInconsistent indicates similar transactions categorized differently.
	IssueTypeInconsistent IssueType = "inconsistent"
	// IssueTypeMissingPattern indicates recurring transactions without pattern rules.
	IssueTypeMissingPattern IssueType = "missing_pattern"
	// IssueTypeDuplicatePattern indicates overlapping or redundant pattern rules.
	IssueTypeDuplicatePattern IssueType = "duplicate_pattern"
	// IssueTypeAmbiguousVendor indicates vendors with unclear categorization.
	IssueTypeAmbiguousVendor IssueType = "ambiguous_vendor"
)

// Focus represents the area of focus for the analysis.
type Focus string

const (
	// FocusCoherence focuses on overall data consistency and quality.
	FocusCoherence Focus = "coherence"
	// FocusPatterns focuses on pattern rule effectiveness and coverage.
	FocusPatterns Focus = "patterns"
	// FocusCategories focuses on category usage and distribution.
	FocusCategories Focus = "categories"
	// FocusAll analyzes all aspects without specific focus.
	FocusAll Focus = "all"
)

// Session represents a single analysis execution.
type Session struct {
	StartedAt   time.Time  `json:"started_at"`
	LastAttempt time.Time  `json:"last_attempt"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Error       *string    `json:"error,omitempty"`
	ReportID    *string    `json:"report_id,omitempty"`
	ID          string     `json:"id"`
	Status      Status     `json:"status"`
	Attempts    int        `json:"attempts"`
}

// Options configures how the analysis should be performed.
type Options struct {
	StartDate    time.Time        `json:"start_date"`
	EndDate      time.Time        `json:"end_date"`
	ProgressFunc ProgressCallback `json:"-"`
	Focus        Focus            `json:"focus"`
	SessionID    string           `json:"session_id"`
	MaxIssues    int              `json:"max_issues"`
	DryRun       bool             `json:"dry_run"`
	AutoApply    bool             `json:"auto_apply"`
}

// ProgressCallback provides updates during analysis execution.
type ProgressCallback func(stage string, percent int)

// Report contains the complete results of an analysis.
type Report struct {
	GeneratedAt       time.Time               `json:"generated_at"`
	PeriodStart       time.Time               `json:"period_start"`
	PeriodEnd         time.Time               `json:"period_end"`
	CategorySummary   map[string]CategoryStat `json:"category_summary"`
	ID                string                  `json:"id"`
	SessionID         string                  `json:"session_id"`
	Issues            []Issue                 `json:"issues"`
	SuggestedPatterns []SuggestedPattern      `json:"suggested_patterns"`
	Insights          []string                `json:"insights"`
	CoherenceScore    float64                 `json:"coherence_score"`
}

// Issue represents a specific problem identified during analysis.
type Issue struct {
	CurrentCategory   *string       `json:"current_category,omitempty"`
	SuggestedCategory *string       `json:"suggested_category,omitempty"`
	Fix               *Fix          `json:"fix,omitempty"`
	ID                string        `json:"id"`
	Type              IssueType     `json:"type"`
	Severity          IssueSeverity `json:"severity"`
	Description       string        `json:"description"`
	TransactionIDs    []string      `json:"transaction_ids"`
	AffectedCount     int           `json:"affected_count"`
	Confidence        float64       `json:"confidence"`
}

// Fix represents a corrective action for an issue.
type Fix struct {
	Data        map[string]any `json:"data"`
	AppliedAt   *time.Time     `json:"applied_at,omitempty"`
	ID          string         `json:"id"`
	IssueID     string         `json:"issue_id"`
	Description string         `json:"description"`
	Type        string         `json:"type"`
	Applied     bool           `json:"applied"`
}

// CategoryStat provides statistical information about a category.
type CategoryStat struct {
	CategoryID       string  `json:"category_id"`
	CategoryName     string  `json:"category_name"`
	TransactionCount int     `json:"transaction_count"`
	TotalAmount      float64 `json:"total_amount"`
	Consistency      float64 `json:"consistency"`
	Issues           int     `json:"issues"`
}

// SuggestedPattern represents a recommended pattern rule.
type SuggestedPattern struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	Impact        string            `json:"impact"`
	ExampleTxnIDs []string          `json:"example_txn_ids"`
	Pattern       model.PatternRule `json:"pattern"`
	MatchCount    int               `json:"match_count"`
	Confidence    float64           `json:"confidence"`
}

// FixPreview shows what would change if a fix is applied.
type FixPreview struct {
	EstimatedImpact map[string]float64 `json:"estimated_impact"`
	FixID           string             `json:"fix_id"`
	Changes         []PreviewChange    `json:"changes"`
	AffectedCount   int                `json:"affected_count"`
}

// PreviewChange represents a single change that would be made.
type PreviewChange struct {
	TransactionID string `json:"transaction_id"`
	FieldName     string `json:"field_name"`
	OldValue      string `json:"old_value"`
	NewValue      string `json:"new_value"`
}

// Validate ensures the AnalysisOptions are valid.
func (o *Options) Validate() error {
	if o.StartDate.IsZero() {
		return fmt.Errorf("start date is required")
	}
	if o.EndDate.IsZero() {
		return fmt.Errorf("end date is required")
	}
	if o.EndDate.Before(o.StartDate) {
		return fmt.Errorf("end date must be after start date")
	}
	if o.MaxIssues < 0 {
		return fmt.Errorf("max issues must be non-negative")
	}
	if o.Focus == "" {
		o.Focus = FocusAll
	}
	return nil
}

// Validate ensures the AnalysisReport is valid.
func (r *Report) Validate() error {
	if r.ID == "" {
		return fmt.Errorf("report ID is required")
	}
	if r.SessionID == "" {
		return fmt.Errorf("session ID is required")
	}
	if r.CoherenceScore < 0.0 || r.CoherenceScore > 1.0 {
		return fmt.Errorf("coherence score must be between 0 and 1")
	}
	for i, issue := range r.Issues {
		if err := issue.Validate(); err != nil {
			return fmt.Errorf("invalid issue at index %d: %w", i, err)
		}
	}
	for i, pattern := range r.SuggestedPatterns {
		if err := pattern.Validate(); err != nil {
			return fmt.Errorf("invalid suggested pattern at index %d: %w", i, err)
		}
	}
	return nil
}

// Validate ensures the Issue is valid.
func (i *Issue) Validate() error {
	if i.ID == "" {
		return fmt.Errorf("issue ID is required")
	}
	if i.Type == "" {
		return fmt.Errorf("issue type is required")
	}
	if i.Severity == "" {
		return fmt.Errorf("issue severity is required")
	}
	if i.Description == "" {
		return fmt.Errorf("issue description is required")
	}
	if i.Confidence < 0.0 || i.Confidence > 1.0 {
		return fmt.Errorf("confidence must be between 0 and 1")
	}
	if i.AffectedCount < 0 {
		return fmt.Errorf("affected count must be non-negative")
	}
	if i.AffectedCount > 0 && len(i.TransactionIDs) == 0 {
		return fmt.Errorf("transaction IDs required when affected count > 0")
	}
	return nil
}

// Validate ensures the SuggestedPattern is valid.
func (s *SuggestedPattern) Validate() error {
	if s.ID == "" {
		return fmt.Errorf("pattern ID is required")
	}
	if s.Name == "" {
		return fmt.Errorf("pattern name is required")
	}
	if s.Description == "" {
		return fmt.Errorf("pattern description is required")
	}
	// Validate pattern rule fields
	if s.Pattern.Name == "" {
		return fmt.Errorf("pattern rule name is required")
	}
	if s.Pattern.DefaultCategory == "" {
		return fmt.Errorf("pattern rule default category is required")
	}
	if s.Confidence < 0.0 || s.Confidence > 1.0 {
		return fmt.Errorf("confidence must be between 0 and 1")
	}
	if s.MatchCount < 0 {
		return fmt.Errorf("match count must be non-negative")
	}
	return nil
}

// GetSeverityOrder returns the numeric priority of a severity (lower is more severe).
func (s IssueSeverity) GetSeverityOrder() int {
	switch s {
	case SeverityCritical:
		return 1
	case SeverityHigh:
		return 2
	case SeverityMedium:
		return 3
	case SeverityLow:
		return 4
	default:
		return 5
	}
}

// IsTerminal returns true if the status represents a final state.
func (s Status) IsTerminal() bool {
	return s == StatusCompleted || s == StatusFailed
}

// GetIssuesBySeverity returns issues filtered by severity.
func (r *Report) GetIssuesBySeverity(severity IssueSeverity) []Issue {
	var filtered []Issue
	for _, issue := range r.Issues {
		if issue.Severity == severity {
			filtered = append(filtered, issue)
		}
	}
	return filtered
}

// GetIssuesByType returns issues filtered by type.
func (r *Report) GetIssuesByType(issueType IssueType) []Issue {
	var filtered []Issue
	for _, issue := range r.Issues {
		if issue.Type == issueType {
			filtered = append(filtered, issue)
		}
	}
	return filtered
}

// HasActionableIssues returns true if the report contains issues with fixes.
func (r *Report) HasActionableIssues() bool {
	for _, issue := range r.Issues {
		if issue.Fix != nil {
			return true
		}
	}
	return false
}
