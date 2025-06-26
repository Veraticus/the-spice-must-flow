// Package engine implements the core classification engine for categorizing transactions.
package engine

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/common"
	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/service"
	"github.com/Veraticus/the-spice-must-flow/internal/storage"
)

// ClassificationEngine orchestrates the classification of transactions.
type ClassificationEngine struct {
	storage    service.Storage
	classifier Classifier
	prompter   Prompter
	batchSize  int
}

// Config holds configuration options for the classification engine.
type Config struct {
	BatchSize         int
	VarianceThreshold float64
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		BatchSize:         50,
		VarianceThreshold: 10.0,
	}
}

// New creates a new classification engine with the given dependencies.
func New(storage service.Storage, classifier Classifier, prompter Prompter) *ClassificationEngine {
	config := DefaultConfig()
	return NewWithConfig(storage, classifier, prompter, config)
}

// NewWithConfig creates a new classification engine with custom configuration.
func NewWithConfig(storage service.Storage, classifier Classifier, prompter Prompter, config Config) *ClassificationEngine {
	return &ClassificationEngine{
		storage:    storage,
		classifier: classifier,
		prompter:   prompter,
		batchSize:  config.BatchSize,
	}
}

// ClassifyTransactions processes unclassified transactions and returns statistics.
func (e *ClassificationEngine) ClassifyTransactions(ctx context.Context, fromDate *time.Time) error {
	slog.Info("Starting classification engine", "from_date", fromDate)

	// Load categories from the database
	categories, err := e.storage.GetCategories(ctx)
	if err != nil {
		return fmt.Errorf("failed to load categories: %w", err)
	}

	if len(categories) == 0 {
		return fmt.Errorf("no categories found in database - please run migrations first")
	}

	// Convert to string slice for LLM
	categoryNames := make([]string, len(categories))
	for i, cat := range categories {
		categoryNames[i] = cat.Name
	}

	slog.Info("Loaded categories", "count", len(categories))

	// Load previous progress
	progress, err := e.storage.GetLatestProgress(ctx)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to load progress: %w", err)
	}

	if progress != nil && !progress.LastProcessedDate.IsZero() {
		fromDate = &progress.LastProcessedDate
		slog.Info("Resuming from previous run",
			"last_processed_date", fromDate,
			"total_processed", progress.TotalProcessed)
	}

	// Get unclassified transactions
	transactions, err := e.storage.GetTransactionsToClassify(ctx, fromDate)
	if err != nil {
		return fmt.Errorf("failed to get transactions: %w", err)
	}

	if len(transactions) == 0 {
		slog.Info("No transactions to classify")
		return nil
	}

	slog.Info("Found transactions to classify", "count", len(transactions))

	// Group transactions by merchant
	merchantGroups := e.groupByMerchant(transactions)

	// Sort merchant groups by transaction count (high-volume first)
	sortedMerchants := e.sortMerchantsByVolume(merchantGroups)

	totalProcessed := 0
	if progress != nil {
		totalProcessed = progress.TotalProcessed
	}

	// Process each merchant group
	for _, merchant := range sortedMerchants {
		select {
		case <-ctx.Done():
			// Save progress before exiting
			if len(merchantGroups[merchant]) > 0 {
				lastTxn := merchantGroups[merchant][0]
				if err := e.saveProgress(ctx, lastTxn.ID, lastTxn.Date, totalProcessed); err != nil {
					slog.Error("Failed to save progress", "error", err)
				}
			}
			return ctx.Err()
		default:
		}

		txns := merchantGroups[merchant]
		slog.Info("Processing merchant group",
			"merchant", merchant,
			"transaction_count", len(txns))

		// Check for existing vendor rule
		vendor, err := e.getVendor(ctx, merchant)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			slog.Error("Failed to get vendor", "merchant", merchant, "error", err)
			continue
		}

		var classifications []model.Classification

		if vendor != nil {
			// Apply existing vendor rule
			slog.Info("Applying vendor rule", "merchant", merchant, "category", vendor.Category)
			classifications = e.applyVendorRule(txns, vendor)

			// Update vendor use count
			vendor.UseCount += len(txns)
			if saveErr := e.storage.SaveVendor(ctx, vendor); saveErr != nil {
				slog.Warn("Failed to update vendor use count", "error", saveErr)
			}
		} else {
			// Need classification
			classifications, err = e.classifyMerchantGroup(ctx, merchant, txns, categories)
			if err != nil {
				slog.Error("Failed to classify merchant group",
					"merchant", merchant,
					"error", err)
				continue
			}
		}

		// Save classifications
		for _, classification := range classifications {
			if err := e.storage.SaveClassification(ctx, &classification); err != nil {
				slog.Error("Failed to save classification",
					"transaction_id", classification.Transaction.ID,
					"error", err)
			}
		}

		totalProcessed += len(classifications)

		// Update progress after each merchant group
		if len(txns) > 0 {
			lastTxn := txns[len(txns)-1]
			if err := e.saveProgress(ctx, lastTxn.ID, lastTxn.Date, totalProcessed); err != nil {
				slog.Warn("Failed to save progress", "error", err)
			}
		}
	}

	// Check if we were canceled before clearing progress
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Clear progress on successful completion
	if err := e.clearProgress(ctx); err != nil {
		slog.Warn("Failed to clear progress", "error", err)
	}

	slog.Info("Classification complete",
		"total_processed", totalProcessed,
		"merchant_groups", len(merchantGroups))

	return nil
}

