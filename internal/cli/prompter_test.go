package cli

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/joshsymonds/the-spice-must-flow/internal/model"
	"github.com/joshsymonds/the-spice-must-flow/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCLIPrompter_ConfirmClassification(t *testing.T) {
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
			},
			input:            "c\nOffice Supplies\n",
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
			},
			input:            "c\n\nRestaurants\n",
			expectedStatus:   model.StatusUserModified,
			expectedCategory: "Restaurants",
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
			input:            "c\nOffice Supplies\n",
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
			input:   "r\na\nc\nGroceries\ns\n",
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
	reader := strings.NewReader("a\nc\nFood\ns\n")
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
			},
			expectedChoice: "a",
		},
		{
			pending: model.PendingClassification{
				Transaction:       createTestTransaction("tx2"),
				SuggestedCategory: "Other",
			},
			expectedChoice: "c",
		},
		{
			pending: model.PendingClassification{
				Transaction:       createTestTransaction("tx3"),
				SuggestedCategory: "Unknown",
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
	assert.Contains(t, completionOutput, "Classification Complete!")
	assert.Contains(t, completionOutput, "Time saved:")
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
	inputs := []string{
		"c\nFood & Dining\n",
		"c\nShopping\n",
		"c\nTransportation\n",
		"c\n\n",
	}

	reader := strings.NewReader(strings.Join(inputs, ""))
	var output bytes.Buffer
	prompter := NewCLIPrompter(reader, &output)
	prompter.SetTotalTransactions(4)

	for i := 0; i < 3; i++ {
		pending := model.PendingClassification{
			Transaction:       createTestTransaction(fmt.Sprintf("tx%d", i)),
			SuggestedCategory: "Other",
		}
		_, err := prompter.ConfirmClassification(context.Background(), pending)
		require.NoError(t, err)
	}

	pending := model.PendingClassification{
		Transaction:       createTestTransaction("tx4"),
		SuggestedCategory: "Other",
	}
	_, err := prompter.ConfirmClassification(context.Background(), pending)
	assert.Error(t, err)

	outputStr := output.String()
	assert.Contains(t, outputStr, "Recent categories:")
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
