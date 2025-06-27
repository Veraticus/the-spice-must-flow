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

// TestClassificationPreventsReclassification verifies that once a transaction
// is classified, it won't be returned for re-classification.
func TestClassificationPreventsReclassification(t *testing.T) {
	ctx := context.Background()

	// Setup test database
	db, err := storage.NewSQLiteStorage(":memory:")
	require.NoError(t, err)
	defer func() {
		if err := db.Close(); err != nil {
			t.Logf("Failed to close db: %v", err)
		}
	}()
	require.NoError(t, db.Migrate(ctx))

	// Create test transactions
	transactions := []model.Transaction{
		{
			ID:           "txn1",
			Hash:         "hash1",
			Date:         time.Now(),
			Name:         "Transaction 1",
			MerchantName: "Merchant 1",
			Amount:       100.00,
			AccountID:    "acc1",
			Type:         "DEBIT",
		},
		{
			ID:           "txn2",
			Hash:         "hash2",
			Date:         time.Now(),
			Name:         "Transaction 2",
			MerchantName: "Merchant 2",
			Amount:       200.00,
			AccountID:    "acc1",
			Type:         "DEBIT",
		},
		{
			ID:           "txn3",
			Hash:         "hash3",
			Date:         time.Now(),
			Name:         "Transaction 3",
			MerchantName: "Merchant 3",
			Amount:       300.00,
			AccountID:    "acc1",
			Type:         "DEBIT",
		},
	}

	// Save all transactions
	err = db.SaveTransactions(ctx, transactions)
	require.NoError(t, err)

	// Create categories
	_, err = db.CreateCategory(ctx, "Food", "Food and dining")
	require.NoError(t, err)
	_, err = db.CreateCategory(ctx, "Transport", "Transportation")
	require.NoError(t, err)
	_, err = db.CreateCategory(ctx, "Shopping", "Shopping and retail")
	require.NoError(t, err)
	_, err = db.CreateCategory(ctx, "Rent", "Monthly rent")
	require.NoError(t, err)

	// Initially, all transactions should be available for classification
	toClassify, err := db.GetTransactionsToClassify(ctx, nil)
	require.NoError(t, err)
	assert.Len(t, toClassify, 3, "All transactions should be unclassified initially")

	// Classify the first transaction
	classification1 := &model.Classification{
		Transaction:  transactions[0],
		Category:     "Food",
		Status:       model.StatusClassifiedByAI,
		Confidence:   0.95,
		ClassifiedAt: time.Now(),
	}
	err = db.SaveClassification(ctx, classification1)
	require.NoError(t, err)

	// Now only 2 transactions should be available for classification
	toClassify, err = db.GetTransactionsToClassify(ctx, nil)
	require.NoError(t, err)
	assert.Len(t, toClassify, 2, "One transaction should be excluded after classification")

	// Verify the classified transaction is not in the list
	for _, txn := range toClassify {
		assert.NotEqual(t, "txn1", txn.ID, "Classified transaction should not be returned")
	}

	// Classify the second transaction with user modification
	classification2 := &model.Classification{
		Transaction:  transactions[1],
		Category:     "Transport",
		Status:       model.StatusUserModified,
		Confidence:   1.0,
		ClassifiedAt: time.Now(),
	}
	err = db.SaveClassification(ctx, classification2)
	require.NoError(t, err)

	// Now only 1 transaction should be available
	toClassify, err = db.GetTransactionsToClassify(ctx, nil)
	require.NoError(t, err)
	assert.Len(t, toClassify, 1, "Two transactions should be excluded after classification")
	assert.Equal(t, "txn3", toClassify[0].ID, "Only unclassified transaction should be returned")

	// Classify the last transaction
	classification3 := &model.Classification{
		Transaction:  transactions[2],
		Category:     "Shopping",
		Status:       model.StatusClassifiedByRule,
		Confidence:   1.0,
		ClassifiedAt: time.Now(),
	}
	err = db.SaveClassification(ctx, classification3)
	require.NoError(t, err)

	// No transactions should be available for classification
	toClassify, err = db.GetTransactionsToClassify(ctx, nil)
	require.NoError(t, err)
	assert.Len(t, toClassify, 0, "No transactions should be available after all are classified")
}

