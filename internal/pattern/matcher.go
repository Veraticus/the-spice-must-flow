package pattern

import (
	"context"
	"regexp"
	"strings"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
)

// MatcherImpl implements Matcher for evaluating pattern rules.
type MatcherImpl struct {
	compiledRegex map[int]*regexp.Regexp
	rules         []Rule
}

// NewMatcher creates a new pattern matcher with the given rules.
func NewMatcher(rules []Rule) *MatcherImpl {
	m := &MatcherImpl{
		rules:         rules,
		compiledRegex: make(map[int]*regexp.Regexp),
	}

	// Pre-compile regex patterns
	for _, rule := range rules {
		if rule.IsRegex && rule.MerchantPattern != "" {
			if re, err := regexp.Compile(rule.MerchantPattern); err == nil {
				m.compiledRegex[rule.ID] = re
			}
		}
	}

	return m
}

// Match evaluates a transaction against all configured patterns and returns matching rules.
func (m *MatcherImpl) Match(_ context.Context, txn model.Transaction) ([]Rule, error) {
	var matches []Rule

	for _, rule := range m.rules {
		if !rule.IsActive {
			continue
		}

		if m.matchesRule(txn, rule) {
			matches = append(matches, rule)
		}
	}

	// Sort by priority (higher priority first)
	sortByPriority(matches)

	return matches, nil
}

// matchesRule checks if a transaction matches a specific rule.
func (m *MatcherImpl) matchesRule(txn model.Transaction, rule Rule) bool {
	// Check merchant pattern
	if !m.matchesMerchant(txn, rule) {
		return false
	}

	// Check amount condition
	if !m.matchesAmount(txn, rule) {
		return false
	}

	// Check direction if specified
	if rule.Direction != nil && txn.Direction != *rule.Direction {
		return false
	}

	return true
}

// matchesMerchant checks if the transaction merchant matches the rule pattern.
func (m *MatcherImpl) matchesMerchant(txn model.Transaction, rule Rule) bool {
	if rule.MerchantPattern == "" {
		return true // No merchant pattern means match all
	}

	merchantName := strings.ToLower(txn.MerchantName)
	if merchantName == "" {
		merchantName = strings.ToLower(txn.Name)
	}

	if rule.IsRegex {
		if re, ok := m.compiledRegex[rule.ID]; ok {
			return re.MatchString(merchantName)
		}
		return false
	}

	// Exact match (case-insensitive)
	return strings.ToLower(rule.MerchantPattern) == merchantName
}

// matchesAmount checks if the transaction amount matches the rule condition.
func (m *MatcherImpl) matchesAmount(txn model.Transaction, rule Rule) bool {
	amount := txn.Amount

	switch rule.AmountCondition {
	case "any":
		return true
	case "lt":
		return rule.AmountValue != nil && amount < *rule.AmountValue
	case "le":
		return rule.AmountValue != nil && amount <= *rule.AmountValue
	case "eq":
		return rule.AmountValue != nil && amount == *rule.AmountValue
	case "ge":
		return rule.AmountValue != nil && amount >= *rule.AmountValue
	case "gt":
		return rule.AmountValue != nil && amount > *rule.AmountValue
	case "range":
		if rule.AmountMin != nil && amount < *rule.AmountMin {
			return false
		}
		if rule.AmountMax != nil && amount > *rule.AmountMax {
			return false
		}
		return true
	}

	return false
}

// sortByPriority sorts pattern rules by priority (highest first).
func sortByPriority(rules []Rule) {
	// Simple bubble sort for small arrays
	for i := 0; i < len(rules)-1; i++ {
		for j := 0; j < len(rules)-i-1; j++ {
			if rules[j].Priority < rules[j+1].Priority {
				rules[j], rules[j+1] = rules[j+1], rules[j]
			}
		}
	}
}
