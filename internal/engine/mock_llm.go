package engine

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/joshsymonds/the-spice-must-flow/internal/model"
	"github.com/joshsymonds/the-spice-must-flow/internal/service"
)

// MockClassifier is a test implementation of the Classifier interface.
// It returns deterministic suggestions based on merchant name for testing.
type MockClassifier struct {
	calls []MockLLMCall
	mu    sync.Mutex
}

// MockLLMCall records details of a classification request.
type MockLLMCall struct {
	Error       error
	Category    string
	Transaction model.Transaction
	Confidence  float64
}

// NewMockClassifier creates a new mock LLM classifier.
func NewMockClassifier() *MockClassifier {
	return &MockClassifier{
		calls: make([]MockLLMCall, 0),
	}
}

// SuggestCategory provides deterministic category suggestions based on merchant name.
func (m *MockClassifier) SuggestCategory(_ context.Context, transaction model.Transaction, categories []string) (string, float64, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Determine category based on merchant name patterns
	merchantLower := strings.ToLower(transaction.MerchantName)
	if merchantLower == "" {
		merchantLower = strings.ToLower(transaction.Name)
	}

	var category string
	var confidence float64
	var isNew bool

	switch {
	case strings.Contains(merchantLower, "starbucks") || strings.Contains(merchantLower, "coffee"):
		category = "Coffee & Dining"
		confidence = 0.92
	case strings.Contains(merchantLower, "amazon"):
		// Amazon has variable confidence based on amount
		switch {
		case transaction.Amount < 50:
			category = "Office Supplies"
			confidence = 0.75
		case transaction.Amount < 200:
			category = "Shopping"
			confidence = 0.80
		default:
			category = "Computer & Electronics"
			confidence = 0.85
		}
	case strings.Contains(merchantLower, "whole foods") || strings.Contains(merchantLower, "grocery"):
		category = "Groceries"
		confidence = 0.95
	case strings.Contains(merchantLower, "shell") || strings.Contains(merchantLower, "chevron") || strings.Contains(merchantLower, "gas"):
		category = "Transportation"
		confidence = 0.90
	case strings.Contains(merchantLower, "netflix") || strings.Contains(merchantLower, "spotify") || strings.Contains(merchantLower, "hulu"):
		category = "Entertainment"
		confidence = 0.98
	case strings.Contains(merchantLower, "target") || strings.Contains(merchantLower, "walmart"):
		category = "Shopping"
		confidence = 0.85
	case strings.Contains(merchantLower, "restaurant") || strings.Contains(merchantLower, "diner") || strings.Contains(merchantLower, "grill"):
		category = "Food & Dining"
		confidence = 0.88
	case strings.Contains(merchantLower, "peloton") || strings.Contains(merchantLower, "fitness"):
		// New category suggestion for fitness-related merchants
		category = "Fitness & Health"
		confidence = 0.75 // Below 0.9 threshold
		isNew = true
	default:
		// Default categorization based on amount
		switch {
		case transaction.Amount < 25:
			category = "Miscellaneous"
			confidence = 0.60
		case transaction.Amount < 100:
			category = "Shopping"
			confidence = 0.65
		default:
			category = "Other Expenses"
			confidence = 0.55
		}
	}

	// Set isNew if confidence is below 0.9 and not already set
	if confidence < 0.9 && !isNew {
		isNew = true
	}

	// Record the call
	call := MockLLMCall{
		Transaction: transaction,
		Category:    category,
		Confidence:  confidence,
		Error:       nil,
	}
	m.calls = append(m.calls, call)

	return category, confidence, isNew, nil
}

// BatchSuggestCategories provides batch suggestions for multiple transactions.
func (m *MockClassifier) BatchSuggestCategories(ctx context.Context, transactions []model.Transaction, categories []string) ([]service.LLMSuggestion, error) {
	suggestions := make([]service.LLMSuggestion, len(transactions))

	for i, txn := range transactions {
		category, confidence, isNew, err := m.SuggestCategory(ctx, txn, categories)
		if err != nil {
			return nil, fmt.Errorf("failed to suggest category for transaction %s: %w", txn.ID, err)
		}

		suggestions[i] = service.LLMSuggestion{
			TransactionID: txn.ID,
			Category:      category,
			Confidence:    confidence,
			IsNew:         isNew,
		}
	}

	return suggestions, nil
}

// GetCalls returns all recorded calls for verification in tests.
func (m *MockClassifier) GetCalls() []MockLLMCall {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Return a copy to prevent external modification
	calls := make([]MockLLMCall, len(m.calls))
	copy(calls, m.calls)
	return calls
}

// CallCount returns the number of times SuggestCategory was called.
func (m *MockClassifier) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

// Reset clears all recorded calls.
func (m *MockClassifier) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = make([]MockLLMCall, 0)
}
