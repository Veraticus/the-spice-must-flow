package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCLIPrompter_ConfirmClassification(t *testing.T) {
	// Create default rankings for tests that need them
	defaultRankings := model.CategoryRankings{
		{Category: "Food & Dining", Score: 0.5},
		{Category: "Shopping", Score: 0.3},
		{Category: "Office Supplies", Score: 0.1},
		{Category: "Other", Score: 0.05},
	}

	tests := []struct {
		name             string
		input            string
		expectedStatus   model.ClassificationStatus
		expectedCategory string
		pending          model.PendingClassification
		expectError      bool
		contextCancelled bool
	}{
		{
			name: "accept AI suggestion",
			pending: model.PendingClassification{
				Transaction: model.Transaction{
					ID:           "tx1",
					Name:         "STARBUCKS #12345",
					MerchantName: "Starbucks",
					Amount:       5.75,
					Date:         time.Now(),
				},
				SuggestedCategory: "Food & Dining",
				Confidence:        0.95,
				SimilarCount:      10,
			},
			input:            "a\n",
			expectedStatus:   model.StatusClassifiedByAI,
			expectedCategory: "Food & Dining",
		},
		{
			name: "custom category",
			pending: model.PendingClassification{
				Transaction: model.Transaction{
					ID:           "tx2",
					Name:         "AMAZON.COM",
					MerchantName: "Amazon",
					Amount:       49.99,
					Date:         time.Now(),
				},
				SuggestedCategory: "Shopping",
				Confidence:        0.75,
				CategoryRankings:  defaultRankings,
			},
			input:            "c\n3\n", // Select Office Supplies by number
			expectedStatus:   model.StatusUserModified,
			expectedCategory: "Office Supplies",
		},
		{
			name: "skip transaction",
			pending: model.PendingClassification{
				Transaction: model.Transaction{
					ID:           "tx3",
					Name:         "UNKNOWN VENDOR",
					MerchantName: "Unknown",
					Amount:       20.00,
					Date:         time.Now(),
				},
				SuggestedCategory: "Other",
				Confidence:        0.30,
			},
			input:          "s\n",
			expectedStatus: model.StatusUnclassified,
		},
		{
			name: "invalid choice then valid",
			pending: model.PendingClassification{
				Transaction: model.Transaction{
					ID:           "tx4",
					Name:         "GROCERY STORE",
					MerchantName: "Grocery Store",
					Amount:       125.50,
					Date:         time.Now(),
				},
				SuggestedCategory: "Groceries",
				Confidence:        0.90,
			},
			input:            "x\na\n",
			expectedStatus:   model.StatusClassifiedByAI,
			expectedCategory: "Groceries",
		},
		{
			name: "context canceled",
			pending: model.PendingClassification{
				Transaction: model.Transaction{
					ID:           "tx5",
					Name:         "TEST",
					MerchantName: "Test",
					Amount:       10.00,
					Date:         time.Now(),
				},
			},
			contextCancelled: true,
			expectError:      true,
		},
		{
			name: "empty custom category then valid",
			pending: model.PendingClassification{
				Transaction: model.Transaction{
					ID:           "tx6",
					Name:         "RESTAURANT",
					MerchantName: "Restaurant",
					Amount:       35.00,
					Date:         time.Now(),
				},
				SuggestedCategory: "Food & Dining",
				Confidence:        0.85,
				CategoryRankings:  defaultRankings,
			},
			input:            "c\nn\n\nRestaurants\n", // Create new category with empty name then valid
			expectedStatus:   model.StatusUserModified,
			expectedCategory: "Restaurants",
		},
		{
			name: "accept new category suggestion",
			pending: model.PendingClassification{
				Transaction: model.Transaction{
					ID:           "tx7",
					Name:         "PELOTON SUBSCRIPTION",
					MerchantName: "Peloton",
					Amount:       39.99,
					Date:         time.Now(),
				},
				SuggestedCategory: "Fitness & Health",
				Confidence:        0.75,
				IsNewCategory:     true,
			},
			input:            "a\n",
			expectedStatus:   model.StatusClassifiedByAI,
			expectedCategory: "Fitness & Health",
		},
		{
			name: "use existing category instead of new",
			pending: model.PendingClassification{
				Transaction: model.Transaction{
					ID:           "tx8",
					Name:         "24 HOUR FITNESS",
					MerchantName: "24 Hour Fitness",
					Amount:       50.00,
					Date:         time.Now(),
				},
				SuggestedCategory: "Fitness & Health",
				Confidence:        0.70,
				IsNewCategory:     true,
				CategoryRankings: model.CategoryRankings{
					{Category: "Entertainment", Score: 0.3},
					{Category: "Shopping", Score: 0.2},
					{Category: "Other", Score: 0.1},
				},
			},
			input:            "e\n1\n", // Select Entertainment by number
			expectedStatus:   model.StatusUserModified,
			expectedCategory: "Entertainment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			var output bytes.Buffer
			prompter := NewCLIPrompter(reader, &output)
			prompter.SetTotalTransactions(10)

			ctx := context.Background()
			if tt.contextCancelled {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			classification, err := prompter.ConfirmClassification(ctx, tt.pending)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, classification.Status)
			if tt.expectedCategory != "" {
				assert.Equal(t, tt.expectedCategory, classification.Category)
			}
			assert.Equal(t, tt.pending.Transaction.ID, classification.Transaction.ID)
			assert.WithinDuration(t, time.Now(), classification.ClassifiedAt, 5*time.Second)

			outputStr := output.String()
			assert.Contains(t, outputStr, tt.pending.Transaction.MerchantName)
			assert.Contains(t, outputStr, fmt.Sprintf("$%.2f", tt.pending.Transaction.Amount))
		})
	}
}

