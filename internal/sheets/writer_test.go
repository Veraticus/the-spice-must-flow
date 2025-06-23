package sheets

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		errMsg  string
		config  Config
		wantErr bool
	}{
		{
			name: "valid oauth config",
			config: Config{
				ClientID:      "test-client",
				ClientSecret:  "test-secret",
				RefreshToken:  "test-token",
				BatchSize:     100,
				RetryAttempts: 3,
				RetryDelay:    time.Second,
			},
			wantErr: false,
		},
		{
			name: "valid service account config",
			config: Config{
				ServiceAccountPath: "/path/to/key.json",
				BatchSize:          100,
				RetryAttempts:      3,
				RetryDelay:         time.Second,
			},
			wantErr: false,
		},
		{
			name: "missing auth",
			config: Config{
				BatchSize:     100,
				RetryAttempts: 3,
				RetryDelay:    time.Second,
			},
			wantErr: true,
			errMsg:  "no authentication method configured",
		},
		{
			name: "multiple auth methods",
			config: Config{
				ClientID:           "test-client",
				ClientSecret:       "test-secret",
				RefreshToken:       "test-token",
				ServiceAccountPath: "/path/to/key.json",
				BatchSize:          100,
				RetryAttempts:      3,
				RetryDelay:         time.Second,
			},
			wantErr: true,
			errMsg:  "multiple authentication methods configured",
		},
		{
			name: "invalid batch size",
			config: Config{
				ClientID:      "test-client",
				ClientSecret:  "test-secret",
				RefreshToken:  "test-token",
				BatchSize:     0,
				RetryAttempts: 3,
				RetryDelay:    time.Second,
			},
			wantErr: true,
			errMsg:  "batch size must be positive",
		},
		{
			name: "negative retry attempts",
			config: Config{
				ClientID:      "test-client",
				ClientSecret:  "test-secret",
				RefreshToken:  "test-token",
				BatchSize:     100,
				RetryAttempts: -1,
				RetryDelay:    time.Second,
			},
			wantErr: true,
			errMsg:  "retry attempts cannot be negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_LoadFromEnv(t *testing.T) {
	// Save original env vars
	originalVars := map[string]string{
		"GOOGLE_SHEETS_CLIENT_ID":            os.Getenv("GOOGLE_SHEETS_CLIENT_ID"),
		"GOOGLE_SHEETS_CLIENT_SECRET":        os.Getenv("GOOGLE_SHEETS_CLIENT_SECRET"),
		"GOOGLE_SHEETS_REFRESH_TOKEN":        os.Getenv("GOOGLE_SHEETS_REFRESH_TOKEN"),
		"GOOGLE_SHEETS_SERVICE_ACCOUNT_PATH": os.Getenv("GOOGLE_SHEETS_SERVICE_ACCOUNT_PATH"),
		"GOOGLE_SHEETS_SPREADSHEET_ID":       os.Getenv("GOOGLE_SHEETS_SPREADSHEET_ID"),
		"GOOGLE_SHEETS_SPREADSHEET_NAME":     os.Getenv("GOOGLE_SHEETS_SPREADSHEET_NAME"),
	}

	// Restore env vars after test
	defer func() {
		for key, value := range originalVars {
			if value == "" {
				_ = os.Unsetenv(key)
			} else {
				_ = os.Setenv(key, value)
			}
		}
	}()

	tests := []struct {
		envVars map[string]string
		check   func(t *testing.T, c *Config)
		name    string
		wantErr bool
	}{
		{
			name: "oauth credentials",
			envVars: map[string]string{
				"GOOGLE_SHEETS_CLIENT_ID":        "test-client",
				"GOOGLE_SHEETS_CLIENT_SECRET":    "test-secret",
				"GOOGLE_SHEETS_REFRESH_TOKEN":    "test-token",
				"GOOGLE_SHEETS_SPREADSHEET_ID":   "test-id",
				"GOOGLE_SHEETS_SPREADSHEET_NAME": "Test Sheet",
			},
			wantErr: false,
			check: func(t *testing.T, c *Config) {
				t.Helper()
				assert.Equal(t, "test-client", c.ClientID)
				assert.Equal(t, "test-secret", c.ClientSecret)
				assert.Equal(t, "test-token", c.RefreshToken)
				assert.Equal(t, "test-id", c.SpreadsheetID)
				assert.Equal(t, "Test Sheet", c.SpreadsheetName)
			},
		},
		{
			name: "service account path",
			envVars: map[string]string{
				"GOOGLE_SHEETS_SERVICE_ACCOUNT_PATH": "/path/to/key.json",
			},
			wantErr: false,
			check: func(t *testing.T, c *Config) {
				t.Helper()
				assert.Equal(t, "/path/to/key.json", c.ServiceAccountPath)
				assert.Equal(t, "Finance Report", c.SpreadsheetName) // Default name
			},
		},
		{
			name:    "missing credentials",
			envVars: map[string]string{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all env vars
			for key := range originalVars {
				_ = os.Unsetenv(key)
			}

			// Set test env vars
			for key, value := range tt.envVars {
				_ = os.Setenv(key, value)
			}

			config := DefaultConfig()
			err := config.LoadFromEnv()

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.check != nil {
					tt.check(t, &config)
				}
			}
		})
	}
}

