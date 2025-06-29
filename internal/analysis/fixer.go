package analysis

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/engine"
	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/service"
)

// TransactionalFixApplier implements FixApplier with transaction safety.
type TransactionalFixApplier struct {
	storage       service.Storage
	patternEngine *engine.PatternClassifier
}

// NewTransactionalFixApplier creates a new fix applier with transaction support.
func NewTransactionalFixApplier(storage service.Storage, patternEngine *engine.PatternClassifier) *TransactionalFixApplier {
	return &TransactionalFixApplier{
		storage:       storage,
		patternEngine: patternEngine,
	}
}

// ApplyPatternFixes creates or updates pattern rules based on suggestions.
func (f *TransactionalFixApplier) ApplyPatternFixes(ctx context.Context, patterns []SuggestedPattern) error {
	if len(patterns) == 0 {
		return nil
	}

	tx, err := f.storage.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Apply each pattern within the transaction
	for _, sp := range patterns {
		// Validate pattern rule fields
		if err := f.validatePatternRule(&sp.Pattern); err != nil {
			return fmt.Errorf("invalid pattern %q: %w", sp.Name, err)
		}

		// Check if category exists
		categories, err := tx.GetCategories(ctx)
		if err != nil {
			return fmt.Errorf("failed to get categories: %w", err)
		}

		categoryExists := false
		for _, cat := range categories {
			if cat.Name == sp.Pattern.DefaultCategory && cat.IsActive {
				categoryExists = true
				break
			}
		}

		if !categoryExists {
			return fmt.Errorf("category %q does not exist or is inactive", sp.Pattern.DefaultCategory)
		}

		// Create the pattern rule
		if err := tx.CreatePatternRule(ctx, &sp.Pattern); err != nil {
			return fmt.Errorf("failed to create pattern rule %q: %w", sp.Name, err)
		}

		slog.Info("created pattern rule",
			"name", sp.Name,
			"category", sp.Pattern.DefaultCategory,
			"confidence", sp.Confidence)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit pattern fixes: %w", err)
	}

	// Refresh the pattern engine with new rules
	if f.patternEngine != nil {
		if err := f.patternEngine.RefreshPatterns(ctx); err != nil {
			slog.Warn("failed to refresh pattern engine", "error", err)
		}
	}

	return nil
}

// ApplyCategoryFixes updates transaction categories based on fixes.
func (f *TransactionalFixApplier) ApplyCategoryFixes(ctx context.Context, fixes []Fix) error {
	if len(fixes) == 0 {
		return nil
	}

	tx, err := f.storage.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Get all categories for validation
	categories, err := tx.GetCategories(ctx)
	if err != nil {
		return fmt.Errorf("failed to get categories: %w", err)
	}

	categoryMap := make(map[string]model.Category)
	for _, cat := range categories {
		categoryMap[cat.Name] = cat
	}

	// Apply each fix within the transaction
	for _, fix := range fixes {
		// Extract category from fix data
		newCategory, ok := fix.Data["category"].(string)
		if !ok {
			return fmt.Errorf("fix %s missing category data", fix.ID)
		}

		// Validate category exists and is active
		cat, exists := categoryMap[newCategory]
		if !exists || !cat.IsActive {
			return fmt.Errorf("category %q does not exist or is inactive", newCategory)
		}

		// Extract transaction IDs from fix data
		txnIDsRaw, ok := fix.Data["transaction_ids"].([]any)
		if !ok {
			return fmt.Errorf("fix %s missing transaction_ids data", fix.ID)
		}

		var txnIDs []string
		for _, id := range txnIDsRaw {
			idStr, ok := id.(string)
			if !ok {
				return fmt.Errorf("invalid transaction ID type in fix %s", fix.ID)
			}
			txnIDs = append(txnIDs, idStr)
		}

		// Update each transaction's classification
		for _, txnID := range txnIDs {
			// Get the transaction
			txn, err := tx.GetTransactionByID(ctx, txnID)
			if err != nil {
				return fmt.Errorf("failed to get transaction %s: %w", txnID, err)
			}

			// Create updated classification
			classification := &model.Classification{
				Transaction:     *txn,
				Category:        newCategory,
				Status:          model.StatusUserModified,
				Confidence:      1.0,
				ClassifiedAt:    time.Now(),
				Notes:           fmt.Sprintf("Applied fix %s", fix.ID),
				BusinessPercent: 0,
			}

			if err := tx.SaveClassification(ctx, classification); err != nil {
				return fmt.Errorf("failed to save classification for transaction %s: %w", txnID, err)
			}
		}

		slog.Info("applied category fix",
			"fix_id", fix.ID,
			"category", newCategory,
			"transactions", len(txnIDs))
	}

	return tx.Commit()
}

