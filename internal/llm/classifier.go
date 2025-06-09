package llm

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/joshsymonds/the-spice-must-flow/internal/model"
	"github.com/joshsymonds/the-spice-must-flow/internal/service"
)

// Classifier implements the engine.Classifier interface using LLM APIs.
type Classifier struct {
	client      Client
	cache       *suggestionCache
	logger      *slog.Logger
	rateLimiter *rateLimiter
	retryOpts   service.RetryOptions
}

// Config holds configuration for the LLM classifier.
type Config struct {
	Provider    string // "openai" or "anthropic"
	APIKey      string
	Model       string
	MaxRetries  int
	RetryDelay  time.Duration
	CacheTTL    time.Duration
	RateLimit   int // requests per minute
	Temperature float64
	MaxTokens   int
}

// NewClassifier creates a new LLM-based classifier.
func NewClassifier(cfg Config, logger *slog.Logger) (*Classifier, error) {
	var client Client
	var err error

	switch strings.ToLower(cfg.Provider) {
	case "openai":
		client, err = newOpenAIClient(cfg)
	case "anthropic":
		client, err = newAnthropicClient(cfg)
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", cfg.Provider)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create LLM client: %w", err)
	}

	retryOpts := service.RetryOptions{
		MaxAttempts:  cfg.MaxRetries,
		InitialDelay: cfg.RetryDelay,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
	}

	if retryOpts.MaxAttempts == 0 {
		retryOpts.MaxAttempts = 3
	}
	if retryOpts.InitialDelay == 0 {
		retryOpts.InitialDelay = time.Second
	}

	return &Classifier{
		client:      client,
		cache:       newSuggestionCache(cfg.CacheTTL),
		logger:      logger,
		retryOpts:   retryOpts,
		rateLimiter: newRateLimiter(cfg.RateLimit),
	}, nil
}

// SuggestCategory suggests a category for a single transaction.
func (c *Classifier) SuggestCategory(ctx context.Context, transaction model.Transaction) (string, float64, error) {
	// Check cache first
	if suggestion, found := c.cache.get(transaction.Hash); found {
		c.logger.Debug("cache hit for transaction",
			"transaction_id", transaction.ID,
			"merchant", transaction.MerchantName)
		return suggestion.Category, suggestion.Confidence, nil
	}

	// Rate limiting
	if err := c.rateLimiter.wait(ctx); err != nil {
		return "", 0, fmt.Errorf("rate limit error: %w", err)
	}

	// Prepare the prompt
	prompt := c.buildPrompt(transaction)

	var category string
	var confidence float64
	var lastErr error

	// Retry logic
	delay := c.retryOpts.InitialDelay
	for attempt := 1; attempt <= c.retryOpts.MaxAttempts; attempt++ {
		c.logger.Debug("attempting LLM classification",
			"attempt", attempt,
			"max_attempts", c.retryOpts.MaxAttempts,
			"transaction_id", transaction.ID)

		response, err := c.client.Classify(ctx, prompt)
		if err == nil {
			category = response.Category
			confidence = response.Confidence
			lastErr = nil // Clear any previous error
			c.logger.Debug("classification succeeded",
				"attempt", attempt,
				"category", category,
				"confidence", confidence)
			break
		}

		lastErr = err
		c.logger.Warn("LLM classification attempt failed",
			"attempt", attempt,
			"max_attempts", c.retryOpts.MaxAttempts,
			"error", err,
			"transaction_id", transaction.ID)

		if attempt < c.retryOpts.MaxAttempts {
			c.logger.Debug("waiting before retry",
				"delay", delay,
				"attempt", attempt)
			select {
			case <-ctx.Done():
				return "", 0, ctx.Err()
			case <-time.After(delay):
				delay = c.calculateBackoff(delay)
			}
		}
	}

	if lastErr != nil {
		return "", 0, fmt.Errorf("classification failed after %d attempts: %w", c.retryOpts.MaxAttempts, lastErr)
	}

	// Cache the result
	c.cache.set(transaction.Hash, service.LLMSuggestion{
		TransactionID: transaction.ID,
		Category:      category,
		Confidence:    confidence,
	})

	c.logger.Info("transaction classified",
		"transaction_id", transaction.ID,
		"merchant", transaction.MerchantName,
		"category", category,
		"confidence", confidence)

	return category, confidence, nil
}

// BatchSuggestCategories suggests categories for multiple transactions.
func (c *Classifier) BatchSuggestCategories(ctx context.Context, transactions []model.Transaction) ([]service.LLMSuggestion, error) {
	suggestions := make([]service.LLMSuggestion, len(transactions))
	errors := make([]error, len(transactions))

	// Process transactions concurrently with a worker pool
	const maxWorkers = 5
	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup

	for i, txn := range transactions {
		wg.Add(1)
		go func(idx int, transaction model.Transaction) {
			defer wg.Done()

			// Acquire semaphore
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				errors[idx] = ctx.Err()
				return
			}

			category, confidence, err := c.SuggestCategory(ctx, transaction)
			if err != nil {
				errors[idx] = err
				return
			}

			suggestions[idx] = service.LLMSuggestion{
				TransactionID: transaction.ID,
				Category:      category,
				Confidence:    confidence,
			}
		}(i, txn)
	}

	wg.Wait()

	// Check for errors
	for i, err := range errors {
		if err != nil {
			return nil, fmt.Errorf("failed to classify transaction %s: %w", transactions[i].ID, err)
		}
	}

	return suggestions, nil
}

// buildPrompt creates the prompt for transaction classification.
func (c *Classifier) buildPrompt(txn model.Transaction) string {
	merchant := txn.MerchantName
	if merchant == "" {
		merchant = txn.Name
	}

	return fmt.Sprintf(`Classify this financial transaction into one of the following personal finance categories:

Categories:
- Coffee & Dining
- Food & Dining
- Groceries
- Transportation
- Entertainment
- Shopping
- Office Supplies
- Computer & Electronics
- Healthcare
- Insurance
- Utilities
- Home & Garden
- Personal Care
- Education
- Travel
- Gifts & Donations
- Taxes
- Investments
- Other Expenses
- Miscellaneous

Transaction Details:
Merchant: %s
Amount: $%.2f
Date: %s
Description: %s
Plaid Category: %s

Respond with only the category name and confidence score (0.0-1.0) in the format:
CATEGORY: <category>
CONFIDENCE: <score>

Consider the merchant name, amount, and context to make the most accurate classification.`,
		merchant,
		txn.Amount,
		txn.Date.Format("2006-01-02"),
		txn.Name,
		txn.PlaidCategory)
}

// calculateBackoff calculates the next retry delay with exponential backoff.
func (c *Classifier) calculateBackoff(current time.Duration) time.Duration {
	next := time.Duration(float64(current) * c.retryOpts.Multiplier)
	if next > c.retryOpts.MaxDelay {
		return c.retryOpts.MaxDelay
	}
	return next
}
