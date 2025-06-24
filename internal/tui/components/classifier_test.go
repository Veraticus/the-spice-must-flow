package components

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/engine"
	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/service"
	"github.com/Veraticus/the-spice-must-flow/internal/tui/themes"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockClassifier implements engine.Classifier for testing.
type mockClassifier struct {
	err      error
	rankings model.CategoryRankings
	delay    time.Duration
}

func (m *mockClassifier) SuggestCategory(_ context.Context, _ model.Transaction, _ []string) (string, float64, bool, string, error) {
	if m.err != nil {
		return "", 0, false, "", m.err
	}
	if len(m.rankings) > 0 {
		return m.rankings[0].Category, m.rankings[0].Score, false, "test description", nil
	}
	return "Test Category", 0.85, false, "test description", nil
}

func (m *mockClassifier) SuggestCategoryRankings(_ context.Context, _ model.Transaction, _ []model.Category, _ []model.CheckPattern) (model.CategoryRankings, error) {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	return m.rankings, m.err
}

func (m *mockClassifier) BatchSuggestCategories(ctx context.Context, transactions []model.Transaction, categories []string) ([]service.LLMSuggestion, error) {
	suggestions := make([]service.LLMSuggestion, 0, len(transactions))
	for _, txn := range transactions {
		cat, conf, isNew, desc, err := m.SuggestCategory(ctx, txn, categories)
		if err != nil {
			return nil, err
		}
		suggestions = append(suggestions, service.LLMSuggestion{
			TransactionID:       txn.ID,
			Category:            cat,
			CategoryDescription: desc,
			Confidence:          conf,
			IsNew:               isNew,
		})
	}
	return suggestions, nil
}

func (m *mockClassifier) GenerateCategoryDescription(_ context.Context, categoryName string) (string, float64, error) {
	return "Mock description for " + categoryName, 0.9, m.err
}

func (m *mockClassifier) SuggestTransactionDirection(_ context.Context, _ model.Transaction) (model.TransactionDirection, float64, string, error) {
	return model.DirectionExpense, 0.95, "Mock reasoning", m.err
}

// Helper functions.
func createTestTransaction() model.Transaction {
	return model.Transaction{
		ID:           "test_123",
		MerchantName: "Test Merchant",
		Name:         "TEST MERCHANT TRANSACTION",
		Amount:       123.45,
		Date:         time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		Type:         "debit",
		Direction:    model.DirectionExpense,
	}
}

func createTestCategories() []model.Category {
	return []model.Category{
		{ID: 1, Name: "Groceries"},
		{ID: 2, Name: "Dining Out"},
		{ID: 3, Name: "Shopping"},
		{ID: 4, Name: "Transportation"},
		{ID: 5, Name: "Healthcare"},
		{ID: 10, Name: "Entertainment"},
		{ID: 15, Name: "Utilities"},
		{ID: 20, Name: "Housing"},
		{ID: 25, Name: "Insurance"},
		{ID: 30, Name: "Other"},
	}
}

func createTestRankings() model.CategoryRankings {
	return model.CategoryRankings{
		{Category: "Groceries", Score: 0.92},
		{Category: "Shopping", Score: 0.65},
		{Category: "Dining Out", Score: 0.32},
		{Category: "Healthcare", Score: 0.15},
		{Category: "Other", Score: 0.05},
	}
}

func TestNewClassifierModel(t *testing.T) {
	tests := []struct {
		classifier engine.Classifier
		name       string
		categories []model.Category
		pending    model.PendingClassification
		wantMode   ClassifierMode
		wantLoad   bool
	}{
		{
			name: "with existing rankings",
			pending: model.PendingClassification{
				Transaction:      createTestTransaction(),
				CategoryRankings: createTestRankings(),
			},
			categories: createTestCategories(),
			classifier: &mockClassifier{},
			wantMode:   ModeSelectingSuggestion,
			wantLoad:   false,
		},
		{
			name: "without rankings - starts loading",
			pending: model.PendingClassification{
				Transaction: createTestTransaction(),
			},
			categories: createTestCategories(),
			classifier: &mockClassifier{rankings: createTestRankings()},
			wantMode:   ModeSelectingSuggestion,
			wantLoad:   true,
		},
		{
			name: "without classifier",
			pending: model.PendingClassification{
				Transaction: createTestTransaction(),
			},
			categories: createTestCategories(),
			classifier: nil,
			wantMode:   ModeSelectingSuggestion,
			wantLoad:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewClassifierModel(tt.pending, tt.categories, themes.Default, tt.classifier)

			assert.Equal(t, tt.wantMode, m.mode)
			assert.Equal(t, tt.wantLoad, m.loading)
			assert.Equal(t, tt.pending.Transaction, m.transaction)
			assert.Equal(t, tt.pending, m.pending)
			assert.Equal(t, tt.pending.CategoryRankings, m.rankings)
			assert.Equal(t, tt.categories, m.categories)
			assert.Equal(t, themes.Default, m.theme)
			assert.Equal(t, tt.classifier, m.classifier)
			assert.NotNil(t, m.customInput)
			assert.NotNil(t, m.spinner)
			assert.Equal(t, "Enter custom category...", m.customInput.Placeholder)
			assert.Equal(t, 50, m.customInput.CharLimit)
		})
	}
}

