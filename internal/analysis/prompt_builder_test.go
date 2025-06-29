package analysis

import (
	"strings"
	"testing"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTemplatePromptBuilder(t *testing.T) {
	pb, err := NewTemplatePromptBuilder()
	require.NoError(t, err)
	assert.NotNil(t, pb)
	assert.Len(t, pb.templates, 3)
	assert.Contains(t, pb.templates, "analysis_prompt")
	assert.Contains(t, pb.templates, "json_schema")
	assert.Contains(t, pb.templates, "correction_prompt")
}

func TestBuildAnalysisPrompt(t *testing.T) {
	pb, err := NewTemplatePromptBuilder()
	require.NoError(t, err)

	now := time.Now()
	startDate := now.AddDate(0, -1, 0)
	endDate := now

	tests := []struct {
		checkOutput func(t *testing.T, output string)
		name        string
		data        PromptData
		wantErr     bool
	}{
		{
			name: "complete prompt with all data",
			data: PromptData{
				Transactions: []model.Transaction{
					{
						ID:       "txn_123",
						Date:     now.AddDate(0, 0, -7),
						Name:     "STARBUCKS STORE #12345",
						Amount:   5.75,
						Type:     "DEBIT",
						Category: []string{"Dining Out"},
					},
					{
						ID:       "txn_456",
						Date:     now.AddDate(0, 0, -5),
						Name:     "WHOLE FOODS MARKET",
						Amount:   125.43,
						Type:     "DEBIT",
						Category: []string{"Shopping"},
					},
				},
				Categories: []model.Category{
					{
						Name:        "Groceries",
						Type:        model.CategoryTypeExpense,
						Description: "Food and household supplies",
					},
					{
						Name:        "Dining Out",
						Type:        model.CategoryTypeExpense,
						Description: "Restaurants and takeout",
					},
					{
						Name:        "Shopping",
						Type:        model.CategoryTypeExpense,
						Description: "General shopping",
					},
				},
				Patterns: []model.PatternRule{
					{
						MerchantPattern: "STARBUCKS",
						DefaultCategory: "Coffee Shops",
						Priority:        10,
					},
				},
				CheckPatterns: []model.CheckPattern{
					{
						PatternName: "Check #1234",
						AmountMin:   floatPtr(100.0),
						AmountMax:   floatPtr(100.0),
						Category:    "Rent",
					},
				},
				RecentVendors: []RecentVendor{
					{
						Name:        "AMAZON",
						Category:    "Shopping",
						Occurrences: 15,
					},
				},
				DateRange: DateRange{
					Start: startDate,
					End:   endDate,
				},
				TotalCount: 100,
				SampleSize: 2,
				AnalysisOptions: Options{
					Focus: FocusPatterns,
				},
			},
			wantErr: false,
			checkOutput: func(t *testing.T, output string) {
				t.Helper()
				// Check that key elements are present
				assert.Contains(t, output, "You are an expert financial analyst")
				assert.Contains(t, output, "100 transactions")
				assert.Contains(t, output, "sample of 2 transactions")
				assert.Contains(t, output, "STARBUCKS STORE #12345")
				assert.Contains(t, output, "WHOLE FOODS MARKET")
				assert.Contains(t, output, "Groceries")
				assert.Contains(t, output, "Pattern: STARBUCKS")
				assert.Contains(t, output, "Check #1234")
				assert.Contains(t, output, "AMAZON → Shopping")
				assert.Contains(t, output, "Pattern Suggestions")
				assert.Contains(t, output, "coherence_score")
			},
		},
		{
			name: "minimal prompt without optional data",
			data: PromptData{
				Transactions: []model.Transaction{
					{
						ID:       "txn_789",
						Date:     now.AddDate(0, 0, -3),
						Name:     "PAYMENT RECEIVED",
						Amount:   1000.00,
						Type:     "CREDIT",
						Category: []string{"Income"},
					},
				},
				Categories: []model.Category{
					{
						Name:        "Income",
						Type:        model.CategoryTypeIncome,
						Description: "All income sources",
					},
				},
				DateRange: DateRange{
					Start: startDate,
					End:   endDate,
				},
				TotalCount:      1,
				SampleSize:      1,
				AnalysisOptions: Options{},
			},
			wantErr: false,
			checkOutput: func(t *testing.T, output string) {
				t.Helper()
				assert.Contains(t, output, "1 transactions")
				assert.NotContains(t, output, "sample of")
				assert.Contains(t, output, "No pattern rules are currently configured")
				assert.NotContains(t, output, "Check #")
				assert.NotContains(t, output, "Recent Vendor")
			},
		},
		{
			name: "focus on categories only",
			data: PromptData{
				Transactions: []model.Transaction{
					{
						ID:       "txn_001",
						Date:     now,
						Name:     "Test transaction",
						Amount:   10.00,
						Type:     "DEBIT",
						Category: []string{"Test"},
					},
				},
				Categories: []model.Category{
					{
						Name: "Test",
						Type: model.CategoryTypeExpense,
					},
				},
				DateRange: DateRange{
					Start: startDate,
					End:   endDate,
				},
				TotalCount: 1,
				SampleSize: 1,
				AnalysisOptions: Options{
					Focus: FocusCategories,
				},
			},
			wantErr: false,
			checkOutput: func(t *testing.T, output string) {
				t.Helper()
				assert.Contains(t, output, "Category Analysis")
				assert.NotContains(t, output, "Pattern Suggestions")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := pb.BuildAnalysisPrompt(tt.data)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				tt.checkOutput(t, output)
			}
		})
	}
}

