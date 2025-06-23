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

	"github.com/Veraticus/the-spice-must-flow/internal/classification"
	"github.com/Veraticus/the-spice-must-flow/internal/common"
	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/service"
	"github.com/Veraticus/the-spice-must-flow/internal/storage"
)

// ClassificationEngine orchestrates the classification of transactions.
type ClassificationEngine struct {
	storage         service.Storage
	classifier      Classifier
	prompter        Prompter
	patternDetector *classification.PatternDetector
	config          Config
}

// Config holds configuration options for the classification engine.
type Config struct {
	BatchSize                    int
	VarianceThreshold            float64
	DirectionConfidenceThreshold float64
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		BatchSize:                    50,
		VarianceThreshold:            10.0,
		DirectionConfidenceThreshold: 0.85, // Same as category auto-classification threshold
	}
}

// New creates a new classification engine with the given dependencies.
func New(storage service.Storage, classifier Classifier, prompter Prompter) *ClassificationEngine {
	config := DefaultConfig()
	return NewWithConfig(storage, classifier, prompter, config)
}

// NewWithConfig creates a new classification engine with custom configuration.
func NewWithConfig(storage service.Storage, classifier Classifier, prompter Prompter, config Config) *ClassificationEngine {
	// Initialize pattern detector with default patterns
	detector, err := classification.NewPatternDetector(classification.DefaultPatterns())
	if err != nil {
		slog.Warn("Failed to initialize pattern detector, continuing without it", "error", err)
		detector = nil
	}

	return &ClassificationEngine{
		storage:         storage,
		classifier:      classifier,
		prompter:        prompter,
		patternDetector: detector,
		config:          config,
	}
}

