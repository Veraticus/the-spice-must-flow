//go:build integration
// +build integration

package sheets

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/joshsymonds/the-spice-must-flow/internal/model"
	"github.com/joshsymonds/the-spice-must-flow/internal/service"
	"github.com/stretchr/testify/require"
)

func TestWriter_Integration_OAuth2(t *testing.T) {
	// Skip if OAuth2 credentials are not available
	clientID := os.Getenv("GOOGLE_SHEETS_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_SHEETS_CLIENT_SECRET")
	refreshToken := os.Getenv("GOOGLE_SHEETS_REFRESH_TOKEN")

	if clientID == "" || clientSecret == "" || refreshToken == "" {
		t.Skip("OAuth2 credentials not available")
	}

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	config := Config{
		ClientID:         clientID,
		ClientSecret:     clientSecret,
		RefreshToken:     refreshToken,
		SpreadsheetName:  "Test Finance Report - Integration",
		EnableFormatting: true,
		TimeZone:         "America/New_York",
		BatchSize:        100,
		RetryAttempts:    3,
		RetryDelay:       time.Second,
	}

	writer, err := NewWriter(ctx, config, logger)
	require.NoError(t, err)

	// Create test data
	classifications := generateTestClassifications()
	summary := generateTestSummary(classifications)

	// Write the report
	err = writer.Write(ctx, classifications, summary)
	require.NoError(t, err)
}

func TestWriter_Integration_ServiceAccount(t *testing.T) {
	// Skip if service account path is not available
	serviceAccountPath := os.Getenv("GOOGLE_SHEETS_SERVICE_ACCOUNT_PATH")
	if serviceAccountPath == "" {
		t.Skip("Service account path not available")
	}

	// Verify the file exists
	if _, err := os.Stat(serviceAccountPath); os.IsNotExist(err) {
		t.Skipf("Service account file does not exist: %s", serviceAccountPath)
	}

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	config := Config{
		ServiceAccountPath: serviceAccountPath,
		SpreadsheetName:    "Test Finance Report - Service Account",
		EnableFormatting:   true,
		TimeZone:           "America/New_York",
		BatchSize:          100,
		RetryAttempts:      3,
		RetryDelay:         time.Second,
	}

	writer, err := NewWriter(ctx, config, logger)
	require.NoError(t, err)

	// Create test data
	classifications := generateTestClassifications()
	summary := generateTestSummary(classifications)

	// Write the report
	err = writer.Write(ctx, classifications, summary)
	require.NoError(t, err)
}

func TestWriter_Integration_ExistingSpreadsheet(t *testing.T) {
	// Skip if credentials and spreadsheet ID are not available
	spreadsheetID := os.Getenv("GOOGLE_SHEETS_TEST_SPREADSHEET_ID")
	if spreadsheetID == "" {
		t.Skip("Test spreadsheet ID not available")
	}

	serviceAccountPath := os.Getenv("GOOGLE_SHEETS_SERVICE_ACCOUNT_PATH")
	if serviceAccountPath == "" {
		t.Skip("Service account path not available")
	}

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	config := Config{
		ServiceAccountPath: serviceAccountPath,
		SpreadsheetID:      spreadsheetID,
		EnableFormatting:   true,
		TimeZone:           "America/New_York",
		BatchSize:          100,
		RetryAttempts:      3,
		RetryDelay:         time.Second,
	}

	writer, err := NewWriter(ctx, config, logger)
	require.NoError(t, err)

	// Create test data
	classifications := generateTestClassifications()
	summary := generateTestSummary(classifications)

	// Write the report
	err = writer.Write(ctx, classifications, summary)
	require.NoError(t, err)
}

func TestWriter_Integration_LargeDataset(t *testing.T) {
	// Skip if service account path is not available
	serviceAccountPath := os.Getenv("GOOGLE_SHEETS_SERVICE_ACCOUNT_PATH")
	if serviceAccountPath == "" {
		t.Skip("Service account path not available")
	}

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	config := Config{
		ServiceAccountPath: serviceAccountPath,
		SpreadsheetName:    "Test Finance Report - Large Dataset",
		EnableFormatting:   true,
		TimeZone:           "America/New_York",
		BatchSize:          500, // Test batching
		RetryAttempts:      3,
		RetryDelay:         time.Second,
	}

	writer, err := NewWriter(ctx, config, logger)
	require.NoError(t, err)

	// Create large test dataset
	classifications := generateLargeTestDataset(2000) // 2000 transactions
	summary := generateTestSummary(classifications)

	// Write the report
	err = writer.Write(ctx, classifications, summary)
	require.NoError(t, err)
}

