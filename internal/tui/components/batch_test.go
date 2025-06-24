package components

import (
	"fmt"
	"testing"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/tui/themes"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper functions for testing.
func createTestPendingClassifications() []model.PendingClassification {
	return []model.PendingClassification{
		{
			Transaction: model.Transaction{
				ID:           "txn1",
				MerchantName: "Walmart",
				Amount:       50.00,
				Date:         time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
				Type:         "debit",
			},
			SuggestedCategory: "Groceries",
			Confidence:        0.92,
		},
		{
			Transaction: model.Transaction{
				ID:           "txn2",
				MerchantName: "Target",
				Amount:       75.50,
				Date:         time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC),
				Type:         "debit",
			},
			SuggestedCategory: "Shopping",
			Confidence:        0.85,
		},
		{
			Transaction: model.Transaction{
				ID:           "txn3",
				MerchantName: "Walmart",
				Amount:       125.00,
				Date:         time.Date(2024, 1, 17, 0, 0, 0, 0, time.UTC),
				Type:         "debit",
			},
			SuggestedCategory: "Groceries",
			Confidence:        0.88,
		},
		{
			Transaction: model.Transaction{
				ID:           "txn4",
				MerchantName: "Shell Gas",
				Amount:       40.00,
				Date:         time.Date(2024, 1, 18, 0, 0, 0, 0, time.UTC),
				Type:         "debit",
			},
			SuggestedCategory: "Transportation",
			Confidence:        0.95,
		},
	}
}

func TestNewBatchViewModel(t *testing.T) {
	pending := createTestPendingClassifications()
	theme := themes.Default

	m := NewBatchViewModel(pending, theme)

	assert.Equal(t, pending, m.pending)
	assert.NotNil(t, m.results)
	assert.Equal(t, 0, len(m.results))
	assert.Equal(t, cap(m.results), len(pending))
	assert.NotNil(t, m.groups)
	assert.Equal(t, theme, m.theme)
	assert.Equal(t, BatchModeReview, m.mode)
	assert.Equal(t, 0, m.currentIndex)
	assert.Equal(t, 0, m.currentGroup)
	assert.Equal(t, 0, m.cursor)
	assert.False(t, m.groupMode)
	assert.Equal(t, 0, m.width)
	assert.Equal(t, 0, m.height)

	// Check groups were created correctly
	assert.Equal(t, 3, len(m.groups)) // Walmart, Target, Shell Gas
}

func TestBatchViewModel_Update_WindowSizeMsg(t *testing.T) {
	m := BatchViewModel{}

	msg := tea.WindowSizeMsg{
		Width:  120,
		Height: 40,
	}

	updated, _ := m.Update(msg)

	assert.Equal(t, 120, updated.width)
	assert.Equal(t, 40, updated.height)
}

