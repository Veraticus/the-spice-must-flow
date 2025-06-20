package model

import "time"

// Category represents a valid expense category.
type Category struct {
	ID          int
	Name        string
	Description string    // Brief description to help LLM classify transactions
	CreatedAt   time.Time
	IsActive    bool
}