func TestCLIPrompter_BatchConfirmClassifications(t *testing.T) {
	// Default rankings for batch tests
	defaultBatchRankings := model.CategoryRankings{
		{Category: "Food & Dining", Score: 0.5},
		{Category: "Shopping", Score: 0.3},
		{Category: "Travel", Score: 0.1},
		{Category: "Other", Score: 0.05},
	}

	createPendingBatch := func(count int, merchant, category string) []model.PendingClassification {
		pending := make([]model.PendingClassification, count)
		for i := 0; i < count; i++ {
			pending[i] = model.PendingClassification{
				Transaction: model.Transaction{
					ID:           fmt.Sprintf("tx%d", i),
					Name:         fmt.Sprintf("%s PURCHASE", merchant),
					MerchantName: merchant,
					Amount:       float64(10 + i*5),
					Date:         time.Now().AddDate(0, 0, -i),
				},
				SuggestedCategory: category,
				Confidence:        0.90,
				CategoryRankings:  defaultBatchRankings,
			}
		}
		return pending
	}

	tests := []struct {
		name             string
		input            string
		expectedCategory string
		pending          []model.PendingClassification
		expectedStatuses []model.ClassificationStatus
		expectError      bool
	}{
		{
			name:             "accept all",
			pending:          createPendingBatch(5, "Starbucks", "Food & Dining"),
			input:            "a\n",
			expectedStatuses: repeatStatus(model.StatusClassifiedByAI, 5),
			expectedCategory: "Food & Dining",
		},
		{
			name:             "custom category for all",
			pending:          createPendingBatch(3, "Amazon", "Shopping"),
			input:            "c\nn\nOffice Supplies\n", // Create new category "Office Supplies"
			expectedStatuses: repeatStatus(model.StatusUserModified, 3),
			expectedCategory: "Office Supplies",
		},
		{
			name:             "skip all",
			pending:          createPendingBatch(4, "Unknown", "Other"),
			input:            "s\n",
			expectedStatuses: repeatStatus(model.StatusUnclassified, 4),
		},
		{
			name:    "review each individually",
			pending: createPendingBatch(3, "Target", "Shopping"),
			input:   "r\na\nc\nn\nGroceries\ns\n", // Review -> Accept first, Custom+New "Groceries" for second, Skip third
			expectedStatuses: []model.ClassificationStatus{
				model.StatusClassifiedByAI,
				model.StatusUserModified,
				model.StatusUnclassified,
			},
		},
		{
			name:             "empty pending list",
			pending:          []model.PendingClassification{},
			expectedStatuses: []model.ClassificationStatus{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			var output bytes.Buffer
			prompter := NewCLIPrompter(reader, &output)
			prompter.SetTotalTransactions(20)

			classifications, err := prompter.BatchConfirmClassifications(context.Background(), tt.pending)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Len(t, classifications, len(tt.pending))

			for i, classification := range classifications {
				assert.Equal(t, tt.expectedStatuses[i], classification.Status)
				assert.Equal(t, tt.pending[i].Transaction.ID, classification.Transaction.ID)

				if tt.expectedCategory != "" && classification.Status != model.StatusUnclassified {
					assert.Equal(t, tt.expectedCategory, classification.Category)
				}
			}

			if len(tt.pending) > 0 {
				outputStr := output.String()
				assert.Contains(t, outputStr, tt.pending[0].Transaction.MerchantName)
				assert.Contains(t, outputStr, fmt.Sprintf("%d", len(tt.pending)))
			}
		})
	}
}

