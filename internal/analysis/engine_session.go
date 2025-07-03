package analysis

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/Veraticus/the-spice-must-flow/internal/llm"
	"github.com/Veraticus/the-spice-must-flow/internal/model"
)

// performSessionBasedAnalysis performs analysis using session-based error correction.
func (e *Engine) performSessionBasedAnalysis(ctx context.Context, session *Session, promptData PromptData, allTransactions []model.Transaction, progress ProgressCallback) (*Report, error) {
	// Check if we have a session-capable LLM client
	sessionClient, ok := e.deps.LLMClient.(SessionLLMClient)
	if !ok {
		// Fallback to old behavior if not session-capable
		slog.Info("LLM client does not support sessions, using legacy retry")
		return e.performAnalysisWithRecovery(ctx, session, promptData, allTransactions, progress)
	}

	// Build the initial prompt
	prompt, err := e.deps.PromptBuilder.BuildAnalysisPrompt(promptData)
	if err != nil {
		return nil, fmt.Errorf("failed to build analysis prompt: %w", err)
	}

	// Prepare transaction data for file-based analysis
	transactionData := e.prepareTransactionData(allTransactions)

	// Start analysis with new session
	progress("Starting AI analysis", 40)

	analysisResult, err := sessionClient.AnalyzeTransactionsWithFileSession(ctx, prompt, transactionData, "")
	if err != nil {
		return nil, fmt.Errorf("initial analysis failed: %w", err)
	}

	// Track the current JSON response and session
	currentJSON := json.RawMessage(analysisResult.Response)
	sessionID := analysisResult.SessionID
	totalCost := analysisResult.TotalCost

	slog.Info("Initial analysis complete",
		"session_id", sessionID,
		"response_length", len(analysisResult.Response),
		"cost", totalCost,
	)

	// Validate and iteratively correct
	const maxCorrectionAttempts = 5
	patcher := llm.NewJSONPatcher()

	for attempt := 1; attempt <= maxCorrectionAttempts; attempt++ {
		// Update session info
		session.Attempts = attempt
		session.Status = StatusValidating
		if updateErr := e.deps.SessionStore.Update(ctx, session); updateErr != nil {
			slog.Warn("Failed to update session", "error", updateErr)
		}

		progress(fmt.Sprintf("Validating response (attempt %d)", attempt), 50+attempt*10)

		// Validate current JSON
		report, validationErr := e.deps.Validator.Validate(currentJSON)
		if validationErr == nil {
			// Success!
			slog.Info("Analysis validated successfully",
				"session_id", sessionID,
				"total_attempts", attempt,
				"total_cost", totalCost,
			)
			return report, nil
		}

		// Extract specific validation errors
		slog.Debug("Validation failed",
			"attempt", attempt,
			"error", validationErr,
		)

		if attempt == maxCorrectionAttempts {
			return nil, fmt.Errorf("validation failed after %d correction attempts: %w", attempt, validationErr)
		}

		// Build correction request from validation error
		correctionReq := e.buildCorrectionRequest(currentJSON, validationErr)

		progress(fmt.Sprintf("Requesting corrections (attempt %d)", attempt), 60+attempt*5)

		// Request corrections using the same session
		correctionResp, err := sessionClient.RequestCorrection(ctx, correctionReq, sessionID)
		if err != nil {
			return nil, fmt.Errorf("correction request %d failed: %w", attempt, err)
		}

		slog.Debug("Received corrections",
			"num_patches", len(correctionResp.Patches),
			"reason", correctionResp.Reason,
		)

		// Apply patches to current JSON
		patchedJSON, err := patcher.ApplyPatches(currentJSON, correctionResp.Patches)
		if err != nil {
			return nil, fmt.Errorf("failed to apply patches: %w", err)
		}

		// Update current JSON for next iteration
		currentJSON = patchedJSON

		// Log patch application
		for i, patch := range correctionResp.Patches {
			slog.Debug("Applied patch",
				"index", i,
				"path", patch.Path,
				"value_preview", fmt.Sprintf("%v", patch.Value)[:100],
			)
		}
	}

	return nil, fmt.Errorf("exhausted correction attempts")
}

// buildCorrectionRequest creates a correction request from validation errors.
func (e *Engine) buildCorrectionRequest(currentJSON json.RawMessage, validationErr error) CorrectionRequest {
	req := CorrectionRequest{
		ValidationError: validationErr.Error(),
		Instructions:    "Please provide minimal JSON patches to fix only the validation errors. Do not change any other parts of the response.",
	}

	// Try to extract specific error locations
	errorMsg := validationErr.Error()

	// Parse common validation error patterns
	if strings.Contains(errorMsg, "transaction IDs required") {
		// Extract which issue is missing transaction IDs
		if idx := strings.Index(errorMsg, "issue at index "); idx >= 0 {
			issueNum := extractNumber(errorMsg[idx+15:])
			req.ErrorLocations = append(req.ErrorLocations, ErrorCorrection{
				ErrorPath:        fmt.Sprintf("issues[%d].transaction_ids", issueNum),
				ErrorDescription: "Transaction IDs array is required when affected_count > 0",
				CurrentValue:     nil,
				ExpectedFormat:   "Array of transaction ID strings",
			})
		}
	}

	// Add more error pattern matching as needed
	if strings.Contains(errorMsg, "invalid JSON") {
		// Try to extract line/column from validator
		section, line, col := e.deps.Validator.ExtractError(currentJSON, validationErr)
		if section != "" {
			req.ErrorLocations = append(req.ErrorLocations, ErrorCorrection{
				ErrorPath:        fmt.Sprintf("line %d, column %d", line, col),
				ErrorDescription: "JSON syntax error",
				CurrentValue:     section,
				ExpectedFormat:   "Valid JSON syntax",
			})
		}
	}

	// If we couldn't extract specific locations, provide general guidance
	if len(req.ErrorLocations) == 0 {
		req.ErrorLocations = append(req.ErrorLocations, ErrorCorrection{
			ErrorPath:        "unknown",
			ErrorDescription: errorMsg,
			CurrentValue:     nil,
			ExpectedFormat:   "Valid according to schema",
		})
	}

	return req
}

// prepareTransactionData converts transactions to map format for LLM.
func (e *Engine) prepareTransactionData(transactions []model.Transaction) map[string]any {
	transactionData := make(map[string]any)
	transactionArray := make([]map[string]any, len(transactions))

	for i, txn := range transactions {
		// Extract the first category from the slice for LLM analysis
		var category string
		if len(txn.Category) > 0 {
			category = txn.Category[0]
		}

		transactionArray[i] = map[string]any{
			"ID":       txn.ID,
			"Date":     txn.Date.Format("2006-01-02"),
			"Name":     txn.Name,
			"Amount":   txn.Amount,
			"Type":     txn.Type,
			"Category": category,
		}
	}
	transactionData["transactions"] = transactionArray
	return transactionData
}

// extractNumber extracts the first number from a string.
func extractNumber(s string) int {
	var num int
	for _, r := range s {
		if r >= '0' && r <= '9' {
			num = num*10 + int(r-'0')
		} else if num > 0 {
			break
		}
	}
	return num
}
