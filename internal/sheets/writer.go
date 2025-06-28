package sheets

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/common"
	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/service"
	"github.com/shopspring/decimal"
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
func (w *Writer) Write(ctx context.Context, classifications []model.Classification, summary *service.ReportSummary, categoryTypes map[string]model.CategoryType) error {
	w.logger.Info("starting report generation",
		"classifications", len(classifications),
		"date_range", fmt.Sprintf("%s to %s", summary.DateRange.Start.Format("2006-01-02"), summary.DateRange.End.Format("2006-01-02")))

	// Get or create spreadsheet with all required tabs
	spreadsheetID, err := w.getOrCreateSpreadsheetWithTabs(ctx)
	if err != nil {
		return fmt.Errorf("failed to get spreadsheet: %w", err)
	}

	// Aggregate data for all tabs
	tabData, err := w.aggregateData(classifications, summary, categoryTypes)
	if err != nil {
		return fmt.Errorf("failed to aggregate data: %w", err)
	}

	// Clear all tabs
	if clearErr := w.clearAllTabs(ctx, spreadsheetID); clearErr != nil {
		return fmt.Errorf("failed to clear tabs: %w", clearErr)
	}

	// Write data to each tab with retry
	retryOpts := service.RetryOptions{
		MaxAttempts:  w.config.RetryAttempts,
		InitialDelay: w.config.RetryDelay,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
	}

	err = common.WithRetry(ctx, func() error {
		return w.writeAllTabs(ctx, spreadsheetID, tabData)
	}, retryOpts)

	if err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}

	// Apply formatting if enabled
	if w.config.EnableFormatting {
		err = common.WithRetry(ctx, func() error {
			return w.applyFormattingToAllTabs(ctx, spreadsheetID)
		}, retryOpts)
		if err != nil {
			w.logger.Warn("failed to apply formatting", "error", err)
			// Don't fail the whole operation if formatting fails
		}
	}

	w.logger.Info("report generation completed",
		"spreadsheet_id", spreadsheetID,
		"total_income", tabData.TotalIncome,
		"total_expenses", tabData.TotalExpenses,
		"total_deductible", tabData.TotalDeductible)

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
func (w *Writer) getOrCreateSpreadsheetWithTabs(ctx context.Context) (string, error) {
	if w.config.SpreadsheetID != "" {
		// Verify the spreadsheet exists and is accessible
		spreadsheet, err := w.service.Spreadsheets.Get(w.config.SpreadsheetID).Context(ctx).Do()
		if err != nil {
			return "", fmt.Errorf("unable to access spreadsheet %s: %w", w.config.SpreadsheetID, err)
		}

		// Ensure all required tabs exist
		if err := w.ensureTabsExist(ctx, spreadsheet); err != nil {
			return "", fmt.Errorf("failed to ensure tabs exist: %w", err)
		}

		return w.config.SpreadsheetID, nil
	}

	// Create a new spreadsheet with all required tabs
	spreadsheet := &sheets.Spreadsheet{
		Properties: &sheets.SpreadsheetProperties{
			Title:    w.config.SpreadsheetName,
			TimeZone: w.config.TimeZone,
		},
		Sheets: []*sheets.Sheet{
			{Properties: &sheets.SheetProperties{Title: "Expenses", Index: 0}},
			{Properties: &sheets.SheetProperties{Title: "Income", Index: 1}},
			{Properties: &sheets.SheetProperties{Title: "Vendor Summary", Index: 2}},
			{Properties: &sheets.SheetProperties{Title: "Category Summary", Index: 3}},
			{Properties: &sheets.SheetProperties{Title: "Business Expenses", Index: 4}},
			{Properties: &sheets.SheetProperties{Title: "Monthly Flow", Index: 5}},
		},
	}

	created, err := w.service.Spreadsheets.Create(spreadsheet).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("unable to create spreadsheet: %w", err)
	}

	w.logger.Info("created new spreadsheet with 6 tabs",
		"id", created.SpreadsheetId,
		"url", created.SpreadsheetUrl)

	return created.SpreadsheetId, nil
}

// ensureTabsExist ensures all required tabs exist in the spreadsheet.
func (w *Writer) ensureTabsExist(ctx context.Context, spreadsheet *sheets.Spreadsheet) error {
	requiredTabs := []string{"Expenses", "Income", "Vendor Summary", "Category Summary", "Business Expenses", "Monthly Flow"}
	existingTabs := make(map[string]bool)

	for _, sheet := range spreadsheet.Sheets {
		existingTabs[sheet.Properties.Title] = true
	}

	var requests []*sheets.Request
	for i, tabName := range requiredTabs {
		if !existingTabs[tabName] {
			requests = append(requests, &sheets.Request{
				AddSheet: &sheets.AddSheetRequest{
					Properties: &sheets.SheetProperties{
						Title: tabName,
						Index: int64(i),
					},
				},
			})
		}
	}

	if len(requests) > 0 {
		batchUpdate := &sheets.BatchUpdateSpreadsheetRequest{
			Requests: requests,
		}
		_, err := w.service.Spreadsheets.BatchUpdate(spreadsheet.SpreadsheetId, batchUpdate).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("failed to add missing tabs: %w", err)
		}
	}

	return nil
}

