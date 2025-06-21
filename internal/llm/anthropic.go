package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// anthropicClient implements the Client interface for Anthropic API.
type anthropicClient struct {
	httpClient  *http.Client
	apiKey      string
	model       string
	temperature float64
	maxTokens   int
}

// newAnthropicClient creates a new Anthropic API client.
func newAnthropicClient(cfg Config) (Client, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("anthropic API key is required")
	}

	model := cfg.Model
	if model == "" {
		model = "claude-3-sonnet-20240229"
	}

	temperature := cfg.Temperature
	if temperature == 0 {
		temperature = 0.3
	}

	maxTokens := cfg.MaxTokens
	if maxTokens == 0 {
		maxTokens = 150
	}

	return &anthropicClient{
		apiKey:      cfg.APIKey,
		model:       model,
		temperature: temperature,
		maxTokens:   maxTokens,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}, nil
}

// Classify sends a classification request to Anthropic.
func (c *anthropicClient) Classify(ctx context.Context, prompt string) (ClassificationResponse, error) {
	systemPrompt := "You are a financial transaction classifier. Respond only with the category and confidence score in the exact format requested."

	requestBody := map[string]any{
		"model":       c.model,
		"max_tokens":  c.maxTokens,
		"temperature": c.temperature,
		"system":      systemPrompt,
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": prompt,
			},
		},
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return ClassificationResponse{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", strings.NewReader(string(jsonBody)))
	if err != nil {
		return ClassificationResponse{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return ClassificationResponse{}, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ClassificationResponse{}, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return ClassificationResponse{}, fmt.Errorf("anthropic API error (status %d): %s", resp.StatusCode, string(body))
	}

	var response anthropicResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return ClassificationResponse{}, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(response.Content) == 0 {
		return ClassificationResponse{}, fmt.Errorf("no content in response")
	}

	return c.parseClassification(response.Content[0].Text)
}

// parseClassification extracts category and confidence from the LLM response.
func (c *anthropicClient) parseClassification(content string) (ClassificationResponse, error) {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	var category string
	var confidence float64
	var isNew bool
	var description string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "CATEGORY:"):
			category = strings.TrimSpace(strings.TrimPrefix(line, "CATEGORY:"))
		case strings.HasPrefix(line, "CONFIDENCE:"):
			confStr := strings.TrimSpace(strings.TrimPrefix(line, "CONFIDENCE:"))
			var err error
			confidence, err = strconv.ParseFloat(confStr, 64)
			if err != nil {
				return ClassificationResponse{}, fmt.Errorf("failed to parse confidence score: %w", err)
			}
		case strings.HasPrefix(line, "NEW:"):
			newStr := strings.TrimSpace(strings.TrimPrefix(line, "NEW:"))
			isNew = strings.ToLower(newStr) == "true"
		case strings.HasPrefix(line, "DESCRIPTION:"):
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

// anthropicResponse represents the Anthropic API response structure.
type anthropicResponse struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	Role         string `json:"role"`
	Model        string `json:"model"`
	StopReason   string `json:"stop_reason"`
	StopSequence string `json:"stop_sequence"`
	Content      []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// GenerateDescription generates a description for a category.
func (c *anthropicClient) GenerateDescription(ctx context.Context, prompt string) (DescriptionResponse, error) {
	requestBody := map[string]any{
		"model":       c.model,
		"max_tokens":  c.maxTokens,
		"temperature": c.temperature,
		"system":      "You are a financial category description generator. Respond only with the description text, no additional formatting.",
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": prompt,
			},
		},
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return DescriptionResponse{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", strings.NewReader(string(jsonBody)))
	if err != nil {
		return DescriptionResponse{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return DescriptionResponse{}, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return DescriptionResponse{}, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return DescriptionResponse{}, fmt.Errorf("anthropic API error (status %d): %s", resp.StatusCode, string(body))
	}

	var response anthropicResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return DescriptionResponse{}, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(response.Content) == 0 {
		return DescriptionResponse{}, fmt.Errorf("no content in response")
	}

	return DescriptionResponse{
		Description: strings.TrimSpace(response.Content[0].Text),
	}, nil
}
