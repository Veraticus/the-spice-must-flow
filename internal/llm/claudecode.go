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
	cliPath     string
}

// newClaudeCodeClient creates a new Claude Code CLI client.
func newClaudeCodeClient(cfg Config) (Client, error) {
	// Use configured path or default
	cliPath := cfg.ClaudeCodePath
	if cliPath == "" {
		cliPath = "claude"
	}

	// Check if claude CLI is available
	if _, err := exec.LookPath(cliPath); err != nil {
		return nil, fmt.Errorf("claude CLI not found at %s: ensure @anthropic-ai/claude-code is installed", cliPath)
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
		cliPath:     cliPath,
	}, nil
}

// Classify sends a classification request to Claude Code.
func (c *claudeCodeClient) Classify(ctx context.Context, prompt string) (ClassificationResponse, error) {
	// Build the full prompt with system context
	fullPrompt := fmt.Sprintf(
		"You are a neutral financial transaction classifier. Your role is to categorize transactions based on WHAT they are (merchant type, service provided) not WHO might be using them or WHY. Avoid any assumptions about personal vs business use. Respond only with the category and confidence score in the exact format requested.\n\n%s",
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
	cmd := exec.CommandContext(ctx, c.cliPath, args...)

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
	cmd = exec.CommandContext(cmdCtx, c.cliPath, args...)
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

	// Check for errors in response
	if response.IsError {
		return ClassificationResponse{}, fmt.Errorf("claude code error in response")
	}

	// Extract classification from response
	if response.Result == "" {
		return ClassificationResponse{}, fmt.Errorf("empty response from claude code")
	}

	return c.parseClassification(response.Result)
}

// parseClassification extracts category and confidence from the response.
func (c *claudeCodeClient) parseClassification(content string) (ClassificationResponse, error) {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	var category string
	var confidence float64
	var isNew bool
	var description string

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
		} else if strings.HasPrefix(line, "NEW:") {
			newStr := strings.TrimSpace(strings.TrimPrefix(line, "NEW:"))
			isNew = strings.ToLower(newStr) == "true"
		} else if strings.HasPrefix(line, "DESCRIPTION:") {
			description = strings.TrimSpace(strings.TrimPrefix(line, "DESCRIPTION:"))
		}
	}

	if category == "" {
		return ClassificationResponse{}, fmt.Errorf("no category found in response: %s", content)
	}

	if confidence == 0 {
		confidence = 0.7 // Default confidence if not provided
	}

	// If confidence is below 0.85 and NEW wasn't explicitly set, assume it's a new category
	if confidence < 0.85 && !isNew {
		isNew = true
	}

	return ClassificationResponse{
		Category:            category,
		Confidence:          confidence,
		IsNew:               isNew,
		CategoryDescription: description,
	}, nil
}

// claudeCodeResponse represents the JSON response from Claude Code CLI.
type claudeCodeResponse struct {
	Result    string  `json:"result"`
	Type      string  `json:"type"`
	SessionID string  `json:"session_id"`
	IsError   bool    `json:"is_error"`
	TotalCost float64 `json:"total_cost_usd"`
}

// GenerateDescription generates a description for a category.
func (c *claudeCodeClient) GenerateDescription(ctx context.Context, prompt string) (DescriptionResponse, error) {
	// Build the full prompt with system context
	fullPrompt := fmt.Sprintf(
		"You are a financial category description generator. Respond only with the description text, no additional formatting.\n\n%s",
		prompt,
	)

	// Build command arguments (similar to Classify method)
	args := []string{
		"-p", fullPrompt,
		"--output-format", "json",
		"--model", c.model,
		"--max-turns", "1",
	}

	// Create command with context
	cmd := exec.CommandContext(ctx, c.cliPath, args...)

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
	cmd = exec.CommandContext(cmdCtx, c.cliPath, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute command
	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return DescriptionResponse{}, fmt.Errorf("claude code error: %s", stderr.String())
		}
		return DescriptionResponse{}, fmt.Errorf("failed to execute claude: %w", err)
	}

	// Parse JSON response
	var response claudeCodeResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		// If JSON parsing fails, use the raw output
		return DescriptionResponse{
			Description: strings.TrimSpace(stdout.String()),
		}, nil
	}

	// Check for errors in response
	if response.IsError {
		return DescriptionResponse{}, fmt.Errorf("claude code error in response")
	}

	return DescriptionResponse{
		Description: strings.TrimSpace(response.Result),
	}, nil
}

// generateDescriptionJSON handles description generation via JSON communication.
func (c *claudeCodeClient) generateDescriptionJSON(ctx context.Context, prompt string) (DescriptionResponse, error) {
	// This would be used when the Claude CLI isn't available
	// For now, we'll return a simple response
	var response claudeCodeResponse
	response.Result = strings.TrimSpace(prompt)

	return DescriptionResponse{
		Description: response.Result,
	}, nil
}