// clearAllTabs clears data from all tabs.
func (w *Writer) clearAllTabs(ctx context.Context, spreadsheetID string) error {
	tabs := []string{"Expenses", "Income", "Vendor Summary", "Category Summary", "Business Expenses", "Monthly Flow"}

	for _, tab := range tabs {
		rangeStr := fmt.Sprintf("%s!A:Z", tab)
		_, err := w.service.Spreadsheets.Values.Clear(spreadsheetID, rangeStr, &sheets.ClearValuesRequest{}).Context(ctx).Do()
		if err != nil {
			w.logger.Warn("failed to clear tab", "tab", tab, "error", err)
			// Continue with other tabs even if one fails
		}
	}

	return nil
}

// aggregateData processes classifications into the TabData structure.
func (w *Writer) aggregateData(classifications []model.Classification, summary *service.ReportSummary, categoryTypes map[string]model.CategoryType) (*TabData, error) {

	data := &TabData{
		DateRange: DateRange{
			Start: summary.DateRange.Start,
			End:   summary.DateRange.End,
		},
		Expenses:         make([]ExpenseRow, 0),
		Income:           make([]IncomeRow, 0),
		VendorSummary:    make([]VendorSummaryRow, 0),
		CategorySummary:  make([]CategorySummaryRow, 0),
		BusinessExpenses: make([]BusinessExpenseRow, 0),
		MonthlyFlow:      make([]MonthlyFlowRow, 0),
	}

	// Maps for aggregation
	vendorMap := make(map[string]*VendorSummaryRow)
	categoryMap := make(map[string]*CategorySummaryRow)
	monthlyMap := make(map[string]*MonthlyFlowRow)

	// Process each classification
	for _, class := range classifications {
		amount := decimal.NewFromFloat(class.Transaction.Amount)

		// Determine if income or expense based on category type
		isIncome := categoryTypes[class.Category] == model.CategoryTypeIncome

		if isIncome {
			// Add to income tab
			data.Income = append(data.Income, IncomeRow{
				Date:     class.Transaction.Date,
				Amount:   amount,
				Source:   class.Transaction.MerchantName,
				Category: class.Category,
				Notes:    class.Notes,
			})
			data.TotalIncome = data.TotalIncome.Add(amount)
		} else {
			// Add to expenses tab
			businessPct := int(class.BusinessPercent)
			data.Expenses = append(data.Expenses, ExpenseRow{
				Date:        class.Transaction.Date,
				Amount:      amount,
				Vendor:      class.Transaction.MerchantName,
				Category:    class.Category,
				BusinessPct: businessPct,
				Notes:       class.Notes,
			})
			data.TotalExpenses = data.TotalExpenses.Add(amount)

			// Add to business expenses if applicable
			if businessPct > 0 {
				deductible := amount.Mul(decimal.NewFromFloat(float64(businessPct) / 100))
				data.BusinessExpenses = append(data.BusinessExpenses, BusinessExpenseRow{
					Date:             class.Transaction.Date,
					Vendor:           class.Transaction.MerchantName,
					Category:         class.Category,
					OriginalAmount:   amount,
					BusinessPct:      businessPct,
					DeductibleAmount: deductible,
					Notes:            class.Notes,
				})
				data.TotalDeductible = data.TotalDeductible.Add(deductible)
			}
		}

		// Update vendor summary
		vendorKey := class.Transaction.MerchantName
		if vendor, exists := vendorMap[vendorKey]; exists {
			vendor.TotalAmount = vendor.TotalAmount.Add(amount)
			vendor.TransactionCount++
		} else {
			vendorMap[vendorKey] = &VendorSummaryRow{
				VendorName:         vendorKey,
				AssociatedCategory: class.Category,
				TotalAmount:        amount,
				TransactionCount:   1,
			}
		}

		// Update category summary
		categoryKey := class.Category
		categoryType := "Expense"
		if isIncome {
			categoryType = "Income"
		}

		if cat, exists := categoryMap[categoryKey]; exists {
			cat.TotalAmount = cat.TotalAmount.Add(amount)
			cat.TransactionCount++
			// Update monthly amount
			monthIndex := class.Transaction.Date.Month() - 1
			cat.MonthlyAmounts[monthIndex] = cat.MonthlyAmounts[monthIndex].Add(amount)
		} else {
			monthlyAmounts := [12]decimal.Decimal{}
			monthIndex := class.Transaction.Date.Month() - 1
			monthlyAmounts[monthIndex] = amount

			categoryMap[categoryKey] = &CategorySummaryRow{
				CategoryName:     categoryKey,
				Type:             categoryType,
				TotalAmount:      amount,
				TransactionCount: 1,
				MonthlyAmounts:   monthlyAmounts,
			}
		}

		// Update monthly flow
		monthKey := class.Transaction.Date.Format("January 2006")
		if month, exists := monthlyMap[monthKey]; exists {
			if isIncome {
				month.TotalIncome = month.TotalIncome.Add(amount)
			} else {
				month.TotalExpenses = month.TotalExpenses.Add(amount)
			}
		} else {
			row := &MonthlyFlowRow{
				Month: monthKey,
			}
			if isIncome {
				row.TotalIncome = amount
			} else {
				row.TotalExpenses = amount
			}
			monthlyMap[monthKey] = row
		}
	}

	// Convert maps to slices
	for _, vendor := range vendorMap {
		data.VendorSummary = append(data.VendorSummary, *vendor)
	}

	for _, category := range categoryMap {
		// Calculate average business percentage for expense categories
		if category.Type == "Expense" && category.TransactionCount > 0 {
			totalBusinessPct := 0
			expenseCount := 0
			for _, expense := range data.Expenses {
				if expense.Category == category.CategoryName {
					totalBusinessPct += expense.BusinessPct
					expenseCount++
				}
			}
			if expenseCount > 0 {
				category.BusinessPct = totalBusinessPct / expenseCount
			}
		}
		data.CategorySummary = append(data.CategorySummary, *category)
	}

	// Create monthly flow with running balance
	months := make([]string, 0, len(monthlyMap))
	for month := range monthlyMap {
		months = append(months, month)
	}
	sort.Strings(months)

	runningBalance := decimal.Zero
	for _, month := range months {
		flow := monthlyMap[month]
		flow.NetFlow = flow.TotalIncome.Sub(flow.TotalExpenses)
		runningBalance = runningBalance.Add(flow.NetFlow)
		flow.RunningBalance = runningBalance
		data.MonthlyFlow = append(data.MonthlyFlow, *flow)
	}

	// Sort vendor summary by total amount descending
	sort.Slice(data.VendorSummary, func(i, j int) bool {
		return data.VendorSummary[i].TotalAmount.GreaterThan(data.VendorSummary[j].TotalAmount)
	})

	// Sort expenses and income by date descending
	sort.Slice(data.Expenses, func(i, j int) bool {
		return data.Expenses[i].Date.After(data.Expenses[j].Date)
	})

	sort.Slice(data.Income, func(i, j int) bool {
		return data.Income[i].Date.After(data.Income[j].Date)
	})

	// Sort business expenses by category, then date
	sort.Slice(data.BusinessExpenses, func(i, j int) bool {
		if data.BusinessExpenses[i].Category != data.BusinessExpenses[j].Category {
			return data.BusinessExpenses[i].Category < data.BusinessExpenses[j].Category
		}
		return data.BusinessExpenses[i].Date.After(data.BusinessExpenses[j].Date)
	})

	return data, nil
}