// classifyMerchantGroup handles classification for a group of transactions from the same merchant.
func (e *ClassificationEngine) classifyMerchantGroup(ctx context.Context, merchant string, txns []model.Transaction, categories []model.Category) ([]model.Classification, error) {
	if len(txns) == 0 {
		return []model.Classification{}, nil
	}

	// Always reload categories to ensure we have the latest list (including any newly created ones)
	freshCategories, err := e.storage.GetCategories(ctx)
	if err != nil {
		slog.Warn("Failed to reload categories, using existing list", "error", err)
		// Use existing categories if reload fails
	} else {
		categories = freshCategories
	}

	// Get AI suggestion for the representative transaction
	representative := txns[0]

	// Load check patterns for CHECK transactions
	var checkPatterns []model.CheckPattern
	if representative.Type == "CHECK" {
		var checkPatternsErr error
		checkPatterns, checkPatternsErr = e.storage.GetMatchingCheckPatterns(ctx, representative)
		if checkPatternsErr != nil {
			slog.Warn("Failed to get check patterns", "error", checkPatternsErr)
			// Continue without patterns rather than failing
		}
	}

	// Filter categories by transaction direction
	categoryModels := e.filterCategoriesByDirection(categories, txns)

	var rankings model.CategoryRankings
	retryErr := common.WithRetry(ctx, func() error {
		var classifyErr error
		rankings, classifyErr = e.classifier.SuggestCategoryRankings(ctx, representative, categoryModels, checkPatterns)
		if classifyErr != nil {
			return &common.RetryableError{Err: classifyErr, Retryable: true}
		}
		return nil
	}, service.RetryOptions{
		MaxAttempts:  3,
		InitialDelay: 500 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		Multiplier:   2.0,
	})

	if retryErr != nil {
		return nil, fmt.Errorf("AI classification failed: %w", retryErr)
	}

	// Extract top-ranked category
	top := rankings.Top()
	if top == nil {
		return nil, fmt.Errorf("no category rankings returned")
	}

	category := top.Category
	confidence := top.Score
	isNew := top.IsNew
	description := top.Description

	// Auto-classify if confidence is high enough (≥85%) and it's not a new category
	if confidence >= 0.85 && !isNew {
		slog.Info("Auto-classifying with high confidence",
			"merchant", merchant,
			"category", category,
			"confidence", confidence,
			"transaction_count", len(txns))

		// IMPORTANT: Only auto-classify if the category already exists
		// Never create new categories without user consent
		existingCategory, categoryCheckErr := e.storage.GetCategoryByName(ctx, category)
		if categoryCheckErr != nil && !errors.Is(categoryCheckErr, storage.ErrCategoryNotFound) {
			return nil, fmt.Errorf("failed to check category existence: %w", categoryCheckErr)
		}

		if existingCategory == nil {
			// Category doesn't exist - treat as new and require manual review
			slog.Warn("Category suggested by LLM doesn't exist, treating as new",
				"category", category,
				"merchant", merchant)
			isNew = true
			// Fall through to manual review
		} else {
			// Category exists - safe to auto-classify
			classifications := make([]model.Classification, len(txns))
			for i, txn := range txns {
				classifications[i] = model.Classification{
					Transaction:  txn,
					Category:     category,
					Status:       model.StatusClassifiedByAI,
					Confidence:   confidence,
					ClassifiedAt: time.Now(),
				}
			}

			// Update pattern use counts if check patterns contributed
			if len(checkPatterns) > 0 {
				for _, pattern := range checkPatterns {
					if pattern.Category == category {
						if incrementErr := e.storage.IncrementCheckPatternUseCount(ctx, pattern.ID); incrementErr != nil {
							slog.Warn("Failed to increment pattern use count",
								"pattern_id", pattern.ID,
								"error", incrementErr)
						}
					}
				}
			}

			// Save vendor rule for auto-classified transactions
			vendor := &model.Vendor{
				Name:        merchant,
				Category:    category,
				LastUpdated: time.Now(),
				UseCount:    len(classifications),
			}

			if saveErr := e.storage.SaveVendor(ctx, vendor); saveErr != nil {
				slog.Warn("Failed to save vendor rule for auto-classified transactions",
					"merchant", merchant,
					"error", saveErr)
			} else {
				slog.Info("Created vendor rule from auto-classification",
					"merchant", merchant,
					"category", vendor.Category)
			}

			return classifications, nil
		}
	}

	// Check if this is a high-variance merchant
	if e.hasHighVariance(txns) {
		slog.Info("High variance detected, reviewing individually",
			"merchant", merchant,
			"transaction_count", len(txns))
		return e.reviewIndividually(ctx, merchant, txns, categories)
	}

	// Prepare for batch review
	pending := make([]model.PendingClassification, len(txns))
	for i, txn := range txns {
		pending[i] = model.PendingClassification{
			Transaction:         txn,
			SuggestedCategory:   category,
			Confidence:          confidence,
			SimilarCount:        len(txns),
			IsNewCategory:       isNew,
			CategoryDescription: description,
			CategoryRankings:    rankings,
			AllCategories:       categories,
			CheckPatterns:       checkPatterns,
		}
	}

	// Get user confirmation
	classifications, err := e.prompter.BatchConfirmClassifications(ctx, pending)
	if err != nil {
		return nil, fmt.Errorf("batch confirmation failed: %w", err)
	}

	// Save vendor rule if confirmed
	if len(classifications) > 0 {
		// Handle special "|DESC|" format for user-provided descriptions
		category := classifications[0].Category
		userProvidedDescription := ""

		if idx := strings.Index(category, "|DESC|"); idx > 0 {
			userProvidedDescription = category[idx+6:]
			category = category[:idx]
			// Update all classifications with the cleaned category name
			for i := range classifications {
				classifications[i].Category = category
			}
		}

		// If this is a new category that was accepted, or if user created a new category via 'E' option
		if (isNew || userProvidedDescription != "") && category != "" {
			// Check if category exists first
			_, err := e.storage.GetCategoryByName(ctx, category)
			if err != nil && errors.Is(err, storage.ErrCategoryNotFound) {
				// Use user-provided description if available, otherwise use AI-generated description
				finalDescription := description
				if userProvidedDescription != "" {
					finalDescription = userProvidedDescription
				}

				// Create the new category with the description
				_, createErr := e.storage.CreateCategoryWithType(ctx, category, finalDescription, model.CategoryTypeExpense)
				if createErr != nil {
					return nil, fmt.Errorf("failed to create new category %q: %w", category, createErr)
				}
				slog.Info("Created new category from batch confirmation",
					"category", category,
					"description", finalDescription)
			}
		}

		vendor := &model.Vendor{
			Name:        merchant,
			Category:    category,
			LastUpdated: time.Now(),
			UseCount:    len(classifications),
		}

		if err := e.storage.SaveVendor(ctx, vendor); err != nil {
			slog.Warn("Failed to save vendor rule",
				"merchant", merchant,
				"error", err)
		} else {
			slog.Info("Created vendor rule",
				"merchant", merchant,
				"category", vendor.Category)
		}

		// Update pattern use counts if check patterns contributed to the confirmed classification
		if len(checkPatterns) > 0 && len(classifications) > 0 {
			confirmedCategory := classifications[0].Category
			for _, pattern := range checkPatterns {
				if pattern.Category == confirmedCategory {
					if incrementErr := e.storage.IncrementCheckPatternUseCount(ctx, pattern.ID); incrementErr != nil {
						slog.Warn("Failed to increment pattern use count",
							"pattern_id", pattern.ID,
							"error", incrementErr)
					}
				}
			}
		}
	}

	return classifications, nil
}

