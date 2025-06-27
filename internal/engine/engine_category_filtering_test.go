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

// TestCategoryFilteringIssue tests if category filtering is causing the "new category" bug.
func TestCategoryFilteringIssue(t *testing.T) {
	// Setup
	db, err := storage.NewSQLiteStorage(":memory:")
	require.NoError(t, err)
	require.NoError(t, db.Migrate(context.Background()))
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Logf("Failed to close database: %v", closeErr)
		}
	}()

	ctx := context.Background()

	// Create initial categories WITHOUT type (mimicking migration behavior)
	// These will default to empty type or expense
	initialCategories := []struct {
		name        string
		description string
		catType     model.CategoryType
	}{
		{"Groceries", "Food and household supplies", model.CategoryTypeExpense},
		{"Entertainment", "Movies, games, and fun activities", model.CategoryTypeExpense},
		{"Transportation", "Gas, public transit, and vehicle expenses", model.CategoryTypeExpense},
		{"Other Income", "Miscellaneous income", model.CategoryTypeIncome},
	}

	for _, cat := range initialCategories {
		created, createErr := db.CreateCategoryWithType(ctx, cat.name, cat.description, cat.catType)
		require.NoError(t, createErr)
		t.Logf("Created category: %s (type: %s)", created.Name, created.Type)
	}

	// Create a debugging classifier that logs what categories it receives
	classifier := &debuggingClassifier{
		targetCategory: "Personal Transfers and Reimbursements",
	}

	// Simple prompter that accepts everything
	prompter := &simpleAcceptPrompter{}

	engine := New(db, classifier, prompter)

	// Create a VENMO transaction with negative amount (income-like)
	txn := model.Transaction{
		ID:           "1",
		Hash:         "hash1",
		MerchantName: "VENMO PAYMENT",
		Name:         "VENMO PAYMENT John Doe",
		Amount:       -50.00, // Negative amount represents income
		Date:         time.Now(),
		Type:         "TRANSFER",
		AccountID:    "acc1",
	}

	err = db.SaveTransactions(ctx, []model.Transaction{txn})
	require.NoError(t, err)

	// Run classification
	err = engine.ClassifyTransactions(ctx, nil)
	require.NoError(t, err)

	// Check what the classifier saw
	assert.NotEmpty(t, classifier.categoriesReceived)
	t.Logf("Classifier received %d categories", len(classifier.categoriesReceived))
	for _, cat := range classifier.categoriesReceived {
		t.Logf("  - %s (type: %s)", cat.Name, cat.Type)
	}

	// With a negative amount transaction (income), only income categories should be shown
	// We created 3 expense categories and 1 income category, so only 1 should be shown
	assert.Equal(t, 1, len(classifier.categoriesReceived), "Should only receive income categories for negative amount")
	assert.Equal(t, "Other Income", classifier.categoriesReceived[0].Name, "Should receive the income category")
}

// debuggingClassifier logs what categories it receives.
type debuggingClassifier struct {
	targetCategory        string
	categoriesReceived    []model.Category
	allCategoriesFiltered bool
}

func (c *debuggingClassifier) SuggestCategory(_ context.Context, _ model.Transaction, _ []string) (string, float64, bool, string, error) {
	return "", 0, false, "", nil
}

func (c *debuggingClassifier) SuggestCategoryRankings(_ context.Context, _ model.Transaction, categories []model.Category, _ []model.CheckPattern) (model.CategoryRankings, error) {
	c.categoriesReceived = categories

	// Check if we got very few categories (likely due to filtering)
	if len(categories) <= 1 {
		c.allCategoriesFiltered = true
	}

	// Check if our target category exists
	hasTarget := false
	for _, cat := range categories {
		if cat.Name == c.targetCategory {
			hasTarget = true
			break
		}
	}

	return model.CategoryRankings{
		{
			Category:    c.targetCategory,
			Score:       0.8,
			IsNew:       !hasTarget,
			Description: "Personal transfers",
		},
	}, nil
}

func (c *debuggingClassifier) BatchSuggestCategories(_ context.Context, _ []model.Transaction, _ []string) ([]service.LLMSuggestion, error) {
	return nil, nil
}

func (c *debuggingClassifier) SuggestCategoryBatch(_ context.Context, requests []llm.MerchantBatchRequest, categories []model.Category) (map[string]model.CategoryRankings, error) {
	c.categoriesReceived = categories

	// Check if we got very few categories (likely due to filtering)
	if len(categories) <= 1 {
		c.allCategoriesFiltered = true
	}

	// Check if our target category exists
	hasTarget := false
	for _, cat := range categories {
		if cat.Name == c.targetCategory {
			hasTarget = true
			break
		}
	}

	results := make(map[string]model.CategoryRankings)
	for _, req := range requests {
		results[req.MerchantID] = model.CategoryRankings{
			{
				Category:    c.targetCategory,
				Score:       0.8,
				IsNew:       !hasTarget,
				Description: "Personal transfers",
			},
		}
	}

	return results, nil
}

func (c *debuggingClassifier) GenerateCategoryDescription(_ context.Context, _ string) (string, float64, error) {
	return "Test description", 0.95, nil
}

// simpleAcceptPrompter accepts all suggestions.
type simpleAcceptPrompter struct{}

func (p *simpleAcceptPrompter) ConfirmClassification(_ context.Context, pending model.PendingClassification) (model.Classification, error) {
	return model.Classification{
		Transaction:  pending.Transaction,
		Category:     pending.SuggestedCategory,
		Status:       model.StatusUserModified,
		Confidence:   1.0,
		ClassifiedAt: time.Now(),
	}, nil
}

func (p *simpleAcceptPrompter) BatchConfirmClassifications(_ context.Context, pending []model.PendingClassification) ([]model.Classification, error) {
	classifications := make([]model.Classification, len(pending))
	for i, pc := range pending {
		classifications[i] = model.Classification{
			Transaction:  pc.Transaction,
			Category:     pc.SuggestedCategory,
			Status:       model.StatusUserModified,
			Confidence:   1.0,
			ClassifiedAt: time.Now(),
		}
	}
	return classifications, nil
}

func (p *simpleAcceptPrompter) GetCompletionStats() service.CompletionStats {
	return service.CompletionStats{}
}
