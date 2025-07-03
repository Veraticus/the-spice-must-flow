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
func (w *Writer) Write(ctx context.Context, classifications []model.Classification, summary *service.ReportSummary, categories []model.Category) error {
	w.logger.Info("starting report generation",
		"classifications", len(classifications),
		"date_range", fmt.Sprintf("%s to %s", summary.DateRange.Start.Format("2006-01-02"), summary.DateRange.End.Format("2006-01-02")))

	// Build category type map from categories array
	categoryTypes := make(map[string]model.CategoryType)
	for _, cat := range categories {
		categoryTypes[cat.Name] = cat.Type
	}

	// Get or create spreadsheet with all required tabs
	spreadsheetID, err := w.getOrCreateSpreadsheetWithTabs(ctx)
	if err != nil {
		return fmt.Errorf("failed to get spreadsheet: %w", err)
	}

	// Aggregate data for all tabs
	tabData, err := w.aggregateData(classifications, summary, categories)
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
		"net_flow", tabData.TotalIncome.Sub(tabData.TotalExpenses))

	return nil
}

// createSheetsService creates the Google Sheets API service.
func createSheetsService(ctx context.Context, config Config) (*sheets.Service, error) {
	// If using service account
	if config.ServiceAccountPath != "" {
		credentialsJSON, err := os.ReadFile(config.ServiceAccountPath)
		if err != nil {
			return nil, fmt.Errorf("unable to read service account file: %w", err)
		}

		// Create service with service account
		srv, err := sheets.NewService(ctx, option.WithCredentialsJSON(credentialsJSON))
		if err != nil {
			return nil, fmt.Errorf("unable to create sheets service with service account: %w", err)
		}
		return srv, nil
	}

	// Otherwise use OAuth2
	oauthConfig := &oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		Endpoint:     google.Endpoint,
		Scopes:       []string{sheets.SpreadsheetsScope},
	}

	// Create token
	token := &oauth2.Token{
		RefreshToken: config.RefreshToken,
		TokenType:    "Bearer",
	}

	// Create client
	client := oauthConfig.Client(ctx, token)

	// Create Sheets service
	srv, err := sheets.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("unable to create sheets service: %w", err)
	}

	return srv, nil
}

// getOrCreateSpreadsheetWithTabs gets the existing spreadsheet or creates a new one with all required tabs.
func (w *Writer) getOrCreateSpreadsheetWithTabs(ctx context.Context) (string, error) {
	if w.config.SpreadsheetID != "" {
		// Use existing spreadsheet, but ensure all tabs exist
		spreadsheet, err := w.service.Spreadsheets.Get(w.config.SpreadsheetID).Context(ctx).Do()
		if err != nil {
			return "", fmt.Errorf("unable to get spreadsheet: %w", err)
		}

		if err := w.ensureTabsExist(ctx, spreadsheet); err != nil {
			return "", fmt.Errorf("failed to ensure tabs exist: %w", err)
		}

		return w.config.SpreadsheetID, nil
	}

	// Create new spreadsheet with all tabs
	return w.createSpreadsheetWithTabs(ctx)
}

// createSpreadsheetWithTabs creates a new spreadsheet with all required tabs.
func (w *Writer) createSpreadsheetWithTabs(ctx context.Context) (string, error) {
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
			{Properties: &sheets.SheetProperties{Title: "Vendor Lookup", Index: 6}},
			{Properties: &sheets.SheetProperties{Title: "Category Lookup", Index: 7}},
			{Properties: &sheets.SheetProperties{Title: "Business Rules", Index: 8}},
		},
	}

	created, err := w.service.Spreadsheets.Create(spreadsheet).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("unable to create spreadsheet: %w", err)
	}

	w.logger.Info("created new spreadsheet with 9 tabs",
		"id", created.SpreadsheetId,
		"url", created.SpreadsheetUrl)

	return created.SpreadsheetId, nil
}

