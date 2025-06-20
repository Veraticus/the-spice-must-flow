//go:build integration
// +build integration

package llm

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/joshsymonds/the-spice-must-flow/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLLMIntegration_OpenAI(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping OpenAI integration test")
	}

	logger := slog.Default()
	cfg := Config{
		Provider:    "openai",
		APIKey:      apiKey,
		Model:       "gpt-3.5-turbo",
		MaxRetries:  3,
		RetryDelay:  time.Second,
		CacheTTL:    5 * time.Minute,
		RateLimit:   20,
		Temperature: 0.3,
		MaxTokens:   150,
	}

	classifier, err := NewClassifier(cfg, logger)
	require.NoError(t, err)

	ctx := context.Background()

	// Test single transaction classification
	t.Run("single transaction", func(t *testing.T) {
		txn := model.Transaction{
			ID:            "test-openai-1",
			Hash:          "hash-openai-1",
			MerchantName:  "Starbucks Coffee",
			Amount:        5.75,
			Date:          time.Now(),
			Category: []string{"FOOD_AND_DRINK"},
		}

		category, confidence, _, _, err := classifier.SuggestCategory(ctx, txn, []string{"Coffee & Dining", "Shopping", "Groceries"})
		require.NoError(t, err)
		assert.NotEmpty(t, category)
		assert.Greater(t, confidence, 0.0)
		assert.LessOrEqual(t, confidence, 1.0)

		// Should contain "Coffee" or "Dining" or "Food"
		assert.Regexp(t, "(?i)(coffee|dining|food)", category)
	})

	// Test batch classification
	t.Run("batch transactions", func(t *testing.T) {
		transactions := []model.Transaction{
			{
				ID:            "test-openai-2",
				Hash:          "hash-openai-2",
				MerchantName:  "Amazon.com",
				Amount:        125.99,
				Date:          time.Now(),
				Category: []string{"SHOPS"},
			},
			{
				ID:            "test-openai-3",
				Hash:          "hash-openai-3",
				MerchantName:  "Whole Foods Market",
				Amount:        87.50,
				Date:          time.Now(),
				Category: []string{"FOOD_AND_DRINK"},
			},
			{
				ID:           "test-openai-4",
				Hash:         "hash-openai-4",
				MerchantName: "Shell Gas Station",
				Amount:       45.00,
				Date:         time.Now(),
			},
		}

		suggestions, err := classifier.BatchSuggestCategories(ctx, transactions, []string{"Coffee & Dining", "Shopping", "Transportation", "Groceries"})
		require.NoError(t, err)
		require.Len(t, suggestions, 3)

		for i, suggestion := range suggestions {
			assert.Equal(t, transactions[i].ID, suggestion.TransactionID)
			assert.NotEmpty(t, suggestion.Category)
			assert.Greater(t, suggestion.Confidence, 0.0)
			assert.LessOrEqual(t, suggestion.Confidence, 1.0)
		}

		// Verify appropriate categories
		assert.Regexp(t, "(?i)(shopping|computer|electronics|office)", suggestions[0].Category)
		assert.Regexp(t, "(?i)(grocer|food)", suggestions[1].Category)
		assert.Regexp(t, "(?i)(transport|gas|fuel)", suggestions[2].Category)
	})

	// Test caching
	t.Run("caching", func(t *testing.T) {
		txn := model.Transaction{
			ID:           "test-cache-1",
			Hash:         "hash-cache-1",
			MerchantName: "Netflix",
			Amount:       15.99,
			Date:         time.Now(),
		}

		// First call
		start := time.Now()
		category1, confidence1, err := classifier.SuggestCategory(ctx, txn)
		require.NoError(t, err)
		duration1 := time.Since(start)

		// Second call (should hit cache)
		start = time.Now()
		category2, confidence2, err := classifier.SuggestCategory(ctx, txn)
		require.NoError(t, err)
		duration2 := time.Since(start)

		// Cache hit should be much faster
		assert.Less(t, duration2, duration1/10)
		assert.Equal(t, category1, category2)
		assert.Equal(t, confidence1, confidence2)
	})
}

