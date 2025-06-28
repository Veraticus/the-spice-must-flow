package engine

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBatchClassifier_NewCategoryCreation(t *testing.T) {
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
	_, err = db.CreateCategory(ctx, "Food", "Dining and groceries")
	require.NoError(t, err)
	_, err = db.CreateCategory(ctx, "Transport", "Transportation costs")
	require.NoError(t, err)

	t.Run("create new category during batch review", func(t *testing.T) {
		// Create test transactions
		txns := []model.Transaction{
			{
				ID:           "new-cat-1",
				Hash:         "hash-new-1",
				Date:         time.Now(),
				Name:         "Specialty Store",
				MerchantName: "Specialty Store",
				Amount:       50.00,
				AccountID:    "test-account",
				Direction:    model.DirectionExpense,
			},
			{
				ID:           "new-cat-2",
				Hash:         "hash-new-2",
				Date:         time.Now().Add(-24 * time.Hour),
				Name:         "Specialty Store",
				MerchantName: "Specialty Store",
				Amount:       75.00,
				AccountID:    "test-account",
				Direction:    model.DirectionExpense,
			},
		}
		err = db.SaveTransactions(ctx, txns)
		require.NoError(t, err)

		// Set up mock classifier that suggests a new category
		classifier := NewMockClassifier()
		classifier.SetBatchResponse(map[string]model.CategoryRankings{
			"Specialty Store": {
				{
					Category:    "Hobbies",
					Score:       0.85,
					IsNew:       true,
					Description: "Hobby and craft supplies",
				},
			},
		})

		// Set up mock prompter that simulates user selecting new category
		prompter := NewMockPrompter(false)
		prompter.SetBatchResponse([]model.Classification{
			{
				Transaction:  txns[0],
				Category:     "Hobbies",
				Status:       model.StatusUserModified,
				Confidence:   1.0,
				ClassifiedAt: time.Now(),
				Notes:        "NEW_CATEGORY|User provided description for hobbies",
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
		assert.NotContains(t, categoryNames, "Hobbies", "Hobbies should not exist yet")

		// Run batch classification
		opts := BatchClassificationOptions{
			AutoAcceptThreshold: 0.95, // Won't auto-accept 0.85 confidence
			BatchSize:           5,
			ParallelWorkers:     1,
			SkipManualReview:    false,
		}

		summary, classifyErr := engine.ClassifyTransactionsBatch(ctx, nil, opts)
		require.NoError(t, classifyErr)
		assert.Equal(t, 1, summary.TotalMerchants)
		assert.Equal(t, 2, summary.TotalTransactions)

		// Verify category was created
		categoriesAfter, catAfterErr := db.GetCategories(ctx)
		require.NoError(t, catAfterErr)

		var hobbiesCategory *model.Category
		for _, cat := range categoriesAfter {
			if cat.Name == "Hobbies" {
				hobbiesCategory = &cat
				break
			}
		}

		require.NotNil(t, hobbiesCategory, "Hobbies category should have been created")
		assert.Contains(t, hobbiesCategory.Description, "User provided description")

		// Verify transactions were classified
		classifications, err := db.GetClassificationsByDateRange(ctx,
			time.Now().Add(-48*time.Hour),
			time.Now().Add(24*time.Hour))
		require.NoError(t, err)

		classifiedCount := 0
		for _, c := range classifications {
			if c.Transaction.MerchantName == "Specialty Store" {
				assert.Equal(t, "Hobbies", c.Category)
				assert.Equal(t, model.StatusUserModified, c.Status)
				classifiedCount++
			}
		}
		assert.Equal(t, 2, classifiedCount, "Both transactions should be classified")
	})

	t.Run("handle new category with AI-generated description", func(t *testing.T) {
		// Create test transaction
		txn := model.Transaction{
			ID:           "new-cat-ai-1",
			Hash:         "hash-new-ai-1",
			Date:         time.Now(),
			Name:         "Gaming Store",
			MerchantName: "Gaming Store",
			Amount:       100.00,
			AccountID:    "test-account",
			Direction:    model.DirectionExpense,
		}
		err = db.SaveTransactions(ctx, []model.Transaction{txn})
		require.NoError(t, err)

		// Set up mock classifier that suggests a new category with description
		classifier := NewMockClassifier()
		classifier.SetBatchResponse(map[string]model.CategoryRankings{
			"Gaming Store": {
				{
					Category:    "Gaming",
					Score:       0.90,
					IsNew:       true,
					Description: "Video games and gaming accessories",
				},
			},
		})

		// Set up mock prompter that accepts the AI suggestion
		prompter := NewMockPrompter(false)
		prompter.SetBatchResponse([]model.Classification{
			{
				Transaction:  txn,
				Category:     "Gaming",
				Status:       model.StatusClassifiedByAI,
				Confidence:   0.90,
				ClassifiedAt: time.Now(),
			},
		})

		// Create engine
		engine := New(db, classifier, prompter)

		// This test directly calls handleBatchReview to test the new category creation

		// Mock the AI-suggested new category in the batch result
		results := []BatchResult{
			{
				Merchant:     "Gaming Store",
				Transactions: []model.Transaction{txn},
				Suggestion: &model.CategoryRanking{
					Category:    "Gaming",
					Score:       0.90,
					IsNew:       true,
					Description: "Video games and gaming accessories",
				},
			},
		}

		// Get current categories
		categories, catErr := db.GetCategories(ctx)
		require.NoError(t, catErr)

		// Call handleBatchReview directly to test the new category creation
		err = engine.handleBatchReview(ctx, results, categories)
		require.NoError(t, err)

		// Verify category was created with AI description
		gamingCat, getCatErr := db.GetCategoryByName(ctx, "Gaming")
		require.NoError(t, getCatErr)
		assert.NotNil(t, gamingCat)
		assert.Equal(t, "Video games and gaming accessories", gamingCat.Description)
	})

	t.Run("skip creating category if already exists", func(t *testing.T) {
		// This tests that we don't error out if user selects "new" category that already exists

		// Create test transaction
		txn := model.Transaction{
			ID:           "existing-cat-1",
			Hash:         "hash-existing-1",
			Date:         time.Now(),
			Name:         "Restaurant XYZ",
			MerchantName: "Restaurant XYZ",
			Amount:       30.00,
			AccountID:    "test-account",
			Direction:    model.DirectionExpense,
		}
		err = db.SaveTransactions(ctx, []model.Transaction{txn})
		require.NoError(t, err)

		// Set up mock prompter that tries to create "Food" (which already exists)
		prompter := NewMockPrompter(false)
		prompter.SetBatchResponse([]model.Classification{
			{
				Transaction:  txn,
				Category:     "Food",
				Status:       model.StatusUserModified,
				Confidence:   1.0,
				ClassifiedAt: time.Now(),
				Notes:        "NEW_CATEGORY|", // Signal new category but it already exists
			},
		})

		classifier := NewMockClassifier()
		engine := New(db, classifier, prompter)

		// Get current categories
		categories, catErr := db.GetCategories(ctx)
		require.NoError(t, catErr)

		// Create batch result
		results := []BatchResult{
			{
				Merchant:     "Restaurant XYZ",
				Transactions: []model.Transaction{txn},
				Suggestion:   nil, // No AI suggestion
			},
		}

		// Should not error even though trying to create existing category
		err = engine.handleBatchReview(ctx, results, categories)
		assert.NoError(t, err)

		// Verify transaction was classified
		classifications, err := db.GetClassificationsByDateRange(ctx,
			time.Now().Add(-24*time.Hour),
			time.Now().Add(24*time.Hour))
		require.NoError(t, err)

		found := false
		for _, c := range classifications {
			if c.Transaction.ID == txn.ID {
				found = true
				assert.Equal(t, "Food", c.Category)
				assert.Equal(t, model.StatusUserModified, c.Status)
				break
			}
		}
		assert.True(t, found, "Transaction should be classified")
	})
}
