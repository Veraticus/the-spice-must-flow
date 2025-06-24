package components

import (
	"fmt"
	"testing"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/engine"
	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/tui/themes"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

// Helper function to create test PendingDirection.
func createTestPendingDirection() engine.PendingDirection {
	return engine.PendingDirection{
		MerchantName:       "Test Merchant",
		SuggestedDirection: model.DirectionExpense,
		Reasoning:          "This appears to be a regular purchase",
		SampleTransaction: model.Transaction{
			ID:           "test_123",
			MerchantName: "Test Merchant",
			Name:         "TEST MERCHANT TRANSACTION",
			Amount:       123.45,
			Date:         time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			Type:         "debit",
		},
		TransactionCount: 5,
		Confidence:       0.85,
	}
}

func TestNewDirectionConfirmModel(t *testing.T) {
	tests := []struct {
		theme        themes.Theme
		name         string
		wantSelected model.TransactionDirection
		pending      engine.PendingDirection
		wantCursor   int
		wantComplete bool
	}{
		{
			name: "default initialization with expense suggestion",
			pending: engine.PendingDirection{
				MerchantName:       "Store",
				SuggestedDirection: model.DirectionExpense,
				Confidence:         0.9,
			},
			theme:        themes.Default,
			wantSelected: model.DirectionExpense,
			wantCursor:   0,
			wantComplete: false,
		},
		{
			name: "initialization with income suggestion",
			pending: engine.PendingDirection{
				MerchantName:       "Employer",
				SuggestedDirection: model.DirectionIncome,
				Confidence:         0.95,
			},
			theme:        themes.Default,
			wantSelected: model.DirectionIncome,
			wantCursor:   0,
			wantComplete: false,
		},
		{
			name: "initialization with transfer suggestion",
			pending: engine.PendingDirection{
				MerchantName:       "My Bank Transfer",
				SuggestedDirection: model.DirectionTransfer,
				Confidence:         0.88,
			},
			theme:        themes.Default,
			wantSelected: model.DirectionTransfer,
			wantCursor:   0,
			wantComplete: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewDirectionConfirmModel(tt.pending, tt.theme)

			assert.Equal(t, tt.pending, m.pending)
			assert.Equal(t, tt.theme, m.theme)
			assert.Equal(t, tt.wantSelected, m.selected)
			assert.Equal(t, tt.wantCursor, m.cursor)
			assert.Equal(t, tt.wantComplete, m.complete)
			assert.Equal(t, 0, m.width)
			assert.Equal(t, 0, m.height)
		})
	}
}

func TestDirectionConfirmModel_Update_Navigation(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		wantSelected model.TransactionDirection
		startCursor  int
		wantCursor   int
	}{
		{
			name:         "navigate down from first",
			key:          "j",
			startCursor:  0,
			wantCursor:   1,
			wantSelected: model.DirectionIncome,
		},
		{
			name:         "navigate down with arrow",
			key:          "down",
			startCursor:  1,
			wantCursor:   2,
			wantSelected: model.DirectionTransfer,
		},
		{
			name:         "navigate down wraps to beginning",
			key:          "j",
			startCursor:  2,
			wantCursor:   0,
			wantSelected: model.DirectionExpense,
		},
		{
			name:         "navigate up from middle",
			key:          "k",
			startCursor:  1,
			wantCursor:   0,
			wantSelected: model.DirectionExpense,
		},
		{
			name:         "navigate up with arrow",
			key:          "up",
			startCursor:  0,
			wantCursor:   2,
			wantSelected: model.DirectionTransfer,
		},
		{
			name:         "navigate up wraps to end",
			key:          "k",
			startCursor:  0,
			wantCursor:   2,
			wantSelected: model.DirectionTransfer,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := DirectionConfirmModel{
				pending:  createTestPendingDirection(),
				cursor:   tt.startCursor,
				selected: model.DirectionExpense, // Start with expense
				theme:    themes.Default,
			}

			var msg tea.Msg
			switch tt.key {
			case "up":
				msg = tea.KeyMsg{Type: tea.KeyUp}
			case "down":
				msg = tea.KeyMsg{Type: tea.KeyDown}
			default:
				msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)}
			}

			updated, cmd := m.Update(msg)

			assert.Nil(t, cmd)
			assert.Equal(t, tt.wantCursor, updated.cursor)
			assert.Equal(t, tt.wantSelected, updated.selected)
			assert.False(t, updated.complete)
		})
	}
}

