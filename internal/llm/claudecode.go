package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"time"
)

// claudeCodeClient implements the Client interface using Claude Code CLI.
type claudeCodeClient struct {
	model       string
	cliPath     string
	temperature float64
	maxTokens   int
	maxTurns    int
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
		"You are a neutral financial transaction classifier. Your role is to categorize transactions based on WHAT they are (merchant type, service provided) not WHO might be using them or WHY. Avoid any assumptions about personal vs business use. You MUST respond with ONLY a valid JSON object. Do not include any explanatory text, markdown formatting, or commentary before or after the JSON. Start your response directly with { and end with }.\n\n%s",
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
		return ClassificationResponse{}, fmt.Errorf("failed to parse claude code response: %w", err)
	}

	// Check for errors in response
	if response.IsError {
		return ClassificationResponse{}, fmt.Errorf("claude code error in response")
	}

	// Extract classification from response
	if response.Result == "" {
		return ClassificationResponse{}, fmt.Errorf("empty response from claude code")
	}

	return c.parseClassification(cleanMarkdownWrapper(response.Result))
}

// parseClassification extracts category and confidence from the response.
func (c *claudeCodeClient) parseClassification(content string) (ClassificationResponse, error) {
	// Parse JSON response
	var jsonResp struct {
		Category    string  `json:"category"`
		Confidence  float64 `json:"confidence"`
		IsNew       bool    `json:"isNew"`
		Description string  `json:"description"`
	}

	if err := json.Unmarshal([]byte(content), &jsonResp); err != nil {
		return ClassificationResponse{}, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	if jsonResp.Category == "" {
		return ClassificationResponse{}, fmt.Errorf("no category found in response")
	}

	return ClassificationResponse{
		Category:            jsonResp.Category,
		Confidence:          jsonResp.Confidence,
		IsNew:               jsonResp.IsNew,
		CategoryDescription: jsonResp.Description,
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

// ClassifyWithRankings sends a ranking classification request to Claude Code.
func (c *claudeCodeClient) ClassifyWithRankings(ctx context.Context, prompt string) (RankingResponse, error) {
	// Build the full prompt with system context
	fullPrompt := fmt.Sprintf(
		"You are a financial transaction classifier. You MUST respond with ONLY a valid JSON object containing rankings. Do not include any explanatory text, markdown formatting, or commentary before or after the JSON. Start your response directly with { and end with }.\n\n%s",
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
			return RankingResponse{}, fmt.Errorf("claude code error: %s", stderr.String())
		}
		return RankingResponse{}, fmt.Errorf("failed to execute claude: %w", err)
	}

	// Parse JSON response
	var response claudeCodeResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		return RankingResponse{}, fmt.Errorf("failed to parse claude code response: %w", err)
	}

	// Check for errors in response
	if response.IsError {
		return RankingResponse{}, fmt.Errorf("claude code error in response")
	}

	// Parse the rankings JSON
	var jsonResp struct {
		Rankings []struct {
			Category string  `json:"category"`
			Score    float64 `json:"score"`
		} `json:"rankings"`
		NewCategory *struct {
			Name        string  `json:"name"`
			Score       float64 `json:"score"`
			Description string  `json:"description"`
		} `json:"newCategory,omitempty"`
	}

	cleanedResult := cleanMarkdownWrapper(response.Result)

	if err := json.Unmarshal([]byte(cleanedResult), &jsonResp); err != nil {
		return RankingResponse{}, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// Build rankings
	var rankings []CategoryRanking
	for _, r := range jsonResp.Rankings {
		rankings = append(rankings, CategoryRanking{
			Category: r.Category,
			Score:    r.Score,
			IsNew:    false,
		})
	}

	// Add new category if present
	if jsonResp.NewCategory != nil {
		rankings = append(rankings, CategoryRanking{
			Category:    jsonResp.NewCategory.Name,
			Score:       jsonResp.NewCategory.Score,
			IsNew:       true,
			Description: jsonResp.NewCategory.Description,
		})
	}

	if len(rankings) == 0 {
		return RankingResponse{}, fmt.Errorf("no rankings found in response")
	}

	return RankingResponse{Rankings: rankings}, nil
}

// GenerateDescription generates a description for a category.
func (c *claudeCodeClient) GenerateDescription(ctx context.Context, prompt string) (DescriptionResponse, error) {
	// Build the full prompt with system context
	fullPrompt := fmt.Sprintf(
		"You are a financial category description generator. You MUST respond with ONLY a valid JSON object. Do not include any explanatory text, markdown formatting, or commentary before or after the JSON. Start your response directly with { and end with }.\n\n%s",
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

	// Parse JSON response from Claude Code
	var response claudeCodeResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		return DescriptionResponse{}, fmt.Errorf("failed to parse claude code response: %w", err)
	}

	// Check for errors in response
	if response.IsError {
		return DescriptionResponse{}, fmt.Errorf("claude code error in response")
	}

	// Parse the actual description response from the result
	cleanedResult := cleanMarkdownWrapper(response.Result)

	var descResp struct {
		Description string  `json:"description"`
		Confidence  float64 `json:"confidence"`
	}
	if err := json.Unmarshal([]byte(cleanedResult), &descResp); err != nil {
		return DescriptionResponse{}, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return DescriptionResponse{
		Description: descResp.Description,
		Confidence:  descResp.Confidence,
	}, nil
}
