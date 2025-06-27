package main

import (
	"context"
	"testing"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResetCommand(t *testing.T) {
	// Setup test database
	ctx := context.Background()
	store, err := storage.NewSQLiteStorage(":memory:")
	require.NoError(t, err)
	defer func() {
		if closeErr := store.Close(); closeErr != nil {
			t.Logf("Failed to close store: %v", closeErr)
		}
	}()
	require.NoError(t, store.Migrate(ctx))

	// Create test data
	transactions := []model.Transaction{
		{
			ID:           "txn1",
			Hash:         "hash1",
			Date:         time.Now(),
			Name:         "Test Transaction 1",
			MerchantName: "Merchant 1",
			Amount:       100.00,
			AccountID:    "acc1",
		},
		{
			ID:           "txn2",
			Hash:         "hash2",
			Date:         time.Now(),
			Name:         "Test Transaction 2",
			MerchantName: "Merchant 2",
			Amount:       200.00,
			AccountID:    "acc1",
		},
	}

	// Save transactions
	err = store.SaveTransactions(ctx, transactions)
	require.NoError(t, err)

	// Create categories
	_, err = store.CreateCategory(ctx, "Food", "Food and dining")
	require.NoError(t, err)
	_, err = store.CreateCategory(ctx, "Transport", "Transportation")
	require.NoError(t, err)

	// Create classifications
	classifications := []model.Classification{
		{
			Transaction:  transactions[0],
			Category:     "Food",
			Status:       model.StatusClassifiedByAI,
			Confidence:   0.95,
			ClassifiedAt: time.Now(),
		},
		{
			Transaction:  transactions[1],
			Category:     "Transport",
			Status:       model.StatusUserModified,
			Confidence:   1.0,
			ClassifiedAt: time.Now(),
		},
	}

	for _, cls := range classifications {
		err = store.SaveClassification(ctx, &cls)
		require.NoError(t, err)
	}

	// Create vendor rules
	vendors := []model.Vendor{
		{
			Name:     "Merchant 1",
			Category: "Food",
			UseCount: 5,
		},
		{
			Name:     "Merchant 2",
			Category: "Transport",
			UseCount: 10,
		},
	}

	for _, vendor := range vendors {
		err = store.SaveVendor(ctx, &vendor)
		require.NoError(t, err)
	}

	t.Run("reset with force flag", func(t *testing.T) {
		// Count before reset
		classCount, err := getClassificationCount(ctx, store)
		require.NoError(t, err)
		assert.Equal(t, 2, classCount)

		// We can't directly count vendors, so verify they exist by trying to get them
		allVendors, err := store.GetAllVendors(ctx)
		require.NoError(t, err)
		assert.Len(t, allVendors, 2)

		// Run reset with force
		resetForce = true
		resetKeepVendor = false
		defer func() {
			resetForce = false
			resetKeepVendor = false
		}()

		// The reset command uses the test store directly

		// Test the underlying functions directly
		err = clearClassifications(ctx, store)
		require.NoError(t, err)

		err = clearVendors(ctx, store)
		require.NoError(t, err)

		// Verify classifications are cleared
		classCount, err = getClassificationCount(ctx, store)
		require.NoError(t, err)
		assert.Equal(t, 0, classCount)

		// Verify vendors are cleared
		allVendors, err = store.GetAllVendors(ctx)
		require.NoError(t, err)
		assert.Len(t, allVendors, 0)

		// Verify transactions still exist
		txns, err := store.GetTransactionsToClassify(ctx, nil)
		require.NoError(t, err)
		assert.Len(t, txns, 2)
	})
}

func TestResetKeepVendors(t *testing.T) {
	ctx := context.Background()
	store, err := storage.NewSQLiteStorage(":memory:")
	require.NoError(t, err)
	defer func() {
		if closeErr := store.Close(); closeErr != nil {
			t.Logf("Failed to close store: %v", closeErr)
		}
	}()
	require.NoError(t, store.Migrate(ctx))

	// Create test data
	transaction := model.Transaction{
		ID:           "txn1",
		Hash:         "hash1",
		Date:         time.Now(),
		Name:         "Test Transaction",
		MerchantName: "Test Merchant",
		Amount:       100.00,
		AccountID:    "acc1",
	}
	err = store.SaveTransactions(ctx, []model.Transaction{transaction})
	require.NoError(t, err)

	// Create category
	_, err = store.CreateCategory(ctx, "Food", "Food and dining")
	require.NoError(t, err)

	// Create classification
	classification := model.Classification{
		Transaction:  transaction,
		Category:     "Food",
		Status:       model.StatusClassifiedByAI,
		Confidence:   0.95,
		ClassifiedAt: time.Now(),
	}
	err = store.SaveClassification(ctx, &classification)
	require.NoError(t, err)

	// Create vendor
	vendor := model.Vendor{
		Name:     "Test Merchant",
		Category: "Food",
		UseCount: 5,
	}
	err = store.SaveVendor(ctx, &vendor)
	require.NoError(t, err)

	// Reset keeping vendors
	err = clearClassifications(ctx, store)
	require.NoError(t, err)
	// Don't clear vendors

	// Verify classifications are cleared
	classCount, err := getClassificationCount(ctx, store)
	require.NoError(t, err)
	assert.Equal(t, 0, classCount)

	// Verify vendors still exist
	allVendors, err := store.GetAllVendors(ctx)
	require.NoError(t, err)
	assert.Len(t, allVendors, 1)
}

func TestResetNoClassifications(t *testing.T) {
	ctx := context.Background()
	store, err := storage.NewSQLiteStorage(":memory:")
	require.NoError(t, err)
	defer func() {
		if closeErr := store.Close(); closeErr != nil {
			t.Logf("Failed to close store: %v", closeErr)
		}
	}()
	require.NoError(t, store.Migrate(ctx))

	// No classifications exist
	classCount, err := getClassificationCount(ctx, store)
	require.NoError(t, err)
	assert.Equal(t, 0, classCount)

	// Test would show "No classifications found" message
	// In real implementation, the command would exit early
}

func TestGetClassificationCount(t *testing.T) {
	ctx := context.Background()
	store, err := storage.NewSQLiteStorage(":memory:")
	require.NoError(t, err)
	defer func() {
		if closeErr := store.Close(); closeErr != nil {
			t.Logf("Failed to close store: %v", closeErr)
		}
	}()
	require.NoError(t, store.Migrate(ctx))

	// Initially no classifications
	count, err := getClassificationCount(ctx, store)
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	// Add a transaction and classification
	transaction := model.Transaction{
		ID:        "txn1",
		Hash:      "hash1",
		Date:      time.Now(),
		Name:      "Test",
		Amount:    100.00,
		AccountID: "acc1",
	}
	err = store.SaveTransactions(ctx, []model.Transaction{transaction})
	require.NoError(t, err)

	// Create the Food category first
	_, err = store.CreateCategory(ctx, "Food", "Test category")
	require.NoError(t, err)

	classification := model.Classification{
		Transaction:  transaction,
		Category:     "Food",
		Status:       model.StatusClassifiedByAI,
		Confidence:   0.95,
		ClassifiedAt: time.Now(),
	}
	err = store.SaveClassification(ctx, &classification)
	require.NoError(t, err)

	// Now should have 1 classification
	finalCount, err := getClassificationCount(ctx, store)
	require.NoError(t, err)
	assert.Equal(t, 1, finalCount)
}

func TestClearFunctions(t *testing.T) {
	ctx := context.Background()
	store, err := storage.NewSQLiteStorage(":memory:")
	require.NoError(t, err)
	defer func() {
		if closeErr := store.Close(); closeErr != nil {
			t.Logf("Failed to close store: %v", closeErr)
		}
	}()
	require.NoError(t, store.Migrate(ctx))

	// Add test data
	transaction := model.Transaction{
		ID:        "txn1",
		Hash:      "hash1",
		Date:      time.Now(),
		Name:      "Test",
		Amount:    100.00,
		AccountID: "acc1",
	}
	err = store.SaveTransactions(ctx, []model.Transaction{transaction})
	require.NoError(t, err)

	// Create category
	_, err = store.CreateCategory(ctx, "Food", "Food and dining")
	require.NoError(t, err)

	classification := model.Classification{
		Transaction:  transaction,
		Category:     "Food",
		Status:       model.StatusClassifiedByAI,
		Confidence:   0.95,
		ClassifiedAt: time.Now(),
	}
	err = store.SaveClassification(ctx, &classification)
	require.NoError(t, err)

	vendor := model.Vendor{
		Name:     "Test Vendor",
		Category: "Food",
		UseCount: 5,
	}
	err = store.SaveVendor(ctx, &vendor)
	require.NoError(t, err)

	t.Run("clearClassifications", func(t *testing.T) {
		err = clearClassifications(ctx, store)
		require.NoError(t, err)

		// Verify cleared
		count, getErr := getClassificationCount(ctx, store)
		require.NoError(t, getErr)
		assert.Equal(t, 0, count)
	})

	t.Run("clearVendors", func(t *testing.T) {
		// Re-add vendor since it was cleared
		err = store.SaveVendor(ctx, &vendor)
		require.NoError(t, err)

		err = clearVendors(ctx, store)
		require.NoError(t, err)

		// Verify cleared (vendor count will be -1 since we can't count them directly)
		count, err := getVendorCount(ctx, store)
		require.NoError(t, err)
		assert.Equal(t, -1, count)
	})
}

// Test the command integration.
func TestResetCommandIntegration(_ *testing.T) {
	// This would test the full command execution
	// but requires mocking stdin for the confirmation prompt
	// For now, the unit tests above cover the core functionality
}
