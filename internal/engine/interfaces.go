package engine

import (
	"context"

	"github.com/joshsymonds/the-spice-must-flow/internal/model"
	"github.com/joshsymonds/the-spice-must-flow/internal/service"
)

// PlaidClient defines the contract for fetching data from Plaid.

// Classifier defines the contract for transaction categorization.
type Classifier interface {
	SuggestCategory(ctx context.Context, transaction model.Transaction) (string, float64, error)
	BatchSuggestCategories(ctx context.Context, transactions []model.Transaction) ([]service.LLMSuggestion, error)
}

// Prompter defines the contract for user interaction during classification.
type Prompter interface {
	ConfirmClassification(ctx context.Context, pending model.PendingClassification) (model.Classification, error)
	BatchConfirmClassifications(ctx context.Context, pending []model.PendingClassification) ([]model.Classification, error)
	GetCompletionStats() service.CompletionStats
}
