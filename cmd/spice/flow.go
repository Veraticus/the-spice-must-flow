package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/joshsymonds/the-spice-must-flow/internal/cli"
	"github.com/joshsymonds/the-spice-must-flow/internal/model"
	"github.com/joshsymonds/the-spice-must-flow/internal/service"
	"github.com/joshsymonds/the-spice-must-flow/internal/sheets"
	"github.com/joshsymonds/the-spice-must-flow/internal/storage"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func flowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "flow",
		Short: "View spending flow reports",
		Long: `Analyze and visualize your financial flow with category breakdowns.
		
This command generates reports showing where your money flows,
with options to export to Google Sheets.`,
		RunE: runFlow,
	}

	// Flags
	cmd.Flags().IntP("year", "y", time.Now().Year(), "Year to analyze")
	cmd.Flags().StringP("month", "m", "", "Specific month to analyze (format: 2024-01)")
	cmd.Flags().Bool("export", false, "Export to Google Sheets")
	cmd.Flags().String("format", "table", "Output format (table, json, csv)")

	// Bind to viper
	_ = viper.BindPFlag("flow.year", cmd.Flags().Lookup("year"))
	_ = viper.BindPFlag("flow.month", cmd.Flags().Lookup("month"))
	_ = viper.BindPFlag("flow.export", cmd.Flags().Lookup("export"))
	_ = viper.BindPFlag("flow.format", cmd.Flags().Lookup("format"))

	return cmd
}

func runFlow(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	year := viper.GetInt("flow.year")
	month := viper.GetString("flow.month")
	export := viper.GetBool("flow.export")
	format := viper.GetString("flow.format")

	slog.Info(cli.FormatTitle("Analyzing your financial flow..."))

	// Calculate date range
	var start, end time.Time
	if month != "" {
		// Parse specific month
		parsed, err := time.Parse("2006-01", month)
		if err != nil {
			return fmt.Errorf("invalid month format '%s', expected YYYY-MM: %w", month, err)
		}
		start = parsed
		end = parsed.AddDate(0, 1, -1).Add(23*time.Hour + 59*time.Minute + 59*time.Second)
	} else {
		// Full year
		start = time.Date(year, 1, 1, 0, 0, 0, 0, time.Local)
		end = time.Date(year, 12, 31, 23, 59, 59, 999999999, time.Local)
	}

	// Initialize storage
	storageService, err := initStorage(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}

	// Fetch classifications
	classifications, err := storageService.GetClassificationsByDateRange(ctx, start, end)
	if err != nil {
		return fmt.Errorf("failed to retrieve classifications: %w", err)
	}

	// Check data coverage and classification status if exporting
	if export {
		// Get unclassified transactions to check completeness
		unclassifiedTxns, err := storageService.GetTransactionsToClassify(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to check for unclassified transactions: %w", err)
		}
		
		// Filter unclassified transactions to our date range
		var unclassifiedInRange []model.Transaction
		for _, tx := range unclassifiedTxns {
			if !tx.Date.Before(start) && !tx.Date.After(end) {
				unclassifiedInRange = append(unclassifiedInRange, tx)
			}
		}
		
		// Check if we have unclassified transactions
		if len(unclassifiedInRange) > 0 {
			var exampleMerchants []string
			for i, tx := range unclassifiedInRange {
				if i < 3 && tx.MerchantName != "" {
					exampleMerchants = append(exampleMerchants, tx.MerchantName)
				}
			}
			
			errorMsg := fmt.Sprintf("cannot export: %d transactions are not classified", len(unclassifiedInRange))
			if len(exampleMerchants) > 0 {
				errorMsg += fmt.Sprintf(" (e.g., %s)", strings.Join(exampleMerchants, ", "))
			}
			errorMsg += "\nRun 'spice classify' to categorize all transactions first"
			return fmt.Errorf(errorMsg)
		}
		
		// For full year exports, validate we have adequate data coverage
		// Use classifications as a proxy for transaction coverage
		if month == "" {
			if err := validateDataCoverageFromClassifications(classifications, start, end); err != nil {
				return err
			}
		}
	}

	// Generate summary
	summary := generateReportSummary(classifications, start, end)

	// Display period
	period := fmt.Sprintf("%d", year)
	if month != "" {
		period = month
	}

	// Build report content
	content := formatReportContent(classifications, summary)

	// Display styled box
	slog.Info(cli.RenderBox(fmt.Sprintf("%s Financial Flow", period), content))

	// Handle export to Google Sheets
	if export {
		if err := exportToSheets(ctx, classifications, summary); err != nil {
			return fmt.Errorf("failed to export to Google Sheets: %w", err)
		}
		slog.Info(cli.FormatSuccess("Successfully exported to Google Sheets!"))
	}

	// Handle other formats
	if format != "table" && !export {
		slog.Warn(cli.FormatWarning(fmt.Sprintf("Output format '%s' not yet implemented", format)))
	}

	return nil
}

func initStorage(ctx context.Context) (service.Storage, error) {
	// Get database path from config
	dbPath := viper.GetString("storage.database_path")
	if dbPath == "" {
		dbPath = "$HOME/.local/share/spice/spice.db"
	}

	// Expand environment variables
	dbPath = os.ExpandEnv(dbPath)

	// Initialize storage
	store, err := storage.NewSQLiteStorage(dbPath)
	if err != nil {
		return nil, err
	}

	// Run migrations
	if err := store.Migrate(ctx); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return store, nil
}

