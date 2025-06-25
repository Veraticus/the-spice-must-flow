package engine

import (
	"context"
	"testing"
	"time"

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
	}{
		{"Groceries", "Food and household supplies"},
		{"Entertainment", "Movies, games, and fun activities"},
		{"Transportation", "Gas, public transit, and vehicle expenses"},
		{"Other Income", "Miscellaneous income"},
	}

	for _, cat := range initialCategories {
		created, err := db.CreateCategory(ctx, cat.name, cat.description)
		require.NoError(t, err)
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
		Amount:       -50.00, // Negative = income
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

	// With all expense categories and a negative amount transaction,
	// no categories should be filtered out (expense categories are shown for negative amounts)
	assert.False(t, classifier.allCategoriesFiltered, "Categories should not be filtered out for negative amounts with expense categories")
}

// debuggingClassifier logs what categories it receives.
type debuggingClassifier struct {
	targetCategory        string
	categoriesReceived    []model.Category
	allCategoriesFiltered bool
}

func (c *debuggingClassifier) SuggestCategory(ctx context.Context, transaction model.Transaction, categories []string) (string, float64, bool, string, error) {
	return "", 0, false, "", nil
}

func (c *debuggingClassifier) SuggestCategoryRankings(ctx context.Context, transaction model.Transaction, categories []model.Category, checkPatterns []model.CheckPattern) (model.CategoryRankings, error) {
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

func (c *debuggingClassifier) BatchSuggestCategories(ctx context.Context, transactions []model.Transaction, categories []string) ([]service.LLMSuggestion, error) {
	return nil, nil
}

func (c *debuggingClassifier) GenerateCategoryDescription(ctx context.Context, categoryName string) (string, float64, error) {
	return "Test description", 0.95, nil
}

// simpleAcceptPrompter accepts all suggestions.
type simpleAcceptPrompter struct{}

func (p *simpleAcceptPrompter) ConfirmClassification(ctx context.Context, pending model.PendingClassification) (model.Classification, error) {
	return model.Classification{
		Transaction:  pending.Transaction,
		Category:     pending.SuggestedCategory,
		Status:       model.StatusUserModified,
		Confidence:   1.0,
		ClassifiedAt: time.Now(),
	}, nil
}

func (p *simpleAcceptPrompter) BatchConfirmClassifications(ctx context.Context, pending []model.PendingClassification) ([]model.Classification, error) {
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
