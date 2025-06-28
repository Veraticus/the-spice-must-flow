package storage

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveClassification_SkipFunctionality(t *testing.T) {
	ctx := context.Background()

	// Create test database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	storage, err := NewSQLiteStorage(dbPath)
	require.NoError(t, err)
	defer func() {
		if closeErr := storage.Close(); closeErr != nil {
			t.Logf("Failed to close storage: %v", closeErr)
		}
	}()

	// Run migrations
	err = storage.Migrate(ctx)
	require.NoError(t, err)

	// Create test category
	_, err = storage.CreateCategory(ctx, "Food", "Dining and groceries")
	require.NoError(t, err)

	// Create test transaction
	txn := model.Transaction{
		ID:           "skip-test-1",
		Hash:         "hash-skip-1",
		Date:         time.Now(),
		Name:         "Test Restaurant",
		MerchantName: "Test Restaurant",
		Amount:       25.50,
		AccountID:    "test-account",
		Direction:    model.DirectionExpense,
	}
	err = storage.SaveTransactions(ctx, []model.Transaction{txn})
	require.NoError(t, err)

	t.Run("skip transaction with unclassified status", func(t *testing.T) {
		classification := &model.Classification{
			Transaction:  txn,
			Category:     "", // Empty category for skip
			Status:       model.StatusUnclassified,
			Confidence:   0.0,
			ClassifiedAt: time.Now(),
		}

		err := storage.SaveClassification(ctx, classification)
		assert.NoError(t, err, "should allow saving unclassified transaction with empty category")

		// Verify it was saved correctly
		classifications, err := storage.GetClassificationsByDateRange(ctx,
			time.Now().Add(-24*time.Hour),
			time.Now().Add(24*time.Hour))
		require.NoError(t, err)

		found := false
		for _, c := range classifications {
			if c.Transaction.ID == txn.ID {
				found = true
				assert.Equal(t, model.StatusUnclassified, c.Status)
				assert.Empty(t, c.Category)
				assert.Equal(t, 0.0, c.Confidence)
				break
			}
		}
		assert.True(t, found, "classification should be saved")
	})

	t.Run("error when unclassified has category", func(t *testing.T) {
		txn2 := txn
		txn2.ID = "skip-test-2"
		txn2.Hash = "hash-skip-2"
		err := storage.SaveTransactions(ctx, []model.Transaction{txn2})
		require.NoError(t, err)

		classification := &model.Classification{
			Transaction:  txn2,
			Category:     "Food", // Should not have category when unclassified
			Status:       model.StatusUnclassified,
			Confidence:   0.0,
			ClassifiedAt: time.Now(),
		}

		err = storage.SaveClassification(ctx, classification)
		assert.Error(t, err, "should not allow unclassified with category")
		assert.Contains(t, err.Error(), "unclassified transactions should not have a category")
	})

	t.Run("error when classified has no category", func(t *testing.T) {
		txn3 := txn
		txn3.ID = "skip-test-3"
		txn3.Hash = "hash-skip-3"
		err := storage.SaveTransactions(ctx, []model.Transaction{txn3})
		require.NoError(t, err)

		classification := &model.Classification{
			Transaction:  txn3,
			Category:     "", // Should have category when classified
			Status:       model.StatusClassifiedByAI,
			Confidence:   0.95,
			ClassifiedAt: time.Now(),
		}

		err = storage.SaveClassification(ctx, classification)
		assert.Error(t, err, "should not allow classified without category")
		assert.Contains(t, err.Error(), "missing category")
	})

	t.Run("no vendor rule created for skipped transactions", func(t *testing.T) {
		txn4 := txn
		txn4.ID = "skip-test-4"
		txn4.Hash = "hash-skip-4"
		txn4.MerchantName = "Unique Skip Merchant"
		err := storage.SaveTransactions(ctx, []model.Transaction{txn4})
		require.NoError(t, err)

		classification := &model.Classification{
			Transaction:  txn4,
			Category:     "",
			Status:       model.StatusUnclassified,
			Confidence:   0.0,
			ClassifiedAt: time.Now(),
		}

		err = storage.SaveClassification(ctx, classification)
		require.NoError(t, err)

		// Verify no vendor rule was created
		vendor, err := storage.GetVendor(ctx, "Unique Skip Merchant")
		assert.Equal(t, sql.ErrNoRows, err, "should return ErrNoRows when vendor not found")
		assert.Nil(t, vendor, "no vendor rule should be created for skipped transactions")
	})
}
