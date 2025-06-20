package llm

import (
	"context"
)

// Client defines the interface for LLM providers.
type Client interface {
	Classify(ctx context.Context, prompt string) (ClassificationResponse, error)
	GenerateDescription(ctx context.Context, prompt string) (DescriptionResponse, error)
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
