// Package engine implements the core classification engine for categorizing transactions.
package engine

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/service"
)

// ClassificationEngine orchestrates the classification of transactions.
type ClassificationEngine struct {
	storage           service.Storage
	classifier        Classifier
	prompter          Prompter
	patternClassifier *PatternClassifier
	batchSize         int
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
	// Create pattern classifier
	patternClassifier, err := NewPatternClassifier(storage)
	if err != nil {
		slog.Warn("failed to create pattern classifier", "error", err)
		// Continue without pattern classifier - fall back to existing behavior
		patternClassifier = nil
	}

	return &ClassificationEngine{
		storage:           storage,
		classifier:        classifier,
		prompter:          prompter,
		patternClassifier: patternClassifier,
		batchSize:         config.BatchSize,
	}
}

// ClassifyTransactions processes unclassified transactions using batch classification internally.
// This method is maintained for backward compatibility but uses batch processing for efficiency.
func (e *ClassificationEngine) ClassifyTransactions(ctx context.Context, fromDate *time.Time) error {
	slog.Info("Starting classification engine (batch mode)", "from_date", fromDate)

	// Use default batch options with manual review enabled
	opts := DefaultBatchOptions()
	opts.SkipManualReview = false // Ensure manual review for compatibility

	// Run batch classification
	summary, err := e.ClassifyTransactionsBatch(ctx, fromDate, opts)
	if err != nil {
		return fmt.Errorf("batch classification failed: %w", err)
	}

	// Log summary for compatibility
	slog.Info("Classification complete",
		"total_merchants", summary.TotalMerchants,
		"total_transactions", summary.TotalTransactions,
		"auto_accepted", summary.AutoAcceptedCount,
		"reviewed", summary.NeedsReviewCount,
		"failed", summary.FailedCount)

	return nil
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
// It checks both exact matches and regex patterns.
func (e *ClassificationEngine) getVendor(ctx context.Context, merchantName string) (*model.Vendor, error) {
	return e.storage.FindVendorMatch(ctx, merchantName)
}

// RefreshPatternRules reloads pattern rules from storage.
func (e *ClassificationEngine) RefreshPatternRules(ctx context.Context) error {
	if e.patternClassifier == nil {
		// Try to create pattern classifier if it doesn't exist
		patternClassifier, err := NewPatternClassifier(e.storage)
		if err != nil {
			return fmt.Errorf("failed to create pattern classifier: %w", err)
		}
		e.patternClassifier = patternClassifier
		return nil
	}

	return e.patternClassifier.RefreshPatterns(ctx)
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
