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

// openAIClient implements the Client interface for OpenAI API.
type openAIClient struct {
	httpClient  *http.Client
	apiKey      string
	model       string
	temperature float64
	maxTokens   int
}

// newOpenAIClient creates a new OpenAI API client.
func newOpenAIClient(cfg Config) (Client, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required")
	}

	model := cfg.Model
	if model == "" {
		model = "gpt-4-turbo-preview"
	}

	temperature := cfg.Temperature
	if temperature == 0 {
		temperature = 0.3
	}

	maxTokens := cfg.MaxTokens
	if maxTokens == 0 {
		maxTokens = 150
	}

	return &openAIClient{
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

// Classify sends a classification request to OpenAI.
func (c *openAIClient) Classify(ctx context.Context, prompt string) (ClassificationResponse, error) {
	requestBody := map[string]any{
		"model": c.model,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "You are a financial transaction classifier. Respond only with the category and confidence score in the exact format requested.",
			},
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"temperature": c.temperature,
		"max_tokens":  c.maxTokens,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return ClassificationResponse{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/chat/completions", strings.NewReader(string(jsonBody)))
	if err != nil {
		return ClassificationResponse{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

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
		return ClassificationResponse{}, fmt.Errorf("OpenAI API error (status %d): %s", resp.StatusCode, string(body))
	}

	var response openAIResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return ClassificationResponse{}, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(response.Choices) == 0 {
		return ClassificationResponse{}, fmt.Errorf("no completion choices returned")
	}

	return c.parseClassification(response.Choices[0].Message.Content)
}

// parseClassification extracts category and confidence from the LLM response.
func (c *openAIClient) parseClassification(content string) (ClassificationResponse, error) {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	var category string
	var confidence float64
	var isNew bool

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
		}
	}

	if category == "" {
		return ClassificationResponse{}, fmt.Errorf("no category found in response: %s", content)
	}

	if confidence == 0 {
		confidence = 0.7 // Default confidence if not provided
	}

	// If confidence is below 0.9 and NEW wasn't explicitly set, assume it's a new category
	if confidence < 0.9 && !isNew {
		isNew = true
	}

	return ClassificationResponse{
		Category:   category,
		Confidence: confidence,
		IsNew:      isNew,
	}, nil
}

// openAIResponse represents the OpenAI API response structure.
type openAIResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
		Index        int    `json:"index"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Created int64 `json:"created"`
}
