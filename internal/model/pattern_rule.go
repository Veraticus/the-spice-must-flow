// Package model defines the core data structures for the spice application.
package model

import (
	"time"
)

// PatternRule represents a rule for matching transactions and suggesting categories.
type PatternRule struct {
	CreatedAt       time.Time             `json:"created_at"`
	UpdatedAt       time.Time             `json:"updated_at"`
	AmountValue     *float64              `json:"amount_value,omitempty"`
	AmountMin       *float64              `json:"amount_min,omitempty"`
	AmountMax       *float64              `json:"amount_max,omitempty"`
	Direction       *TransactionDirection `json:"direction,omitempty"`
	Name            string                `json:"name"`
	Description     string                `json:"description"`
	MerchantPattern string                `json:"merchant_pattern"`
	AmountCondition string                `json:"amount_condition"`
	DefaultCategory string                `json:"default_category"`
	Priority        int                   `json:"priority"`
	ID              int                   `json:"id"`
	Confidence      float64               `json:"confidence"`
	UseCount        int                   `json:"use_count"`
	IsActive        bool                  `json:"is_active"`
	IsRegex         bool                  `json:"is_regex"`
}

// AmountConditionType represents the type of amount comparison.
type AmountConditionType string

// Amount condition constants.
const (
	AmountLessThan     AmountConditionType = "lt"
	AmountLessEqual    AmountConditionType = "le"
	AmountEqual        AmountConditionType = "eq"
	AmountGreaterEqual AmountConditionType = "ge"
	AmountGreaterThan  AmountConditionType = "gt"
	AmountRange        AmountConditionType = "range"
	AmountAny          AmountConditionType = "any"
)
