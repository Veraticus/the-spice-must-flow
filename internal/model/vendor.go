package model

import "time"

// Vendor represents a known merchant with a user-confirmed category.
type Vendor struct {
	LastUpdated time.Time
	Name        string
	Category    string
	UseCount    int
}