func TestDirectionConfirmModel_Update_Selection(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		wantSelected model.TransactionDirection
		cursor       int
		wantComplete bool
	}{
		{
			name:         "accept with enter",
			key:          "enter",
			cursor:       1,
			wantComplete: true,
			wantSelected: model.DirectionIncome,
		},
		{
			name:         "accept with 'a'",
			key:          "a",
			cursor:       2,
			wantComplete: true,
			wantSelected: model.DirectionTransfer,
		},
		{
			name:         "quick select 1 - expense",
			key:          "1",
			cursor:       2, // Cursor doesn't matter for quick select
			wantComplete: true,
			wantSelected: model.DirectionExpense,
		},
		{
			name:         "quick select 2 - income",
			key:          "2",
			cursor:       0, // Cursor doesn't matter for quick select
			wantComplete: true,
			wantSelected: model.DirectionIncome,
		},
		{
			name:         "quick select 3 - transfer",
			key:          "3",
			cursor:       1, // Cursor doesn't matter for quick select
			wantComplete: true,
			wantSelected: model.DirectionTransfer,
		},
		{
			name:         "escape completes with current selection",
			key:          "esc",
			cursor:       2,
			wantComplete: true,
			wantSelected: model.DirectionTransfer, // Escape completes but cursor update overwrites the suggestion
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pending := createTestPendingDirection()
			m := DirectionConfirmModel{
				pending:  pending,
				cursor:   tt.cursor,
				selected: model.DirectionExpense, // Start with expense
				theme:    themes.Default,
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

			assert.Nil(t, cmd)
			assert.Equal(t, tt.wantComplete, updated.complete)
			if tt.wantComplete {
				assert.Equal(t, tt.wantSelected, updated.selected)
			}
		})
	}
}

func TestDirectionConfirmModel_Update_QuickSelect_UpdatesCursor(t *testing.T) {
	// Test that quick select also updates cursor position
	m := DirectionConfirmModel{
		pending:  createTestPendingDirection(),
		cursor:   0,
		selected: model.DirectionExpense,
		theme:    themes.Default,
	}

	// Quick select 2 should set cursor to 1 and select Income
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")}
	updated, _ := m.Update(msg)

	assert.Equal(t, 1, updated.cursor)
	assert.Equal(t, model.DirectionIncome, updated.selected)
	assert.True(t, updated.complete)
}

func TestDirectionConfirmModel_Update_WindowSize(t *testing.T) {
	m := DirectionConfirmModel{
		pending: createTestPendingDirection(),
		theme:   themes.Default,
	}

	msg := tea.WindowSizeMsg{
		Width:  120,
		Height: 40,
	}

	updated, cmd := m.Update(msg)

	assert.Nil(t, cmd)
	assert.Equal(t, 120, updated.width)
	assert.Equal(t, 40, updated.height)
}

func TestDirectionConfirmModel_Update_UnhandledKeys(t *testing.T) {
	m := DirectionConfirmModel{
		pending:  createTestPendingDirection(),
		cursor:   1,
		selected: model.DirectionIncome,
		theme:    themes.Default,
	}

	// Test various unhandled keys
	unhandledKeys := []string{"x", "q", "?", "4", "0", "9"}

	for _, key := range unhandledKeys {
		t.Run(fmt.Sprintf("unhandled key: %s", key), func(t *testing.T) {
			msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
			updated, cmd := m.Update(msg)

			// Should not change state
			assert.Nil(t, cmd)
			assert.Equal(t, m.cursor, updated.cursor)
			assert.Equal(t, m.selected, updated.selected)
			assert.Equal(t, m.complete, updated.complete)
		})
	}
}

