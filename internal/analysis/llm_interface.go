package analysis

import (
	"context"
)

// LLMClient defines the interface for LLM operations specific to analysis.
// This wraps the general LLM client with analysis-specific methods.
type LLMClient interface {
	// AnalyzeTransactions performs AI analysis on transactions and returns raw JSON.
	AnalyzeTransactions(ctx context.Context, prompt string) (string, error)
	// AnalyzeTransactionsWithFile performs AI analysis using file-based approach for large datasets.
	AnalyzeTransactionsWithFile(ctx context.Context, prompt string, transactionData map[string]interface{}) (string, error)
	// ValidateAndCorrectResponse attempts to fix invalid JSON responses.
	ValidateAndCorrectResponse(ctx context.Context, correctionPrompt string) (string, error)
}
