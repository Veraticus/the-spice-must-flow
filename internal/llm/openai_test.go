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

func TestNewOpenAIClient(t *testing.T) {
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
				Model:       "gpt-4",
				Temperature: 0.5,
				MaxTokens:   200,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := newOpenAIClient(tt.config)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, client)
			}
		})
	}
}

// testOpenAIClient extends openAIClient to allow URL override for testing.
type testOpenAIClient struct {
	*openAIClient
	baseURL string
}

func (c *testOpenAIClient) Classify(ctx context.Context, prompt string) (ClassificationResponse, error) {
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
		return ClassificationResponse{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/chat/completions", strings.NewReader(string(jsonBody)))
	if err != nil {
		return ClassificationResponse{}, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

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
		return ClassificationResponse{}, fmt.Errorf("OpenAI API error (status %d): %s", resp.StatusCode, string(body))
	}

	var response openAIResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return ClassificationResponse{}, err
	}

	if len(response.Choices) == 0 {
		return ClassificationResponse{}, fmt.Errorf("no completion choices returned")
	}

	return c.parseClassification(response.Choices[0].Message.Content)
}

func TestOpenAIClient_Classify(t *testing.T) {
	tests := []struct {
		name           string
		wantCategory   string
		mockResponse   openAIResponse
		statusCode     int
		wantConfidence float64
		wantErr        bool
	}{
		{
			name: "successful classification",
			mockResponse: openAIResponse{
				Choices: []struct {
					Message struct {
						Role    string `json:"role"`
						Content string `json:"content"`
					} `json:"message"`
					FinishReason string `json:"finish_reason"`
					Index        int    `json:"index"`
				}{
					{
						Index: 0,
						Message: struct {
							Role    string `json:"role"`
							Content string `json:"content"`
						}{
							Content: "CATEGORY: Coffee & Dining\nCONFIDENCE: 0.95",
						},
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
			mockResponse: openAIResponse{
				Choices: []struct {
					Message struct {
						Role    string `json:"role"`
						Content string `json:"content"`
					} `json:"message"`
					FinishReason string `json:"finish_reason"`
					Index        int    `json:"index"`
				}{
					{
						Index: 0,
						Message: struct {
							Role    string `json:"role"`
							Content string `json:"content"`
						}{
							Content: "CATEGORY: Shopping",
						},
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
			name: "no choices in response",
			mockResponse: openAIResponse{
				Choices: []struct {
					Message struct {
						Role    string `json:"role"`
						Content string `json:"content"`
					} `json:"message"`
					FinishReason string `json:"finish_reason"`
					Index        int    `json:"index"`
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
				assert.Equal(t, "/v1/chat/completions", r.URL.Path)
				assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

				w.WriteHeader(tt.statusCode)
				if tt.statusCode == http.StatusOK {
					_ = json.NewEncoder(w).Encode(tt.mockResponse)
				}
			}))
			defer server.Close()

			// Create test client with server URL
			client := &testOpenAIClient{
				openAIClient: &openAIClient{
					apiKey:      "test-key",
					model:       "gpt-4",
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

func TestOpenAIClient_ParseClassification(t *testing.T) {
	client := &openAIClient{}

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
