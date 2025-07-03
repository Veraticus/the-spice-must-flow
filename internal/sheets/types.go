package sheets

import (
	"time"

	"github.com/shopspring/decimal"
)

// ExpenseRow represents a single row in the Expenses tab.
type ExpenseRow struct {
	Date        time.Time
	Amount      decimal.Decimal
	Vendor      string
	Category    string
	Notes       string
	BusinessPct int
}

// IncomeRow represents a single row in the Income tab.
type IncomeRow struct {
	Date     time.Time
	Amount   decimal.Decimal
	Source   string // vendor/payer
	Category string
	Notes    string
}

// VendorSummaryRow represents a single row in the Vendor Summary tab.
type VendorSummaryRow struct {
	VendorName         string
	AssociatedCategory string
	TotalAmount        decimal.Decimal
	TransactionCount   int
}

// CategorySummaryRow represents a single row in the Category Summary tab.
type CategorySummaryRow struct {
	MonthlyAmounts   [12]decimal.Decimal
	CategoryName     string
	Type             string
	TotalAmount      decimal.Decimal
	TransactionCount int
	BusinessPct      int
}

// BusinessExpenseRow represents a single row in the Business Expenses tab.
type BusinessExpenseRow struct {
	Date             time.Time
	Vendor           string
	Category         string
	OriginalAmount   decimal.Decimal
	DeductibleAmount decimal.Decimal
	Notes            string
	BusinessPct      int
}

// MonthlyFlowRow represents a single row in the Monthly Flow tab.
type MonthlyFlowRow struct {
	Month          string // e.g., "January 2024"
	TotalIncome    decimal.Decimal
	TotalExpenses  decimal.Decimal
	NetFlow        decimal.Decimal // Income - Expenses
	RunningBalance decimal.Decimal
}

// VendorLookupRow represents a single row in the Vendor Lookup tab.
type VendorLookupRow struct {
	VendorName string
	Category   string
}

// CategoryLookupRow represents a single row in the Category Lookup tab.
type CategoryLookupRow struct {
	CategoryName       string
	Type               string // income/expense/system
	Description        string
	DefaultBusinessPct int
}

// BusinessRuleLookupRow represents a single row in the Business Rules Lookup tab.
type BusinessRuleLookupRow struct {
	VendorPattern string
	Category      string
	BusinessPct   int
	Notes         string
}

// TabData holds all the data for the complete spreadsheet export.
type TabData struct {
	DateRange           DateRange
	TotalIncome         decimal.Decimal
	TotalExpenses       decimal.Decimal
	TotalDeductible     decimal.Decimal
	Expenses            []ExpenseRow
	Income              []IncomeRow
	VendorSummary       []VendorSummaryRow
	CategorySummary     []CategorySummaryRow
	BusinessExpenses    []BusinessExpenseRow
	MonthlyFlow         []MonthlyFlowRow
	VendorLookup        []VendorLookupRow
	CategoryLookup      []CategoryLookupRow
	BusinessRulesLookup []BusinessRuleLookupRow
}

// DateRange represents the time period covered by the report.
type DateRange struct {
	Start time.Time
	End   time.Time
}
