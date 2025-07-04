package engine

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/Veraticus/the-spice-must-flow/internal/llm"
	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/service"
)

// MockClassifier is a test implementation of the Classifier interface.
// It returns deterministic suggestions based on merchant name for testing.
type MockClassifier struct {
	generateDescriptionError    error
	batchResponse               map[string]model.CategoryRankings
	generateDescriptionResponse string
	calls                       []MockLLMCall
	mu                          sync.Mutex
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
func (m *MockClassifier) SuggestCategory(_ context.Context, transaction model.Transaction, _ []string) (string, float64, bool, string, error) {
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
	var description string

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
		description = "Expenses related to fitness equipment, gym memberships, and health activities"
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

	// Generate description for new categories
	if isNew && description == "" {
		description = fmt.Sprintf("Expenses related to %s", strings.ToLower(category))
	}

	// Record the call
	call := MockLLMCall{
		Transaction: transaction,
		Category:    category,
		Confidence:  confidence,
		Error:       nil,
	}
	m.calls = append(m.calls, call)

	return category, confidence, isNew, description, nil
}

// BatchSuggestCategories provides batch suggestions for multiple transactions.
func (m *MockClassifier) BatchSuggestCategories(ctx context.Context, transactions []model.Transaction, categories []string) ([]service.LLMSuggestion, error) {
	suggestions := make([]service.LLMSuggestion, len(transactions))

	for i, txn := range transactions {
		category, confidence, isNew, description, err := m.SuggestCategory(ctx, txn, categories)
		if err != nil {
			return nil, fmt.Errorf("failed to suggest category for transaction %s: %w", txn.ID, err)
		}

		suggestions[i] = service.LLMSuggestion{
			TransactionID:       txn.ID,
			Category:            category,
			Confidence:          confidence,
			IsNew:               isNew,
			CategoryDescription: description,
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
	m.batchResponse = nil
}

// SetBatchResponse sets a preset response for the next SuggestCategoryBatch call.
func (m *MockClassifier) SetBatchResponse(response map[string]model.CategoryRankings) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.batchResponse = response
}

// GenerateCategoryDescription generates a mock description for testing.
func (m *MockClassifier) GenerateCategoryDescription(_ context.Context, categoryName string) (string, float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Return configured response if set
	if m.generateDescriptionResponse != "" || m.generateDescriptionError != nil {
		return m.generateDescriptionResponse, 0.95, m.generateDescriptionError
	}

	// Default: Generate a simple description based on the category name
	return fmt.Sprintf("Expenses related to %s and associated services", strings.ToLower(categoryName)), 0.95, nil
}

// SetGenerateDescriptionResponse configures the response for GenerateCategoryDescription calls.
func (m *MockClassifier) SetGenerateDescriptionResponse(description string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.generateDescriptionResponse = description
	m.generateDescriptionError = err
}

// SuggestCategoryRankings provides deterministic category rankings for testing.
func (m *MockClassifier) SuggestCategoryRankings(_ context.Context, transaction model.Transaction, categories []model.Category, _ []model.CheckPattern) (model.CategoryRankings, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Create rankings for all provided categories
	rankings := make(model.CategoryRankings, 0, len(categories)+1) // +1 for potential new category

	merchantLower := strings.ToLower(transaction.MerchantName)
	if merchantLower == "" {
		merchantLower = strings.ToLower(transaction.Name)
	}

	// Score each existing category
	for _, cat := range categories {
		var score float64
		catLower := strings.ToLower(cat.Name)

		// Special handling for CHECK transactions
		if transaction.Type == "CHECK" {
			// For checks, give reasonable base scores to common check categories
			switch {
			case strings.Contains(catLower, "utilities"):
				score = 0.50
			case strings.Contains(catLower, "home") && strings.Contains(catLower, "services"):
				score = 0.45
			case strings.Contains(catLower, "insurance"):
				score = 0.40
			case strings.Contains(catLower, "rent"):
				score = 0.55
			case strings.Contains(catLower, "other"):
				score = 0.20
			default:
				score = 0.10
			}
		} else {
			// Deterministic scoring based on merchant and category names
			switch {
			case strings.Contains(merchantLower, "starbucks") && strings.Contains(catLower, "coffee"):
				score = 0.95
			case strings.Contains(merchantLower, "starbucks") && strings.Contains(catLower, "dining"):
				score = 0.82
			case strings.Contains(merchantLower, "amazon") && strings.Contains(catLower, "shopping"):
				score = 0.80
			case strings.Contains(merchantLower, "amazon") && strings.Contains(catLower, "office"):
				score = 0.70
			case strings.Contains(merchantLower, "whole foods") && strings.Contains(catLower, "groceries"):
				score = 0.95
			case strings.Contains(merchantLower, "shell") && strings.Contains(catLower, "transportation"):
				score = 0.90
			case strings.Contains(merchantLower, "netflix") && strings.Contains(catLower, "entertainment"):
				score = 0.98
			default:
				// Base score on partial matches
				switch {
				case strings.Contains(catLower, "shopping"):
					score = 0.30
				case strings.Contains(catLower, "misc") || strings.Contains(catLower, "other"):
					score = 0.20
				default:
					score = 0.10
				}
			}
		}

		rankings = append(rankings, model.CategoryRanking{
			Category: cat.Name,
			Score:    score,
			IsNew:    false,
		})
	}

	// Add a new category suggestion for fitness merchants
	if strings.Contains(merchantLower, "peloton") || strings.Contains(merchantLower, "fitness") {
		var alreadyHasFitness bool
		for _, cat := range categories {
			if strings.Contains(strings.ToLower(cat.Name), "fitness") || strings.Contains(strings.ToLower(cat.Name), "health") {
				alreadyHasFitness = true
				break
			}
		}

		if !alreadyHasFitness {
			rankings = append(rankings, model.CategoryRanking{
				Category:    "Fitness & Health",
				Score:       0.75,
				IsNew:       true,
				Description: "Expenses related to fitness equipment, gym memberships, and health activities",
			})
		}
	}

	// Sort rankings by score
	rankings.Sort()

	// Record the call
	call := MockLLMCall{
		Transaction: transaction,
		Category:    rankings.Top().Category,
		Confidence:  rankings.Top().Score,
		Error:       nil,
	}
	m.calls = append(m.calls, call)

	return rankings, nil
}

// SuggestCategoryBatch provides deterministic batch category suggestions for testing.
func (m *MockClassifier) SuggestCategoryBatch(_ context.Context, requests []llm.MerchantBatchRequest, categories []model.Category) (map[string]model.CategoryRankings, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Record calls for each merchant in the batch
	for _, req := range requests {
		call := MockLLMCall{
			Transaction: req.SampleTransaction,
			Error:       nil,
		}
		m.calls = append(m.calls, call)
	}

	// If preset batch response is available, use it
	if m.batchResponse != nil {
		response := m.batchResponse
		m.batchResponse = nil // Clear after use
		return response, nil
	}

	results := make(map[string]model.CategoryRankings)

	// Process each merchant request
	for _, req := range requests {
		// Create rankings similar to SuggestCategoryRankings
		rankings := make(model.CategoryRankings, 0, len(categories))

		merchantLower := strings.ToLower(req.MerchantName)

		// Score each category
		for _, cat := range categories {
			var score float64
			catLower := strings.ToLower(cat.Name)

			// Simple scoring based on merchant name patterns
			switch {
			case strings.Contains(merchantLower, "starbucks") && strings.Contains(catLower, "coffee"):
				score = 0.95
			case strings.Contains(merchantLower, "whole foods") && strings.Contains(catLower, "groceries"):
				score = 0.95
			case strings.Contains(merchantLower, "walmart") && strings.Contains(catLower, "groceries"):
				score = 0.90
			case strings.Contains(merchantLower, "target") && strings.Contains(catLower, "department"):
				score = 0.88
			case strings.Contains(merchantLower, "shell") && strings.Contains(catLower, "gas"):
				score = 0.92
			case strings.Contains(merchantLower, "check") && req.SampleTransaction.Type == "CHECK":
				// For check transactions, give base scores similar to SuggestCategoryRankings
				switch {
				case strings.Contains(catLower, "utilities"):
					score = 0.50
				case strings.Contains(catLower, "home") && strings.Contains(catLower, "services"):
					score = 0.45
				case strings.Contains(catLower, "insurance"):
					score = 0.40
				case strings.Contains(catLower, "rent"):
					score = 0.55
				case strings.Contains(catLower, "other"):
					score = 0.20
				default:
					score = 0.10
				}
			default:
				// Base score on partial matches, similar to SuggestCategoryRankings
				switch {
				case strings.Contains(catLower, "shopping"):
					score = 0.30
				case strings.Contains(catLower, "misc") || strings.Contains(catLower, "other"):
					score = 0.20
				default:
					score = 0.10
				}
			}

			rankings = append(rankings, model.CategoryRanking{
				Category:    cat.Name,
				Score:       score,
				IsNew:       false,
				Description: cat.Description,
			})
		}

		// Sort rankings
		rankings.Sort()

		// Store results
		results[req.MerchantID] = rankings
	}

	return results, nil
}