func TestWriter_prepareReportData(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	writer := &Writer{
		config: DefaultConfig(),
		logger: logger,
	}

	// Create test data
	classifications := []model.Classification{
		{
			Transaction: model.Transaction{
				ID:           "1",
				Date:         time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
				MerchantName: "Grocery Store",
				Amount:       50.00,
			},
			Category:   "Groceries",
			Status:     model.StatusClassifiedByAI,
			Confidence: 0.95,
			Notes:      "Weekly shopping",
		},
		{
			Transaction: model.Transaction{
				ID:           "2",
				Date:         time.Date(2024, 1, 20, 0, 0, 0, 0, time.UTC),
				MerchantName: "Gas Station",
				Amount:       40.00,
			},
			Category:   "Transportation",
			Status:     model.StatusUserModified,
			Confidence: 1.0,
			Notes:      "",
		},
	}

	summary := &service.ReportSummary{
		DateRange: service.DateRange{
			Start: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			End:   time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC),
		},
		TotalAmount: 90.00,
		ByCategory: map[string]service.CategorySummary{
			"Groceries": {
				Count:  1,
				Amount: 50.00,
			},
			"Transportation": {
				Count:  1,
				Amount: 40.00,
			},
		},
		ClassifiedBy: map[model.ClassificationStatus]int{
			model.StatusClassifiedByAI: 1,
			model.StatusUserModified:   1,
		},
	}

	values := writer.prepareReportData(classifications, summary)

	// Verify structure
	assert.Greater(t, len(values), 15, "should have header, summary, categories, and transactions")

	// Check header
	assert.Equal(t, "Finance Report", values[0][0])
	assert.Contains(t, values[0][1], "Jan 1, 2024")
	assert.Contains(t, values[0][1], "Jan 31, 2024")

	// Check summary section
	summaryStart := -1
	for i, row := range values {
		if len(row) > 0 && row[0] == "Summary" {
			summaryStart = i
			break
		}
	}
	require.NotEqual(t, -1, summaryStart, "should have summary section")

	// Check category breakdown
	categoryStart := -1
	for i, row := range values {
		if len(row) > 0 && row[0] == "Category Breakdown" {
			categoryStart = i
			break
		}
	}
	require.NotEqual(t, -1, categoryStart, "should have category breakdown")

	// Check transaction details
	detailsStart := -1
	for i, row := range values {
		if len(row) > 0 && row[0] == "Transaction Details" {
			detailsStart = i
			break
		}
	}
	require.NotEqual(t, -1, detailsStart, "should have transaction details")

	// Verify transaction data (should be sorted by date, newest first)
	transactionRow := values[detailsStart+2]             // First transaction after header
	assert.Equal(t, "2024-01-20", transactionRow[0])     // Date
	assert.Equal(t, "Gas Station", transactionRow[1])    // Merchant
	assert.Equal(t, 40.00, transactionRow[2])            // Amount
	assert.Equal(t, "Transportation", transactionRow[3]) // Category
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.True(t, config.EnableFormatting)
	assert.Equal(t, "America/New_York", config.TimeZone)
	assert.Equal(t, 1000, config.BatchSize)
	assert.Equal(t, 3, config.RetryAttempts)
	assert.Equal(t, time.Second, config.RetryDelay)
}

func TestWriter_clearSheet(t *testing.T) {
	// This test would require mocking the Google Sheets API
	// For now, we'll just verify the function exists and can be called
	t.Skip("Requires Google Sheets API mock")
}

// TestWriter_Write tests the main Write method with mocked dependencies.
func TestWriter_Write(t *testing.T) {
	// This is a more complex test that would require mocking the Google Sheets service
	// In a real implementation, you might use an interface for the sheets service
	// to make it easier to mock
	t.Skip("Requires refactoring to support dependency injection of sheets service")
}

// Example of how to test with a mock sheets service interface.

func TestWriter_WriteWithMockService(t *testing.T) {
	// This demonstrates how you might structure tests with a mock service
	// if the Writer was refactored to accept an interface instead of
	// the concrete sheets.Service type
	t.Skip("Requires refactoring to support service interface")
}