// writeAllTabs writes data to all tabs in the spreadsheet.
func (w *Writer) writeAllTabs(ctx context.Context, spreadsheetID string, data *TabData) error {
	// Write each tab
	if err := w.writeExpensesTab(ctx, spreadsheetID, data.Expenses); err != nil {
		return fmt.Errorf("failed to write expenses tab: %w", err)
	}

	if err := w.writeIncomeTab(ctx, spreadsheetID, data.Income); err != nil {
		return fmt.Errorf("failed to write income tab: %w", err)
	}

	if err := w.writeVendorSummaryTab(ctx, spreadsheetID, data.VendorSummary); err != nil {
		return fmt.Errorf("failed to write vendor summary tab: %w", err)
	}

	if err := w.writeCategorySummaryTab(ctx, spreadsheetID, data.CategorySummary); err != nil {
		return fmt.Errorf("failed to write category summary tab: %w", err)
	}

	if err := w.writeBusinessExpensesTab(ctx, spreadsheetID, data.BusinessExpenses); err != nil {
		return fmt.Errorf("failed to write business expenses tab: %w", err)
	}

	if err := w.writeMonthlyFlowTab(ctx, spreadsheetID, data.MonthlyFlow); err != nil {
		return fmt.Errorf("failed to write monthly flow tab: %w", err)
	}

	return nil
}

