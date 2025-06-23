package engine

import (
	"context"
	"testing"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassificationEngine_CheckPatternIntegration(t *testing.T) {
	ctx := context.Background()

	// Setup storage
	db, err := storage.NewSQLiteStorage(":memory:")
	require.NoError(t, err)
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Errorf("Failed to close database: %v", closeErr)
		}
	}()

	require.NoError(t, db.Migrate(ctx))

	// Create categories
	categories := []string{"Utilities", "Home Services", "Insurance", "Other"}
	for _, cat := range categories {
		_, createErr := db.CreateCategory(ctx, cat, "Test category: "+cat, model.CategoryTypeExpense)
		require.NoError(t, createErr)
	}

	// Create check patterns
	patterns := []model.CheckPattern{
		{
			PatternName:     "Monthly electric bill",
			Category:        "Utilities",
			AmountMin:       ptr(100.0),
			AmountMax:       ptr(200.0),
			DayOfMonthMin:   ptrInt(10),
			DayOfMonthMax:   ptrInt(15),
			ConfidenceBoost: 0.3,
			Active:          true,
		},
		{
			PatternName:     "Cleaning service",
			Category:        "Home Services",
			AmountMin:       ptr(250.0),
			AmountMax:       ptr(250.0), // Exact amount, different from electric bill
			DayOfMonthMin:   ptrInt(20),
			DayOfMonthMax:   ptrInt(25),
			ConfidenceBoost: 0.4,
			Active:          true,
		},
		{
			PatternName:     "Insurance payment",
			Category:        "Insurance",
			AmountMin:       ptr(500.0),
			AmountMax:       ptr(700.0),
			DayOfMonthMin:   ptrInt(1),
			DayOfMonthMax:   ptrInt(7),
			ConfidenceBoost: 0.5, // Higher boost to ensure it wins
			Active:          true,
		},
	}

	for _, pattern := range patterns {
		createErr := db.CreateCheckPattern(ctx, &pattern)
		require.NoError(t, createErr)
	}

	// Create check transactions
	transactions := []model.Transaction{
		{
			ID:           "check1",
			Hash:         "hash-check1",
			Date:         time.Date(2025, 1, 12, 0, 0, 0, 0, time.UTC), // Day 12
			Name:         "CHECK 1234",
			MerchantName: "CHECK 1234",
			Amount:       150.0,
			Type:         "CHECK",
			AccountID:    "acc1",
			Direction:    model.DirectionExpense,
		},
		{
			ID:           "check2",
			Hash:         "hash-check2",
			Date:         time.Date(2025, 1, 22, 0, 0, 0, 0, time.UTC), // Day 22 (between 20-25)
			Name:         "CHECK 1235",
			MerchantName: "CHECK 1235",
			Amount:       250.0, // Match cleaning service pattern
			Type:         "CHECK",
			AccountID:    "acc1",
			Direction:    model.DirectionExpense,
		},
		{
			ID:           "check3",
			Hash:         "hash-check3",
			Date:         time.Date(2025, 1, 5, 0, 0, 0, 0, time.UTC), // Day 5
			Name:         "CHECK 1236",
			MerchantName: "CHECK 1236",
			Amount:       600.0,
			Type:         "CHECK",
			AccountID:    "acc1",
			Direction:    model.DirectionExpense,
		},
		{
			ID:           "non-check",
			Hash:         "hash-non-check",
			Date:         time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
			Name:         "REGULAR TRANSACTION",
			MerchantName: "Some Store",
			Amount:       150.0,
			Type:         "DEBIT",
			AccountID:    "acc1",
			Direction:    model.DirectionExpense,
		},
	}

	err = db.SaveTransactions(ctx, transactions)
	require.NoError(t, err)

	// Create mock LLM and prompter
	llm := NewMockClassifier()
	prompter := NewMockPrompter(true) // Auto-accept

	// Create engine
	engine := New(db, llm, prompter)

	// Run classification
	err = engine.ClassifyTransactions(ctx, nil)
	require.NoError(t, err)

	// Verify classifications
	t.Run("check patterns applied correctly", func(t *testing.T) {
		// Get classifications - use a date range that includes all transactions
		classifications, err := db.GetClassificationsByDateRange(ctx, time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC))
		require.NoError(t, err)

		// Find our classified transactions
		classifiedChecks := make(map[string]*model.Classification)
		for i := range classifications {
			if classifications[i].Transaction.Type == "CHECK" {
				classifiedChecks[classifications[i].Transaction.ID] = &classifications[i]
			}
		}

		// Debug: print what we found
		t.Logf("Found %d classifications", len(classifications))
		for i, cls := range classifications {
			t.Logf("Classification %d: ID=%s, Type=%s, Category=%s", i, cls.Transaction.ID, cls.Transaction.Type, cls.Category)
		}
		t.Logf("Check transactions found:")
		for id, cls := range classifiedChecks {
			t.Logf("Check ID %s: category=%s", id, cls.Category)
		}

		// CHECK 1234 should match "Monthly electric bill" pattern (amount + day of month match)
		check1 := classifiedChecks["check1"]
		require.NotNil(t, check1, "check1 not found in classifications")
		assert.Equal(t, "Utilities", check1.Category, "Check matching electric bill pattern should be classified as Utilities")

		// CHECK 1235 should match "Cleaning service" pattern (exact amount match)
		check2 := classifiedChecks["check2"]
		require.NotNil(t, check2)
		assert.Equal(t, "Home Services", check2.Category, "Check matching cleaning service pattern should be classified as Home Services")

		// CHECK 1236 should match "Insurance payment" pattern (amount >= 500)
		check3 := classifiedChecks["check3"]
		require.NotNil(t, check3)
		assert.Equal(t, "Insurance", check3.Category, "Check matching insurance pattern should be classified as Insurance")
	})

	t.Run("pattern use counts updated", func(t *testing.T) {
		// Get updated patterns
		activePatterns, err := db.GetActiveCheckPatterns(ctx)
		require.NoError(t, err)

		// Create a map for easier lookup
		patternMap := make(map[string]*model.CheckPattern)
		for i := range activePatterns {
			patternMap[activePatterns[i].PatternName] = &activePatterns[i]
		}

		// Verify use counts were incremented
		electricPattern := patternMap["Monthly electric bill"]
		assert.Equal(t, 1, electricPattern.UseCount, "Electric bill pattern should have been used once")

		cleaningPattern := patternMap["Cleaning service"]
		assert.Equal(t, 1, cleaningPattern.UseCount, "Cleaning service pattern should have been used once")

		insurancePattern := patternMap["Insurance payment"]
		assert.Equal(t, 1, insurancePattern.UseCount, "Insurance pattern should have been used once")
	})

	t.Run("non-check transactions not affected", func(t *testing.T) {
		// Verify non-check transaction was classified normally
		classifications, err := db.GetClassificationsByDateRange(ctx, time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC))
		require.NoError(t, err)

		for _, cls := range classifications {
			if cls.Transaction.ID == "non-check" {
				assert.Equal(t, "DEBIT", cls.Transaction.Type)
				assert.NotEmpty(t, cls.Category, "Non-check transaction should still be classified")
				assert.NotEqual(t, "Utilities", cls.Category, "Non-check should not match check patterns")
				assert.NotEqual(t, "Home Services", cls.Category, "Non-check should not match check patterns")
				break
			}
		}
	})
}

