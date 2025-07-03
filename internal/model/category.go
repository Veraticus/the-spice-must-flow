package model

import "time"

// CategoryType indicates whether a category is for income, expense, or system use.
type CategoryType string

const (
	// CategoryTypeIncome represents categories for income transactions.
	CategoryTypeIncome CategoryType = "income"
	// CategoryTypeExpense represents categories for expense transactions.
	CategoryTypeExpense CategoryType = "expense"
	// CategoryTypeSystem represents system-managed categories (e.g., transfers).
	CategoryTypeSystem CategoryType = "system"
)

// Category represents a valid expense category.
type Category struct {
	CreatedAt              time.Time
	Name                   string
	Description            string
	Type                   CategoryType
	ID                     int
	DefaultBusinessPercent int
	IsActive               bool
}