// ensureTabsExist ensures all required tabs exist in the spreadsheet.
func (w *Writer) ensureTabsExist(ctx context.Context, spreadsheet *sheets.Spreadsheet) error {
	requiredTabs := []string{"Expenses", "Income", "Vendor Summary", "Category Summary", "Business Expenses", "Monthly Flow", "Vendor Lookup", "Category Lookup", "Business Rules"}
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
	tabs := []string{"Expenses", "Income", "Vendor Summary", "Category Summary", "Business Expenses", "Monthly Flow", "Vendor Lookup", "Category Lookup", "Business Rules"}

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
func (w *Writer) aggregateData(classifications []model.Classification, summary *service.ReportSummary, categories []model.Category) (*TabData, error) {

	data := &TabData{
		DateRange: DateRange{
			Start: summary.DateRange.Start,
			End:   summary.DateRange.End,
		},
		Expenses:            make([]ExpenseRow, 0),
		Income:              make([]IncomeRow, 0),
		VendorSummary:       make([]VendorSummaryRow, 0),
		CategorySummary:     make([]CategorySummaryRow, 0),
		BusinessExpenses:    make([]BusinessExpenseRow, 0),
		MonthlyFlow:         make([]MonthlyFlowRow, 0),
		VendorLookup:        make([]VendorLookupRow, 0),
		CategoryLookup:      make([]CategoryLookupRow, 0),
		BusinessRulesLookup: make([]BusinessRuleLookupRow, 0),
	}

	// Build category maps from categories array
	categoryTypes := make(map[string]model.CategoryType)
	categoryInfoMap := make(map[string]*model.Category)
	for i := range categories {
		categoryTypes[categories[i].Name] = categories[i].Type
		categoryInfoMap[categories[i].Name] = &categories[i]
	}

	// Maps for aggregation
	vendorSummaryMap := make(map[string]*VendorSummaryRow)
	categorySummaryMap := make(map[string]*CategorySummaryRow)
	monthlyMap := make(map[string]*MonthlyFlowRow)
	// Maps for lookup tables
	vendorLookupMap := make(map[string]string)   // vendor -> category
	categoryLookupMap := make(map[string]string) // category -> type

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
		if vendor, exists := vendorSummaryMap[vendorKey]; exists {
			vendor.TotalAmount = vendor.TotalAmount.Add(amount)
			vendor.TransactionCount++
		} else {
			vendorSummaryMap[vendorKey] = &VendorSummaryRow{
				VendorName:         vendorKey,
				AssociatedCategory: class.Category,
				TotalAmount:        amount,
				TransactionCount:   1,
			}
		}
		// Track vendor -> category mapping for lookup table
		vendorLookupMap[vendorKey] = class.Category

		// Update category summary
		categoryKey := class.Category
		categoryType := "Expense"
		if isIncome {
			categoryType = "Income"
		}

		if cat, exists := categorySummaryMap[categoryKey]; exists {
			cat.TotalAmount = cat.TotalAmount.Add(amount)
			cat.TransactionCount++
			// Update monthly amount
			monthIndex := class.Transaction.Date.Month() - 1
			cat.MonthlyAmounts[monthIndex] = cat.MonthlyAmounts[monthIndex].Add(amount)
		} else {
			monthlyAmounts := [12]decimal.Decimal{}
			monthIndex := class.Transaction.Date.Month() - 1
			monthlyAmounts[monthIndex] = amount

			categorySummaryMap[categoryKey] = &CategorySummaryRow{
				CategoryName:     categoryKey,
				Type:             categoryType,
				TotalAmount:      amount,
				TransactionCount: 1,
				MonthlyAmounts:   monthlyAmounts,
			}
		}
		// Track category -> type mapping for lookup table
		categoryLookupMap[categoryKey] = categoryType

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
	for _, vendor := range vendorSummaryMap {
		data.VendorSummary = append(data.VendorSummary, *vendor)
	}

	for _, category := range categorySummaryMap {
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

	// Build vendor lookup table from map
	for vendor, category := range vendorLookupMap {
		data.VendorLookup = append(data.VendorLookup, VendorLookupRow{
			VendorName: vendor,
			Category:   category,
		})
	}
	sort.Slice(data.VendorLookup, func(i, j int) bool {
		return data.VendorLookup[i].VendorName < data.VendorLookup[j].VendorName
	})

	// Build category lookup table - include ALL categories, not just used ones
	// First add all categories from the categories array
	for _, cat := range categories {
		data.CategoryLookup = append(data.CategoryLookup, CategoryLookupRow{
			CategoryName:       cat.Name,
			Type:               string(cat.Type),
			Description:        cat.Description,
			DefaultBusinessPct: cat.DefaultBusinessPercent,
		})
		// Make sure it's in the map for backward compatibility
		categoryLookupMap[cat.Name] = string(cat.Type)
	}

	// Then add any additional categories from classifications that weren't in the array
	for category, catType := range categoryLookupMap {
		found := false
		for _, existing := range data.CategoryLookup {
			if existing.CategoryName == category {
				found = true
				break
			}
		}
		if !found {
			lookupRow := CategoryLookupRow{
				CategoryName:       category,
				Type:               catType,
				Description:        "",
				DefaultBusinessPct: 0,
			}
			data.CategoryLookup = append(data.CategoryLookup, lookupRow)
		}
	}
	sort.Slice(data.CategoryLookup, func(i, j int) bool {
		return data.CategoryLookup[i].CategoryName < data.CategoryLookup[j].CategoryName
	})

	// Build business rules lookup from unique vendor/category/business% combinations
	businessRulesMap := make(map[string]BusinessRuleLookupRow)
	for _, expense := range data.Expenses {
		if expense.BusinessPct > 0 {
			key := fmt.Sprintf("%s:%s:%d", expense.Vendor, expense.Category, expense.BusinessPct)
			if _, exists := businessRulesMap[key]; !exists {
				businessRulesMap[key] = BusinessRuleLookupRow{
					VendorPattern: expense.Vendor,
					Category:      expense.Category,
					BusinessPct:   expense.BusinessPct,
					Notes:         "",
				}
			}
		}
	}

	// Convert to slice and sort
	for _, rule := range businessRulesMap {
		data.BusinessRulesLookup = append(data.BusinessRulesLookup, rule)
	}
	sort.Slice(data.BusinessRulesLookup, func(i, j int) bool {
		if data.BusinessRulesLookup[i].VendorPattern != data.BusinessRulesLookup[j].VendorPattern {
			return data.BusinessRulesLookup[i].VendorPattern < data.BusinessRulesLookup[j].VendorPattern
		}
		return data.BusinessRulesLookup[i].Category < data.BusinessRulesLookup[j].Category
	})

	return data, nil
}

// writeAllTabs writes data to all tabs in the spreadsheet.
func (w *Writer) writeAllTabs(ctx context.Context, spreadsheetID string, data *TabData) error {
	// Write lookup tables first (they need to exist for formulas to work)
	if err := w.writeVendorLookupTab(ctx, spreadsheetID, data.VendorLookup); err != nil {
		return fmt.Errorf("failed to write vendor lookup tab: %w", err)
	}

	if err := w.writeCategoryLookupTab(ctx, spreadsheetID, data.CategoryLookup); err != nil {
		return fmt.Errorf("failed to write category lookup tab: %w", err)
	}

	if err := w.writeBusinessRulesTab(ctx, spreadsheetID, data.BusinessRulesLookup); err != nil {
		return fmt.Errorf("failed to write business rules tab: %w", err)
	}

	// Write transaction tabs
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

	// Format Vendor Lookup tab
	if sheetID, ok := sheetIDs["Vendor Lookup"]; ok {
		requests = append(requests, w.formatVendorLookupTab(sheetID)...)
	}

	// Format Category Lookup tab
	if sheetID, ok := sheetIDs["Category Lookup"]; ok {
		requests = append(requests, w.formatCategoryLookupTab(sheetID)...)
	}

	// Format Business Rules tab
	if sheetID, ok := sheetIDs["Business Rules"]; ok {
		requests = append(requests, w.formatBusinessRulesTab(sheetID)...)
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

// writeExpensesTab writes expense data to the Expenses tab with formulas.
func (w *Writer) writeExpensesTab(ctx context.Context, spreadsheetID string, expenses []ExpenseRow) error {
	// Prepare values
	values := [][]any{
		// Header row
		{"Date", "Amount", "Vendor", "Category", "Business %", "Notes"},
	}

	// Add expense rows with formulas
	for i, expense := range expenses {
		row := i + 2 // Account for header row, 1-based indexing

		// Category formula using VLOOKUP to find category from vendor
		categoryFormula := fmt.Sprintf(`=IFERROR(VLOOKUP(C%d,'Vendor Lookup'!A:B,2,FALSE),"%s")`, row, expense.Category)

		// Business percentage formula with smart override support
		// First tries to find a specific rule in Business Rules table
		// If not found, uses VLOOKUP to get the category default from Category Lookup
		// We use the static category value from the expense data, not the formula result
		businessPctFormula := fmt.Sprintf(
			`=IFERROR(INDEX('Business Rules'!C:C,MATCH(1,(C%d='Business Rules'!A:A)*(D%d='Business Rules'!B:B),0)),IFERROR(VLOOKUP("%s",'Category Lookup'!A:D,4,FALSE)/100,%g))`,
			row, row, expense.Category, float64(expense.BusinessPct)/100,
		)

		values = append(values, []any{
			expense.Date.Format("2006-01-02"),
			expense.Amount.InexactFloat64(),
			expense.Vendor,
			categoryFormula,    // Use formula instead of static value
			businessPctFormula, // Use formula to lookup business %
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

// writeIncomeTab writes income data to the Income tab with formulas.
func (w *Writer) writeIncomeTab(ctx context.Context, spreadsheetID string, income []IncomeRow) error {
	// Prepare values
	values := [][]any{
		// Header row
		{"Date", "Amount", "Source", "Category", "Notes"},
	}

	// Add income rows with formulas
	for i, inc := range income {
		row := i + 2 // Account for header row, 1-based indexing

		// Category formula using VLOOKUP to find category from source/vendor
		categoryFormula := fmt.Sprintf(`=IFERROR(VLOOKUP(C%d,'Vendor Lookup'!A:B,2,FALSE),"%s")`, row, inc.Category)

		values = append(values, []any{
			inc.Date.Format("2006-01-02"),
			inc.Amount.InexactFloat64(),
			inc.Source,
			categoryFormula, // Use formula instead of static value
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

// writeVendorSummaryTab writes vendor summary data with formulas.
func (w *Writer) writeVendorSummaryTab(ctx context.Context, spreadsheetID string, vendors []VendorSummaryRow) error {
	// Prepare values
	values := [][]any{
		// Header row
		{"Vendor Name", "Category", "Total Amount", "Transaction Count"},
	}

	// Add vendor rows with formulas
	for i, vendor := range vendors {
		row := i + 2 // Account for header row, 1-based indexing

		// Category lookup from Vendor Lookup table
		categoryFormula := fmt.Sprintf(`=IFERROR(VLOOKUP(A%d,'Vendor Lookup'!A:B,2,FALSE),"")`, row)

		// Total amount formula - sum from both Expenses and Income sheets
		totalFormula := fmt.Sprintf(
			`=SUMIF(Expenses!C:C,A%d,Expenses!B:B)+SUMIF(Income!C:C,A%d,Income!B:B)`,
			row, row,
		)

		// Transaction count formula
		countFormula := fmt.Sprintf(
			`=COUNTIF(Expenses!C:C,A%d)+COUNTIF(Income!C:C,A%d)`,
			row, row,
		)

		values = append(values, []any{
			vendor.VendorName,
			categoryFormula,
			totalFormula,
			countFormula,
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

// writeCategorySummaryTab writes category summary data with formulas.
func (w *Writer) writeCategorySummaryTab(ctx context.Context, spreadsheetID string, categories []CategorySummaryRow) error {
	// Prepare header
	header := []any{
		"Category", "Type", "Total Amount", "Count", "Avg Business % (Edit in Category Lookup)",
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

	currentRow := 2 // Track current row for formulas

	// Add section headers and data
	if len(incomeCategories) > 0 {
		values = append(values,
			[]any{}, // Empty row
			[]any{"INCOME CATEGORIES"})
		currentRow += 2

		for _, cat := range incomeCategories {
			// Type lookup from Category Lookup table
			typeFormula := fmt.Sprintf(`=IFERROR(VLOOKUP(A%d,'Category Lookup'!A:B,2,FALSE),"Income")`, currentRow)

			// Total amount formula
			totalFormula := fmt.Sprintf(`=SUMIF(Income!D:D,A%d,Income!B:B)`, currentRow)

			// Count formula
			countFormula := fmt.Sprintf(`=COUNTIF(Income!D:D,A%d)`, currentRow)

			row := []any{
				cat.CategoryName,
				typeFormula,
				totalFormula,
				countFormula,
				"", // No business % for income
			}

			// Add monthly amount formulas
			for i := 1; i <= 12; i++ {
				// This is simplified - in reality we'd need the actual year from the date range
				monthFormula := fmt.Sprintf(
					`=SUMIFS(Income!B:B,Income!D:D,A%d,Income!A:A,">="&DATE(YEAR(TODAY()),%d,1),Income!A:A,"<"&DATE(YEAR(TODAY()),%d,1))`,
					currentRow, i, i+1,
				)
				row = append(row, monthFormula)
			}

			values = append(values, row)
			currentRow++
		}
	}

	if len(expenseCategories) > 0 {
		values = append(values,
			[]any{}, // Empty row
			[]any{"EXPENSE CATEGORIES"})
		currentRow += 2

		for _, cat := range expenseCategories {
			// Type lookup from Category Lookup table
			typeFormula := fmt.Sprintf(`=IFERROR(VLOOKUP(A%d,'Category Lookup'!A:B,2,FALSE),"Expense")`, currentRow)

			// Total amount formula
			totalFormula := fmt.Sprintf(`=SUMIF(Expenses!D:D,A%d,Expenses!B:B)`, currentRow)

			// Count formula
			countFormula := fmt.Sprintf(`=COUNTIF(Expenses!D:D,A%d)`, currentRow)

			// Average business percentage formula
			// The Expenses!E:E column already contains percentage values
			businessPctFormula := fmt.Sprintf(
				`=IFERROR(AVERAGEIF(Expenses!D:D,A%d,Expenses!E:E),0)`,
				currentRow,
			)

			row := []any{
				cat.CategoryName,
				typeFormula,
				totalFormula,
				countFormula,
				businessPctFormula,
			}

			// Add monthly amount formulas
			for i := 1; i <= 12; i++ {
				// This is simplified - in reality we'd need the actual year from the date range
				monthFormula := fmt.Sprintf(
					`=SUMIFS(Expenses!B:B,Expenses!D:D,A%d,Expenses!A:A,">="&DATE(YEAR(TODAY()),%d,1),Expenses!A:A,"<"&DATE(YEAR(TODAY()),%d,1))`,
					currentRow, i, i+1,
				)
				row = append(row, monthFormula)
			}

			values = append(values, row)
			currentRow++
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

// writeBusinessExpensesTab writes business expense data with category totals.
func (w *Writer) writeBusinessExpensesTab(ctx context.Context, spreadsheetID string, expenses []BusinessExpenseRow) error {
	// Prepare values
	values := [][]any{
		// Header row
		{"Date", "Vendor", "Category", "Amount", "Business %", "Deductible", "Notes"},
	}

	// Group by category and add subtotals
	currentCategory := ""
	categoryTotal := decimal.Zero
	grandTotal := decimal.Zero

	for i, expense := range expenses {
		// Add category header and subtotal from previous category
		if expense.Category != currentCategory {
			// Add subtotal for previous category if not the first
			if currentCategory != "" && !categoryTotal.IsZero() {
				values = append(values, []any{
					"", "", fmt.Sprintf("Subtotal - %s", currentCategory), "", "", categoryTotal.InexactFloat64(), "",
				})
			}

			// Add category header
			values = append(values,
				[]any{}, // Empty row
				[]any{fmt.Sprintf("CATEGORY: %s", expense.Category)})

			currentCategory = expense.Category
			categoryTotal = decimal.Zero
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

// formatExpensesTab formats the Expenses tab.
func (w *Writer) formatExpensesTab(sheetID int64) []*sheets.Request {
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
		// Format amount column as currency
		{
			RepeatCell: &sheets.RepeatCellRequest{
				Range: &sheets.GridRange{
					SheetId:          sheetID,
					StartRowIndex:    1,
					EndRowIndex:      1000,
					StartColumnIndex: 1,
					EndColumnIndex:   2,
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
		// Format business % as percentage
		{
			RepeatCell: &sheets.RepeatCellRequest{
				Range: &sheets.GridRange{
					SheetId:          sheetID,
					StartRowIndex:    1,
					EndRowIndex:      1000,
					StartColumnIndex: 4,
					EndColumnIndex:   5,
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

// formatIncomeTab formats the Income tab.
func (w *Writer) formatIncomeTab(sheetID int64) []*sheets.Request {
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
		// Format amount column as currency
		{
			RepeatCell: &sheets.RepeatCellRequest{
				Range: &sheets.GridRange{
					SheetId:          sheetID,
					StartRowIndex:    1,
					EndRowIndex:      1000,
					StartColumnIndex: 1,
					EndColumnIndex:   2,
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
	}

	return requests
}

// formatVendorSummaryTab formats the Vendor Summary tab.
func (w *Writer) formatVendorSummaryTab(sheetID int64) []*sheets.Request {
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
		// Format total amount column as currency
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
							Type:    "CURRENCY",
							Pattern: "$#,##0.00",
						},
					},
				},
				Fields: "userEnteredFormat.numberFormat",
			},
		},
	}

	return requests
}

// formatCategorySummaryTab formats the Category Summary tab.
func (w *Writer) formatCategorySummaryTab(sheetID int64) []*sheets.Request {
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
		// Format amount columns as currency
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
							Type:    "CURRENCY",
							Pattern: "$#,##0.00",
						},
					},
				},
				Fields: "userEnteredFormat.numberFormat",
			},
		},
		// Format business % column as percentage
		{
			RepeatCell: &sheets.RepeatCellRequest{
				Range: &sheets.GridRange{
					SheetId:          sheetID,
					StartRowIndex:    1,
					EndRowIndex:      1000,
					StartColumnIndex: 4,
					EndColumnIndex:   5,
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
		// Format monthly columns as currency
		{
			RepeatCell: &sheets.RepeatCellRequest{
				Range: &sheets.GridRange{
					SheetId:          sheetID,
					StartRowIndex:    1,
					EndRowIndex:      1000,
					StartColumnIndex: 5,
					EndColumnIndex:   17,
				},
				Cell: &sheets.CellData{
					UserEnteredFormat: &sheets.CellFormat{
						NumberFormat: &sheets.NumberFormat{
							Type:    "CURRENCY",
							Pattern: "$#,##0",
						},
					},
				},
				Fields: "userEnteredFormat.numberFormat",
			},
		},
	}

	return requests
}

// formatBusinessExpensesTab formats the Business Expenses tab.
func (w *Writer) formatBusinessExpensesTab(sheetID int64) []*sheets.Request {
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
		// Format amount columns as currency
		{
			RepeatCell: &sheets.RepeatCellRequest{
				Range: &sheets.GridRange{
					SheetId:          sheetID,
					StartRowIndex:    1,
					EndRowIndex:      1000,
					StartColumnIndex: 3,
					EndColumnIndex:   4,
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
		// Format business % as percentage
		{
			RepeatCell: &sheets.RepeatCellRequest{
				Range: &sheets.GridRange{
					SheetId:          sheetID,
					StartRowIndex:    1,
					EndRowIndex:      1000,
					StartColumnIndex: 4,
					EndColumnIndex:   5,
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
		// Format deductible column as currency
		{
			RepeatCell: &sheets.RepeatCellRequest{
				Range: &sheets.GridRange{
					SheetId:          sheetID,
					StartRowIndex:    1,
					EndRowIndex:      1000,
					StartColumnIndex: 5,
					EndColumnIndex:   6,
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
	}

	return requests
}

// formatMonthlyFlowTab formats the Monthly Flow tab.
func (w *Writer) formatMonthlyFlowTab(sheetID int64) []*sheets.Request {
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
		// Format amount columns as currency
		{
			RepeatCell: &sheets.RepeatCellRequest{
				Range: &sheets.GridRange{
					SheetId:          sheetID,
					StartRowIndex:    1,
					EndRowIndex:      1000,
					StartColumnIndex: 1,
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
		// Conditional formatting - red for negative net flow
		{
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
		{
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
	}

	return requests
}

// writeVendorLookupTab writes the vendor lookup table.
func (w *Writer) writeVendorLookupTab(ctx context.Context, spreadsheetID string, vendors []VendorLookupRow) error {
	// Prepare values
	values := [][]any{
		// Header row
		{"Vendor", "Category"},
	}

	// Add vendor rows
	for _, vendor := range vendors {
		values = append(values, []any{
			vendor.VendorName,
			vendor.Category,
		})
	}

	// Write to sheet
	valueRange := &sheets.ValueRange{
		Values: values,
	}

	rangeStr := "Vendor Lookup!A1"
	_, err := w.service.Spreadsheets.Values.Update(spreadsheetID, rangeStr, valueRange).
		ValueInputOption("USER_ENTERED").
		Context(ctx).
		Do()

	return err
}

// writeCategoryLookupTab writes the category lookup table.
func (w *Writer) writeCategoryLookupTab(ctx context.Context, spreadsheetID string, categories []CategoryLookupRow) error {
	// Prepare values
	values := [][]any{
		// Header row
		{"Category", "Type", "Description", "Default Business %"},
	}

	// Add category rows
	for _, category := range categories {
		values = append(values, []any{
			category.CategoryName,
			category.Type,
			category.Description,
			category.DefaultBusinessPct,
		})
	}

	// Write to sheet
	valueRange := &sheets.ValueRange{
		Values: values,
	}

	rangeStr := "Category Lookup!A1"
	_, err := w.service.Spreadsheets.Values.Update(spreadsheetID, rangeStr, valueRange).
		ValueInputOption("USER_ENTERED").
		Context(ctx).
		Do()

	return err
}

// formatVendorLookupTab formats the Vendor Lookup tab.
func (w *Writer) formatVendorLookupTab(sheetID int64) []*sheets.Request {
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
	}

	return requests
}

// formatCategoryLookupTab formats the Category Lookup tab.
func (w *Writer) formatCategoryLookupTab(sheetID int64) []*sheets.Request {
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
	}

	return requests
}
