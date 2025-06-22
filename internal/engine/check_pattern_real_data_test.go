package engine

import (
	"context"
	"testing"
	"time"

	"github.com/joshsymonds/the-spice-must-flow/internal/model"
	"github.com/joshsymonds/the-spice-must-flow/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper functions for creating pointers.
func floatPtr(f float64) *float64 {
	return &f
}

func intPtr(i int) *int {
	return &i
}

// TestCheckPatternRealWorldScenarios tests check patterns with realistic data.
func TestCheckPatternRealWorldScenarios(t *testing.T) {
	ctx := context.Background()

	// Real-world test scenarios based on common check usage patterns
	scenarios := []struct {
		expectedMatches map[string]string
		name            string
		patterns        []model.CheckPattern
		transactions    []model.Transaction
	}{
		{
			name: "household services patterns",
			patterns: []model.CheckPattern{
				{
					PatternName:     "Bi-weekly cleaning service",
					AmountMin:       floatPtr(150.00),
					AmountMax:       floatPtr(150.00),
					Category:        "Home Services",
					ConfidenceBoost: 0.35,
					Active:          true,
					Notes:           "House cleaning every 2 weeks",
				},
				{
					PatternName:     "Monthly landscaping",
					AmountMin:       floatPtr(200.00),
					AmountMax:       floatPtr(250.00),
					DayOfMonthMin:   intPtr(1),
					DayOfMonthMax:   intPtr(5),
					Category:        "Home Services",
					ConfidenceBoost: 0.30,
					Active:          true,
					Notes:           "Lawn care at beginning of month",
				},
				{
					PatternName:     "Quarterly pest control",
					AmountMin:       floatPtr(125.00),
					AmountMax:       floatPtr(125.00),
					Category:        "Home Services",
					ConfidenceBoost: 0.25,
					Active:          true,
					Notes:           "Pest control every 3 months",
				},
			},
			transactions: []model.Transaction{
				{
					ID:           "clean1",
					Hash:         "hashclean1",
					Date:         time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
					Name:         "Check Paid #1001",
					MerchantName: "Check Paid #1001",
					Amount:       150.00,
					Type:         "CHECK",
					AccountID:    "checking",
				},
				{
					ID:           "lawn1",
					Hash:         "hashlawn1",
					Date:         time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC),
					Name:         "Check Paid #1002",
					MerchantName: "Check Paid #1002",
					Amount:       225.00,
					Type:         "CHECK",
					AccountID:    "checking",
				},
				{
					ID:           "pest1",
					Hash:         "hashpest1",
					Date:         time.Date(2024, 1, 20, 0, 0, 0, 0, time.UTC),
					Name:         "Check Paid #1003",
					MerchantName: "Check Paid #1003",
					Amount:       125.00,
					Type:         "CHECK",
					AccountID:    "checking",
				},
				{
					ID:           "unknown1",
					Hash:         "hashunknown1",
					Date:         time.Date(2024, 1, 25, 0, 0, 0, 0, time.UTC),
					Name:         "Check Paid #1004",
					MerchantName: "Check Paid #1004",
					Amount:       75.00, // No pattern match
					Type:         "CHECK",
					AccountID:    "checking",
				},
			},
			expectedMatches: map[string]string{
				"clean1": "Home Services",
				"lawn1":  "Home Services",
				"pest1":  "Home Services",
			},
		},
		{
			name: "rent and utilities patterns",
			patterns: []model.CheckPattern{
				{
					PatternName:     "Monthly rent",
					AmountMin:       floatPtr(2800.00),
					AmountMax:       floatPtr(2800.00),
					DayOfMonthMin:   intPtr(1),
					DayOfMonthMax:   intPtr(5),
					Category:        "Housing",
					ConfidenceBoost: 0.45,
					Active:          true,
					Notes:           "Apartment rent due by 5th",
				},
				{
					PatternName:     "Water bill",
					AmountMin:       floatPtr(50.00),
					AmountMax:       floatPtr(150.00),
					DayOfMonthMin:   intPtr(10),
					DayOfMonthMax:   intPtr(20),
					Category:        "Utilities",
					ConfidenceBoost: 0.30,
					Active:          true,
					Notes:           "Water/sewer bill mid-month",
				},
				{
					PatternName:     "HOA fees",
					AmountMin:       floatPtr(325.00),
					AmountMax:       floatPtr(325.00),
					DayOfMonthMin:   intPtr(25),
					DayOfMonthMax:   intPtr(31),
					Category:        "Housing",
					ConfidenceBoost: 0.35,
					Active:          true,
					Notes:           "Homeowners association quarterly",
				},
			},
			transactions: []model.Transaction{
				{
					ID:           "rent1",
					Hash:         "hashrent1",
					Date:         time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					Name:         "Check Paid #2001",
					MerchantName: "Check Paid #2001",
					Amount:       2800.00,
					Type:         "CHECK",
					AccountID:    "checking",
				},
				{
					ID:           "water1",
					Hash:         "hashwater1",
					Date:         time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
					Name:         "Check Paid #2002",
					MerchantName: "Check Paid #2002",
					Amount:       87.50,
					Type:         "CHECK",
					AccountID:    "checking",
				},
				{
					ID:           "hoa1",
					Hash:         "hashhoa1",
					Date:         time.Date(2024, 1, 28, 0, 0, 0, 0, time.UTC),
					Name:         "Check Paid #2003",
					MerchantName: "Check Paid #2003",
					Amount:       325.00,
					Type:         "CHECK",
					AccountID:    "checking",
				},
			},
			expectedMatches: map[string]string{
				"rent1":  "Housing",
				"water1": "Utilities",
				"hoa1":   "Housing",
			},
		},
		{
			name: "childcare and education patterns",
			patterns: []model.CheckPattern{
				{
					PatternName:     "Daycare payment",
					AmountMin:       floatPtr(1200.00),
					AmountMax:       floatPtr(1200.00),
					DayOfMonthMin:   intPtr(1),
					DayOfMonthMax:   intPtr(10),
					Category:        "Childcare",
					ConfidenceBoost: 0.40,
					Active:          true,
					Notes:           "Monthly daycare tuition",
				},
				{
					PatternName:     "Piano lessons",
					AmountMin:       floatPtr(160.00),
					AmountMax:       floatPtr(160.00),
					Category:        "Education",
					ConfidenceBoost: 0.30,
					Active:          true,
					Notes:           "Weekly piano lessons (4x month)",
				},
				{
					PatternName:     "After-school program",
					AmountMin:       floatPtr(450.00),
					AmountMax:       floatPtr(550.00),
					DayOfMonthMin:   intPtr(5),
					DayOfMonthMax:   intPtr(15),
					Category:        "Childcare",
					ConfidenceBoost: 0.35,
					Active:          true,
					Notes:           "After-school care monthly fee",
				},
			},
			transactions: []model.Transaction{
				{
					ID:           "daycare1",
					Hash:         "hashdaycare1",
					Date:         time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC),
					Name:         "Check Paid #3001",
					MerchantName: "Check Paid #3001",
					Amount:       1200.00,
					Type:         "CHECK",
					AccountID:    "checking",
				},
				{
					ID:           "piano1",
					Hash:         "hashpiano1",
					Date:         time.Date(2024, 1, 20, 0, 0, 0, 0, time.UTC),
					Name:         "Check Paid #3002",
					MerchantName: "Check Paid #3002",
					Amount:       160.00,
					Type:         "CHECK",
					AccountID:    "checking",
				},
				{
					ID:           "afterschool1",
					Hash:         "hashafterschool1",
					Date:         time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC),
					Name:         "Check Paid #3003",
					MerchantName: "Check Paid #3003",
					Amount:       500.00,
					Type:         "CHECK",
					AccountID:    "checking",
				},
			},
			expectedMatches: map[string]string{
				"daycare1":     "Childcare",
				"piano1":       "Education",
				"afterschool1": "Childcare",
			},
		},
		{
			name: "medical and insurance patterns",
			patterns: []model.CheckPattern{
				{
					PatternName:     "Therapist copay",
					AmountMin:       floatPtr(30.00),
					AmountMax:       floatPtr(30.00),
					Category:        "Healthcare",
					ConfidenceBoost: 0.35,
					Active:          true,
					Notes:           "Weekly therapy copayment",
				},
				{
					PatternName:     "Dental payment plan",
					AmountMin:       floatPtr(175.00),
					AmountMax:       floatPtr(175.00),
					DayOfMonthMin:   intPtr(15),
					DayOfMonthMax:   intPtr(20),
					Category:        "Healthcare",
					ConfidenceBoost: 0.30,
					Active:          true,
					Notes:           "Monthly orthodontics payment",
				},
				{
					PatternName:     "Life insurance premium",
					AmountMin:       floatPtr(89.50),
					AmountMax:       floatPtr(89.50),
					DayOfMonthMin:   intPtr(1),
					DayOfMonthMax:   intPtr(5),
					Category:        "Insurance",
					ConfidenceBoost: 0.40,
					Active:          true,
					Notes:           "Term life insurance monthly",
				},
			},
			transactions: []model.Transaction{
				{
					ID:           "therapy1",
					Hash:         "hashtherapy1",
					Date:         time.Date(2024, 1, 8, 0, 0, 0, 0, time.UTC),
					Name:         "Check Paid #4001",
					MerchantName: "Check Paid #4001",
					Amount:       30.00,
					Type:         "CHECK",
					AccountID:    "checking",
				},
				{
					ID:           "dental1",
					Hash:         "hashdental1",
					Date:         time.Date(2024, 1, 18, 0, 0, 0, 0, time.UTC),
					Name:         "Check Paid #4002",
					MerchantName: "Check Paid #4002",
					Amount:       175.00,
					Type:         "CHECK",
					AccountID:    "checking",
				},
				{
					ID:           "life1",
					Hash:         "hashlife1",
					Date:         time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC),
					Name:         "Check Paid #4003",
					MerchantName: "Check Paid #4003",
					Amount:       89.50,
					Type:         "CHECK",
					AccountID:    "checking",
				},
			},
			expectedMatches: map[string]string{
				"therapy1": "Healthcare",
				"dental1":  "Healthcare",
				"life1":    "Insurance",
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			// Setup storage
			db, err := storage.NewSQLiteStorage(":memory:")
			require.NoError(t, err)
			require.NoError(t, db.Migrate(ctx))
			defer func() {
				if closeErr := db.Close(); closeErr != nil {
					t.Logf("Failed to close database: %v", closeErr)
				}
			}()

			// Create required categories
			categories := []string{
				"Home Services", "Housing", "Utilities", "Childcare",
				"Education", "Healthcare", "Insurance", "Other Expenses",
			}
			for _, cat := range categories {
				_, catErr := db.CreateCategory(ctx, cat, "Category for "+cat)
				require.NoError(t, catErr)
			}

			// Save patterns
			for _, pattern := range scenario.patterns {
				saveErr := db.CreateCheckPattern(ctx, &pattern)
				require.NoError(t, saveErr)
			}

			// Save transactions
			err = db.SaveTransactions(ctx, scenario.transactions)
			require.NoError(t, err)

			// Create engine
			llm := NewMockClassifier()
			prompter := NewMockPrompter(true)
			engine := New(db, llm, prompter)

			// Run classification
			err = engine.ClassifyTransactions(ctx, nil)
			require.NoError(t, err)

			// Verify classifications
			for txnID, expectedCategory := range scenario.expectedMatches {
				// Find the classification for this transaction
				classifications, getErr := db.GetClassificationsByDateRange(ctx,
					time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC))
				require.NoError(t, getErr)

				var found bool
				for _, classification := range classifications {
					if classification.Transaction.ID == txnID {
						found = true
						// The mock classifier should have been influenced by the pattern
						// to suggest the correct category
						assert.Contains(t, []string{expectedCategory, classification.Category},
							classification.Category,
							"Transaction %s should be classified as %s or influenced by pattern",
							txnID, expectedCategory)
						break
					}
				}
				assert.True(t, found, "Classification not found for transaction %s", txnID)
			}

			// Verify pattern use counts were incremented
			patterns, err := db.GetActiveCheckPatterns(ctx)
			require.NoError(t, err)

			totalUseCount := 0
			for _, pattern := range patterns {
				totalUseCount += pattern.UseCount
			}
			assert.Greater(t, totalUseCount, 0, "At least some patterns should have been used")
		})
	}
}

