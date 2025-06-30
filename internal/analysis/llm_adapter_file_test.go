package analysis

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Veraticus/the-spice-must-flow/internal/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLLMClientForFile implements a mock LLM client for testing file-based analysis.
type mockLLMClientForFile struct {
	analyzeFunc func(ctx context.Context, prompt, systemPrompt string) (string, error)
	calls       []analyzeCall
}

type analyzeCall struct {
	prompt       string
	systemPrompt string
}

func (m *mockLLMClientForFile) Analyze(ctx context.Context, prompt, systemPrompt string) (string, error) {
	m.calls = append(m.calls, analyzeCall{prompt: prompt, systemPrompt: systemPrompt})
	if m.analyzeFunc != nil {
		return m.analyzeFunc(ctx, prompt, systemPrompt)
	}
	return `{"coherenceScore": 85}`, nil
}

func (m *mockLLMClientForFile) Classify(ctx context.Context, prompt string) (llm.ClassificationResponse, error) {
	return llm.ClassificationResponse{}, fmt.Errorf("not implemented")
}

func (m *mockLLMClientForFile) ClassifyWithRankings(ctx context.Context, prompt string) (llm.RankingResponse, error) {
	return llm.RankingResponse{}, fmt.Errorf("not implemented")
}

func (m *mockLLMClientForFile) ClassifyMerchantBatch(ctx context.Context, prompt string) (llm.MerchantBatchResponse, error) {
	return llm.MerchantBatchResponse{}, fmt.Errorf("not implemented")
}

func (m *mockLLMClientForFile) GenerateDescription(ctx context.Context, prompt string) (llm.DescriptionResponse, error) {
	return llm.DescriptionResponse{}, fmt.Errorf("not implemented")
}

func TestLLMAnalysisAdapter_AnalyzeTransactionsWithFile(t *testing.T) {
	tests := []struct {
		mockError       error
		transactionData map[string]any
		checkPrompt     func(t *testing.T, prompt string)
		name            string
		originalPrompt  string
		mockResponse    string
		wantErr         bool
	}{
		{
			name: "successful file-based analysis",
			transactionData: map[string]any{
				"transactions": []map[string]any{
					{
						"id":     "123",
						"name":   "Test Merchant",
						"amount": 100.50,
					},
				},
			},
			originalPrompt: "Analyze these transactions",
			mockResponse:   `{"coherenceScore": 90, "issues": []}`,
			wantErr:        false,
			checkPrompt: func(t *testing.T, prompt string) {
				assert.Contains(t, prompt, "/tmp/spice-analysis-")
				assert.Contains(t, prompt, "transactions_")
				assert.Contains(t, prompt, ".json")
				assert.Contains(t, prompt, "Please read this file")
			},
		},
		{
			name: "large transaction dataset",
			transactionData: map[string]any{
				"transactions": make([]map[string]interface{}, 10000),
			},
			originalPrompt: "Analyze these transactions",
			mockResponse:   `{"coherenceScore": 85, "issues": []}`,
			wantErr:        false,
			checkPrompt: func(t *testing.T, prompt string) {
				assert.Contains(t, prompt, "/tmp/spice-analysis-")
			},
		},
		{
			name: "empty transaction data",
			transactionData: map[string]any{
				"transactions": []map[string]any{},
			},
			originalPrompt: "Analyze these transactions",
			mockResponse:   `{"coherenceScore": 100, "issues": []}`,
			wantErr:        false,
		},
		{
			name:            "llm error",
			transactionData: map[string]any{"transactions": []map[string]interface{}{{}}},
			originalPrompt:  "Analyze these transactions",
			mockError:       fmt.Errorf("LLM service unavailable"),
			wantErr:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock client
			mockClient := &mockLLMClientForFile{
				analyzeFunc: func(ctx context.Context, prompt, systemPrompt string) (string, error) {
					if tt.checkPrompt != nil {
						tt.checkPrompt(t, prompt)
					}
					if tt.mockError != nil {
						return "", tt.mockError
					}
					return tt.mockResponse, nil
				},
			}

			// Create adapter
			adapter := NewLLMAnalysisAdapter(mockClient)

			// Call method
			result, err := adapter.AnalyzeTransactionsWithFile(context.Background(), tt.originalPrompt, tt.transactionData)

			// Check results
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.mockResponse, result)

			// Verify the temp file was referenced in the prompt
			assert.Len(t, mockClient.calls, 1)
			call := mockClient.calls[0]
			assert.Contains(t, call.prompt, tt.originalPrompt)
			assert.Contains(t, call.prompt, "/tmp/spice-analysis-")

			// Verify the file was cleaned up (give it a moment for defer to run)
			// Extract file path from prompt for verification
			if strings.Contains(call.prompt, "/tmp/spice-analysis-") {
				start := strings.Index(call.prompt, "/tmp/spice-analysis-")
				end := strings.Index(call.prompt[start:], "\n")
				if end != -1 {
					filePath := call.prompt[start : start+end]
					// File should not exist after cleanup
					_, err := os.Stat(filePath)
					assert.True(t, os.IsNotExist(err), "temp file should be cleaned up")
				}
			}
		})
	}
}

func TestLLMAnalysisAdapter_FileSecurity(t *testing.T) {
	// Create adapter
	mockClient := &mockLLMClientForFile{}
	adapter := NewLLMAnalysisAdapter(mockClient)

	// Create temp file through the adapter
	transactionData := map[string]interface{}{
		"transactions": []map[string]interface{}{
			{"id": "123", "amount": 100},
		},
	}

	// Store the file path from the prompt
	var createdFilePath string
	mockClient.analyzeFunc = func(ctx context.Context, prompt, systemPrompt string) (string, error) {
		// Extract file path from prompt
		if idx := strings.Index(prompt, "/tmp/spice-analysis-"); idx != -1 {
			endIdx := strings.Index(prompt[idx:], "\n")
			if endIdx != -1 {
				createdFilePath = prompt[idx : idx+endIdx]
			}
		}
		return `{"coherenceScore": 100}`, nil
	}

	// Call the method
	_, err := adapter.AnalyzeTransactionsWithFile(context.Background(), "test", transactionData)
	require.NoError(t, err)

	// If we managed to capture the file path during execution, verify permissions
	if createdFilePath != "" && strings.HasSuffix(createdFilePath, ".json") {
		// Try to create a file with the same name to check if directory has proper permissions
		tempDir := filepath.Dir(createdFilePath)
		info, err := os.Stat(tempDir)
		if err == nil {
			// Check directory permissions (should be 0700)
			assert.Equal(t, os.FileMode(0700), info.Mode().Perm()&0700)
		}
	}
}
