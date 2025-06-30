package analysis

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/common"
	"github.com/Veraticus/the-spice-must-flow/internal/llm"
	"github.com/Veraticus/the-spice-must-flow/internal/service"
)

// LLMAnalysisAdapter wraps the LLM client with analysis-specific functionality.
type LLMAnalysisAdapter struct {
	client          llm.Client
	tempFileManager *TempFileManager
	retryOptions    service.RetryOptions
}

// Ensure LLMAnalysisAdapter implements LLMClient interface.
var _ LLMClient = (*LLMAnalysisAdapter)(nil)

// NewLLMAnalysisAdapter creates a new LLM adapter.
func NewLLMAnalysisAdapter(client llm.Client) *LLMAnalysisAdapter {
	// Generate unique temp directory for this session
	tempBaseDir := fmt.Sprintf("/tmp/spice-analysis-%d", os.Getpid())

	return &LLMAnalysisAdapter{
		client: client,
		retryOptions: service.RetryOptions{
			MaxAttempts:  3,
			InitialDelay: 1 * time.Second,
			MaxDelay:     30 * time.Second,
			Multiplier:   2.0,
		},
		tempFileManager: NewTempFileManager(tempBaseDir),
	}
}

// NewLLMAnalysisAdapterWithRetry creates a new LLM adapter with custom retry options.
func NewLLMAnalysisAdapterWithRetry(client llm.Client, retryOptions service.RetryOptions) *LLMAnalysisAdapter {
	// Generate unique temp directory for this session
	tempBaseDir := fmt.Sprintf("/tmp/spice-analysis-%d", os.Getpid())

	return &LLMAnalysisAdapter{
		client:          client,
		retryOptions:    retryOptions,
		tempFileManager: NewTempFileManager(tempBaseDir),
	}
}

// AnalyzeTransactions performs AI analysis on transactions with retry logic.
func (a *LLMAnalysisAdapter) AnalyzeTransactions(ctx context.Context, prompt string) (string, error) {
	var responseJSON string
	var lastErr error

	// System prompt for analysis - include "ultrathink" for better reasoning
	systemPrompt := `You are an AI assistant specialized in financial transaction analysis. Your task is to analyze transaction categorization patterns and provide detailed insights. 

IMPORTANT: Please ultrathink through this analysis carefully, examining patterns and inconsistencies thoroughly before responding.

You MUST respond with ONLY a valid JSON object that matches the provided schema. Do not include any explanatory text, markdown formatting, or commentary before or after the JSON. Start your response directly with { and end with }.

If the prompt references a file path (starting with /tmp/spice-analysis-), please read that file to get the full transaction data before performing your analysis.

Focus on:
1. Detecting inconsistent categorizations
2. Identifying missing pattern rules
3. Suggesting improvements
4. Calculating coherence scores

Be specific and actionable in your recommendations.`

	err := common.WithRetry(ctx, func() error {
		// Log the full prompt in debug mode
		slog.Debug("Sending analysis request to LLM",
			"prompt_length", len(prompt),
			"system_prompt_length", len(systemPrompt),
		)

		// Use the new Analyze method for general-purpose analysis
		response, err := a.client.Analyze(ctx, prompt, systemPrompt)
		if err != nil {
			lastErr = err
			slog.Debug("LLM request failed", "error", err)
			// Check if error is retryable
			return &common.RetryableError{
				Err:       fmt.Errorf("LLM request failed: %w", err),
				Retryable: isRetryableError(err),
			}
		}

		// Log the response in debug mode
		slog.Debug("Received LLM response",
			"response_length", len(response),
			"response_preview", truncateForLog(response, 200),
		)

		responseJSON = response
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

	// System prompt for correction
	systemPrompt := `You are a JSON correction specialist. Your task is to fix the malformed JSON response based on the error information provided.

You MUST respond with ONLY a valid JSON object. Do not include any explanatory text, markdown formatting, or commentary before or after the JSON. Start your response directly with { and end with }.

Focus on:
1. Fixing the specific JSON syntax error mentioned
2. Maintaining all the original data and structure
3. Ensuring proper JSON formatting throughout`

	err := common.WithRetry(ctx, func() error {
		// Use the Analyze method for corrections too
		response, err := a.client.Analyze(ctx, correctionPrompt, systemPrompt)
		if err != nil {
			lastErr = err
			return &common.RetryableError{
				Err:       fmt.Errorf("correction request failed: %w", err),
				Retryable: isRetryableError(err),
			}
		}

		responseJSON = response
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

// truncateForLog truncates a string for logging, preserving readability.
func truncateForLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// AnalyzeTransactionsWithFile performs AI analysis using file-based approach for large datasets.
func (a *LLMAnalysisAdapter) AnalyzeTransactionsWithFile(ctx context.Context, prompt string, transactionData map[string]interface{}) (string, error) {
	// Create temporary file with transaction data
	filePath, cleanup, err := a.tempFileManager.CreateTransactionFile(transactionData)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer cleanup()

	// Modify prompt to reference the file
	filePrompt := fmt.Sprintf(`The transaction data is stored in a JSON file at: %s

Please read this file to access the full transaction data before performing your analysis.

%s`, filePath, prompt)

	slog.Debug("Using file-based analysis",
		"file_path", filePath,
		"temp_dir", a.tempFileManager.GetBaseDir(),
		"original_prompt_length", len(prompt),
	)

	// Use the standard AnalyzeTransactions with the file reference
	return a.AnalyzeTransactions(ctx, filePrompt)
}