// TestClassificationWithEngine tests that the classification engine respects
// already classified transactions.
func TestClassificationWithEngine(t *testing.T) {
	ctx := context.Background()

	// Setup
	db, err := storage.NewSQLiteStorage(":memory:")
	require.NoError(t, err)
	defer func() {
		if err := db.Close(); err != nil {
			t.Logf("Failed to close db: %v", err)
		}
	}()
	require.NoError(t, db.Migrate(ctx))

	// Create categories
	categories := []string{"Food", "Transport", "Shopping"}
	for _, cat := range categories {
		_, catErr := db.CreateCategory(ctx, cat, "Test category")
		require.NoError(t, catErr)
	}

	// Create test transactions
	transactions := []model.Transaction{
		{
			ID:           "txn1",
			Hash:         "hash1",
			Date:         time.Now(),
			Name:         "Starbucks Coffee",
			MerchantName: "Starbucks",
			Amount:       5.00,
			AccountID:    "acc1",
			Type:         "DEBIT",
		},
		{
			ID:           "txn2",
			Hash:         "hash2",
			Date:         time.Now(),
			Name:         "Walmart Purchase",
			MerchantName: "Walmart",
			Amount:       50.00,
			AccountID:    "acc1",
			Type:         "DEBIT",
		},
	}

	err = db.SaveTransactions(ctx, transactions)
	require.NoError(t, err)

	// Pre-classify one transaction
	preClassification := &model.Classification{
		Transaction:  transactions[0],
		Category:     "Food",
		Status:       model.StatusUserModified,
		Confidence:   1.0,
		ClassifiedAt: time.Now(),
		Notes:        "Pre-classified by user",
	}
	err = db.SaveClassification(ctx, preClassification)
	require.NoError(t, err)

	// Create engine with mock classifier and prompter
	classifier := NewMockClassifier()
	prompter := NewMockPrompter(true) // Auto-accept all
	engine := New(db, classifier, prompter)

	// Run classification
	err = engine.ClassifyTransactions(ctx, nil)
	require.NoError(t, err)

	// Check classifications
	allClassifications, err := db.GetClassificationsByDateRange(
		ctx,
		time.Now().AddDate(0, 0, -1),
		time.Now().AddDate(0, 0, 1),
	)
	require.NoError(t, err)

	// Should have 2 classifications total
	assert.Len(t, allClassifications, 2)

	// Find the pre-classified transaction
	var starbucksClassification *model.Classification
	var walmartClassification *model.Classification

	for i := range allClassifications {
		switch allClassifications[i].Transaction.ID {
		case "txn1":
			starbucksClassification = &allClassifications[i]
		case "txn2":
			walmartClassification = &allClassifications[i]
		}
	}

	require.NotNil(t, starbucksClassification, "Starbucks classification should exist")
	require.NotNil(t, walmartClassification, "Walmart classification should exist")

	// Verify the pre-classified transaction wasn't changed
	assert.Equal(t, "Food", starbucksClassification.Category)
	assert.Equal(t, model.StatusUserModified, starbucksClassification.Status)
	assert.Equal(t, "Pre-classified by user", starbucksClassification.Notes)

	// Verify only the unclassified transaction was processed
	assert.Equal(t, "Shopping", walmartClassification.Category) // Based on mock classifier logic
	assert.NotEqual(t, model.StatusUserModified, walmartClassification.Status)

	// Verify the mock classifier was only called once (for the unclassified transaction)
	assert.Equal(t, 1, classifier.CallCount(), "Classifier should only be called for unclassified transactions")
}

// TestResetEnablesReclassification verifies that after reset, transactions
// can be classified again.
func TestResetEnablesReclassification(t *testing.T) {
	ctx := context.Background()

	// Setup
	db, err := storage.NewSQLiteStorage(":memory:")
	require.NoError(t, err)
	defer func() {
		if err := db.Close(); err != nil {
			t.Logf("Failed to close db: %v", err)
		}
	}()
	require.NoError(t, db.Migrate(ctx))

	// Create and save transaction
	transaction := model.Transaction{
		ID:           "txn1",
		Hash:         "hash1",
		Date:         time.Now(),
		Name:         "Test Transaction",
		MerchantName: "Test Merchant",
		Amount:       100.00,
		AccountID:    "acc1",
		Type:         "DEBIT",
	}
	err = db.SaveTransactions(ctx, []model.Transaction{transaction})
	require.NoError(t, err)

	// Create category
	_, err = db.CreateCategory(ctx, "Food", "Food and dining")
	require.NoError(t, err)

	// Initially should be available for classification
	toClassify, err := db.GetTransactionsToClassify(ctx, nil)
	require.NoError(t, err)
	assert.Len(t, toClassify, 1)

	// Classify it
	classification := &model.Classification{
		Transaction:  transaction,
		Category:     "Food",
		Status:       model.StatusClassifiedByAI,
		Confidence:   0.95,
		ClassifiedAt: time.Now(),
	}
	err = db.SaveClassification(ctx, classification)
	require.NoError(t, err)

	// Should not be available for classification anymore
	toClassify, err = db.GetTransactionsToClassify(ctx, nil)
	require.NoError(t, err)
	assert.Len(t, toClassify, 0)

	// Clear classifications (simulate reset)
	err = db.ClearAllClassifications(ctx)
	require.NoError(t, err)

	// Should be available for classification again
	toClassify, err = db.GetTransactionsToClassify(ctx, nil)
	require.NoError(t, err)
	assert.Len(t, toClassify, 1)
	assert.Equal(t, "txn1", toClassify[0].ID)
}
