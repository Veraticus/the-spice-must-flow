package engine

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/pattern"
	"github.com/Veraticus/the-spice-must-flow/internal/service"
)

// PatternClassifier wraps pattern-based classification functionality.
type PatternClassifier struct {
	storage   service.Storage
	matcher   pattern.Matcher
	suggester pattern.CategorySuggester
	validator pattern.TransactionValidator
}

// NewPatternClassifier creates a new pattern-based classifier.
func NewPatternClassifier(storage service.Storage) (*PatternClassifier, error) {
	// Get active pattern rules from storage
	rules, err := storage.GetActivePatternRules(context.Background())
	if err != nil {
		return nil, err
	}

	// Convert storage rules to pattern rules
	patternRules := make([]pattern.Rule, len(rules))
	copy(patternRules, rules)

	// Create pattern components
	matcher := pattern.NewMatcher(patternRules)
	validator := pattern.NewValidator()
	suggester := pattern.NewSuggester(matcher, validator)

	return &PatternClassifier{
		storage:   storage,
		matcher:   matcher,
		suggester: suggester,
		validator: validator,
	}, nil
}

// ClassifyWithPatterns attempts to classify transactions using pattern rules.
// Returns nil if no matching patterns are found.
func (pc *PatternClassifier) ClassifyWithPatterns(ctx context.Context, txns []model.Transaction) (*model.CategoryRanking, error) {
	if len(txns) == 0 {
		return nil, fmt.Errorf("no transactions provided")
	}

	// Use the first transaction as representative
	// (assuming all transactions in the group are from the same merchant)
	representativeTxn := txns[0]

	// Get categories for validation
	categories, err := pc.storage.GetCategories(ctx)
	if err != nil {
		return nil, err
	}

	// Get suggestions with validation
	suggestions, err := pc.suggester.SuggestWithValidation(ctx, representativeTxn, categories)
	if err != nil {
		return nil, err
	}

	if len(suggestions) == 0 {
		// No matching patterns found
		return nil, nil //nolint:nilnil // Expected behavior when no patterns match
	}

	// Use the highest confidence suggestion
	topSuggestion := suggestions[0]

	// Log pattern match
	slog.Info("pattern rule matched",
		"merchant", representativeTxn.MerchantName,
		"category", topSuggestion.Category,
		"confidence", topSuggestion.Confidence,
		"reason", topSuggestion.Reason,
		"rule_id", topSuggestion.RuleID)

	// Increment use count if we have a rule ID
	if topSuggestion.RuleID != nil {
		if err := pc.storage.IncrementPatternRuleUseCount(ctx, *topSuggestion.RuleID); err != nil {
			slog.Warn("failed to increment pattern rule use count",
				"rule_id", *topSuggestion.RuleID,
				"error", err)
		}
	}

	return &model.CategoryRanking{
		Category:    topSuggestion.Category,
		Score:       topSuggestion.Confidence,
		IsNew:       false,
		Description: topSuggestion.Reason,
	}, nil
}

// RefreshPatterns reloads pattern rules from storage.
func (pc *PatternClassifier) RefreshPatterns(ctx context.Context) error {
	// Get active pattern rules from storage
	rules, err := pc.storage.GetActivePatternRules(ctx)
	if err != nil {
		return err
	}

	// Convert storage rules to pattern rules
	patternRules := make([]pattern.Rule, len(rules))
	copy(patternRules, rules)

	// Create new matcher with updated rules
	pc.matcher = pattern.NewMatcher(patternRules)
	pc.suggester = pattern.NewSuggester(pc.matcher, pc.validator)

	slog.Info("refreshed pattern rules", "count", len(patternRules))
	return nil
}
