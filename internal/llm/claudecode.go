package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
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
		maxTurns:    10, // Allow up to 10 turns for complex analysis
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
	rankings := make([]CategoryRanking, 0, len(jsonResp.Rankings))
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

// ClassifyMerchantBatch classifies multiple merchants in a single API call.
func (c *claudeCodeClient) ClassifyMerchantBatch(ctx context.Context, prompt string) (MerchantBatchResponse, error) {
	// Build the full prompt with system context
	fullPrompt := fmt.Sprintf(
		"You are a financial transaction classifier. You MUST respond with ONLY a valid JSON object containing merchant classifications. Do not include any explanatory text, markdown formatting, or commentary before or after the JSON. Start your response directly with { and end with }.\n\n%s",
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
			return MerchantBatchResponse{}, fmt.Errorf("claude code error: %s", stderr.String())
		}
		return MerchantBatchResponse{}, fmt.Errorf("failed to execute claude: %w", err)
	}

	// Parse JSON response
	var response claudeCodeResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		return MerchantBatchResponse{}, fmt.Errorf("failed to parse claude code response: %w", err)
	}

	// Check for errors in response
	if response.IsError {
		return MerchantBatchResponse{}, fmt.Errorf("claude code error in response")
	}

	// Extract classification from response
	if response.Result == "" {
		return MerchantBatchResponse{}, fmt.Errorf("empty response from claude code")
	}

	return c.parseMerchantBatchResponse(cleanMarkdownWrapper(response.Result))
}