func TestLLMIntegration_Anthropic(t *testing.T) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY not set, skipping Anthropic integration test")
	}

	logger := slog.Default()
	cfg := Config{
		Provider:    "anthropic",
		APIKey:      apiKey,
		Model:       "claude-3-haiku-20240307",
		MaxRetries:  3,
		RetryDelay:  time.Second,
		CacheTTL:    5 * time.Minute,
		RateLimit:   20,
		Temperature: 0.3,
		MaxTokens:   150,
	}

	classifier, err := NewClassifier(cfg, logger)
	require.NoError(t, err)

	ctx := context.Background()

	// Test single transaction classification
	t.Run("single transaction", func(t *testing.T) {
		txn := model.Transaction{
			ID:            "test-anthropic-1",
			Hash:          "hash-anthropic-1",
			MerchantName:  "Uber",
			Amount:        25.50,
			Date:          time.Now(),
			Category: []string{"TRAVEL"},
		}

		category, confidence, _, _, err := classifier.SuggestCategory(ctx, txn, []string{"Coffee & Dining", "Shopping", "Groceries"})
		require.NoError(t, err)
		assert.NotEmpty(t, category)
		assert.Greater(t, confidence, 0.0)
		assert.LessOrEqual(t, confidence, 1.0)

		// Should be categorized as Transportation or Travel
		assert.Regexp(t, "(?i)(transport|travel)", category)
	})

	// Test edge cases
	t.Run("edge cases", func(t *testing.T) {
		transactions := []model.Transaction{
			{
				ID:           "test-edge-1",
				Hash:         "hash-edge-1",
				MerchantName: "", // Empty merchant name
				Name:         "DIRECT DEPOSIT PAYROLL",
				Amount:       2500.00,
				Date:         time.Now(),
			},
			{
				ID:           "test-edge-2",
				Hash:         "hash-edge-2",
				MerchantName: "7384928374 MERCHANT",
				Amount:       0.01, // Very small amount
				Date:         time.Now(),
			},
		}

		suggestions, err := classifier.BatchSuggestCategories(ctx, transactions, []string{"Coffee & Dining", "Shopping", "Transportation", "Groceries"})
		require.NoError(t, err)
		require.Len(t, suggestions, 2)

		// Even edge cases should get reasonable categories
		for _, suggestion := range suggestions {
			assert.NotEmpty(t, suggestion.Category)
			assert.Greater(t, suggestion.Confidence, 0.0)
		}
	})
}

func TestLLMIntegration_RateLimiting(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping rate limiting test")
	}

	logger := slog.Default()
	cfg := Config{
		Provider:    "openai",
		APIKey:      apiKey,
		Model:       "gpt-3.5-turbo",
		MaxRetries:  1,
		RetryDelay:  time.Second,
		CacheTTL:    5 * time.Minute,
		RateLimit:   2, // Very low rate limit for testing
		Temperature: 0.3,
		MaxTokens:   150,
	}

	classifier, err := NewClassifier(cfg, logger)
	require.NoError(t, err)

	ctx := context.Background()

	// Make rapid requests to test rate limiting
	start := time.Now()
	for i := 0; i < 3; i++ {
		txn := model.Transaction{
			ID:           "test-rate-" + string(rune(i)),
			Hash:         "hash-rate-" + string(rune(i)),
			MerchantName: "Test Merchant",
			Amount:       10.00,
			Date:         time.Now(),
		}
		_, _, _, _, err := classifier.SuggestCategory(ctx, txn, []string{"Test Category"})
		require.NoError(t, err)
	}
	duration := time.Since(start)

	// With rate limit of 2/minute, 3rd request should have waited
	assert.Greater(t, duration, 500*time.Millisecond)
}
