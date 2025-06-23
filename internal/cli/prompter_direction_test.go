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

func TestPrompter_ConfirmTransactionDirection(t *testing.T) {
	tests := []struct {
		name              string
		userInput         string
		expectedDirection model.TransactionDirection
		expectedOutput    []string
		pending           engine.PendingDirection
		expectError       bool
	}{
		{
			name: "accept AI suggestion",
			pending: engine.PendingDirection{
				MerchantName:     "Test Merchant",
				TransactionCount: 3,
				SampleTransaction: model.Transaction{
					Name:   "TEST PURCHASE",
					Amount: 100.00,
					Date:   time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
					Type:   "DEBIT",
				},
				SuggestedDirection: model.DirectionExpense,
				Confidence:         0.75,
				Reasoning:          "Looks like a purchase transaction",
			},
			userInput:         "a",
			expectedDirection: model.DirectionExpense,
			expectedOutput: []string{
				"Direction Detection - Test Merchant",
				"Found 3 transaction(s)",
				"Sample transaction:",
				"Description: TEST PURCHASE",
				"Amount: $100.00",
				"Date: Jan 15, 2024",
				"Type: DEBIT",
				"Suggested: Expense (75% confidence)",
				"Reasoning: Looks like a purchase transaction",
				"[1] Income",
				"[2] Expense",
				"[3] Transfer",
				"[A] Accept AI suggestion (Expense)",
			},
		},
		{
			name: "override with income",
			pending: engine.PendingDirection{
				MerchantName:     "Employer Corp",
				TransactionCount: 1,
				SampleTransaction: model.Transaction{
					Name:   "PAYROLL DEPOSIT",
					Amount: 5000.00,
					Date:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				},
				SuggestedDirection: model.DirectionTransfer,
				Confidence:         0.60,
				Reasoning:          "Could be a transfer",
			},
			userInput:         "1",
			expectedDirection: model.DirectionIncome,
			expectedOutput: []string{
				"Direction Detection - Employer Corp",
				"[1] Income",
				"[A] Accept AI suggestion (Transfer)",
			},
		},
		{
			name: "select expense",
			pending: engine.PendingDirection{
				MerchantName:     "Unknown Vendor",
				TransactionCount: 5,
				SampleTransaction: model.Transaction{
					Name:   "VENDOR PAYMENT",
					Amount: 250.00,
					Date:   time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC),
				},
				SuggestedDirection: model.DirectionTransfer,
				Confidence:         0.40,
				Reasoning:          "Uncertain transaction type",
			},
			userInput:         "2",
			expectedDirection: model.DirectionExpense,
		},
		{
			name: "select transfer",
			pending: engine.PendingDirection{
				MerchantName:     "Bank Transfer",
				TransactionCount: 2,
				SampleTransaction: model.Transaction{
					Name:   "TRANSFER TO SAVINGS",
					Amount: 1000.00,
					Date:   time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC),
				},
				SuggestedDirection: model.DirectionExpense,
				Confidence:         0.50,
				Reasoning:          "Money movement detected",
			},
			userInput:         "3",
			expectedDirection: model.DirectionTransfer,
		},
		{
			name: "context cancellation",
			pending: engine.PendingDirection{
				MerchantName:     "Test",
				TransactionCount: 1,
				SampleTransaction: model.Transaction{
					Name:   "TEST",
					Amount: 10.00,
					Date:   time.Now(),
				},
				SuggestedDirection: model.DirectionExpense,
				Confidence:         0.50,
				Reasoning:          "Test",
			},
			userInput:   "", // Will be canceled
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			var cancelCtx context.Context
			var cancel context.CancelFunc

			if tt.expectError && tt.userInput == "" {
				cancelCtx, cancel = context.WithCancel(ctx)
				ctx = cancelCtx
				// Cancel immediately to simulate context cancellation
				cancel()
			}

			// Create buffers for I/O
			var output bytes.Buffer
			inputReader := NewMockReader(tt.userInput)

			// Create prompter
			prompter := &Prompter{
				writer: &output,
				reader: inputReader,
				ctx:    ctx,
			}

			// Run the test
			direction, err := prompter.ConfirmTransactionDirection(ctx, tt.pending)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedDirection, direction)

			// Check output contains expected strings
			outputStr := output.String()
			for _, expected := range tt.expectedOutput {
				assert.Contains(t, outputStr, expected,
					"Output should contain: %s", expected)
			}
		})
	}
}

// TestDirectionDisplay tests the direction display formatting.
func TestDirectionDisplay(t *testing.T) {
	tests := []struct {
		direction model.TransactionDirection
		expected  string
	}{
		{model.DirectionIncome, "Income"},
		{model.DirectionExpense, "Expense"},
		{model.DirectionTransfer, "Transfer"},
		{model.TransactionDirection("custom"), "custom"},
	}

	for _, tt := range tests {
		t.Run(string(tt.direction), func(t *testing.T) {
			result := getDirectionDisplay(tt.direction)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// MockReader simulates user input for testing.
type MockReader struct {
	input string
	read  bool
}

func NewMockReader(input string) *NonBlockingReader {
	// Create a reader that returns the input string
	reader := strings.NewReader(input + "\n")
	return NewNonBlockingReader(reader)
}
