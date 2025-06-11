package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// claudeCodeClient implements the Client interface using Claude Code CLI.
type claudeCodeClient struct {
	model       string
	temperature float64
	maxTokens   int
	maxTurns    int
}

// newClaudeCodeClient creates a new Claude Code CLI client.
func newClaudeCodeClient(cfg Config) (Client, error) {
	// Check if claude CLI is available
	if _, err := exec.LookPath("claude"); err != nil {
		return nil, fmt.Errorf("claude CLI not found: ensure @anthropic-ai/claude-code is installed")
	}

	model := cfg.Model
	if model == "" {
		model = "sonnet" // Use latest Sonnet version
	}

	temperature := cfg.Temperature
	if temperature == 0 {
		temperature = 0.3
	}

	maxTokens := cfg.MaxTokens
	if maxTokens == 0 {
		maxTokens = 150
	}

	return &claudeCodeClient{
		model:       model,
		temperature: temperature,
		maxTokens:   maxTokens,
		maxTurns:    1, // Single turn for categorization
	}, nil
}

// Classify sends a classification request to Claude Code.
func (c *claudeCodeClient) Classify(ctx context.Context, prompt string) (ClassificationResponse, error) {
	// Build the full prompt with system context
	fullPrompt := fmt.Sprintf(
		"You are a financial transaction classifier. Respond only with the category and confidence score in the exact format requested.\n\n%s",
		prompt,
	)

	// Build command arguments
	args := []string{
		"-p", fullPrompt,
		"--output-format", "json",
		"--model", c.model,
		"--max-turns", strconv.Itoa(c.maxTurns),
	}

	// Create command with context
	cmd := exec.CommandContext(ctx, "claude", args...)

	// Capture both stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Set timeout if not already set in context
	cmdCtx := ctx
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		cmdCtx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}
	cmd = exec.CommandContext(cmdCtx, "claude", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute command
	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return ClassificationResponse{}, fmt.Errorf("claude code error: %s", stderr.String())
		}
		return ClassificationResponse{}, fmt.Errorf("failed to execute claude: %w", err)
	}

	// Parse JSON response
	var response claudeCodeResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		// If JSON parsing fails, try to parse as plain text
		return c.parseClassification(stdout.String())
	}

	// Extract classification from response
	if response.Content == "" {
		return ClassificationResponse{}, fmt.Errorf("empty response from claude code")
	}

	return c.parseClassification(response.Content)
}

// parseClassification extracts category and confidence from the response.
func (c *claudeCodeClient) parseClassification(content string) (ClassificationResponse, error) {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	var category string
	var confidence float64

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "CATEGORY:") {
			category = strings.TrimSpace(strings.TrimPrefix(line, "CATEGORY:"))
		} else if strings.HasPrefix(line, "CONFIDENCE:") {
			confStr := strings.TrimSpace(strings.TrimPrefix(line, "CONFIDENCE:"))
			var err error
			confidence, err = strconv.ParseFloat(confStr, 64)
			if err != nil {
				return ClassificationResponse{}, fmt.Errorf("failed to parse confidence score: %w", err)
			}
		}
	}

	if category == "" {
		return ClassificationResponse{}, fmt.Errorf("no category found in response: %s", content)
	}

	if confidence == 0 {
		confidence = 0.7 // Default confidence if not provided
	}

	return ClassificationResponse{
		Category:   category,
		Confidence: confidence,
	}, nil
}

// claudeCodeResponse represents the JSON response from Claude Code CLI.
type claudeCodeResponse struct {
	Content   string `json:"content"`
	Type      string `json:"type"`
	SessionID string `json:"session_id"`
}
