package sheets

import (
	"context"

	"google.golang.org/api/sheets/v4"
)

// writeBusinessRulesTab writes the business rules lookup table.
func (w *Writer) writeBusinessRulesTab(ctx context.Context, spreadsheetID string, rules []BusinessRuleLookupRow) error {
	// Prepare values
	values := [][]any{
		// Header row
		{"Vendor Pattern", "Category", "Business %", "Notes"},
	}

	// Add rule rows
	for _, rule := range rules {
		values = append(values, []any{
			rule.VendorPattern,
			rule.Category,
			rule.BusinessPct,
			rule.Notes,
		})
	}

	// Write to sheet
	valueRange := &sheets.ValueRange{
		Values: values,
	}

	rangeStr := "Business Rules!A1"
	_, err := w.service.Spreadsheets.Values.Update(spreadsheetID, rangeStr, valueRange).
		ValueInputOption("USER_ENTERED").
		Context(ctx).
		Do()

	return err
}

// formatBusinessRulesTab formats the Business Rules tab.
func (w *Writer) formatBusinessRulesTab(sheetID int64) []*sheets.Request {
	requests := []*sheets.Request{
		// Bold header row
		{
			RepeatCell: &sheets.RepeatCellRequest{
				Range: &sheets.GridRange{
					SheetId:       sheetID,
					StartRowIndex: 0,
					EndRowIndex:   1,
				},
				Cell: &sheets.CellData{
					UserEnteredFormat: &sheets.CellFormat{
						TextFormat: &sheets.TextFormat{
							Bold: true,
						},
						BackgroundColor: &sheets.Color{
							Red:   0.9,
							Green: 0.9,
							Blue:  0.9,
							Alpha: 1.0,
						},
					},
				},
				Fields: "userEnteredFormat.textFormat,userEnteredFormat.backgroundColor",
			},
		},
		// Format business % as percentage
		{
			RepeatCell: &sheets.RepeatCellRequest{
				Range: &sheets.GridRange{
					SheetId:          sheetID,
					StartRowIndex:    1,
					EndRowIndex:      1000,
					StartColumnIndex: 2,
					EndColumnIndex:   3,
				},
				Cell: &sheets.CellData{
					UserEnteredFormat: &sheets.CellFormat{
						NumberFormat: &sheets.NumberFormat{
							Type:    "PERCENT",
							Pattern: "0%",
						},
					},
				},
				Fields: "userEnteredFormat.numberFormat",
			},
		},
	}

	return requests
}
