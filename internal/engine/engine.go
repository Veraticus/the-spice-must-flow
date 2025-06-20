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

	"github.com/joshsymonds/the-spice-must-flow/internal/common"
	"github.com/joshsymonds/the-spice-must-flow/internal/model"
	"github.com/joshsymonds/the-spice-must-flow/internal/service"
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
			classifications, err = e.classifyMerchantGroup(ctx, merchant, txns, categoryNames)
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
func (e *ClassificationEngine) classifyMerchantGroup(ctx context.Context, merchant string, txns []model.Transaction, categories []string) ([]model.Classification, error) {
	if len(txns) == 0 {
		return []model.Classification{}, nil
	}

	// Get AI suggestion for the representative transaction
	representative := txns[0]

	var category string
	var confidence float64
	var isNew bool
	var description string

	err := common.WithRetry(ctx, func() error {
		var err error
		category, confidence, isNew, description, err = e.classifier.SuggestCategory(ctx, representative, categories)
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
		if err := e.ensureCategoryExists(ctx, category); err != nil {
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
	}

	return classifications, nil
}

// reviewIndividually handles high-variance merchants by reviewing each transaction.
func (e *ClassificationEngine) reviewIndividually(ctx context.Context, _ string, txns []model.Transaction, categories []string) ([]model.Classification, error) {
	classifications := make([]model.Classification, 0, len(txns))

	for _, txn := range txns {
		// Get AI suggestion for each transaction
		category, confidence, isNew, description, err := e.classifier.SuggestCategory(ctx, txn, categories)
		if err != nil {
			slog.Warn("Failed to get AI suggestion",
				"transaction_id", txn.ID,
				"error", err)
			// Use a default category if AI fails
			category = "Other Expenses"
			confidence = 0.0
			isNew = false
			description = ""
		}

		pending := model.PendingClassification{
			Transaction:         txn,
			SuggestedCategory:   category,
			Confidence:          confidence,
			SimilarCount:        1,
			IsNewCategory:       isNew,
			CategoryDescription: description,
		}

		classification, err := e.prompter.ConfirmClassification(ctx, pending)
		if err != nil {
			slog.Warn("Failed to confirm classification",
				"transaction_id", txn.ID,
				"error", err)
			continue
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

// ensureCategoryExists checks if a category exists and creates it if necessary.
func (e *ClassificationEngine) ensureCategoryExists(ctx context.Context, categoryName string) error {
	// Check if category already exists
	existingCategory, err := e.storage.GetCategoryByName(ctx, categoryName)
	if err != nil {
		return fmt.Errorf("failed to check category existence: %w", err)
	}

	// Category already exists
	if existingCategory != nil {
		return nil
	}

	// Use LLM to generate a proper description for the category
	description, err := e.classifier.GenerateCategoryDescription(ctx, categoryName)
	if err != nil {
		// Fall back to a simple description if LLM fails
		slog.Warn("Failed to generate category description, using fallback",
			"category", categoryName,
			"error", err)
		description = fmt.Sprintf("Category for %s related expenses", categoryName)
	}

	newCategory, err := e.storage.CreateCategory(ctx, categoryName, description)
	if err != nil {
		return fmt.Errorf("failed to create category: %w", err)
	}

	slog.Info("Created new category",
		"category", newCategory.Name,
		"id", newCategory.ID)

	return nil
}