func TestBatchViewModel_HandleReviewMode(t *testing.T) {
	tests := []struct {
		name               string
		key                string
		groupMode          bool
		initialCursor      int
		initialGroup       int
		wantCursor         int
		wantGroup          int
		wantMode           BatchMode
		wantSelectedAction BatchAction
		groupCount         int
		pendingCount       int
	}{
		// Navigation tests - individual mode
		{
			name:          "navigate down in individual mode",
			key:           "j",
			groupMode:     false,
			initialCursor: 0,
			wantCursor:    1,
			pendingCount:  4,
		},
		{
			name:          "navigate down at end",
			key:           "down",
			groupMode:     false,
			initialCursor: 3,
			wantCursor:    3,
			pendingCount:  4,
		},
		{
			name:          "navigate up in individual mode",
			key:           "k",
			groupMode:     false,
			initialCursor: 2,
			wantCursor:    1,
			pendingCount:  4,
		},
		{
			name:          "navigate up at start",
			key:           "up",
			groupMode:     false,
			initialCursor: 0,
			wantCursor:    0,
			pendingCount:  4,
		},
		// Navigation tests - group mode
		{
			name:         "navigate down in group mode",
			key:          "j",
			groupMode:    true,
			initialGroup: 0,
			wantGroup:    1,
			groupCount:   3,
		},
		{
			name:         "navigate down at end in group mode",
			key:          "down",
			groupMode:    true,
			initialGroup: 2,
			wantGroup:    2,
			groupCount:   3,
		},
		{
			name:         "navigate up in group mode",
			key:          "k",
			groupMode:    true,
			initialGroup: 2,
			wantGroup:    1,
			groupCount:   3,
		},
		{
			name:         "navigate up at start in group mode",
			key:          "up",
			groupMode:    true,
			initialGroup: 0,
			wantGroup:    0,
			groupCount:   3,
		},
		// Mode changes
		{
			name:      "toggle group mode",
			key:       "g",
			groupMode: false,
		},
		{
			name:               "accept all",
			key:                "a",
			wantMode:           BatchModeConfirm,
			wantSelectedAction: BatchActionAcceptAll,
		},
		{
			name:               "skip all",
			key:                "s",
			wantMode:           BatchModeConfirm,
			wantSelectedAction: BatchActionSkipAll,
		},
		{
			name:               "review each",
			key:                "r",
			wantMode:           BatchModeApply,
			wantSelectedAction: BatchActionReviewEach,
		},
		{
			name:               "apply category",
			key:                "c",
			wantMode:           BatchModeApply,
			wantSelectedAction: BatchActionApplyCategory,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pending := createTestPendingClassifications()
			if tt.pendingCount > 0 {
				pending = pending[:tt.pendingCount]
			}
			m := NewBatchViewModel(pending, themes.Default)
			m.groupMode = tt.groupMode
			m.cursor = tt.initialCursor
			m.currentGroup = tt.initialGroup

			var msg tea.KeyMsg
			switch tt.key {
			case "up":
				msg = tea.KeyMsg{Type: tea.KeyUp}
			case "down":
				msg = tea.KeyMsg{Type: tea.KeyDown}
			default:
				msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)}
			}

			updated, _ := m.handleReviewMode(msg)

			switch {
			case tt.key == "g":
				assert.Equal(t, !tt.groupMode, updated.groupMode)
			case tt.wantMode != 0:
				assert.Equal(t, tt.wantMode, updated.mode)
				assert.Equal(t, tt.wantSelectedAction, updated.selectedAction)
				if tt.key == "r" {
					assert.Equal(t, 0, updated.currentIndex)
				}
			default:
				if tt.groupMode {
					assert.Equal(t, tt.wantGroup, updated.currentGroup)
				} else {
					assert.Equal(t, tt.wantCursor, updated.cursor)
				}
			}
		})
	}
}

func TestBatchViewModel_HandleApplyMode(t *testing.T) {
	tests := []struct {
		name           string
		key            string
		selectedAction BatchAction
		currentIndex   int
		pendingCount   int
		wantMode       BatchMode
		wantResults    int
	}{
		// Review each tests
		{
			name:           "accept current transaction",
			selectedAction: BatchActionReviewEach,
			key:            "a",
			currentIndex:   0,
			pendingCount:   2,
			wantResults:    1,
		},
		{
			name:           "accept with y",
			selectedAction: BatchActionReviewEach,
			key:            "y",
			currentIndex:   0,
			pendingCount:   2,
			wantResults:    1,
		},
		{
			name:           "skip current transaction",
			selectedAction: BatchActionReviewEach,
			key:            "s",
			currentIndex:   0,
			pendingCount:   2,
			wantResults:    1,
		},
		{
			name:           "skip with n",
			selectedAction: BatchActionReviewEach,
			key:            "n",
			currentIndex:   0,
			pendingCount:   2,
			wantResults:    1,
		},
		{
			name:           "escape from review",
			selectedAction: BatchActionReviewEach,
			key:            "esc",
			currentIndex:   0,
			wantMode:       BatchModeReview,
		},
		{
			name:           "complete review - last transaction",
			selectedAction: BatchActionReviewEach,
			key:            "a",
			currentIndex:   1,
			pendingCount:   2,
			wantResults:    1,
			wantMode:       BatchModeReview,
		},
		// Apply category tests
		{
			name:           "apply category mode",
			selectedAction: BatchActionApplyCategory,
			key:            "enter",
			wantMode:       BatchModeReview,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pending := createTestPendingClassifications()
			if tt.pendingCount > 0 {
				pending = pending[:tt.pendingCount]
			}
			m := NewBatchViewModel(pending, themes.Default)
			m.mode = BatchModeApply
			m.selectedAction = tt.selectedAction
			m.currentIndex = tt.currentIndex

			var msg tea.KeyMsg
			switch tt.key {
			case "esc":
				msg = tea.KeyMsg{Type: tea.KeyEsc}
			case "enter":
				msg = tea.KeyMsg{Type: tea.KeyEnter}
			default:
				msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)}
			}

			updated, _ := m.handleApplyMode(msg)

			if tt.wantMode != 0 {
				assert.Equal(t, tt.wantMode, updated.mode)
			}
			if tt.wantResults > 0 {
				assert.Equal(t, tt.wantResults, len(updated.results))

				// Check the result was created correctly
				result := updated.results[0]
				switch tt.key {
				case "a", "y":
					assert.Equal(t, model.StatusClassifiedByAI, result.Status)
					assert.NotEmpty(t, result.Category)
					assert.Greater(t, result.Confidence, 0.0)
				case "s", "n":
					assert.Equal(t, model.StatusUnclassified, result.Status)
					assert.Empty(t, result.Category)
				}
			}
		})
	}
}

