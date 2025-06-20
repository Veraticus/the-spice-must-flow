package model

import "time"

// Category represents a valid expense category.
type Category struct {
	CreatedAt   time.Time
	Name        string
	Description string
	ID          int
	IsActive    bool
}
