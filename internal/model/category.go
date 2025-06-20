package model

import "time"

// Category represents a valid expense category.
type Category struct {
	ID        int
	Name      string
	CreatedAt time.Time
	IsActive  bool
}