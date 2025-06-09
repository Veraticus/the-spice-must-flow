package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
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
