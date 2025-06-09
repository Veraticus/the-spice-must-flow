package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAnthropicClient(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				APIKey: "test-key",
			},
			wantErr: false,
		},
		{
			name: "missing API key",
			config: Config{
				APIKey: "",
			},
			wantErr: true,
		},
		{
			name: "custom model and settings",
			config: Config{
				APIKey:      "test-key",
				Model:       "claude-3-opus-20240229",
				Temperature: 0.5,
				MaxTokens:   200,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := newAnthropicClient(tt.config)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, client)
			}
		})
	}
}

// testAnthropicClient extends anthropicClient to allow URL override for testing.
type testAnthropicClient struct {
	*anthropicClient
	baseURL string
}

func (c *testAnthropicClient) Classify(ctx context.Context, prompt string) (ClassificationResponse, error) {
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
		return ClassificationResponse{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/messages", strings.NewReader(string(jsonBody)))
	if err != nil {
		return ClassificationResponse{}, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return ClassificationResponse{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ClassificationResponse{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return ClassificationResponse{}, fmt.Errorf("Anthropic API error (status %d): %s", resp.StatusCode, string(body))
	}

	var response anthropicResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return ClassificationResponse{}, err
	}

	if len(response.Content) == 0 {
		return ClassificationResponse{}, fmt.Errorf("no content in response")
	}

	return c.parseClassification(response.Content[0].Text)
}

func TestAnthropicClient_Classify(t *testing.T) {
	tests := []struct {
		name           string
		wantCategory   string
		mockResponse   anthropicResponse
		statusCode     int
		wantConfidence float64
		wantErr        bool
	}{
		{
			name: "successful classification",
			mockResponse: anthropicResponse{
				Content: []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				}{
					{
						Type: "text",
						Text: "CATEGORY: Coffee & Dining\nCONFIDENCE: 0.95",
					},
				},
			},
			statusCode:     http.StatusOK,
			wantCategory:   "Coffee & Dining",
			wantConfidence: 0.95,
			wantErr:        false,
		},
		{
			name: "missing confidence uses default",
			mockResponse: anthropicResponse{
				Content: []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				}{
					{
						Type: "text",
						Text: "CATEGORY: Shopping",
					},
				},
			},
			statusCode:     http.StatusOK,
			wantCategory:   "Shopping",
			wantConfidence: 0.7,
			wantErr:        false,
		},
		{
			name:       "API error",
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
		{
			name: "no content in response",
			mockResponse: anthropicResponse{
				Content: []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				}{},
			},
			statusCode: http.StatusOK,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/v1/messages", r.URL.Path)
				assert.Equal(t, "test-key", r.Header.Get("x-api-key"))
				assert.Equal(t, "2023-06-01", r.Header.Get("anthropic-version"))

				w.WriteHeader(tt.statusCode)
				if tt.statusCode == http.StatusOK {
					_ = json.NewEncoder(w).Encode(tt.mockResponse)
				}
			}))
			defer server.Close()

			// Create test client with server URL
			client := &testAnthropicClient{
				anthropicClient: &anthropicClient{
					apiKey:      "test-key",
					model:       "claude-3-sonnet",
					temperature: 0.3,
					maxTokens:   150,
					httpClient:  server.Client(),
				},
				baseURL: server.URL,
			}

			// Make request
			ctx := context.Background()
			resp, err := client.Classify(ctx, "Test prompt")

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantCategory, resp.Category)
				assert.Equal(t, tt.wantConfidence, resp.Confidence)
			}
		})
	}
}

func TestAnthropicClient_ParseClassification(t *testing.T) {
	client := &anthropicClient{}

	tests := []struct {
		name           string
		content        string
		wantCategory   string
		wantConfidence float64
		wantErr        bool
	}{
		{
			name:           "standard format",
			content:        "CATEGORY: Coffee & Dining\nCONFIDENCE: 0.95",
			wantCategory:   "Coffee & Dining",
			wantConfidence: 0.95,
			wantErr:        false,
		},
		{
			name:           "with extra whitespace",
			content:        "  CATEGORY:  Shopping  \n  CONFIDENCE:  0.85  ",
			wantCategory:   "Shopping",
			wantConfidence: 0.85,
			wantErr:        false,
		},
		{
			name:           "missing confidence",
			content:        "CATEGORY: Groceries",
			wantCategory:   "Groceries",
			wantConfidence: 0.7,
			wantErr:        false,
		},
		{
			name:    "missing category",
			content: "CONFIDENCE: 0.90",
			wantErr: true,
		},
		{
			name:    "invalid confidence format",
			content: "CATEGORY: Travel\nCONFIDENCE: invalid",
			wantErr: true,
		},
		{
			name:           "multiline with extra text",
			content:        "Here's my classification:\nCATEGORY: Entertainment\nCONFIDENCE: 0.88\nBased on the transaction details...",
			wantCategory:   "Entertainment",
			wantConfidence: 0.88,
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := client.parseClassification(tt.content)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantCategory, resp.Category)
				assert.Equal(t, tt.wantConfidence, resp.Confidence)
			}
		})
	}
}