// ApplyRecategorizations moves transactions to their suggested categories.
func (f *TransactionalFixApplier) ApplyRecategorizations(ctx context.Context, issues []Issue) error {
	if len(issues) == 0 {
		return nil
	}

	tx, err := f.storage.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Get all categories for validation
	categories, err := tx.GetCategories(ctx)
	if err != nil {
		return fmt.Errorf("failed to get categories: %w", err)
	}

	categoryMap := make(map[string]model.Category)
	for _, cat := range categories {
		categoryMap[cat.Name] = cat
	}

	// Process each issue
	recategorizedCount := 0
	for _, issue := range issues {
		// Only process miscategorized issues with suggestions
		if issue.Type != IssueTypeMiscategorized || issue.SuggestedCategory == nil {
			continue
		}

		// Validate suggested category exists and is active
		cat, exists := categoryMap[*issue.SuggestedCategory]
		if !exists || !cat.IsActive {
			slog.Warn("skipping recategorization to invalid category",
				"category", *issue.SuggestedCategory,
				"issue_id", issue.ID)
			continue
		}

		// Update each transaction
		for _, txnID := range issue.TransactionIDs {
			// Get the transaction
			txn, err := tx.GetTransactionByID(ctx, txnID)
			if err != nil {
				return fmt.Errorf("failed to get transaction %s: %w", txnID, err)
			}

			// Create updated classification
			classification := &model.Classification{
				Transaction:     *txn,
				Category:        *issue.SuggestedCategory,
				Status:          model.StatusUserModified,
				Confidence:      issue.Confidence,
				ClassifiedAt:    time.Now(),
				Notes:           fmt.Sprintf("Recategorized from %s (issue: %s)", valueOrDefault(issue.CurrentCategory, "unclassified"), issue.ID),
				BusinessPercent: 0,
			}

			if err := tx.SaveClassification(ctx, classification); err != nil {
				return fmt.Errorf("failed to save classification for transaction %s: %w", txnID, err)
			}

			recategorizedCount++
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit recategorizations: %w", err)
	}

	slog.Info("completed recategorizations",
		"issues_processed", len(issues),
		"transactions_updated", recategorizedCount)

	return nil
}

// PreviewFix shows what would change if a fix is applied without making changes.
func (f *TransactionalFixApplier) PreviewFix(ctx context.Context, fix Fix) (*FixPreview, error) {
	preview := &FixPreview{
		FixID:           fix.ID,
		Changes:         []PreviewChange{},
		EstimatedImpact: make(map[string]float64),
	}

	// Handle different fix types
	switch fix.Type {
	case "category_update":
		return f.previewCategoryFix(ctx, fix, preview)
	case "pattern_creation":
		return f.previewPatternFix(ctx, fix, preview)
	default:
		return nil, fmt.Errorf("unknown fix type: %s", fix.Type)
	}
}

func (f *TransactionalFixApplier) previewCategoryFix(ctx context.Context, fix Fix, preview *FixPreview) (*FixPreview, error) {
	// Extract new category
	newCategory, ok := fix.Data["category"].(string)
	if !ok {
		return nil, fmt.Errorf("fix missing category data")
	}

	// Extract transaction IDs
	txnIDsRaw, ok := fix.Data["transaction_ids"].([]any)
	if !ok {
		return nil, fmt.Errorf("fix missing transaction_ids data")
	}

	var totalAmount float64
	for _, idRaw := range txnIDsRaw {
		txnID, ok := idRaw.(string)
		if !ok {
			continue
		}

		// Get current transaction and classification
		txn, err := f.storage.GetTransactionByID(ctx, txnID)
		if err != nil {
			return nil, fmt.Errorf("failed to get transaction %s: %w", txnID, err)
		}

		// For preview, we'll show unclassified as the old value
		// since we don't have a direct method to get classification by transaction ID
		oldCategory := "unclassified"

		preview.Changes = append(preview.Changes, PreviewChange{
			TransactionID: txnID,
			FieldName:     "category",
			OldValue:      oldCategory,
			NewValue:      newCategory,
		})

		totalAmount += txn.Amount
	}

	preview.AffectedCount = len(preview.Changes)
	preview.EstimatedImpact["total_amount"] = totalAmount
	preview.EstimatedImpact["transaction_count"] = float64(preview.AffectedCount)

	return preview, nil
}

func (f *TransactionalFixApplier) previewPatternFix(_ context.Context, fix Fix, preview *FixPreview) (*FixPreview, error) {
	// Extract pattern data
	patternName, ok := fix.Data["pattern_name"].(string)
	if !ok {
		return nil, fmt.Errorf("fix missing pattern_name data")
	}

	matchCount, ok := fix.Data["match_count"].(float64)
	if !ok {
		matchCount = 0
	}

	preview.Changes = append(preview.Changes, PreviewChange{
		TransactionID: "",
		FieldName:     "pattern_rule",
		OldValue:      "none",
		NewValue:      patternName,
	})

	preview.AffectedCount = int(matchCount)
	preview.EstimatedImpact["future_matches"] = matchCount

	return preview, nil
}

// validatePatternRule ensures a pattern rule has valid fields.
func (f *TransactionalFixApplier) validatePatternRule(rule *model.PatternRule) error {
	if rule.Name == "" {
		return fmt.Errorf("pattern name is required")
	}
	if rule.MerchantPattern == "" {
		return fmt.Errorf("merchant pattern is required")
	}
	if rule.DefaultCategory == "" {
		return fmt.Errorf("default category is required")
	}
	if rule.Confidence < 0 || rule.Confidence > 1 {
		return fmt.Errorf("confidence must be between 0 and 1")
	}
	if rule.Priority < 0 {
		return fmt.Errorf("priority must be non-negative")
	}

	// Set defaults for new patterns
	if rule.CreatedAt.IsZero() {
		rule.CreatedAt = time.Now()
	}
	if rule.UpdatedAt.IsZero() {
		rule.UpdatedAt = time.Now()
	}
	if !rule.IsActive {
		rule.IsActive = true
	}

	return nil
}

// valueOrDefault returns the value if not nil, otherwise returns the default.
func valueOrDefault(value *string, defaultValue string) string {
	if value == nil {
		return defaultValue
	}
	return *value
}