func TestCLIPrompter_PatternDetection(t *testing.T) {
	reader := strings.NewReader("a\na\na\na\n")
	var output bytes.Buffer
	prompter := NewCLIPrompter(reader, &output)
	prompter.SetTotalTransactions(10)

	merchant := "Starbucks"
	category := "Coffee Shops"

	for i := 0; i < 4; i++ {
		pending := []model.PendingClassification{{
			Transaction: model.Transaction{
				ID:           fmt.Sprintf("tx%d", i),
				MerchantName: merchant,
				Amount:       5.50,
				Date:         time.Now(),
			},
			SuggestedCategory: category,
			Confidence:        0.95,
		}}

		classifications, err := prompter.BatchConfirmClassifications(context.Background(), pending)
		require.NoError(t, err)
		assert.Len(t, classifications, 1)
	}

	outputStr := output.String()
	assert.Contains(t, outputStr, "Last 3 were categorized as Coffee Shops")
}

func TestCLIPrompter_CompletionStats(t *testing.T) {
	// Default rankings for stats test
	statsRankings := model.CategoryRankings{
		{Category: "Shopping", Score: 0.5},
		{Category: "Food", Score: 0.3},
		{Category: "Other", Score: 0.1},
	}

	reader := strings.NewReader("a\nc\nn\nFood\ns\n") // Accept, Custom+New "Food", Skip
	var output bytes.Buffer
	prompter := NewCLIPrompter(reader, &output)
	prompter.SetTotalTransactions(3)

	testCases := []struct {
		expectedChoice string
		pending        model.PendingClassification
	}{
		{
			pending: model.PendingClassification{
				Transaction:       createTestTransaction("tx1"),
				SuggestedCategory: "Shopping",
				CategoryRankings:  statsRankings,
			},
			expectedChoice: "a",
		},
		{
			pending: model.PendingClassification{
				Transaction:       createTestTransaction("tx2"),
				SuggestedCategory: "Other",
				CategoryRankings:  statsRankings,
			},
			expectedChoice: "c",
		},
		{
			pending: model.PendingClassification{
				Transaction:       createTestTransaction("tx3"),
				SuggestedCategory: "Unknown",
				CategoryRankings:  statsRankings,
			},
			expectedChoice: "s",
		},
	}

	for _, tc := range testCases {
		_, err := prompter.ConfirmClassification(context.Background(), tc.pending)
		require.NoError(t, err)
	}

	stats := prompter.GetCompletionStats()
	assert.Equal(t, 2, stats.TotalTransactions)
	assert.Equal(t, 1, stats.AutoClassified)
	assert.Equal(t, 1, stats.UserClassified)

	prompter.ShowCompletion()
	completionOutput := output.String()

	// Find the JSON output (it should be the last line)
	lines := strings.Split(strings.TrimSpace(completionOutput), "\n")
	jsonLine := lines[len(lines)-1]

	// Parse and validate JSON
	var result map[string]any
	err := json.Unmarshal([]byte(jsonLine), &result)
	assert.NoError(t, err)
	assert.Equal(t, float64(2), result["total_transactions"])
	assert.Equal(t, float64(1), result["auto_classified"])
	assert.Equal(t, float64(1), result["user_classified"])
}