// reviewIndividually handles high-variance merchants by reviewing each transaction.
func (e *ClassificationEngine) reviewIndividually(ctx context.Context, _ string, txns []model.Transaction, categories []model.Category) ([]model.Classification, error) {
	classifications := make([]model.Classification, 0, len(txns))

	for _, txn := range txns {
		// Reload categories to get any newly created ones
		freshCategories, err := e.storage.GetCategories(ctx)
		if err != nil {
			slog.Warn("Failed to reload categories", "error", err)
			// Use existing categories if reload fails
		} else {
			categories = freshCategories
		}

		// Filter categories by transaction direction for this specific transaction
		categoryModels := e.filterCategoriesByDirection(categories, []model.Transaction{txn})

		// Load check patterns for CHECK transactions
		var checkPatterns []model.CheckPattern
		if txn.Type == "CHECK" {
			checkPatterns, err = e.storage.GetMatchingCheckPatterns(ctx, txn)
			if err != nil {
				slog.Warn("Failed to get check patterns for individual review", "error", err)
			}
		}

		// Get AI suggestion for each transaction
		rankings, err := e.classifier.SuggestCategoryRankings(ctx, txn, categoryModels, checkPatterns)
		if err != nil {
			slog.Warn("Failed to get AI suggestion",
				"transaction_id", txn.ID,
				"error", err)
			// Use a default category if AI fails
			rankings = model.CategoryRankings{{
				Category:    "Other Expenses",
				Score:       0.0,
				IsNew:       false,
				Description: "",
			}}
		}

		// Extract top-ranked category
		top := rankings.Top()
		if top == nil {
			top = &model.CategoryRanking{
				Category:    "Other Expenses",
				Score:       0.0,
				IsNew:       false,
				Description: "",
			}
		}

		pending := model.PendingClassification{
			Transaction:         txn,
			SuggestedCategory:   top.Category,
			Confidence:          top.Score,
			SimilarCount:        1,
			IsNewCategory:       top.IsNew,
			CategoryDescription: top.Description,
			CategoryRankings:    rankings,
			AllCategories:       categories,
			CheckPatterns:       checkPatterns,
		}

		classification, err := e.prompter.ConfirmClassification(ctx, pending)
		if err != nil {
			slog.Warn("Failed to confirm classification",
				"transaction_id", txn.ID,
				"error", err)
			continue
		}

		// Handle special "|DESC|" format for user-provided descriptions
		category := classification.Category
		userProvidedDescription := ""

		if idx := strings.Index(category, "|DESC|"); idx > 0 {
			userProvidedDescription = category[idx+6:]
			category = category[:idx]
			// Update the classification with the cleaned category name
			classification.Category = category
		}

		// If this is a new category that was accepted, or if user created a new category via 'E' option
		if (pending.IsNewCategory || userProvidedDescription != "") && category != "" {
			// Check if category exists first
			_, err := e.storage.GetCategoryByName(ctx, category)
			if err != nil && errors.Is(err, storage.ErrCategoryNotFound) {
				// Use user-provided description if available, otherwise use AI-generated description
				finalDescription := pending.CategoryDescription
				if userProvidedDescription != "" {
					finalDescription = userProvidedDescription
				}

				// Create the new category with the description
				_, createErr := e.storage.CreateCategoryWithType(ctx, category, finalDescription, model.CategoryTypeExpense)
				if createErr != nil {
					slog.Error("Failed to create new category",
						"category", category,
						"error", createErr)
					continue
				}
				slog.Info("Created new category from user confirmation",
					"category", category,
					"description", finalDescription)
			}
		}

		// Update pattern use counts if check patterns contributed
		if len(checkPatterns) > 0 && classification.Category != "" {
			for _, pattern := range checkPatterns {
				if pattern.Category == classification.Category {
					if incrementErr := e.storage.IncrementCheckPatternUseCount(ctx, pattern.ID); incrementErr != nil {
						slog.Warn("Failed to increment pattern use count in individual review",
							"pattern_id", pattern.ID,
							"error", incrementErr)
					}
				}
			}
		}

		classifications = append(classifications, classification)
	}

	return classifications, nil
}

