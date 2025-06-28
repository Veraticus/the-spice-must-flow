package engine

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"log/slog"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEngine_BatchClassification_AIGeneratedDescription(t *testing.T) {
	ctx := context.Background()

	// Create test database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := storage.NewSQLiteStorage(dbPath)
	require.NoError(t, err)
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Logf("Failed to close database: %v", closeErr)
		}
	}()

	// Run migrations
	err = db.Migrate(ctx)
	require.NoError(t, err)

	// Create initial categories
	_, err = db.CreateCategory(ctx, "Food & Dining", "Restaurants and groceries")
	require.NoError(t, err)
	_, err = db.CreateCategory(ctx, "Transportation", "Travel and transport costs")
	require.NoError(t, err)

	t.Run("AI generates description when user chooses", func(t *testing.T) {
		// Create test transactions
		txns := []model.Transaction{
			{
				ID:           "ai-desc-1",
				Hash:         "hash-ai-1",
				Date:         time.Now(),
				Name:         "Beach Resort",
				MerchantName: "Beach Resort",
				Amount:       500.00,
				AccountID:    "test-account",
				Direction:    model.DirectionExpense,
			},
			{
				ID:           "ai-desc-2",
				Hash:         "hash-ai-2",
				Date:         time.Now().Add(-24 * time.Hour),
				Name:         "Beach Resort",
				MerchantName: "Beach Resort",
				Amount:       750.00,
				AccountID:    "test-account",
				Direction:    model.DirectionExpense,
			},
		}
		err = db.SaveTransactions(ctx, txns)
		require.NoError(t, err)

		// Set up mock classifier that will generate description
		classifier := NewMockClassifier()
		classifier.SetGenerateDescriptionResponse(
			"Expenses related to travel, vacations, and leisure trips",
			nil,
		)

		// Set up mock prompter that simulates user choosing new category with AI description
		prompter := NewMockPrompter(false)
		// The prompter will return a classification with NEW_CATEGORY| signal
		prompter.SetBatchResponse([]model.Classification{
			{
				Transaction:  txns[0],
				Category:     "Vacation",
				Status:       model.StatusUserModified,
				Confidence:   1.0,
				ClassifiedAt: time.Now(),
				Notes:        "NEW_CATEGORY|", // Empty after pipe = AI should generate
			},
			{
				Transaction:  txns[1],
				Category:     "Vacation",
				Status:       model.StatusUserModified,
				Confidence:   1.0,
				ClassifiedAt: time.Now(),
			},
		})

		// Create engine
		engine := New(db, classifier, prompter)

		// Get categories before
		categoriesBefore, catErr := db.GetCategories(ctx)
		require.NoError(t, catErr)
		categoryNames := make([]string, len(categoriesBefore))
		for i, cat := range categoriesBefore {
			categoryNames[i] = cat.Name
		}
		assert.NotContains(t, categoryNames, "Vacation", "Vacation should not exist yet")

		// Run batch classification
		opts := BatchClassificationOptions{
			AutoAcceptThreshold: 0.95,
			BatchSize:           5,
			ParallelWorkers:     1,
		}

		fromDate := time.Now().Add(-48 * time.Hour)
		summary, classifyErr := engine.ClassifyTransactionsBatch(ctx, &fromDate, opts)
		require.NoError(t, classifyErr)

		// Verify some transactions were processed
		assert.Greater(t, summary.TotalTransactions, 0)
		slog.Info("Classification summary",
			"total_merchants", summary.TotalMerchants,
			"total_transactions", summary.TotalTransactions,
			"auto_accepted", summary.AutoAcceptedCount,
			"needs_review", summary.NeedsReviewCount)

		// Verify the category was created with AI-generated description
		createdCategory, getCatErr := db.GetCategoryByName(ctx, "Vacation")
		require.NoError(t, getCatErr)
		assert.Equal(t, "Vacation", createdCategory.Name)
		assert.Equal(t, "Expenses related to travel, vacations, and leisure trips", createdCategory.Description)
		assert.Equal(t, model.CategoryTypeExpense, createdCategory.Type)

		// Verify classifications were saved
		savedClassifications, getClassErr := db.GetClassificationsByDateRange(ctx,
			time.Now().Add(-48*time.Hour), time.Now().Add(24*time.Hour))
		require.NoError(t, getClassErr)

		// Find our classifications
		found := 0
		for _, cls := range savedClassifications {
			if cls.Transaction.MerchantName == "Beach Resort" {
				assert.Equal(t, "Vacation", cls.Category)
				assert.Equal(t, model.StatusUserModified, cls.Status)
				found++
			}
		}
		assert.Equal(t, 2, found, "Should have found both classified transactions")
	})

	t.Run("user provides description directly", func(t *testing.T) {
		// Create test transaction
		txn := model.Transaction{
			ID:           "user-desc-1",
			Hash:         "hash-user-1",
			Date:         time.Now(),
			Name:         "Home Depot",
			MerchantName: "Home Depot",
			Amount:       150.00,
			AccountID:    "test-account",
			Direction:    model.DirectionExpense,
		}
		err = db.SaveTransactions(ctx, []model.Transaction{txn})
		require.NoError(t, err)

		// Set up mock classifier
		classifier := NewMockClassifier()

		// Set up mock prompter that returns user-provided description
		prompter := NewMockPrompter(false)
		userDescription := "Expenses for home repairs, renovations, and improvements"
		prompter.SetBatchResponse([]model.Classification{
			{
				Transaction:  txn,
				Category:     "Home Improvement",
				Status:       model.StatusUserModified,
				Confidence:   1.0,
				ClassifiedAt: time.Now(),
				Notes:        "NEW_CATEGORY|" + userDescription, // User provided description
			},
		})

		// Create engine
		engine := New(db, classifier, prompter)

		// Run batch classification
		opts := BatchClassificationOptions{
			AutoAcceptThreshold: 0.95,
			BatchSize:           5,
			ParallelWorkers:     1,
		}

		fromDate := time.Now().Add(-48 * time.Hour)
		_, err = engine.ClassifyTransactionsBatch(ctx, &fromDate, opts)
		require.NoError(t, err)

		// Verify the category was created with user's description
		createdCategory, err := db.GetCategoryByName(ctx, "Home Improvement")
		require.NoError(t, err)
		assert.Equal(t, "Home Improvement", createdCategory.Name)
		assert.Equal(t, userDescription, createdCategory.Description)
		assert.Equal(t, model.CategoryTypeExpense, createdCategory.Type)
	})
}
