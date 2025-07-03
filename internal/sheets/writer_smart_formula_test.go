package sheets

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriter_SmartBusinessPercentFormulas(t *testing.T) {

	// Create test data
	categories := []model.Category{
		{
			ID:                     1,
			Name:                   "Office Supplies",
			Type:                   model.CategoryTypeExpense,
			DefaultBusinessPercent: 100,
			IsActive:               true,
		},
		{
			ID:                     2,
			Name:                   "Meals",
			Type:                   model.CategoryTypeExpense,
			DefaultBusinessPercent: 50,
			IsActive:               true,
		},
		{
			ID:                     3,
			Name:                   "Personal",
			Type:                   model.CategoryTypeExpense,
			DefaultBusinessPercent: 0,
			IsActive:               true,
		},
	}

	testDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	classifications := []model.Classification{
		{
			Transaction: model.Transaction{
				ID:           "tx1",
				Date:         testDate,
				MerchantName: "Office Depot",
				Amount:       100.00,
			},
			Category:        "Office Supplies",
			BusinessPercent: 100, // Matches category default
			Status:          model.StatusClassifiedByRule,
		},
		{
			Transaction: model.Transaction{
				ID:           "tx2",
				Date:         testDate,
				MerchantName: "Restaurant ABC",
				Amount:       50.00,
			},
			Category:        "Meals",
			BusinessPercent: 75, // Override - doesn't match default of 50
			Status:          model.StatusClassifiedByRule,
		},
		{
			Transaction: model.Transaction{
				ID:           "tx3",
				Date:         testDate,
				MerchantName: "Personal Store",
				Amount:       25.00,
			},
			Category:        "Personal",
			BusinessPercent: 0, // Matches category default
			Status:          model.StatusClassifiedByRule,
		},
	}

	summary := &service.ReportSummary{
		DateRange: service.DateRange{
			Start: testDate.AddDate(0, 0, -7),
			End:   testDate.AddDate(0, 0, 7),
		},
	}

	// Create a writer for testing
	writer := &Writer{
		logger: testLogger(),
	}

	// Test aggregateData function
	tabData, err := writer.aggregateData(classifications, summary, categories)
	require.NoError(t, err)

	// Verify CategoryLookup has DefaultBusinessPct populated
	assert.Len(t, tabData.CategoryLookup, 3)
	for _, catLookup := range tabData.CategoryLookup {
		switch catLookup.CategoryName {
		case "Office Supplies":
			assert.Equal(t, 100, catLookup.DefaultBusinessPct)
		case "Meals":
			assert.Equal(t, 50, catLookup.DefaultBusinessPct)
		case "Personal":
			assert.Equal(t, 0, catLookup.DefaultBusinessPct)
		}
	}

	// Verify the expenses have the correct business percentages
	assert.Len(t, tabData.Expenses, 3)
	assert.Equal(t, 100, tabData.Expenses[0].BusinessPct) // Office Depot
	assert.Equal(t, 75, tabData.Expenses[1].BusinessPct)  // Restaurant ABC (override)
	assert.Equal(t, 0, tabData.Expenses[2].BusinessPct)   // Personal Store
}

func TestWriter_CategoryLookupInclusion(t *testing.T) {
	// Test that all categories are included in the lookup, even if no transactions
	categories := []model.Category{
		{
			ID:                     1,
			Name:                   "Used Category",
			Type:                   model.CategoryTypeExpense,
			DefaultBusinessPercent: 50,
			Description:            "This one has transactions",
			IsActive:               true,
		},
		{
			ID:                     2,
			Name:                   "Unused Category",
			Type:                   model.CategoryTypeExpense,
			DefaultBusinessPercent: 75,
			Description:            "This one has no transactions",
			IsActive:               true,
		},
	}

	classifications := []model.Classification{
		{
			Transaction: model.Transaction{
				ID:           "tx1",
				Date:         time.Now(),
				MerchantName: "Test Vendor",
				Amount:       100.00,
			},
			Category:        "Used Category",
			BusinessPercent: 50,
			Status:          model.StatusClassifiedByRule,
		},
	}

	summary := &service.ReportSummary{
		DateRange: service.DateRange{
			Start: time.Now().AddDate(0, -1, 0),
			End:   time.Now(),
		},
	}

	writer := &Writer{logger: testLogger()}

	tabData, err := writer.aggregateData(classifications, summary, categories)
	require.NoError(t, err)

	// Both categories should be in the lookup
	assert.Len(t, tabData.CategoryLookup, 2)

	// Verify both categories are present with correct data
	categoryMap := make(map[string]CategoryLookupRow)
	for _, cat := range tabData.CategoryLookup {
		categoryMap[cat.CategoryName] = cat
	}

	usedCat, exists := categoryMap["Used Category"]
	assert.True(t, exists)
	assert.Equal(t, 50, usedCat.DefaultBusinessPct)
	assert.Equal(t, "This one has transactions", usedCat.Description)

	unusedCat, exists := categoryMap["Unused Category"]
	assert.True(t, exists)
	assert.Equal(t, 75, unusedCat.DefaultBusinessPct)
	assert.Equal(t, "This one has no transactions", unusedCat.Description)
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, nil))
}
