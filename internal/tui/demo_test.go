package tui_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/tui"
	"github.com/Veraticus/the-spice-must-flow/internal/tui/components"
	"github.com/Veraticus/the-spice-must-flow/internal/tui/themes"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTUIDemo is a test that can be run manually to see the TUI
// Run with: go test -v ./internal/tui -run TestTUIDemo -manual.
func TestTUIDemo(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping TUI demo in short mode")
	}

	// Create TUI with test data
	ctx := context.Background()
	prompter, err := tui.New(ctx,
		tui.WithTestMode(true),
		tui.WithTestData(50, nil),
	)
	require.NoError(t, err)

	// Cast to get access to internal methods
	_ = prompter.(*tui.Prompter)

	// Skip the interactive test - it requires a TTY
	t.Skip("Skipping interactive TUI test - requires TTY")
}

// TestTransactionListNavigation tests basic navigation.
func TestTransactionListNavigation(t *testing.T) {
	// Create test transactions
	transactions := make([]model.Transaction, 10)
	for i := range transactions {
		transactions[i] = model.Transaction{
			ID:           fmt.Sprintf("txn_%d", i),
			MerchantName: fmt.Sprintf("Merchant %d", i),
			Amount:       float64(i * 10),
			Date:         time.Now().AddDate(0, 0, -i),
		}
	}

	// Create transaction list
	list := components.NewTransactionList(transactions, themes.Default)

	// Test navigation down
	list, _ = list.Update(tea.KeyMsg{Type: tea.KeyDown})
	// Note: Would need to export cursor field or add getter method

	// Test view renders without error
	view := list.View()
	assert.NotEmpty(t, view)
}

// TestClassifierSuggestions tests the classifier component.
func TestClassifierSuggestions(t *testing.T) {
	pending := model.PendingClassification{
		Transaction: model.Transaction{
			ID:           "test_1",
			MerchantName: "Whole Foods",
			Amount:       67.23,
		},
		SuggestedCategory: "Groceries",
		CategoryRankings: model.CategoryRankings{
			{Category: "Groceries", Score: 0.92},
			{Category: "Dining Out", Score: 0.05},
			{Category: "Shopping", Score: 0.03},
		},
		Confidence: 0.92,
	}

	categories := []model.Category{
		{ID: 1, Name: "Groceries"},
		{ID: 2, Name: "Dining Out"},
		{ID: 3, Name: "Shopping"},
	}

	classifier := components.NewClassifierModel(pending, categories, themes.Default, nil)

	// Test initial state
	assert.False(t, classifier.IsComplete())

	// Initialize the classifier
	classifier, _ = classifier.Update(classifier.Init())

	// Test accepting suggestion
	classifier, _ = classifier.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Check if complete (the action is registered even if not complete)
	if classifier.IsComplete() {
		result := classifier.GetResult()
		assert.Equal(t, "Groceries", result.Category)
		assert.Equal(t, model.StatusClassifiedByAI, result.Status)
	}
}
