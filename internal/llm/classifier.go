package llm

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/joshsymonds/the-spice-must-flow/internal/common"
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
	Provider    string // "openai", "anthropic", or "claudecode"
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
	case "claudecode":
		client, err = newClaudeCodeClient(cfg)
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
func (c *Classifier) SuggestCategory(ctx context.Context, transaction model.Transaction, categories []string) (string, float64, bool, error) {
	// Check cache first
	if suggestion, found := c.cache.get(transaction.Hash); found {
		c.logger.Debug("cache hit for transaction",
			"transaction_id", transaction.ID,
			"merchant", transaction.MerchantName)
		return suggestion.Category, suggestion.Confidence, suggestion.IsNew, nil
	}

	// Rate limiting
	if err := c.rateLimiter.wait(ctx); err != nil {
		return "", 0, false, fmt.Errorf("rate limit error: %w", err)
	}

	// Prepare the prompt
	prompt := c.buildPrompt(transaction, categories)

	var category string
	var confidence float64
	var isNew bool

	// Use common retry logic
	err := common.WithRetry(ctx, func() error {
		c.logger.Debug("attempting LLM classification",
			"transaction_id", transaction.ID)

		response, err := c.client.Classify(ctx, prompt)
		if err != nil {
			c.logger.Warn("LLM classification attempt failed",
				"error", err,
				"transaction_id", transaction.ID)
			// All LLM errors are considered retryable
			return &common.RetryableError{Err: err, Retryable: true}
		}

		category = response.Category
		confidence = response.Confidence
		isNew = response.IsNew
		c.logger.Debug("classification succeeded",
			"category", category,
			"confidence", confidence,
			"isNew", isNew)
		return nil
	}, c.retryOpts)

	if err != nil {
		return "", 0, false, fmt.Errorf("classification failed: %w", err)
	}

	// Cache the result
	c.cache.set(transaction.Hash, service.LLMSuggestion{
		TransactionID: transaction.ID,
		Category:      category,
		Confidence:    confidence,
		IsNew:         isNew,
	})

	c.logger.Info("transaction classified",
		"transaction_id", transaction.ID,
		"merchant", transaction.MerchantName,
		"category", category,
		"confidence", confidence,
		"isNew", isNew)

	return category, confidence, isNew, nil
}

// BatchSuggestCategories suggests categories for multiple transactions.
func (c *Classifier) BatchSuggestCategories(ctx context.Context, transactions []model.Transaction, categories []string) ([]service.LLMSuggestion, error) {
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

			category, confidence, isNew, err := c.SuggestCategory(ctx, transaction, categories)
			if err != nil {
				errors[idx] = err
				return
			}

			suggestions[idx] = service.LLMSuggestion{
				TransactionID: transaction.ID,
				Category:      category,
				Confidence:    confidence,
				IsNew:         isNew,
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
func (c *Classifier) buildPrompt(txn model.Transaction, categories []string) string {
	merchant := txn.MerchantName
	if merchant == "" {
		merchant = txn.Name
	}

	// Build category list
	categoryList := ""
	for _, cat := range categories {
		categoryList += fmt.Sprintf("- %s\n", cat)
	}

	// Build transaction details, handling optional fields
	transactionDetails := fmt.Sprintf("Merchant: %s\nAmount: $%.2f\nDate: %s\nDescription: %s",
		merchant,
		txn.Amount,
		txn.Date.Format("2006-01-02"),
		txn.Name)
	
	// Include transaction type if available
	if txn.Type != "" {
		transactionDetails += fmt.Sprintf("\nTransaction Type: %s", txn.Type)
	}
	
	// Include check number if it's a check
	if txn.CheckNumber != "" {
		transactionDetails += fmt.Sprintf("\nCheck Number: %s", txn.CheckNumber)
	}
	
	// Include category hints if available (from any source)
	if len(txn.Category) > 0 {
		categoryHint := strings.Join(txn.Category, " > ")
		transactionDetails += fmt.Sprintf("\nCategory Hint: %s", categoryHint)
	}

	return fmt.Sprintf(`Classify this financial transaction into the most appropriate category based solely on the transaction details.

IMPORTANT GUIDELINES:
- Base your classification purely on what the transaction IS, not assumptions about its purpose
- A coffee shop transaction could be personal breakfast OR a business meeting - classify by merchant type, not assumed intent
- Avoid inferring business vs personal use - that's for the user to decide
- When suggesting new categories, keep them neutral and descriptive (e.g., "Dining" not "Business Meals")

Existing Categories:
%s
Transaction Details:
%s

Instructions:
1. If you are confident (85%% or higher) that the transaction fits one of the existing categories, respond:
   CATEGORY: <existing category name>
   CONFIDENCE: <0.85-1.0>

2. If you are less confident (<85%%) that it fits existing categories, suggest a new category:
   CATEGORY: <new category suggestion>
   CONFIDENCE: <0.0-0.84>
   NEW: true

Focus on WHAT the transaction is, not WHY it might have occurred.`,
		categoryList,
		transactionDetails)
}

// Close stops background goroutines and cleans up resources.
func (c *Classifier) Close() error {
	if c.cache != nil {
		c.cache.Close()
	}
	if c.rateLimiter != nil {
		c.rateLimiter.Close()
	}
	return nil
}
