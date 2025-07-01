package llm

import (
	"context"
	"encoding/json"
	"fmt"
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
	Value interface{} `json:"value"`
	Path  string      `json:"path"`
}

// PatchRequest is what we ask the LLM to provide when fixing errors.
type PatchRequest struct {
	ErrorDescription string      `json:"error_description"`
	ErrorPath        string      `json:"error_path,omitempty"`
	OriginalValue    interface{} `json:"original_value,omitempty"`
	SuggestedFix     string      `json:"suggested_fix"`
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

// ApplyPatch applies a JSON patch to the given JSON data.
func ApplyPatch(original json.RawMessage, patch JSONPatch) (json.RawMessage, error) {
	// Parse the original JSON into a generic structure
	var data interface{}
	if err := json.Unmarshal(original, &data); err != nil {
		return nil, fmt.Errorf("failed to parse original JSON: %w", err)
	}

	// Apply the patch
	if err := applyPatchToData(&data, patch.Path, patch.Value); err != nil {
		return nil, fmt.Errorf("failed to apply patch: %w", err)
	}

	// Marshal back to JSON
	result, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal patched data: %w", err)
	}

	return result, nil
}

// applyPatchToData applies a patch to nested data using a path.
func applyPatchToData(data *interface{}, path string, value interface{}) error {
	// This is a simplified implementation - in production, you'd want to use
	// a proper JSON path library or implement full path parsing

	// For now, let's handle basic cases like:
	// - "field"
	// - "parent.child"
	// - "array[0]"
	// - "parent.array[0].field"

	// TODO: Implement proper JSON path parsing and application
	// For the MVP, we'll require the LLM to provide simple paths

	return fmt.Errorf("path parsing not yet implemented: %s", path)
}

// ApplyPatches applies multiple patches to JSON data.
func ApplyPatches(original json.RawMessage, patches []JSONPatch) (json.RawMessage, error) {
	result := original
	for i, patch := range patches {
		var err error
		result, err = ApplyPatch(result, patch)
		if err != nil {
			return nil, fmt.Errorf("failed to apply patch %d: %w", i, err)
		}
	}
	return result, nil
}
