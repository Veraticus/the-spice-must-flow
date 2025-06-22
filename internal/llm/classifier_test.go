package llm

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/joshsymonds/the-spice-must-flow/internal/model"
	"github.com/joshsymonds/the-spice-must-flow/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockClient is a test implementation of the Client interface.
type mockClient struct {
	merchantResponses map[string]ClassificationResponse
	responses         []ClassificationResponse
	errors            []error
	calls             int
	mu                sync.Mutex
}

func (m *mockClient) Classify(_ context.Context, prompt string) (ClassificationResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// If merchantResponses is set, use it for predictable batch testing
	if m.merchantResponses != nil {
		// Extract merchant name from prompt
		for merchant, response := range m.merchantResponses {
			if strings.Contains(prompt, merchant) {
				m.calls++
				return response, nil
			}
		}
	}

	// Get current call index
	callIdx := m.calls
	m.calls++

	// Check if we should return an error for this call
	if callIdx < len(m.errors) && m.errors[callIdx] != nil {
		return ClassificationResponse{}, m.errors[callIdx]
	}

	// Otherwise return a response if available
	if callIdx < len(m.responses) {
		return m.responses[callIdx], nil
	}

	return ClassificationResponse{}, fmt.Errorf("no more mock responses (call %d, responses: %d)", callIdx, len(m.responses))
}

func (m *mockClient) ClassifyWithRankings(_ context.Context, prompt string) (RankingResponse, error) {
	// For testing, return a simple ranking based on the Classify method
	classResp, err := m.Classify(context.Background(), prompt)
	if err != nil {
		return RankingResponse{}, err
	}

	// Convert single classification to rankings
	rankings := []CategoryRanking{
		{
			Category:    classResp.Category,
			Score:       classResp.Confidence,
			IsNew:       classResp.IsNew,
			Description: classResp.CategoryDescription,
		},
	}

	return RankingResponse{Rankings: rankings}, nil
}

func (m *mockClient) GenerateDescription(_ context.Context, categoryName string) (DescriptionResponse, error) {
	return DescriptionResponse{
		Description: "Mock description for " + categoryName,
	}, nil
}

func TestNewClassifier(t *testing.T) {
	logger := slog.Default()

	tests := []struct {
		name    string
		errMsg  string
		config  Config
		wantErr bool
	}{
		{
			name: "valid openai config",
			config: Config{
				Provider: "openai",
				APIKey:   "test-key",
			},
			wantErr: false,
		},
		{
			name: "valid anthropic config",
			config: Config{
				Provider: "anthropic",
				APIKey:   "test-key",
			},
			wantErr: false,
		},
		{
			name: "unsupported provider",
			config: Config{
				Provider: "unknown",
				APIKey:   "test-key",
			},
			wantErr: true,
			errMsg:  "unsupported LLM provider: unknown",
		},
		{
			name: "missing api key for openai",
			config: Config{
				Provider: "openai",
			},
			wantErr: true,
			errMsg:  "OpenAI API key is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			classifier, err := NewClassifier(tt.config, logger)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
				assert.NotNil(t, classifier)
			}
		})
	}
}

func TestClassifier_SuggestCategory(t *testing.T) {
	// Set up debug logging for tests
	opts := &slog.HandlerOptions{Level: slog.LevelDebug}
	handler := slog.NewTextHandler(os.Stderr, opts)
	logger := slog.New(handler)

	ctx := context.Background()

	txn := model.Transaction{
		ID:           "test-123",
		Hash:         "hash-123",
		MerchantName: "Starbucks",
		Amount:       5.75,
		Date:         time.Now(),
	}

	tests := []struct {
		name          string
		expectedCat   string
		mockResponses []ClassificationResponse
		mockErrors    []error
		maxRetries    int
		expectedConf  float64
		expectedCalls int
		expectError   bool
	}{
		{
			name: "successful classification",
			mockResponses: []ClassificationResponse{
				{Category: "Coffee & Dining", Confidence: 0.95},
			},
			maxRetries:    3,
			expectedCat:   "Coffee & Dining",
			expectedConf:  0.95,
			expectError:   false,
			expectedCalls: 1,
		},
		{
			name: "retry on failure then success",
			mockResponses: []ClassificationResponse{
				{}, // This will be skipped due to error
				{Category: "Coffee & Dining", Confidence: 0.90},
			},
			mockErrors: []error{
				fmt.Errorf("temporary error"),
				nil,
			},
			maxRetries:    3,
			expectedCat:   "Coffee & Dining",
			expectedConf:  0.90,
			expectError:   false,
			expectedCalls: 2,
		},
		{
			name: "all retries fail",
			mockErrors: []error{
				fmt.Errorf("error 1"),
				fmt.Errorf("error 2"),
				fmt.Errorf("error 3"),
			},
			maxRetries:    3,
			expectError:   true,
			expectedCalls: 3,
		},
		{
			name: "cache hit on second call",
			mockResponses: []ClassificationResponse{
				{Category: "Coffee & Dining", Confidence: 0.95},
			},
			maxRetries:    3,
			expectedCat:   "Coffee & Dining",
			expectedConf:  0.95,
			expectError:   false,
			expectedCalls: 1, // Second call should hit cache
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockClient{
				responses: tt.mockResponses,
				errors:    tt.mockErrors,
			}

			classifier := &Classifier{
				client:      mock,
				cache:       newSuggestionCache(5 * time.Minute),
				logger:      logger,
				rateLimiter: newRateLimiter(60),
				retryOpts: service.RetryOptions{
					MaxAttempts:  tt.maxRetries,
					InitialDelay: time.Millisecond,
					MaxDelay:     10 * time.Millisecond,
					Multiplier:   2.0,
				},
			}

			// First call
			category, confidence, isNew, description, err := classifier.SuggestCategory(ctx, txn, []string{"Groceries", "Entertainment"})

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedCat, category)
				assert.Equal(t, tt.expectedConf, confidence)
			}

			assert.Equal(t, tt.expectedCalls, mock.calls)

			// Test cache hit if applicable
			if tt.name == "cache hit on second call" && !tt.expectError {
				// Second call should hit cache
				category2, confidence2, isNew2, description2, err2 := classifier.SuggestCategory(ctx, txn, []string{"Groceries", "Entertainment"})
				require.NoError(t, err2)
				assert.Equal(t, category, category2)
				assert.Equal(t, confidence, confidence2)
				assert.Equal(t, isNew, isNew2)
				assert.Equal(t, description, description2)
				assert.Equal(t, tt.expectedCalls, mock.calls) // No additional calls
			}
		})
	}
}

