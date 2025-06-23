//go:build integration
// +build integration

package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/engine"
	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPrompterWithEngine tests the integration between the CLI prompter and the engine.
func TestPrompterWithEngine(t *testing.T) {
	// Create mock transaction
	txn := model.Transaction{
		ID:           "test-tx-1",
		Name:         "STARBUCKS STORE #12345",
		MerchantName: "Starbucks",
		Amount:       5.75,
		Date:         time.Now(),
		AccountID:    "test-account",
		Direction:    model.DirectionExpense,
	}

	// Test single classification
	t.Run("single transaction classification", func(t *testing.T) {
		input := "a\n" // Accept AI suggestion
		reader := strings.NewReader(input)
		var output bytes.Buffer

		prompter := NewCLIPrompter(reader, &output)

		// Verify it implements the engine.Prompter interface
		var _ engine.Prompter = prompter

		pending := model.PendingClassification{
			Transaction:       txn,
			SuggestedCategory: "Coffee & Dining",
			Confidence:        0.92,
			SimilarCount:      0,
		}

		ctx := context.Background()
		classification, err := prompter.ConfirmClassification(ctx, pending)

		require.NoError(t, err)
		assert.Equal(t, "Coffee & Dining", classification.Category)
		assert.Equal(t, model.StatusClassifiedByAI, classification.Status)

		// Check output formatting
		outputStr := output.String()
		assert.Contains(t, outputStr, "Transaction Review: Starbucks")
		assert.Contains(t, outputStr, "AI Suggestion:")
		assert.Contains(t, outputStr, "Coffee & Dining")
		assert.Contains(t, outputStr, "92% confidence")
	})

	// Test batch classification
	t.Run("batch classification", func(t *testing.T) {
		input := "a\n" // Accept all
		reader := strings.NewReader(input)
		var output bytes.Buffer

		prompter := NewCLIPrompter(reader, &output)
		prompter.SetTotalTransactions(5)

		pending := []model.PendingClassification{
			{
				Transaction:       txn,
				SuggestedCategory: "Coffee & Dining",
				Confidence:        0.92,
			},
			{
				Transaction: model.Transaction{
					ID:           "test-tx-2",
					Name:         "STARBUCKS STORE #67890",
					MerchantName: "Starbucks",
					Amount:       4.25,
					Date:         time.Now().AddDate(0, 0, -1),
					AccountID:    "test-account",
					Direction:    model.DirectionExpense,
				},
				SuggestedCategory: "Coffee & Dining",
				Confidence:        0.92,
			},
		}

		ctx := context.Background()
		classifications, err := prompter.BatchConfirmClassifications(ctx, pending)

		require.NoError(t, err)
		assert.Len(t, classifications, 2)

		for _, c := range classifications {
			assert.Equal(t, "Coffee & Dining", c.Category)
			assert.Equal(t, model.StatusClassifiedByAI, c.Status)
		}

		// Check output shows batch review
		outputStr := output.String()
		assert.Contains(t, outputStr, "Batch Review: Starbucks")
		assert.Contains(t, outputStr, "Transactions: 2")
		assert.Contains(t, outputStr, "Accept for all 2 transactions")

		// Check completion stats
		stats := prompter.GetCompletionStats()
		assert.Equal(t, 2, stats.TotalTransactions)
		assert.Equal(t, 2, stats.AutoClassified)
		assert.Equal(t, 0, stats.UserClassified)
		assert.Equal(t, 1, stats.NewVendorRules)
	})

	// Test interrupt handling
	t.Run("interrupt handling", func(t *testing.T) {
		var output bytes.Buffer
		handler := NewInterruptHandler(&output)

		ctx := context.Background()
		ctx = handler.HandleInterrupts(ctx, true)

		// Context should not be cancelled initially
		select {
		case <-ctx.Done():
			t.Fatal("Context should not be cancelled initially")
		default:
		}

		// Cancel the context to simulate interrupt
		if cancelFunc := ctx.Value("cancel"); cancelFunc != nil {
			if cancel, ok := cancelFunc.(context.CancelFunc); ok {
				cancel()
			}
		}

		// Handler can detect if interrupted (in real scenario, signal would trigger this)
		assert.False(t, handler.WasInterrupted()) // No signal sent, just context cancelled
	})
}

// TestPrompterEdgeCases tests edge cases and error conditions.
func TestPrompterEdgeCases(t *testing.T) {
	t.Run("empty merchant name", func(t *testing.T) {
		// Need to provide input for promptCategorySelection when "c" is chosen
		// The promptCategorySelection will need a category name or number
		input := "c\nn\nMiscellaneous\n" // c = custom, n = new category, then the name
		reader := strings.NewReader(input)
		var output bytes.Buffer

		prompter := NewCLIPrompter(reader, &output)

		pending := model.PendingClassification{
			Transaction: model.Transaction{
				ID:           "test-tx",
				Name:         "UNKNOWN TRANSACTION",
				MerchantName: "", // Empty merchant name
				Amount:       50.00,
				Date:         time.Now(),
				Direction:    model.DirectionExpense,
				AccountID:    "test-account",
			},
			SuggestedCategory: "Other",
			Confidence:        0.50,
		}

		ctx := context.Background()
		classification, err := prompter.ConfirmClassification(ctx, pending)

		require.NoError(t, err)
		assert.Equal(t, "Miscellaneous", classification.Category)
		assert.Equal(t, model.StatusUserModified, classification.Status)

		// Should use transaction name when merchant name is empty
		outputStr := output.String()
		assert.Contains(t, outputStr, "UNKNOWN TRANSACTION")
	})

	t.Run("very long merchant name", func(t *testing.T) {
		input := "s\n"
		reader := strings.NewReader(input)
		var output bytes.Buffer

		prompter := NewCLIPrompter(reader, &output)

		longName := strings.Repeat("Very Long Merchant Name ", 10)
		pending := model.PendingClassification{
			Transaction: model.Transaction{
				ID:           "test-tx",
				Name:         longName,
				MerchantName: longName,
				Amount:       100.00,
				Date:         time.Now(),
				Direction:    model.DirectionExpense,
				AccountID:    "test-account",
			},
			SuggestedCategory: "Shopping",
			Confidence:        0.70,
		}

		ctx := context.Background()
		_, err := prompter.ConfirmClassification(ctx, pending)

		require.NoError(t, err)
		// Output should still be readable despite long name
		outputStr := output.String()
		assert.Contains(t, outputStr, "Transaction Review:")
	})
}