func TestDirectionConfirmModel_View(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() DirectionConfirmModel
		contains []string
	}{
		{
			name: "displays all key information",
			setup: func() DirectionConfirmModel {
				return DirectionConfirmModel{
					pending:  createTestPendingDirection(),
					cursor:   0,
					selected: model.DirectionExpense,
					theme:    themes.Default,
					width:    100,
					height:   30,
				}
			},
			contains: []string{
				"Confirm Transaction Direction",
				"Test Merchant",
				"Transactions: 5",
				"AI Suggestion",
				"expense",
				"85% confidence",
				"This appears to be a regular purchase",
				"💸", "expense", "Money leaving your accounts",
				"[2]", "💰", "income", "Money entering your accounts",
				"[3]", "🔄", "transfer", "Moving between your accounts",
				"Sample:",
				"TEST MERCHANT TRANSACTION",
				"Jan 15, 2024",
				"-$123.45", // Should show negative for expense
				"Navigate", "Quick select", "Confirm", "Use suggestion",
			},
		},
		{
			name: "shows correct selection indicator",
			setup: func() DirectionConfirmModel {
				return DirectionConfirmModel{
					pending:  createTestPendingDirection(),
					cursor:   1, // Income selected
					selected: model.DirectionIncome,
					theme:    themes.Default,
					width:    100,
					height:   30,
				}
			},
			contains: []string{
				"> ",       // Selection indicator should be on income line
				"+$123.45", // Should show positive for income
			},
		},
		{
			name: "shows transfer amount without sign",
			setup: func() DirectionConfirmModel {
				return DirectionConfirmModel{
					pending:  createTestPendingDirection(),
					cursor:   2,
					selected: model.DirectionTransfer,
					theme:    themes.Default,
					width:    100,
					height:   30,
				}
			},
			contains: []string{
				"$123.45", // Transfer should not have +/- prefix
			},
		},
		{
			name: "formats high confidence suggestion",
			setup: func() DirectionConfirmModel {
				pending := createTestPendingDirection()
				pending.Confidence = 0.98
				return DirectionConfirmModel{
					pending:  pending,
					selected: model.DirectionExpense,
					theme:    themes.Default,
					width:    100,
					height:   30,
				}
			},
			contains: []string{
				"98% confidence",
			},
		},
		{
			name: "handles different merchant and transaction names",
			setup: func() DirectionConfirmModel {
				pending := createTestPendingDirection()
				pending.SampleTransaction.MerchantName = "Amazon"
				pending.SampleTransaction.Name = "AMZN MKTP US*123456789"
				return DirectionConfirmModel{
					pending:  pending,
					selected: model.DirectionExpense,
					theme:    themes.Default,
					width:    100,
					height:   30,
				}
			},
			contains: []string{
				"AMZN MKTP US*123456789",
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

func TestDirectionConfirmModel_Helpers(t *testing.T) {
	t.Run("IsComplete", func(t *testing.T) {
		m := DirectionConfirmModel{
			pending:  createTestPendingDirection(),
			complete: false,
		}
		assert.False(t, m.IsComplete())

		m.complete = true
		assert.True(t, m.IsComplete())
	})

	t.Run("GetResult", func(t *testing.T) {
		tests := []struct {
			name     string
			selected model.TransactionDirection
			want     model.TransactionDirection
		}{
			{
				name:     "returns expense",
				selected: model.DirectionExpense,
				want:     model.DirectionExpense,
			},
			{
				name:     "returns income",
				selected: model.DirectionIncome,
				want:     model.DirectionIncome,
			},
			{
				name:     "returns transfer",
				selected: model.DirectionTransfer,
				want:     model.DirectionTransfer,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				m := DirectionConfirmModel{
					selected: tt.selected,
				}
				assert.Equal(t, tt.want, m.GetResult())
			})
		}
	})

	t.Run("Resize", func(t *testing.T) {
		m := DirectionConfirmModel{}
		m.Resize(150, 50)
		assert.Equal(t, 150, m.width)
		assert.Equal(t, 50, m.height)
	})
}

func TestDirectionConfirmModel_getDirectionIcon(t *testing.T) {
	tests := []struct {
		name      string
		direction model.TransactionDirection
		wantIcon  string
	}{
		{
			name:      "expense icon",
			direction: model.DirectionExpense,
			wantIcon:  "💸",
		},
		{
			name:      "income icon",
			direction: model.DirectionIncome,
			wantIcon:  "💰",
		},
		{
			name:      "transfer icon",
			direction: model.DirectionTransfer,
			wantIcon:  "🔄",
		},
		{
			name:      "unknown direction",
			direction: model.TransactionDirection("unknown"),
			wantIcon:  "❓",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := DirectionConfirmModel{}
			icon := m.getDirectionIcon(tt.direction)
			assert.Equal(t, tt.wantIcon, icon)
		})
	}
}

func TestDirectionConfirmModel_RenderMethods(t *testing.T) {
	theme := themes.Default

	t.Run("renderMerchantInfo", func(t *testing.T) {
		m := DirectionConfirmModel{
			pending: engine.PendingDirection{
				MerchantName:     "Test Store",
				TransactionCount: 10,
			},
			theme: theme,
		}

		info := m.renderMerchantInfo()
		assert.Contains(t, info, "Merchant:")
		assert.Contains(t, info, "Test Store")
		assert.Contains(t, info, "Transactions: 10")
	})

	t.Run("renderSuggestion with different confidence levels", func(t *testing.T) {
		tests := []struct {
			name       string
			wantPct    string
			confidence float64
		}{
			{
				name:       "high confidence",
				confidence: 0.95,
				wantPct:    "95%",
			},
			{
				name:       "medium confidence",
				confidence: 0.65,
				wantPct:    "65%",
			},
			{
				name:       "low confidence",
				confidence: 0.33,
				wantPct:    "33%",
			},
			{
				name:       "zero confidence",
				confidence: 0.0,
				wantPct:    "0%",
			},
			{
				name:       "full confidence",
				confidence: 1.0,
				wantPct:    "100%",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				m := DirectionConfirmModel{
					pending: engine.PendingDirection{
						SuggestedDirection: model.DirectionExpense,
						Confidence:         tt.confidence,
						Reasoning:          "Test reasoning",
					},
					theme: theme,
				}

				suggestion := m.renderSuggestion()
				assert.Contains(t, suggestion, "AI Suggestion:")
				assert.Contains(t, suggestion, "expense")
				assert.Contains(t, suggestion, tt.wantPct)
				assert.Contains(t, suggestion, "Test reasoning")
			})
		}
	})

	t.Run("renderOptions with different cursor positions", func(t *testing.T) {
		for cursor := 0; cursor < 3; cursor++ {
			t.Run(fmt.Sprintf("cursor at %d", cursor), func(t *testing.T) {
				m := DirectionConfirmModel{
					cursor: cursor,
					theme:  theme,
				}

				options := m.renderOptions()

				// Check all options are present
				assert.Contains(t, options, "expense")
				assert.Contains(t, options, "income")
				assert.Contains(t, options, "transfer")
				assert.Contains(t, options, "Money leaving your accounts")
				assert.Contains(t, options, "Money entering your accounts")
				assert.Contains(t, options, "Moving between your accounts")

				// Check selection indicator
				assert.Contains(t, options, "> ")

				// Check that non-selected items have [n] prefix
				if cursor != 0 {
					assert.Contains(t, options, "[1]")
				}
				if cursor != 1 {
					assert.Contains(t, options, "[2]")
				}
				if cursor != 2 {
					assert.Contains(t, options, "[3]")
				}
			})
		}
	})

	t.Run("renderSampleTransaction with different amounts", func(t *testing.T) {
		baseTransaction := model.Transaction{
			Name:   "Test Transaction",
			Date:   time.Date(2024, 2, 20, 0, 0, 0, 0, time.UTC),
			Amount: 99.99,
		}

		tests := []struct {
			name       string
			selected   model.TransactionDirection
			wantAmount string
		}{
			{
				name:       "expense shows negative",
				selected:   model.DirectionExpense,
				wantAmount: "-$99.99",
			},
			{
				name:       "income shows positive",
				selected:   model.DirectionIncome,
				wantAmount: "+$99.99",
			},
			{
				name:       "transfer shows no sign",
				selected:   model.DirectionTransfer,
				wantAmount: "$99.99",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				m := DirectionConfirmModel{
					pending: engine.PendingDirection{
						SampleTransaction: baseTransaction,
					},
					selected: tt.selected,
					theme:    theme,
				}

				sample := m.renderSampleTransaction()
				assert.Contains(t, sample, "Sample:")
				assert.Contains(t, sample, "Test Transaction")
				assert.Contains(t, sample, "Feb 20, 2024")
				assert.Contains(t, sample, tt.wantAmount)
			})
		}
	})
}