func TestBatchViewModel_HandleConfirmMode(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		wantMode   BatchMode
		shouldQuit bool
	}{
		{
			name:       "confirm with y",
			key:        "y",
			shouldQuit: true,
		},
		{
			name:       "confirm with enter",
			key:        "enter",
			shouldQuit: true,
		},
		{
			name:     "cancel with n",
			key:      "n",
			wantMode: BatchModeReview,
		},
		{
			name:     "cancel with esc",
			key:      "esc",
			wantMode: BatchModeReview,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewBatchViewModel(createTestPendingClassifications(), themes.Default)
			m.mode = BatchModeConfirm
			m.selectedAction = BatchActionAcceptAll

			var msg tea.KeyMsg
			switch tt.key {
			case "enter":
				msg = tea.KeyMsg{Type: tea.KeyEnter}
			case "esc":
				msg = tea.KeyMsg{Type: tea.KeyEsc}
			default:
				msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)}
			}

			updated, cmd := m.handleConfirmMode(msg)

			if tt.shouldQuit {
				assert.NotNil(t, cmd)
				// Check that action was applied
				assert.NotEmpty(t, updated.results)
			} else {
				assert.Equal(t, tt.wantMode, updated.mode)
			}
		})
	}
}

func TestBatchViewModel_View(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() BatchViewModel
		contains []string
	}{
		{
			name: "review mode - individual view",
			setup: func() BatchViewModel {
				m := NewBatchViewModel(createTestPendingClassifications(), themes.Default)
				m.width = 80
				m.height = 24
				return m
			},
			contains: []string{
				"Batch Classification",
				"4 transactions to classify",
				"Toggle groups",
				"Accept all",
				"Skip all",
				"Review each",
				"Apply category",
			},
		},
		{
			name: "review mode - group view",
			setup: func() BatchViewModel {
				m := NewBatchViewModel(createTestPendingClassifications(), themes.Default)
				m.groupMode = true
				m.width = 80
				m.height = 24
				return m
			},
			contains: []string{
				"Batch Classification",
				"3 groups (4 transactions)",
				"Walmart",
				"(2 txns)",
				"Target",
				"Shell Gas",
			},
		},
		{
			name: "apply mode - individual review",
			setup: func() BatchViewModel {
				m := NewBatchViewModel(createTestPendingClassifications(), themes.Default)
				m.mode = BatchModeApply
				m.selectedAction = BatchActionReviewEach
				m.currentIndex = 1
				return m
			},
			contains: []string{
				"Transaction 2 of 4",
				"Target",
				"75.50",
				"Suggested: ",
				"Shopping",
				"Accept",
				"Skip",
				"Cancel",
			},
		},
		{
			name: "apply mode - category picker",
			setup: func() BatchViewModel {
				m := NewBatchViewModel(createTestPendingClassifications(), themes.Default)
				m.mode = BatchModeApply
				m.selectedAction = BatchActionApplyCategory
				return m
			},
			contains: []string{
				"Category picker not implemented",
			},
		},
		{
			name: "confirm mode - accept all",
			setup: func() BatchViewModel {
				m := NewBatchViewModel(createTestPendingClassifications(), themes.Default)
				m.mode = BatchModeConfirm
				m.selectedAction = BatchActionAcceptAll
				return m
			},
			contains: []string{
				"Confirm Action",
				"Accept All Suggestions",
				"This will classify 4 transactions",
				"Confirm",
				"Cancel",
			},
		},
		{
			name: "confirm mode - skip all",
			setup: func() BatchViewModel {
				m := NewBatchViewModel(createTestPendingClassifications(), themes.Default)
				m.mode = BatchModeConfirm
				m.selectedAction = BatchActionSkipAll
				return m
			},
			contains: []string{
				"Confirm Action",
				"Skip All",
				"This will skip 4 transactions",
				"Confirm",
				"Cancel",
			},
		},
		{
			name: "complete state",
			setup: func() BatchViewModel {
				m := NewBatchViewModel(createTestPendingClassifications()[:2], themes.Default)
				m.mode = BatchModeApply
				m.selectedAction = BatchActionReviewEach
				m.currentIndex = 2 // Past the end
				m.results = []model.Classification{
					{Status: model.StatusClassifiedByAI},
				}
				return m
			},
			contains: []string{
				"Batch Classification Complete!",
				"Completed: 1 classified, 1 skipped",
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

func TestBatchViewModel_RenderTransactionList(t *testing.T) {
	m := NewBatchViewModel(createTestPendingClassifications(), themes.Default)
	m.width = 80

	tests := []struct {
		name     string
		contains []string
		cursor   int
	}{
		{
			name:   "cursor at start",
			cursor: 0,
			contains: []string{
				"> Walmart",
				"50.00",
				"Groceries",
			},
		},
		{
			name:   "cursor in middle",
			cursor: 2,
			contains: []string{
				"Walmart",
				"125.00",
				"> ",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m.cursor = tt.cursor
			view := m.renderTransactionList()

			for _, expected := range tt.contains {
				assert.Contains(t, view, expected)
			}
		})
	}
}

func TestBatchViewModel_RenderGroups(t *testing.T) {
	m := NewBatchViewModel(createTestPendingClassifications(), themes.Default)
	m.groupMode = true
	m.width = 80

	tests := []struct {
		name         string
		contains     []string
		currentGroup int
	}{
		{
			name:         "first group selected",
			currentGroup: 0,
			contains: []string{
				"> ",
				"Walmart",
				"(2 txns)",
				"Groceries",
			},
		},
		{
			name:         "second group selected",
			currentGroup: 1,
			contains: []string{
				"Walmart",
				"> ",
				"Target",
				"(1 txns)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m.currentGroup = tt.currentGroup
			view := m.renderGroups()

			for _, expected := range tt.contains {
				assert.Contains(t, view, expected)
			}
		})
	}
}

func TestBatchViewModel_HelperMethods(t *testing.T) {
	t.Run("acceptCurrent", func(t *testing.T) {
		m := NewBatchViewModel(createTestPendingClassifications(), themes.Default)
		m.currentIndex = 0

		m.acceptCurrent()

		require.Len(t, m.results, 1)
		result := m.results[0]
		assert.Equal(t, "txn1", result.Transaction.ID)
		assert.Equal(t, "Groceries", result.Category)
		assert.Equal(t, model.StatusClassifiedByAI, result.Status)
		assert.Equal(t, 0.92, result.Confidence)
		assert.NotZero(t, result.ClassifiedAt)
	})

	t.Run("acceptCurrent out of bounds", func(t *testing.T) {
		m := NewBatchViewModel(createTestPendingClassifications(), themes.Default)
		m.currentIndex = 10 // Out of bounds

		m.acceptCurrent()

		assert.Len(t, m.results, 0)
	})

	t.Run("skipCurrent", func(t *testing.T) {
		m := NewBatchViewModel(createTestPendingClassifications(), themes.Default)
		m.currentIndex = 1

		m.skipCurrent()

		require.Len(t, m.results, 1)
		result := m.results[0]
		assert.Equal(t, "txn2", result.Transaction.ID)
		assert.Empty(t, result.Category)
		assert.Equal(t, model.StatusUnclassified, result.Status)
		assert.NotZero(t, result.ClassifiedAt)
	})

	t.Run("skipCurrent out of bounds", func(t *testing.T) {
		m := NewBatchViewModel(createTestPendingClassifications(), themes.Default)
		m.currentIndex = 10 // Out of bounds

		m.skipCurrent()

		assert.Len(t, m.results, 0)
	})

	t.Run("nextTransaction", func(t *testing.T) {
		m := NewBatchViewModel(createTestPendingClassifications(), themes.Default)
		m.mode = BatchModeApply
		m.currentIndex = 0

		m.nextTransaction()
		assert.Equal(t, 1, m.currentIndex)
		assert.Equal(t, BatchModeApply, m.mode)

		// Go to end
		m.currentIndex = 3
		m.nextTransaction()
		assert.Equal(t, 4, m.currentIndex)
		assert.Equal(t, BatchModeReview, m.mode) // Mode changed
	})

	t.Run("applySelectedAction - AcceptAll", func(t *testing.T) {
		m := NewBatchViewModel(createTestPendingClassifications(), themes.Default)
		m.selectedAction = BatchActionAcceptAll

		m.applySelectedAction()

		assert.Len(t, m.results, 4)
		for i, result := range m.results {
			assert.Equal(t, model.StatusClassifiedByAI, result.Status)
			assert.NotEmpty(t, result.Category)
			assert.Greater(t, result.Confidence, 0.0)
			assert.NotZero(t, result.ClassifiedAt)
			assert.Equal(t, m.pending[i].Transaction.ID, result.Transaction.ID)
		}
	})

	t.Run("applySelectedAction - SkipAll", func(t *testing.T) {
		m := NewBatchViewModel(createTestPendingClassifications(), themes.Default)
		m.selectedAction = BatchActionSkipAll

		m.applySelectedAction()

		assert.Len(t, m.results, 4)
		for i, result := range m.results {
			assert.Equal(t, model.StatusUnclassified, result.Status)
			assert.Empty(t, result.Category)
			assert.Zero(t, result.Confidence)
			assert.NotZero(t, result.ClassifiedAt)
			assert.Equal(t, m.pending[i].Transaction.ID, result.Transaction.ID)
		}
	})

	t.Run("GetResults", func(t *testing.T) {
		m := NewBatchViewModel(createTestPendingClassifications(), themes.Default)

		// No results yet
		results := m.GetResults()
		assert.Nil(t, results)

		// Partial results
		m.results = []model.Classification{{}, {}}
		results = m.GetResults()
		assert.Nil(t, results)

		// All results
		m.results = []model.Classification{{}, {}, {}, {}}
		results = m.GetResults()
		assert.NotNil(t, results)
		assert.Len(t, results, 4)
	})

	t.Run("Resize", func(t *testing.T) {
		m := BatchViewModel{}
		m.Resize(100, 50)
		assert.Equal(t, 100, m.width)
		assert.Equal(t, 50, m.height)
	})
}

func TestGroupPendingTransactions(t *testing.T) {
	pending := createTestPendingClassifications()
	groups := groupPendingTransactions(pending)

	assert.Len(t, groups, 3) // Walmart, Target, Shell Gas

	// Find Walmart group
	var walmartGroup *BatchGroup
	for i := range groups {
		if groups[i].Key == "Walmart" {
			walmartGroup = &groups[i]
			break
		}
	}

	require.NotNil(t, walmartGroup)
	assert.Equal(t, "Walmart", walmartGroup.Key)
	assert.Len(t, walmartGroup.Transactions, 2)
	assert.Equal(t, "Groceries", walmartGroup.SuggestedCategory)
	assert.Equal(t, 0.9, walmartGroup.Confidence) // Average of 0.92 and 0.88
}

func TestBatchViewModel_EdgeCases(t *testing.T) {
	t.Run("empty pending list", func(t *testing.T) {
		m := NewBatchViewModel([]model.PendingClassification{}, themes.Default)
		assert.Empty(t, m.pending)
		assert.Empty(t, m.groups)
		assert.Empty(t, m.results)
	})

	t.Run("single transaction", func(t *testing.T) {
		pending := createTestPendingClassifications()[:1]
		m := NewBatchViewModel(pending, themes.Default)
		assert.Len(t, m.pending, 1)
		assert.Len(t, m.groups, 1)
	})

	t.Run("renderIndividualReview at boundary", func(t *testing.T) {
		m := NewBatchViewModel(createTestPendingClassifications(), themes.Default)
		m.mode = BatchModeApply
		m.selectedAction = BatchActionReviewEach
		m.currentIndex = 4 // Equal to length

		view := m.renderIndividualReview()
		assert.Contains(t, view, "Batch Classification Complete!")
	})

	t.Run("transactions with long merchant names", func(t *testing.T) {
		pending := []model.PendingClassification{
			{
				Transaction: model.Transaction{
					ID:           "long1",
					MerchantName: "This is a very long merchant name that should be truncated",
					Amount:       100.00,
					Date:         time.Now(),
				},
				SuggestedCategory: "Shopping",
				Confidence:        0.85,
			},
		}
		m := NewBatchViewModel(pending, themes.Default)
		m.width = 80

		view := m.renderTransactionList()
		assert.Contains(t, view, "...")
		assert.NotContains(t, view, "should be truncated")
	})

	t.Run("scrolling with many transactions", func(t *testing.T) {
		// Create many transactions
		pending := make([]model.PendingClassification, 20)
		for i := 0; i < 20; i++ {
			pending[i] = model.PendingClassification{
				Transaction: model.Transaction{
					ID:           fmt.Sprintf("txn%d", i),
					MerchantName: fmt.Sprintf("Merchant %d", i),
					Amount:       float64(i) * 10,
					Date:         time.Now(),
				},
				SuggestedCategory: "Test",
				Confidence:        0.8,
			}
		}

		m := NewBatchViewModel(pending, themes.Default)
		m.cursor = 10 // Middle of list

		view := m.renderTransactionList()
		// Should show items around cursor (5-14)
		assert.Contains(t, view, "Merchant 5")
		assert.Contains(t, view, "Merchant 14")
		assert.NotContains(t, view, "Merchant 0")
		assert.NotContains(t, view, "Merchant 19")
	})
}

func TestBatchViewModel_RenderConfirmMode_EdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		contains       string
		selectedAction BatchAction
	}{
		{
			name:           "unknown action",
			selectedAction: BatchAction(99),
			contains:       "", // Should handle gracefully
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewBatchViewModel(createTestPendingClassifications(), themes.Default)
			m.mode = BatchModeConfirm
			m.selectedAction = tt.selectedAction

			view := m.renderConfirmMode()
			if tt.contains != "" {
				assert.Contains(t, view, tt.contains)
			}
		})
	}
}