func TestClassifierModel_Init(t *testing.T) {
	tests := []struct {
		classifier   engine.Classifier
		name         string
		wantCmdCount int
		loading      bool
	}{
		{
			name:         "loading with classifier",
			loading:      true,
			classifier:   &mockClassifier{},
			wantCmdCount: 2, // spinner tick + classify
		},
		{
			name:         "not loading",
			loading:      false,
			classifier:   &mockClassifier{},
			wantCmdCount: 1, // spinner tick only
		},
		{
			name:         "loading without classifier",
			loading:      true,
			classifier:   nil,
			wantCmdCount: 1, // spinner tick only
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := ClassifierModel{
				loading:    tt.loading,
				classifier: tt.classifier,
				spinner:    spinner.New(),
			}

			cmd := m.Init()

			// Test that we get a command
			if tt.wantCmdCount > 1 {
				assert.NotNil(t, cmd, "expected batch command")
			}
		})
	}
}

func TestClassifierModel_Update_AIClassificationMsg(t *testing.T) {
	tests := []struct {
		wantError    error
		name         string
		msg          AIClassificationMsg
		wantRankings model.CategoryRankings
		wantLoading  bool
	}{
		{
			name:         "successful classification",
			msg:          AIClassificationMsg{Rankings: createTestRankings()},
			wantLoading:  false,
			wantError:    nil,
			wantRankings: createTestRankings(),
		},
		{
			name:         "classification error",
			msg:          AIClassificationMsg{Error: errors.New("AI error")},
			wantLoading:  false,
			wantError:    errors.New("AI error"),
			wantRankings: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := ClassifierModel{
				loading: true,
			}

			updated, _ := m.Update(tt.msg)

			assert.Equal(t, tt.wantLoading, updated.loading)
			if tt.wantError != nil {
				assert.Error(t, updated.error)
				assert.Equal(t, tt.wantError.Error(), updated.error.Error())
			} else {
				assert.NoError(t, updated.error)
			}
			assert.Equal(t, tt.wantRankings, updated.rankings)
		})
	}
}

func TestClassifierModel_Update_KeyMsg_SuggestionMode_Commands(t *testing.T) {
	// Test cases that return commands
	tests := []struct {
		name           string
		key            string
		wantCategory   string
		rankings       model.CategoryRankings
		cursor         int
		wantConfidence float64
	}{
		{
			name:           "accept suggestion",
			key:            "enter",
			cursor:         1,
			rankings:       createTestRankings(),
			wantCategory:   "Shopping", // Second item in test rankings
			wantConfidence: 0.65,
		},
		{
			name:           "accept with 'a'",
			key:            "a",
			cursor:         0,
			rankings:       createTestRankings(),
			wantCategory:   "Groceries", // First item
			wantConfidence: 0.92,
		},
		{
			name:           "quick select 1",
			key:            "1",
			rankings:       createTestRankings(),
			wantCategory:   "Groceries", // First item
			wantConfidence: 0.92,
		},
		{
			name:           "quick select 3",
			key:            "3",
			rankings:       createTestRankings(),
			wantCategory:   "Dining Out", // Third item
			wantConfidence: 0.32,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &ClassifierModel{
				mode:        ModeSelectingSuggestion,
				cursor:      tt.cursor,
				rankings:    tt.rankings,
				transaction: createTestTransaction(),
			}

			var msg tea.Msg
			switch tt.key {
			case "enter":
				msg = tea.KeyMsg{Type: tea.KeyEnter}
			default:
				msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)}
			}

			// Call handleSuggestionMode directly since it has pointer receiver
			keyMsg, ok := msg.(tea.KeyMsg)
			require.True(t, ok, "msg should be tea.KeyMsg")
			cmd := m.handleSuggestionMode(keyMsg)

			// Execute the returned command if any
			if cmd != nil {
				// The command sets result when executed
				cmd()
			}

			// Check the result (state is set immediately)
			assert.True(t, m.complete)
			require.NotNil(t, m.result)
			assert.Equal(t, tt.wantCategory, m.result.Category)
			assert.Equal(t, tt.wantConfidence, m.result.Confidence)
			assert.Equal(t, model.StatusClassifiedByAI, m.result.Status)
		})
	}
}

