package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/cli"
	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/service"
	"github.com/Veraticus/the-spice-must-flow/internal/sheets"
	"github.com/Veraticus/the-spice-must-flow/internal/storage"

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

	fmt.Println(cli.FormatTitle("Analyzing your financial flow...")) //nolint:forbidigo // User-facing output

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

	// Check data coverage and classification status if exporting
	if export {
		// Get unclassified transactions to check completeness
		unclassifiedTxns, getErr := storageService.GetTransactionsToClassify(ctx, nil)
		if getErr != nil {
			return fmt.Errorf("failed to check for unclassified transactions: %w", getErr)
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

			errorMsg := fmt.Sprintf("Cannot export: %d transactions need to be categorized first", len(unclassifiedInRange))
			if len(exampleMerchants) > 0 {
				errorMsg += fmt.Sprintf(" (e.g., %s)", strings.Join(exampleMerchants, ", "))
			}
			errorMsg += "\nRun 'spice classify' to categorize all transactions first"
			return errors.New(errorMsg)
		}

		// For full year exports, validate we have adequate data coverage
		classifications, classErr := storageService.GetClassificationsByDateRange(ctx, start, end)
		if classErr != nil {
			return fmt.Errorf("failed to retrieve classifications: %w", classErr)
		}

		if month == "" {
			if validateErr := validateDataCoverageFromClassifications(classifications, start, end); validateErr != nil {
				return validateErr
			}
		}
	}

	// Generate cash flow summary
	summary, err := generateCashFlowSummary(ctx, storageService, start, end)
	if err != nil {
		return fmt.Errorf("failed to generate cash flow summary: %w", err)
	}

	// Display period
	period := fmt.Sprintf("%d", year)
	if month != "" {
		period = month
	}

	// Build report content
	content := formatCashFlowContent(summary)

	// Display styled box with the exact format from design doc
	fmt.Println(cli.RenderBox(fmt.Sprintf("💰 Cash Flow Summary - %s", period), content)) //nolint:forbidigo // User-facing output

	// Handle export to Google Sheets
	if export {
		if err := exportToSheets(ctx, storageService, summary); err != nil {
			return fmt.Errorf("failed to export to Google Sheets: %w", err)
		}
		fmt.Println(cli.FormatSuccess("Successfully exported to Google Sheets!")) //nolint:forbidigo // User-facing output
	}

	// Handle other formats
	if format != "table" && !export {
		slog.Warn(cli.FormatWarning(fmt.Sprintf("Output format '%s' not yet implemented", format)))
	}

	return nil
}

