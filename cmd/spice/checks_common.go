package main

import (
	"fmt"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
)

// formatPatternAmounts formats the amount range for display.
func formatPatternAmounts(pattern model.CheckPattern) string {
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