func TestBatchViewModel_CompleteFlows(t *testing.T) {
	t.Run("complete accept all flow", func(t *testing.T) {
		m := NewBatchViewModel(createTestPendingClassifications(), themes.Default)

		// Start review mode
		assert.Equal(t, BatchModeReview, m.mode)

		// Press 'a' to accept all
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}
		m, _ = m.Update(msg)
		assert.Equal(t, BatchModeConfirm, m.mode)
		assert.Equal(t, BatchActionAcceptAll, m.selectedAction)

		// Confirm with 'y'
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}
		m, cmd := m.Update(msg)
		assert.NotNil(t, cmd) // Should quit
		assert.Len(t, m.results, 4)

		// Check results
		results := m.GetResults()
		assert.NotNil(t, results)
		assert.Len(t, results, 4)
	})

	t.Run("complete review each flow", func(t *testing.T) {
		m := NewBatchViewModel(createTestPendingClassifications()[:2], themes.Default)

		// Start review each
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}
		m, _ = m.Update(msg)
		assert.Equal(t, BatchModeApply, m.mode)
		assert.Equal(t, BatchActionReviewEach, m.selectedAction)

		// Accept first
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}
		m, _ = m.Update(msg)
		assert.Equal(t, 1, m.currentIndex)
		assert.Len(t, m.results, 1)

		// Skip second
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
		m, _ = m.Update(msg)
		assert.Equal(t, 2, m.currentIndex)
		assert.Len(t, m.results, 2)
		assert.Equal(t, BatchModeReview, m.mode) // Back to review after completing all
	})

	t.Run("cancel from confirm mode", func(t *testing.T) {
		m := NewBatchViewModel(createTestPendingClassifications(), themes.Default)

		// Go to confirm mode
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
		m, _ = m.Update(msg)
		assert.Equal(t, BatchModeConfirm, m.mode)

		// Cancel
		msg = tea.KeyMsg{Type: tea.KeyEsc}
		m, _ = m.Update(msg)
		assert.Equal(t, BatchModeReview, m.mode)
		assert.Empty(t, m.results)
	})
}

