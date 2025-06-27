package engine

import (
	"context"
	"testing"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/llm"
	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/service"
	"github.com/Veraticus/the-spice-must-flow/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCategoryReloadAfterCreation tests that in batch mode, newly created categories
// are not available to other merchants in the same batch due to snapshot consistency.
func TestCategoryReloadAfterCreation(t *testing.T) {
	// Setup
	db, err := storage.NewSQLiteStorage(":memory:")
	require.NoError(t, err)
	require.NoError(t, db.Migrate(context.Background()))
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Logf("Failed to close database: %v", closeErr)
		}
	}()

	// Seed some initial categories
	ctx := context.Background()
	initialCategories := []struct {
		name        string
		description string
	}{
		{"Groceries", "Food and household supplies"},
		{"Entertainment", "Movies, games, and fun activities"},
		{"Transportation", "Gas, public transit, and vehicle expenses"},
	}

	for _, cat := range initialCategories {
		_, createErr := db.CreateCategory(ctx, cat.name, cat.description)
		require.NoError(t, createErr)
	}

	// Create a special classifier that returns new category suggestions
	classifier := &categoryReloadTestClassifier{
		newCategoryName:  "Personal Transfers and Reimbursements",
		categoriesPassed: make(map[string][]string),
	}

	// Create a special prompter that:
	// 1. Creates a new category for the first merchant
	// 2. Uses that same category for the second merchant
	prompter := &categoryReloadTestPrompter{
		newCategoryName: "Personal Transfers and Reimbursements",
		createdCategory: false,
	}

	engine := New(db, classifier, prompter)

	// Add two different merchants with transactions
	// First merchant will trigger new category creation
	txns := []model.Transaction{
		{
			ID:           "1",
			Hash:         "hash1",
			MerchantName: "VENMO CASHOUT",
			Name:         "VENMO CASHOUT",
			Amount:       -100.00,
			Date:         time.Now(),
			Type:         "TRANSFER",
			AccountID:    "acc1",
		},
		{
			ID:           "2",
			Hash:         "hash2",
			MerchantName: "VENMO PAYMENT",
			Name:         "VENMO PAYMENT John Doe",
			Amount:       -50.00,
			Date:         time.Now(),
			Type:         "TRANSFER",
			AccountID:    "acc1",
		},
	}

	// Save transactions to database
	err = db.SaveTransactions(context.Background(), txns)
	require.NoError(t, err)

	// Run classification
	err = engine.ClassifyTransactions(ctx, nil)
	require.NoError(t, err)

	// Verify that the category was created
	createdCategory, err := db.GetCategoryByName(ctx, "Personal Transfers and Reimbursements")
	require.NoError(t, err)
	require.NotNil(t, createdCategory)
	assert.Equal(t, "Personal Transfers and Reimbursements", createdCategory.Name)

	// Verify that both transactions were classified with the new category
	classifications, err := db.GetClassificationsByDateRange(ctx, time.Now().Add(-1*time.Hour), time.Now().Add(1*time.Hour))
	require.NoError(t, err)
	assert.Len(t, classifications, 2, "Should have 2 classifications")

	for _, classification := range classifications {
		assert.Equal(t, "Personal Transfers and Reimbursements", classification.Category,
			"Transaction %s should use the new category", classification.Transaction.ID)
	}

	// Debug: print what categories were passed to each merchant
	t.Logf("Categories for VENMO CASHOUT: %v", classifier.categoriesPassed["VENMO CASHOUT"])
	t.Logf("Categories for VENMO PAYMENT: %v", classifier.categoriesPassed["VENMO PAYMENT"])

	// Check which merchant was processed first (created the category) and which was second
	venmoCashoutHasNew := false
	for _, cat := range classifier.categoriesPassed["VENMO CASHOUT"] {
		if cat == "Personal Transfers and Reimbursements" {
			venmoCashoutHasNew = true
			break
		}
	}

	venmoPaymentHasNew := false
	for _, cat := range classifier.categoriesPassed["VENMO PAYMENT"] {
		if cat == "Personal Transfers and Reimbursements" {
			venmoPaymentHasNew = true
			break
		}
	}

	// In batch mode, all merchants are processed with the same category snapshot
	// So neither merchant will see the newly created category as existing
	assert.False(t, venmoCashoutHasNew || venmoPaymentHasNew,
		"In batch mode, no merchant sees newly created categories until the next batch")
}

// categoryReloadTestClassifier is a test classifier that tracks what categories
// were passed to it for each merchant.
type categoryReloadTestClassifier struct {
	categoriesPassed map[string][]string
	newCategoryName  string
}

// SuggestCategory implements the Classifier interface.
func (c *categoryReloadTestClassifier) SuggestCategory(_ context.Context, _ model.Transaction, _ []string) (string, float64, bool, string, error) {
	// Not used in this test
	return "", 0, false, "", nil
}