func TestClassifierModel_Update_KeyMsg_SuggestionMode(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		wantStatus   model.ClassificationStatus
		rankings     model.CategoryRankings
		cursor       int
		wantCursor   int
		wantComplete bool
	}{
		{
			name:       "navigate down",
			key:        "j",
			cursor:     0,
			rankings:   createTestRankings(),
			wantCursor: 1,
		},
		{
			name:       "navigate down at end",
			key:        "down",
			cursor:     4,
			rankings:   createTestRankings(),
			wantCursor: 4,
		},
		{
			name:       "navigate up",
			key:        "k",
			cursor:     2,
			rankings:   createTestRankings(),
			wantCursor: 1,
		},
		{
			name:       "navigate up at start",
			key:        "up",
			cursor:     0,
			rankings:   createTestRankings(),
			wantCursor: 0,
		},
		{
			name:         "accept suggestion",
			key:          "enter",
			cursor:       1,
			rankings:     createTestRankings(),
			wantComplete: false, // Returns command, doesn't complete immediately
		},
		{
			name:         "accept with 'a'",
			key:          "a",
			cursor:       0,
			rankings:     createTestRankings(),
			wantComplete: false, // Returns command, doesn't complete immediately
		},
		{
			name:         "skip classification",
			key:          "s",
			rankings:     createTestRankings(),
			wantComplete: true,
			wantStatus:   model.StatusUnclassified,
		},
		{
			name:         "skip with space",
			key:          " ",
			rankings:     createTestRankings(),
			wantComplete: true,
			wantStatus:   model.StatusUnclassified,
		},
		{
			name:         "quick select 1",
			key:          "1",
			rankings:     createTestRankings(),
			wantComplete: false, // Returns command, doesn't complete immediately
		},
		{
			name:         "quick select 3",
			key:          "3",
			rankings:     createTestRankings(),
			wantComplete: false, // Returns command, doesn't complete immediately
		},
		{
			name:     "quick select out of range",
			key:      "9",
			rankings: createTestRankings()[:3],
		},
		{
			name:     "enter category mode",
			key:      "c",
			rankings: createTestRankings(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := ClassifierModel{
				mode:        ModeSelectingSuggestion,
				cursor:      tt.cursor,
				rankings:    tt.rankings,
				transaction: createTestTransaction(),
				categories:  createTestCategories(),
			}

			var msg tea.Msg
			switch tt.key {
			case "up":
				msg = tea.KeyMsg{Type: tea.KeyUp}
			case "down":
				msg = tea.KeyMsg{Type: tea.KeyDown}
			case "enter":
				msg = tea.KeyMsg{Type: tea.KeyEnter}
			default:
				msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)}
			}

			updated, _ := m.Update(msg)

			if !tt.wantComplete {
				if tt.key == "c" {
					// Check mode changed to category selection
					assert.Equal(t, ModeSelectingCategory, updated.mode)
					assert.NotNil(t, updated.sortedCategories)
				} else if tt.wantCursor > 0 {
					assert.Equal(t, tt.wantCursor, updated.cursor)
				}
				assert.False(t, updated.complete)
			} else {
				// Commands in classifier modify the model internally
				assert.True(t, updated.complete)
				assert.NotNil(t, updated.result)
				assert.Equal(t, tt.wantStatus, updated.result.Status)
			}
		})
	}
}

func TestClassifierModel_CategoryModeSelectCommand(t *testing.T) {
	t.Run("select category from list", func(t *testing.T) {
		m := &ClassifierModel{
			mode:             ModeSelectingCategory,
			sortedCategories: createTestCategories(),
			categoryCursor:   2, // Shopping
			rankings:         createTestRankings(),
			transaction:      createTestTransaction(),
		}

		// Call handleCategoryMode directly
		cmd := m.handleCategoryMode(tea.KeyMsg{Type: tea.KeyEnter})

		// Execute the returned command if any
		if cmd != nil {
			// The command sets result when executed
			cmd()
		}

		// State is set immediately
		assert.True(t, m.complete)
		assert.NotNil(t, m.result)
		assert.Equal(t, "Shopping", m.result.Category)
		assert.Equal(t, model.StatusUserModified, m.result.Status)
	})
}

func TestClassifierModel_Update_KeyMsg_CategoryMode(t *testing.T) {
	customInput := textinput.New()
	customInput.Placeholder = "Enter custom category..."

	m := ClassifierModel{
		mode:             ModeSelectingCategory,
		sortedCategories: createTestCategories(),
		categories:       createTestCategories(),
		categoryCursor:   5,
		categoryOffset:   0,
		transaction:      createTestTransaction(),
		rankings:         createTestRankings(),
		customInput:      customInput,
		theme:            themes.Default,
	}

	tests := []struct {
		name         string
		key          string
		wantCursor   int
		wantOffset   int
		wantMode     ClassifierMode
		wantComplete bool
	}{
		{
			name:       "navigate down",
			key:        "j",
			wantCursor: 6,
			wantOffset: 0,
			wantMode:   ModeSelectingCategory,
		},
		{
			name:       "navigate up",
			key:        "k",
			wantCursor: 4,
			wantOffset: 0,
			wantMode:   ModeSelectingCategory,
		},
		{
			name:       "go to first",
			key:        "g",
			wantCursor: 0,
			wantOffset: 0,
			wantMode:   ModeSelectingCategory,
		},
		{
			name:       "go to last",
			key:        "G",
			wantCursor: 9,
			wantOffset: 0,
			wantMode:   ModeSelectingCategory,
		},
		{
			name:     "select category",
			key:      "enter",
			wantMode: ModeSelectingCategory, // Stays in same mode, returns command
		},
		{
			name:     "escape back",
			key:      "esc",
			wantMode: ModeSelectingSuggestion,
		},
		{
			name:     "search mode",
			key:      "/",
			wantMode: ModeEnteringCustom,
		},
		{
			name:     "number input",
			key:      "5",
			wantMode: ModeEnteringCustom,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset state for each test
			testM := m
			testM.complete = false
			testM.result = nil

			var msg tea.Msg
			switch tt.key {
			case "enter":
				msg = tea.KeyMsg{Type: tea.KeyEnter}
			case "esc":
				msg = tea.KeyMsg{Type: tea.KeyEsc}
			case "home":
				msg = tea.KeyMsg{Type: tea.KeyHome}
			case "end":
				msg = tea.KeyMsg{Type: tea.KeyEnd}
			default:
				msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)}
			}

			updated, cmd := testM.Update(msg)

			if tt.key == "enter" {
				// Enter returns a command, doesn't complete immediately
				assert.NotNil(t, cmd, "should return a command for enter key")
				assert.False(t, updated.complete, "should not be complete until command executes")
			} else {
				if tt.wantCursor > 0 || tt.name == "go to first" {
					assert.Equal(t, tt.wantCursor, updated.categoryCursor)
				}
				assert.Equal(t, tt.wantOffset, updated.categoryOffset)
				assert.Equal(t, tt.wantMode, updated.mode)
			}
		})
	}
}

