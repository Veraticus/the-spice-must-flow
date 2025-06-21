package model

import (
	"encoding/json"
	"fmt"
	"time"
)

// CheckPattern represents a pattern for automatically categorizing check transactions.
type CheckPattern struct {
	CreatedAt          time.Time
	UpdatedAt          time.Time
	DayOfMonthMax      *int
	AmountMax          *float64
	CheckNumberPattern *CheckNumberMatcher
	DayOfMonthMin      *int
	AmountMin          *float64
	Category           string
	Notes              string
	PatternName        string
	ID                 int64
	ConfidenceBoost    float64
	UseCount           int
	Active             bool
}

// CheckNumberMatcher represents complex check number matching patterns.
type CheckNumberMatcher struct {
	Modulo int `json:"modulo,omitempty"` // e.g., check number % 10 == offset
	Offset int `json:"offset,omitempty"`
}

// MarshalJSON handles JSON serialization for CheckNumberPattern field.
func (p *CheckPattern) MarshalJSON() ([]byte, error) {
	type Alias CheckPattern

	// Convert CheckNumberMatcher to JSON string for database storage
	var checkNumberJSON *string
	if p.CheckNumberPattern != nil {
		data, err := json.Marshal(p.CheckNumberPattern)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal check number pattern: %w", err)
		}
		str := string(data)
		checkNumberJSON = &str
	}

	return json.Marshal(&struct {
		CheckNumberPattern *string `json:"check_number_pattern,omitempty"`
		*Alias
	}{
		CheckNumberPattern: checkNumberJSON,
		Alias:              (*Alias)(p),
	})
}

// Matches determines if a transaction matches this pattern.
func (p *CheckPattern) Matches(txn Transaction) bool {
	if !p.Active {
		return false
	}

	// Check transaction type
	if txn.Type != "CHECK" {
		return false
	}

	// Check amount range
	if p.AmountMin != nil && txn.Amount < *p.AmountMin {
		return false
	}
	if p.AmountMax != nil && txn.Amount > *p.AmountMax {
		return false
	}

	// Check day of month
	if p.DayOfMonthMin != nil || p.DayOfMonthMax != nil {
		day := txn.Date.Day()
		if p.DayOfMonthMin != nil && day < *p.DayOfMonthMin {
			return false
		}
		if p.DayOfMonthMax != nil && day > *p.DayOfMonthMax {
			return false
		}
	}

	// Check number pattern matching (if implemented)
	// This would require parsing check number from transaction name
	// For now, we'll skip this check

	return true
}

// Validate ensures the pattern has valid data.
func (p *CheckPattern) Validate() error {
	if p.PatternName == "" {
		return fmt.Errorf("pattern name is required")
	}

	if p.Category == "" {
		return fmt.Errorf("category is required")
	}

	// Validate amount range
	if p.AmountMin != nil && p.AmountMax != nil && *p.AmountMin > *p.AmountMax {
		return fmt.Errorf("amount min must be less than or equal to amount max")
	}

	// Validate day of month range
	if p.DayOfMonthMin != nil && (*p.DayOfMonthMin < 1 || *p.DayOfMonthMin > 31) {
		return fmt.Errorf("day of month min must be between 1 and 31")
	}
	if p.DayOfMonthMax != nil && (*p.DayOfMonthMax < 1 || *p.DayOfMonthMax > 31) {
		return fmt.Errorf("day of month max must be between 1 and 31")
	}
	if p.DayOfMonthMin != nil && p.DayOfMonthMax != nil && *p.DayOfMonthMin > *p.DayOfMonthMax {
		return fmt.Errorf("day of month min must be less than or equal to day of month max")
	}

	// Validate confidence boost
	if p.ConfidenceBoost < 0 || p.ConfidenceBoost > 1 {
		return fmt.Errorf("confidence boost must be between 0 and 1")
	}

	return nil
}
