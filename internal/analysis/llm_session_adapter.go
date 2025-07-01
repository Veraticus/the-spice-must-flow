package analysis

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/llm"
	"github.com/Veraticus/the-spice-must-flow/internal/service"
)

// SessionLLMAnalysisAdapter wraps an LLM SessionClient with analysis-specific functionality.
type SessionLLMAnalysisAdapter struct {
	*LLMAnalysisAdapter
	sessionClient llm.SessionClient
	jsonPatcher   *llm.JSONPatcher
}

// Ensure SessionLLMAnalysisAdapter implements SessionLLMClient interface.
var _ SessionLLMClient = (*SessionLLMAnalysisAdapter)(nil)

// NewSessionLLMAnalysisAdapter creates a new session-aware LLM adapter.
func NewSessionLLMAnalysisAdapter(client llm.SessionClient) *SessionLLMAnalysisAdapter {
	// Generate unique temp directory for this session
	tempBaseDir := fmt.Sprintf("/tmp/spice-analysis-%d", os.Getpid())

	// Create the base adapter
	baseAdapter := &LLMAnalysisAdapter{
		client: client,
		retryOptions: service.RetryOptions{
			MaxAttempts:  3,
			InitialDelay: 1 * time.Second,
			MaxDelay:     30 * time.Second,
			Multiplier:   2.0,
		},
		tempFileManager: NewTempFileManager(tempBaseDir),
	}

	return &SessionLLMAnalysisAdapter{
		LLMAnalysisAdapter: baseAdapter,
		sessionClient:      client,
		jsonPatcher:        llm.NewJSONPatcher(),
	}
}

// AnalyzeTransactionsWithSession performs analysis with session support.
func (a *SessionLLMAnalysisAdapter) AnalyzeTransactionsWithSession(ctx context.Context, prompt string, sessionID string) (SessionAnalysisResult, error) {
	// System prompt for analysis
	systemPrompt := a.getAnalysisSystemPrompt()

	slog.Debug("Starting session-based analysis",
		"session_id", sessionID,
		"is_new_session", sessionID == "",
		"prompt_length", len(prompt),
	)

	// Use the session-aware Analyze method
	result, err := a.sessionClient.AnalyzeWithSession(ctx, prompt, systemPrompt, sessionID)
	if err != nil {
		return SessionAnalysisResult{}, fmt.Errorf("session analysis failed: %w", err)
	}

	slog.Debug("Session analysis completed",
		"session_id", result.SessionID,
		"num_turns", result.NumTurns,
		"total_cost", result.TotalCost,
		"response_length", len(result.Response),
	)

	return SessionAnalysisResult{
		Response:  result.Response,
		SessionID: result.SessionID,
		TotalCost: result.TotalCost,
		NumTurns:  result.NumTurns,
	}, nil
}

// AnalyzeTransactionsWithFileSession performs file-based analysis with session support.
func (a *SessionLLMAnalysisAdapter) AnalyzeTransactionsWithFileSession(ctx context.Context, prompt string, transactionData map[string]any, sessionID string) (SessionAnalysisResult, error) {
	// Create temporary file with transaction data
	filePath, cleanup, err := a.tempFileManager.CreateTransactionFile(transactionData)
	if err != nil {
		return SessionAnalysisResult{}, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer cleanup()

	// Modify prompt to reference the file
	filePrompt := fmt.Sprintf(`The transaction data is stored in a JSON file at: %s

Please read this file to access the full transaction data before performing your analysis.

%s`, filePath, prompt)

	slog.Debug("Using file-based session analysis",
		"file_path", filePath,
		"session_id", sessionID,
		"is_new_session", sessionID == "",
		"temp_dir", a.tempFileManager.GetBaseDir(),
	)

	return a.AnalyzeTransactionsWithSession(ctx, filePrompt, sessionID)
}

// RequestCorrection asks the LLM to provide patches for specific errors.
func (a *SessionLLMAnalysisAdapter) RequestCorrection(ctx context.Context, request CorrectionRequest, sessionID string) (CorrectionResponse, error) {
	if sessionID == "" {
		return CorrectionResponse{}, fmt.Errorf("session ID required for corrections")
	}

	// Build a focused prompt for corrections
	prompt := a.buildCorrectionPrompt(request)

	// System prompt specifically for corrections
	systemPrompt := `You are a JSON correction specialist. Your previous analysis had validation errors that need to be fixed.

You MUST respond with ONLY a valid JSON object containing patches. Do not include any explanatory text before or after the JSON.

The response must follow this exact format:
{
  "patches": [
    {
      "path": "the.json.path[0].to.fix",
      "value": "the corrected value"
    }
  ],
  "reason": "Brief explanation of the corrections"
}

Only provide patches for the specific errors mentioned. Do not modify any other parts of the JSON.`

	slog.Debug("Requesting correction",
		"session_id", sessionID,
		"num_errors", len(request.ErrorLocations),
	)

	// Continue the session with correction request
	result, err := a.sessionClient.AnalyzeWithSession(ctx, prompt, systemPrompt, sessionID)
	if err != nil {
		return CorrectionResponse{}, fmt.Errorf("correction request failed: %w", err)
	}

	// Parse the correction response
	var correctionResp CorrectionResponse
	if err := json.Unmarshal([]byte(result.Response), &correctionResp); err != nil {
		slog.Error("Failed to parse correction response",
			"error", err,
			"response", result.Response,
		)
		return CorrectionResponse{}, fmt.Errorf("invalid correction response format: %w", err)
	}

	slog.Debug("Correction response received",
		"num_patches", len(correctionResp.Patches),
		"session_id", result.SessionID,
		"total_turns", result.NumTurns,
	)

	return correctionResp, nil
}

// buildCorrectionPrompt creates a focused prompt for error corrections.
func (a *SessionLLMAnalysisAdapter) buildCorrectionPrompt(request CorrectionRequest) string {
	prompt := fmt.Sprintf(`The analysis response has validation errors that need to be corrected.

Validation Error: %s

Specific issues to fix:
`, request.ValidationError)

	for i, err := range request.ErrorLocations {
		prompt += fmt.Sprintf(`
%d. Path: %s
   Error: %s
   Current Value: %v
   Expected: %s
`, i+1, err.ErrorPath, err.ErrorDescription, err.CurrentValue, err.ExpectedFormat)
	}

	prompt += fmt.Sprintf(`

%s

Please provide JSON patches to fix only these specific errors. Do not modify any other parts of the response.`, request.Instructions)

	return prompt
}

// getAnalysisSystemPrompt returns the system prompt for analysis.
func (a *SessionLLMAnalysisAdapter) getAnalysisSystemPrompt() string {
	return `You are an AI assistant specialized in financial transaction analysis. Your task is to analyze transaction categorization patterns and provide detailed insights. 

IMPORTANT: Please ultrathink through this analysis carefully, examining patterns and inconsistencies thoroughly before responding.

You MUST respond with ONLY a valid JSON object that matches the provided schema. Do not include any explanatory text, markdown formatting, or commentary before or after the JSON. Start your response directly with { and end with }.

If the prompt references a file path (starting with /tmp/spice-analysis-), please read that file to get the full transaction data before performing your analysis.

Focus on:
1. Detecting inconsistent categorizations
2. Identifying missing pattern rules
3. Suggesting improvements
4. Calculating coherence scores

Be specific and actionable in your recommendations.`
}