func TestCLIPrompter_ContextCancellation(t *testing.T) {
	reader := strings.NewReader("")
	var output bytes.Buffer
	prompter := NewCLIPrompter(reader, &output)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	pending := model.PendingClassification{
		Transaction:       createTestTransaction("tx1"),
		SuggestedCategory: "Test",
	}

	_, err := prompter.ConfirmClassification(ctx, pending)
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)

	_, err = prompter.BatchConfirmClassifications(ctx, []model.PendingClassification{pending})
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestCLIPrompter_RecentCategories(t *testing.T) {
	// Default rankings for recent categories test
	recentRankings := model.CategoryRankings{
		{Category: "Food & Dining", Score: 0.5},
		{Category: "Shopping", Score: 0.3},
		{Category: "Transportation", Score: 0.2},
		{Category: "Other", Score: 0.1},
	}

	inputs := []string{
		"c\n1\n", // Select Food & Dining by number
		"c\n2\n", // Select Shopping by number
		"c\n3\n", // Select Transportation by number
		"c\n\n",  // Empty input for error test
	}

	reader := strings.NewReader(strings.Join(inputs, ""))
	var output bytes.Buffer
	prompter := NewCLIPrompter(reader, &output)
	prompter.SetTotalTransactions(4)

	for i := 0; i < 3; i++ {
		pending := model.PendingClassification{
			Transaction:       createTestTransaction(fmt.Sprintf("tx%d", i)),
			SuggestedCategory: "Other",
			CategoryRankings:  recentRankings,
		}
		_, err := prompter.ConfirmClassification(context.Background(), pending)
		require.NoError(t, err)
	}

	pending := model.PendingClassification{
		Transaction:       createTestTransaction("tx4"),
		SuggestedCategory: "Other",
		CategoryRankings:  recentRankings,
	}
	_, err := prompter.ConfirmClassification(context.Background(), pending)
	assert.Error(t, err)

	outputStr := output.String()
	// The new UI shows categories in ranked order, not "Recent categories"
	assert.Contains(t, outputStr, "Select category (ranked by likelihood)")
	assert.Contains(t, outputStr, "Transportation")
	assert.Contains(t, outputStr, "Shopping")
	assert.Contains(t, outputStr, "Food & Dining")
}

func TestCLIPrompter_ProgressBar(t *testing.T) {
	reader := strings.NewReader("a\na\n")
	var output bytes.Buffer
	prompter := NewCLIPrompter(reader, &output)
	prompter.SetTotalTransactions(2)

	for i := 0; i < 2; i++ {
		pending := model.PendingClassification{
			Transaction:       createTestTransaction(fmt.Sprintf("tx%d", i)),
			SuggestedCategory: "Test",
		}
		_, err := prompter.ConfirmClassification(context.Background(), pending)
		require.NoError(t, err)
	}

	outputStr := output.String()
	assert.Contains(t, outputStr, "Classifying transactions")
	assert.Contains(t, outputStr, "(2/2)")
}

func TestCLIPrompter_TimeSavingsCalculation(t *testing.T) {
	tests := []struct {
		name           string
		expectedOutput string
		stats          service.CompletionStats
	}{
		{
			name: "seconds",
			stats: service.CompletionStats{
				TotalTransactions: 10,
				AutoClassified:    8,
			},
			expectedOutput: "40 seconds",
		},
		{
			name: "minutes",
			stats: service.CompletionStats{
				TotalTransactions: 50,
				AutoClassified:    40,
			},
			expectedOutput: "3.3 minutes",
		},
		{
			name: "hours",
			stats: service.CompletionStats{
				TotalTransactions: 1000,
				AutoClassified:    900,
			},
			expectedOutput: "1.2 hours",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompter := &Prompter{}
			timeSaved := prompter.calculateTimeSaved(tt.stats)
			assert.Equal(t, tt.expectedOutput, timeSaved)
		})
	}
}

func TestCLIPrompter_InputValidation(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
	}{
		{
			name:        "EOF during choice",
			input:       "",
			expectError: true,
		},
		{
			name:        "multiple invalid choices",
			input:       "x\ny\nz\n",
			expectError: true,
		},
		{
			name:        "valid after multiple invalid",
			input:       "x\ny\na\n",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			var output bytes.Buffer
			prompter := NewCLIPrompter(reader, &output)

			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			choice, err := prompter.promptChoice(ctx, "Test", []string{"a", "b", "c"})

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, "a", choice)
			}
		})
	}
}

func createTestTransaction(id string) model.Transaction {
	return model.Transaction{
		ID:           id,
		Name:         "TEST TRANSACTION",
		MerchantName: "Test Merchant",
		Amount:       100.00,
		Date:         time.Now(),
		AccountID:    "acc123",
	}
}

func repeatStatus(status model.ClassificationStatus, count int) []model.ClassificationStatus {
	statuses := make([]model.ClassificationStatus, count)
	for i := 0; i < count; i++ {
		statuses[i] = status
	}
	return statuses
}

