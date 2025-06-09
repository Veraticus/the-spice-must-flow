package llm

import (
	"context"
)

// Client defines the interface for LLM providers.
type Client interface {
	Classify(ctx context.Context, prompt string) (ClassificationResponse, error)
}

// ClassificationResponse contains the LLM's classification result.
type ClassificationResponse struct {
	Category   string
	Confidence float64
}
