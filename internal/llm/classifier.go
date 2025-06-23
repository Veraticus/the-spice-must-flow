package llm

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/common"
	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/service"
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
	Provider       string
	APIKey         string
	Model          string
	ClaudeCodePath string
	MaxRetries     int
	RetryDelay     time.Duration
	CacheTTL       time.Duration
	RateLimit      int
	Temperature    float64
	MaxTokens      int
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
// This method now uses the ranking system internally for backward compatibility.
func (c *Classifier) SuggestCategory(ctx context.Context, transaction model.Transaction, categories []string) (string, float64, bool, string, error) {
	// Check cache first
	if suggestion, found := c.cache.get(transaction.Hash); found {
		c.logger.Debug("cache hit for transaction",
			"transaction_id", transaction.ID,
			"merchant", transaction.MerchantName)
		return suggestion.Category, suggestion.Confidence, suggestion.IsNew, suggestion.CategoryDescription, nil
	}

	// Convert string categories to model.Category slice
	// Since we don't have descriptions here, we'll use empty descriptions
	categoryModels := make([]model.Category, len(categories))
	for i, cat := range categories {
		categoryModels[i] = model.Category{
			Name:        cat,
			Description: "", // Will be populated by LLM if needed
		}
	}

	// Use the new ranking method internally
	rankings, err := c.SuggestCategoryRankings(ctx, transaction, categoryModels, nil)
	if err != nil {
		return "", 0, false, "", err
	}

	// Get the top-ranked category
	top := rankings.Top()
	if top == nil {
		return "", 0, false, "", fmt.Errorf("no category rankings returned")
	}

	// Cache the result
	c.cache.set(transaction.Hash, service.LLMSuggestion{
		TransactionID:       transaction.ID,
		Category:            top.Category,
		Confidence:          top.Score,
		IsNew:               top.IsNew,
		CategoryDescription: top.Description,
	})

	c.logger.Info("transaction classified (via rankings)",
		"transaction_id", transaction.ID,
		"merchant", transaction.MerchantName,
		"category", top.Category,
		"confidence", top.Score,
		"isNew", top.IsNew)

	return top.Category, top.Score, top.IsNew, top.Description, nil
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

			category, confidence, isNew, description, err := c.SuggestCategory(ctx, transaction, categories)
			if err != nil {
				errors[idx] = err
				return
			}

			suggestions[idx] = service.LLMSuggestion{
				TransactionID:       transaction.ID,
				Category:            category,
				Confidence:          confidence,
				IsNew:               isNew,
				CategoryDescription: description,
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
   DESCRIPTION: <1-2 sentence description explaining what transactions belong in this category>

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

// GenerateCategoryDescription generates a description for a category name.
func (c *Classifier) GenerateCategoryDescription(ctx context.Context, categoryName string) (string, error) {
	// Rate limiting
	if err := c.rateLimiter.wait(ctx); err != nil {
		return "", fmt.Errorf("rate limit error: %w", err)
	}

	prompt := fmt.Sprintf(`Generate a concise, helpful description for the financial category "%s".

The description should:
- Be 1-2 sentences maximum
- Explain what types of transactions belong in this category
- Be clear and specific without being overly technical
- Help both humans and AI systems understand the category's purpose

Respond with ONLY the description text, no additional formatting or explanation.`, categoryName)

	var description string

	// Use common retry logic
	err := common.WithRetry(ctx, func() error {
		response, err := c.client.GenerateDescription(ctx, prompt)
		if err != nil {
			c.logger.Warn("description generation attempt failed",
				"error", err,
				"category", categoryName)
			return &common.RetryableError{Err: err, Retryable: true}
		}

		description = response.Description
		return nil
	}, c.retryOpts)

	if err != nil {
		return "", fmt.Errorf("description generation failed: %w", err)
	}

	c.logger.Info("generated category description",
		"category", categoryName,
		"description", description)

	return description, nil
}

// SuggestCategoryRankings suggests category rankings for a transaction.
func (c *Classifier) SuggestCategoryRankings(ctx context.Context, transaction model.Transaction, categories []model.Category, checkPatterns []model.CheckPattern) (model.CategoryRankings, error) {
	// Check cache first for backward compatibility with SuggestCategory
	if suggestion, found := c.cache.get(transaction.Hash); found {
		c.logger.Debug("cache hit for transaction (converting to rankings)",
			"transaction_id", transaction.ID,
			"merchant", transaction.MerchantName)

		// Convert cached suggestion to rankings format
		rankings := model.CategoryRankings{
			{
				Category:    suggestion.Category,
				Score:       suggestion.Confidence,
				IsNew:       suggestion.IsNew,
				Description: suggestion.CategoryDescription,
			},
		}
		return rankings, nil
	}

	// Rate limiting
	if err := c.rateLimiter.wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit error: %w", err)
	}

	// Prepare the ranking prompt
	prompt := c.buildPromptWithRanking(transaction, categories, checkPatterns)

	var rankings model.CategoryRankings

	// Use common retry logic
	err := common.WithRetry(ctx, func() error {
		c.logger.Debug("attempting LLM ranking classification",
			"transaction_id", transaction.ID)

		response, err := c.client.ClassifyWithRankings(ctx, prompt)
		if err != nil {
			c.logger.Warn("LLM ranking classification attempt failed",
				"error", err,
				"transaction_id", transaction.ID)
			return &common.RetryableError{Err: err, Retryable: true}
		}

		// Convert response to model.CategoryRankings
		rankings = make(model.CategoryRankings, len(response.Rankings))
		for i, r := range response.Rankings {
			rankings[i] = model.CategoryRanking{
				Category:    r.Category,
				Score:       r.Score,
				IsNew:       r.IsNew,
				Description: r.Description,
			}
		}

		// Validate the rankings
		if err := rankings.Validate(); err != nil {
			c.logger.Warn("invalid rankings from LLM",
				"error", err,
				"transaction_id", transaction.ID)
			return &common.RetryableError{Err: fmt.Errorf("invalid rankings: %w", err), Retryable: true}
		}

		return nil
	}, c.retryOpts)

	if err != nil {
		return nil, fmt.Errorf("ranking classification failed: %w", err)
	}

	// Apply check pattern boosts if any patterns matched
	if len(checkPatterns) > 0 {
		rankings.ApplyCheckPatternBoosts(checkPatterns)
		c.logger.Debug("applied check pattern boosts",
			"transaction_id", transaction.ID,
			"pattern_count", len(checkPatterns))
	}

	// Sort rankings by score
	rankings.Sort()

	c.logger.Info("transaction rankings classified",
		"transaction_id", transaction.ID,
		"merchant", transaction.MerchantName,
		"top_category", rankings.Top().Category,
		"top_score", rankings.Top().Score,
		"ranking_count", len(rankings))

	return rankings, nil
}

// SuggestTransactionDirection suggests the direction for a transaction (income/expense/transfer).
func (c *Classifier) SuggestTransactionDirection(ctx context.Context, transaction model.Transaction) (model.TransactionDirection, float64, string, error) {
	// Rate limiting
	if err := c.rateLimiter.wait(ctx); err != nil {
		return "", 0, "", fmt.Errorf("rate limit error: %w", err)
	}

	prompt := c.buildDirectionPrompt(transaction)

	var direction model.TransactionDirection
	var confidence float64
	var reasoning string

	// Use common retry logic
	err := common.WithRetry(ctx, func() error {
		c.logger.Debug("attempting LLM direction detection",
			"transaction_id", transaction.ID)

		response, err := c.client.ClassifyDirection(ctx, prompt)
		if err != nil {
			c.logger.Warn("LLM direction detection attempt failed",
				"error", err,
				"transaction_id", transaction.ID)
			return &common.RetryableError{Err: err, Retryable: true}
		}

		direction = response.Direction
		confidence = response.Confidence
		reasoning = response.Reasoning

		// Validate the direction
		switch direction {
		case model.DirectionIncome, model.DirectionExpense, model.DirectionTransfer:
			// Valid
		default:
			return &common.RetryableError{
				Err:       fmt.Errorf("invalid direction returned: %s", direction),
				Retryable: true,
			}
		}

		return nil
	}, c.retryOpts)

	if err != nil {
		return "", 0, "", fmt.Errorf("direction detection failed: %w", err)
	}

	c.logger.Info("transaction direction detected",
		"transaction_id", transaction.ID,
		"merchant", transaction.MerchantName,
		"direction", direction,
		"confidence", confidence)

	return direction, confidence, reasoning, nil
}

// buildDirectionPrompt creates the prompt for transaction direction detection.
func (c *Classifier) buildDirectionPrompt(txn model.Transaction) string {
	merchant := txn.MerchantName
	if merchant == "" {
		merchant = txn.Name
	}

	// Build transaction details
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

	// Include category hints if available
	if len(txn.Category) > 0 {
		categoryHint := strings.Join(txn.Category, " > ")
		transactionDetails += fmt.Sprintf("\nCategory Hint: %s", categoryHint)
	}

	return fmt.Sprintf(`Analyze this financial transaction and determine its direction (income, expense, or transfer).

Transaction Details:
%s

Common patterns:
- INCOME: salary, payroll, interest, dividends, refunds (positive amounts back to account), deposits, rewards
- EXPENSE: purchases, payments, fees, withdrawals, services (money leaving the account)
- TRANSFER: moving money between your own accounts, internal transfers, credit card payments

Respond in this exact format:
DIRECTION: <income|expense|transfer>
CONFIDENCE: <0.0-1.0>
REASONING: <brief explanation of why this direction was chosen>

Focus on transaction patterns and merchant types, not assumptions about personal vs business use.`,
		transactionDetails)
}

// buildPromptWithRanking creates the prompt for transaction ranking classification.
func (c *Classifier) buildPromptWithRanking(txn model.Transaction, categories []model.Category, checkPatterns []model.CheckPattern) string {
	merchant := txn.MerchantName
	if merchant == "" {
		merchant = txn.Name
	}

	// Build transaction details
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

	// Add check pattern hints if applicable
	checkHints := ""
	if txn.Type == "CHECK" && len(checkPatterns) > 0 {
		checkHints = "Check Pattern Matches:\n"
		for _, pattern := range checkPatterns {
			checkHints += fmt.Sprintf("- Pattern '%s' suggests category '%s' (based on %d previous uses)\n",
				pattern.PatternName, pattern.Category, pattern.UseCount)
		}
		checkHints += "\n"
	}

	// Build category list with descriptions
	categoryList := ""
	for _, cat := range categories {
		categoryList += fmt.Sprintf("- %s: %s\n", cat.Name, cat.Description)
	}

	return fmt.Sprintf(`You are a financial transaction classifier. Your task is to rank ALL provided categories by how likely this transaction belongs to each one.

Transaction Details:
%s

%s
Categories to rank:
%s

Instructions:
1. Analyze the transaction and rank EVERY category by likelihood (0.0 to 1.0)
2. The scores should be relative probabilities (they don't need to sum to 1.0)
3. If none of the existing categories fit well, you may suggest ONE new category with score >0.7
4. Return results in this exact format:

RANKINGS:
category_name|score
category_name|score
...

NEW_CATEGORY (only if needed):
name: Category Name
score: 0.75
description: One sentence description of what belongs in this category`,
		transactionDetails,
		checkHints,
		categoryList)
}
