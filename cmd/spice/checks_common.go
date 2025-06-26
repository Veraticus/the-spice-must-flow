package main

import (
	"fmt"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
)

// formatPatternAmounts formats the amount range for display.
func formatPatternAmounts(pattern model.CheckPattern) string {
	// Check for multiple specific amounts first
	if len(pattern.Amounts) > 0 {
		result := ""
		for i, amount := range pattern.Amounts {
			if i > 0 {
				result += ", "
			}
			result += fmt.Sprintf("$%.2f", amount)
		}
		return result
	}

	// Fall back to range/exact amount
	if pattern.AmountMin == nil {
		return "N/A"
	}
	if pattern.AmountMax == nil || *pattern.AmountMax == *pattern.AmountMin {
		// Exact amount
		return fmt.Sprintf("$%.2f", *pattern.AmountMin)
	}
	// Range
	return fmt.Sprintf("$%.2f-$%.2f", *pattern.AmountMin, *pattern.AmountMax)
}