func generateReportSummary(classifications []model.Classification, start, end time.Time) *service.ReportSummary {
	summary := &service.ReportSummary{
		DateRange: service.DateRange{
			Start: start,
			End:   end,
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

func formatReportContent(classifications []model.Classification, summary *service.ReportSummary) string {
	if len(classifications) == 0 {
		return `No data available yet.
Run 'spice classify' to categorize transactions first.`
	}

	content := fmt.Sprintf(`Total outflow: $%.2f
Transactions: %d
Categories: %d

Top Categories:`, summary.TotalAmount, len(classifications), len(summary.ByCategory))

	// Sort categories by amount
	type catAmount struct {
		name   string
		amount float64
		count  int
	}
	categories := make([]catAmount, 0, len(summary.ByCategory))
	for cat, sum := range summary.ByCategory {
		categories = append(categories, catAmount{cat, sum.Amount, sum.Count})
	}

	// Sort by amount descending
	for i := 0; i < len(categories)-1; i++ {
		for j := i + 1; j < len(categories); j++ {
			if categories[j].amount > categories[i].amount {
				categories[i], categories[j] = categories[j], categories[i]
			}
		}
	}

	// Show top 5 categories
	for i := 0; i < len(categories) && i < 5; i++ {
		cat := categories[i]
		content += fmt.Sprintf("\n  %-20s $%10.2f (%d transactions)", cat.name, cat.amount, cat.count)
	}

	return content
}

func exportToSheets(ctx context.Context, classifications []model.Classification, summary *service.ReportSummary) error {
	// Load Google Sheets config from environment
	sheetsConfig := sheets.DefaultConfig()
	if err := sheetsConfig.LoadFromEnv(); err != nil {
		return fmt.Errorf("failed to load Google Sheets config: %w", err)
	}

	// Override spreadsheet name if specified
	reportName := viper.GetString("sheets.spreadsheet_name")
	if reportName != "" {
		sheetsConfig.SpreadsheetName = reportName
	}

	// Create sheets writer
	writer, err := sheets.NewWriter(ctx, sheetsConfig, slog.Default())
	if err != nil {
		return fmt.Errorf("failed to create sheets writer: %w", err)
	}

	// Write the report
	if err := writer.Write(ctx, classifications, summary); err != nil {
		return fmt.Errorf("failed to write report: %w", err)
	}

	return nil
}

// validateDataCoverageFromClassifications ensures we have sufficient transaction data for the requested period
// Note: This uses classifications as a proxy for transaction coverage. The assumption is that
// if we have classified transactions, we have imported data for that period.
func validateDataCoverageFromClassifications(classifications []model.Classification, start, end time.Time) error {
	if len(classifications) == 0 {
		return fmt.Errorf("no transaction data found for %d", start.Year())
	}

	// Sort classifications by date
	sortedClassifications := make([]model.Classification, len(classifications))
	copy(sortedClassifications, classifications)
	for i := 0; i < len(sortedClassifications)-1; i++ {
		for j := i + 1; j < len(sortedClassifications); j++ {
			if sortedClassifications[j].Transaction.Date.Before(sortedClassifications[i].Transaction.Date) {
				sortedClassifications[i], sortedClassifications[j] = sortedClassifications[j], sortedClassifications[i]
			}
		}
	}

	// Find gaps of 30+ days
	var gaps []string
	
	// Check gap at start
	firstTransaction := sortedClassifications[0].Transaction.Date
	startGap := int(firstTransaction.Sub(start).Hours() / 24)
	if startGap >= 30 {
		gaps = append(gaps, fmt.Sprintf("%s to %s (%d days)", 
			start.Format("Jan 2, 2006"),
			firstTransaction.AddDate(0, 0, -1).Format("Jan 2, 2006"),
			startGap))
	}

	// Check gaps between transactions
	for i := 0; i < len(sortedClassifications)-1; i++ {
		current := sortedClassifications[i].Transaction.Date
		next := sortedClassifications[i+1].Transaction.Date
		
		// Calculate gap between transactions (exclusive of transaction dates)
		gapStart := current.AddDate(0, 0, 1)
		gapEnd := next.AddDate(0, 0, -1)
		gapDays := int(gapEnd.Sub(gapStart).Hours()/24) + 1
		
		if gapDays >= 30 {
			gaps = append(gaps, fmt.Sprintf("%s to %s (%d days)",
				gapStart.Format("Jan 2, 2006"),
				gapEnd.Format("Jan 2, 2006"),
				gapDays))
		}
	}

	// Check gap at end
	lastTransaction := sortedClassifications[len(sortedClassifications)-1].Transaction.Date
	endGap := int(end.Sub(lastTransaction).Hours() / 24)
	if endGap >= 30 {
		gaps = append(gaps, fmt.Sprintf("%s to %s (%d days)",
			lastTransaction.AddDate(0, 0, 1).Format("Jan 2, 2006"),
			end.Format("Jan 2, 2006"),
			endGap))
	}

	// Display warnings for gaps
	if len(gaps) > 0 {
		slog.Warn(cli.FormatWarning("Data gaps detected in the requested export period:"))
		for _, gap := range gaps {
			slog.Warn(cli.FormatWarning(fmt.Sprintf("  • Missing data: %s", gap)))
		}
		slog.Warn(cli.FormatWarning("This could mean no transactions occurred or data hasn't been imported."))
	}

	// Calculate overall coverage for info
	actualStart := sortedClassifications[0].Transaction.Date
	actualEnd := sortedClassifications[len(sortedClassifications)-1].Transaction.Date
	requiredDays := int(end.Sub(start).Hours() / 24) + 1
	actualDays := int(actualEnd.Sub(actualStart).Hours() / 24) + 1
	coveragePercent := float64(actualDays) / float64(requiredDays) * 100

	slog.Info(cli.FormatInfo(fmt.Sprintf("Data coverage: %s to %s (%.0f%% of requested period)",
		actualStart.Format("Jan 2"),
		actualEnd.Format("Jan 2"),
		coveragePercent)))

	return nil
}