func TestCLIPrompter_promptCategorySelection(t *testing.T) {
	// Create test rankings
	createTestRankings := func() model.CategoryRankings {
		return model.CategoryRankings{
			{Category: "Food & Dining", Score: 0.72, Description: "Restaurants, groceries, and food delivery"},
			{Category: "Transportation", Score: 0.18, Description: "Gas, parking, public transit, rideshare"},
			{Category: "Shopping", Score: 0.08, Description: "Clothing, electronics, general retail"},
			{Category: "Entertainment", Score: 0.05, Description: "Movies, games, streaming services"},
			{Category: "Home Services", Score: 0.03, Description: "Cleaning, repairs, maintenance"},
			{Category: "Healthcare", Score: 0.02, Description: "Medical, dental, pharmacy"},
			{Category: "Personal Care", Score: 0.01, Description: "Hair, beauty, spa services"},
			{Category: "Other Expenses", Score: 0.00, Description: ""},
		}
	}

	// Create test check patterns
	createTestCheckPatterns := func() []model.CheckPattern {
		return []model.CheckPattern{
			{
				ID:              1,
				PatternName:     "Monthly cleaning",
				Category:        "Home Services",
				ConfidenceBoost: 0.3,
			},
		}
	}

	tests := []struct {
		name             string
		input            string
		expectedCategory string
		rankings         model.CategoryRankings
		checkPatterns    []model.CheckPattern
		expectError      bool
		contextCancelled bool
	}{
		{
			name:             "select by number",
			input:            "1\n",
			rankings:         createTestRankings(),
			expectedCategory: "Food & Dining",
		},
		{
			name:             "select by category name (exact match)",
			input:            "Transportation\n",
			rankings:         createTestRankings(),
			expectedCategory: "Transportation",
		},
		{
			name:             "select by category name (case insensitive)",
			input:            "shopping\n",
			rankings:         createTestRankings(),
			expectedCategory: "Shopping",
		},
		{
			name:             "create new category",
			input:            "n\nBusiness Expenses\n",
			rankings:         createTestRankings(),
			expectedCategory: "Business Expenses",
		},
		{
			name:             "invalid selection then valid number",
			input:            "xyz\n2\n",
			rankings:         createTestRankings(),
			expectedCategory: "Transportation",
		},
		{
			name:             "empty input then valid selection",
			input:            "\n3\n",
			rankings:         createTestRankings(),
			expectedCategory: "Shopping",
		},
		{
			name:             "select category with check pattern match",
			input:            "5\n",
			rankings:         createTestRankings(),
			checkPatterns:    createTestCheckPatterns(),
			expectedCategory: "Home Services",
		},
		{
			name:             "create new category with empty name then valid",
			input:            "n\n\nTravel\n",
			rankings:         createTestRankings(),
			expectedCategory: "Travel",
		},
		{
			name:             "context canceled",
			rankings:         createTestRankings(),
			contextCancelled: true,
			expectError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			var output bytes.Buffer
			prompter := NewCLIPrompter(reader, &output)

			ctx := context.Background()
			if tt.contextCancelled {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			category, err := prompter.promptCategorySelection(ctx, tt.rankings, tt.checkPatterns)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedCategory, category)
			}

			// Verify output contains expected elements
			outputStr := output.String()
			if !tt.expectError {
				assert.Contains(t, outputStr, "Select category (ranked by likelihood)")

				// Check that categories are displayed with scores
				if tt.rankings[0].Score > 0.01 {
					assert.Contains(t, outputStr, fmt.Sprintf("%s (%.0f%% match)",
						tt.rankings[0].Category, tt.rankings[0].Score*100))
				}

				// Check for check pattern indicator
				if len(tt.checkPatterns) > 0 {
					assert.Contains(t, outputStr, "matches pattern")
				}

				// Check for descriptions
				for _, ranking := range tt.rankings {
					if ranking.Description != "" && ranking.Score > 0.01 {
						assert.Contains(t, outputStr, ranking.Description)
					}
				}

				// Check for new category option
				assert.Contains(t, outputStr, "[N] Create new category")
			}
		})
	}
}

func TestCLIPrompter_promptCategorySelection_ManyCategories(t *testing.T) {
	// Create rankings with more than 15 categories
	var rankings model.CategoryRankings
	for i := 0; i < 20; i++ {
		rankings = append(rankings, model.CategoryRanking{
			Category: fmt.Sprintf("Category %d", i+1),
			Score:    float64(20-i) / 100,
		})
	}

	tests := []struct {
		name             string
		input            string
		expectedCategory string
	}{
		{
			name:             "show more categories",
			input:            "m\n16\n",
			expectedCategory: "Category 16",
		},
		{
			name:             "select category after show more",
			input:            "m\n20\n",
			expectedCategory: "Category 20",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			var output bytes.Buffer
			prompter := NewCLIPrompter(reader, &output)

			category, err := prompter.promptCategorySelection(context.Background(), rankings, nil)

			require.NoError(t, err)
			assert.Equal(t, tt.expectedCategory, category)

			// Verify "Show more" option appears
			outputStr := output.String()
			assert.Contains(t, outputStr, "[M] Show 5 more categories")
		})
	}
}