// applyVendorRule creates classifications based on an existing vendor rule.
func (e *ClassificationEngine) applyVendorRule(txns []model.Transaction, vendor *model.Vendor) []model.Classification {
	classifications := make([]model.Classification, len(txns))

	for i, txn := range txns {
		classifications[i] = model.Classification{
			Transaction:  txn,
			Category:     vendor.Category,
			Status:       model.StatusClassifiedByRule,
			Confidence:   1.0,
			ClassifiedAt: time.Now(),
		}
	}

	return classifications
}

// hasHighVariance checks if a merchant's transactions have high variance.
func (e *ClassificationEngine) hasHighVariance(txns []model.Transaction) bool {
	if len(txns) < 5 {
		return false
	}

	var minAmount, maxAmount float64
	for i, txn := range txns {
		amount := txn.Amount
		if amount < 0 {
			amount = -amount // Use absolute value
		}

		if i == 0 {
			minAmount = amount
			maxAmount = amount
		} else {
			if amount < minAmount {
				minAmount = amount
			}
			if amount > maxAmount {
				maxAmount = amount
			}
		}
	}

	// Avoid division by zero
	if minAmount == 0 {
		return maxAmount > 100 // If min is 0, any significant max indicates variance
	}

	// Check if max is more than 10x min
	return maxAmount/minAmount > 10
}