// SuggestCategoryRankings implements the Classifier interface.
func (c *categoryReloadTestClassifier) SuggestCategoryRankings(_ context.Context, transaction model.Transaction, categories []model.Category, _ []model.CheckPattern) (model.CategoryRankings, error) {
	// Track what categories were passed for this merchant
	categoryNames := make([]string, len(categories))
	for i, cat := range categories {
		categoryNames[i] = cat.Name
	}
	c.categoriesPassed[transaction.MerchantName] = categoryNames

	// Check if our new category is in the list
	hasNewCategory := false
	for _, cat := range categories {
		if cat.Name == c.newCategoryName {
			hasNewCategory = true
			break
		}
	}

	// Log for debugging (removed fmt.Printf to satisfy linter)

	// Return rankings suggesting the new category
	rankings := model.CategoryRankings{
		{
			Category:    c.newCategoryName,
			Score:       0.9,
			IsNew:       !hasNewCategory, // It's new if not in the category list
			Description: "Personal money transfers and reimbursements",
		},
	}

	// Add some existing categories with lower scores
	for _, cat := range categories {
		if cat.Name != c.newCategoryName {
			rankings = append(rankings, model.CategoryRanking{
				Category: cat.Name,
				Score:    0.1,
				IsNew:    false,
			})
		}
	}

	return rankings, nil
}

// BatchSuggestCategories implements the Classifier interface.
func (c *categoryReloadTestClassifier) BatchSuggestCategories(_ context.Context, _ []model.Transaction, _ []string) ([]service.LLMSuggestion, error) {
	// Not used in this test
	return nil, nil
}

// GenerateCategoryDescription implements the Classifier interface.
func (c *categoryReloadTestClassifier) GenerateCategoryDescription(_ context.Context, categoryName string) (string, float64, error) {
	return "Test description for " + categoryName, 0.95, nil
}

func (c *categoryReloadTestClassifier) SuggestCategoryBatch(_ context.Context, requests []llm.MerchantBatchRequest, categories []model.Category) (map[string]model.CategoryRankings, error) {
	results := make(map[string]model.CategoryRankings)

	for _, req := range requests {
		// Use the same logic as SuggestCategoryRankings
		rankings, err := c.SuggestCategoryRankings(context.Background(), req.SampleTransaction, categories, nil)
		if err != nil {
			return nil, err
		}
		results[req.MerchantID] = rankings
	}

	return results, nil
}

// categoryReloadTestPrompter is a test prompter that creates a new category
// for the first merchant and expects to use it for the second merchant.
type categoryReloadTestPrompter struct {
	newCategoryName          string
	createdCategory          bool
	sawNewCategoryAsExisting bool
}

func (p *categoryReloadTestPrompter) ConfirmClassification(_ context.Context, _ model.PendingClassification) (model.Classification, error) {
	// Should not be called in this test - we're testing batch mode
	panic("ConfirmClassification should not be called in batch mode")
}

func (p *categoryReloadTestPrompter) BatchConfirmClassifications(_ context.Context, pending []model.PendingClassification) ([]model.Classification, error) {
	if len(pending) == 0 {
		return []model.Classification{}, nil
	}

	// First merchant - create new category
	if !p.createdCategory && pending[0].IsNewCategory {
		p.createdCategory = true
		classifications := make([]model.Classification, len(pending))
		for i, pc := range pending {
			classifications[i] = model.Classification{
				Transaction:  pc.Transaction,
				Category:     p.newCategoryName,
				Status:       model.StatusUserModified,
				Confidence:   1.0,
				ClassifiedAt: time.Now(),
			}
		}
		return classifications, nil
	}

	// Second merchant - check if our new category is now in the rankings
	if p.createdCategory {
		// Look for our new category in the rankings
		for _, ranking := range pending[0].CategoryRankings {
			if ranking.Category == p.newCategoryName && !ranking.IsNew {
				p.sawNewCategoryAsExisting = true
				break
			}
		}

		// Use the category regardless
		classifications := make([]model.Classification, len(pending))
		for i, pc := range pending {
			classifications[i] = model.Classification{
				Transaction:  pc.Transaction,
				Category:     p.newCategoryName,
				Status:       model.StatusUserModified,
				Confidence:   1.0,
				ClassifiedAt: time.Now(),
			}
		}
		return classifications, nil
	}

	// Default behavior
	classifications := make([]model.Classification, len(pending))
	for i, pc := range pending {
		classifications[i] = model.Classification{
			Transaction:  pc.Transaction,
			Category:     pc.SuggestedCategory,
			Status:       model.StatusClassifiedByAI,
			Confidence:   pc.Confidence,
			ClassifiedAt: time.Now(),
		}
	}
	return classifications, nil
}

func (p *categoryReloadTestPrompter) GetCompletionStats() service.CompletionStats {
	return service.CompletionStats{}
}