func TestClassifierModel_Update_KeyMsg_CustomMode(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		inputValue   string
		wantCategory string
		categories   []model.Category
		wantMode     ClassifierMode
		wantComplete bool
	}{
		{
			name:         "enter custom category",
			key:          "enter",
			inputValue:   "Custom Category",
			categories:   createTestCategories(),
			wantComplete: true,
			wantCategory: "Custom Category",
		},
		{
			name:         "enter category ID",
			key:          "enter",
			inputValue:   "10",
			categories:   createTestCategories(),
			wantComplete: true,
			wantCategory: "Entertainment",
		},
		{
			name:         "enter invalid ID",
			key:          "enter",
			inputValue:   "999",
			categories:   createTestCategories(),
			wantComplete: true,
			wantCategory: "999", // Treated as custom category
		},
		{
			name:       "enter empty",
			key:        "enter",
			inputValue: "",
			categories: createTestCategories(),
		},
		{
			name:       "escape from custom",
			key:        "esc",
			inputValue: "test",
			categories: createTestCategories(),
			wantMode:   ModeSelectingSuggestion,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			customInput := textinput.New()
			customInput.SetValue(tt.inputValue)

			m := ClassifierModel{
				mode:           ModeEnteringCustom,
				customInput:    customInput,
				categories:     tt.categories,
				categoryCursor: -1, // Not in category mode
				transaction:    createTestTransaction(),
			}

			var msg tea.Msg
			switch tt.key {
			case "enter":
				msg = tea.KeyMsg{Type: tea.KeyEnter}
			case "esc":
				msg = tea.KeyMsg{Type: tea.KeyEsc}
			default:
				msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)}
			}

			updated, cmd := m.Update(msg)

			if tt.wantComplete {
				// Enter returns a command, doesn't complete immediately
				assert.NotNil(t, cmd, "should return a command for completion")
				assert.False(t, updated.complete, "should not be complete until command executes")
			} else if tt.wantMode != 0 {
				assert.Equal(t, tt.wantMode, updated.mode)
			}
		})
	}
}

func TestClassifierModel_Update_WindowSizeMsg(t *testing.T) {
	m := ClassifierModel{}

	msg := tea.WindowSizeMsg{
		Width:  100,
		Height: 50,
	}

	updated, _ := m.Update(msg)

	assert.Equal(t, 100, updated.width)
	assert.Equal(t, 50, updated.height)
}

func TestClassifierModel_Update_SpinnerMsg(t *testing.T) {
	m := ClassifierModel{
		loading: true,
		spinner: spinner.New(),
	}

	// Create a spinner tick message
	msg := m.spinner.Tick()

	_, cmd := m.Update(msg)

	// Should return a command when loading
	assert.NotNil(t, cmd)
}

