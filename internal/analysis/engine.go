package analysis

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
)

// Analyze performs AI-powered transaction analysis.
func (e *Engine) Analyze(ctx context.Context, opts Options) (*Report, error) {
	// Initialize progress tracking
	progress := opts.ProgressFunc
	if progress == nil {
		progress = func(string, int) {} // no-op
	}

	// Step 1: Create or continue session
	progress("Initializing session", 5)
	session, err := e.createOrContinueSession(ctx, opts.SessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Update session status
	session.Status = StatusInProgress
	session.LastAttempt = time.Now()
	if updateErr := e.deps.SessionStore.Update(ctx, session); updateErr != nil {
		return nil, fmt.Errorf("failed to update session: %w", updateErr)
	}

	// Step 2: Load data
	progress("Loading transactions", 10)
	transactions, err := e.loadTransactions(ctx, opts.StartDate, opts.EndDate)
	if err != nil {
		return nil, e.failSession(ctx, session, fmt.Errorf("failed to load transactions: %w", err))
	}

	progress("Loading categories", 15)
	categories, err := e.loadCategories(ctx)
	if err != nil {
		return nil, e.failSession(ctx, session, fmt.Errorf("failed to load categories: %w", err))
	}

	progress("Loading patterns", 20)
	patterns, err := e.loadPatterns(ctx)
	if err != nil {
		return nil, e.failSession(ctx, session, fmt.Errorf("failed to load patterns: %w", err))
	}

	checkPatterns, err := e.loadCheckPatterns(ctx)
	if err != nil {
		return nil, e.failSession(ctx, session, fmt.Errorf("failed to load check patterns: %w", err))
	}

	// Step 3: Analyze vendors for context
	progress("Analyzing vendor patterns", 25)
	vendorAnalysis, err := e.analyzeVendors(ctx, transactions)
	if err != nil {
		slog.Warn("Failed to analyze vendors", "error", err)
		vendorAnalysis = []RecentVendor{} // Continue without vendor analysis
	}

	// Step 4: Build prompt data
	progress("Preparing analysis", 30)

	// Always use file-based analysis
	fileBasedData := &FileBasedPromptData{
		TransactionCount:   len(transactions),
		UseFileBasedPrompt: true,
		// FilePath will be set by the LLM adapter when it creates the temp file
	}
	slog.Info("Using file-based analysis",
		"transaction_count", len(transactions),
	)

	promptData := PromptData{
		DateRange: DateRange{
			Start: opts.StartDate,
			End:   opts.EndDate,
		},
		Transactions:    nil, // Never include transactions in prompt
		Categories:      categories,
		Patterns:        patterns,
		CheckPatterns:   checkPatterns,
		RecentVendors:   vendorAnalysis,
		AnalysisOptions: opts,
		TotalCount:      len(transactions),
		SampleSize:      0,
		FileBasedData:   fileBasedData,
	}

	// Step 5: Perform analysis with validation recovery
	progress("Running AI analysis", 40)
	report, err := e.performAnalysisWithRecovery(ctx, session, promptData, transactions, progress)
	if err != nil {
		return nil, e.failSession(ctx, session, fmt.Errorf("analysis failed: %w", err))
	}

	// Step 6: Save report
	progress("Saving report", 80)
	reportID := uuid.New().String()
	report.ID = reportID
	report.SessionID = session.ID
	report.GeneratedAt = time.Now()
	report.PeriodStart = opts.StartDate
	report.PeriodEnd = opts.EndDate

	if err := e.deps.ReportStore.SaveReport(ctx, report); err != nil {
		return nil, e.failSession(ctx, session, fmt.Errorf("failed to save report: %w", err))
	}

	// Step 7: Update session as completed
	now := time.Now()
	session.Status = StatusCompleted
	session.CompletedAt = &now
	session.ReportID = &reportID
	if err := e.deps.SessionStore.Update(ctx, session); err != nil {
		slog.Warn("Failed to update session status", "error", err)
		// Continue, as analysis is complete
	}

	// Step 8: Apply fixes if requested
	if opts.AutoApply && !opts.DryRun {
		progress("Applying recommended fixes", 90)
		if err := e.applyFixes(ctx, report); err != nil {
			slog.Warn("Failed to apply some fixes", "error", err)
			// Continue, as analysis is complete
		}
	}

	progress("Analysis complete", 100)
	return report, nil
}

// performAnalysisWithRecovery performs the LLM analysis with validation recovery loop.
func (e *Engine) performAnalysisWithRecovery(ctx context.Context, session *Session, promptData PromptData, allTransactions []model.Transaction, progress ProgressCallback) (*Report, error) {
	const maxAttempts = 3

	// Build the initial prompt
	prompt, err := e.deps.PromptBuilder.BuildAnalysisPrompt(promptData)
	if err != nil {
		return nil, fmt.Errorf("failed to build analysis prompt: %w", err)
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Update session attempt count
		session.Attempts = attempt
		session.Status = StatusValidating
		if updateErr := e.deps.SessionStore.Update(ctx, session); updateErr != nil {
			slog.Warn("Failed to update session attempt", "error", updateErr)
		}

		progress(fmt.Sprintf("Analysis attempt %d/%d", attempt, maxAttempts), 40+attempt*10)

		// Always use file-based analysis
		var responseJSON string
		// Prepare transaction data for file-based analysis
		transactionData := make(map[string]interface{})
		transactionArray := make([]map[string]interface{}, len(allTransactions))

		for i, txn := range allTransactions {
			// Extract the first category from the slice for LLM analysis
			var category string
			if len(txn.Category) > 0 {
				category = txn.Category[0]
			}

			transactionArray[i] = map[string]interface{}{
				"ID":       txn.ID,
				"Date":     txn.Date.Format("2006-01-02"),
				"Name":     txn.Name,
				"Amount":   txn.Amount,
				"Type":     txn.Type,
				"Category": category,
			}
		}
		transactionData["transactions"] = transactionArray

		responseJSON, err = e.deps.LLMClient.AnalyzeTransactionsWithFile(ctx, prompt, transactionData)
		if err != nil {
			lastErr = fmt.Errorf("LLM request failed (attempt %d): %w", attempt, err)
			slog.Warn("Analysis request failed", "attempt", attempt, "error", err)
			continue
		}

		// Validate response
		report, validationErr := e.deps.Validator.Validate([]byte(responseJSON))
		if validationErr == nil {
			// Success!
			return report, nil
		}

		lastErr = fmt.Errorf("validation failed (attempt %d): %w", attempt, validationErr)
		slog.Warn("Response validation failed", "attempt", attempt, "error", validationErr)

		// If this isn't the last attempt, try to correct
		if attempt < maxAttempts {
			progress(fmt.Sprintf("Correcting response (attempt %d)", attempt), 50+attempt*5)

			// Extract error details
			section, line, col := e.deps.Validator.ExtractError([]byte(responseJSON), validationErr)

			// Build correction prompt
			correctionData := CorrectionPromptData{
				OriginalPrompt:  prompt,
				InvalidResponse: responseJSON,
				ErrorSection:    section,
				LineNumber:      line,
				ColumnNumber:    col,
				ErrorDetails:    validationErr.Error(),
			}

			correctionPrompt, err := e.deps.PromptBuilder.BuildCorrectionPrompt(correctionData)
			if err != nil {
				lastErr = fmt.Errorf("failed to build correction prompt: %w", err)
				continue
			}

			// Use the correction prompt for the next attempt
			prompt = correctionPrompt
		}
	}

	// All attempts failed
	return nil, fmt.Errorf("analysis failed after %d attempts: %w", maxAttempts, lastErr)
}

// createOrContinueSession creates a new session or continues an existing one.
func (e *Engine) createOrContinueSession(ctx context.Context, sessionID string) (*Session, error) {
	if sessionID != "" {
		// Try to continue existing session
		session, err := e.deps.SessionStore.Get(ctx, sessionID)
		if err == nil && session.Status != StatusCompleted && session.Status != StatusFailed {
			slog.Info("Continuing existing session", "id", sessionID)
			return session, nil
		}
	}

	// Create new session
	newID := uuid.New().String()
	session := &Session{
		ID:          newID,
		Status:      StatusPending,
		StartedAt:   time.Now(),
		LastAttempt: time.Now(),
		Attempts:    0,
	}

	if err := e.deps.SessionStore.Create(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	slog.Info("Created new analysis session", "id", newID)
	return session, nil
}

// failSession updates the session status to failed and returns the error.
func (e *Engine) failSession(ctx context.Context, session *Session, err error) error {
	session.Status = StatusFailed
	errStr := err.Error()
	session.Error = &errStr

	if updateErr := e.deps.SessionStore.Update(ctx, session); updateErr != nil {
		slog.Warn("Failed to update session status", "error", updateErr)
	}

	return err
}

// loadTransactions loads transactions within the specified date range.
func (e *Engine) loadTransactions(ctx context.Context, startDate, endDate time.Time) ([]model.Transaction, error) {
	// Get classifications in date range to extract transactions
	classifications, err := e.deps.Storage.GetClassificationsByDateRange(ctx, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to query classifications: %w", err)
	}

	// Extract unique transactions from classifications
	txnMap := make(map[string]model.Transaction)
	for _, c := range classifications {
		txn := c.Transaction
		// Use the classification category instead of the original transaction categories
		if c.Category != "" {
			txn.Category = []string{c.Category}
		}
		txnMap[c.Transaction.ID] = txn
	}

	// Convert to slice
	transactions := make([]model.Transaction, 0, len(txnMap))
	for _, txn := range txnMap {
		transactions = append(transactions, txn)
	}

	return transactions, nil
}

// loadCategories loads all available categories.
func (e *Engine) loadCategories(ctx context.Context) ([]model.Category, error) {
	categories, err := e.deps.Storage.GetCategories(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load categories: %w", err)
	}

	return categories, nil
}

// loadPatterns loads all pattern rules.
func (e *Engine) loadPatterns(ctx context.Context) ([]model.PatternRule, error) {
	patterns, err := e.deps.Storage.GetActivePatternRules(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load patterns: %w", err)
	}

	return patterns, nil
}

// loadCheckPatterns loads all check patterns.
func (e *Engine) loadCheckPatterns(ctx context.Context) ([]model.CheckPattern, error) {
	patterns, err := e.deps.Storage.GetActiveCheckPatterns(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load check patterns: %w", err)
	}

	return patterns, nil
}

// analyzeVendors analyzes transaction vendors to provide context.
func (e *Engine) analyzeVendors(_ context.Context, transactions []model.Transaction) ([]RecentVendor, error) {
	vendorCounts := make(map[string]map[string]int) // vendor -> category -> count

	for _, txn := range transactions {
		if txn.MerchantName != "" && len(txn.Category) > 0 {
			if vendorCounts[txn.MerchantName] == nil {
				vendorCounts[txn.MerchantName] = make(map[string]int)
			}
			vendorCounts[txn.MerchantName][txn.Category[0]]++
		}
	}

	// Convert to RecentVendor list
	vendors := make([]RecentVendor, 0, len(vendorCounts))
	for vendor, categories := range vendorCounts {
		// Find most common category for this vendor
		var topCategory string
		var topCount int
		for category, count := range categories {
			if count > topCount {
				topCategory = category
				topCount = count
			}
		}

		vendors = append(vendors, RecentVendor{
			Name:        vendor,
			Category:    topCategory,
			Occurrences: topCount,
		})
	}

	// Limit to top 20 vendors
	if len(vendors) > 20 {
		vendors = vendors[:20]
	}

	return vendors, nil
}

// applyFixes applies recommended fixes from the report.
func (e *Engine) applyFixes(ctx context.Context, report *Report) error {
	var errs []error

	// Apply pattern fixes
	if len(report.SuggestedPatterns) > 0 {
		if err := e.deps.FixApplier.ApplyPatternFixes(ctx, report.SuggestedPatterns); err != nil {
			errs = append(errs, fmt.Errorf("pattern fixes: %w", err))
		}
	}

	// Apply category fixes
	var categoryFixes []Fix
	for _, issue := range report.Issues {
		if issue.Fix != nil && issue.Fix.Type == "update_category" {
			categoryFixes = append(categoryFixes, *issue.Fix)
		}
	}
	if len(categoryFixes) > 0 {
		if err := e.deps.FixApplier.ApplyCategoryFixes(ctx, categoryFixes); err != nil {
			errs = append(errs, fmt.Errorf("category fixes: %w", err))
		}
	}

	// Apply recategorizations
	var recategorizations []Issue
	for _, issue := range report.Issues {
		if issue.Type == IssueTypeMiscategorized && issue.Fix != nil {
			recategorizations = append(recategorizations, issue)
		}
	}
	if len(recategorizations) > 0 {
		if err := e.deps.FixApplier.ApplyRecategorizations(ctx, recategorizations); err != nil {
			errs = append(errs, fmt.Errorf("recategorizations: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to apply %d fixes", len(errs))
	}

	return nil
}
