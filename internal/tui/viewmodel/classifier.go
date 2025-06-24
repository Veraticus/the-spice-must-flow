// Package viewmodel defines the data structures for TUI rendering.
package viewmodel

import (
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
)

// ClassifierMode represents the mode of the classifier component.
type ClassifierMode int

const (
	// ModeSelectingSuggestion indicates the user is selecting from AI suggestions.
	ModeSelectingSuggestion ClassifierMode = iota
	// ModeEnteringCustom indicates the user is entering a custom category.
	ModeEnteringCustom
	// ModeSelectingCategory indicates the user is browsing all categories.
	ModeSelectingCategory
	// ModeConfirming indicates the user is confirming their selection.
	ModeConfirming
)

// ClassifierView represents the classifier component's display data.
type ClassifierView struct {
	Transaction       TransactionView
	CustomInput       string
	Error             string
	Categories        []CategoryView
	Mode              ClassifierMode
	Cursor            int
	CategoryOffset    int
	MaxDisplayItems   int
	ShowAllCategories bool
}

// TransactionView represents transaction display data.
type TransactionView struct {
	Date         time.Time
	MerchantName string
	ID           string
	CheckNumber  string
	Type         string
	Direction    model.TransactionDirection
	Amount       float64
}

// CategoryView represents a category option.
type CategoryView struct {
	Name           string
	Icon           string
	Confidence     float64
	ID             int
	DisplayIndex   int // For showing "1.", "2.", etc.
	IsSelected     bool
	HasPattern     bool
	IsAISuggestion bool
}

// IsValidSelection returns true if this category can be selected.
func (cv CategoryView) IsValidSelection() bool {
	return cv.ID > 0 && cv.Name != ""
}

// HasError returns true if the classifier has an error.
func (cv ClassifierView) HasError() bool {
	return cv.Error != ""
}

// IsCustomMode returns true if the classifier is in custom category entry mode.
func (cv ClassifierView) IsCustomMode() bool {
	return cv.Mode == ModeEnteringCustom
}

// HasSuggestions returns true if there are AI suggestions available.
func (cv ClassifierView) HasSuggestions() bool {
	for _, cat := range cv.Categories {
		if cat.IsAISuggestion {
			return true
		}
	}
	return false
}