func TestDirectionConfirmModel_EdgeCases(t *testing.T) {
	t.Run("empty merchant name", func(t *testing.T) {
		m := DirectionConfirmModel{
			pending: engine.PendingDirection{
				MerchantName:     "",
				TransactionCount: 1,
			},
			theme: themes.Default,
		}

		info := m.renderMerchantInfo()
		assert.Contains(t, info, "Merchant:")
		assert.Contains(t, info, "Transactions: 1")
	})

	t.Run("very long reasoning text", func(t *testing.T) {
		longReasoning := "This is a very long reasoning text that explains why the AI thinks this transaction should be categorized as an expense. It considers multiple factors including the merchant type, transaction amount, historical patterns, and contextual clues from the transaction description."

		m := DirectionConfirmModel{
			pending: engine.PendingDirection{
				SuggestedDirection: model.DirectionExpense,
				Confidence:         0.9,
				Reasoning:          longReasoning,
			},
			theme: themes.Default,
		}

		suggestion := m.renderSuggestion()
		assert.Contains(t, suggestion, longReasoning)
	})

	t.Run("zero transactions", func(t *testing.T) {
		m := DirectionConfirmModel{
			pending: engine.PendingDirection{
				MerchantName:     "Test",
				TransactionCount: 0,
			},
			theme: themes.Default,
		}

		info := m.renderMerchantInfo()
		assert.Contains(t, info, "Transactions: 0")
	})

	t.Run("very small window size", func(t *testing.T) {
		m := DirectionConfirmModel{
			pending: createTestPendingDirection(),
			theme:   themes.Default,
			width:   40,
			height:  10,
		}

		view := m.View()
		assert.NotEmpty(t, view)
		// Should still render something even with small window
	})

	t.Run("transaction with zero amount", func(t *testing.T) {
		pending := createTestPendingDirection()
		pending.SampleTransaction.Amount = 0.0

		m := DirectionConfirmModel{
			pending:  pending,
			selected: model.DirectionExpense,
			theme:    themes.Default,
		}

		sample := m.renderSampleTransaction()
		assert.Contains(t, sample, "-$0.00")
	})

	t.Run("transaction with very large amount", func(t *testing.T) {
		pending := createTestPendingDirection()
		pending.SampleTransaction.Amount = 1234567.89

		m := DirectionConfirmModel{
			pending:  pending,
			selected: model.DirectionIncome,
			theme:    themes.Default,
		}

		sample := m.renderSampleTransaction()
		assert.Contains(t, sample, "+$1234567.89")
	})
}

