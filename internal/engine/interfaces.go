package engine

import (
	"context"

	"github.com/Veraticus/the-spice-must-flow/internal/llm"
	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/service"
)

// Classifier defines the contract for transaction categorization.
type Classifier interface {
	SuggestCategory(ctx context.Context, transaction model.Transaction, categories []string) (category string, confidence float64, isNew bool, description string, err error)
	SuggestCategoryRankings(ctx context.Context, transaction model.Transaction, categories []model.Category, checkPatterns []model.CheckPattern) (model.CategoryRankings, error)
	BatchSuggestCategories(ctx context.Context, transactions []model.Transaction, categories []string) ([]service.LLMSuggestion, error)
	SuggestCategoryBatch(ctx context.Context, requests []llm.MerchantBatchRequest, categories []model.Category) (map[string]model.CategoryRankings, error)
	GenerateCategoryDescription(ctx context.Context, categoryName string) (description string, confidence float64, err error)
}

// Prompter defines the contract for user interaction during classification.
type Prompter interface {
	ConfirmClassification(ctx context.Context, pending model.PendingClassification) (model.Classification, error)
	BatchConfirmClassifications(ctx context.Context, pending []model.PendingClassification) ([]model.Classification, error)
	GetCompletionStats() service.CompletionStats
}