func TestCLIPrompter_ConfirmClassification_WithRankings(t *testing.T) {
	// Test the enhanced UI with category rankings
	rankings := model.CategoryRankings{
		{Category: "Shopping", Score: 0.75, Description: "Clothing, electronics, general retail"},
		{Category: "Office Supplies", Score: 0.15, Description: "Business and office materials"},
		{Category: "Home & Garden", Score: 0.08, Description: "Home improvement and gardening"},
		{Category: "Other Expenses", Score: 0.02, Description: ""},
	}

	checkPatterns := []model.CheckPattern{
		{
			ID:              1,
			PatternName:     "Office supplies check",
			Category:        "Office Supplies",
			ConfidenceBoost: 0.3,
		},
	}

	tests := []struct {
		name             string
		input            string
		expectedCategory string
		expectedStatus   model.ClassificationStatus
		pending          model.PendingClassification
	}{
		{
			name: "select existing category by number",
			pending: model.PendingClassification{
				Transaction: model.Transaction{
					ID:           "tx1",
					Name:         "STAPLES #123",
					MerchantName: "Staples",
					Amount:       89.99,
					Date:         time.Now(),
				},
				SuggestedCategory: "Shopping",
				Confidence:        0.75,
				CategoryRankings:  rankings,
			},
			input:            "c\n2\n", // Choose custom category, then select #2 (Office Supplies)
			expectedCategory: "Office Supplies",
			expectedStatus:   model.StatusUserModified,
		},
		{
			name: "select existing category by name",
			pending: model.PendingClassification{
				Transaction: model.Transaction{
					ID:           "tx2",
					Name:         "HOME DEPOT",
					MerchantName: "Home Depot",
					Amount:       234.56,
					Date:         time.Now(),
				},
				SuggestedCategory: "Shopping",
				Confidence:        0.75,
				CategoryRankings:  rankings,
			},
			input:            "c\nhome & garden\n", // Case insensitive name selection
			expectedCategory: "Home & Garden",
			expectedStatus:   model.StatusUserModified,
		},
		{
			name: "create new category via enhanced UI",
			pending: model.PendingClassification{
				Transaction: model.Transaction{
					ID:           "tx3",
					Name:         "CHECK #1234",
					MerchantName: "Check",
					Amount:       100.00,
					Date:         time.Now(),
					Type:         "CHECK",
				},
				SuggestedCategory: "Office Supplies",
				Confidence:        0.45,
				CategoryRankings:  rankings,
				CheckPatterns:     checkPatterns,
			},
			input:            "c\nn\nBusiness Services\n", // Custom category -> New -> Enter name
			expectedCategory: "Business Services",
			expectedStatus:   model.StatusUserModified,
		},
		{
			name: "new category suggestion - use existing instead",
			pending: model.PendingClassification{
				Transaction: model.Transaction{
					ID:           "tx4",
					Name:         "SPECIALTY STORE",
					MerchantName: "Specialty Store",
					Amount:       50.00,
					Date:         time.Now(),
				},
				SuggestedCategory:   "Specialty Shopping",
				CategoryDescription: "A new category for specialty items",
				Confidence:          0.65,
				IsNewCategory:       true,
				CategoryRankings:    rankings,
			},
			input:            "e\n1\n", // Use existing category -> Select #1 (Shopping)
			expectedCategory: "Shopping",
			expectedStatus:   model.StatusUserModified,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			var writer bytes.Buffer
			prompter := NewCLIPrompter(reader, &writer)

			classification, err := prompter.ConfirmClassification(context.Background(), tt.pending)

			require.NoError(t, err)
			assert.Equal(t, tt.expectedCategory, classification.Category)
			assert.Equal(t, tt.expectedStatus, classification.Status)

			// Verify enhanced UI elements appear in output
			output := writer.String()
			if strings.Contains(tt.input, "c\n") || strings.Contains(tt.input, "e\n") {
				// Should show ranked categories when custom category is selected
				assert.Contains(t, output, "Select category (ranked by likelihood)")
				assert.Contains(t, output, "(75% match)")                           // Shopping score
				assert.Contains(t, output, "Clothing, electronics, general retail") // Description
				assert.Contains(t, output, "[N] Create new category")
			}

			// Check for check pattern indicator
			if len(tt.pending.CheckPatterns) > 0 {
				assert.Contains(t, output, "matches pattern")
			}
		})
	}
}
