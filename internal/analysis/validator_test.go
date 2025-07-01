package analysis

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONValidator_Validate(t *testing.T) {
	validator := NewJSONValidator()

	// Helper to create a valid report
	createValidReport := func() *Report {
		now := time.Now()
		return &Report{
			ID:             "report-123",
			SessionID:      "session-123",
			GeneratedAt:    now,
			PeriodStart:    now.AddDate(0, -1, 0),
			PeriodEnd:      now,
			CoherenceScore: 0.85,
			Issues: []Issue{
				{
					ID:                "issue-1",
					Type:              IssueTypeMiscategorized,
					Severity:          SeverityHigh,
					Description:       "Transaction miscategorized",
					TransactionIDs:    []string{"txn-1", "txn-2"},
					AffectedCount:     2,
					Confidence:        0.9,
					CurrentCategory:   stringPtr("Entertainment"),
					SuggestedCategory: stringPtr("Groceries"),
				},
			},
			SuggestedPatterns: []SuggestedPattern{
				{
					ID:            "pattern-1",
					Name:          "Walmart Groceries",
					Description:   "Walmart transactions under $200",
					Impact:        "Would correctly categorize 50 transactions",
					MatchCount:    50,
					Confidence:    0.95,
					ExampleTxnIDs: []string{"txn-3", "txn-4"},
					Pattern: model.PatternRule{
						Name:            "Walmart Groceries",
						MerchantPattern: "WALMART",
						DefaultCategory: "Groceries",
						AmountCondition: "lt",
						AmountValue:     floatPtr(200),
						Confidence:      0.95,
					},
				},
			},
			Insights: []string{
				"Found 10 recurring subscriptions",
				"Entertainment spending increased 20% this month",
			},
			CategorySummary: map[string]CategoryStat{
				"Groceries": {
					CategoryID:       "cat-1",
					CategoryName:     "Groceries",
					TransactionCount: 45,
					TotalAmount:      1234.56,
					Consistency:      0.92,
					Issues:           2,
				},
			},
		}
	}

	tests := []struct {
		setupReport func() *Report
		name        string
		input       string
		errContains string
		wantErr     bool
	}{
		{
			name: "valid report JSON",
			input: func() string {
				report := createValidReport()
				data, _ := json.Marshal(report)
				return string(data)
			}(),
			wantErr: false,
		},
		{
			name:        "invalid JSON syntax",
			input:       `{"id": "test", "broken": }`,
			wantErr:     true,
			errContains: "failed to parse JSON report",
		},
		{
			name:        "missing required field - ID",
			input:       `{"session_id": "session-123", "generated_at": "2024-01-01T00:00:00Z", "period_start": "2024-01-01T00:00:00Z", "period_end": "2024-01-31T00:00:00Z", "coherence_score": 0.85, "issues": [], "suggested_patterns": []}`,
			wantErr:     true,
			errContains: "report ID is required",
		},
		{
			name:        "missing required field - SessionID",
			input:       `{"id": "report-123", "generated_at": "2024-01-01T00:00:00Z", "period_start": "2024-01-01T00:00:00Z", "period_end": "2024-01-31T00:00:00Z", "coherence_score": 0.85, "issues": [], "suggested_patterns": []}`,
			wantErr:     true,
			errContains: "session ID is required",
		},
		{
			name: "invalid coherence score - too high",
			input: func() string {
				report := createValidReport()
				report.CoherenceScore = 1.5
				data, _ := json.Marshal(report)
				return string(data)
			}(),
			wantErr:     true,
			errContains: "coherence score must be between 0 and 1",
		},
		{
			name: "invalid coherence score - negative",
			input: func() string {
				report := createValidReport()
				report.CoherenceScore = -0.5
				data, _ := json.Marshal(report)
				return string(data)
			}(),
			wantErr:     true,
			errContains: "coherence score must be between 0 and 1",
		},
		{
			name: "invalid issue - missing ID",
			input: func() string {
				report := createValidReport()
				report.Issues[0].ID = ""
				data, _ := json.Marshal(report)
				return string(data)
			}(),
			wantErr:     true,
			errContains: "issue ID is required",
		},
		{
			name: "custom issue type - allowed for flexibility",
			input: func() string {
				report := createValidReport()
				report.Issues[0].Type = "custom_issue_type"
				data, _ := json.Marshal(report)
				return string(data)
			}(),
			wantErr: false, // Should succeed - AI can discover new issue types
		},
		{
			name: "invalid issue severity",
			input: func() string {
				report := createValidReport()
				report.Issues[0].Severity = "extreme"
				data, _ := json.Marshal(report)
				return string(data)
			}(),
			wantErr:     true,
			errContains: "invalid issue severity",
		},
		{
			name: "issue with no affected transactions",
			input: func() string {
				report := createValidReport()
				report.Issues[0].TransactionIDs = []string{}
				data, _ := json.Marshal(report)
				return string(data)
			}(),
			wantErr:     true,
			errContains: "transaction IDs required when affected count > 0",
		},
		{
			name: "issue with invalid confidence",
			input: func() string {
				report := createValidReport()
				report.Issues[0].Confidence = 1.5
				data, _ := json.Marshal(report)
				return string(data)
			}(),
			wantErr:     true,
			errContains: "confidence must be between 0 and 1",
		},
		{
			name: "issue with invalid fix",
			input: func() string {
				report := createValidReport()
				report.Issues[0].Fix = &Fix{
					ID:          "",
					IssueID:     "issue-1",
					Type:        "recategorize",
					Description: "Fix description",
					Data:        map[string]any{"category": "Groceries"},
				}
				data, _ := json.Marshal(report)
				return string(data)
			}(),
			wantErr:     true,
			errContains: "fix ID is required",
		},
		{
			name: "invalid suggested pattern - missing name",
			input: func() string {
				report := createValidReport()
				report.SuggestedPatterns[0].Name = ""
				data, _ := json.Marshal(report)
				return string(data)
			}(),
			wantErr:     true,
			errContains: "pattern name is required",
		},
		{
			name: "invalid suggested pattern - zero match count",
			input: func() string {
				report := createValidReport()
				report.SuggestedPatterns[0].MatchCount = 0
				data, _ := json.Marshal(report)
				return string(data)
			}(),
			wantErr:     true,
			errContains: "match count must be positive",
		},
		{
			name: "invalid suggested pattern - invalid confidence",
			input: func() string {
				report := createValidReport()
				report.SuggestedPatterns[0].Confidence = -0.1
				data, _ := json.Marshal(report)
				return string(data)
			}(),
			wantErr:     true,
			errContains: "confidence must be between 0 and 1",
		},
		{
			name: "invalid suggested pattern - invalid pattern rule",
			input: func() string {
				report := createValidReport()
				report.SuggestedPatterns[0].Pattern.Name = ""
				data, _ := json.Marshal(report)
				return string(data)
			}(),
			wantErr:     true,
			errContains: "pattern rule name is required",
		},
		{
			name:        "unknown fields rejected",
			input:       `{"id": "report-123", "session_id": "session-123", "unknown_field": "value", "generated_at": "2024-01-01T00:00:00Z", "period_start": "2024-01-01T00:00:00Z", "period_end": "2024-01-31T00:00:00Z", "coherence_score": 0.85, "issues": [], "suggested_patterns": []}`,
			wantErr:     true,
			errContains: "unknown field",
		},
		{
			name: "valid report with all optional fields",
			input: func() string {
				report := createValidReport()
				report.Issues[0].Fix = &Fix{
					ID:          "fix-1",
					IssueID:     "issue-1",
					Type:        "recategorize",
					Description: "Recategorize to Groceries",
					Data: map[string]any{
						"category": "Groceries",
						"reason":   "Walmart is primarily groceries",
					},
					Applied:   true,
					AppliedAt: timePtr(time.Now()),
				}
				data, _ := json.Marshal(report)
				return string(data)
			}(),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report, err := validator.Validate([]byte(tt.input))

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				assert.Nil(t, report)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, report)
			}
		})
	}
}