func TestBatchViewModel_Min_Max_Functions(t *testing.T) {
	// Test the inline min/max functions through usage
	m := NewBatchViewModel(createTestPendingClassifications(), themes.Default)

	// Test max boundary
	m.cursor = 10
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	m, _ = m.Update(msg)
	assert.Equal(t, 3, m.cursor) // Should be capped at len(pending)-1

	// Test min boundary
	m.cursor = 0
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}
	m, _ = m.Update(msg)
	assert.Equal(t, 0, m.cursor) // Should stay at 0
}

func TestTruncate(t *testing.T) {
	// The truncate function is used internally, test through the render functions
	m := NewBatchViewModel([]model.PendingClassification{
		{
			Transaction: model.Transaction{
				MerchantName: "Short",
				Amount:       50.00,
			},
			SuggestedCategory: "Test",
			Confidence:        0.9,
		},
		{
			Transaction: model.Transaction{
				MerchantName: "This is a very long merchant name that needs truncation",
				Amount:       50.00,
			},
			SuggestedCategory: "Test",
			Confidence:        0.9,
		},
	}, themes.Default)

	view := m.renderTransactionList()
	assert.Contains(t, view, "Short")
	assert.Contains(t, view, "...")
	assert.NotContains(t, view, "needs truncation")

	// Also test truncation in group mode
	m.groupMode = true
	view = m.renderGroups()
	assert.Contains(t, view, "...")
}

