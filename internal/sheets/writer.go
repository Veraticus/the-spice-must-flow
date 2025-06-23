package sheets

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sort"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/service"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

// Writer implements the ReportWriter interface for Google Sheets.
type Writer struct {
	service *sheets.Service
	logger  *slog.Logger
	config  Config
}

// NewWriter creates a new Google Sheets report writer.
func NewWriter(ctx context.Context, config Config, logger *slog.Logger) (*Writer, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Create the Sheets service
	service, err := createSheetsService(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create sheets service: %w", err)
	}

	return &Writer{
		config:  config,
		service: service,
		logger:  logger,
	}, nil
}

// WriteCashFlow writes a cash flow report with income and expense separation.
func (w *Writer) WriteCashFlow(ctx context.Context, classifications []model.Classification, summary *service.CashFlowSummary) error {
	w.logger.Info("starting cash flow report generation",
		"classifications", len(classifications),
		"date_range", fmt.Sprintf("%s to %s", summary.DateRange.Start.Format("2006-01-02"), summary.DateRange.End.Format("2006-01-02")))

	// Get or create spreadsheet
	spreadsheetID, err := w.getOrCreateSpreadsheet(ctx)
	if err != nil {
		return fmt.Errorf("failed to get spreadsheet: %w", err)
	}

	// Separate classifications by direction
	var incomeClassifications []model.Classification
	var expenseClassifications []model.Classification
	for _, c := range classifications {
		switch c.Transaction.Direction {
		case model.DirectionIncome:
			incomeClassifications = append(incomeClassifications, c)
		case model.DirectionExpense:
			expenseClassifications = append(expenseClassifications, c)
			// Skip transfers - they're excluded from cash flow
		}
	}

	// Ensure all sheets exist
	if err := w.ensureSheets(ctx, spreadsheetID, []string{"Summary", "Income", "Expenses"}); err != nil {
		return fmt.Errorf("failed to ensure sheets: %w", err)
	}

	// Write summary sheet
	if err := w.writeSummarySheet(ctx, spreadsheetID, summary); err != nil {
		return fmt.Errorf("failed to write summary sheet: %w", err)
	}

	// Write income sheet
	if err := w.writeTransactionSheet(ctx, spreadsheetID, "Income", incomeClassifications); err != nil {
		return fmt.Errorf("failed to write income sheet: %w", err)
	}

	// Write expenses sheet
	if err := w.writeTransactionSheet(ctx, spreadsheetID, "Expenses", expenseClassifications); err != nil {
		return fmt.Errorf("failed to write expenses sheet: %w", err)
	}

	w.logger.Info("cash flow report generation completed",
		"spreadsheet_id", spreadsheetID,
		"income_count", len(incomeClassifications),
		"expense_count", len(expenseClassifications),
		"total_count", len(classifications))

	return nil
}

// createSheetsService creates a Google Sheets API service.
func createSheetsService(ctx context.Context, config Config) (*sheets.Service, error) {
	var client *oauth2.Config
	var tokenSource oauth2.TokenSource

	if config.ServiceAccountPath != "" {
		// Use service account authentication
		jsonKey, err := os.ReadFile(config.ServiceAccountPath)
		if err != nil {
			return nil, fmt.Errorf("unable to read service account key file: %w", err)
		}

		jwtConfig, err := google.JWTConfigFromJSON(jsonKey, sheets.SpreadsheetsScope)
		if err != nil {
			return nil, fmt.Errorf("unable to parse service account key: %w", err)
		}

		tokenSource = jwtConfig.TokenSource(ctx)
	} else {
		// Use OAuth2 authentication
		client = &oauth2.Config{
			ClientID:     config.ClientID,
			ClientSecret: config.ClientSecret,
			Endpoint:     google.Endpoint,
			Scopes:       []string{sheets.SpreadsheetsScope},
		}

		token := &oauth2.Token{
			RefreshToken: config.RefreshToken,
			TokenType:    "Bearer",
		}

		tokenSource = client.TokenSource(ctx, token)
	}

	httpClient := oauth2.NewClient(ctx, tokenSource)
	srv, err := sheets.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("unable to create sheets service: %w", err)
	}

	return srv, nil
}

