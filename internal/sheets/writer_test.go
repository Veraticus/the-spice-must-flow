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

func TestWriter_aggregateData(t *testing.T) {
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
			Category:        "Groceries",
			Status:          model.StatusClassifiedByAI,
			Confidence:      0.95,
			Notes:           "Weekly shopping",
			BusinessPercent: 0,
		},
		{
			Transaction: model.Transaction{
				ID:           "2",
				Date:         time.Date(2024, 1, 20, 0, 0, 0, 0, time.UTC),
				MerchantName: "Gas Station",
				Amount:       40.00,
			},
			Category:        "Transportation",
			Status:          model.StatusUserModified,
			Confidence:      1.0,
			Notes:           "Business trip",
			BusinessPercent: 50,
		},
		{
			Transaction: model.Transaction{
				ID:           "3",
				Date:         time.Date(2024, 1, 25, 0, 0, 0, 0, time.UTC),
				MerchantName: "Salary Deposit",
				Amount:       1000.00,
			},
			Category:        "Income",
			Status:          model.StatusClassifiedByRule,
			Confidence:      1.0,
			Notes:           "",
			BusinessPercent: 0,
		},
	}

	summary := &service.ReportSummary{
		DateRange: service.DateRange{
			Start: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			End:   time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC),
		},
		TotalAmount: 1090.00,
		ByCategory: map[string]service.CategorySummary{
			"Groceries": {
				Count:  1,
				Amount: 50.00,
			},
			"Transportation": {
				Count:  1,
				Amount: 40.00,
			},
			"Income": {
				Count:  1,
				Amount: 1000.00,
			},
		},
		ClassifiedBy: map[model.ClassificationStatus]int{
			model.StatusClassifiedByAI:   1,
			model.StatusUserModified:     1,
			model.StatusClassifiedByRule: 1,
		},
	}

	// Create category types map for the test
	categoryTypes := map[string]model.CategoryType{
		"Groceries":      model.CategoryTypeExpense,
		"Transportation": model.CategoryTypeExpense,
		"Income":         model.CategoryTypeIncome,
	}

	tabData, err := writer.aggregateData(classifications, summary, categoryTypes)
	require.NoError(t, err)

	// Verify income and expense separation
	assert.Len(t, tabData.Income, 1, "should have 1 income transaction")
	assert.Len(t, tabData.Expenses, 2, "should have 2 expense transactions")

	// Verify business expenses
	assert.Len(t, tabData.BusinessExpenses, 1, "should have 1 business expense")
	assert.Equal(t, 50, tabData.BusinessExpenses[0].BusinessPct)
	assert.Equal(t, "20", tabData.BusinessExpenses[0].DeductibleAmount.String())

	// Verify vendor summary
	assert.Len(t, tabData.VendorSummary, 3, "should have 3 vendors")

	// Verify category summary
	assert.Len(t, tabData.CategorySummary, 3, "should have 3 categories")

	// Verify monthly flow
	assert.Len(t, tabData.MonthlyFlow, 1, "should have 1 month")
	assert.Equal(t, "1000", tabData.MonthlyFlow[0].TotalIncome.String())
	assert.Equal(t, "90", tabData.MonthlyFlow[0].TotalExpenses.String())
	assert.Equal(t, "910", tabData.MonthlyFlow[0].NetFlow.String())

	// Verify totals
	assert.Equal(t, "1000", tabData.TotalIncome.String())
	assert.Equal(t, "90", tabData.TotalExpenses.String())
	assert.Equal(t, "20", tabData.TotalDeductible.String())
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

