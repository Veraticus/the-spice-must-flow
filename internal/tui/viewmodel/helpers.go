package viewmodel

import (
	"fmt"
	"strings"
	"time"
)

// String returns a string representation of the classifier mode.
func (m ClassifierMode) String() string {
	switch m {
	case ModeSelectingSuggestion:
		return "SelectingSuggestion"
	case ModeEnteringCustom:
		return "EnteringCustom"
	case ModeSelectingCategory:
		return "SelectingCategory"
	case ModeConfirming:
		return "Confirming"
	default:
		return fmt.Sprintf("Unknown(%d)", m)
	}
}

// String returns a string representation of the app state.
func (s AppState) String() string {
	switch s {
	case StateLoading:
		return "Loading"
	case StateClassifying:
		return "Classifying"
	case StateStats:
		return "Stats"
	case StateError:
		return "Error"
	case StateWaiting:
		return "Waiting"
	default:
		return fmt.Sprintf("Unknown(%d)", s)
	}
}

// GetVisibleCategories returns categories that should be displayed based on current offset.
func (v ClassifierView) GetVisibleCategories() []CategoryView {
	if v.ShowAllCategories && len(v.Categories) > v.MaxDisplayItems {
		end := v.CategoryOffset + v.MaxDisplayItems
		if end > len(v.Categories) {
			end = len(v.Categories)
		}
		return v.Categories[v.CategoryOffset:end]
	}
	return v.Categories
}

// CanScrollUp returns true if the category list can scroll up.
func (v ClassifierView) CanScrollUp() bool {
	return v.ShowAllCategories && v.CategoryOffset > 0
}

// CanScrollDown returns true if the category list can scroll down.
func (v ClassifierView) CanScrollDown() bool {
	return v.ShowAllCategories && v.CategoryOffset+v.MaxDisplayItems < len(v.Categories)
}

// GetSelectedCategory returns the currently selected category if any.
func (v ClassifierView) GetSelectedCategory() *CategoryView {
	for i := range v.Categories {
		if v.Categories[i].IsSelected {
			return &v.Categories[i]
		}
	}
	return nil
}

// FormatAmount formats a transaction amount for display.
func FormatAmount(amount float64) string {
	return fmt.Sprintf("$%.2f", amount)
}

// TruncateString truncates a string to the specified length with ellipsis.
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// GetProgressPercentage calculates the progress percentage.
func (p ProgressView) GetProgressPercentage() float64 {
	if p.Total == 0 {
		return 0
	}
	return float64(p.Current) / float64(p.Total) * 100
}

// GetProgressBar returns a text-based progress bar.
func (p ProgressView) GetProgressBar(width int) string {
	if width <= 0 {
		return ""
	}

	percentage := p.GetProgressPercentage()
	filled := int(float64(width) * percentage / 100)

	if filled > width {
		filled = width
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return bar
}

// IsAnyTransactionSelected returns true if any transaction is selected.
func (v TransactionListView) IsAnyTransactionSelected() bool {
	for _, t := range v.Transactions {
		if t.IsSelected {
			return true
		}
	}
	return false
}

// GetSelectedTransactions returns all selected transactions.
func (v TransactionListView) GetSelectedTransactions() []TransactionItemView {
	var selected []TransactionItemView
	for _, t := range v.Transactions {
		if t.IsSelected {
			selected = append(selected, t)
		}
	}
	return selected
}

// FormatAmountWithSign formats amount with appropriate +/- prefix based on direction.
func FormatAmountWithSign(amount float64, isCredit bool) string {
	sign := "-"
	if isCredit {
		sign = "+"
	}
	return fmt.Sprintf("%s$%.2f", sign, amount)
}

// FormatDate formats a date for consistent display.
func FormatDate(t time.Time) string {
	return t.Format("2006-01-02")
}

// FormatDateShort formats a date in short form.
func FormatDateShort(t time.Time) string {
	return t.Format("Jan 02")
}

// GetConfidenceBar returns a visual confidence bar representation.
func GetConfidenceBar(confidence float64, width int) string {
	if width <= 0 {
		return ""
	}

	// Clamp confidence to 0-1 range
	if confidence < 0 {
		confidence = 0
	} else if confidence > 1 {
		confidence = 1
	}

	filled := int(confidence * float64(width))
	empty := width - filled

	return strings.Repeat("█", filled) + strings.Repeat("░", empty)
}

// GetConfidenceLevel returns a human-readable confidence level.
func GetConfidenceLevel(confidence float64) string {
	switch {
	case confidence >= 0.8:
		return "High"
	case confidence >= 0.5:
		return "Medium"
	default:
		return "Low"
	}
}

// SanitizeForDisplay removes potentially problematic characters for terminal display.
func SanitizeForDisplay(s string) string {
	// Remove control characters and normalize whitespace
	s = strings.Map(func(r rune) rune {
		if r < 32 && r != '\t' {
			return ' '
		}
		return r
	}, s)

	// Collapse multiple spaces
	return strings.Join(strings.Fields(s), " ")
}

// FormatDuration formats a duration in human-readable form.
func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		minutes := int(d.Minutes())
		seconds := int(d.Seconds()) % 60
		if seconds > 0 {
			return fmt.Sprintf("%dm %ds", minutes, seconds)
		}
		return fmt.Sprintf("%dm", minutes)
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	if minutes > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dh", hours)
}