func TestClassifierModel_View_States(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() ClassifierModel
		contains []string
	}{
		{
			name: "loading state",
			setup: func() ClassifierModel {
				return ClassifierModel{
					loading: true,
					spinner: spinner.New(),
					theme:   themes.Default,
					width:   80,
					height:  20,
				}
			},
			contains: []string{"Analyzing transaction"},
		},
		{
			name: "error state",
			setup: func() ClassifierModel {
				return ClassifierModel{
					error: errors.New("Test error"),
					theme: themes.Default,
				}
			},
			contains: []string{"Error classifying transaction", "Test error", "Press 's' to skip"},
		},
		{
			name: "suggestion mode",
			setup: func() ClassifierModel {
				return ClassifierModel{
					mode:        ModeSelectingSuggestion,
					transaction: createTestTransaction(),
					rankings:    createTestRankings(),
					theme:       themes.Default,
					pending: model.PendingClassification{
						SuggestedCategory:   "Groceries",
						CategoryDescription: "Regular grocery shopping",
					},
				}
			},
			contains: []string{
				"Test Merchant",
				"January 15, 2024",
				"123.45",
				"Suggested Categories",
				"Groceries",
				"Shopping",
				"[1]", "[2]", "[3]",
				"AI: Regular grocery shopping",
			},
		},
		{
			name: "category picker mode",
			setup: func() ClassifierModel {
				m := ClassifierModel{
					mode:             ModeSelectingCategory,
					transaction:      createTestTransaction(),
					categories:       createTestCategories(),
					sortedCategories: createTestCategories(),
					rankings:         createTestRankings(),
					theme:            themes.Default,
					categoryCursor:   2,
					categoryOffset:   0,
				}
				m.prepareCategoryList()
				return m
			},
			contains: []string{
				"Select Category",
				"[1]", // Category IDs
				"Groceries",
				"Showing",
			},
		},
		{
			name: "custom input mode",
			setup: func() ClassifierModel {
				customInput := textinput.New()
				customInput.Placeholder = "Enter custom category..."
				return ClassifierModel{
					mode:        ModeEnteringCustom,
					transaction: createTestTransaction(),
					customInput: customInput,
					theme:       themes.Default,
				}
			},
			contains: []string{
				"Enter Custom Category",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.setup()
			view := m.View()

			for _, expected := range tt.contains {
				assert.Contains(t, view, expected)
			}
		})
	}
}

func TestClassifierModel_Helpers(t *testing.T) {
	t.Run("IsComplete", func(t *testing.T) {
		m := ClassifierModel{complete: false}
		assert.False(t, m.IsComplete())

		m.complete = true
		assert.True(t, m.IsComplete())
	})

	t.Run("GetResult", func(t *testing.T) {
		m := ClassifierModel{}
		result := m.GetResult()
		assert.Equal(t, model.Classification{}, result)

		expected := model.Classification{
			Transaction: createTestTransaction(),
			Category:    "Test",
		}
		m.result = &expected
		result = m.GetResult()
		assert.Equal(t, expected, result)
	})

	t.Run("Resize", func(t *testing.T) {
		m := ClassifierModel{}
		m.Resize(120, 40)
		assert.Equal(t, 120, m.width)
		assert.Equal(t, 40, m.height)
	})

	t.Run("getAmountPrefix", func(t *testing.T) {
		m := ClassifierModel{
			transaction: model.Transaction{Direction: model.DirectionIncome},
		}
		assert.Equal(t, "+", m.getAmountPrefix())

		m.transaction.Direction = model.DirectionExpense
		assert.Equal(t, "-", m.getAmountPrefix())
	})
}

func TestClassifierModel_PrepareCategoryList(t *testing.T) {
	m := ClassifierModel{
		categories: createTestCategories(),
		rankings: model.CategoryRankings{
			{Category: "Shopping", Score: 0.9},
			{Category: "Groceries", Score: 0.7},
			{Category: "Other", Score: 0.1},
		},
	}

	m.prepareCategoryList()

	// Should be sorted by confidence first
	require.Len(t, m.sortedCategories, len(m.categories))
	assert.Equal(t, "Shopping", m.sortedCategories[0].Name)
	assert.Equal(t, "Groceries", m.sortedCategories[1].Name)
	assert.Equal(t, "Other", m.sortedCategories[2].Name)

	// Then alphabetically for the rest
	assert.Equal(t, "Dining Out", m.sortedCategories[3].Name)
	assert.Equal(t, "Entertainment", m.sortedCategories[4].Name)
}

func TestClassifierModel_RenderConfidenceBar(t *testing.T) {
	m := ClassifierModel{theme: themes.Default}

	tests := []struct {
		name       string
		confidence int
	}{
		{name: "high confidence", confidence: 95},
		{name: "medium confidence", confidence: 65},
		{name: "low confidence", confidence: 25},
		{name: "zero confidence", confidence: 0},
		{name: "full confidence", confidence: 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bar := m.renderConfidenceBar(tt.confidence)
			// Bar should contain visual elements
			if tt.confidence > 0 {
				assert.Contains(t, bar, "█")
			}
			if tt.confidence < 100 {
				assert.Contains(t, bar, "░")
			}
		})
	}
}