// getOrCreateSpreadsheet gets an existing spreadsheet or creates a new one.
func (w *Writer) getOrCreateSpreadsheet(ctx context.Context) (string, error) {
	if w.config.SpreadsheetID != "" {
		// Verify the spreadsheet exists and is accessible
		_, err := w.service.Spreadsheets.Get(w.config.SpreadsheetID).Context(ctx).Do()
		if err != nil {
			return "", fmt.Errorf("unable to access spreadsheet %s: %w", w.config.SpreadsheetID, err)
		}
		return w.config.SpreadsheetID, nil
	}

	// Create a new spreadsheet
	spreadsheet := &sheets.Spreadsheet{
		Properties: &sheets.SpreadsheetProperties{
			Title:    w.config.SpreadsheetName,
			TimeZone: w.config.TimeZone,
		},
		Sheets: []*sheets.Sheet{
			{
				Properties: &sheets.SheetProperties{
					Title: "Transactions",
				},
			},
		},
	}

	created, err := w.service.Spreadsheets.Create(spreadsheet).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("unable to create spreadsheet: %w", err)
	}

	w.logger.Info("created new spreadsheet",
		"id", created.SpreadsheetId,
		"url", created.SpreadsheetUrl)

	return created.SpreadsheetId, nil
}

type categorySummary struct {
	name   string
	count  int
	amount float64
}

func sortCategoriesByAmount(categories map[string]service.CategorySummary) []categorySummary {
	result := make([]categorySummary, 0, len(categories))
	for name, summary := range categories {
		result = append(result, categorySummary{name, summary.Count, summary.Amount})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].amount > result[j].amount
	})
	return result
}

// prepareReportData prepares the data for the report.
func (w *Writer) prepareReportData(classifications []model.Classification, summary *service.ReportSummary) [][]any {
	// Pre-allocate with estimated capacity
	// Header(2) + Summary(4) + Category header(2) + categories + empty(1) + Classification(1+status count) + empty(2) + Transaction header(2) + transactions
	estimatedRows := 13 + len(summary.ByCategory) + len(summary.ClassifiedBy) + len(classifications)
	values := make([][]any, 0, estimatedRows)

	// Add header and summary in one append
	values = append(values,
		[]any{
			"Finance Report",
			fmt.Sprintf("%s - %s", summary.DateRange.Start.Format("Jan 2, 2006"), summary.DateRange.End.Format("Jan 2, 2006")),
		},
		[]any{}, // Empty row
		[]any{"Summary"},
		[]any{"Total Amount", summary.TotalAmount},
		[]any{"Total Transactions", len(classifications)},
		[]any{}, // Empty row
		[]any{"Category Breakdown"},
		[]any{"Category", "Count", "Amount"},
	)

	// Sort categories by amount (descending)
	categories := make([]string, 0, len(summary.ByCategory))
	for category := range summary.ByCategory {
		categories = append(categories, category)
	}
	sort.Slice(categories, func(i, j int) bool {
		return summary.ByCategory[categories[i]].Amount > summary.ByCategory[categories[j]].Amount
	})

	for _, category := range categories {
		catSummary := summary.ByCategory[category]
		values = append(values, []any{
			category,
			catSummary.Count,
			catSummary.Amount,
		})
	}

	// Add empty row and classification breakdown
	values = append(values,
		[]any{}, // Empty row
		[]any{"Classification Method"},
	)
	for status, count := range summary.ClassifiedBy {
		values = append(values, []any{
			string(status),
			count,
		})
	}

	// Add empty rows and transaction details header
	values = append(values,
		[]any{}, // Empty row
		[]any{}, // Empty row
		[]any{"Transaction Details"},
		[]any{
			"Date",
			"Merchant",
			"Amount",
			"Category",
			"Classification",
			"Confidence",
			"Notes",
		})

	// Sort classifications by date (newest first)
	sort.Slice(classifications, func(i, j int) bool {
		return classifications[i].Transaction.Date.After(classifications[j].Transaction.Date)
	})

	// Add each transaction
	for _, class := range classifications {
		values = append(values, []any{
			class.Transaction.Date.Format("2006-01-02"),
			class.Transaction.MerchantName,
			class.Transaction.Amount,
			class.Category,
			string(class.Status),
			fmt.Sprintf("%.2f", class.Confidence),
			class.Notes,
		})
	}

	return values
}

