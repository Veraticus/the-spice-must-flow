package analysis

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/common"
	"github.com/Veraticus/the-spice-must-flow/internal/llm"
	"github.com/Veraticus/the-spice-must-flow/internal/service"
)

// LLMAnalysisAdapter wraps the LLM client with analysis-specific functionality.
type LLMAnalysisAdapter struct {
	client       llm.Client
	retryOptions service.RetryOptions
}

// Ensure LLMAnalysisAdapter implements LLMClient interface.
var _ LLMClient = (*LLMAnalysisAdapter)(nil)

// NewLLMAnalysisAdapter creates a new LLM adapter.
func NewLLMAnalysisAdapter(client llm.Client) *LLMAnalysisAdapter {
	return &LLMAnalysisAdapter{
		client: client,
		retryOptions: service.RetryOptions{
			MaxAttempts:  3,
			InitialDelay: 1 * time.Second,
			MaxDelay:     30 * time.Second,
			Multiplier:   2.0,
		},
	}
}

// NewLLMAnalysisAdapterWithRetry creates a new LLM adapter with custom retry options.
func NewLLMAnalysisAdapterWithRetry(client llm.Client, retryOptions service.RetryOptions) *LLMAnalysisAdapter {
	return &LLMAnalysisAdapter{
		client:       client,
		retryOptions: retryOptions,
	}
}

// AnalyzeTransactions performs AI analysis on transactions with retry logic.
func (a *LLMAnalysisAdapter) AnalyzeTransactions(ctx context.Context, prompt string) (string, error) {
	var responseJSON string
	var lastErr error

	err := common.WithRetry(ctx, func() error {
		// Call the LLM
		response, err := a.client.Classify(ctx, prompt)
		if err != nil {
			lastErr = err
			// Check if error is retryable
			return &common.RetryableError{
				Err:       fmt.Errorf("LLM request failed: %w", err),
				Retryable: isRetryableError(err),
			}
		}

		// The Classify method returns a ClassificationResponse, but for analysis
		// we expect the full JSON response in the Category field
		responseJSON = response.Category
		return nil
	}, a.retryOptions)

	if err != nil {
		if lastErr != nil {
			return "", lastErr
		}
		return "", err
	}

	return responseJSON, nil
}

// ValidateAndCorrectResponse attempts to fix invalid JSON responses.
func (a *LLMAnalysisAdapter) ValidateAndCorrectResponse(ctx context.Context, correctionPrompt string) (string, error) {
	var responseJSON string
	var lastErr error

	err := common.WithRetry(ctx, func() error {
		// Use a shorter retry for corrections
		response, err := a.client.Classify(ctx, correctionPrompt)
		if err != nil {
			lastErr = err
			return &common.RetryableError{
				Err:       fmt.Errorf("correction request failed: %w", err),
				Retryable: isRetryableError(err),
			}
		}

		responseJSON = response.Category
		return nil
	}, service.RetryOptions{
		MaxAttempts:  2, // Fewer attempts for corrections
		InitialDelay: 500 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		Multiplier:   2.0,
	})

	if err != nil {
		if lastErr != nil {
			return "", lastErr
		}
		return "", err
	}

	return responseJSON, nil
}

// AnalyzeWithFallback performs analysis with automatic correction on JSON errors.
func (a *LLMAnalysisAdapter) AnalyzeWithFallback(ctx context.Context, promptBuilder *TemplatePromptBuilder, promptData PromptData, validator ReportValidator) (*Report, error) {
	// Build the initial prompt
	prompt, err := promptBuilder.BuildAnalysisPrompt(promptData)
	if err != nil {
		return nil, fmt.Errorf("failed to build analysis prompt: %w", err)
	}

	// First attempt
	responseJSON, err := a.AnalyzeTransactions(ctx, prompt)
	if err == nil {
		// Validate the response
		report, validationErr := validator.Validate([]byte(responseJSON))
		if validationErr == nil {
			return report, nil
		}
		slog.Warn("Initial analysis response failed validation", "error", validationErr)
		err = validationErr // Set err for later use
	} else {
		slog.Warn("Initial analysis request failed", "error", err)
	}

	// If we get here, either the request failed or validation failed
	// Try to extract the error details for correction
	var correctionData CorrectionPromptData
	correctionData.OriginalPrompt = prompt

	if responseJSON != "" {
		// We got a response but it failed validation
		correctionData.InvalidResponse = responseJSON
		if validator != nil {
			section, line, col := validator.ExtractError([]byte(responseJSON), err)
			correctionData.ErrorSection = section
			correctionData.LineNumber = line
			correctionData.ColumnNumber = col
		}
	}

	correctionData.ErrorDetails = err.Error()

	// Build correction prompt
	correctionPrompt, err := promptBuilder.BuildCorrectionPrompt(correctionData)
	if err != nil {
		return nil, fmt.Errorf("failed to build correction prompt: %w", err)
	}

	// Attempt correction
	correctedJSON, err := a.ValidateAndCorrectResponse(ctx, correctionPrompt)
	if err != nil {
		return nil, fmt.Errorf("correction attempt failed: %w", err)
	}

	// Validate the corrected response
	finalReport, err := validator.Validate([]byte(correctedJSON))
	if err != nil {
		return nil, fmt.Errorf("corrected response still invalid: %w", err)
	}

	return finalReport, nil
}

// isRetryableError determines if an error should trigger a retry.
func isRetryableError(err error) bool {
	// Add specific error checks here based on your LLM client's error types
	// For now, we'll consider network errors and rate limits as retryable

	// Check for rate limit errors
	if err == common.ErrRateLimit {
		return true
	}

	// Check error message for common retryable patterns
	errMsg := err.Error()
	retryablePatterns := []string{
		"timeout",
		"connection",
		"temporary",
		"rate limit",
		"429", // HTTP Too Many Requests
		"503", // HTTP Service Unavailable
		"504", // HTTP Gateway Timeout
	}

	for _, pattern := range retryablePatterns {
		if contains(errMsg, pattern) {
			return true
		}
	}

	return false
}

// contains checks if a string contains a substring (case-insensitive).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsAt(s, substr)
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalFold(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

func equalFold(s, t string) bool {
	if len(s) != len(t) {
		return false
	}
	for i := 0; i < len(s); i++ {
		if toLower(s[i]) != toLower(t[i]) {
			return false
		}
	}
	return true
}

func toLower(b byte) byte {
	if 'A' <= b && b <= 'Z' {
		return b + ('a' - 'A')
	}
	return b
}
