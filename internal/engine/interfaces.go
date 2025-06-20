package engine

import (
	"context"

	"github.com/joshsymonds/the-spice-must-flow/internal/model"
	"github.com/joshsymonds/the-spice-must-flow/internal/service"
)

// Classifier defines the contract for transaction categorization.
type Classifier interface {
	SuggestCategory(ctx context.Context, transaction model.Transaction, categories []string) (category string, confidence float64, isNew bool, description string, err error)
	BatchSuggestCategories(ctx context.Context, transactions []model.Transaction, categories []string) ([]service.LLMSuggestion, error)
	GenerateCategoryDescription(ctx context.Context, categoryName string) (description string, err error)
}

// Prompter defines the contract for user interaction during classification.
type Prompter interface {
	ConfirmClassification(ctx context.Context, pending model.PendingClassification) (model.Classification, error)
	BatchConfirmClassifications(ctx context.Context, pending []model.PendingClassification) ([]model.Classification, error)
	GetCompletionStats() service.CompletionStats
}
