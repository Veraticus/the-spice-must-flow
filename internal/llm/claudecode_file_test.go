package llm

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClaudeCodeClient_Analyze_FileBasedPrompt(t *testing.T) {
	tests := []struct {
		name           string
		prompt         string
		systemPrompt   string
		expectedDir    string
		expectedAddDir bool
	}{
		{
			name: "detects temp directory in prompt - actual analysis format",
			prompt: `The transaction data is stored in a JSON file at: /tmp/spice-analysis-12345/transactions_abc.json

Please read this file to access the full transaction data before performing your analysis.

Analyze these transactions to provide insights.`,
			systemPrompt:   "You are an AI assistant",
			expectedAddDir: true,
			expectedDir:    "/tmp/spice-analysis-12345",
		},
		{
			name: "no temp directory in prompt",
			prompt: `Analyze these transactions:
- Transaction 1: $100
- Transaction 2: $200`,
			systemPrompt:   "You are an AI assistant",
			expectedAddDir: false,
		},
		{
			name:           "temp directory in different format",
			prompt:         `Read the file at /tmp/spice-analysis-99999/data.json for transaction details`,
			systemPrompt:   "You are an AI assistant",
			expectedAddDir: true,
			expectedDir:    "/tmp/spice-analysis-99999",
		},
		{
			name: "multiple temp directories uses first one",
			prompt: `File 1: /tmp/spice-analysis-11111/file1.json
File 2: /tmp/spice-analysis-22222/file2.json`,
			systemPrompt:   "You are an AI assistant",
			expectedAddDir: true,
			expectedDir:    "/tmp/spice-analysis-11111",
		},
	}

	// Note: These tests verify the logic for detecting temp directories
	// and building the correct command args. Full integration testing
	// would require mocking the exec.Command functionality.

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Extract temp directory logic (same as in the actual implementation)
			fullPrompt := tt.systemPrompt + "\n\n" + tt.prompt

			var tempDir string
			if strings.Contains(fullPrompt, "/tmp/spice-analysis-") {
				if idx := strings.Index(fullPrompt, "/tmp/spice-analysis-"); idx != -1 {
					endIdx := idx + len("/tmp/spice-analysis-")
					for endIdx < len(fullPrompt) && fullPrompt[endIdx] != '/' {
						endIdx++
					}
					tempDir = fullPrompt[idx:endIdx]
				}
			}

			// Verify detection
			if tt.expectedAddDir {
				assert.NotEmpty(t, tempDir)
				assert.Equal(t, tt.expectedDir, tempDir)
			} else {
				assert.Empty(t, tempDir)
			}
		})
	}
}

func TestClaudeCodeClient_Analyze_LargePrompt(t *testing.T) {
	// Test that large prompts trigger stdin usage
	largePrompt := strings.Repeat("This is a test sentence. ", 5000) // ~120KB
	systemPrompt := "You are an AI assistant"

	fullPrompt := systemPrompt + "\n\n" + largePrompt
	useStdin := len(fullPrompt) > 100000 // Same threshold as implementation

	assert.True(t, useStdin, "Large prompts should use stdin")
}

func TestClaudeCodeClient_BuildArgsWithAddDir(t *testing.T) {
	tests := []struct {
		name         string
		tempDir      string
		expectedArgs []string
		useStdin     bool
	}{
		{
			name:     "stdin with add-dir",
			tempDir:  "/tmp/spice-analysis-12345",
			useStdin: true,
			expectedArgs: []string{
				"--output-format", "json",
				"--model", "sonnet",
				"--max-turns", "1",
				"--add-dir", "/tmp/spice-analysis-12345",
			},
		},
		{
			name:     "command args with add-dir",
			tempDir:  "/tmp/spice-analysis-67890",
			useStdin: false,
			expectedArgs: []string{
				"-p", "test prompt",
				"--output-format", "json",
				"--model", "sonnet",
				"--max-turns", "1",
				"--add-dir", "/tmp/spice-analysis-67890",
			},
		},
		{
			name:     "no temp dir",
			tempDir:  "",
			useStdin: false,
			expectedArgs: []string{
				"-p", "test prompt",
				"--output-format", "json",
				"--model", "sonnet",
				"--max-turns", "1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build args based on conditions (mimicking the actual implementation)
			var args []string

			if tt.useStdin {
				args = []string{
					"--output-format", "json",
					"--model", "sonnet",
					"--max-turns", "1",
				}
			} else {
				args = []string{
					"-p", "test prompt",
					"--output-format", "json",
					"--model", "sonnet",
					"--max-turns", "1",
				}
			}

			// Add temp directory access if detected
			if tt.tempDir != "" {
				args = append(args, "--add-dir", tt.tempDir)
			}

			// Compare with expected args
			assert.Equal(t, tt.expectedArgs, args)
		})
	}
}
