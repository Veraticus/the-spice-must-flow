package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/tui/components"
	"github.com/Veraticus/the-spice-must-flow/internal/tui/themes"
	tea "github.com/charmbracelet/bubbletea"
)

// TestVisualOutput captures TUI output for viewing.
func TestVisualOutput(t *testing.T) {
	// Create model with test data
	m := newModel(Config{
		TestMode: true,
		Width:    120,
		Height:   40,
		Theme:    themes.Default,
	})

	// Generate test data
	m.transactions = generateTestTransactions(50)
	m.categories = generateTestCategories()
	m.ready = true

	// Initialize components
	m.transactionList = components.NewTransactionList(m.transactions, m.theme)
	m.statsPanel = components.NewStatsPanelModel(m.theme)
	m.statsPanel.SetTotal(len(m.transactions))

	// Test different views
	tests := []struct {
		setup  func()
		name   string
		state  State
		width  int
		height int
	}{
		{
			name:   "transaction_list_full",
			state:  StateList,
			width:  120,
			height: 40,
		},
		{
			name:   "transaction_list_compact",
			state:  StateList,
			width:  80,
			height: 24,
		},
		{
			name:   "classification_view",
			state:  StateClassifying,
			width:  120,
			height: 40,
			setup: func() {
				m.pending = []model.PendingClassification{{
					Transaction:       m.transactions[0],
					SuggestedCategory: "Groceries",
					CategoryRankings: model.CategoryRankings{
						{Category: "Groceries", Score: 0.92},
						{Category: "Shopping", Score: 0.45},
						{Category: "Dining Out", Score: 0.12},
					},
					Confidence: 0.92,
				}}
				m.classifier = components.NewClassifierModel(
					m.pending[0],
					m.categories,
					m.theme,
					nil,
				)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test
			m.state = tt.state
			m.width = tt.width
			m.height = tt.height

			if tt.setup != nil {
				tt.setup()
			}

			// Render view
			output := m.View()

			// Log output for manual inspection
			fmt.Printf("\n=== %s ===\n", tt.name)
			fmt.Printf("Size: %dx%d\n", tt.width, tt.height)
			fmt.Printf("%s\n", output)
			fmt.Println(strings.Repeat("=", 60))
		})
	}
}

// TestComponentViews tests individual component rendering.
func TestComponentViews(t *testing.T) {
	theme := themes.Default

	t.Run("transaction_list", func(t *testing.T) {
		transactions := generateTestTransactions(20)
		list := components.NewTransactionList(transactions, theme)
		list.Resize(80, 30)

		// Simulate navigation
		list.Update(tea.KeyMsg{Type: tea.KeyDown})
		list.Update(tea.KeyMsg{Type: tea.KeyDown})

		output := list.View()
		fmt.Printf("\n=== Transaction List ===\n%s\n", output)
	})

	t.Run("classifier", func(t *testing.T) {
		pending := model.PendingClassification{
			Transaction: model.Transaction{
				ID:           "test_1",
				MerchantName: "Whole Foods Market",
				Amount:       67.23,
				Date:         generateTestTransactions(1)[0].Date,
			},
			SuggestedCategory: "Groceries",
			CategoryRankings: model.CategoryRankings{
				{Category: "Groceries", Score: 0.92},
				{Category: "Shopping", Score: 0.45},
				{Category: "Dining Out", Score: 0.12},
				{Category: "Healthcare", Score: 0.05},
				{Category: "Other", Score: 0.02},
			},
			Confidence:          0.92,
			CategoryDescription: "Regular grocery shopping at a supermarket chain",
		}

		classifier := components.NewClassifierModel(pending, generateTestCategories(), theme, nil)
		classifier.Resize(80, 30)

		output := classifier.View()
		fmt.Printf("\n=== Classifier (Suggestion Mode) ===\n%s\n", output)

		// Simulate pressing 'c' to show category picker
		classifier, _ = classifier.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})

		output = classifier.View()
		fmt.Printf("\n=== Classifier (Category Picker) ===\n%s\n", output)

		// Simulate scrolling down a few times
		for i := 0; i < 5; i++ {
			classifier, _ = classifier.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		}

		output = classifier.View()
		fmt.Printf("\n=== Classifier (Category Picker - Scrolled) ===\n%s\n", output)

		// Simulate typing a category ID number
		classifier, _ = classifier.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})

		output = classifier.View()
		fmt.Printf("\n=== Classifier (Entering Category ID) ===\n%s\n", output)
	})

	t.Run("stats_panel", func(t *testing.T) {
		stats := components.NewStatsPanelModel(theme)
		stats.SetTotal(100)
		stats.Resize(40, 30)

		// Simulate some classifications
		for i := 0; i < 30; i++ {
			classification := model.Classification{
				Category: "Groceries",
				Status:   model.StatusClassifiedByAI,
			}
			stats.Update(components.ClassificationCompleteMsg{
				Classification: classification,
			})
		}

		output := stats.View()
		fmt.Printf("\n=== Stats Panel ===\n%s\n", output)
	})

	t.Run("search_mode", func(t *testing.T) {
		list := components.NewTransactionList(generateTestTransactions(10), theme)
		list.Resize(80, 30)

		// Enter search mode
		list.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})

		output := list.View()
		fmt.Printf("\n=== Search Mode ===\n%s\n", output)
	})
}

// TestThemeVariations tests different themes.
func TestThemeVariations(t *testing.T) {
	themes := []struct {
		theme themes.Theme
		name  string
	}{
		{themes.Default, "default"},
		{themes.CatppuccinMocha, "catppuccin"},
	}

	for _, th := range themes {
		t.Run(th.name, func(t *testing.T) {
			list := components.NewTransactionList(generateTestTransactions(5), th.theme)
			list.Resize(80, 20)

			output := list.View()
			fmt.Printf("\n=== Theme: %s ===\n%s\n", th.name, output)
		})
	}
}