// groupByMerchant groups transactions by merchant name.
func (e *ClassificationEngine) groupByMerchant(transactions []model.Transaction) map[string][]model.Transaction {
	groups := make(map[string][]model.Transaction)

	for _, txn := range transactions {
		merchant := txn.MerchantName
		if merchant == "" {
			merchant = txn.Name // Fallback to raw name
		}
		merchant = strings.TrimSpace(merchant)

		groups[merchant] = append(groups[merchant], txn)
	}

	return groups
}

// sortMerchantsByVolume returns merchant names sorted by transaction count (descending).
func (e *ClassificationEngine) sortMerchantsByVolume(groups map[string][]model.Transaction) []string {
	type merchantVolume struct {
		name  string
		count int
	}

	volumes := make([]merchantVolume, 0, len(groups))
	for merchant, txns := range groups {
		volumes = append(volumes, merchantVolume{
			name:  merchant,
			count: len(txns),
		})
	}

	// Sort by count descending
	sort.Slice(volumes, func(i, j int) bool {
		return volumes[i].count > volumes[j].count
	})

	// Extract sorted merchant names
	merchants := make([]string, len(volumes))
	for i, v := range volumes {
		merchants[i] = v.name
	}

	return merchants
}

// getVendor retrieves a vendor from storage (which has its own cache).
func (e *ClassificationEngine) getVendor(ctx context.Context, merchantName string) (*model.Vendor, error) {
	return e.storage.GetVendor(ctx, merchantName)
}