// ensureSheets ensures that all required sheets exist in the spreadsheet.
func (w *Writer) ensureSheets(ctx context.Context, spreadsheetID string, sheetNames []string) error {
	// Get current spreadsheet to check existing sheets
	spreadsheet, err := w.service.Spreadsheets.Get(spreadsheetID).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to get spreadsheet: %w", err)
	}

	// Create a map of existing sheet names
	existingSheets := make(map[string]bool)
	for _, sheet := range spreadsheet.Sheets {
		existingSheets[sheet.Properties.Title] = true
	}

	// Create requests for missing sheets
	var requests []*sheets.Request
	for i, name := range sheetNames {
		if !existingSheets[name] {
			requests = append(requests, &sheets.Request{
				AddSheet: &sheets.AddSheetRequest{
					Properties: &sheets.SheetProperties{
						Title: name,
						Index: int64(i),
					},
				},
			})
		}
	}

	// Execute batch update if there are sheets to create
	if len(requests) > 0 {
		batchUpdate := &sheets.BatchUpdateSpreadsheetRequest{
			Requests: requests,
		}
		_, err := w.service.Spreadsheets.BatchUpdate(spreadsheetID, batchUpdate).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("failed to create sheets: %w", err)
		}
	}

	return nil
}

// writeSummarySheet writes the cash flow summary to the Summary sheet.
func (w *Writer) writeSummarySheet(ctx context.Context, spreadsheetID string, summary *service.CashFlowSummary) error {
	// Prepare summary data
	values := [][]any{
		{"Cash Flow Summary", fmt.Sprintf("%s - %s", summary.DateRange.Start.Format("Jan 2, 2006"), summary.DateRange.End.Format("Jan 2, 2006"))},
		{}, // Empty row
		{"Total Income", summary.TotalIncome},
		{"Total Expenses", summary.TotalExpenses},
		{"Net Cash Flow", summary.NetCashFlow},
		{}, // Empty row
		{"Income by Category"},
		{"Category", "Count", "Amount"},
	}

	// Add income categories
	incomeCategories := sortCategoriesByAmount(summary.IncomeByCategory)
	for _, cat := range incomeCategories {
		values = append(values, []any{cat.name, cat.count, cat.amount})
	}

	values = append(values, []any{}) // Empty row
	values = append(values, []any{"Expenses by Category"})
	values = append(values, []any{"Category", "Count", "Amount"})

	// Add expense categories
	expenseCategories := sortCategoriesByAmount(summary.ExpensesByCategory)
	for _, cat := range expenseCategories {
		values = append(values, []any{cat.name, cat.count, cat.amount})
	}

	// Clear and write to Summary sheet
	clearRange := "Summary!A1:Z1000"
	_, err := w.service.Spreadsheets.Values.Clear(spreadsheetID, clearRange, &sheets.ClearValuesRequest{}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to clear summary sheet: %w", err)
	}

	valueRange := &sheets.ValueRange{
		Range:  "Summary!A1",
		Values: values,
	}

	_, err = w.service.Spreadsheets.Values.Update(spreadsheetID, "Summary!A1", valueRange).
		ValueInputOption("USER_ENTERED").
		Context(ctx).
		Do()

	return err
}

// writeTransactionSheet writes transaction data to a specific sheet.
func (w *Writer) writeTransactionSheet(ctx context.Context, spreadsheetID, sheetName string, classifications []model.Classification) error {
	// Prepare transaction data
	values := [][]any{
		{"Date", "Description", "Merchant", "Category", "Amount"},
	}

	// Sort by date descending
	sort.Slice(classifications, func(i, j int) bool {
		return classifications[i].Transaction.Date.After(classifications[j].Transaction.Date)
	})

	// Add transaction rows
	for _, c := range classifications {
		values = append(values, []any{
			c.Transaction.Date.Format("2006-01-02"),
			c.Transaction.Name,
			c.Transaction.MerchantName,
			c.Category,
			c.Transaction.Amount,
		})
	}

	// Add totals row
	if len(classifications) > 0 {
		total := 0.0
		for _, c := range classifications {
			total += c.Transaction.Amount
		}
		values = append(values, []any{})
		values = append(values, []any{"Total", "", "", "", total})
	}

	// Clear and write to sheet
	clearRange := fmt.Sprintf("%s!A1:Z10000", sheetName)
	_, err := w.service.Spreadsheets.Values.Clear(spreadsheetID, clearRange, &sheets.ClearValuesRequest{}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to clear %s sheet: %w", sheetName, err)
	}

	valueRange := &sheets.ValueRange{
		Range:  fmt.Sprintf("%s!A1", sheetName),
		Values: values,
	}

	_, err = w.service.Spreadsheets.Values.Update(spreadsheetID, fmt.Sprintf("%s!A1", sheetName), valueRange).
		ValueInputOption("USER_ENTERED").
		Context(ctx).
		Do()

	if err != nil {
		return fmt.Errorf("failed to write %s sheet: %w", sheetName, err)
	}

	// Apply formatting
	if w.config.EnableFormatting {
		if err := w.applyTransactionSheetFormatting(ctx, spreadsheetID, sheetName, len(values)); err != nil {
			w.logger.Warn("failed to apply formatting", "sheet", sheetName, "error", err)
		}
	}

	return nil
}