func TestBuildCorrectionPrompt(t *testing.T) {
	pb, err := NewTemplatePromptBuilder()
	require.NoError(t, err)

	tests := []struct {
		checkOutput func(t *testing.T, output string)
		name        string
		data        CorrectionPromptData
		wantErr     bool
	}{
		{
			name: "complete correction data",
			data: CorrectionPromptData{
				OriginalPrompt:  "Analyze these transactions...",
				InvalidResponse: `{"coherence_score": 85, "issues": [}`,
				ErrorDetails:    "unexpected end of JSON input",
				ErrorSection:    `"issues": [}`,
				LineNumber:      2,
				ColumnNumber:    15,
			},
			wantErr: false,
			checkOutput: func(t *testing.T, output string) {
				t.Helper()
				assert.Contains(t, output, "invalid JSON")
				assert.Contains(t, output, "Analyze these transactions...")
				assert.Contains(t, output, `"issues": [}`)
				assert.Contains(t, output, "line 2, column 15")
				assert.Contains(t, output, "unexpected end of JSON input")
			},
		},
		{
			name: "minimal correction data",
			data: CorrectionPromptData{
				OriginalPrompt:  "Original prompt",
				InvalidResponse: "Not valid JSON",
				ErrorDetails:    "invalid character 'N'",
			},
			wantErr: false,
			checkOutput: func(t *testing.T, output string) {
				t.Helper()
				assert.Contains(t, output, "Original prompt")
				assert.Contains(t, output, "Not valid JSON")
				assert.Contains(t, output, "invalid character 'N'")
				assert.NotContains(t, output, "line")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := pb.BuildCorrectionPrompt(tt.data)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				tt.checkOutput(t, output)
			}
		})
	}
}

func TestTemplateFunctions(t *testing.T) {
	t.Run("formatAmount", func(t *testing.T) {
		assert.Equal(t, "$5.75", formatAmount(5.75))
		assert.Equal(t, "$100.00", formatAmount(100))
		assert.Equal(t, "$0.99", formatAmount(0.99))
	})

	t.Run("formatDate", func(t *testing.T) {
		date := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
		assert.Equal(t, "2024-01-15", formatDate(date))
	})

	t.Run("truncate", func(t *testing.T) {
		assert.Equal(t, "Hello", truncate("Hello", 10))
		assert.Equal(t, "Hello...", truncate("Hello World", 8))
		assert.Equal(t, "Hel...", truncate("Hello World", 6))
		assert.Equal(t, "...", truncate("Hello", 3))
	})
}

func TestPromptOutput(t *testing.T) {
	// Test that the generated prompts are well-formed
	pb, err := NewTemplatePromptBuilder()
	require.NoError(t, err)

	data := PromptData{
		Transactions: []model.Transaction{
			{
				ID:       "test_123",
				Date:     time.Now(),
				Name:     "Test Transaction",
				Amount:   50.00,
				Type:     "DEBIT",
				Category: []string{"Test Category"},
			},
		},
		Categories: []model.Category{
			{
				Name:        "Test Category",
				Type:        model.CategoryTypeExpense,
				Description: "For testing",
			},
		},
		DateRange: DateRange{
			Start: time.Now().AddDate(0, -1, 0),
			End:   time.Now(),
		},
		TotalCount:      1,
		SampleSize:      1,
		AnalysisOptions: Options{},
	}

	prompt, err := pb.BuildAnalysisPrompt(data)
	require.NoError(t, err)

	// Check for proper structure
	assert.True(t, strings.Contains(prompt, "## Analysis Context"))
	assert.True(t, strings.Contains(prompt, "## Transactions to Analyze"))
	assert.True(t, strings.Contains(prompt, "## Your Task"))
	assert.True(t, strings.Contains(prompt, "## Response Format"))

	// Ensure JSON schema is included
	assert.True(t, strings.Contains(prompt, `"coherence_score"`))
	assert.True(t, strings.Contains(prompt, `"issues"`))
	assert.True(t, strings.Contains(prompt, `"transaction_fixes"`))
}
