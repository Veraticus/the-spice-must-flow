package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/common"
	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/service"
	"github.com/Veraticus/the-spice-must-flow/internal/storage"
)

// BatchClassificationOptions configures batch classification behavior.
type BatchClassificationOptions struct {
	AutoAcceptThreshold float64 // Confidence threshold for auto-acceptance (0.0-1.0)
	BatchSize           int     // Number of merchants to process in each LLM batch
	ParallelWorkers     int     // Number of parallel workers
	SkipManualReview    bool    // Skip manual review of low-confidence items
}

// DefaultBatchOptions returns sensible defaults.
func DefaultBatchOptions() BatchClassificationOptions {
	return BatchClassificationOptions{
		AutoAcceptThreshold: 0.95,
		BatchSize:           20,
		ParallelWorkers:     5,
	}
}

// BatchResult contains the classification result for a merchant group.
type BatchResult struct {
	Error        error
	Suggestion   *model.CategoryRanking
	Merchant     string
	Transactions []model.Transaction
	AutoAccepted bool
}

// BatchClassificationSummary contains statistics about the batch run.
type BatchClassificationSummary struct {
	TotalMerchants    int
	TotalTransactions int
	AutoAcceptedCount int
	AutoAcceptedTxns  int
	NeedsReviewCount  int
	NeedsReviewTxns   int
	FailedCount       int
	ProcessingTime    time.Duration
}

// ClassifyTransactionsBatch performs batch classification with parallel processing.
func (e *ClassificationEngine) ClassifyTransactionsBatch(ctx context.Context, fromDate *time.Time, opts BatchClassificationOptions) (*BatchClassificationSummary, error) {
	startTime := time.Now()

	// Get transactions to classify
	transactions, err := e.GetTransactionsToClassify(ctx, fromDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get transactions: %w", err)
	}

	if len(transactions) == 0 {
		slog.Info("No transactions to classify")
		return &BatchClassificationSummary{}, nil
	}

	// Group by merchant
	merchantGroups := e.groupByMerchant(transactions)
	sortedMerchants := e.sortMerchantsByVolume(merchantGroups)

	slog.Info("Starting batch classification",
		"total_transactions", len(transactions),
		"unique_merchants", len(merchantGroups),
		"auto_accept_threshold", fmt.Sprintf("%.0f%%", opts.AutoAcceptThreshold*100))

	// Get categories upfront
	categories, err := e.storage.GetCategories(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get categories: %w", err)
	}

	// Process all merchants in parallel
	results := e.processMerchantsParallel(ctx, sortedMerchants, merchantGroups, categories, opts)

	// Build summary and separate results
	summary := &BatchClassificationSummary{
		TotalMerchants:    len(merchantGroups),
		TotalTransactions: len(transactions),
		ProcessingTime:    time.Since(startTime),
	}

	var autoAccepted []BatchResult
	var needsReview []BatchResult

	for _, result := range results {
		if result.Error != nil {
			summary.FailedCount++
			slog.Warn("Failed to classify merchant",
				"merchant", result.Merchant,
				"error", result.Error)
			continue
		}

		if result.Suggestion != nil && result.Suggestion.Score >= opts.AutoAcceptThreshold && !result.Suggestion.IsNew {
			result.AutoAccepted = true
			autoAccepted = append(autoAccepted, result)
			summary.AutoAcceptedCount++
			summary.AutoAcceptedTxns += len(result.Transactions)
		} else {
			needsReview = append(needsReview, result)
			summary.NeedsReviewCount++
			summary.NeedsReviewTxns += len(result.Transactions)
		}
	}

	// Auto-save high confidence classifications
	if err := e.saveAutoAcceptedBatch(ctx, autoAccepted); err != nil {
		slog.Error("Failed to save some auto-accepted classifications", "error", err)
	}

	// Handle manual review for remaining items (unless skipped)
	if len(needsReview) > 0 && !opts.SkipManualReview {
		if err := e.handleBatchReview(ctx, needsReview, categories); err != nil {
			return summary, fmt.Errorf("batch review failed: %w", err)
		}
	} else if len(needsReview) > 0 {
		slog.Info("Skipping manual review",
			"merchants_skipped", len(needsReview),
			"transactions_skipped", summary.NeedsReviewTxns,
			"reason", fmt.Sprintf("below %.0f%% confidence threshold", opts.AutoAcceptThreshold*100))
	}

	return summary, nil
}

