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

Respond with a JSON object in this exact format:

For existing categories (confidence >= 0.85):
{
  "category": "<existing category name>",
  "confidence": <0.85-1.0>,
  "isNew": false
}

For new category suggestions (confidence < 0.85):
{
  "category": "<new category suggestion>",
  "confidence": <0.0-0.84>,
  "isNew": true,
  "description": "<1-2 sentence description explaining what transactions belong in this category>"
}

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
func (c *Classifier) GenerateCategoryDescription(ctx context.Context, categoryName string) (string, float64, error) {
	// Rate limiting
	if err := c.rateLimiter.wait(ctx); err != nil {
		return "", 0, fmt.Errorf("rate limit error: %w", err)
	}

	prompt := fmt.Sprintf(`Generate a concise, helpful description for the financial category "%s".

The description should:
- Be 1-2 sentences maximum
- Explain what types of transactions belong in this category
- Be clear and specific without being overly technical
- Help both humans and AI systems understand the category's purpose

Respond with a JSON object in this exact format:
{
  "description": "<your 1-2 sentence description>",
  "confidence": <0.0-1.0>
}

Your confidence should reflect how well you understand what this category represents:
- 0.90-1.00: Very clear understanding of the category
- 0.70-0.89: Good understanding with minor uncertainty
- 0.50-0.69: Moderate understanding, category name is somewhat ambiguous
- Below 0.50: Low understanding, category is very unclear`, categoryName)

	var description string
	var confidence float64

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
		confidence = response.Confidence
		return nil
	}, c.retryOpts)

	if err != nil {
		return "", 0, fmt.Errorf("description generation failed: %w", err)
	}

	c.logger.Info("generated category description",
		"category", categoryName,
		"description", description,
		"confidence", confidence)

	return description, confidence, nil
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
		// Only log at debug level if needed for error diagnostics
		// c.logger.Debug("attempting LLM ranking classification",
		//	"transaction_id", transaction.ID)

		response, err := c.client.ClassifyWithRankings(ctx, prompt)
		if err != nil {
			c.logger.Warn("LLM ranking classification attempt failed",
				"error", err,
				"transaction_id", transaction.ID)
			return &common.RetryableError{Err: err, Retryable: true}
		}

		// Raw response will be logged only if there's an error

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
			// Log detailed debugging information
			c.logger.Warn("invalid rankings from LLM",
				"error", err,
				"transaction_id", transaction.ID,
				"prompt", prompt,
				"response_rankings", response.Rankings,
				"parsed_rankings", rankings)

			// Also log the available categories for comparison
			categoryNames := make([]string, len(categories))
			for i, cat := range categories {
				categoryNames[i] = cat.Name
			}
			c.logger.Debug("available categories for comparison",
				"transaction_id", transaction.ID,
				"available_categories", categoryNames)

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
Categories to rank (USE THESE EXACT NAMES):
%s

