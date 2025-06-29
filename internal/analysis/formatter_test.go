package analysis

import (
	"strings"
	"testing"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCLIFormatter_FormatSummary(t *testing.T) {
	formatter := NewCLIFormatter()
	now := time.Now()

	tests := []struct {
		name        string
		report      *Report
		contains    []string
		notContains []string
	}{
		{
			name:   "nil report",
			report: nil,
			contains: []string{
				"No report available",
			},
		},
		{
			name: "complete report",
			report: &Report{
				ID:             "test-report-1",
				SessionID:      "session-1",
				GeneratedAt:    now,
				PeriodStart:    now.AddDate(0, -1, 0),
				PeriodEnd:      now,
				CoherenceScore: 0.85,
				Issues: []Issue{
					{
						ID:             "issue-1",
						Type:           IssueTypeMiscategorized,
						Severity:       SeverityHigh,
						Description:    "Groceries miscategorized as Entertainment",
						AffectedCount:  5,
						Confidence:     0.9,
						TransactionIDs: []string{"txn-1", "txn-2", "txn-3", "txn-4", "txn-5"},
					},
					{
						ID:             "issue-2",
						Type:           IssueTypeMissingPattern,
						Severity:       SeverityMedium,
						Description:    "Recurring Netflix subscription has no pattern",
						AffectedCount:  12,
						Confidence:     0.95,
						TransactionIDs: []string{"txn-10", "txn-11"},
					},
				},
				CategorySummary: map[string]CategoryStat{
					"groceries": {
						CategoryID:       "cat-1",
						CategoryName:     "Groceries",
						TransactionCount: 50,
						TotalAmount:      1250.50,
						Consistency:      0.92,
						Issues:           2,
					},
					"entertainment": {
						CategoryID:       "cat-2",
						CategoryName:     "Entertainment",
						TransactionCount: 20,
						TotalAmount:      450.00,
						Consistency:      0.65,
						Issues:           5,
					},
				},
				Insights: []string{
					"Your grocery spending is highly consistent",
					"Consider creating patterns for recurring subscriptions",
				},
				SuggestedPatterns: []SuggestedPattern{
					{
						ID:          "pattern-1",
						Name:        "Netflix Subscription",
						Description: "Monthly Netflix charge",
						Impact:      "Will automatically categorize 12 transactions per year",
						MatchCount:  12,
						Confidence:  0.98,
						Pattern: model.PatternRule{
							Name:            "Netflix Subscription",
							DefaultCategory: "entertainment",
						},
					},
				},
			},
			contains: []string{
				"Transaction Analysis Report",
				"Period:",
				"Generated:",
				"Coherence Score: 85.0%",
				"Issues Found:",
				"high: 1",
				"medium: 1",
				"Category Summary:",
				"Groceries",
				"Entertainment",
				"Key Insights:",
				"Suggested Pattern Rules:",
				"Netflix Subscription",
			},
			notContains: []string{
				"No issues found",
				"Low:",
				"Critical:",
			},
		},
		{
			name: "perfect coherence score",
			report: &Report{
				ID:             "test-report-2",
				SessionID:      "session-2",
				GeneratedAt:    now,
				PeriodStart:    now.AddDate(0, -1, 0),
				PeriodEnd:      now,
				CoherenceScore: 0.95,
				Issues:         []Issue{},
			},
			contains: []string{
				"🎯 Coherence Score: 95.0%",
			},
		},
		{
			name: "low coherence score",
			report: &Report{
				ID:             "test-report-3",
				SessionID:      "session-3",
				GeneratedAt:    now,
				PeriodStart:    now.AddDate(0, -1, 0),
				PeriodEnd:      now,
				CoherenceScore: 0.45,
				Issues:         []Issue{},
			},
			contains: []string{
				"❌ Coherence Score: 45.0%",
			},
		},
		{
			name: "no issues",
			report: &Report{
				ID:             "test-report-4",
				SessionID:      "session-4",
				GeneratedAt:    now,
				PeriodStart:    now.AddDate(0, -1, 0),
				PeriodEnd:      now,
				CoherenceScore: 0.92,
				Issues:         []Issue{},
			},
			contains: []string{
				"✅ No issues found!",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatter.FormatSummary(tt.report)

			// Check for expected content
			for _, expected := range tt.contains {
				assert.Contains(t, result, expected, "Expected to find: %s", expected)
			}

			// Check for unexpected content
			for _, unexpected := range tt.notContains {
				assert.NotContains(t, result, unexpected, "Did not expect to find: %s", unexpected)
			}
		})
	}
}

func TestCLIFormatter_FormatIssue(t *testing.T) {
	formatter := NewCLIFormatter()

	tests := []struct {
		name     string
		contains []string
		issue    Issue
	}{
		{
			name: "miscategorized issue with fix",
			issue: Issue{
				ID:                "issue-1",
				Type:              IssueTypeMiscategorized,
				Severity:          SeverityHigh,
				Description:       "5 grocery transactions incorrectly categorized as Entertainment",
				AffectedCount:     5,
				Confidence:        0.92,
				TransactionIDs:    []string{"txn-1", "txn-2", "txn-3", "txn-4", "txn-5"},
				CurrentCategory:   stringPtr("Entertainment"),
				SuggestedCategory: stringPtr("Groceries"),
				Fix: &Fix{
					ID:          "fix-1",
					IssueID:     "issue-1",
					Description: "Recategorize transactions to Groceries",
					Type:        "recategorization",
					Applied:     false,
				},
			},
			contains: []string{
				"⚠️ high Issue [miscategorized]",
				"5 grocery transactions incorrectly categorized as Entertainment",
				"Affected: 5 transaction(s)",
				"Confidence: 92%",
				"ID: issue-1",
				"Entertainment → Groceries",
				"🔧 Suggested Fix:",
				"Recategorize transactions to Groceries",
				"Not applied",
				"Affected Transactions:",
				"txn-1",
				"... and 2 more",
			},
		},
		{
			name: "pattern issue without fix",
			issue: Issue{
				ID:             "issue-2",
				Type:           IssueTypeMissingPattern,
				Severity:       SeverityMedium,
				Description:    "Recurring Spotify charges could benefit from a pattern rule",
				AffectedCount:  12,
				Confidence:     0.88,
				TransactionIDs: []string{"txn-10"},
			},
			contains: []string{
				"⚡ medium Issue [missing_pattern]",
				"Recurring Spotify charges",
				"Affected: 12 transaction(s)",
				"Confidence: 88%",
			},
		},
		{
			name: "critical issue",
			issue: Issue{
				ID:             "issue-3",
				Type:           IssueTypeInconsistent,
				Severity:       SeverityCritical,
				Description:    "Critical data inconsistency detected",
				AffectedCount:  50,
				Confidence:     0.99,
				TransactionIDs: []string{},
			},
			contains: []string{
				"🚨 critical Issue [inconsistent]",
				"Critical data inconsistency",
				"Affected: 50 transaction(s)",
			},
		},
		{
			name: "low severity issue",
			issue: Issue{
				ID:             "issue-4",
				Type:           IssueTypeAmbiguousVendor,
				Severity:       SeverityLow,
				Description:    "Vendor name could be clearer",
				AffectedCount:  2,
				Confidence:     0.65,
				TransactionIDs: []string{"txn-20", "txn-21"},
			},
			contains: []string{
				"💡 low Issue [ambiguous_vendor]",
				"Vendor name could be clearer",
				"Confidence: 65%",
			},
		},
		{
			name: "applied fix",
			issue: Issue{
				ID:            "issue-5",
				Type:          IssueTypeDuplicatePattern,
				Severity:      SeverityMedium,
				Description:   "Duplicate pattern rules detected",
				AffectedCount: 3,
				Confidence:    0.85,
				Fix: &Fix{
					ID:          "fix-5",
					IssueID:     "issue-5",
					Description: "Remove duplicate pattern",
					Type:        "pattern_removal",
					Applied:     true,
					AppliedAt:   timePtr(time.Now()),
				},
			},
			contains: []string{
				"Applied at",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatter.FormatIssue(tt.issue)

			for _, expected := range tt.contains {
				assert.Contains(t, result, expected, "Expected to find: %s", expected)
			}
		})
	}
}

func TestCLIFormatter_FormatInteractive(t *testing.T) {
	formatter := NewCLIFormatter()
	now := time.Now()

	tests := []struct {
		name     string
		report   *Report
		contains []string
	}{
		{
			name:   "nil report",
			report: nil,
			contains: []string{
				"No report available",
			},
		},
		{
			name: "interactive menu",
			report: &Report{
				ID:             "test-report",
				SessionID:      "session-1",
				GeneratedAt:    now,
				PeriodStart:    now.AddDate(0, -1, 0),
				PeriodEnd:      now,
				CoherenceScore: 0.78,
				Issues: []Issue{
					{ID: "i1", Type: IssueTypeMiscategorized, Severity: SeverityHigh},
					{ID: "i2", Type: IssueTypeMissingPattern, Severity: SeverityMedium},
					{
						ID:   "i3",
						Type: IssueTypeInconsistent,
						Fix:  &Fix{ID: "f1", Applied: false},
					},
				},
				CategorySummary: map[string]CategoryStat{
					"cat1": {CategoryName: "Groceries"},
					"cat2": {CategoryName: "Entertainment"},
				},
				Insights: []string{
					"Insight 1",
					"Insight 2",
				},
				SuggestedPatterns: []SuggestedPattern{
					{ID: "p1", Name: "Pattern 1"},
				},
			},
			contains: []string{
				"Analysis Report - Interactive View",
				"Use arrow keys to navigate",
				"Coherence: 78%",
				"Issues: 3",
				"Patterns: 1",
				"Categories: 2",
				"Menu Options:",
				"[1] View Issues by Severity (3)",
				"[2] View Category Analysis (2)",
				"[3] View Suggested Patterns (1)",
				"[4] View Insights (2)",
				"[5] Apply Fixes (1)",
				"[6] Export Report",
			},
		},
		{
			name: "no actionable issues",
			report: &Report{
				ID:             "test-report-2",
				SessionID:      "session-2",
				GeneratedAt:    now,
				PeriodStart:    now.AddDate(0, -1, 0),
				PeriodEnd:      now,
				CoherenceScore: 0.95,
				Issues: []Issue{
					{ID: "i1", Type: IssueTypeMiscategorized, Severity: SeverityLow},
				},
			},
			contains: []string{
				"[5] Apply Fixes",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatter.FormatInteractive(tt.report)

			for _, expected := range tt.contains {
				assert.Contains(t, result, expected, "Expected to find: %s", expected)
			}
		})
	}
}

func TestFormatHelpers(t *testing.T) {
	formatter := NewCLIFormatter()

	t.Run("progress bar rendering", func(t *testing.T) {
		tests := []struct {
			expected string
			score    float64
		}{
			{strings.Repeat("░", 30), 0.0},
			{strings.Repeat("█", 15) + strings.Repeat("░", 15), 0.5},
			{strings.Repeat("█", 30), 1.0},
			{strings.Repeat("█", 9) + strings.Repeat("░", 21), 0.333},
		}

		for _, tt := range tests {
			// Extract the progress bar from coherence score display
			result := formatter.formatCoherenceScore(tt.score)
			lines := strings.Split(result, "\n")
			require.Len(t, lines, 2, "Expected score and progress bar")

			// The second line should be the progress bar
			bar := stripANSI(lines[1])
			assert.Equal(t, tt.expected, bar, "Score %.2f", tt.score)
		}
	})

	t.Run("severity icons", func(t *testing.T) {
		tests := []struct {
			severity IssueSeverity
			icon     string
		}{
			{SeverityCritical, "🚨"},
			{SeverityHigh, "⚠️"},
			{SeverityMedium, "⚡"},
			{SeverityLow, "💡"},
		}

		for _, tt := range tests {
			icon := formatter.getSeverityIcon(tt.severity)
			assert.Equal(t, tt.icon, icon)
		}
	})

	t.Run("category table limits", func(t *testing.T) {
		// Create many categories
		categories := make(map[string]CategoryStat)
		for i := 0; i < 15; i++ {
			categories[string(rune('a'+i))] = CategoryStat{
				CategoryID:       string(rune('a' + i)),
				CategoryName:     strings.ToUpper(string(rune('a' + i))),
				TransactionCount: 15 - i, // Descending order
				TotalAmount:      float64((15 - i) * 100),
				Consistency:      0.8,
			}
		}

		result := formatter.formatCategorySummary(categories)
		assert.Contains(t, result, "... and 5 more categories")

		// Should show top 10
		for i := 0; i < 10; i++ {
			assert.Contains(t, result, strings.ToUpper(string(rune('a'+i))))
		}
		// Should not show 11th and beyond
		assert.NotContains(t, result, strings.ToUpper(string(rune('a'+10))))
	})
}

func TestStyles(t *testing.T) {
	t.Run("NewStyles creates valid styles", func(t *testing.T) {
		styles := NewStyles()
		require.NotNil(t, styles)

		// Test that all styles are initialized
		assert.NotNil(t, styles.Title)
		assert.NotNil(t, styles.Success)
		assert.NotNil(t, styles.Error)
		assert.NotNil(t, styles.Box)
		assert.NotNil(t, styles.Critical)
	})

	t.Run("ForSeverity returns correct styles", func(t *testing.T) {
		styles := NewStyles()

		tests := []struct {
			severity IssueSeverity
			notNil   bool
		}{
			{SeverityCritical, true},
			{SeverityHigh, true},
			{SeverityMedium, true},
			{SeverityLow, true},
			{IssueSeverity("unknown"), true}, // Should return Normal
		}

		for _, tt := range tests {
			style := styles.ForSeverity(tt.severity)
			assert.NotNil(t, style)
		}
	})

	t.Run("RenderProgressBar", func(t *testing.T) {
		styles := NewStyles()

		tests := []struct {
			progress float64
			width    int
			filled   int
		}{
			{0.0, 10, 0},
			{0.5, 10, 5},
			{1.0, 10, 10},
			{0.33, 10, 3},
			{1.5, 10, 10}, // Clamped to max
			{-0.5, 10, 0}, // Clamped to min
		}

		for _, tt := range tests {
			bar := styles.RenderProgressBar(tt.progress, tt.width)
			filled := strings.Count(bar, "█")
			assert.Equal(t, tt.filled, filled, "Progress %.2f", tt.progress)
			// Count runes, not bytes, since Unicode characters take multiple bytes
			runeCount := len([]rune(bar))
			assert.Equal(t, tt.width, runeCount, "Progress %.2f rune count", tt.progress)
		}
	})

	t.Run("WithWidth adjusts box widths", func(t *testing.T) {
		styles := NewStyles()
		narrow := styles.WithWidth(80)

		assert.NotEqual(t, styles, narrow)
		// Boxes should be adjusted for narrow terminal
		assert.NotEqual(t, styles.Box, narrow.Box)
	})
}

// Helper functions

// stripANSI removes ANSI color codes for testing.
func stripANSI(s string) string {
	// Simple ANSI stripping for tests
	for strings.Contains(s, "\x1b[") {
		start := strings.Index(s, "\x1b[")
		end := strings.Index(s[start:], "m")
		if end == -1 {
			break
		}
		s = s[:start] + s[start+end+1:]
	}
	return s
}