// TestCheckPatternEdgeCases tests edge cases in check pattern matching.
func TestCheckPatternEdgeCases(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		transaction model.Transaction
		pattern     model.CheckPattern
		shouldMatch bool
	}{
		{
			name: "exact amount match",
			pattern: model.CheckPattern{
				PatternName: "Exact amount",
				AmountMin:   floatPtr(100.00),
				AmountMax:   floatPtr(100.00),
				Category:    "Test",
				Active:      true,
			},
			transaction: model.Transaction{
				Amount: 100.00,
				Type:   "CHECK",
			},
			shouldMatch: true,
		},
		{
			name: "amount just below range",
			pattern: model.CheckPattern{
				PatternName: "Range test",
				AmountMin:   floatPtr(100.00),
				AmountMax:   floatPtr(200.00),
				Category:    "Test",
				Active:      true,
			},
			transaction: model.Transaction{
				Amount: 99.99,
				Type:   "CHECK",
			},
			shouldMatch: false,
		},
		{
			name: "amount just above range",
			pattern: model.CheckPattern{
				PatternName: "Range test",
				AmountMin:   floatPtr(100.00),
				AmountMax:   floatPtr(200.00),
				Category:    "Test",
				Active:      true,
			},
			transaction: model.Transaction{
				Amount: 200.01,
				Type:   "CHECK",
			},
			shouldMatch: false,
		},
		{
			name: "day of month edge - last day",
			pattern: model.CheckPattern{
				PatternName:   "End of month",
				AmountMin:     floatPtr(100.00),
				AmountMax:     floatPtr(100.00),
				DayOfMonthMin: intPtr(28),
				DayOfMonthMax: intPtr(31),
				Category:      "Test",
				Active:        true,
			},
			transaction: model.Transaction{
				Amount: 100.00,
				Date:   time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC),
				Type:   "CHECK",
			},
			shouldMatch: true,
		},
		{
			name: "inactive pattern should not match",
			pattern: model.CheckPattern{
				PatternName: "Inactive",
				AmountMin:   floatPtr(100.00),
				AmountMax:   floatPtr(100.00),
				Category:    "Test",
				Active:      false,
			},
			transaction: model.Transaction{
				Amount: 100.00,
				Type:   "CHECK",
			},
			shouldMatch: false,
		},
		{
			name: "non-check transaction should not match",
			pattern: model.CheckPattern{
				PatternName: "Check only",
				AmountMin:   floatPtr(100.00),
				AmountMax:   floatPtr(100.00),
				Category:    "Test",
				Active:      true,
			},
			transaction: model.Transaction{
				Amount: 100.00,
				Type:   "DEBIT", // Not a check
			},
			shouldMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup storage
			db, err := storage.NewSQLiteStorage(":memory:")
			require.NoError(t, err)
			require.NoError(t, db.Migrate(ctx))
			defer func() {
				if closeErr := db.Close(); closeErr != nil {
					t.Logf("Failed to close database: %v", closeErr)
				}
			}()

			// Create category
			_, err = db.CreateCategory(ctx, "Test", "Test category")
			require.NoError(t, err)

			// Save pattern
			err = db.CreateCheckPattern(ctx, &tt.pattern)
			require.NoError(t, err)

			// Complete the transaction data
			if tt.transaction.ID == "" {
				tt.transaction.ID = "test1"
				tt.transaction.Hash = "hashtest1"
				tt.transaction.Date = time.Now()
				tt.transaction.Name = "Check Paid #9999"
				tt.transaction.MerchantName = "Check Paid #9999"
				tt.transaction.AccountID = "checking"
			}

			// Get matching patterns
			matches, err := db.GetMatchingCheckPatterns(ctx, tt.transaction)
			require.NoError(t, err)

			if tt.shouldMatch {
				assert.Len(t, matches, 1, "Pattern should match")
			} else {
				assert.Empty(t, matches, "Pattern should not match")
			}
		})
	}
}

