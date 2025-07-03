package llm

import (
	"context"
	"encoding/json"
)

// AnalysisSession represents an ongoing analysis conversation.
type AnalysisSession struct {
	SessionID      string          `json:"session_id"`
	OriginalResult json.RawMessage `json:"original_result"`
	Patches        []JSONPatch     `json:"patches"`
	CurrentResult  json.RawMessage `json:"current_result"`
}

// JSONPatch represents a single JSON patch operation.
type JSONPatch struct {
	Value any    `json:"value"`
	Path  string `json:"path"`
}

// PatchRequest is what we ask the LLM to provide when fixing errors.
type PatchRequest struct {
	ErrorDescription string `json:"error_description"`
	ErrorPath        string `json:"error_path,omitempty"`
	OriginalValue    any    `json:"original_value,omitempty"`
	SuggestedFix     string `json:"suggested_fix"`
}

// PatchResponse is what the LLM returns with corrections.
type PatchResponse struct {
	Reason  string      `json:"reason,omitempty"`
	Patches []JSONPatch `json:"patches"`
}

// SessionClient extends the Client interface with session support.
type SessionClient interface {
	Client
	// AnalyzeWithSession starts a new analysis session or continues an existing one.
	AnalyzeWithSession(ctx context.Context, prompt string, systemPrompt string, sessionID string) (AnalysisResult, error)
}

// AnalysisResult contains both the response and session information.
type AnalysisResult struct {
	Response  string
	SessionID string
	TotalCost float64
	NumTurns  int
}

// JSON patching functionality has been moved to json_patch.go
// Use JSONPatcher.ApplyPatch() and JSONPatcher.ApplyPatches() instead