CRITICAL INSTRUCTIONS:
1. You MUST include ALL categories listed above in your rankings
2. Use the EXACT category names as shown above (case-sensitive, no modifications)
3. Each category should appear EXACTLY ONCE in the rankings
4. Assign a likelihood score from 0.0 to 1.0 for each category
5. The scores should reflect relative likelihood (they don't need to sum to 1.0)
6. DO NOT create variations of existing category names
7. If none of the existing categories fit well (all scores < 0.3), you may suggest ONE new category

Respond with a JSON object in this exact format:
{
  "rankings": [
    {
      "category": "EXACT_CATEGORY_NAME_FROM_LIST",
      "score": 0.95
    },
    {
      "category": "ANOTHER_EXACT_CATEGORY_NAME",
      "score": 0.05
    }
  ],
  "newCategory": {
    "name": "New Category Name",
    "score": 0.75,
    "description": "One sentence description of what belongs in this category"
  }
}

IMPORTANT: 
- Each category from the list above MUST appear exactly once in rankings
- Use the exact category names - do not modify them
- "newCategory" field is optional - only include if suggesting a genuinely new category`,
		transactionDetails,
		checkHints,
		categoryList)
}

// SuggestCategoryBatch suggests categories for multiple merchants in a single LLM call.
func (c *Classifier) SuggestCategoryBatch(ctx context.Context, requests []MerchantBatchRequest, categories []model.Category) (map[string]model.CategoryRankings, error) {
	if len(requests) == 0 {
		return make(map[string]model.CategoryRankings), nil
	}

	// Rate limiting for batch request
	if err := c.rateLimiter.wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit error: %w", err)
	}

	// Build batch prompt
	prompt := c.buildBatchPrompt(requests, categories)

	var batchResponse MerchantBatchResponse

	// Use common retry logic
	err := common.WithRetry(ctx, func() error {
		response, err := c.client.ClassifyMerchantBatch(ctx, prompt)
		if err != nil {
			c.logger.Warn("LLM batch classification attempt failed",
				"error", err,
				"batch_size", len(requests))
			return &common.RetryableError{Err: err, Retryable: true}
		}

		batchResponse = response
		return nil
	}, c.retryOpts)

	if err != nil {
		return nil, fmt.Errorf("batch classification failed: %w", err)
	}

	// Convert response to map of rankings
	results := make(map[string]model.CategoryRankings)

	// First, add all merchants with empty rankings
	for _, req := range requests {
		results[req.MerchantID] = model.CategoryRankings{}
	}

	// Then populate with actual results
	for _, classification := range batchResponse.Classifications {
		rankings := make(model.CategoryRankings, len(classification.Rankings))
		for i, r := range classification.Rankings {
			rankings[i] = model.CategoryRanking{
				Category:    r.Category,
				Score:       r.Score,
				IsNew:       r.IsNew,
				Description: r.Description,
			}
		}

		// Validate and sort rankings
		if err := rankings.Validate(); err != nil {
			c.logger.Warn("invalid rankings for merchant in batch",
				"merchant_id", classification.MerchantID,
				"error", err)
			continue
		}

		rankings.Sort()
		results[classification.MerchantID] = rankings

		// Cache the result using transaction hash from the sample
		for _, req := range requests {
			if req.MerchantID == classification.MerchantID && rankings.Top() != nil {
				c.cache.set(req.SampleTransaction.Hash, service.LLMSuggestion{
					TransactionID:       req.SampleTransaction.ID,
					Category:            rankings.Top().Category,
					Confidence:          rankings.Top().Score,
					IsNew:               rankings.Top().IsNew,
					CategoryDescription: rankings.Top().Description,
				})
				break
			}
		}
	}

	// Log results
	c.logger.Info("batch classification completed",
		"requested_merchants", len(requests),
		"successful_classifications", len(batchResponse.Classifications),
		"failed_classifications", len(requests)-len(batchResponse.Classifications))

	return results, nil
}

// buildBatchPrompt creates the prompt for batch merchant classification.
func (c *Classifier) buildBatchPrompt(requests []MerchantBatchRequest, categories []model.Category) string {
	// Build category list with descriptions
	categoryList := ""
	for _, cat := range categories {
		categoryList += fmt.Sprintf("- %s: %s\n", cat.Name, cat.Description)
	}

	// Build merchant details
	merchantDetails := ""
	for i, req := range requests {
		txn := req.SampleTransaction
		merchantDetails += fmt.Sprintf(`Merchant %d (ID: %s):
- Name: %s
- Sample Transaction: %s
- Amount: $%.2f
- Transaction Count: %d
- Transaction Type: %s

`, i+1, req.MerchantID, req.MerchantName, txn.Name, txn.Amount, req.TransactionCount, txn.Type)
	}

	return fmt.Sprintf(`You are a financial transaction classifier. Your task is to classify MULTIPLE merchants based on their transaction patterns.

Categories (USE THESE EXACT NAMES):
%s

Merchants to Classify:
%s

CRITICAL INSTRUCTIONS:
1. Classify ALL merchants listed above
2. For each merchant, provide the top 3-5 most likely categories with scores
3. Use the EXACT category names as shown above (case-sensitive)
4. Assign likelihood scores from 0.0 to 1.0
5. Each merchant MUST have a unique merchantId matching the ID provided
6. If a merchant clearly doesn't fit any category (all scores < 0.3), you may suggest ONE new category

Respond with a JSON object in this exact format:
{
  "classifications": [
    {
      "merchantId": "merchant-id-here",
      "rankings": [
        {"category": "EXACT_CATEGORY_NAME", "score": 0.95, "isNew": false},
        {"category": "ANOTHER_CATEGORY", "score": 0.05, "isNew": false}
      ]
    },
    {
      "merchantId": "another-merchant-id",
      "rankings": [
        {"category": "CATEGORY_NAME", "score": 0.90, "isNew": false},
        {"category": "New Category Name", "score": 0.85, "isNew": true, "description": "One sentence description"}
      ]
    }
  ]
}

IMPORTANT: Include ALL merchants in your response. Each merchantId must match exactly.`,
		categoryList,
		merchantDetails)
}