func TestJSONValidator_ExtractError(t *testing.T) {
	validator := NewJSONValidator()

	tests := []struct {
		err          error
		checkSection func(t *testing.T, section string)
		name         string
		data         string
		wantSection  string
		wantLine     int
		wantColumn   int
	}{
		{
			name: "JSON syntax error",
			data: `{
				"id": "test",
				"broken": ,
				"field": "value"
			}`,
			err:        &json.SyntaxError{Offset: 32},
			wantLine:   3,
			wantColumn: 13,
			checkSection: func(t *testing.T, section string) {
				t.Helper()
				assert.Contains(t, section, "broken")
			},
		},
		{
			name: "type mismatch error",
			data: `{"id": "test", "coherence_score": "not a number"}`,
			err: &json.UnmarshalTypeError{
				Field:  "coherence_score",
				Type:   floatType(),
				Offset: 34,
			},
			wantLine:   1,
			wantColumn: 35,
			checkSection: func(t *testing.T, section string) {
				t.Helper()
				assert.Contains(t, section, "coherence_score")
				assert.Contains(t, section, "float64")
			},
		},
		{
			name: "validation error with array index",
			data: `{"issues": [{"id": "1"}, {"id": "2"}]}`,
			err:  fmt.Errorf("invalid issue at index 1: missing description"),
			checkSection: func(t *testing.T, section string) {
				t.Helper()
				// Should extract context around issues array
				assert.True(t, strings.Contains(section, "issues") || section == "array element")
			},
		},
		{
			name: "validation error with field name",
			data: `{"id": "", "session_id": "test"}`,
			err:  fmt.Errorf("invalid field 'id': cannot be empty"),
			checkSection: func(t *testing.T, section string) {
				t.Helper()
				// Should extract context around id field
				assert.True(t, strings.Contains(section, "id") || section == "field")
			},
		},
		{
			name: "generic error",
			data: `{"id": "test"}`,
			err:  fmt.Errorf("some generic error"),
			checkSection: func(t *testing.T, section string) {
				t.Helper()
				assert.Equal(t, "unknown", section)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			section, line, column := validator.ExtractError([]byte(tt.data), tt.err)

			if tt.wantLine > 0 {
				assert.Equal(t, tt.wantLine, line)
			}
			if tt.wantColumn > 0 {
				assert.Equal(t, tt.wantColumn, column)
			}
			if tt.checkSection != nil {
				tt.checkSection(t, section)
			}
		})
	}
}

func TestCalculatePosition(t *testing.T) {
	tests := []struct {
		name       string
		data       string
		offset     int64
		wantLine   int
		wantColumn int
	}{
		{
			name:       "first character",
			data:       "hello world",
			offset:     0,
			wantLine:   1,
			wantColumn: 1,
		},
		{
			name:       "middle of first line",
			data:       "hello world",
			offset:     6,
			wantLine:   1,
			wantColumn: 7,
		},
		{
			name:       "second line",
			data:       "hello\nworld",
			offset:     7,
			wantLine:   2,
			wantColumn: 2,
		},
		{
			name:       "multiple lines",
			data:       "line1\nline2\nline3",
			offset:     14,
			wantLine:   3,
			wantColumn: 3,
		},
		{
			name:       "offset beyond data",
			data:       "short",
			offset:     100,
			wantLine:   1,
			wantColumn: 6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			line, column := calculatePosition([]byte(tt.data), tt.offset)
			assert.Equal(t, tt.wantLine, line)
			assert.Equal(t, tt.wantColumn, column)
		})
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}

func floatType() reflect.Type {
	var f float64
	return reflect.TypeOf(f)
}