func TestClassifier_BatchSuggestCategories(t *testing.T) {
	logger := slog.Default()
	ctx := context.Background()

	transactions := []model.Transaction{
		{
			ID:           "test-1",
			Hash:         "hash-1",
			MerchantName: "Starbucks",
			Amount:       5.75,
			Date:         time.Now(),
		},
		{
			ID:           "test-2",
			Hash:         "hash-2",
			MerchantName: "Amazon",
			Amount:       125.00,
			Date:         time.Now(),
		},
		{
			ID:           "test-3",
			Hash:         "hash-3",
			MerchantName: "Whole Foods",
			Amount:       87.50,
			Date:         time.Now(),
		},
	}

	mock := &mockClient{
		merchantResponses: map[string]ClassificationResponse{
			"Starbucks":   {Category: "Coffee & Dining", Confidence: 0.95},
			"Amazon":      {Category: "Shopping", Confidence: 0.85},
			"Whole Foods": {Category: "Groceries", Confidence: 0.92},
		},
	}

	classifier := &Classifier{
		client:      mock,
		cache:       newSuggestionCache(5 * time.Minute),
		logger:      logger,
		rateLimiter: newRateLimiter(60),
		retryOpts: service.RetryOptions{
			MaxAttempts:  3,
			InitialDelay: time.Millisecond,
			MaxDelay:     10 * time.Millisecond,
			Multiplier:   2.0,
		},
	}

	suggestions, err := classifier.BatchSuggestCategories(ctx, transactions, []string{"Coffee & Dining", "Shopping", "Transportation"})
	require.NoError(t, err)
	require.Len(t, suggestions, 3)

	// Verify suggestions
	assert.Equal(t, "test-1", suggestions[0].TransactionID)
	assert.Equal(t, "Coffee & Dining", suggestions[0].Category)
	assert.Equal(t, 0.95, suggestions[0].Confidence)

	assert.Equal(t, "test-2", suggestions[1].TransactionID)
	assert.Equal(t, "Shopping", suggestions[1].Category)
	assert.Equal(t, 0.85, suggestions[1].Confidence)

	assert.Equal(t, "test-3", suggestions[2].TransactionID)
	assert.Equal(t, "Groceries", suggestions[2].Category)
	assert.Equal(t, 0.92, suggestions[2].Confidence)
}

func TestClassifier_BuildPrompt(t *testing.T) {
	classifier := &Classifier{}

	tests := []struct {
		name     string
		contains []string
		txn      model.Transaction
	}{
		{
			name: "with merchant name",
			txn: model.Transaction{
				MerchantName: "Starbucks",
				Name:         "STARBUCKS STORE #1234",
				Amount:       5.75,
				Date:         time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
				Category:     []string{"FOOD_AND_DRINK"},
			},
			contains: []string{
				"Merchant: Starbucks",
				"Amount: $5.75",
				"Date: 2024-01-15",
				"Description: STARBUCKS STORE #1234",
				"Category Hint: FOOD_AND_DRINK",
			},
		},
		{
			name: "without merchant name",
			txn: model.Transaction{
				Name:     "AMAZON MARKETPLACE",
				Amount:   125.99,
				Date:     time.Date(2024, 1, 20, 0, 0, 0, 0, time.UTC),
				Category: []string{"SHOPS"},
			},
			contains: []string{
				"Merchant: AMAZON MARKETPLACE",
				"Amount: $125.99",
				"Date: 2024-01-20",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := classifier.buildPrompt(tt.txn, []string{"Groceries", "Entertainment", "Coffee & Dining"})
			for _, expected := range tt.contains {
				assert.Contains(t, prompt, expected)
			}
		})
	}
}
