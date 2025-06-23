package engine

import (
	"context"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/service"
)

// Classifier defines the contract for transaction categorization.
type Classifier interface {
	SuggestCategory(ctx context.Context, transaction model.Transaction, categories []string) (category string, confidence float64, isNew bool, description string, err error)
	SuggestCategoryRankings(ctx context.Context, transaction model.Transaction, categories []model.Category, checkPatterns []model.CheckPattern) (model.CategoryRankings, error)
	BatchSuggestCategories(ctx context.Context, transactions []model.Transaction, categories []string) ([]service.LLMSuggestion, error)
	GenerateCategoryDescription(ctx context.Context, categoryName string) (description string, err error)
	SuggestTransactionDirection(ctx context.Context, transaction model.Transaction) (direction model.TransactionDirection, confidence float64, reasoning string, err error)
}

// PendingDirection represents a direction detection that needs user confirmation.
type PendingDirection struct {
	MerchantName       string
	SuggestedDirection model.TransactionDirection
	Reasoning          string
	SampleTransaction  model.Transaction
	TransactionCount   int
	Confidence         float64
}

// Prompter defines the contract for user interaction during classification.
type Prompter interface {
	ConfirmClassification(ctx context.Context, pending model.PendingClassification) (model.Classification, error)
	BatchConfirmClassifications(ctx context.Context, pending []model.PendingClassification) ([]model.Classification, error)
	ConfirmTransactionDirection(ctx context.Context, pending PendingDirection) (model.TransactionDirection, error)
	GetCompletionStats() service.CompletionStats
}