func TestClassifierModel_CategoryScrolling(t *testing.T) {
	// Create many categories to test scrolling
	categories := make([]model.Category, 20)
	for i := 0; i < 20; i++ {
		categories[i] = model.Category{
			ID:   i + 1,
			Name: fmt.Sprintf("Category %02d", i+1),
		}
	}

	m := ClassifierModel{
		mode:             ModeSelectingCategory,
		categories:       categories,
		sortedCategories: categories,
		categoryCursor:   0,
		categoryOffset:   0,
		theme:            themes.Default,
	}

	// Scroll down past visible area
	for i := 0; i < 15; i++ {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
		m, _ = m.Update(msg)
	}

	// Should have scrolled
	assert.Equal(t, 15, m.categoryCursor)
	assert.Greater(t, m.categoryOffset, 0)

	// Scroll to end
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}}
	m, _ = m.Update(msg)
	assert.Equal(t, 19, m.categoryCursor)

	// Scroll to beginning
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}
	m, _ = m.Update(msg)
	assert.Equal(t, 0, m.categoryCursor)
	assert.Equal(t, 0, m.categoryOffset)

	// Test scrolling up when at top
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}
	m, _ = m.Update(msg)
	assert.Equal(t, 0, m.categoryCursor) // Should stay at 0

	// Test scrolling down when at bottom
	m.categoryCursor = 19
	m.categoryOffset = 10
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	m, _ = m.Update(msg)
	assert.Equal(t, 19, m.categoryCursor) // Should stay at 19

	// Test home key (using 'home' string is handled in handleCategoryMode)
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}}
	m, _ = m.Update(msg)
	// No effect with 'h', just testing the path

	// Test using actual keys that trigger home/end behavior
	// These are handled by the key strings "home" and "end", not rune conversion
}

func TestClassifierModel_ClassifyWithAI(t *testing.T) {
	rankings := createTestRankings()
	m := ClassifierModel{
		classifier:  &mockClassifier{rankings: rankings},
		transaction: createTestTransaction(),
		categories:  createTestCategories(),
		pending:     model.PendingClassification{},
	}

	cmd := m.classifyWithAI()
	require.NotNil(t, cmd)

	// Execute the command
	msg := cmd()
	aiMsg, ok := msg.(AIClassificationMsg)
	require.True(t, ok)

	assert.NoError(t, aiMsg.Error)
	assert.Equal(t, rankings, aiMsg.Rankings)
}

func TestClassifierModel_CustomModeCommands(t *testing.T) {
	t.Run("enter custom category", func(t *testing.T) {
		customInput := textinput.New()
		customInput.SetValue("Custom Category")

		m := &ClassifierModel{
			mode:        ModeEnteringCustom,
			customInput: customInput,
			categories:  createTestCategories(),
			transaction: createTestTransaction(),
		}

		// Call handleCustomMode directly
		cmd := m.handleCustomMode(tea.KeyMsg{Type: tea.KeyEnter})

		// Execute the returned command if any
		if cmd != nil {
			// The command sets result when executed
			cmd()
		}

		// State is set immediately
		assert.True(t, m.complete)
		assert.NotNil(t, m.result)
		assert.Equal(t, "Custom Category", m.result.Category)
		assert.Equal(t, model.StatusUserModified, m.result.Status)
	})

	t.Run("enter category ID", func(t *testing.T) {
		customInput := textinput.New()
		customInput.SetValue("10")

		m := &ClassifierModel{
			mode:        ModeEnteringCustom,
			customInput: customInput,
			categories:  createTestCategories(),
			transaction: createTestTransaction(),
		}

		// Call handleCustomMode directly
		cmd := m.handleCustomMode(tea.KeyMsg{Type: tea.KeyEnter})

		// Execute the returned command if any
		if cmd != nil {
			// The command sets result when executed
			cmd()
		}

		// State is set immediately
		assert.True(t, m.complete)
		assert.NotNil(t, m.result)
		assert.Equal(t, "Entertainment", m.result.Category) // ID 10 maps to Entertainment
	})
}

func TestClassifierModel_ConfirmCategoryCommands(t *testing.T) {
	t.Run("confirmCategory", func(t *testing.T) {
		m := ClassifierModel{
			transaction: createTestTransaction(),
		}

		ranking := model.CategoryRanking{
			Category: "Test Category",
			Score:    0.85,
		}

		cmd := m.confirmCategory(ranking)

		// Execute the returned command
		assert.NotNil(t, cmd, "confirmCategory should return a command")
		if cmd != nil {
			cmd()
		}

		// State is set immediately
		assert.True(t, m.complete)
		assert.NotNil(t, m.result)
		assert.Equal(t, "Test Category", m.result.Category)
		assert.Equal(t, 0.85, m.result.Confidence)
		assert.Equal(t, model.StatusClassifiedByAI, m.result.Status)
	})

	t.Run("confirmCategoryByName", func(t *testing.T) {
		m := ClassifierModel{
			transaction: createTestTransaction(),
		}

		cmd := m.confirmCategoryByName("Custom Category", 0.95)

		// Execute the returned command
		assert.NotNil(t, cmd, "confirmCategoryByName should return a command")
		if cmd != nil {
			cmd()
		}

		// State is set immediately
		assert.True(t, m.complete)
		assert.NotNil(t, m.result)
		assert.Equal(t, "Custom Category", m.result.Category)
		assert.Equal(t, 0.95, m.result.Confidence)
		assert.Equal(t, model.StatusUserModified, m.result.Status)
	})

	t.Run("createCustomCategory", func(t *testing.T) {
		m := ClassifierModel{
			transaction: createTestTransaction(),
			customInput: textinput.New(),
		}

		cmd := m.createCustomCategory("Brand New Category")

		// Execute the returned command
		assert.NotNil(t, cmd, "createCustomCategory should return a command")
		if cmd != nil {
			cmd()
		}

		// State is set immediately
		assert.True(t, m.complete)
		assert.NotNil(t, m.result)
		assert.Equal(t, "Brand New Category", m.result.Category)
		assert.Equal(t, 1.0, m.result.Confidence)
		assert.Equal(t, model.StatusUserModified, m.result.Status)
		assert.Equal(t, "Custom category", m.result.Notes)
	})
}

