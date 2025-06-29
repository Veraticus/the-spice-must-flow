// Package pattern provides intelligent pattern-based transaction validation and categorization.
package pattern

import (
	"context"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
)

// TransactionValidator validates that transactions have consistent directions based on their categories.
type TransactionValidator interface {
	// ValidateDirection ensures the transaction's direction is consistent with its category type.
	ValidateDirection(ctx context.Context, txn model.Transaction, category model.Category) error
}

// CategorySuggester provides intelligent category suggestions based on patterns and transaction attributes.
type CategorySuggester interface {
	// Suggest returns category suggestions with confidence scores and reasons.
	Suggest(ctx context.Context, txn model.Transaction) ([]Suggestion, error)
	// SuggestWithValidation returns only suggestions that pass direction validation.
	SuggestWithValidation(ctx context.Context, txn model.Transaction, categories []model.Category) ([]Suggestion, error)
}

// Matcher evaluates transactions against pattern rules.
type Matcher interface {
	// Match evaluates a transaction against all configured patterns and returns matching rules.
	Match(ctx context.Context, txn model.Transaction) ([]Rule, error)
}

// Suggestion represents a category suggestion with confidence and reasoning.
type Suggestion struct {
	RuleID     *int
	Category   string
	Reason     string
	Confidence float64
}

// Rule is an alias to the model.PatternRule type for convenience.
type Rule = model.PatternRule