// applyFormattingToAllTabs applies formatting to all tabs.
func (w *Writer) applyFormattingToAllTabs(ctx context.Context, spreadsheetID string) error {
	// Get spreadsheet to get sheet IDs
	spreadsheet, err := w.service.Spreadsheets.Get(spreadsheetID).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to get spreadsheet: %w", err)
	}

	// Map tab names to sheet IDs
	sheetIDs := make(map[string]int64)
	for _, sheet := range spreadsheet.Sheets {
		sheetIDs[sheet.Properties.Title] = sheet.Properties.SheetId
	}

	// Prepare batch update request
	var requests []*sheets.Request

	// Format Expenses tab
	if sheetID, ok := sheetIDs["Expenses"]; ok {
		requests = append(requests, w.formatExpensesTab(sheetID)...)
	}

	// Format Income tab
	if sheetID, ok := sheetIDs["Income"]; ok {
		requests = append(requests, w.formatIncomeTab(sheetID)...)
	}

	// Format Vendor Summary tab
	if sheetID, ok := sheetIDs["Vendor Summary"]; ok {
		requests = append(requests, w.formatVendorSummaryTab(sheetID)...)
	}

	// Format Category Summary tab
	if sheetID, ok := sheetIDs["Category Summary"]; ok {
		requests = append(requests, w.formatCategorySummaryTab(sheetID)...)
	}

	// Format Business Expenses tab
	if sheetID, ok := sheetIDs["Business Expenses"]; ok {
		requests = append(requests, w.formatBusinessExpensesTab(sheetID)...)
	}

	// Format Monthly Flow tab
	if sheetID, ok := sheetIDs["Monthly Flow"]; ok {
		requests = append(requests, w.formatMonthlyFlowTab(sheetID)...)
	}

	// Apply all formatting in a single batch
	if len(requests) > 0 {
		batchUpdateRequest := &sheets.BatchUpdateSpreadsheetRequest{
			Requests: requests,
		}

		_, err = w.service.Spreadsheets.BatchUpdate(spreadsheetID, batchUpdateRequest).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("failed to apply formatting: %w", err)
		}
	}

	return nil
}

// writeExpensesTab writes expense data to the Expenses tab.
func (w *Writer) writeExpensesTab(ctx context.Context, spreadsheetID string, expenses []ExpenseRow) error {
	// Prepare values
	values := [][]any{
		// Header row
		{"Date", "Amount", "Vendor", "Category", "Business %", "Notes"},
	}

	// Add expense rows
	for _, expense := range expenses {
		values = append(values, []any{
			expense.Date.Format("2006-01-02"),
			expense.Amount.InexactFloat64(),
			expense.Vendor,
			expense.Category,
			expense.BusinessPct,
			expense.Notes,
		})
	}

	// Write to sheet
	valueRange := &sheets.ValueRange{
		Values: values,
	}

	rangeStr := "Expenses!A1"
	_, err := w.service.Spreadsheets.Values.Update(spreadsheetID, rangeStr, valueRange).
		ValueInputOption("USER_ENTERED").
		Context(ctx).
		Do()

	return err
}

// writeIncomeTab writes income data to the Income tab.
func (w *Writer) writeIncomeTab(ctx context.Context, spreadsheetID string, income []IncomeRow) error {
	// Prepare values
	values := [][]any{
		// Header row
		{"Date", "Amount", "Source", "Category", "Notes"},
	}

	// Add income rows
	for _, inc := range income {
		values = append(values, []any{
			inc.Date.Format("2006-01-02"),
			inc.Amount.InexactFloat64(),
			inc.Source,
			inc.Category,
			inc.Notes,
		})
	}

	// Write to sheet
	valueRange := &sheets.ValueRange{
		Values: values,
	}

	rangeStr := "Income!A1"
	_, err := w.service.Spreadsheets.Values.Update(spreadsheetID, rangeStr, valueRange).
		ValueInputOption("USER_ENTERED").
		Context(ctx).
		Do()

	return err
}

// writeVendorSummaryTab writes vendor summary data.
func (w *Writer) writeVendorSummaryTab(ctx context.Context, spreadsheetID string, vendors []VendorSummaryRow) error {
	// Prepare values
	values := [][]any{
		// Header row
		{"Vendor Name", "Category", "Total Amount", "Transaction Count"},
	}

	// Add vendor rows
	for _, vendor := range vendors {
		values = append(values, []any{
			vendor.VendorName,
			vendor.AssociatedCategory,
			vendor.TotalAmount.InexactFloat64(),
			vendor.TransactionCount,
		})
	}

	// Write to sheet
	valueRange := &sheets.ValueRange{
		Values: values,
	}

	rangeStr := "Vendor Summary!A1"
	_, err := w.service.Spreadsheets.Values.Update(spreadsheetID, rangeStr, valueRange).
		ValueInputOption("USER_ENTERED").
		Context(ctx).
		Do()

	return err
}