// parseMerchantBatchResponse parses the batch classification response.
func (c *claudeCodeClient) parseMerchantBatchResponse(content string) (MerchantBatchResponse, error) {
	// Expected JSON structure:
	// {
	//   "classifications": [
	//     {
	//       "merchantId": "merchant-1",
	//       "rankings": [
	//         {"category": "Groceries", "score": 0.95, "isNew": false},
	//         {"category": "Food & Dining", "score": 0.05, "isNew": false}
	//       ]
	//     },
	//     ...
	//   ]
	// }
	var jsonResp struct {
		Classifications []struct {
			MerchantID string `json:"merchantId"`
			Rankings   []struct {
				Category    string  `json:"category"`
				Score       float64 `json:"score"`
				IsNew       bool    `json:"isNew"`
				Description string  `json:"description,omitempty"`
			} `json:"rankings"`
		} `json:"classifications"`
	}

	if err := json.Unmarshal([]byte(content), &jsonResp); err != nil {
		return MerchantBatchResponse{}, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// Convert to our response format
	classifications := make([]MerchantClassification, 0, len(jsonResp.Classifications))
	for _, c := range jsonResp.Classifications {
		rankings := make([]CategoryRanking, 0, len(c.Rankings))
		for _, r := range c.Rankings {
			rankings = append(rankings, CategoryRanking{
				Category:    r.Category,
				Score:       r.Score,
				IsNew:       r.IsNew,
				Description: r.Description,
			})
		}
		classifications = append(classifications, MerchantClassification{
			MerchantID: c.MerchantID,
			Rankings:   rankings,
		})
	}

	return MerchantBatchResponse{
		Classifications: classifications,
	}, nil
}

// Analyze performs general-purpose AI analysis and returns raw response text.
func (c *claudeCodeClient) Analyze(ctx context.Context, prompt string, systemPrompt string) (string, error) {
	// Build the full prompt with system context
	fullPrompt := systemPrompt + "\n\n" + prompt

	// Log prompt details in debug mode
	slog.Debug("Claude Code analysis request",
		"model", c.model,
		"prompt_length", len(fullPrompt),
		"prompt_preview", truncateStr(fullPrompt, 500),
	)

	// For very large prompts, use stdin instead of command args
	// Command line args are limited to ~2MB on most systems
	useStdin := len(fullPrompt) > 100000 // Use stdin for prompts over 100KB

	var args []string
	var cmd *exec.Cmd

	// Check if prompt contains a file path reference to temp directory
	var tempDir string
	// Look for the specific pattern used by the analysis command:
	// "The transaction data is stored in a JSON file at: /tmp/spice-analysis-XXX/transactions_UUID.json"
	fileAtPattern := "file at: /tmp/spice-analysis-"
	if idx := strings.Index(fullPrompt, fileAtPattern); idx != -1 {
		// Start from after "file at: "
		pathStart := idx + len("file at: ")

		// Find the end of the directory path (up to the next slash after the directory name)
		dirStart := pathStart
		dirEnd := pathStart + len("/tmp/spice-analysis-")

		// Continue until we find a '/' which indicates the start of the filename
		for dirEnd < len(fullPrompt) && fullPrompt[dirEnd] != '/' {
			dirEnd++
		}

		// Extract the directory path
		if dirEnd < len(fullPrompt) && fullPrompt[dirEnd] == '/' {
			tempDir = fullPrompt[dirStart:dirEnd]

			// Extract more context for debugging
			contextEnd := dirEnd + 50
			if contextEnd > len(fullPrompt) {
				contextEnd = len(fullPrompt)
			}

			slog.Debug("Detected temp directory in prompt",
				"temp_dir", tempDir,
				"full_path_context", fullPrompt[pathStart:contextEnd])
		} else {
			slog.Debug("Found file path pattern but couldn't extract directory",
				"pattern_index", idx)
		}
	}

	if useStdin {
		// Use stdin for large prompts
		args = []string{
			"--print", // Required for --output-format json to return result field
			"--output-format", "json",
			"--model", c.model,
			"--max-turns", strconv.Itoa(c.maxTurns),
		}
		// Add temp directory access if detected
		if tempDir != "" {
			args = append(args, "--add-dir", tempDir)
			slog.Debug("Added temp directory access", "dir", tempDir)
		}
		cmd = exec.CommandContext(ctx, c.cliPath, args...)
		cmd.Stdin = strings.NewReader(fullPrompt)
		slog.Debug("Using stdin for large prompt", "size", len(fullPrompt))
	} else {
		// Use command args for smaller prompts
		args = []string{
			"-p", fullPrompt,
			"--output-format", "json",
			"--model", c.model,
			"--max-turns", strconv.Itoa(c.maxTurns),
		}
		// Add temp directory access if detected
		if tempDir != "" {
			args = append(args, "--add-dir", tempDir)
			slog.Debug("Added temp directory access", "dir", tempDir)
		}
		cmd = exec.CommandContext(ctx, c.cliPath, args...)
	}

	// Capture both stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Set timeout if not already set in context
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		// Use 5 minute timeout for analysis with ultrathink
		ctx, cancel = context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
	}

	// Recreate command with potentially updated context
	if useStdin {
		cmd = exec.CommandContext(ctx, c.cliPath, args...)
		cmd.Stdin = strings.NewReader(fullPrompt)
	} else {
		cmd = exec.CommandContext(ctx, c.cliPath, args...)
	}
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Log command execution start
	slog.Debug("Executing Claude Code command",
		"timeout", "5m",
		"args", args,
	)

	// Execute command
	startTime := time.Now()
	if err := cmd.Run(); err != nil {
		duration := time.Since(startTime)
		slog.Debug("Claude Code command failed",
			"duration", duration,
			"error", err,
			"stderr", stderr.String(),
			"stdout_size", stdout.Len(),
		)
		if stderr.Len() > 0 {
			return "", fmt.Errorf("claude code error after %v: %s", duration, stderr.String())
		}
		return "", fmt.Errorf("failed to execute claude after %v: %w", duration, err)
	}

	duration := time.Since(startTime)
	slog.Debug("Claude Code command completed",
		"duration", duration,
		"stdout_size", stdout.Len(),
		"stderr_size", stderr.Len(),
		"stdout", stdout.String(),
		"stderr", stderr.String(),
	)

	// Parse JSON response from Claude Code
	var response claudeCodeResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		slog.Debug("Failed to parse Claude Code JSON response",
			"error", err,
			"raw_output", truncateStr(stdout.String(), 1000),
		)
		return "", fmt.Errorf("failed to parse claude code response: %w", err)
	}

	// Check for errors in response
	if response.IsError {
		slog.Debug("Claude Code returned error response",
			"is_error", response.IsError,
			"result", truncateStr(response.Result, 500),
		)
		return "", fmt.Errorf("claude code error in response")
	}

	// Log response in debug mode
	slog.Debug("Claude Code analysis response",
		"response_length", len(response.Result),
		"response_preview", truncateStr(response.Result, 500),
		"is_error", response.IsError,
	)

	// Return the raw result without any parsing
	return response.Result, nil
}

// truncateStr truncates a string for logging purposes.
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