// applyTransactionSheetFormatting applies formatting to transaction sheets.
func (w *Writer) applyTransactionSheetFormatting(ctx context.Context, spreadsheetID, sheetName string, rowCount int) error {
	// Get sheet ID
	spreadsheet, err := w.service.Spreadsheets.Get(spreadsheetID).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to get spreadsheet: %w", err)
	}

	var sheetID int64
	for _, sheet := range spreadsheet.Sheets {
		if sheet.Properties.Title == sheetName {
			sheetID = sheet.Properties.SheetId
			break
		}
	}

	requests := []*sheets.Request{
		// Format header row
		{
			RepeatCell: &sheets.RepeatCellRequest{
				Range: &sheets.GridRange{
					SheetId:          sheetID,
					StartRowIndex:    0,
					EndRowIndex:      1,
					StartColumnIndex: 0,
					EndColumnIndex:   5,
				},
				Cell: &sheets.CellData{
					UserEnteredFormat: &sheets.CellFormat{
						BackgroundColor: &sheets.Color{
							Red:   0.2,
							Green: 0.2,
							Blue:  0.2,
						},
						TextFormat: &sheets.TextFormat{
							Bold: true,
							ForegroundColor: &sheets.Color{
								Red:   1,
								Green: 1,
								Blue:  1,
							},
						},
					},
				},
				Fields: "userEnteredFormat(backgroundColor,textFormat)",
			},
		},
		// Format currency column
		{
			RepeatCell: &sheets.RepeatCellRequest{
				Range: &sheets.GridRange{
					SheetId:          sheetID,
					StartRowIndex:    1,
					EndRowIndex:      int64(rowCount),
					StartColumnIndex: 4,
					EndColumnIndex:   5,
				},
				Cell: &sheets.CellData{
					UserEnteredFormat: &sheets.CellFormat{
						NumberFormat: &sheets.NumberFormat{
							Type:    "CURRENCY",
							Pattern: "$#,##0.00",
						},
					},
				},
				Fields: "userEnteredFormat.numberFormat",
			},
		},
		// Auto-resize columns
		{
			AutoResizeDimensions: &sheets.AutoResizeDimensionsRequest{
				Dimensions: &sheets.DimensionRange{
					SheetId:    sheetID,
					Dimension:  "COLUMNS",
					StartIndex: 0,
					EndIndex:   5,
				},
			},
		},
		// Freeze header row
		{
			UpdateSheetProperties: &sheets.UpdateSheetPropertiesRequest{
				Properties: &sheets.SheetProperties{
					SheetId: sheetID,
					GridProperties: &sheets.GridProperties{
						FrozenRowCount: 1,
					},
				},
				Fields: "gridProperties.frozenRowCount",
			},
		},
	}

	// Format total row if it exists
	if rowCount > 2 {
		requests = append(requests, &sheets.Request{
			RepeatCell: &sheets.RepeatCellRequest{
				Range: &sheets.GridRange{
					SheetId:          sheetID,
					StartRowIndex:    int64(rowCount - 1),
					EndRowIndex:      int64(rowCount),
					StartColumnIndex: 0,
					EndColumnIndex:   5,
				},
				Cell: &sheets.CellData{
					UserEnteredFormat: &sheets.CellFormat{
						TextFormat: &sheets.TextFormat{
							Bold: true,
						},
						Borders: &sheets.Borders{
							Top: &sheets.Border{
								Style: "SOLID",
								Width: 2,
							},
						},
					},
				},
				Fields: "userEnteredFormat(textFormat,borders)",
			},
		})
	}

	batchUpdate := &sheets.BatchUpdateSpreadsheetRequest{
		Requests: requests,
	}

	_, err = w.service.Spreadsheets.BatchUpdate(spreadsheetID, batchUpdate).Context(ctx).Do()
	return err
}