// writeCategorySummaryTab writes category summary data with monthly breakdown.
func (w *Writer) writeCategorySummaryTab(ctx context.Context, spreadsheetID string, categories []CategorySummaryRow) error {
	// Prepare header
	header := []any{
		"Category", "Type", "Total Amount", "Count", "Business %",
		"Jan", "Feb", "Mar", "Apr", "May", "Jun",
		"Jul", "Aug", "Sep", "Oct", "Nov", "Dec",
	}

	values := [][]any{header}

	// Separate income and expense categories
	var incomeCategories, expenseCategories []CategorySummaryRow
	for _, cat := range categories {
		if cat.Type == "Income" {
			incomeCategories = append(incomeCategories, cat)
		} else {
			expenseCategories = append(expenseCategories, cat)
		}
	}

	// Add section headers and data
	if len(incomeCategories) > 0 {
		values = append(values,
			[]any{}, // Empty row
			[]any{"INCOME CATEGORIES"})

		for _, cat := range incomeCategories {
			row := []any{
				cat.CategoryName,
				cat.Type,
				cat.TotalAmount.InexactFloat64(),
				cat.TransactionCount,
				"", // No business % for income
			}
			// Add monthly amounts
			for i := 0; i < 12; i++ {
				row = append(row, cat.MonthlyAmounts[i].InexactFloat64())
			}
			values = append(values, row)
		}
	}

	if len(expenseCategories) > 0 {
		values = append(values,
			[]any{}, // Empty row
			[]any{"EXPENSE CATEGORIES"})

		for _, cat := range expenseCategories {
			row := []any{
				cat.CategoryName,
				cat.Type,
				cat.TotalAmount.InexactFloat64(),
				cat.TransactionCount,
				cat.BusinessPct,
			}
			// Add monthly amounts
			for i := 0; i < 12; i++ {
				row = append(row, cat.MonthlyAmounts[i].InexactFloat64())
			}
			values = append(values, row)
		}
	}

	// Write to sheet
	valueRange := &sheets.ValueRange{
		Values: values,
	}

	rangeStr := "Category Summary!A1"
	_, err := w.service.Spreadsheets.Values.Update(spreadsheetID, rangeStr, valueRange).
		ValueInputOption("USER_ENTERED").
		Context(ctx).
		Do()

	return err
}

// writeBusinessExpensesTab writes business expense data with deductible calculations.
func (w *Writer) writeBusinessExpensesTab(ctx context.Context, spreadsheetID string, expenses []BusinessExpenseRow) error {
	// Prepare values
	values := [][]any{
		// Header row
		{"Date", "Vendor", "Category", "Original Amount", "Business %", "Deductible Amount", "Notes"},
	}

	// Group by category and add rows
	currentCategory := ""
	categoryTotal := decimal.Zero
	grandTotal := decimal.Zero

	for i, expense := range expenses {
		// Add category header and subtotal when category changes
		if expense.Category != currentCategory {
			if currentCategory != "" && !categoryTotal.IsZero() {
				// Add subtotal for previous category
				values = append(values,
					[]any{
						"", "", fmt.Sprintf("Subtotal - %s", currentCategory), "", "", categoryTotal.InexactFloat64(), "",
					},
					[]any{}) // Empty row
			}
			currentCategory = expense.Category
			categoryTotal = decimal.Zero

			// Add category header
			values = append(values, []any{
				fmt.Sprintf("Category: %s", expense.Category),
			})
		}

		// Add expense row
		values = append(values, []any{
			expense.Date.Format("2006-01-02"),
			expense.Vendor,
			expense.Category,
			expense.OriginalAmount.InexactFloat64(),
			expense.BusinessPct,
			expense.DeductibleAmount.InexactFloat64(),
			expense.Notes,
		})

		categoryTotal = categoryTotal.Add(expense.DeductibleAmount)
		grandTotal = grandTotal.Add(expense.DeductibleAmount)

		// Add final subtotal if this is the last expense
		if i == len(expenses)-1 && !categoryTotal.IsZero() {
			values = append(values, []any{
				"", "", fmt.Sprintf("Subtotal - %s", currentCategory), "", "", categoryTotal.InexactFloat64(), "",
			})
		}
	}

	// Add grand total
	if !grandTotal.IsZero() {
		values = append(values,
			[]any{}, // Empty row
			[]any{
				"", "", "GRAND TOTAL (Schedule C)", "", "", grandTotal.InexactFloat64(), "",
			})
	}

	// Write to sheet
	valueRange := &sheets.ValueRange{
		Values: values,
	}

	rangeStr := "Business Expenses!A1"
	_, err := w.service.Spreadsheets.Values.Update(spreadsheetID, rangeStr, valueRange).
		ValueInputOption("USER_ENTERED").
		Context(ctx).
		Do()

	return err
}