// Helper functions for generating test data
func generateTestClassifications() []model.Classification {
	baseDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	return []model.Classification{
		{
			Transaction: model.Transaction{
				ID:           "test-1",
				Date:         baseDate,
				MerchantName: "Whole Foods Market",
				Amount:       125.50,
			},
			Category:     "Groceries",
			Status:       model.StatusClassifiedByAI,
			Confidence:   0.95,
			Notes:        "Weekly grocery shopping",
			ClassifiedAt: baseDate.Add(time.Hour),
		},
		{
			Transaction: model.Transaction{
				ID:           "test-2",
				Date:         baseDate.Add(2 * 24 * time.Hour),
				MerchantName: "Shell Gas Station",
				Amount:       45.00,
			},
			Category:     "Transportation",
			Status:       model.StatusUserModified,
			Confidence:   1.0,
			Notes:        "",
			ClassifiedAt: baseDate.Add(2*24*time.Hour + time.Hour),
		},
		{
			Transaction: model.Transaction{
				ID:           "test-3",
				Date:         baseDate.Add(5 * 24 * time.Hour),
				MerchantName: "Amazon.com",
				Amount:       89.99,
			},
			Category:     "Shopping",
			Status:       model.StatusClassifiedByAI,
			Confidence:   0.88,
			Notes:        "Electronics purchase",
			ClassifiedAt: baseDate.Add(5*24*time.Hour + time.Hour),
		},
		{
			Transaction: model.Transaction{
				ID:           "test-4",
				Date:         baseDate.Add(10 * 24 * time.Hour),
				MerchantName: "Netflix",
				Amount:       15.99,
			},
			Category:     "Entertainment",
			Status:       model.StatusClassifiedByRule,
			Confidence:   1.0,
			Notes:        "Monthly subscription",
			ClassifiedAt: baseDate.Add(10*24*time.Hour + time.Hour),
		},
	}
}

func generateTestSummary(classifications []model.Classification) *service.ReportSummary {
	summary := &service.ReportSummary{
		DateRange: service.DateRange{
			Start: classifications[0].Transaction.Date,
			End:   classifications[len(classifications)-1].Transaction.Date,
		},
		ByCategory:   make(map[string]service.CategorySummary),
		ClassifiedBy: make(map[model.ClassificationStatus]int),
		TotalAmount:  0,
	}

	// Calculate totals
	for _, c := range classifications {
		summary.TotalAmount += c.Transaction.Amount

		// Update category summary
		catSum := summary.ByCategory[c.Category]
		catSum.Count++
		catSum.Amount += c.Transaction.Amount
		summary.ByCategory[c.Category] = catSum

		// Update classification status counts
		summary.ClassifiedBy[c.Status]++
	}

	return summary
}

func generateLargeTestDataset(count int) []model.Classification {
	classifications := make([]model.Classification, count)
	baseDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	categories := []string{"Groceries", "Transportation", "Shopping", "Entertainment", "Dining", "Healthcare", "Utilities"}
	merchants := []string{
		"Whole Foods", "Trader Joe's", "Shell", "Chevron", "Amazon", "Target",
		"Netflix", "Spotify", "Restaurant A", "Restaurant B", "CVS", "Walgreens",
		"Electric Company", "Water Company",
	}
	statuses := []model.ClassificationStatus{
		model.StatusClassifiedByAI,
		model.StatusUserModified,
		model.StatusClassifiedByRule,
	}

	for i := 0; i < count; i++ {
		classifications[i] = model.Classification{
			Transaction: model.Transaction{
				ID:           fmt.Sprintf("test-%d", i),
				Date:         baseDate.Add(time.Duration(i) * time.Hour),
				MerchantName: merchants[i%len(merchants)],
				Amount:       float64(20+i%200) + 0.99,
			},
			Category:     categories[i%len(categories)],
			Status:       statuses[i%len(statuses)],
			Confidence:   0.80 + float64(i%20)/100,
			Notes:        fmt.Sprintf("Transaction %d", i),
			ClassifiedAt: baseDate.Add(time.Duration(i)*time.Hour + time.Minute),
		}
	}

	return classifications
}
