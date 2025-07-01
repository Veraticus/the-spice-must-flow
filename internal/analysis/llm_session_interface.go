package analysis

import (
	"context"

	"github.com/Veraticus/the-spice-must-flow/internal/llm"
)

// SessionAnalysisResult contains the result of an analysis with session info.
type SessionAnalysisResult struct {
	Response  string  // The JSON response from the LLM
	SessionID string  // Session ID for continuing the conversation
	TotalCost float64 // Running total cost for this session
	NumTurns  int     // Number of conversation turns so far
}

// ErrorCorrection represents a request to fix a specific error.
type ErrorCorrection struct {
	ErrorPath        string      `json:"error_path"`        // JSON path where error occurred
	ErrorDescription string      `json:"error_description"` // What went wrong
	CurrentValue     interface{} `json:"current_value"`     // The problematic value
	ExpectedFormat   string      `json:"expected_format"`   // What format/type is expected
}

// CorrectionRequest is sent to the LLM to fix validation errors.
type CorrectionRequest struct {
	ValidationError string            `json:"validation_error"`
	Instructions    string            `json:"instructions"`
	ErrorLocations  []ErrorCorrection `json:"error_locations"`
}

// CorrectionResponse is what the LLM returns with fixes.
type CorrectionResponse struct {
	Reason  string          `json:"reason"`
	Patches []llm.JSONPatch `json:"patches"`
}

// SessionLLMClient extends the basic LLMClient with session support.
type SessionLLMClient interface {
	LLMClient

	// AnalyzeTransactionsWithSession performs analysis with session support.
	// If sessionID is empty, starts a new session. Otherwise continues existing session.
	AnalyzeTransactionsWithSession(ctx context.Context, prompt string, sessionID string) (SessionAnalysisResult, error)

	// AnalyzeTransactionsWithFileSession performs file-based analysis with session support.
	AnalyzeTransactionsWithFileSession(ctx context.Context, prompt string, transactionData map[string]any, sessionID string) (SessionAnalysisResult, error)

	// RequestCorrection asks the LLM to provide patches for specific errors.
	// Must be called with a valid sessionID from a previous analysis.
	RequestCorrection(ctx context.Context, request CorrectionRequest, sessionID string) (CorrectionResponse, error)
}
