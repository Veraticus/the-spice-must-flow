package pattern

import (
	"context"
	"fmt"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
)

// Ensure Suggester implements CategorySuggester interface.
var _ CategorySuggester = (*Suggester)(nil)

// Suggester implements CategorySuggester using pattern rules.
type Suggester struct {
	matcher   Matcher
	validator TransactionValidator
}

// NewSuggester creates a new category suggester.
func NewSuggester(matcher Matcher, validator TransactionValidator) *Suggester {
	return &Suggester{
		matcher:   matcher,
		validator: validator,
	}
}

// Suggest returns category suggestions with confidence scores and reasons.
func (s *Suggester) Suggest(ctx context.Context, txn model.Transaction) ([]Suggestion, error) {
	// Get matching pattern rules
	rules, err := s.matcher.Match(ctx, txn)
	if err != nil {
		return nil, fmt.Errorf("failed to match patterns: %w", err)
	}

	// Convert rules to suggestions
	suggestions := make([]Suggestion, 0, len(rules))
	seen := make(map[string]bool) // Avoid duplicate categories

	for _, rule := range rules {
		if seen[rule.DefaultCategory] {
			continue
		}
		seen[rule.DefaultCategory] = true

		suggestion := Suggestion{
			Category:   rule.DefaultCategory,
			Confidence: rule.Confidence,
			Reason:     s.generateReason(txn, rule),
			RuleID:     &rule.ID,
		}
		suggestions = append(suggestions, suggestion)
	}

	return suggestions, nil
}

// generateReason creates a human-readable explanation for why a category was suggested.
func (s *Suggester) generateReason(txn model.Transaction, rule Rule) string {
	merchant := txn.MerchantName
	if merchant == "" {
		merchant = txn.Name
	}

	// Build reason based on rule conditions
	reason := fmt.Sprintf("Transactions from %s", merchant)

	// Add amount condition if present
	switch rule.AmountCondition {
	case "lt":
		if rule.AmountValue != nil {
			reason += fmt.Sprintf(" under $%.2f", *rule.AmountValue)
		}
	case "gt":
		if rule.AmountValue != nil {
			reason += fmt.Sprintf(" over $%.2f", *rule.AmountValue)
		}
	case "range":
		if rule.AmountMin != nil && rule.AmountMax != nil {
			reason += fmt.Sprintf(" between $%.2f and $%.2f", *rule.AmountMin, *rule.AmountMax)
		}
	}

	// Add direction if specified
	if rule.Direction != nil {
		switch *rule.Direction {
		case model.DirectionIncome:
			reason += " (income)"
		case model.DirectionExpense:
			reason += " (expense)"
		}
	}

	reason += fmt.Sprintf(" are usually categorized as %s", rule.DefaultCategory)

	return reason
}

// SuggestWithValidation returns only suggestions that pass direction validation.
func (s *Suggester) SuggestWithValidation(ctx context.Context, txn model.Transaction, categories []model.Category) ([]Suggestion, error) {
	// Get all suggestions
	suggestions, err := s.Suggest(ctx, txn)
	if err != nil {
		return nil, err
	}

	// Create category map for validation
	categoryMap := make(map[string]model.Category)
	for _, cat := range categories {
		categoryMap[cat.Name] = cat
	}

	// Filter suggestions that pass validation
	var validSuggestions []Suggestion
	for _, suggestion := range suggestions {
		cat, exists := categoryMap[suggestion.Category]
		if !exists {
			continue // Skip unknown categories
		}

		// Check if the category is valid for this transaction's direction
		if err := s.validator.ValidateDirection(ctx, txn, cat); err == nil {
			validSuggestions = append(validSuggestions, suggestion)
		}
	}

	return validSuggestions, nil
}