// writeMonthlyFlowTab writes monthly cash flow analysis.
func (w *Writer) writeMonthlyFlowTab(ctx context.Context, spreadsheetID string, monthlyFlow []MonthlyFlowRow) error {
	// Prepare values
	values := [][]any{
		// Header row
		{"Month", "Total Income", "Total Expenses", "Net Flow", "Running Balance"},
	}

	// Add monthly rows
	for _, month := range monthlyFlow {
		values = append(values, []any{
			month.Month,
			month.TotalIncome.InexactFloat64(),
			month.TotalExpenses.InexactFloat64(),
			month.NetFlow.InexactFloat64(),
			month.RunningBalance.InexactFloat64(),
		})
	}

	// Add yearly totals
	if len(monthlyFlow) > 0 {
		var totalIncome, totalExpenses decimal.Decimal
		for _, month := range monthlyFlow {
			totalIncome = totalIncome.Add(month.TotalIncome)
			totalExpenses = totalExpenses.Add(month.TotalExpenses)
		}
		netFlow := totalIncome.Sub(totalExpenses)

		values = append(values,
			[]any{}, // Empty row
			[]any{
				"YEARLY TOTALS",
				totalIncome.InexactFloat64(),
				totalExpenses.InexactFloat64(),
				netFlow.InexactFloat64(),
				"",
			})

		// Add averages
		monthCount := decimal.NewFromInt(int64(len(monthlyFlow)))
		values = append(values, []any{
			"MONTHLY AVERAGES",
			totalIncome.Div(monthCount).InexactFloat64(),
			totalExpenses.Div(monthCount).InexactFloat64(),
			netFlow.Div(monthCount).InexactFloat64(),
			"",
		})
	}

	// Write to sheet
	valueRange := &sheets.ValueRange{
		Values: values,
	}

	rangeStr := "Monthly Flow!A1"
	_, err := w.service.Spreadsheets.Values.Update(spreadsheetID, rangeStr, valueRange).
		ValueInputOption("USER_ENTERED").
		Context(ctx).
		Do()

	return err
}

// Helper functions for common formatting patterns