// ClassifyTransactions processes unclassified transactions and returns statistics.
func (e *ClassificationEngine) ClassifyTransactions(ctx context.Context, fromDate *time.Time) error {
	slog.Info("Starting classification engine", "from_date", fromDate)

	// Do an initial check for categories
	categories, err := e.storage.GetCategories(ctx)
	if err != nil {
		return fmt.Errorf("failed to load categories: %w", err)
	}

	if len(categories) == 0 {
		return fmt.Errorf("no categories found in database - please add categories using 'spice categories add <name>' first")
	}

	slog.Info("Initial categories loaded", "count", len(categories))

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
	for _, groupKey := range sortedMerchants {
		select {
		case <-ctx.Done():
			// Save progress before exiting
			if len(merchantGroups[groupKey]) > 0 {
				lastTxn := merchantGroups[groupKey][0]
				if err := e.saveProgress(ctx, lastTxn.ID, lastTxn.Date, totalProcessed); err != nil {
					slog.Error("Failed to save progress", "error", err)
				}
			}
			return ctx.Err()
		default:
		}

		txns := merchantGroups[groupKey]

		// Extract merchant name and direction from group key
		parts := strings.Split(groupKey, "|")
		merchant := parts[0]
		direction := model.TransactionDirection("")
		if len(parts) > 1 {
			direction = model.TransactionDirection(parts[1])
		}

		// If transactions don't have direction set, detect it
		if direction == "" && len(txns) > 0 {
			// Use the first transaction as representative for direction detection
			repTxn := txns[0]
			detectedDir, confidence, reasoning, err := e.classifier.SuggestTransactionDirection(ctx, repTxn)
			if err != nil {
				slog.Warn("Failed to detect transaction direction",
					"merchant", merchant,
					"error", err)
				// Default to expense if detection fails
				direction = model.DirectionExpense
			} else {
				// Check if we need user confirmation
				if confidence < e.config.DirectionConfidenceThreshold {
					// Prepare pending direction for user confirmation
					pending := PendingDirection{
						MerchantName:       merchant,
						TransactionCount:   len(txns),
						SampleTransaction:  repTxn,
						SuggestedDirection: detectedDir,
						Confidence:         confidence,
						Reasoning:          reasoning,
					}

					// Get user confirmation
					confirmedDir, err := e.prompter.ConfirmTransactionDirection(ctx, pending)
					if err != nil {
						slog.Error("Failed to confirm transaction direction",
							"merchant", merchant,
							"error", err)
						// Fall back to AI suggestion if user confirmation fails
						direction = detectedDir
					} else {
						direction = confirmedDir
						slog.Info("User confirmed transaction direction",
							"merchant", merchant,
							"direction", direction,
							"ai_suggestion", detectedDir,
							"ai_confidence", confidence)
					}
				} else {
					// High confidence, use AI detection
					direction = detectedDir
					slog.Info("Auto-detected transaction direction",
						"merchant", merchant,
						"direction", direction,
						"confidence", confidence,
						"reasoning", reasoning)
				}

				// Update all transactions in the group with the confirmed direction
				for i := range txns {
					txns[i].Direction = direction
					// Also update in storage
					if updateErr := e.storage.UpdateTransactionDirection(ctx, txns[i].ID, direction); updateErr != nil {
						slog.Warn("Failed to update transaction direction",
							"transaction_id", txns[i].ID,
							"error", updateErr)
					}
				}
			}
		}

		slog.Info("Processing merchant group",
			"merchant", merchant,
			"direction", direction,
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
			// Need classification - reload categories to get any newly created ones
			categories, err := e.storage.GetCategories(ctx)
			if err != nil {
				slog.Error("Failed to reload categories", "error", err)
				continue
			}

			// Filter categories based on transaction direction
			var filteredCategories []model.Category
			for _, cat := range categories {
				// Match category type to transaction direction
				switch direction {
				case model.DirectionIncome:
					if cat.Type == model.CategoryTypeIncome {
						filteredCategories = append(filteredCategories, cat)
					}
				case model.DirectionExpense:
					if cat.Type == model.CategoryTypeExpense || cat.Type == "" {
						// Include categories without type for backward compatibility
						filteredCategories = append(filteredCategories, cat)
					}
				case model.DirectionTransfer:
					if cat.Type == model.CategoryTypeSystem {
						filteredCategories = append(filteredCategories, cat)
					}
				default:
					// If no direction is set, include all categories
					filteredCategories = append(filteredCategories, cat)
				}
			}

			// If no categories match the direction, fall back to all categories
			if len(filteredCategories) == 0 {
				slog.Warn("No categories found for direction, using all categories",
					"direction", direction,
					"total_categories", len(categories))
				filteredCategories = categories
			}

			// Convert to string slice for LLM
			categoryNames := make([]string, len(filteredCategories))
			for i, cat := range filteredCategories {
				categoryNames[i] = cat.Name
			}

			classifications, err = e.classifyMerchantGroup(ctx, merchant, txns, categoryNames, direction)
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
func (e *ClassificationEngine) classifyMerchantGroup(ctx context.Context, merchant string, txns []model.Transaction, categories []string, direction model.TransactionDirection) ([]model.Classification, error) {
	if len(txns) == 0 {
		return []model.Classification{}, nil
	}

	// Get AI suggestion for the representative transaction
	representative := txns[0]

	// Load check patterns for CHECK transactions
	var checkPatterns []model.CheckPattern
	if representative.Type == "CHECK" {
		var err error
		checkPatterns, err = e.storage.GetMatchingCheckPatterns(ctx, representative)
		if err != nil {
			slog.Warn("Failed to get check patterns", "error", err)
			// Continue without patterns rather than failing
		}
	}

	// Convert category strings to model.Category objects
	categoryModels := make([]model.Category, len(categories))
	for i, cat := range categories {
		categoryModels[i] = model.Category{
			Name:        cat,
			Description: "", // Will be populated by storage if needed
		}
	}

	var rankings model.CategoryRankings
	err := common.WithRetry(ctx, func() error {
		var err error
		rankings, err = e.classifier.SuggestCategoryRankings(ctx, representative, categoryModels, checkPatterns)
		if err != nil {
			return &common.RetryableError{Err: err, Retryable: true}
		}
		return nil
	}, service.RetryOptions{
		MaxAttempts:  3,
		InitialDelay: 500 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		Multiplier:   2.0,
	})

	if err != nil {
		return nil, fmt.Errorf("AI classification failed: %w", err)
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

	// Check if this is a high-variance merchant
	if e.hasHighVariance(txns) {
		slog.Info("High variance detected, reviewing individually",
			"merchant", merchant,
			"transaction_count", len(txns))
		// Reload categories to get any newly created ones
		reloadedCategories, reloadErr := e.storage.GetCategories(ctx)
		if reloadErr != nil {
			return nil, fmt.Errorf("failed to reload categories: %w", reloadErr)
		}

		// Convert to string slice for LLM
		categoryNames := make([]string, len(reloadedCategories))
		for i, cat := range reloadedCategories {
			categoryNames[i] = cat.Name
		}

		return e.reviewIndividually(ctx, merchant, txns, categoryNames, direction)
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
			CheckPatterns:       checkPatterns,
			SuggestedDirection:  txn.Direction,
			DirectionConfidence: 1.0, // Already set from import or detection
			DirectionReasoning:  "Direction was pre-determined",
		}
	}

	// Get user confirmation
	classifications, err := e.prompter.BatchConfirmClassifications(ctx, pending)
	if err != nil {
		return nil, fmt.Errorf("batch confirmation failed: %w", err)
	}

	// Save vendor rule if confirmed
	if len(classifications) > 0 {
		// Ensure the category exists (in case user created a new one)
		category := classifications[0].Category
		if err := e.ensureCategoryExists(ctx, category, direction); err != nil {
			slog.Warn("Failed to ensure category exists",
				"category", category,
				"error", err)
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
func (e *ClassificationEngine) reviewIndividually(ctx context.Context, _ string, txns []model.Transaction, categories []string, direction model.TransactionDirection) ([]model.Classification, error) {
	classifications := make([]model.Classification, 0, len(txns))

	// Convert category strings to model.Category objects
	categoryModels := make([]model.Category, len(categories))
	for i, cat := range categories {
		categoryModels[i] = model.Category{
			Name:        cat,
			Description: "", // Will be populated by storage if needed
		}
	}

	for _, txn := range txns {
		// Load check patterns for CHECK transactions
		var checkPatterns []model.CheckPattern
		if txn.Type == "CHECK" {
			var err error
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
			defaultCategory := getDefaultCategory(direction)
			rankings = model.CategoryRankings{{
				Category:    defaultCategory,
				Score:       0.0,
				IsNew:       false,
				Description: "",
			}}
		}

		// Extract top-ranked category
		top := rankings.Top()
		if top == nil {
			defaultCategory := getDefaultCategory(direction)
			top = &model.CategoryRanking{
				Category:    defaultCategory,
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
			CheckPatterns:       checkPatterns,
			SuggestedDirection:  txn.Direction,
			DirectionConfidence: 1.0, // Already set from import or detection
			DirectionReasoning:  "Direction was pre-determined",
		}

		classification, err := e.prompter.ConfirmClassification(ctx, pending)
		if err != nil {
			slog.Warn("Failed to confirm classification",
				"transaction_id", txn.ID,
				"error", err)
			continue
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

// groupByMerchant groups transactions by merchant name and direction.
func (e *ClassificationEngine) groupByMerchant(transactions []model.Transaction) map[string][]model.Transaction {
	groups := make(map[string][]model.Transaction)

	for _, txn := range transactions {
		merchant := txn.MerchantName
		if merchant == "" {
			merchant = txn.Name // Fallback to raw name
		}
		merchant = strings.TrimSpace(merchant)

		// Include direction in the grouping key to ensure income and expense
		// transactions are classified separately
		groupKey := fmt.Sprintf("%s|%s", merchant, txn.Direction)

		groups[groupKey] = append(groups[groupKey], txn)
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

// getDefaultCategory returns the appropriate default category based on transaction direction.
func getDefaultCategory(direction model.TransactionDirection) string {
	switch direction {
	case model.DirectionIncome:
		return "Other Income"
	case model.DirectionExpense:
		return "Other Expenses"
	case model.DirectionTransfer:
		return "Transfers"
	default:
		return "Other Expenses"
	}
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

// ensureCategoryExists checks if a category exists and creates it if necessary.
func (e *ClassificationEngine) ensureCategoryExists(ctx context.Context, categoryName string, direction model.TransactionDirection) error {
	// Check if category already exists
	existingCategory, err := e.storage.GetCategoryByName(ctx, categoryName)
	if err != nil && !errors.Is(err, storage.ErrCategoryNotFound) {
		return fmt.Errorf("failed to check category existence: %w", err)
	}

	// Category already exists
	if existingCategory != nil {
		return nil
	}

	// Use LLM to generate a proper description for the category
	description, confidence, err := e.classifier.GenerateCategoryDescription(ctx, categoryName)
	if err != nil {
		// Fall back to a simple description if LLM fails
		slog.Warn("Failed to generate category description, using fallback",
			"category", categoryName,
			"error", err)
		description = fmt.Sprintf("Category for %s", categoryName)
	} else if confidence < 0.7 {
		// Log when confidence is low but still use the description
		slog.Info("Generated category description with low confidence",
			"category", categoryName,
			"confidence", confidence,
			"description", description)
	}

	// Determine category type based on transaction direction
	var categoryType model.CategoryType
	switch direction {
	case model.DirectionIncome:
		categoryType = model.CategoryTypeIncome
	case model.DirectionExpense:
		categoryType = model.CategoryTypeExpense
	case model.DirectionTransfer:
		categoryType = model.CategoryTypeSystem
	default:
		// Default to expense for backward compatibility
		categoryType = model.CategoryTypeExpense
	}

	newCategory, err := e.storage.CreateCategory(ctx, categoryName, description, categoryType)
	if err != nil {
		return fmt.Errorf("failed to create category: %w", err)
	}

	slog.Info("Created new category",
		"category", newCategory.Name,
		"id", newCategory.ID)

	return nil
}
