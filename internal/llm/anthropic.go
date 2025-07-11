package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	systemPrompt := "You are a financial transaction classifier. You MUST respond with ONLY a valid JSON object. Do not include any explanatory text, markdown formatting, or commentary before or after the JSON. Start your response directly with { and end with }."

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
	// Parse JSON response
	var jsonResp struct {
		Category    string  `json:"category"`
		Confidence  float64 `json:"confidence"`
		IsNew       bool    `json:"isNew"`
		Description string  `json:"description,omitempty"`
	}

	content = cleanMarkdownWrapper(content)

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

// ClassifyWithRankings sends a ranking classification request to Anthropic.
func (c *anthropicClient) ClassifyWithRankings(ctx context.Context, prompt string) (RankingResponse, error) {
	systemPrompt := "You are a financial transaction classifier. You MUST respond with ONLY a valid JSON object containing rankings. Do not include any explanatory text, markdown formatting, or commentary before or after the JSON. Start your response directly with { and end with }."

	requestBody := map[string]any{
		"model":       c.model,
		"max_tokens":  c.maxTokens * 3, // More tokens needed for ranking all categories
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
		return RankingResponse{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", strings.NewReader(string(jsonBody)))
	if err != nil {
		return RankingResponse{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return RankingResponse{}, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return RankingResponse{}, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return RankingResponse{}, fmt.Errorf("anthropic API error (status %d): %s", resp.StatusCode, string(body))
	}

	var response anthropicResponse
	if unmarshalErr := json.Unmarshal(body, &response); unmarshalErr != nil {
		return RankingResponse{}, fmt.Errorf("failed to parse response: %w", unmarshalErr)
	}

	if len(response.Content) == 0 {
		return RankingResponse{}, fmt.Errorf("no content in response")
	}

	// Try to parse as JSON first
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

	content := response.Content[0].Text

	// Clean any markdown wrappers that might be present
	content = cleanMarkdownWrapper(content)

	if unmarshalErr := json.Unmarshal([]byte(content), &jsonResp); unmarshalErr == nil {
		// Successfully parsed JSON
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

		return RankingResponse{Rankings: rankings}, nil
	}

	return RankingResponse{}, fmt.Errorf("failed to parse JSON response: %w", err)
}

// GenerateDescription generates a description for a category.
func (c *anthropicClient) GenerateDescription(ctx context.Context, prompt string) (DescriptionResponse, error) {
	requestBody := map[string]any{
		"model":       c.model,
		"max_tokens":  c.maxTokens,
		"temperature": c.temperature,
		"system":      "You are a financial category description generator. You MUST respond with ONLY a valid JSON object. Do not include any explanatory text, markdown formatting, or commentary before or after the JSON. Start your response directly with { and end with }.",
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

	// Parse the JSON response
	var descResp struct {
		Description string  `json:"description"`
		Confidence  float64 `json:"confidence"`
	}

	content := response.Content[0].Text
	content = cleanMarkdownWrapper(content)

	if err := json.Unmarshal([]byte(content), &descResp); err != nil {
		return DescriptionResponse{}, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return DescriptionResponse{
		Description: descResp.Description,
		Confidence:  descResp.Confidence,
	}, nil
}

// ClassifyMerchantBatch classifies multiple merchants in a single API call.
func (c *anthropicClient) ClassifyMerchantBatch(ctx context.Context, prompt string) (MerchantBatchResponse, error) {
	systemPrompt := "You are a financial transaction classifier. You MUST respond with ONLY a valid JSON object containing merchant classifications. Do not include any explanatory text, markdown formatting, or commentary before or after the JSON. Start your response directly with { and end with }."

	requestBody := map[string]any{
		"model":       c.model,
		"max_tokens":  c.maxTokens * 10, // More tokens needed for batch response
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
		return MerchantBatchResponse{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", strings.NewReader(string(jsonBody)))
	if err != nil {
		return MerchantBatchResponse{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return MerchantBatchResponse{}, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return MerchantBatchResponse{}, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return MerchantBatchResponse{}, fmt.Errorf("anthropic API error (status %d): %s", resp.StatusCode, string(body))
	}

	var response anthropicResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return MerchantBatchResponse{}, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(response.Content) == 0 {
		return MerchantBatchResponse{}, fmt.Errorf("no content in response")
	}

	return c.parseMerchantBatchResponse(response.Content[0].Text)
}

// parseMerchantBatchResponse parses the batch classification response.
func (c *anthropicClient) parseMerchantBatchResponse(content string) (MerchantBatchResponse, error) {
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

	content = cleanMarkdownWrapper(content)

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
func (c *anthropicClient) Analyze(ctx context.Context, prompt string, systemPrompt string) (string, error) {
	// Use higher token limit for analysis
	maxTokens := 4000
	if c.maxTokens > 4000 {
		maxTokens = c.maxTokens
	}

	requestBody := map[string]any{
		"model":       c.model,
		"max_tokens":  maxTokens,
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
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", strings.NewReader(string(jsonBody)))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("anthropic API error (status %d): %s", resp.StatusCode, string(body))
	}

	var response anthropicResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(response.Content) == 0 {
		return "", fmt.Errorf("no content in response")
	}

	// Return raw content without any parsing
	return response.Content[0].Text, nil
}