func TestDirectionConfirmModel_UpdateCursorMapping(t *testing.T) {
	// Test that cursor position correctly maps to direction selection
	tests := []struct {
		wantSelected model.TransactionDirection
		cursor       int
	}{
		{
			cursor:       0,
			wantSelected: model.DirectionExpense,
		},
		{
			cursor:       1,
			wantSelected: model.DirectionIncome,
		},
		{
			cursor:       2,
			wantSelected: model.DirectionTransfer,
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("cursor %d maps to %s", tt.cursor, tt.wantSelected), func(t *testing.T) {
			m := DirectionConfirmModel{
				pending: createTestPendingDirection(),
				cursor:  tt.cursor,
				theme:   themes.Default,
			}

			// Simulate any key that triggers cursor update
			msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}
			updated, _ := m.Update(msg)

			// The update should have set the selected based on the new cursor position
			// Since we started at tt.cursor and pressed 'j', cursor will be (tt.cursor + 1) % 3
			expectedCursor := (tt.cursor + 1) % 3
			var expectedSelected model.TransactionDirection
			switch expectedCursor {
			case 0:
				expectedSelected = model.DirectionExpense
			case 1:
				expectedSelected = model.DirectionIncome
			case 2:
				expectedSelected = model.DirectionTransfer
			}

			assert.Equal(t, expectedCursor, updated.cursor)
			assert.Equal(t, expectedSelected, updated.selected)
		})
	}
}

