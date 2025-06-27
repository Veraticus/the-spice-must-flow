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

// TestCategoryStillMarkedAsNew tests that in batch mode, all merchants in the same batch
// see the same category snapshot, so a new category appears as "new" to all of them.
func TestCategoryStillMarkedAsNew(t *testing.T) {
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

	// Create a classifier that ALWAYS suggests the same new category
	// This simulates an LLM that consistently suggests "Personal Transfers and Reimbursements"
	// for all VENMO transactions
	classifier := &alwaysNewCategoryClassifier{
		newCategoryName: "Personal Transfers and Reimbursements",
		callCount:       0,
	}

	// Create a prompter that tracks what was suggested
	prompter := &trackingSuggestionsPrompter{
		suggestionsReceived: make([]model.PendingClassification, 0),
	}

	engine := New(db, classifier, prompter)

	// Add multiple VENMO transactions from different merchants
	// These should all get the same category suggestion
	txns := []model.Transaction{
		{
			ID:           "1",
			Hash:         "hash1",
			MerchantName: "VENMO CASHOUT 1",
			Name:         "VENMO CASHOUT",
			Amount:       -100.00,
			Date:         time.Now(),
			Type:         "TRANSFER",
			AccountID:    "acc1",
		},
		{
			ID:           "2",
			Hash:         "hash2",
			MerchantName: "VENMO PAYMENT 1",
			Name:         "VENMO PAYMENT John Doe",
			Amount:       -50.00,
			Date:         time.Now(),
			Type:         "TRANSFER",
			AccountID:    "acc1",
		},
		{
			ID:           "3",
			Hash:         "hash3",
			MerchantName: "VENMO PAYMENT 2",
			Name:         "VENMO PAYMENT Jane Smith",
			Amount:       -75.00,
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

	// Verify the results
	assert.Equal(t, 3, len(prompter.suggestionsReceived), "Should have received 3 suggestions")

	// Check each suggestion
	for i, suggestion := range prompter.suggestionsReceived {
		t.Logf("Suggestion %d: Category=%s, IsNew=%v, Merchant=%s",
			i+1, suggestion.SuggestedCategory, suggestion.IsNewCategory,
			suggestion.Transaction.MerchantName)

		// In batch mode, all merchants in the same batch see the same category snapshot
		// So they will all see the category as new since it doesn't exist at the start
		assert.True(t, suggestion.IsNewCategory,
			"In batch mode, all merchants in the batch see the category as new")
	}

	// Also verify what the classifier saw
	t.Logf("Classifier was called %d times", classifier.callCount)
	for i, cats := range classifier.categoriesSeenPerCall {
		t.Logf("Call %d saw %d categories: %v", i+1, len(cats), cats)
	}
}

// alwaysNewCategoryClassifier is a test classifier that always suggests the same
// category and tracks what categories it was given.
type alwaysNewCategoryClassifier struct {
	newCategoryName       string
	categoriesSeenPerCall [][]string
	callCount             int
}

func (c *alwaysNewCategoryClassifier) SuggestCategory(_ context.Context, _ model.Transaction, _ []string) (string, float64, bool, string, error) {
	return "", 0, false, "", nil
}

func (c *alwaysNewCategoryClassifier) SuggestCategoryRankings(_ context.Context, _ model.Transaction, categories []model.Category, _ []model.CheckPattern) (model.CategoryRankings, error) {
	c.callCount++

	// Track what categories we saw
	categoryNames := make([]string, len(categories))
	for i, cat := range categories {
		categoryNames[i] = cat.Name
	}
	c.categoriesSeenPerCall = append(c.categoriesSeenPerCall, categoryNames)

	// Check if our target category exists
	hasTargetCategory := false
	for _, cat := range categories {
		if cat.Name == c.newCategoryName {
			hasTargetCategory = true
			break
		}
	}

	// Always suggest our target category with medium confidence
	// Use 0.8 which is below the 0.85 auto-classification threshold
	rankings := model.CategoryRankings{
		{
			Category:    c.newCategoryName,
			Score:       0.80, // Medium confidence - requires user confirmation
			IsNew:       !hasTargetCategory,
			Description: "Personal money transfers and reimbursements",
		},
	}

	// Add other categories with low scores
	for _, cat := range categories {
		if cat.Name != c.newCategoryName {
			rankings = append(rankings, model.CategoryRanking{
				Category: cat.Name,
				Score:    0.05,
				IsNew:    false,
			})
		}
	}

	return rankings, nil
}

func (c *alwaysNewCategoryClassifier) BatchSuggestCategories(_ context.Context, transactions []model.Transaction, categories []string) ([]service.LLMSuggestion, error) {
	// For this test, we need to return suggestions for each transaction
	suggestions := make([]service.LLMSuggestion, 0, len(transactions))

	for _, txn := range transactions {
		c.callCount++

		// Track what categories we saw
		c.categoriesSeenPerCall = append(c.categoriesSeenPerCall, categories)

		// Check if our target category exists
		hasTargetCategory := false
		for _, cat := range categories {
			if cat == c.newCategoryName {
				hasTargetCategory = true
				break
			}
		}

		// Always suggest our target category with medium confidence
		suggestions = append(suggestions, service.LLMSuggestion{
			TransactionID:       txn.ID,
			Category:            c.newCategoryName,
			Confidence:          0.80, // Below auto-classification threshold
			IsNew:               !hasTargetCategory,
			CategoryDescription: "Personal money transfers and reimbursements",
		})
	}

	return suggestions, nil
}

func (c *alwaysNewCategoryClassifier) SuggestCategoryBatch(ctx context.Context, requests []llm.MerchantBatchRequest, categories []model.Category) (map[string]model.CategoryRankings, error) {
	// Create rankings for each merchant
	results := make(map[string]model.CategoryRankings)

	for _, req := range requests {
		// Use the same logic as SuggestCategoryRankings
		rankings, err := c.SuggestCategoryRankings(ctx, req.SampleTransaction, categories, nil)
		if err != nil {
			return nil, err
		}
		results[req.MerchantID] = rankings
	}

	return results, nil
}

func (c *alwaysNewCategoryClassifier) GenerateCategoryDescription(_ context.Context, categoryName string) (string, float64, error) {
	return "Test description for " + categoryName, 0.95, nil
}

// trackingSuggestionsPrompter tracks all suggestions it receives.
type trackingSuggestionsPrompter struct {
	suggestionsReceived []model.PendingClassification
}

func (p *trackingSuggestionsPrompter) ConfirmClassification(_ context.Context, _ model.PendingClassification) (model.Classification, error) {
	// Should not be called in this test - we're testing batch mode
	panic("ConfirmClassification should not be called in batch mode")
}

func (p *trackingSuggestionsPrompter) BatchConfirmClassifications(_ context.Context, pending []model.PendingClassification) ([]model.Classification, error) {
	// Track what we received
	p.suggestionsReceived = append(p.suggestionsReceived, pending...)

	// Accept all suggestions
	classifications := make([]model.Classification, len(pending))
	for i, pc := range pending {
		classifications[i] = model.Classification{
			Transaction:  pc.Transaction,
			Category:     pc.SuggestedCategory,
			Status:       model.StatusUserModified, // User "accepted" the suggestion
			Confidence:   1.0,
			ClassifiedAt: time.Now(),
		}
	}

	return classifications, nil
}

func (p *trackingSuggestionsPrompter) GetCompletionStats() service.CompletionStats {
	return service.CompletionStats{}
}