// processMerchantsParallel processes merchants in parallel batches.
func (e *ClassificationEngine) processMerchantsParallel(
	ctx context.Context,
	sortedMerchants []string,
	merchantGroups map[string][]model.Transaction,
	categories []model.Category,
	opts BatchClassificationOptions,
) []BatchResult {
	// Create work channel
	workChan := make(chan string, len(sortedMerchants))
	for _, merchant := range sortedMerchants {
		workChan <- merchant
	}
	close(workChan)

	// Results channel
	resultsChan := make(chan BatchResult, len(sortedMerchants))

	// Start workers
	var wg sync.WaitGroup
	wg.Add(opts.ParallelWorkers)

	for i := 0; i < opts.ParallelWorkers; i++ {
		go func(workerID int) {
			defer wg.Done()
			e.batchWorker(ctx, workerID, workChan, resultsChan, merchantGroups, categories, opts)
		}(i)
	}

	// Wait for workers and close results
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results
	var results []BatchResult
	for result := range resultsChan {
		results = append(results, result)
	}

	return results
}

// batchWorker processes merchants from the work channel.
func (e *ClassificationEngine) batchWorker(
	ctx context.Context,
	workerID int,
	workChan <-chan string,
	resultsChan chan<- BatchResult,
	merchantGroups map[string][]model.Transaction,
	categories []model.Category,
	opts BatchClassificationOptions,
) {
	batch := make([]string, 0, opts.BatchSize)

	for merchant := range workChan {
		select {
		case <-ctx.Done():
			return
		default:
		}

		batch = append(batch, merchant)

		// Process batch when full or channel empty
		if len(batch) >= opts.BatchSize || len(workChan) == 0 {
			results := e.processMerchantBatch(ctx, batch, merchantGroups, categories)
			for _, result := range results {
				resultsChan <- result
			}
			batch = batch[:0] // Reset batch
		}
	}

	// Process any remaining merchants
	if len(batch) > 0 {
		results := e.processMerchantBatch(ctx, batch, merchantGroups, categories)
		for _, result := range results {
			resultsChan <- result
		}
	}
}

// processMerchantBatch processes a batch of merchants.
func (e *ClassificationEngine) processMerchantBatch(
	ctx context.Context,
	merchants []string,
	merchantGroups map[string][]model.Transaction,
	categories []model.Category,
) []BatchResult {
	results := make([]BatchResult, len(merchants))

	// First check for existing vendor rules
	for i, merchant := range merchants {
		txns := merchantGroups[merchant]
		result := BatchResult{
			Merchant:     merchant,
			Transactions: txns,
		}

		// Check vendor rule
		vendor, err := e.getVendor(ctx, merchant)
		if err == nil && vendor != nil {
			// Use existing vendor rule
			result.Suggestion = &model.CategoryRanking{
				Category:    vendor.Category,
				Score:       1.0, // Vendor rules have 100% confidence
				IsNew:       false,
				Description: "", // Vendors don't have descriptions
			}
			result.AutoAccepted = true
			results[i] = result
			continue
		}

		// Need LLM classification
		if len(txns) == 0 {
			result.Error = fmt.Errorf("no transactions for merchant")
			results[i] = result
			continue
		}

		representative := txns[0]

		// Get check patterns if applicable
		var checkPatterns []model.CheckPattern
		if representative.Type == "CHECK" {
			checkPatterns, _ = e.storage.GetMatchingCheckPatterns(ctx, representative)
		}

		// Filter categories by direction
		filteredCategories := e.filterCategoriesByDirection(categories, txns)

		// Get LLM suggestion with retry
		rankings, err := e.getSuggestionWithRetry(ctx, representative, filteredCategories, checkPatterns)
		if err != nil {
			result.Error = err
			results[i] = result
			continue
		}

		top := rankings.Top()
		if top == nil {
			result.Error = fmt.Errorf("no category suggestion returned")
			results[i] = result
			continue
		}

		result.Suggestion = top
		results[i] = result
	}

	return results
}

