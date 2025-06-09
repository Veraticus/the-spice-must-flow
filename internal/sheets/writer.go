package sheets

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"time"

	"github.com/joshsymonds/the-spice-must-flow/internal/common"
	"github.com/joshsymonds/the-spice-must-flow/internal/model"
	"github.com/joshsymonds/the-spice-must-flow/internal/service"
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

// Write implements the ReportWriter interface.
func (w *Writer) Write(ctx context.Context, classifications []model.Classification, summary *service.ReportSummary) error {
	w.logger.Info("starting report generation",
		"classifications", len(classifications),
		"date_range", fmt.Sprintf("%s to %s", summary.DateRange.Start.Format("2006-01-02"), summary.DateRange.End.Format("2006-01-02")))

	// Get or create spreadsheet
	spreadsheetID, err := w.getOrCreateSpreadsheet(ctx)
	if err != nil {
		return fmt.Errorf("failed to get spreadsheet: %w", err)
	}

	// Clear existing data
	if clearErr := w.clearSheet(ctx, spreadsheetID); clearErr != nil {
		return fmt.Errorf("failed to clear sheet: %w", clearErr)
	}

	// Prepare the data
	values := w.prepareReportData(classifications, summary)

	// Write data in batches with retry
	retryOpts := service.RetryOptions{
		MaxAttempts:  w.config.RetryAttempts,
		InitialDelay: w.config.RetryDelay,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
	}

	err = common.WithRetry(ctx, func() error {
		return w.writeData(ctx, spreadsheetID, values)
	}, retryOpts)

	if err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}

	// Apply formatting if enabled
	if w.config.EnableFormatting {
		err = common.WithRetry(ctx, func() error {
			return w.applyFormatting(ctx, spreadsheetID, len(values))
		}, retryOpts)
		if err != nil {
			w.logger.Warn("failed to apply formatting", "error", err)
			// Don't fail the whole operation if formatting fails
		}
	}

	w.logger.Info("report generation completed",
		"spreadsheet_id", spreadsheetID,
		"rows_written", len(values))

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

// clearSheet clears all data from the sheet.
func (w *Writer) clearSheet(ctx context.Context, spreadsheetID string) error {
	_, err := w.service.Spreadsheets.Values.Clear(spreadsheetID, "A:Z", &sheets.ClearValuesRequest{}).Context(ctx).Do()
	return err
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

// writeData writes the data to the spreadsheet.
func (w *Writer) writeData(ctx context.Context, spreadsheetID string, values [][]any) error {
	// Write in batches to avoid API limits
	for i := 0; i < len(values); i += w.config.BatchSize {
		end := i + w.config.BatchSize
		if end > len(values) {
			end = len(values)
		}

		batch := values[i:end]
		valueRange := &sheets.ValueRange{
			Values: batch,
		}

		rangeStr := fmt.Sprintf("A%d", i+1)
		_, err := w.service.Spreadsheets.Values.Update(spreadsheetID, rangeStr, valueRange).
			ValueInputOption("USER_ENTERED").
			Context(ctx).
			Do()

		if err != nil {
			return fmt.Errorf("failed to write batch starting at row %d: %w", i+1, err)
		}

		w.logger.Debug("wrote batch", "start_row", i+1, "rows", len(batch))
	}

	return nil
}

// applyFormatting applies formatting to the spreadsheet.
func (w *Writer) applyFormatting(ctx context.Context, spreadsheetID string, totalRows int) error {
	requests := []*sheets.Request{
		// Format header
		{
			RepeatCell: &sheets.RepeatCellRequest{
				Range: &sheets.GridRange{
					SheetId:          0,
					StartRowIndex:    0,
					EndRowIndex:      1,
					StartColumnIndex: 0,
					EndColumnIndex:   2,
				},
				Cell: &sheets.CellData{
					UserEnteredFormat: &sheets.CellFormat{
						TextFormat: &sheets.TextFormat{
							Bold:     true,
							FontSize: 16,
						},
					},
				},
				Fields: "userEnteredFormat.textFormat",
			},
		},
		// Format section headers
		{
			RepeatCell: &sheets.RepeatCellRequest{
				Range: &sheets.GridRange{
					SheetId:          0,
					StartRowIndex:    2,
					EndRowIndex:      int64(totalRows),
					StartColumnIndex: 0,
					EndColumnIndex:   1,
				},
				Cell: &sheets.CellData{
					UserEnteredFormat: &sheets.CellFormat{
						TextFormat: &sheets.TextFormat{
							Bold: true,
						},
					},
				},
				Fields: "userEnteredFormat.textFormat",
			},
		},
		// Format currency columns
		{
			RepeatCell: &sheets.RepeatCellRequest{
				Range: &sheets.GridRange{
					SheetId:          0,
					StartRowIndex:    0,
					EndRowIndex:      int64(totalRows),
					StartColumnIndex: 2,
					EndColumnIndex:   3,
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
					SheetId:    0,
					Dimension:  "COLUMNS",
					StartIndex: 0,
					EndIndex:   7,
				},
			},
		},
		// Freeze header rows
		{
			UpdateSheetProperties: &sheets.UpdateSheetPropertiesRequest{
				Properties: &sheets.SheetProperties{
					SheetId: 0,
					GridProperties: &sheets.GridProperties{
						FrozenRowCount: 2,
					},
				},
				Fields: "gridProperties.frozenRowCount",
			},
		},
	}

	batchUpdate := &sheets.BatchUpdateSpreadsheetRequest{
		Requests: requests,
	}

	_, err := w.service.Spreadsheets.BatchUpdate(spreadsheetID, batchUpdate).Context(ctx).Do()
	return err
}
