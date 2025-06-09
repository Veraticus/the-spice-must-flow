// Package sheets provides Google Sheets API integration for report generation.
package sheets

import (
	"fmt"
	"os"
	"time"
)

// Config holds the configuration for the Google Sheets writer.
type Config struct {
	ClientID           string
	ClientSecret       string
	RefreshToken       string
	ServiceAccountPath string
	SpreadsheetID      string
	SpreadsheetName    string
	TimeZone           string
	BatchSize          int
	RetryAttempts      int
	RetryDelay         time.Duration
	EnableFormatting   bool
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		EnableFormatting: true,
		TimeZone:         "America/New_York",
		BatchSize:        1000,
		RetryAttempts:    3,
		RetryDelay:       time.Second,
	}
}

// LoadFromEnv loads the configuration from environment variables.
func (c *Config) LoadFromEnv() error {
	// OAuth2 credentials
	c.ClientID = os.Getenv("GOOGLE_SHEETS_CLIENT_ID")
	c.ClientSecret = os.Getenv("GOOGLE_SHEETS_CLIENT_SECRET")
	c.RefreshToken = os.Getenv("GOOGLE_SHEETS_REFRESH_TOKEN")

	// Service account path (alternative to OAuth2)
	c.ServiceAccountPath = os.Getenv("GOOGLE_SHEETS_SERVICE_ACCOUNT_PATH")

	// Spreadsheet settings
	c.SpreadsheetID = os.Getenv("GOOGLE_SHEETS_SPREADSHEET_ID")
	c.SpreadsheetName = os.Getenv("GOOGLE_SHEETS_SPREADSHEET_NAME")

	// Validate that we have at least one auth method
	if c.ServiceAccountPath == "" && (c.ClientID == "" || c.ClientSecret == "" || c.RefreshToken == "") {
		return fmt.Errorf("missing Google Sheets authentication: provide either service account path or OAuth2 credentials")
	}

	// Use default name if not provided
	if c.SpreadsheetName == "" {
		c.SpreadsheetName = "Finance Report"
	}

	return nil
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	// Check authentication
	hasOAuth := c.ClientID != "" && c.ClientSecret != "" && c.RefreshToken != ""
	hasServiceAccount := c.ServiceAccountPath != ""

	if !hasOAuth && !hasServiceAccount {
		return fmt.Errorf("no authentication method configured")
	}

	if hasOAuth && hasServiceAccount {
		return fmt.Errorf("multiple authentication methods configured; use either OAuth2 or service account")
	}

	// Validate batch size
	if c.BatchSize <= 0 {
		return fmt.Errorf("batch size must be positive")
	}

	// Validate retry settings
	if c.RetryAttempts < 0 {
		return fmt.Errorf("retry attempts cannot be negative")
	}

	if c.RetryDelay < 0 {
		return fmt.Errorf("retry delay cannot be negative")
	}

	return nil
}
