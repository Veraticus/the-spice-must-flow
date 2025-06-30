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
				"content": "You are a financial transaction classifier. You MUST respond with ONLY a valid JSON object. Do not include any explanatory text, markdown formatting, or commentary before or after the JSON. Start your response directly with { and end with }.",
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

// ClassifyWithRankings sends a ranking classification request to OpenAI.
func (c *openAIClient) ClassifyWithRankings(ctx context.Context, prompt string) (RankingResponse, error) {
	requestBody := map[string]any{
		"model": c.model,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "You are a financial transaction classifier. You MUST respond with ONLY a valid JSON object containing rankings. Do not include any explanatory text, markdown formatting, or commentary before or after the JSON. Start your response directly with { and end with }.",
			},
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"temperature": c.temperature,
		"max_tokens":  c.maxTokens * 3, // More tokens needed for ranking all categories
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return RankingResponse{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/chat/completions", strings.NewReader(string(jsonBody)))
	if err != nil {
		return RankingResponse{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

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
		return RankingResponse{}, fmt.Errorf("OpenAI API error (status %d): %s", resp.StatusCode, string(body))
	}

	var response openAIResponse
	if unmarshalErr := json.Unmarshal(body, &response); unmarshalErr != nil {
		return RankingResponse{}, fmt.Errorf("failed to parse response: %w", unmarshalErr)
	}

	if len(response.Choices) == 0 {
		return RankingResponse{}, fmt.Errorf("no completion choices returned")
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

	content := response.Choices[0].Message.Content

	// Clean any markdown wrappers that might be present
	content = cleanMarkdownWrapper(content)

	if err := json.Unmarshal([]byte(content), &jsonResp); err != nil {
		return RankingResponse{}, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// Successfully parsed JSON
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
func (c *openAIClient) GenerateDescription(ctx context.Context, prompt string) (DescriptionResponse, error) {
	requestBody := map[string]any{
		"model": c.model,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "You are a financial category description generator. You MUST respond with ONLY a valid JSON object. Do not include any explanatory text, markdown formatting, or commentary before or after the JSON. Start your response directly with { and end with }.",
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
		return DescriptionResponse{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/chat/completions", strings.NewReader(string(jsonBody)))
	if err != nil {
		return DescriptionResponse{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

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
		return DescriptionResponse{}, fmt.Errorf("OpenAI API error (status %d): %s", resp.StatusCode, string(body))
	}

	var response openAIResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return DescriptionResponse{}, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(response.Choices) == 0 {
		return DescriptionResponse{}, fmt.Errorf("no completion choices returned")
	}

	// Parse JSON response
	var jsonResp struct {
		Description string  `json:"description"`
		Confidence  float64 `json:"confidence"`
	}

	content := response.Choices[0].Message.Content
	content = cleanMarkdownWrapper(content)

	if err := json.Unmarshal([]byte(content), &jsonResp); err != nil {
		return DescriptionResponse{}, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return DescriptionResponse{
		Description: jsonResp.Description,
		Confidence:  jsonResp.Confidence,
	}, nil
}

// ClassifyMerchantBatch classifies multiple merchants in a single API call.
func (c *openAIClient) ClassifyMerchantBatch(ctx context.Context, prompt string) (MerchantBatchResponse, error) {
	requestBody := map[string]any{
		"model": c.model,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "You are a financial transaction classifier. You MUST respond with ONLY a valid JSON object containing merchant classifications. Do not include any explanatory text, markdown formatting, or commentary before or after the JSON. Start your response directly with { and end with }.",
			},
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"temperature": c.temperature,
		"max_tokens":  c.maxTokens * 10, // More tokens needed for batch response
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return MerchantBatchResponse{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/chat/completions", strings.NewReader(string(jsonBody)))
	if err != nil {
		return MerchantBatchResponse{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

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
		return MerchantBatchResponse{}, fmt.Errorf("OpenAI API error (status %d): %s", resp.StatusCode, string(body))
	}

	var response openAIResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return MerchantBatchResponse{}, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(response.Choices) == 0 {
		return MerchantBatchResponse{}, fmt.Errorf("no completion choices returned")
	}

	return c.parseMerchantBatchResponse(response.Choices[0].Message.Content)
}

// parseMerchantBatchResponse parses the batch classification response.
func (c *openAIClient) parseMerchantBatchResponse(content string) (MerchantBatchResponse, error) {
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
func (c *openAIClient) Analyze(ctx context.Context, prompt string, systemPrompt string) (string, error) {
	// Use higher token limit for analysis
	maxTokens := 4000
	if c.maxTokens > 4000 {
		maxTokens = c.maxTokens
	}

	requestBody := map[string]any{
		"model": c.model,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": systemPrompt,
			},
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"temperature": c.temperature,
		"max_tokens":  maxTokens,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/chat/completions", strings.NewReader(string(jsonBody)))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

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
		return "", fmt.Errorf("OpenAI API error (status %d): %s", resp.StatusCode, string(body))
	}

	var response openAIResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no completion choices returned")
	}

	// Return raw content without any parsing
	return response.Choices[0].Message.Content, nil
}
