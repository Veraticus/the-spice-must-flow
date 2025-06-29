package analysis

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
)

func TestOptions_Validate(t *testing.T) {
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)
	tomorrow := now.AddDate(0, 0, 1)

	tests := []struct {
		name    string
		errMsg  string
		opts    Options
		wantErr bool
	}{
		{
			name: "valid options",
			opts: Options{
				StartDate: yesterday,
				EndDate:   now,
				Focus:     FocusAll,
				MaxIssues: 10,
			},
			wantErr: false,
		},
		{
			name: "missing start date",
			opts: Options{
				EndDate:   now,
				Focus:     FocusAll,
				MaxIssues: 10,
			},
			wantErr: true,
			errMsg:  "start date is required",
		},
		{
			name: "missing end date",
			opts: Options{
				StartDate: yesterday,
				Focus:     FocusAll,
				MaxIssues: 10,
			},
			wantErr: true,
			errMsg:  "end date is required",
		},
		{
			name: "end date before start date",
			opts: Options{
				StartDate: tomorrow,
				EndDate:   yesterday,
				Focus:     FocusAll,
				MaxIssues: 10,
			},
			wantErr: true,
			errMsg:  "end date must be after start date",
		},
		{
			name: "negative max issues",
			opts: Options{
				StartDate: yesterday,
				EndDate:   now,
				Focus:     FocusAll,
				MaxIssues: -1,
			},
			wantErr: true,
			errMsg:  "max issues must be non-negative",
		},
		{
			name: "empty focus defaults to all",
			opts: Options{
				StartDate: yesterday,
				EndDate:   now,
				MaxIssues: 10,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opts.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
				if tt.name == "empty focus defaults to all" {
					assert.Equal(t, FocusAll, tt.opts.Focus)
				}
			}
		})
	}
}