func TestClassificationEngine_CheckPatternAutoClassification(t *testing.T) {
	ctx := context.Background()

	// Setup storage
	db, err := storage.NewSQLiteStorage(":memory:")
	require.NoError(t, err)
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Errorf("Failed to close database: %v", closeErr)
		}
	}()

	require.NoError(t, db.Migrate(ctx))

	// Create categories
	_, err = db.CreateCategory(ctx, "Rent", "Monthly rent payment", model.CategoryTypeExpense)
	require.NoError(t, err)
	_, err = db.CreateCategory(ctx, "Other", "Other expenses", model.CategoryTypeExpense)
	require.NoError(t, err)

	// Create a high-confidence check pattern
	pattern := model.CheckPattern{
		PatternName:     "Monthly rent",
		Category:        "Rent",
		AmountMin:       ptr(2000.0),
		AmountMax:       ptr(2000.0),
		ConfidenceBoost: 0.5, // This should push confidence over 85%
		Active:          true,
	}
	err = db.CreateCheckPattern(ctx, &pattern)
	require.NoError(t, err)

	// Create a check transaction that matches the pattern
	transaction := model.Transaction{
		ID:           "rent-check",
		Hash:         "hash-rent",
		Date:         time.Now(),
		Name:         "CHECK 5000",
		MerchantName: "CHECK 5000",
		Amount:       2000.0,
		Type:         "CHECK",
		AccountID:    "acc1",
		Direction:    model.DirectionExpense,
	}
	err = db.SaveTransactions(ctx, []model.Transaction{transaction})
	require.NoError(t, err)

	// Create mocks
	llm := NewMockClassifier()
	prompter := NewMockPrompter(true)

	// Create engine
	engine := New(db, llm, prompter)

	// Run classification
	err = engine.ClassifyTransactions(ctx, nil)
	require.NoError(t, err)

	// Verify auto-classification occurred
	stats := prompter.GetCompletionStats()
	assert.Equal(t, 0, stats.TotalTransactions, "Transaction should have been auto-classified, bypassing prompter")

	// Verify the transaction was classified correctly
	classifications, err := db.GetClassificationsByDateRange(ctx, transaction.Date.AddDate(0, 0, -1), transaction.Date.AddDate(0, 0, 1))
	require.NoError(t, err)
	require.Len(t, classifications, 1)
	assert.Equal(t, "Rent", classifications[0].Category)

	// Verify pattern use count was incremented
	patterns, err := db.GetActiveCheckPatterns(ctx)
	require.NoError(t, err)
	require.Len(t, patterns, 1)
	assert.Equal(t, 1, patterns[0].UseCount, "Pattern use count should be incremented for auto-classification")
}

// Helper function to create pointer to float64.
func ptr(f float64) *float64 {
	return &f
}

// Helper function to create pointer to int.
func ptrInt(i int) *int {
	return &i
}