// formatCurrencyColumn formats a column as currency ($#,##0.00).
func formatCurrencyColumn(sheetID int64, column, startRow, endRow int) *sheets.Request {
	return &sheets.Request{
		RepeatCell: &sheets.RepeatCellRequest{
			Range: &sheets.GridRange{
				SheetId:          sheetID,
				StartRowIndex:    int64(startRow),
				EndRowIndex:      int64(endRow),
				StartColumnIndex: int64(column),
				EndColumnIndex:   int64(column + 1),
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
	}
}

// formatPercentageColumn formats a column as percentage.
func formatPercentageColumn(sheetID int64, column, startRow, endRow int) *sheets.Request {
	return &sheets.Request{
		RepeatCell: &sheets.RepeatCellRequest{
			Range: &sheets.GridRange{
				SheetId:          sheetID,
				StartRowIndex:    int64(startRow),
				EndRowIndex:      int64(endRow),
				StartColumnIndex: int64(column),
				EndColumnIndex:   int64(column + 1),
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
	}
}

// formatDateColumn formats a column as date (YYYY-MM-DD).
func formatDateColumn(sheetID int64, column, startRow, endRow int) *sheets.Request {
	return &sheets.Request{
		RepeatCell: &sheets.RepeatCellRequest{
			Range: &sheets.GridRange{
				SheetId:          sheetID,
				StartRowIndex:    int64(startRow),
				EndRowIndex:      int64(endRow),
				StartColumnIndex: int64(column),
				EndColumnIndex:   int64(column + 1),
			},
			Cell: &sheets.CellData{
				UserEnteredFormat: &sheets.CellFormat{
					NumberFormat: &sheets.NumberFormat{
						Type:    "DATE",
						Pattern: "yyyy-mm-dd",
					},
				},
			},
			Fields: "userEnteredFormat.numberFormat",
		},
	}
}

// formatHeaderRow formats a row as bold.
func formatHeaderRow(sheetID int64, row, startColumn, endColumn int) *sheets.Request {
	return &sheets.Request{
		RepeatCell: &sheets.RepeatCellRequest{
			Range: &sheets.GridRange{
				SheetId:          sheetID,
				StartRowIndex:    int64(row),
				EndRowIndex:      int64(row + 1),
				StartColumnIndex: int64(startColumn),
				EndColumnIndex:   int64(endColumn),
			},
			Cell: &sheets.CellData{
				UserEnteredFormat: &sheets.CellFormat{
					TextFormat: &sheets.TextFormat{
						Bold: true,
					},
				},
			},
			Fields: "userEnteredFormat.textFormat.bold",
		},
	}
}

// freezeRows freezes the specified number of rows at the top of the sheet.
func freezeRows(sheetID int64, rowCount int) *sheets.Request {
	return &sheets.Request{
		UpdateSheetProperties: &sheets.UpdateSheetPropertiesRequest{
			Properties: &sheets.SheetProperties{
				SheetId: sheetID,
				GridProperties: &sheets.GridProperties{
					FrozenRowCount: int64(rowCount),
				},
			},
			Fields: "gridProperties.frozenRowCount",
		},
	}
}

// addBorders adds borders around the specified range.
func addBorders(sheetID int64, startRow, endRow, startColumn, endColumn int) *sheets.Request {
	border := &sheets.Border{
		Style: "SOLID",
		Color: &sheets.Color{
			Red:   0.0,
			Green: 0.0,
			Blue:  0.0,
			Alpha: 1.0,
		},
	}

	return &sheets.Request{
		UpdateBorders: &sheets.UpdateBordersRequest{
			Range: &sheets.GridRange{
				SheetId:          sheetID,
				StartRowIndex:    int64(startRow),
				EndRowIndex:      int64(endRow),
				StartColumnIndex: int64(startColumn),
				EndColumnIndex:   int64(endColumn),
			},
			Top:    border,
			Bottom: border,
			Left:   border,
			Right:  border,
		},
	}
}

// autoResizeColumns auto-resizes columns to fit content.
func autoResizeColumns(sheetID int64, startColumn, endColumn int) *sheets.Request {
	return &sheets.Request{
		AutoResizeDimensions: &sheets.AutoResizeDimensionsRequest{
			Dimensions: &sheets.DimensionRange{
				SheetId:    sheetID,
				Dimension:  "COLUMNS",
				StartIndex: int64(startColumn),
				EndIndex:   int64(endColumn),
			},
		},
	}
}

// formatNumberColumn formats a column as a plain number.
func formatNumberColumn(sheetID int64, column, startRow, endRow int) *sheets.Request {
	return &sheets.Request{
		RepeatCell: &sheets.RepeatCellRequest{
			Range: &sheets.GridRange{
				SheetId:          sheetID,
				StartRowIndex:    int64(startRow),
				EndRowIndex:      int64(endRow),
				StartColumnIndex: int64(column),
				EndColumnIndex:   int64(column + 1),
			},
			Cell: &sheets.CellData{
				UserEnteredFormat: &sheets.CellFormat{
					NumberFormat: &sheets.NumberFormat{
						Type:    "NUMBER",
						Pattern: "#,##0",
					},
				},
			},
			Fields: "userEnteredFormat.numberFormat",
		},
	}
}

// Tab-specific formatting methods

// formatExpensesTab formats the Expenses tab.
func (w *Writer) formatExpensesTab(sheetID int64) []*sheets.Request {
	var requests []*sheets.Request

	// Bold header row
	requests = append(requests,
		formatHeaderRow(sheetID, 0, 0, 6),
		freezeRows(sheetID, 1),
		formatDateColumn(sheetID, 0, 1, 1000),
		formatCurrencyColumn(sheetID, 1, 1, 1000),
		formatPercentageColumn(sheetID, 4, 1, 1000),
		autoResizeColumns(sheetID, 0, 6),
		addBorders(sheetID, 0, 1000, 0, 6))

	return requests
}

// formatIncomeTab formats the Income tab.
func (w *Writer) formatIncomeTab(sheetID int64) []*sheets.Request {
	var requests []*sheets.Request

	// Bold header row
	requests = append(requests,
		formatHeaderRow(sheetID, 0, 0, 5),
		freezeRows(sheetID, 1),
		formatDateColumn(sheetID, 0, 1, 1000),
		formatCurrencyColumn(sheetID, 1, 1, 1000),
		autoResizeColumns(sheetID, 0, 5),
		addBorders(sheetID, 0, 1000, 0, 5))

	return requests
}

// formatVendorSummaryTab formats the Vendor Summary tab.
func (w *Writer) formatVendorSummaryTab(sheetID int64) []*sheets.Request {
	var requests []*sheets.Request

	// Bold header row
	requests = append(requests,
		formatHeaderRow(sheetID, 0, 0, 4),
		freezeRows(sheetID, 1),
		formatCurrencyColumn(sheetID, 2, 1, 1000),
		formatNumberColumn(sheetID, 3, 1, 1000),
		autoResizeColumns(sheetID, 0, 4),
		addBorders(sheetID, 0, 1000, 0, 4))

	return requests
}

// formatCategorySummaryTab formats the Category Summary tab.
func (w *Writer) formatCategorySummaryTab(sheetID int64) []*sheets.Request {
	var requests []*sheets.Request

	// Bold header row
	requests = append(requests,
		formatHeaderRow(sheetID, 0, 0, 18),
		freezeRows(sheetID, 1),
		formatCurrencyColumn(sheetID, 2, 1, 1000),
		formatPercentageColumn(sheetID, 4, 1, 1000))

	// Format monthly columns (F-Q) as currency
	for col := 5; col <= 16; col++ {
		requests = append(requests, formatCurrencyColumn(sheetID, col, 1, 1000))
	}

	// Bold section headers (INCOME CATEGORIES, EXPENSE CATEGORIES)
	// These would be in specific rows - would need to track during data write

	// Add conditional formatting for monthly amounts > average
	avgFormula := "=AVERAGE($F2:$Q2)"
	for col := 5; col <= 16; col++ {
		requests = append(requests, &sheets.Request{
			AddConditionalFormatRule: &sheets.AddConditionalFormatRuleRequest{
				Rule: &sheets.ConditionalFormatRule{
					Ranges: []*sheets.GridRange{
						{
							SheetId:          sheetID,
							StartRowIndex:    1,
							EndRowIndex:      1000,
							StartColumnIndex: int64(col),
							EndColumnIndex:   int64(col + 1),
						},
					},
					BooleanRule: &sheets.BooleanRule{
						Condition: &sheets.BooleanCondition{
							Type: "CUSTOM_FORMULA",
							Values: []*sheets.ConditionValue{
								{
									UserEnteredValue: fmt.Sprintf("=$%s2>%s", string(rune('F'+col-5)), avgFormula),
								},
							},
						},
						Format: &sheets.CellFormat{
							BackgroundColor: &sheets.Color{
								Red:   1.0,
								Green: 0.9,
								Blue:  0.9,
								Alpha: 1.0,
							},
						},
					},
				},
			},
		})
	}

	// Auto-resize all columns
	requests = append(requests,
		autoResizeColumns(sheetID, 0, 18),
		addBorders(sheetID, 0, 1000, 0, 18))

	return requests
}

// formatBusinessExpensesTab formats the Business Expenses tab.
func (w *Writer) formatBusinessExpensesTab(sheetID int64) []*sheets.Request {
	var requests []*sheets.Request

	// Bold header row
	requests = append(requests,
		formatHeaderRow(sheetID, 0, 0, 7),
		freezeRows(sheetID, 1),
		formatDateColumn(sheetID, 0, 1, 1000),
		formatCurrencyColumn(sheetID, 3, 1, 1000),
		formatPercentageColumn(sheetID, 4, 1, 1000),
		formatCurrencyColumn(sheetID, 5, 1, 1000),
		autoResizeColumns(sheetID, 0, 7),
		addBorders(sheetID, 0, 1000, 0, 7))

	return requests
}

// formatMonthlyFlowTab formats the Monthly Flow tab.
func (w *Writer) formatMonthlyFlowTab(sheetID int64) []*sheets.Request {
	var requests []*sheets.Request

	// Bold header row
	requests = append(requests,
		formatHeaderRow(sheetID, 0, 0, 5),
		freezeRows(sheetID, 1))

	// Format currency columns (B-E)
	for col := 1; col <= 4; col++ {
		requests = append(requests, formatCurrencyColumn(sheetID, col, 1, 1000))
	}

	// Add conditional formatting for Net Flow (column D) - red if negative, green if positive
	requests = append(requests,
		&sheets.Request{
			AddConditionalFormatRule: &sheets.AddConditionalFormatRuleRequest{
				Rule: &sheets.ConditionalFormatRule{
					Ranges: []*sheets.GridRange{
						{
							SheetId:          sheetID,
							StartRowIndex:    1,
							EndRowIndex:      1000,
							StartColumnIndex: 3,
							EndColumnIndex:   4,
						},
					},
					BooleanRule: &sheets.BooleanRule{
						Condition: &sheets.BooleanCondition{
							Type: "NUMBER_LESS",
							Values: []*sheets.ConditionValue{
								{
									UserEnteredValue: "0",
								},
							},
						},
						Format: &sheets.CellFormat{
							TextFormat: &sheets.TextFormat{
								ForegroundColor: &sheets.Color{
									Red:   0.8,
									Green: 0.0,
									Blue:  0.0,
									Alpha: 1.0,
								},
							},
						},
					},
				},
			},
		},
		&sheets.Request{
			AddConditionalFormatRule: &sheets.AddConditionalFormatRuleRequest{
				Rule: &sheets.ConditionalFormatRule{
					Ranges: []*sheets.GridRange{
						{
							SheetId:          sheetID,
							StartRowIndex:    1,
							EndRowIndex:      1000,
							StartColumnIndex: 3,
							EndColumnIndex:   4,
						},
					},
					BooleanRule: &sheets.BooleanRule{
						Condition: &sheets.BooleanCondition{
							Type: "NUMBER_GREATER",
							Values: []*sheets.ConditionValue{
								{
									UserEnteredValue: "0",
								},
							},
						},
						Format: &sheets.CellFormat{
							TextFormat: &sheets.TextFormat{
								ForegroundColor: &sheets.Color{
									Red:   0.0,
									Green: 0.6,
									Blue:  0.0,
									Alpha: 1.0,
								},
							},
						},
					},
				},
			},
		},
		autoResizeColumns(sheetID, 0, 5),
		addBorders(sheetID, 0, 1000, 0, 5))

	return requests
}