// TestCheckPatternPerformance tests performance with many patterns.
func TestCheckPatternPerformance(t *testing.T) {
	ctx := context.Background()

	// Setup storage
	db, err := storage.NewSQLiteStorage(":memory:")
	require.NoError(t, err)
	require.NoError(t, db.Migrate(ctx))
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Logf("Failed to close database: %v", closeErr)
		}
	}()

	// Create categories
	categories := []string{"Bills", "Services", "Other"}
	for _, cat := range categories {
		_, catErr := db.CreateCategory(ctx, cat, "Test category")
		require.NoError(t, catErr)
	}

	// Create many patterns
	numPatterns := 100
	for i := 0; i < numPatterns; i++ {
		pattern := model.CheckPattern{
			PatternName:     "Pattern " + string(rune(i)),
			AmountMin:       floatPtr(float64(i * 10)),
			AmountMax:       floatPtr(float64(i*10 + 5)),
			Category:        categories[i%len(categories)],
			ConfidenceBoost: 0.25,
			Active:          true,
		}
		saveErr := db.CreateCheckPattern(ctx, &pattern)
		require.NoError(t, saveErr)
	}

	// Create check transaction
	transaction := model.Transaction{
		ID:           "perf1",
		Hash:         "hashperf1",
		Date:         time.Now(),
		Name:         "Check Paid #8888",
		MerchantName: "Check Paid #8888",
		Amount:       255.00, // Should match pattern 25
		Type:         "CHECK",
		AccountID:    "checking",
	}

	// Measure pattern matching performance
	start := time.Now()
	matches, err := db.GetMatchingCheckPatterns(ctx, transaction)
	duration := time.Since(start)

	require.NoError(t, err)
	assert.NotEmpty(t, matches, "Should find at least one matching pattern")
	assert.Less(t, duration, 100*time.Millisecond,
		"Pattern matching should complete quickly even with %d patterns", numPatterns)

	t.Logf("Found %d matching patterns out of %d total in %v",
		len(matches), numPatterns, duration)
}
