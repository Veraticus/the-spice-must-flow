package model

import "time"

// VendorSource indicates how a vendor rule was created.
type VendorSource string

const (
	// SourceAuto indicates vendor was created automatically from classification.
	SourceAuto VendorSource = "AUTO"
	// SourceManual indicates vendor was created via CLI command.
	SourceManual VendorSource = "MANUAL"
	// SourceAutoConfirmed indicates auto-created vendor that user has edited.
	SourceAutoConfirmed VendorSource = "AUTO_CONFIRMED"
)

// Vendor represents a known merchant with a user-confirmed category.
type Vendor struct {
	LastUpdated time.Time
	Name        string
	Category    string
	Source      VendorSource
	UseCount    int
	IsRegex     bool
}