func TestBatchViewModel_GroupConfidenceCalculation(t *testing.T) {
	pending := []model.PendingClassification{
		{
			Transaction: model.Transaction{
				ID:           "1",
				MerchantName: "TestMerchant",
			},
			SuggestedCategory: "Cat1",
			Confidence:        0.8,
		},
		{
			Transaction: model.Transaction{
				ID:           "2",
				MerchantName: "TestMerchant",
			},
			SuggestedCategory: "Cat1",
			Confidence:        0.6,
		},
		{
			Transaction: model.Transaction{
				ID:           "3",
				MerchantName: "TestMerchant",
			},
			SuggestedCategory: "Cat2", // Different category, but should take first
			Confidence:        0.9,
		},
	}

	groups := groupPendingTransactions(pending)
	assert.Len(t, groups, 1)
	assert.Equal(t, "TestMerchant", groups[0].Key)
	assert.Equal(t, "Cat1", groups[0].SuggestedCategory)  // First transaction's category
	assert.InDelta(t, 0.7666, groups[0].Confidence, 0.01) // Average: (0.8+0.6+0.9)/3
}

// Add test for fmt import.
func TestBatchViewModel_FormatStrings(t *testing.T) {
	m := NewBatchViewModel(createTestPendingClassifications(), themes.Default)
	m.width = 80
	m.height = 24

	// Test various string formatting in views
	view := m.View()
	assert.Contains(t, view, "4 transactions") // Uses fmt.Sprintf

	m.groupMode = true
	view = m.View()
	assert.Contains(t, view, "3 groups") // Uses fmt.Sprintf
}

func TestBatchViewModel_View_DefaultCase(t *testing.T) {
	// Test the default case in View()
	m := NewBatchViewModel(createTestPendingClassifications(), themes.Default)
	m.mode = BatchMode(99) // Invalid mode

	view := m.View()
	assert.Equal(t, "", view)
}

func TestBatchViewModel_RenderApplyMode_DefaultCase(t *testing.T) {
	// Test the default case in renderApplyMode()
	m := NewBatchViewModel(createTestPendingClassifications(), themes.Default)
	m.mode = BatchModeApply
	m.selectedAction = BatchAction(99) // Invalid action

	view := m.renderApplyMode()
	assert.Equal(t, "", view)
}