func TestReport_Validate(t *testing.T) {
	validIssue := Issue{
		ID:             "issue-1",
		Type:           IssueTypeMiscategorized,
		Severity:       SeverityHigh,
		Description:    "Test issue",
		AffectedCount:  1,
		TransactionIDs: []string{"txn-1"},
		Confidence:     0.8,
	}

	validPattern := SuggestedPattern{
		ID:          "pattern-1",
		Name:        "Test Pattern",
		Description: "Test pattern description",
		Pattern: model.PatternRule{
			Name:            "Test Pattern",
			DefaultCategory: "TestCategory",
			AmountMin:       floatPtr(10.0),
			AmountMax:       floatPtr(100.0),
		},
		Confidence: 0.9,
		MatchCount: 5,
	}

	tests := []struct {
		name    string
		errMsg  string
		report  Report
		wantErr bool
	}{
		{
			name: "valid report",
			report: Report{
				ID:                "report-1",
				SessionID:         "session-1",
				GeneratedAt:       time.Now(),
				PeriodStart:       time.Now().AddDate(0, -1, 0),
				PeriodEnd:         time.Now(),
				CoherenceScore:    0.85,
				Issues:            []Issue{validIssue},
				SuggestedPatterns: []SuggestedPattern{validPattern},
			},
			wantErr: false,
		},
		{
			name: "missing report ID",
			report: Report{
				SessionID:      "session-1",
				GeneratedAt:    time.Now(),
				CoherenceScore: 0.85,
			},
			wantErr: true,
			errMsg:  "report ID is required",
		},
		{
			name: "missing session ID",
			report: Report{
				ID:             "report-1",
				GeneratedAt:    time.Now(),
				CoherenceScore: 0.85,
			},
			wantErr: true,
			errMsg:  "session ID is required",
		},
		{
			name: "coherence score too low",
			report: Report{
				ID:             "report-1",
				SessionID:      "session-1",
				GeneratedAt:    time.Now(),
				CoherenceScore: -0.1,
			},
			wantErr: true,
			errMsg:  "coherence score must be between 0 and 1",
		},
		{
			name: "coherence score too high",
			report: Report{
				ID:             "report-1",
				SessionID:      "session-1",
				GeneratedAt:    time.Now(),
				CoherenceScore: 1.1,
			},
			wantErr: true,
			errMsg:  "coherence score must be between 0 and 1",
		},
		{
			name: "invalid issue",
			report: Report{
				ID:             "report-1",
				SessionID:      "session-1",
				GeneratedAt:    time.Now(),
				CoherenceScore: 0.85,
				Issues: []Issue{
					{
						ID:       "issue-1",
						Type:     IssueTypeMiscategorized,
						Severity: SeverityHigh,
						// Missing description
					},
				},
			},
			wantErr: true,
			errMsg:  "invalid issue at index 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.report.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIssue_Validate(t *testing.T) {
	tests := []struct {
		name    string
		errMsg  string
		issue   Issue
		wantErr bool
	}{
		{
			name: "valid issue",
			issue: Issue{
				ID:             "issue-1",
				Type:           IssueTypeMiscategorized,
				Severity:       SeverityHigh,
				Description:    "Test issue",
				AffectedCount:  2,
				TransactionIDs: []string{"txn-1", "txn-2"},
				Confidence:     0.8,
			},
			wantErr: false,
		},
		{
			name: "missing ID",
			issue: Issue{
				Type:        IssueTypeMiscategorized,
				Severity:    SeverityHigh,
				Description: "Test issue",
			},
			wantErr: true,
			errMsg:  "issue ID is required",
		},
		{
			name: "missing type",
			issue: Issue{
				ID:          "issue-1",
				Severity:    SeverityHigh,
				Description: "Test issue",
			},
			wantErr: true,
			errMsg:  "issue type is required",
		},
		{
			name: "missing severity",
			issue: Issue{
				ID:          "issue-1",
				Type:        IssueTypeMiscategorized,
				Description: "Test issue",
			},
			wantErr: true,
			errMsg:  "issue severity is required",
		},
		{
			name: "missing description",
			issue: Issue{
				ID:       "issue-1",
				Type:     IssueTypeMiscategorized,
				Severity: SeverityHigh,
			},
			wantErr: true,
			errMsg:  "issue description is required",
		},
		{
			name: "confidence too low",
			issue: Issue{
				ID:          "issue-1",
				Type:        IssueTypeMiscategorized,
				Severity:    SeverityHigh,
				Description: "Test issue",
				Confidence:  -0.1,
			},
			wantErr: true,
			errMsg:  "confidence must be between 0 and 1",
		},
		{
			name: "confidence too high",
			issue: Issue{
				ID:          "issue-1",
				Type:        IssueTypeMiscategorized,
				Severity:    SeverityHigh,
				Description: "Test issue",
				Confidence:  1.1,
			},
			wantErr: true,
			errMsg:  "confidence must be between 0 and 1",
		},
		{
			name: "negative affected count",
			issue: Issue{
				ID:            "issue-1",
				Type:          IssueTypeMiscategorized,
				Severity:      SeverityHigh,
				Description:   "Test issue",
				AffectedCount: -1,
				Confidence:    0.8,
			},
			wantErr: true,
			errMsg:  "affected count must be non-negative",
		},
		{
			name: "affected count without transaction IDs",
			issue: Issue{
				ID:            "issue-1",
				Type:          IssueTypeMiscategorized,
				Severity:      SeverityHigh,
				Description:   "Test issue",
				AffectedCount: 2,
				Confidence:    0.8,
			},
			wantErr: true,
			errMsg:  "transaction IDs required when affected count > 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.issue.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSuggestedPattern_Validate(t *testing.T) {
	tests := []struct {
		name    string
		errMsg  string
		pattern SuggestedPattern
		wantErr bool
	}{
		{
			name: "valid pattern",
			pattern: SuggestedPattern{
				ID:          "pattern-1",
				Name:        "Test Pattern",
				Description: "Test pattern description",
				Pattern: model.PatternRule{
					Name:            "Test Pattern",
					DefaultCategory: "TestCategory",
					AmountMin:       floatPtr(10.0),
					AmountMax:       floatPtr(100.0),
				},
				Confidence: 0.9,
				MatchCount: 5,
			},
			wantErr: false,
		},
		{
			name: "missing ID",
			pattern: SuggestedPattern{
				Name:        "Test Pattern",
				Description: "Test pattern description",
				Pattern: model.PatternRule{
					Name:            "Test Pattern",
					DefaultCategory: "TestCategory",
				},
				Confidence: 0.9,
			},
			wantErr: true,
			errMsg:  "pattern ID is required",
		},
		{
			name: "missing name",
			pattern: SuggestedPattern{
				ID:          "pattern-1",
				Description: "Test pattern description",
				Pattern: model.PatternRule{
					DefaultCategory: "TestCategory",
				},
				Confidence: 0.9,
			},
			wantErr: true,
			errMsg:  "pattern name is required",
		},
		{
			name: "missing description",
			pattern: SuggestedPattern{
				ID:   "pattern-1",
				Name: "Test Pattern",
				Pattern: model.PatternRule{
					Name:            "Test Pattern",
					DefaultCategory: "TestCategory",
				},
				Confidence: 0.9,
			},
			wantErr: true,
			errMsg:  "pattern description is required",
		},
		{
			name: "invalid pattern rule",
			pattern: SuggestedPattern{
				ID:          "pattern-1",
				Name:        "Test Pattern",
				Description: "Test pattern description",
				Pattern:     model.PatternRule{
					// Missing required fields
				},
				Confidence: 0.9,
			},
			wantErr: true,
			errMsg:  "pattern rule name is required",
		},
		{
			name: "confidence too low",
			pattern: SuggestedPattern{
				ID:          "pattern-1",
				Name:        "Test Pattern",
				Description: "Test pattern description",
				Pattern: model.PatternRule{
					Name:            "Test Pattern",
					DefaultCategory: "TestCategory",
				},
				Confidence: -0.1,
			},
			wantErr: true,
			errMsg:  "confidence must be between 0 and 1",
		},
		{
			name: "negative match count",
			pattern: SuggestedPattern{
				ID:          "pattern-1",
				Name:        "Test Pattern",
				Description: "Test pattern description",
				Pattern: model.PatternRule{
					Name:            "Test Pattern",
					DefaultCategory: "TestCategory",
				},
				Confidence: 0.9,
				MatchCount: -1,
			},
			wantErr: true,
			errMsg:  "match count must be non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.pattern.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIssueSeverity_GetSeverityOrder(t *testing.T) {
	tests := []struct {
		name     string
		severity IssueSeverity
		want     int
	}{
		{
			name:     "critical",
			severity: SeverityCritical,
			want:     1,
		},
		{
			name:     "high",
			severity: SeverityHigh,
			want:     2,
		},
		{
			name:     "medium",
			severity: SeverityMedium,
			want:     3,
		},
		{
			name:     "low",
			severity: SeverityLow,
			want:     4,
		},
		{
			name:     "unknown",
			severity: IssueSeverity("unknown"),
			want:     5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.severity.GetSeverityOrder()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestStatus_IsTerminal(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		want   bool
	}{
		{
			name:   "pending is not terminal",
			status: StatusPending,
			want:   false,
		},
		{
			name:   "in progress is not terminal",
			status: StatusInProgress,
			want:   false,
		},
		{
			name:   "validating is not terminal",
			status: StatusValidating,
			want:   false,
		},
		{
			name:   "completed is terminal",
			status: StatusCompleted,
			want:   true,
		},
		{
			name:   "failed is terminal",
			status: StatusFailed,
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.status.IsTerminal()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestReport_GetIssuesBySeverity(t *testing.T) {
	report := &Report{
		Issues: []Issue{
			{ID: "1", Severity: SeverityCritical},
			{ID: "2", Severity: SeverityHigh},
			{ID: "3", Severity: SeverityCritical},
			{ID: "4", Severity: SeverityMedium},
			{ID: "5", Severity: SeverityLow},
		},
	}

	tests := []struct {
		name     string
		severity IssueSeverity
		wantIDs  []string
	}{
		{
			name:     "critical issues",
			severity: SeverityCritical,
			wantIDs:  []string{"1", "3"},
		},
		{
			name:     "high issues",
			severity: SeverityHigh,
			wantIDs:  []string{"2"},
		},
		{
			name:     "medium issues",
			severity: SeverityMedium,
			wantIDs:  []string{"4"},
		},
		{
			name:     "low issues",
			severity: SeverityLow,
			wantIDs:  []string{"5"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := report.GetIssuesBySeverity(tt.severity)
			var gotIDs []string
			for _, issue := range issues {
				gotIDs = append(gotIDs, issue.ID)
			}
			assert.Equal(t, tt.wantIDs, gotIDs)
		})
	}
}

func TestReport_GetIssuesByType(t *testing.T) {
	report := &Report{
		Issues: []Issue{
			{ID: "1", Type: IssueTypeMiscategorized},
			{ID: "2", Type: IssueTypeInconsistent},
			{ID: "3", Type: IssueTypeMiscategorized},
			{ID: "4", Type: IssueTypeMissingPattern},
			{ID: "5", Type: IssueTypeDuplicatePattern},
		},
	}

	tests := []struct {
		name      string
		issueType IssueType
		wantIDs   []string
	}{
		{
			name:      "miscategorized issues",
			issueType: IssueTypeMiscategorized,
			wantIDs:   []string{"1", "3"},
		},
		{
			name:      "inconsistent issues",
			issueType: IssueTypeInconsistent,
			wantIDs:   []string{"2"},
		},
		{
			name:      "missing pattern issues",
			issueType: IssueTypeMissingPattern,
			wantIDs:   []string{"4"},
		},
		{
			name:      "duplicate pattern issues",
			issueType: IssueTypeDuplicatePattern,
			wantIDs:   []string{"5"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := report.GetIssuesByType(tt.issueType)
			var gotIDs []string
			for _, issue := range issues {
				gotIDs = append(gotIDs, issue.ID)
			}
			assert.Equal(t, tt.wantIDs, gotIDs)
		})
	}
}

func TestReport_HasActionableIssues(t *testing.T) {
	tests := []struct {
		name   string
		report Report
		want   bool
	}{
		{
			name: "report with actionable issues",
			report: Report{
				Issues: []Issue{
					{ID: "1", Fix: &Fix{ID: "fix-1"}},
					{ID: "2"},
				},
			},
			want: true,
		},
		{
			name: "report without actionable issues",
			report: Report{
				Issues: []Issue{
					{ID: "1"},
					{ID: "2"},
				},
			},
			want: false,
		},
		{
			name:   "empty report",
			report: Report{},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.report.HasActionableIssues()
			assert.Equal(t, tt.want, got)
		})
	}
}

// Helper function for creating float pointers.
func floatPtr(f float64) *float64 {
	return &f
}
