package pattern

import (
	"context"
	"fmt"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
)

// Validator implements TransactionValidator for direction consistency checks.
type Validator struct{}

// NewValidator creates a new transaction validator.
func NewValidator() *Validator {
	return &Validator{}
}

// ValidateDirection ensures the transaction's direction is consistent with its category type.
func (v *Validator) ValidateDirection(_ context.Context, txn model.Transaction, category model.Category) error {
	// System categories (transfers) can be used with any direction
	if category.Type == model.CategoryTypeSystem {
		return nil
	}

	// Map transaction direction to expected category type
	var expectedType model.CategoryType
	switch txn.Direction {
	case model.DirectionIncome:
		expectedType = model.CategoryTypeIncome
	case model.DirectionExpense:
		expectedType = model.CategoryTypeExpense
	case model.DirectionTransfer:
		expectedType = model.CategoryTypeSystem
	default:
		// If direction is not set, infer from amount (backward compatibility)
		if txn.Amount < 0 {
			expectedType = model.CategoryTypeIncome
		} else {
			expectedType = model.CategoryTypeExpense
		}
	}

	// Validate category type matches expected type
	if category.Type != expectedType {
		return fmt.Errorf("category %q has type %s but transaction has direction %s",
			category.Name, category.Type, txn.Direction)
	}

	return nil
}

// ValidateSuggestions ensures all suggestions have valid categories for the transaction direction.
func (v *Validator) ValidateSuggestions(ctx context.Context, txn model.Transaction, suggestions []Suggestion, categories []model.Category) error {
	// Create category map for quick lookup
	categoryMap := make(map[string]model.Category)
	for _, cat := range categories {
		categoryMap[cat.Name] = cat
	}

	// Validate each suggestion
	for _, suggestion := range suggestions {
		cat, exists := categoryMap[suggestion.Category]
		if !exists {
			return fmt.Errorf("suggestion references unknown category %q", suggestion.Category)
		}

		if err := v.ValidateDirection(ctx, txn, cat); err != nil {
			return fmt.Errorf("invalid suggestion: %w", err)
		}
	}

	return nil
}
