package engine

import (
	"testing"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestBatchClassificationOptions(t *testing.T) {
	opts := DefaultBatchOptions()

	assert.Equal(t, 0.95, opts.AutoAcceptThreshold)
	assert.Equal(t, 20, opts.BatchSize)
	assert.Equal(t, 5, opts.ParallelWorkers)
}

func TestBatchClassificationSummaryDisplay(t *testing.T) {
	summary := &BatchClassificationSummary{
		TotalMerchants:    100,
		TotalTransactions: 500,
		AutoAcceptedCount: 85,
		AutoAcceptedTxns:  400,
		NeedsReviewCount:  10,
		NeedsReviewTxns:   80,
		FailedCount:       5,
		ProcessingTime:    30 * time.Second,
	}

	display := summary.GetDisplay()
	assert.Contains(t, display, "Batch Classification Complete")
	assert.Contains(t, display, "100")
	assert.Contains(t, display, "85 merchants (85%)")
	assert.Contains(t, display, "30s")
}

func TestBatchClassificationEmptySummary(t *testing.T) {
	summary := &BatchClassificationSummary{}
	display := summary.GetDisplay()
	assert.Equal(t, "No transactions to classify", display)
}

func TestBatchResultStructure(t *testing.T) {
	result := BatchResult{
		Merchant: "STARBUCKS",
		Transactions: []model.Transaction{
			{
				ID:           "1",
				MerchantName: "STARBUCKS",
				Amount:       -5.50,
			},
		},
		Suggestion: &model.CategoryRanking{
			Category: "Coffee Shops",
			Score:    0.96,
			IsNew:    false,
		},
		// Error field is nil for successful classification
		AutoAccepted: true,
	}

	assert.Equal(t, "STARBUCKS", result.Merchant)
	assert.Len(t, result.Transactions, 1)
	assert.NotNil(t, result.Suggestion)
	assert.True(t, result.AutoAccepted)
}
