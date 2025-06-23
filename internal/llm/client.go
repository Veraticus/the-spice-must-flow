package llm

import (
	"context"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
)

// Client defines the interface for LLM providers.
type Client interface {
	Classify(ctx context.Context, prompt string) (ClassificationResponse, error)
	ClassifyWithRankings(ctx context.Context, prompt string) (RankingResponse, error)
	GenerateDescription(ctx context.Context, prompt string) (DescriptionResponse, error)
	ClassifyDirection(ctx context.Context, prompt string) (DirectionResponse, error)
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