func TestWriter_applyFormatting(t *testing.T) {
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

// Formatting tests

func TestFormatCurrencyColumn(t *testing.T) {
	req := formatCurrencyColumn(123, 1, 0, 100)

	assert.NotNil(t, req)
	assert.NotNil(t, req.RepeatCell)
	assert.Equal(t, int64(123), req.RepeatCell.Range.SheetId)
	assert.Equal(t, int64(0), req.RepeatCell.Range.StartRowIndex)
	assert.Equal(t, int64(100), req.RepeatCell.Range.EndRowIndex)
	assert.Equal(t, int64(1), req.RepeatCell.Range.StartColumnIndex)
	assert.Equal(t, int64(2), req.RepeatCell.Range.EndColumnIndex)
	assert.Equal(t, "CURRENCY", req.RepeatCell.Cell.UserEnteredFormat.NumberFormat.Type)
	assert.Equal(t, "$#,##0.00", req.RepeatCell.Cell.UserEnteredFormat.NumberFormat.Pattern)
	assert.Equal(t, "userEnteredFormat.numberFormat", req.RepeatCell.Fields)
}

func TestFormatPercentageColumn(t *testing.T) {
	req := formatPercentageColumn(456, 3, 1, 50)

	assert.NotNil(t, req)
	assert.NotNil(t, req.RepeatCell)
	assert.Equal(t, int64(456), req.RepeatCell.Range.SheetId)
	assert.Equal(t, int64(1), req.RepeatCell.Range.StartRowIndex)
	assert.Equal(t, int64(50), req.RepeatCell.Range.EndRowIndex)
	assert.Equal(t, int64(3), req.RepeatCell.Range.StartColumnIndex)
	assert.Equal(t, int64(4), req.RepeatCell.Range.EndColumnIndex)
	assert.Equal(t, "PERCENT", req.RepeatCell.Cell.UserEnteredFormat.NumberFormat.Type)
	assert.Equal(t, "0%", req.RepeatCell.Cell.UserEnteredFormat.NumberFormat.Pattern)
}

func TestFormatDateColumn(t *testing.T) {
	req := formatDateColumn(789, 0, 1, 200)

	assert.NotNil(t, req)
	assert.NotNil(t, req.RepeatCell)
	assert.Equal(t, int64(789), req.RepeatCell.Range.SheetId)
	assert.Equal(t, int64(1), req.RepeatCell.Range.StartRowIndex)
	assert.Equal(t, int64(200), req.RepeatCell.Range.EndRowIndex)
	assert.Equal(t, int64(0), req.RepeatCell.Range.StartColumnIndex)
	assert.Equal(t, int64(1), req.RepeatCell.Range.EndColumnIndex)
	assert.Equal(t, "DATE", req.RepeatCell.Cell.UserEnteredFormat.NumberFormat.Type)
	assert.Equal(t, "yyyy-mm-dd", req.RepeatCell.Cell.UserEnteredFormat.NumberFormat.Pattern)
}

func TestFormatHeaderRow(t *testing.T) {
	req := formatHeaderRow(111, 0, 0, 5)

	assert.NotNil(t, req)
	assert.NotNil(t, req.RepeatCell)
	assert.Equal(t, int64(111), req.RepeatCell.Range.SheetId)
	assert.Equal(t, int64(0), req.RepeatCell.Range.StartRowIndex)
	assert.Equal(t, int64(1), req.RepeatCell.Range.EndRowIndex)
	assert.Equal(t, int64(0), req.RepeatCell.Range.StartColumnIndex)
	assert.Equal(t, int64(5), req.RepeatCell.Range.EndColumnIndex)
	assert.True(t, req.RepeatCell.Cell.UserEnteredFormat.TextFormat.Bold)
	assert.Equal(t, "userEnteredFormat.textFormat.bold", req.RepeatCell.Fields)
}

func TestFreezeRows(t *testing.T) {
	req := freezeRows(222, 2)

	assert.NotNil(t, req)
	assert.NotNil(t, req.UpdateSheetProperties)
	assert.Equal(t, int64(222), req.UpdateSheetProperties.Properties.SheetId)
	assert.Equal(t, int64(2), req.UpdateSheetProperties.Properties.GridProperties.FrozenRowCount)
	assert.Equal(t, "gridProperties.frozenRowCount", req.UpdateSheetProperties.Fields)
}

func TestAddBorders(t *testing.T) {
	req := addBorders(333, 0, 10, 0, 5)

	assert.NotNil(t, req)
	assert.NotNil(t, req.UpdateBorders)
	assert.Equal(t, int64(333), req.UpdateBorders.Range.SheetId)
	assert.Equal(t, int64(0), req.UpdateBorders.Range.StartRowIndex)
	assert.Equal(t, int64(10), req.UpdateBorders.Range.EndRowIndex)
	assert.Equal(t, int64(0), req.UpdateBorders.Range.StartColumnIndex)
	assert.Equal(t, int64(5), req.UpdateBorders.Range.EndColumnIndex)

	// Check border style
	assert.Equal(t, "SOLID", req.UpdateBorders.Top.Style)
	assert.Equal(t, "SOLID", req.UpdateBorders.Bottom.Style)
	assert.Equal(t, "SOLID", req.UpdateBorders.Left.Style)
	assert.Equal(t, "SOLID", req.UpdateBorders.Right.Style)
}

func TestAutoResizeColumns(t *testing.T) {
	req := autoResizeColumns(444, 2, 8)

	assert.NotNil(t, req)
	assert.NotNil(t, req.AutoResizeDimensions)
	assert.Equal(t, int64(444), req.AutoResizeDimensions.Dimensions.SheetId)
	assert.Equal(t, "COLUMNS", req.AutoResizeDimensions.Dimensions.Dimension)
	assert.Equal(t, int64(2), req.AutoResizeDimensions.Dimensions.StartIndex)
	assert.Equal(t, int64(8), req.AutoResizeDimensions.Dimensions.EndIndex)
}

func TestFormatNumberColumn(t *testing.T) {
	req := formatNumberColumn(555, 4, 1, 100)

	assert.NotNil(t, req)
	assert.NotNil(t, req.RepeatCell)
	assert.Equal(t, int64(555), req.RepeatCell.Range.SheetId)
	assert.Equal(t, int64(1), req.RepeatCell.Range.StartRowIndex)
	assert.Equal(t, int64(100), req.RepeatCell.Range.EndRowIndex)
	assert.Equal(t, int64(4), req.RepeatCell.Range.StartColumnIndex)
	assert.Equal(t, int64(5), req.RepeatCell.Range.EndColumnIndex)
	assert.Equal(t, "NUMBER", req.RepeatCell.Cell.UserEnteredFormat.NumberFormat.Type)
	assert.Equal(t, "#,##0", req.RepeatCell.Cell.UserEnteredFormat.NumberFormat.Pattern)
}

func TestWriter_formatExpensesTab(t *testing.T) {
	writer := &Writer{
		logger: slog.New(slog.NewTextHandler(os.Stdout, nil)),
		config: DefaultConfig(),
	}

	requests := writer.formatExpensesTab(100)

	// Should have: header row, freeze, date column, currency column, percentage column, auto-resize, borders
	assert.Len(t, requests, 7)

	// Verify each request type
	foundHeader := false
	foundFreeze := false
	foundDate := false
	foundCurrency := false
	foundPercentage := false
	foundAutoResize := false
	foundBorders := false

	for _, req := range requests {
		if req.RepeatCell != nil && req.RepeatCell.Cell.UserEnteredFormat != nil {
			if req.RepeatCell.Cell.UserEnteredFormat.TextFormat != nil && req.RepeatCell.Cell.UserEnteredFormat.TextFormat.Bold {
				foundHeader = true
			}
			if req.RepeatCell.Cell.UserEnteredFormat.NumberFormat != nil {
				switch req.RepeatCell.Cell.UserEnteredFormat.NumberFormat.Type {
				case "DATE":
					foundDate = true
				case "CURRENCY":
					foundCurrency = true
				case "PERCENT":
					foundPercentage = true
				}
			}
		}
		if req.UpdateSheetProperties != nil {
			foundFreeze = true
		}
		if req.AutoResizeDimensions != nil {
			foundAutoResize = true
		}
		if req.UpdateBorders != nil {
			foundBorders = true
		}
	}

	assert.True(t, foundHeader, "should have header formatting")
	assert.True(t, foundFreeze, "should have freeze rows")
	assert.True(t, foundDate, "should have date formatting")
	assert.True(t, foundCurrency, "should have currency formatting")
	assert.True(t, foundPercentage, "should have percentage formatting")
	assert.True(t, foundAutoResize, "should have auto-resize")
	assert.True(t, foundBorders, "should have borders")
}

func TestWriter_formatIncomeTab(t *testing.T) {
	writer := &Writer{
		logger: slog.New(slog.NewTextHandler(os.Stdout, nil)),
		config: DefaultConfig(),
	}

	requests := writer.formatIncomeTab(200)

	// Should have: header row, freeze, date column, currency column, auto-resize, borders
	assert.Len(t, requests, 6)

	// Verify it doesn't have percentage formatting (income tab doesn't have business %)
	hasPercentage := false
	for _, req := range requests {
		if req.RepeatCell != nil && req.RepeatCell.Cell.UserEnteredFormat != nil &&
			req.RepeatCell.Cell.UserEnteredFormat.NumberFormat != nil &&
			req.RepeatCell.Cell.UserEnteredFormat.NumberFormat.Type == "PERCENT" {
			hasPercentage = true
		}
	}
	assert.False(t, hasPercentage, "Income tab should not have percentage formatting")
}

func TestWriter_formatVendorSummaryTab(t *testing.T) {
	writer := &Writer{
		logger: slog.New(slog.NewTextHandler(os.Stdout, nil)),
		config: DefaultConfig(),
	}

	requests := writer.formatVendorSummaryTab(300)

	// Should have: header row, freeze, currency column, number column, auto-resize, borders
	assert.Len(t, requests, 6)

	// Verify it has number formatting
	hasNumber := false
	for _, req := range requests {
		if req.RepeatCell != nil && req.RepeatCell.Cell.UserEnteredFormat != nil &&
			req.RepeatCell.Cell.UserEnteredFormat.NumberFormat != nil &&
			req.RepeatCell.Cell.UserEnteredFormat.NumberFormat.Type == "NUMBER" {
			hasNumber = true
		}
	}
	assert.True(t, hasNumber, "Vendor Summary tab should have number formatting")
}

func TestWriter_formatCategorySummaryTab(t *testing.T) {
	writer := &Writer{
		logger: slog.New(slog.NewTextHandler(os.Stdout, nil)),
		config: DefaultConfig(),
	}

	requests := writer.formatCategorySummaryTab(400)

	// Should have multiple requests including conditional formatting
	assert.True(t, len(requests) > 10, "Category Summary tab should have many formatting requests")

	// Verify it has conditional formatting
	hasConditional := false
	for _, req := range requests {
		if req.AddConditionalFormatRule != nil {
			hasConditional = true
			// Verify the conditional rule is properly structured
			assert.NotNil(t, req.AddConditionalFormatRule.Rule)
			assert.NotNil(t, req.AddConditionalFormatRule.Rule.BooleanRule)
			assert.Equal(t, "CUSTOM_FORMULA", req.AddConditionalFormatRule.Rule.BooleanRule.Condition.Type)
		}
	}
	assert.True(t, hasConditional, "Category Summary tab should have conditional formatting")
}

func TestWriter_formatBusinessExpensesTab(t *testing.T) {
	writer := &Writer{
		logger: slog.New(slog.NewTextHandler(os.Stdout, nil)),
		config: DefaultConfig(),
	}

	requests := writer.formatBusinessExpensesTab(500)

	// Should have: header row, freeze, date column, 2 currency columns, percentage column, auto-resize, borders
	assert.Len(t, requests, 8)

	// Count currency columns (should be 2)
	currencyCount := 0
	for _, req := range requests {
		if req.RepeatCell != nil && req.RepeatCell.Cell.UserEnteredFormat != nil &&
			req.RepeatCell.Cell.UserEnteredFormat.NumberFormat != nil &&
			req.RepeatCell.Cell.UserEnteredFormat.NumberFormat.Type == "CURRENCY" {
			currencyCount++
		}
	}
	assert.Equal(t, 2, currencyCount, "Business Expenses tab should have 2 currency columns")
}

func TestWriter_formatMonthlyFlowTab(t *testing.T) {
	writer := &Writer{
		logger: slog.New(slog.NewTextHandler(os.Stdout, nil)),
		config: DefaultConfig(),
	}

	requests := writer.formatMonthlyFlowTab(600)

	// Should have multiple requests including conditional formatting for Net Flow
	assert.True(t, len(requests) >= 7, "Monthly Flow tab should have at least 7 formatting requests")

	// Verify it has 2 conditional formatting rules (red for negative, green for positive)
	conditionalCount := 0
	for _, req := range requests {
		if req.AddConditionalFormatRule != nil {
			conditionalCount++
			rule := req.AddConditionalFormatRule.Rule
			assert.NotNil(t, rule)
			assert.NotNil(t, rule.BooleanRule)

			// Check that it targets the Net Flow column (column 3)
			assert.Equal(t, int64(3), rule.Ranges[0].StartColumnIndex)
			assert.Equal(t, int64(4), rule.Ranges[0].EndColumnIndex)
		}
	}
	assert.Equal(t, 2, conditionalCount, "Monthly Flow tab should have 2 conditional formatting rules")
}

func TestWriter_applyFormattingToAllTabs_Integration(t *testing.T) {
	// This is an integration test that would require mocking the Google Sheets API
	t.Skip("Requires mocking Google Sheets API for full integration test")
}