// getSuggestionWithRetry gets LLM suggestion with retry logic.
func (e *ClassificationEngine) getSuggestionWithRetry(
	ctx context.Context,
	transaction model.Transaction,
	categories []model.Category,
	checkPatterns []model.CheckPattern,
) (model.CategoryRankings, error) {
	var rankings model.CategoryRankings

	err := common.WithRetry(ctx, func() error {
		var err error
		rankings, err = e.classifier.SuggestCategoryRankings(ctx, transaction, categories, checkPatterns)
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

	return rankings, err
}

// saveAutoAcceptedBatch saves all auto-accepted classifications.
func (e *ClassificationEngine) saveAutoAcceptedBatch(ctx context.Context, results []BatchResult) error {
	saved := 0

	for _, result := range results {
		if result.Suggestion == nil {
			continue
		}

		// Verify category exists before saving (extra safety check)
		existingCategory, err := e.storage.GetCategoryByName(ctx, result.Suggestion.Category)
		if err != nil && !errors.Is(err, storage.ErrCategoryNotFound) {
			slog.Error("Failed to check category existence",
				"category", result.Suggestion.Category,
				"error", err)
			continue
		}
		
		if existingCategory == nil {
			slog.Error("Category doesn't exist, skipping auto-accept",
				"category", result.Suggestion.Category,
				"merchant", result.Merchant)
			continue
		}

		// Apply classifications to all transactions in the group
		for _, txn := range result.Transactions {
			classification := model.Classification{
				Transaction:  txn,
				Category:     result.Suggestion.Category,
				Status:       model.StatusClassifiedByAI,
				Confidence:   result.Suggestion.Score,
				ClassifiedAt: time.Now(),
			}

			if err := e.storage.SaveClassification(ctx, &classification); err != nil {
				slog.Error("Failed to save classification",
					"transaction_id", txn.ID,
					"error", err)
			} else {
				saved++
			}
		}

		// Save vendor rule if suggested
		if result.Suggestion.Score >= 0.85 {
			vendor := &model.Vendor{
				Name:        result.Merchant,
				Category:    result.Suggestion.Category,
				UseCount:    len(result.Transactions),
				LastUpdated: time.Now(),
			}
			if err := e.storage.SaveVendor(ctx, vendor); err != nil {
				slog.Warn("Failed to save vendor rule", "error", err)
			}
		}
	}

	slog.Info("Auto-accepted classifications saved",
		"count", saved,
		"merchants", len(results))

	return nil
}

// handleBatchReview handles the interactive review of uncertain classifications.
func (e *ClassificationEngine) handleBatchReview(ctx context.Context, needsReview []BatchResult, categories []model.Category) error {
	// Sort by confidence (lowest first, so most uncertain are reviewed first)
	sort.Slice(needsReview, func(i, j int) bool {
		scoreI := float64(0)
		scoreJ := float64(0)
		if needsReview[i].Suggestion != nil {
			scoreI = needsReview[i].Suggestion.Score
		}
		if needsReview[j].Suggestion != nil {
			scoreJ = needsReview[j].Suggestion.Score
		}
		return scoreI < scoreJ
	})

	// Process each merchant that needs review
	for i, result := range needsReview {
		if len(result.Transactions) == 0 {
			continue
		}

		// Create pending classification with first transaction as representative
		pending := model.PendingClassification{
			Transaction:  result.Transactions[0],
			SimilarCount: len(result.Transactions) - 1,
		}

		// Add suggestion if available
		if result.Suggestion != nil {
			pending.SuggestedCategory = result.Suggestion.Category
			pending.Confidence = result.Suggestion.Score
			pending.IsNewCategory = result.Suggestion.IsNew
			pending.CategoryDescription = result.Suggestion.Description
		}

		// Get user confirmation
		classification, err := e.prompter.ConfirmClassification(ctx, pending)
		if err != nil {
			if err == context.Canceled {
				return err
			}
			slog.Error("Failed to get user confirmation", "error", err)
			continue
		}

		// Apply classification to all transactions in the group
		for _, txn := range result.Transactions {
			// Create new classification for each transaction
			txnClassification := model.Classification{
				Transaction:  txn,
				Category:     classification.Category,
				Status:       classification.Status,
				Confidence:   classification.Confidence,
				ClassifiedAt: time.Now(),
				Notes:        classification.Notes,
			}

			if err := e.storage.SaveClassification(ctx, &txnClassification); err != nil {
				slog.Error("Failed to save classification",
					"transaction_id", txn.ID,
					"error", err)
			}

			// Update the pending classification's transaction for the next iteration if needed
			if i+1 < len(result.Transactions) {
				pending.Transaction = result.Transactions[i+1]
			}
		}
	}

	return nil
}

// GetDisplay returns a formatted summary for display.
func (s *BatchClassificationSummary) GetDisplay() string {
	if s.TotalMerchants == 0 {
		return "No transactions to classify"
	}

	autoPercent := float64(s.AutoAcceptedCount) / float64(s.TotalMerchants) * 100

	summary := fmt.Sprintf(`📊 Batch Classification Complete
────────────────────────────────────
Total merchants:      %d
Total transactions:   %d

✅ Auto-accepted:     %d merchants (%.0f%%) - %d transactions`,
		s.TotalMerchants,
		s.TotalTransactions,
		s.AutoAcceptedCount,
		autoPercent,
		s.AutoAcceptedTxns)

	if s.NeedsReviewCount > 0 {
		summary += fmt.Sprintf("\n⚠️  Needs review:      %d merchants - %d transactions",
			s.NeedsReviewCount, s.NeedsReviewTxns)
	}

	if s.FailedCount > 0 {
		summary += fmt.Sprintf("\n❌ Failed:            %d merchants", s.FailedCount)
	}

	summary += fmt.Sprintf("\n\n⏱️  Processing time:   %s", s.ProcessingTime.Round(time.Second))

	return summary
}
