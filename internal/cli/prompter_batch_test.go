package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCLIPrompter_PromptBatchClassification(t *testing.T) {
	// Create test transactions for batch
	transactions := []model.Transaction{
		{
			ID:           "batch1",
			Name:         "AUTOMATIC PAYMENT - THANK",
			MerchantName: "AUTOMATIC PAYMENT - THANK",
			Amount:       100.00,
			Date:         time.Now(),
		},
		{
			ID:           "batch2",
			Name:         "AUTOMATIC PAYMENT - THANK",
			MerchantName: "AUTOMATIC PAYMENT - THANK",
			Amount:       150.00,
			Date:         time.Now().Add(-24 * time.Hour),
		},
	}

	pendingBatch := make([]model.PendingClassification, len(transactions))
	for i, txn := range transactions {
		pendingBatch[i] = model.PendingClassification{
			Transaction:       txn,
			SuggestedCategory: "Housing",
			Confidence:        0.85,
		}
	}

	tests := []struct {
		name             string
		input            string
		expectedStatus   model.ClassificationStatus
		expectedCategory string
		expectedCount    int
		expectError      bool
	}{
		{
			name:             "skip all transactions in batch",
			input:            "s\n",
			expectedCount:    2,
			expectedStatus:   model.StatusUnclassified,
			expectedCategory: "", // Empty category for skip
		},
		{
			name:             "accept all in batch",
			input:            "a\n",
			expectedCount:    2,
			expectedStatus:   model.StatusClassifiedByAI,
			expectedCategory: "Housing",
		},
		{
			name:             "select category for all",
			input:            "e\nn\nUtilities\nn\n", // Select category -> New -> "Utilities" -> No description
			expectedCount:    2,
			expectedStatus:   model.StatusUserModified,
			expectedCategory: "Utilities",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			var writer bytes.Buffer
			prompter := NewCLIPrompter(reader, &writer)

			// Set total transactions for progress tracking
			prompter.SetTotalTransactions(len(pendingBatch))

			classifications, err := prompter.BatchConfirmClassifications(context.Background(), pendingBatch)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Len(t, classifications, tt.expectedCount)

			// Verify all classifications have the expected status and category
			for _, c := range classifications {
				assert.Equal(t, tt.expectedStatus, c.Status)
				assert.Equal(t, tt.expectedCategory, c.Category)

				// For skip, verify category is explicitly empty
				if tt.expectedStatus == model.StatusUnclassified {
					assert.Empty(t, c.Category, "skipped transactions must have empty category")
				}
			}

			// Check output contains expected messages
			output := writer.String()
			if tt.name == "skip all transactions in batch" {
				assert.Contains(t, output, "Skipped 2 transactions")
			}
		})
	}
}

func TestCLIPrompter_SkipAllClassifications(t *testing.T) {
	pending := []model.PendingClassification{
		{
			Transaction: model.Transaction{
				ID:           "skip1",
				Name:         "UNKNOWN VENDOR 1",
				MerchantName: "Unknown1",
				Amount:       25.00,
				Date:         time.Now(),
			},
			SuggestedCategory: "Other",
			Confidence:        0.25,
		},
		{
			Transaction: model.Transaction{
				ID:           "skip2",
				Name:         "UNKNOWN VENDOR 2",
				MerchantName: "Unknown2",
				Amount:       30.00,
				Date:         time.Now(),
			},
			SuggestedCategory: "Other",
			Confidence:        0.30,
		},
	}

	reader := strings.NewReader("")
	var writer bytes.Buffer
	prompter := NewCLIPrompter(reader, &writer)

	classifications, err := prompter.skipAllClassifications(pending)

	require.NoError(t, err)
	assert.Len(t, classifications, 2)

	// Verify all classifications are properly set for skip
	for i, c := range classifications {
		assert.Equal(t, pending[i].Transaction.ID, c.Transaction.ID)
		assert.Equal(t, model.StatusUnclassified, c.Status)
		assert.Empty(t, c.Category, "skipped classification must have empty category")
		assert.Equal(t, 0.0, c.Confidence)
		assert.NotZero(t, c.ClassifiedAt)
	}

	// Check that warning message was displayed
	output := writer.String()
	assert.Contains(t, output, "Skipped 2 transactions")
}