func initStorage(ctx context.Context) (service.Storage, error) {
	// Get database path from config
	dbPath := viper.GetString("database.path")
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

func generateCashFlowSummary(ctx context.Context, store service.Storage, start, end time.Time) (*service.CashFlowSummary, error) {
	// Get cash flow summary from storage
	summary, err := store.GetCashFlow(ctx, start, end)
	if err != nil {
		// Fallback to manual calculation if the method is not implemented yet
		return calculateCashFlowSummary(ctx, store, start, end)
	}
	return summary, nil
}

func calculateCashFlowSummary(ctx context.Context, store service.Storage, start, end time.Time) (*service.CashFlowSummary, error) {
	summary := &service.CashFlowSummary{
		DateRange: service.DateRange{
			Start: start,
			End:   end,
		},
		IncomeByCategory:   make(map[string]service.CategorySummary),
		ExpensesByCategory: make(map[string]service.CategorySummary),
		Insights:           []string{},
	}

	// Get all classifications for the period
	classifications, err := store.GetClassificationsByDateRange(ctx, start, end)
	if err != nil {
		return nil, err
	}

	// Calculate totals by direction
	for _, c := range classifications {
		switch c.Transaction.Direction {
		case model.DirectionIncome:
			summary.TotalIncome += c.Transaction.Amount
			catSum := summary.IncomeByCategory[c.Category]
			catSum.Count++
			catSum.Amount += c.Transaction.Amount
			summary.IncomeByCategory[c.Category] = catSum
		case model.DirectionExpense:
			summary.TotalExpenses += c.Transaction.Amount
			catSum := summary.ExpensesByCategory[c.Category]
			catSum.Count++
			catSum.Amount += c.Transaction.Amount
			summary.ExpensesByCategory[c.Category] = catSum
		case model.DirectionTransfer:
			summary.TransferTotal += c.Transaction.Amount
		}
	}

	// Calculate net cash flow
	summary.NetCashFlow = summary.TotalIncome - summary.TotalExpenses

	// Generate insights
	summary.Insights = generateInsights(summary)

	return summary, nil
}

func generateInsights(summary *service.CashFlowSummary) []string {
	insights := []string{}

	// Savings rate insight
	if summary.TotalIncome > 0 {
		savingsRate := (summary.NetCashFlow / summary.TotalIncome) * 100
		if savingsRate > 0 {
			insights = append(insights, fmt.Sprintf("You saved %.1f%% of your income", savingsRate))
		} else {
			insights = append(insights, fmt.Sprintf("You spent %.1f%% more than you earned", -savingsRate))
		}
	}

	// Largest expense category
	var largestCategory string
	var largestAmount float64
	for cat, sum := range summary.ExpensesByCategory {
		if sum.Amount > largestAmount {
			largestCategory = cat
			largestAmount = sum.Amount
		}
	}
	if largestCategory != "" && summary.TotalExpenses > 0 {
		percentage := (largestAmount / summary.TotalExpenses) * 100
		insights = append(insights, fmt.Sprintf("Largest expense: %s (%.1f%% of expenses)", largestCategory, percentage))
	}

	// Income trend (would need historical data for real comparison)
	if summary.TotalIncome > 0 {
		insights = append(insights, "Income tracking enabled - trends will appear next month")
	}

	return insights
}

func formatCashFlowContent(summary *service.CashFlowSummary) string {
	if summary.TotalIncome == 0 && summary.TotalExpenses == 0 && summary.TransferTotal == 0 {
		return `No data available yet.
Run 'spice classify' to categorize transactions first.`
	}

	var content strings.Builder

	// Income section
	content.WriteString(fmt.Sprintf("📈 INCOME                              $%.2f\n", summary.TotalIncome))
	if len(summary.IncomeByCategory) > 0 {
		// Sort income categories by amount
		incomeCategories := sortCategories(summary.IncomeByCategory)
		for i, cat := range incomeCategories {
			if i < 3 { // Show top 3
				content.WriteString(fmt.Sprintf("├─ %-30s $%.2f\n", cat.name, cat.amount))
			} else if i == 3 {
				// Sum remaining categories
				var remainingAmount float64
				remainingCount := 0
				for j := i; j < len(incomeCategories); j++ {
					remainingAmount += incomeCategories[j].amount
					remainingCount++
				}
				content.WriteString(fmt.Sprintf("└─ Other (%d categories)              $%.2f\n", remainingCount, remainingAmount))
				break
			}
		}
	}

	content.WriteString("\n")

	// Expenses section
	content.WriteString(fmt.Sprintf("📉 EXPENSES                            $%.2f\n", summary.TotalExpenses))
	if len(summary.ExpensesByCategory) > 0 {
		// Sort expense categories by amount
		expenseCategories := sortCategories(summary.ExpensesByCategory)
		for i, cat := range expenseCategories {
			if i < 3 { // Show top 3
				content.WriteString(fmt.Sprintf("├─ %-30s $%.2f\n", cat.name, cat.amount))
			} else if i == 3 {
				// Sum remaining categories
				var remainingAmount float64
				remainingCount := 0
				for j := i; j < len(expenseCategories); j++ {
					remainingAmount += expenseCategories[j].amount
					remainingCount++
				}
				content.WriteString(fmt.Sprintf("└─ Other (%d categories)              $%.2f\n", remainingCount, remainingAmount))
				break
			}
		}
	}

	// Transfers section (only if present)
	if summary.TransferTotal > 0 {
		content.WriteString("\n")
		content.WriteString(fmt.Sprintf("➡️  TRANSFERS (excluded)                 $%.2f\n", summary.TransferTotal))
	}

	// Net cash flow
	content.WriteString("\n")
	content.WriteString("═══════════════════════════════════════════════\n")
	netFlowSign := ""
	if summary.NetCashFlow >= 0 {
		netFlowSign = "+"
	}
	content.WriteString(fmt.Sprintf("✨ NET CASH FLOW                      %s$%.2f\n", netFlowSign, summary.NetCashFlow))

	// Insights
	if len(summary.Insights) > 0 {
		content.WriteString("\n")
		content.WriteString("📊 Insights:\n")
		for _, insight := range summary.Insights {
			content.WriteString(fmt.Sprintf("• %s\n", insight))
		}
	}

	return content.String()
}

type categoryAmount struct {
	name   string
	amount float64
	count  int
}

func sortCategories(categories map[string]service.CategorySummary) []categoryAmount {
	result := make([]categoryAmount, 0, len(categories))
	for name, summary := range categories {
		result = append(result, categoryAmount{name, summary.Amount, summary.Count})
	}

	// Sort by amount descending
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].amount > result[i].amount {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result
}

func exportToSheets(ctx context.Context, storage service.Storage, summary *service.CashFlowSummary) error {
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

	// Get all classifications for detailed export
	classifications, err := storage.GetClassificationsByDateRange(ctx, summary.DateRange.Start, summary.DateRange.End)
	if err != nil {
		return fmt.Errorf("failed to retrieve classifications for export: %w", err)
	}

	// Write the cash flow report
	if err := writer.WriteCashFlow(ctx, classifications, summary); err != nil {
		return fmt.Errorf("failed to write cash flow report: %w", err)
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
	requiredDays := int(end.Sub(start).Hours()/24) + 1
	actualDays := int(actualEnd.Sub(actualStart).Hours()/24) + 1
	coveragePercent := float64(actualDays) / float64(requiredDays) * 100

	fmt.Println(cli.FormatInfo(fmt.Sprintf("Data coverage: %s to %s (%.0f%% of requested period)", //nolint:forbidigo // User-facing output
		actualStart.Format("Jan 2"),
		actualEnd.Format("Jan 2"),
		coveragePercent)))

	return nil
}
