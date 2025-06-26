package engine

import (
	"encoding/json"
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

	// Parse JSON output
	var result map[string]any
	err := json.Unmarshal([]byte(display), &result)
	assert.NoError(t, err)

	// Check JSON fields
	assert.Equal(t, float64(100), result["total_merchants"])
	assert.Equal(t, float64(500), result["total_transactions"])
	assert.Equal(t, float64(85), result["auto_accepted_count"])
	assert.Equal(t, float64(85), result["auto_accepted_percent"])
	assert.Equal(t, float64(400), result["auto_accepted_transactions"])
	assert.Equal(t, float64(10), result["needs_review_count"])
	assert.Equal(t, float64(80), result["needs_review_transactions"])
	assert.Equal(t, float64(5), result["failed_count"])
	assert.Equal(t, "30s", result["processing_time"])
}

func TestBatchClassificationEmptySummary(t *testing.T) {
	summary := &BatchClassificationSummary{}
	display := summary.GetDisplay()
	assert.Equal(t, `{"message":"No transactions to classify"}`, display)
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
