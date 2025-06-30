package llm

import (
	"context"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
)

// Client defines the interface for LLM providers.
type Client interface {
	Classify(ctx context.Context, prompt string) (ClassificationResponse, error)
	ClassifyWithRankings(ctx context.Context, prompt string) (RankingResponse, error)
	ClassifyMerchantBatch(ctx context.Context, prompt string) (MerchantBatchResponse, error)
	GenerateDescription(ctx context.Context, prompt string) (DescriptionResponse, error)
	// Analyze performs general-purpose AI analysis and returns raw response text.
	// This is used for complex analysis tasks that return arbitrary JSON or text.
	Analyze(ctx context.Context, prompt string, systemPrompt string) (string, error)
	// TODO: Add when implementing direction detection
	// ClassifyDirection(ctx context.Context, prompt string) (DirectionResponse, error)
}

// ClassificationResponse contains the LLM's classification result.
type ClassificationResponse struct {
	Category            string
	CategoryDescription string
	Confidence          float64
	IsNew               bool
}

// DescriptionResponse contains the LLM's generated description.
type DescriptionResponse struct {
	Description string
	Confidence  float64
}

// RankingResponse contains the LLM's category rankings result.
type RankingResponse struct {
	Rankings []CategoryRanking
}

// CategoryRanking represents a single category ranking from the LLM.
type CategoryRanking struct {
	Category    string
	Description string
	Score       float64
	IsNew       bool
}

// DirectionResponse contains the LLM's direction detection result.
type DirectionResponse struct {
	Direction  model.TransactionDirection
	Reasoning  string
	Confidence float64
}

// MerchantBatchRequest represents a single merchant in a batch classification request.
type MerchantBatchRequest struct {
	MerchantID        string
	MerchantName      string
	SampleTransaction model.Transaction
	TransactionCount  int
}

// MerchantBatchResponse contains classification results for multiple merchants.
type MerchantBatchResponse struct {
	Classifications []MerchantClassification
}

// MerchantClassification represents the classification result for a single merchant.
type MerchantClassification struct {
	MerchantID string
	Rankings   []CategoryRanking
}