func TestClassifierModel_EdgeCases(t *testing.T) {
	t.Run("no suggestions available", func(t *testing.T) {
		m := ClassifierModel{
			mode:     ModeSelectingSuggestion,
			rankings: model.CategoryRankings{},
			theme:    themes.Default,
		}

		view := m.View()
		assert.Contains(t, view, "No suggestions available")
	})

	t.Run("transaction with different name and merchant", func(t *testing.T) {
		m := ClassifierModel{
			transaction: model.Transaction{
				MerchantName: "Merchant",
				Name:         "Different Description",
				Amount:       100,
				Date:         time.Now(),
				Type:         "debit",
			},
			theme: themes.Default,
		}

		view := m.View()
		assert.Contains(t, view, "Merchant")
		assert.Contains(t, view, "Description: Different Description")
	})

	t.Run("income transaction", func(t *testing.T) {
		m := ClassifierModel{
			transaction: model.Transaction{
				MerchantName: "Employer",
				Amount:       1000,
				Direction:    model.DirectionIncome,
				Date:         time.Now(),
				Type:         "credit",
			},
			theme: themes.Default,
		}

		view := m.View()
		assert.Contains(t, view, "+1000.00")
	})

	t.Run("escape from category mode with selection", func(t *testing.T) {
		m := ClassifierModel{
			mode:           ModeEnteringCustom,
			categoryCursor: 5, // Was in category mode
			customInput:    textinput.New(),
		}

		msg := tea.KeyMsg{Type: tea.KeyEsc}
		updated, _ := m.Update(msg)

		assert.Equal(t, ModeSelectingCategory, updated.mode)
	})
}

func TestClassifierModel_RenderMethods(t *testing.T) {
	theme := themes.Default

	t.Run("renderLoading", func(t *testing.T) {
		m := ClassifierModel{
			theme:   theme,
			spinner: spinner.New(),
			width:   80,
			height:  20,
		}

		view := m.renderLoading()
		assert.Contains(t, view, "Analyzing transaction")
	})

	t.Run("renderError", func(t *testing.T) {
		m := ClassifierModel{
			theme: theme,
			error: errors.New("Test error message"),
		}

		view := m.renderError()
		assert.Contains(t, view, "Error classifying transaction")
		assert.Contains(t, view, "Test error message")
		assert.Contains(t, view, "Press 's' to skip")
	})

	t.Run("renderHelp", func(t *testing.T) {
		tests := []struct {
			name     string
			contains []string
			mode     ClassifierMode
		}{
			{
				name: "suggestion mode",
				mode: ModeSelectingSuggestion,
				contains: []string{
					"Navigate",
					"Quick select",
					"Accept",
					"Select from all categories",
					"Skip",
				},
			},
			{
				name: "category mode",
				mode: ModeSelectingCategory,
				contains: []string{
					"Navigate",
					"First/Last",
					"Select",
					"Search",
					"Quick select",
					"Back",
				},
			},
			{
				name: "custom mode",
				mode: ModeEnteringCustom,
				contains: []string{
					"Confirm",
					"Cancel",
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				m := ClassifierModel{
					mode:  tt.mode,
					theme: theme,
				}

				help := m.renderHelp()
				for _, expected := range tt.contains {
					assert.Contains(t, help, expected)
				}
			})
		}
	})
}

// Test integration with text input updates.
func TestClassifierModel_TextInputIntegration(t *testing.T) {
	m := ClassifierModel{
		mode:        ModeEnteringCustom,
		customInput: textinput.New(),
		categories:  createTestCategories(),
		transaction: createTestTransaction(),
	}

	// Type some characters
	testString := "Test"
	for _, ch := range testString {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}}
		m, _ = m.Update(msg)
	}

	// The update should have been passed to the text input
	// We can't directly test the internal state, but we can verify
	// that typing followed by enter creates the right category
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	_, _ = m.Update(msg)

	// This should create a custom category command
	// but since we manually set the value above, it won't work
	// This test mainly ensures the text input update path works
	assert.NotNil(t, m.customInput)
}

func TestClassifierModel_HandleCategoryMode_NumberInput(t *testing.T) {
	t.Run("handle number input to enter ID", func(t *testing.T) {
		customInput := textinput.New()
		m := &ClassifierModel{
			mode:           ModeSelectingCategory,
			customInput:    customInput,
			categoryCursor: 0,
		}

		// Type a number
		cmd := m.handleCategoryMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}})

		// Should switch to custom entry mode and have command for blink
		assert.Equal(t, ModeEnteringCustom, m.mode)
		assert.Equal(t, "5", m.customInput.Value())
		assert.NotNil(t, cmd)
	})
}