func TestDirectionConfirmModel_MultipleUpdates(t *testing.T) {
	// Test a sequence of updates to ensure state consistency
	m := NewDirectionConfirmModel(createTestPendingDirection(), themes.Default)

	// Initial state
	assert.Equal(t, 0, m.cursor)
	assert.Equal(t, model.DirectionExpense, m.selected)
	assert.False(t, m.complete)

	// Navigate down twice
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	assert.Equal(t, 1, m.cursor)
	assert.Equal(t, model.DirectionIncome, m.selected)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	assert.Equal(t, 2, m.cursor)
	assert.Equal(t, model.DirectionTransfer, m.selected)

	// Navigate up once
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	assert.Equal(t, 1, m.cursor)
	assert.Equal(t, model.DirectionIncome, m.selected)

	// Quick select 3
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")})
	assert.Equal(t, 2, m.cursor)
	assert.Equal(t, model.DirectionTransfer, m.selected)
	assert.True(t, m.complete)
}

func TestDirectionConfirmModel_NonKeyMessages(t *testing.T) {
	m := NewDirectionConfirmModel(createTestPendingDirection(), themes.Default)

	// Test with other message types
	otherMessages := []tea.Msg{
		nil,
		struct{}{},
		"string message",
		123,
	}

	for _, msg := range otherMessages {
		t.Run(fmt.Sprintf("message type %T", msg), func(t *testing.T) {
			updated, cmd := m.Update(msg)
			assert.Nil(t, cmd)
			assert.Equal(t, m, updated)
		})
	}
}

func TestDirectionConfirmModel_CompleteStatePersistence(t *testing.T) {
	// The direction component continues to process updates even when complete
	// This tests that behavior
	m := DirectionConfirmModel{
		pending:  createTestPendingDirection(),
		cursor:   1,
		selected: model.DirectionIncome,
		complete: true,
		theme:    themes.Default,
	}

	t.Run("navigation still works when complete", func(t *testing.T) {
		// Navigate down
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
		assert.Equal(t, 2, updated.cursor)
		assert.Equal(t, model.DirectionTransfer, updated.selected)
		assert.True(t, updated.complete)
	})

	t.Run("quick select still works when complete", func(t *testing.T) {
		// Quick select 1
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")})
		assert.Equal(t, 0, updated.cursor)
		assert.Equal(t, model.DirectionExpense, updated.selected)
		assert.True(t, updated.complete)
	})

	t.Run("window size updates work when complete", func(t *testing.T) {
		updated, _ := m.Update(tea.WindowSizeMsg{Width: 200, Height: 60})
		assert.Equal(t, 200, updated.width)
		assert.Equal(t, 60, updated.height)
		assert.True(t, updated.complete)
	})
}

func TestDirectionConfirmModel_FullCoverage(t *testing.T) {
	// Additional tests to ensure complete coverage

	t.Run("all navigation paths", func(t *testing.T) {
		m := NewDirectionConfirmModel(createTestPendingDirection(), themes.Default)

		// Test all cursor positions with all navigation keys
		for startCursor := 0; startCursor < 3; startCursor++ {
			m.cursor = startCursor

			// Test down navigation
			updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
			assert.Equal(t, (startCursor+1)%3, updated.cursor)

			// Reset and test up navigation
			m.cursor = startCursor
			updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
			assert.Equal(t, (startCursor+2)%3, updated.cursor)
		}
	})

	t.Run("view with nil theme fields", func(t *testing.T) {
		// Create a minimal theme to test defensive rendering
		minimalTheme := themes.Theme{}
		m := DirectionConfirmModel{
			pending: createTestPendingDirection(),
			theme:   minimalTheme,
			width:   80,
			height:  24,
		}

		// Should not panic
		view := m.View()
		assert.NotEmpty(t, view)
	})

	t.Run("escape behavior verification", func(t *testing.T) {
		pending := createTestPendingDirection()
		pending.SuggestedDirection = model.DirectionIncome

		m := DirectionConfirmModel{
			pending:  pending,
			cursor:   0,
			selected: model.DirectionExpense, // Different from suggestion
			theme:    themes.Default,
		}

		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})

		assert.True(t, updated.complete)
		// Escape sets to suggested direction but cursor 0 maps to expense
		assert.Equal(t, model.DirectionExpense, updated.selected)
	})
}
