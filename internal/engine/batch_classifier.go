package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/llm"
	"github.com/Veraticus/the-spice-must-flow/internal/model"
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
		BatchSize:           5,
		ParallelWorkers:     2,
	}
}

// BatchResult contains the classification result for a merchant group.
type BatchResult struct {
	Error        error
	Suggestion   *model.CategoryRanking
	Merchant     string
	Transactions []model.Transaction
	UsedPatterns []model.CheckPattern
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
	results := make([]BatchResult, 0, len(sortedMerchants))
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
	// workerID is used for debugging/logging purposes
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
			slog.Debug("worker processing batch",
				"worker_id", workerID,
				"batch_size", len(batch),
				"merchants", batch)
			results := e.processMerchantBatch(ctx, batch, merchantGroups, categories, opts)
			for _, result := range results {
				resultsChan <- result
			}
			batch = batch[:0] // Reset batch
		}
	}

	// Process any remaining merchants
	if len(batch) > 0 {
		slog.Debug("worker processing final batch",
			"worker_id", workerID,
			"batch_size", len(batch),
			"merchants", batch)
		results := e.processMerchantBatch(ctx, batch, merchantGroups, categories, opts)
		for _, result := range results {
			resultsChan <- result
		}
	}
}

// processMerchantBatch processes a batch of merchants using the new batch LLM API.
func (e *ClassificationEngine) processMerchantBatch(
	ctx context.Context,
	merchants []string,
	merchantGroups map[string][]model.Transaction,
	categories []model.Category,
	opts BatchClassificationOptions,
) []BatchResult {
	results := make([]BatchResult, len(merchants))
	needsLLM := make([]llm.MerchantBatchRequest, 0, len(merchants))
	needsLLMIndices := make([]int, 0, len(merchants))

	// First pass: check for existing vendor rules and prepare LLM requests
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

			// Log vendor rule match
			slog.Info("merchant classified (vendor rule)",
				"merchant", merchant,
				"category", vendor.Category,
				"confidence", "1.00",
				"transaction_count", len(txns))
			continue
		}

		// Need LLM classification
		if len(txns) == 0 {
			result.Error = fmt.Errorf("no transactions for merchant")
			results[i] = result
			continue
		}

		// Prepare batch request for this merchant
		req := llm.MerchantBatchRequest{
			MerchantID:        merchant,
			MerchantName:      merchant,
			SampleTransaction: txns[0],
			TransactionCount:  len(txns),
		}
		needsLLM = append(needsLLM, req)
		needsLLMIndices = append(needsLLMIndices, i)
		results[i] = result
	}

	// If no merchants need LLM classification, return early
	if len(needsLLM) == 0 {
		return results
	}

	// Filter categories by direction for all transactions
	// Use the most common direction from all transactions
	allTxns := make([]model.Transaction, 0)
	for _, req := range needsLLM {
		allTxns = append(allTxns, merchantGroups[req.MerchantID]...)
	}
	filteredCategories := e.filterCategoriesByDirection(categories, allTxns)

	// Process LLM requests in batches
	llmBatchSize := opts.BatchSize
	for start := 0; start < len(needsLLM); start += llmBatchSize {
		end := start + llmBatchSize
		if end > len(needsLLM) {
			end = len(needsLLM)
		}

		batch := needsLLM[start:end]
		batchIndices := needsLLMIndices[start:end]

		// Get batch classifications from LLM
		batchRankings, err := e.classifier.SuggestCategoryBatch(ctx, batch, filteredCategories)
		if err != nil {
			// If batch fails, mark all merchants in batch as failed
			for j, idx := range batchIndices {
				results[idx].Error = fmt.Errorf("batch classification failed: %w", err)
				results[idx].Merchant = batch[j].MerchantID
				results[idx].Transactions = merchantGroups[batch[j].MerchantID]
			}
			continue
		}

		// Process batch results
		for j, req := range batch {
			idx := batchIndices[j]
			merchantID := req.MerchantID

			rankings, found := batchRankings[merchantID]
			if !found || len(rankings) == 0 {
				results[idx].Error = fmt.Errorf("no rankings returned for merchant")
				results[idx].Merchant = merchantID
				results[idx].Transactions = merchantGroups[merchantID]
				continue
			}

			// Apply check pattern boosts if applicable
			txns := merchantGroups[merchantID]
			var usedPatterns []model.CheckPattern
			if len(txns) > 0 && txns[0].Type == "CHECK" {
				checkPatterns, _ := e.storage.GetMatchingCheckPatterns(ctx, txns[0])
				if len(checkPatterns) > 0 {
					// Apply boosts
					rankings.ApplyCheckPatternBoosts(checkPatterns)
					rankings.Sort()

					// Check if any pattern's category is now the top after boosting
					newTopCategory := ""
					if top := rankings.Top(); top != nil {
						newTopCategory = top.Category
					}

					// Track patterns that match the final chosen category
					for _, pattern := range checkPatterns {
						if pattern.Category == newTopCategory {
							usedPatterns = append(usedPatterns, pattern)
						}
					}
				}
			}

			top := rankings.Top()
			if top == nil {
				results[idx].Error = fmt.Errorf("no category suggestion returned")
				results[idx].Merchant = merchantID
				results[idx].Transactions = txns
				continue
			}

			results[idx].Merchant = merchantID
			results[idx].Transactions = txns
			results[idx].Suggestion = top
			results[idx].UsedPatterns = usedPatterns

			// Log the classification result for this merchant
			slog.Info("merchant classified",
				"merchant", merchantID,
				"category", top.Category,
				"confidence", fmt.Sprintf("%.2f", top.Score),
				"isNew", top.IsNew,
				"transaction_count", len(txns))
		}
	}

	return results
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
		isVendorRule := result.Suggestion.Score == 1.0
		for _, txn := range result.Transactions {
			// Determine status based on confidence score
			status := model.StatusClassifiedByAI
			if isVendorRule {
				// Score of 1.0 indicates vendor rule
				status = model.StatusClassifiedByRule
			}

			classification := model.Classification{
				Transaction:  txn,
				Category:     result.Suggestion.Category,
				Status:       status,
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

		// Increment use counts for check patterns that were used
		for _, pattern := range result.UsedPatterns {
			if err := e.storage.IncrementCheckPatternUseCount(ctx, pattern.ID); err != nil {
				slog.Warn("Failed to increment check pattern use count",
					"pattern_id", pattern.ID,
					"pattern_name", pattern.PatternName,
					"error", err)
			}
		}

		// Update vendor use count if this was a vendor rule
		if isVendorRule {
			// Get existing vendor to update use count
			vendor, err := e.storage.GetVendor(ctx, result.Merchant)
			if err == nil && vendor != nil {
				vendor.UseCount += len(result.Transactions)
				vendor.LastUpdated = time.Now()
				if err := e.storage.SaveVendor(ctx, vendor); err != nil {
					slog.Warn("Failed to update vendor use count", "error", err)
				}
			}
		} else if result.Suggestion.Score >= 0.85 {
			// Save new vendor rule if high confidence
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

	// Keep track of the current category list
	currentCategories := categories

	// Process each merchant group separately
	for _, result := range needsReview {
		if len(result.Transactions) == 0 {
			continue
		}

		// Create pending classifications for all transactions in this merchant group
		pendingClassifications := make([]model.PendingClassification, 0, len(result.Transactions))

		// Get check patterns if this is a check transaction
		var checkPatterns []model.CheckPattern
		if result.Transactions[0].Type == "CHECK" {
			checkPatterns, _ = e.storage.GetMatchingCheckPatterns(ctx, result.Transactions[0])
		}

		// Get category rankings for display
		var categoryRankings model.CategoryRankings
		if result.Suggestion != nil {
			// Build a simple rankings list from the suggestion
			categoryRankings = model.CategoryRankings{
				{
					Category:    result.Suggestion.Category,
					Score:       result.Suggestion.Score,
					IsNew:       result.Suggestion.IsNew,
					Description: result.Suggestion.Description,
				},
			}
		}

		// Create a pending classification for each transaction
		for _, txn := range result.Transactions {
			pending := model.PendingClassification{
				Transaction:      txn,
				SimilarCount:     len(result.Transactions) - 1,
				CategoryRankings: categoryRankings,
				AllCategories:    currentCategories,
				CheckPatterns:    checkPatterns,
			}

			// Add suggestion if available
			if result.Suggestion != nil {
				pending.SuggestedCategory = result.Suggestion.Category
				pending.Confidence = result.Suggestion.Score
				pending.IsNewCategory = result.Suggestion.IsNew
				pending.CategoryDescription = result.Suggestion.Description
			}

			pendingClassifications = append(pendingClassifications, pending)
		}

		// Get batch confirmation from user for this merchant group
		classifications, err := e.prompter.BatchConfirmClassifications(ctx, pendingClassifications)
		if err != nil {
			// Check if this is a context cancellation (user interrupt)
			if errors.Is(err, context.Canceled) {
				return err
			}
			// Log the error but try to continue with the next merchant
			slog.Error("Batch confirmation failed for merchant",
				"merchant", result.Merchant,
				"error", err,
				"transaction_count", len(result.Transactions))
			// Skip this merchant and continue with the next one
			continue
		}

		// Process confirmed classifications
		if len(classifications) > 0 {
			classification := classifications[0] // Use the first classification as template

			// If this is a new category that was accepted, create it
			if result.Suggestion != nil && result.Suggestion.IsNew && classification.Category != "" {
				// Check if category exists first
				_, err := e.storage.GetCategoryByName(ctx, classification.Category)
				if err != nil && errors.Is(err, storage.ErrCategoryNotFound) {
					// Create the new category with the provided description
					_, createErr := e.storage.CreateCategoryWithType(ctx, classification.Category, result.Suggestion.Description, model.CategoryTypeExpense)
					if createErr != nil {
						slog.Error("Failed to create new category",
							"category", classification.Category,
							"error", createErr)
						continue
					}
					slog.Info("Created new category from user confirmation",
						"category", classification.Category,
						"description", result.Suggestion.Description)

					// Refresh the category list after creating a new category
					updatedCategories, refreshErr := e.storage.GetCategories(ctx)
					if refreshErr != nil {
						slog.Warn("Failed to refresh categories after creation",
							"error", refreshErr)
					} else {
						currentCategories = updatedCategories
					}
				}
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
			}

			// Increment use counts for check patterns that were used if the classification matches
			for _, pattern := range result.UsedPatterns {
				if pattern.Category == classification.Category {
					if err := e.storage.IncrementCheckPatternUseCount(ctx, pattern.ID); err != nil {
						slog.Warn("Failed to increment check pattern use count",
							"pattern_id", pattern.ID,
							"pattern_name", pattern.PatternName,
							"error", err)
					}
				}
			}

			// Create vendor rule if user modified a high-confidence suggestion
			if classification.Status == model.StatusUserModified && result.Suggestion != nil && result.Suggestion.Score >= 0.85 {
				vendor := &model.Vendor{
					Name:        result.Merchant,
					Category:    classification.Category,
					UseCount:    len(result.Transactions),
					LastUpdated: time.Now(),
				}
				if err := e.storage.SaveVendor(ctx, vendor); err != nil {
					slog.Warn("Failed to save vendor rule", "error", err)
				}
			}
		}
	}

	return nil
}

// GetDisplay returns a JSON representation of the summary.
func (s *BatchClassificationSummary) GetDisplay() string {
	if s.TotalMerchants == 0 {
		return `{"message":"No transactions to classify"}`
	}

	autoPercent := float64(s.AutoAcceptedCount) / float64(s.TotalMerchants) * 100

	type summaryJSON struct {
		ProcessingTime      string  `json:"processing_time"`
		TotalMerchants      int     `json:"total_merchants"`
		TotalTransactions   int     `json:"total_transactions"`
		AutoAcceptedCount   int     `json:"auto_accepted_count"`
		AutoAcceptedPercent float64 `json:"auto_accepted_percent"`
		AutoAcceptedTxns    int     `json:"auto_accepted_transactions"`
		NeedsReviewCount    int     `json:"needs_review_count"`
		NeedsReviewTxns     int     `json:"needs_review_transactions"`
		FailedCount         int     `json:"failed_count"`
	}

	data := summaryJSON{
		TotalMerchants:      s.TotalMerchants,
		TotalTransactions:   s.TotalTransactions,
		AutoAcceptedCount:   s.AutoAcceptedCount,
		AutoAcceptedPercent: autoPercent,
		AutoAcceptedTxns:    s.AutoAcceptedTxns,
		NeedsReviewCount:    s.NeedsReviewCount,
		NeedsReviewTxns:     s.NeedsReviewTxns,
		FailedCount:         s.FailedCount,
		ProcessingTime:      s.ProcessingTime.Round(time.Second).String(),
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Sprintf(`{"error":"Failed to marshal summary: %v"}`, err)
	}

	return string(bytes)
}