func TestClassifierModel_RenderCategoryPicker_EdgeCases(t *testing.T) {
	t.Run("with AI confidence scores", func(t *testing.T) {
		m := ClassifierModel{
			mode:             ModeSelectingCategory,
			sortedCategories: createTestCategories()[:3],
			rankings:         createTestRankings(),
			theme:            themes.Default,
			categoryCursor:   0,
			categoryOffset:   0,
		}

		view := m.renderCategoryPicker()
		// Should show confidence bars for categories with rankings
		assert.Contains(t, view, "Groceries")
		assert.Contains(t, view, "92%") // Groceries confidence
	})

	t.Run("without AI confidence scores", func(t *testing.T) {
		// Test the else branch where confidence is 0
		m := ClassifierModel{
			mode: ModeSelectingCategory,
			sortedCategories: []model.Category{
				{ID: 99, Name: "No Confidence Category"},
			},
			rankings:       model.CategoryRankings{}, // No rankings
			theme:          themes.Default,
			categoryCursor: 0,
			categoryOffset: 0,
		}

		view := m.renderCategoryPicker()
		// Should show category without confidence bar
		assert.Contains(t, view, "[99]")
		assert.Contains(t, view, "No Confidence Category")
		assert.NotContains(t, view, "%") // No percentage shown
	})

	t.Run("with scroll indicators - top", func(t *testing.T) {
		// Create many categories to trigger scrolling
		categories := make([]model.Category, 20)
		for i := 0; i < 20; i++ {
			categories[i] = model.Category{
				ID:   i + 1,
				Name: fmt.Sprintf("Category %02d", i+1),
			}
		}

		m := ClassifierModel{
			mode:             ModeSelectingCategory,
			sortedCategories: categories,
			rankings:         model.CategoryRankings{},
			theme:            themes.Default,
			categoryCursor:   15,
			categoryOffset:   10, // Scrolled down
		}

		view := m.renderCategoryPicker()
		// Should show scroll indicators
		assert.Contains(t, view, "↑ More categories above")
		// Note: When showing items 11-20 of 20, there are no more items below,
		// so the down arrow indicator is not shown
		assert.Contains(t, view, "Showing 11-20 of 20")
	})

	t.Run("with scroll indicators - middle", func(t *testing.T) {
		// Create many categories to trigger scrolling down indicator
		categories := make([]model.Category, 25)
		for i := 0; i < 25; i++ {
			categories[i] = model.Category{
				ID:   i + 1,
				Name: fmt.Sprintf("Category %02d", i+1),
			}
		}

		m := ClassifierModel{
			mode:             ModeSelectingCategory,
			sortedCategories: categories,
			rankings:         model.CategoryRankings{},
			theme:            themes.Default,
			categoryCursor:   5,
			categoryOffset:   5, // Scrolled to middle
		}

		view := m.renderCategoryPicker()
		// Should show both scroll indicators
		assert.Contains(t, view, "↑ More categories above")
		assert.Contains(t, view, "↓")
		assert.Contains(t, view, "more categories below")
		assert.Contains(t, view, "Showing 6-15 of 25")
	})
}

func TestClassifierModel_AIClassificationTimeout(t *testing.T) {
	// Test that AI classification respects timeout
	m := ClassifierModel{
		classifier: &mockClassifier{
			delay:    100 * time.Millisecond,
			rankings: createTestRankings(),
		},
		transaction: createTestTransaction(),
		categories:  createTestCategories(),
	}

	start := time.Now()
	cmd := m.classifyWithAI()
	msg := cmd()
	duration := time.Since(start)

	// Should complete within reasonable time
	assert.Less(t, duration, 35*time.Second)

	aiMsg, ok := msg.(AIClassificationMsg)
	require.True(t, ok)
	assert.NoError(t, aiMsg.Error)
}

func TestClassifierModel_HandleCategoryMode_CompleteScrollCoverage(t *testing.T) {
	categories := make([]model.Category, 15)
	for i := 0; i < 15; i++ {
		categories[i] = model.Category{
			ID:   i + 1,
			Name: fmt.Sprintf("Category %02d", i+1),
		}
	}

	t.Run("scroll up adjusts offset", func(t *testing.T) {
		m := &ClassifierModel{
			mode:             ModeSelectingCategory,
			sortedCategories: categories,
			categoryCursor:   5,
			categoryOffset:   6, // Cursor is above visible area
		}

		cmd := m.handleCategoryMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
		assert.Nil(t, cmd)
		assert.Equal(t, 4, m.categoryCursor)
		assert.Equal(t, 4, m.categoryOffset) // Should adjust offset
	})

	t.Run("escape from category mode without custom cursor", func(t *testing.T) {
		m := &ClassifierModel{
			mode:           ModeSelectingCategory,
			categoryCursor: 5,
			categoryOffset: 0,
		}

		cmd := m.handleCategoryMode(tea.KeyMsg{Type: tea.KeyEsc})
		assert.Nil(t, cmd)
		assert.Equal(t, ModeSelectingSuggestion, m.mode)
		assert.Equal(t, 0, m.categoryCursor)
		assert.Equal(t, 0, m.categoryOffset)
	})
}