// saveProgress saves the current classification progress.
func (e *ClassificationEngine) saveProgress(ctx context.Context, lastID string, lastDate time.Time, totalProcessed int) error {
	progress := &model.ClassificationProgress{
		LastProcessedID:   lastID,
		LastProcessedDate: lastDate,
		TotalProcessed:    totalProcessed,
		StartedAt:         time.Now(),
	}

	return e.storage.SaveProgress(ctx, progress)
}

// clearProgress removes saved progress after successful completion.
func (e *ClassificationEngine) clearProgress(ctx context.Context) error {
	// Save a completed progress marker
	progress := &model.ClassificationProgress{
		LastProcessedID:   "",
		LastProcessedDate: time.Time{},
		TotalProcessed:    0,
		StartedAt:         time.Now(),
	}

	return e.storage.SaveProgress(ctx, progress)
}

// filterCategoriesByDirection filters categories based on transaction direction.
func (e *ClassificationEngine) filterCategoriesByDirection(categories []model.Category, txns []model.Transaction) []model.Category {
	if len(txns) == 0 {
		return categories
	}

	// Determine the dominant direction of the transactions
	// For positive amounts, we want expense categories (outgoing money)
	// For negative amounts, we want income categories (incoming money)
	var positiveCount, negativeCount int
	for _, txn := range txns {
		if txn.Amount >= 0 {
			positiveCount++
		} else {
			negativeCount++
		}
	}

	// Determine if we should show income or expense categories
	showIncomeCategories := negativeCount > positiveCount

	// Also check if any transaction has explicit direction set
	hasExplicitDirection := false
	var explicitDirection model.TransactionDirection
	for _, txn := range txns {
		if txn.Direction != "" {
			hasExplicitDirection = true
			explicitDirection = txn.Direction
			break
		}
	}

	filtered := make([]model.Category, 0, len(categories))
	for _, cat := range categories {
		// Always include system categories (transfers)
		if cat.Type == model.CategoryTypeSystem {
			filtered = append(filtered, cat)
			continue
		}

		// If we have explicit direction, use that
		if hasExplicitDirection {
			switch explicitDirection {
			case model.DirectionIncome:
				if cat.Type == model.CategoryTypeIncome {
					filtered = append(filtered, cat)
				}
			case model.DirectionExpense:
				if cat.Type == model.CategoryTypeExpense || cat.Type == "" {
					// Include expense categories and untyped categories
					filtered = append(filtered, cat)
				}
			case model.DirectionTransfer:
				if cat.Type == model.CategoryTypeSystem {
					filtered = append(filtered, cat)
				}
			}
		} else {
			// Use amount-based heuristic
			if showIncomeCategories {
				if cat.Type == model.CategoryTypeIncome {
					filtered = append(filtered, cat)
				}
			} else {
				if cat.Type == model.CategoryTypeExpense || cat.Type == "" {
					// Include expense categories and untyped categories
					filtered = append(filtered, cat)
				}
			}
		}
	}

	// If we filtered out all categories, return the original list
	// This shouldn't happen in practice but is a safety measure
	if len(filtered) == 0 {
		slog.Warn("All categories filtered out by direction, returning full list")
		return categories
	}

	return filtered
}
