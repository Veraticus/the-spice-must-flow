// Package config provides configuration utilities for the application.
package config

import (
	"os"

	"github.com/joshsymonds/the-spice-must-flow/internal/sheets"
	"github.com/spf13/viper"
)

// LoadSheetsConfig loads Google Sheets configuration from Viper and environment variables.
// It follows this precedence:
// 1. Viper configuration (from config file or SPICE_ env vars)
// 2. Direct environment variables (GOOGLE_SHEETS_*)
// 3. Default values
func LoadSheetsConfig() (*sheets.Config, error) {
	config := sheets.DefaultConfig()

	// Load from Viper first
	if v := viper.GetString("sheets.service_account_path"); v != "" {
		config.ServiceAccountPath = expandPath(v)
	}
	if v := viper.GetString("sheets.client_id"); v != "" {
		config.ClientID = v
	}
	if v := viper.GetString("sheets.client_secret"); v != "" {
		config.ClientSecret = v
	}
	if v := viper.GetString("sheets.refresh_token"); v != "" {
		config.RefreshToken = v
	}
	if v := viper.GetString("sheets.spreadsheet_id"); v != "" {
		config.SpreadsheetID = v
	}
	if v := viper.GetString("sheets.spreadsheet_name"); v != "" {
		config.SpreadsheetName = v
	}

	// Override with direct environment variables if not set
	if config.ServiceAccountPath == "" {
		if v := os.Getenv("GOOGLE_SHEETS_SERVICE_ACCOUNT_PATH"); v != "" {
			config.ServiceAccountPath = expandPath(v)
		}
	}
	if config.ClientID == "" {
		config.ClientID = os.Getenv("GOOGLE_SHEETS_CLIENT_ID")
	}
	if config.ClientSecret == "" {
		config.ClientSecret = os.Getenv("GOOGLE_SHEETS_CLIENT_SECRET")
	}
	if config.RefreshToken == "" {
		config.RefreshToken = os.Getenv("GOOGLE_SHEETS_REFRESH_TOKEN")
	}
	if config.SpreadsheetID == "" {
		config.SpreadsheetID = os.Getenv("GOOGLE_SHEETS_SPREADSHEET_ID")
	}
	if config.SpreadsheetName == "" || config.SpreadsheetName == "Finance Report" {
		if v := os.Getenv("GOOGLE_SHEETS_SPREADSHEET_NAME"); v != "" {
			config.SpreadsheetName = v
		}
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, err
	}

	return &config, nil
}

func expandPath(path string) string {
	if path != "" && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err == nil {
			path = home + path[1:]
		}
	}
	return os.ExpandEnv(path)
}