package engine

import (
	"context"
	"testing"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessMerchantBatch(t *testing.T) {
	ctx := context.Background()

	// Create real storage for testing
	db, err := storage.NewSQLiteStorage(":memory:")
	require.NoError(t, err)
	require.NoError(t, db.Migrate(ctx))

	// Create mock classifier
	mockClassifier := NewMockClassifier()

	engine := &ClassificationEngine{
		storage:    db,
		classifier: mockClassifier,
	}

	// Test data
	merchants := []string{"Walmart", "Target", "Shell"}
	merchantGroups := map[string][]model.Transaction{
		"Walmart": {
			{ID: "tx1", MerchantName: "Walmart", Amount: 50.00, Type: "DEBIT"},
			{ID: "tx2", MerchantName: "Walmart", Amount: 75.00, Type: "DEBIT"},
		},
		"Target": {
			{ID: "tx3", MerchantName: "Target", Amount: 120.00, Type: "DEBIT"},
		},
		"Shell": {
			{ID: "tx4", MerchantName: "Shell", Amount: 40.00, Type: "DEBIT"},
		},
	}

	categories := []model.Category{
		{Name: "Groceries", Description: "Grocery stores"},
		{Name: "Department Stores", Description: "General merchandise"},
		{Name: "Gas Stations", Description: "Fuel and gas"},
	}

	opts := BatchClassificationOptions{
		BatchSize:       5,
		ParallelWorkers: 2,
	}

	// Test 1: Successful batch classification
	t.Run("successful batch classification", func(t *testing.T) {
		results := engine.processMerchantBatch(ctx, merchants, merchantGroups, categories, opts)

		assert.Len(t, results, 3)

		// Verify results
		for _, result := range results {
			assert.NoError(t, result.Error)
			assert.NotNil(t, result.Suggestion)
			assert.Contains(t, []string{"Walmart", "Target", "Shell"}, result.Merchant)
		}
	})

	// Test 2: Existing vendor rule
	t.Run("existing vendor rule", func(t *testing.T) {
		// Create categories in database first
		for _, cat := range categories {
			_, err := db.CreateCategoryWithType(ctx, cat.Name, cat.Description, model.CategoryTypeExpense)
			if err != nil && err.Error() != "category already exists" {
				require.NoError(t, err)
			}
		}

		// Create a vendor rule for Walmart
		vendor := &model.Vendor{
			Name:        "Walmart",
			Category:    "Groceries",
			LastUpdated: time.Now(),
			UseCount:    1,
		}
		err := db.SaveVendor(ctx, vendor)
		require.NoError(t, err)

		results := engine.processMerchantBatch(ctx, merchants, merchantGroups, categories, opts)

		assert.Len(t, results, 3)

		// Find Walmart result
		var walmartResult *BatchResult
		for i := range results {
			if results[i].Merchant == "Walmart" {
				walmartResult = &results[i]
				break
			}
		}

		require.NotNil(t, walmartResult)
		assert.NoError(t, walmartResult.Error)
		assert.NotNil(t, walmartResult.Suggestion)
		assert.Equal(t, "Groceries", walmartResult.Suggestion.Category)
		assert.Equal(t, 1.0, walmartResult.Suggestion.Score) // Vendor rules have 100% confidence
		assert.True(t, walmartResult.AutoAccepted)
	})

	// Test 3: Empty merchants
	t.Run("empty merchants", func(t *testing.T) {
		results := engine.processMerchantBatch(ctx, []string{}, merchantGroups, categories, opts)
		assert.Empty(t, results)
	})

	// Test 4: Check pattern boosting
	t.Run("check pattern boosting", func(t *testing.T) {
		// Create check transaction
		checkMerchants := []string{"CHECK 1234"}
		checkGroups := map[string][]model.Transaction{
			"CHECK 1234": {
				{ID: "tx5", MerchantName: "CHECK 1234", Amount: 500.00, Type: "CHECK", CheckNumber: "1234"},
			},
		}

		// Create check pattern
		pattern := &model.CheckPattern{
			Category:    "Rent",
			PatternName: "Monthly Rent",
			AmountMin:   floatPtr(450),
			AmountMax:   floatPtr(550),
			UseCount:    10,
		}
		err := db.CreateCheckPattern(ctx, pattern)
		require.NoError(t, err)

		// Add Rent category
		categories = append(categories, model.Category{Name: "Rent", Description: "Rent payments"})

		results := engine.processMerchantBatch(ctx, checkMerchants, checkGroups, categories, opts)

		assert.Len(t, results, 1)
		assert.NoError(t, results[0].Error)
		assert.NotNil(t, results[0].Suggestion)
		// The mock classifier should apply check pattern boosting
		assert.Equal(t, "Rent", results[0].Suggestion.Category)
	})
}

func TestBatchWorker(t *testing.T) {
	ctx := context.Background()

	db, err := storage.NewSQLiteStorage(":memory:")
	require.NoError(t, err)
	require.NoError(t, db.Migrate(ctx))

	mockClassifier := NewMockClassifier()

	engine := &ClassificationEngine{
		storage:    db,
		classifier: mockClassifier,
	}

	// Create work channel with merchants
	workChan := make(chan string, 10)
	merchants := []string{"M1", "M2", "M3", "M4", "M5", "M6", "M7"}
	for _, m := range merchants {
		workChan <- m
	}
	close(workChan)

	resultsChan := make(chan BatchResult, 10)

	merchantGroups := make(map[string][]model.Transaction)
	for _, m := range merchants {
		merchantGroups[m] = []model.Transaction{
			{ID: m + "-tx1", MerchantName: m, Amount: 50.00},
		}
	}

	categories := []model.Category{
		{Name: "Test Category", Description: "Test"},
	}

	opts := BatchClassificationOptions{
		BatchSize: 3, // Process 3 merchants at a time
	}

	// Run worker
	go func() {
		engine.batchWorker(ctx, 0, workChan, resultsChan, merchantGroups, categories, opts)
		close(resultsChan)
	}()

	// Collect results
	results := make([]BatchResult, 0, 7)
	for result := range resultsChan {
		results = append(results, result)
	}

	assert.Len(t, results, 7)
	for _, result := range results {
		assert.NoError(t, result.Error)
		assert.NotNil(t, result.Suggestion)
	}
}

func TestBatchClassificationSummary(t *testing.T) {
	summary := &BatchClassificationSummary{
		TotalMerchants:    100,
		TotalTransactions: 500,
		AutoAcceptedCount: 85,
		AutoAcceptedTxns:  425,
		NeedsReviewCount:  10,
		NeedsReviewTxns:   50,
		FailedCount:       5,
		ProcessingTime:    30 * time.Second,
	}

	display := summary.GetDisplay()

	// Verify JSON format
	assert.Contains(t, display, `"total_merchants":100`)
	assert.Contains(t, display, `"auto_accepted_percent":85`)
	assert.Contains(t, display, `"processing_time":"30s"`)

	// Test empty summary
	emptySummary := &BatchClassificationSummary{}
	emptyDisplay := emptySummary.GetDisplay()
	assert.Equal(t, `{"message":"No transactions to classify"}`, emptyDisplay)
}

func TestProcessMerchantsParallel(t *testing.T) {
	ctx := context.Background()

	db, err := storage.NewSQLiteStorage(":memory:")
	require.NoError(t, err)
	require.NoError(t, db.Migrate(ctx))

	mockClassifier := NewMockClassifier()

	engine := &ClassificationEngine{
		storage:    db,
		classifier: mockClassifier,
	}

	// Create test data
	merchants := []string{"M1", "M2", "M3", "M4", "M5"}
	merchantGroups := make(map[string][]model.Transaction)
	for _, m := range merchants {
		merchantGroups[m] = []model.Transaction{
			{ID: m + "-tx1", MerchantName: m, Amount: 50.00},
		}
	}

	categories := []model.Category{
		{Name: "Test", Description: "Test category"},
	}

	opts := BatchClassificationOptions{
		BatchSize:       3,
		ParallelWorkers: 2,
	}

	results := engine.processMerchantsParallel(ctx, merchants, merchantGroups, categories, opts)

	assert.Len(t, results, 5)

	// Verify all merchants were processed
	processedMerchants := make(map[string]bool)
	for _, result := range results {
		processedMerchants[result.Merchant] = true
		assert.NoError(t, result.Error)
		assert.NotNil(t, result.Suggestion)
	}

	for _, m := range merchants {
		assert.True(t, processedMerchants[m], "merchant %s not processed", m)
	}
}

func TestClassifyTransactionsBatch(t *testing.T) {
	ctx := context.Background()

	// Create test storage
	db, err := storage.NewSQLiteStorage(":memory:")
	require.NoError(t, err)
	require.NoError(t, db.Migrate(ctx))

	// Add categories
	categories := []model.Category{
		{Name: "Groceries", Type: model.CategoryTypeExpense, Description: "Grocery stores"},
		{Name: "Shopping", Type: model.CategoryTypeExpense, Description: "Shopping"},
		{Name: "Gas", Type: model.CategoryTypeExpense, Description: "Gas stations"},
	}
	for _, cat := range categories {
		_, createErr := db.CreateCategoryWithType(ctx, cat.Name, cat.Description, cat.Type)
		require.NoError(t, createErr)
	}

	// Add test transactions
	transactions := []model.Transaction{
		{
			ID:           "tx1",
			Hash:         "hash1",
			Name:         "WALMART STORE #123",
			MerchantName: "Walmart",
			Amount:       50.00,
			Type:         "DEBIT",
			Date:         time.Now(),
			AccountID:    "acc1",
		},
		{
			ID:           "tx2",
			Hash:         "hash2",
			Name:         "WALMART STORE #456",
			MerchantName: "Walmart",
			Amount:       75.00,
			Type:         "DEBIT",
			Date:         time.Now(),
			AccountID:    "acc1",
		},
		{
			ID:           "tx3",
			Hash:         "hash3",
			Name:         "SHELL GAS STATION",
			MerchantName: "Shell",
			Amount:       40.00,
			Type:         "DEBIT",
			Date:         time.Now(),
			AccountID:    "acc1",
		},
	}

	err = db.SaveTransactions(ctx, transactions)
	require.NoError(t, err)

	// Create engine with mock classifier
	mockClassifier := NewMockClassifier()
	mockPrompter := NewMockPrompter(true) // Auto-accept all

	engine := &ClassificationEngine{
		storage:    db,
		classifier: mockClassifier,
		prompter:   mockPrompter,
	}

	opts := BatchClassificationOptions{
		AutoAcceptThreshold: 0.80,
		BatchSize:           5,
		ParallelWorkers:     2,
		SkipManualReview:    false,
	}

	// Run batch classification
	summary, err := engine.ClassifyTransactionsBatch(ctx, nil, opts)
	require.NoError(t, err)

	// Verify summary
	assert.Equal(t, 2, summary.TotalMerchants) // Walmart and Shell
	assert.Equal(t, 3, summary.TotalTransactions)
	assert.Equal(t, 0, summary.FailedCount)

	// Verify classifications were saved (check transactions are now classified)
	txns, err := db.GetTransactionsToClassify(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, 0, len(txns)) // All should be classified
}
