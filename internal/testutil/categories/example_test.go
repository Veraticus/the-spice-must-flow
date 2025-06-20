package categories_test

import (
	"context"
	"testing"
	"time"

	"github.com/joshsymonds/the-spice-must-flow/internal/model"
	"github.com/joshsymonds/the-spice-must-flow/internal/service"
	"github.com/joshsymonds/the-spice-must-flow/internal/testutil"
	"github.com/joshsymonds/the-spice-must-flow/internal/testutil/categories"
)

// Example_basicUsage demonstrates the simplest way to use the test infrastructure.
func Example_basicUsage() {
	t := &testing.T{} // In real tests, this comes from the test function

	// Set up a test database with basic categories
	db := testutil.SetupTestDBWithBuilder(t, func(b categories.Builder) categories.Builder {
		return b.WithBasicCategories()
	})

	// Use the storage for your test
	ctx := context.Background()
	cats, _ := db.Storage.GetCategories(ctx)
	
	// Categories are available for use
	for _, cat := range cats {
		_ = cat.Name // Use category
	}
}

// Example_customCategories shows how to add specific categories for a test.
func Example_customCategories() {
	t := &testing.T{}

	db := testutil.SetupTestDBWithBuilder(t, func(b categories.Builder) categories.Builder {
		return b.
			WithCategory(categories.CategoryGroceries).
			WithCategory("My Custom Category").
			WithCategories("Category A", "Category B", "Category C")
	})

	// Access a specific category
	groceries := db.Categories.MustFind(t, categories.CategoryGroceries)
	_ = groceries.Name // "Groceries"
}

// Example_fixtures demonstrates using predefined category sets.
func Example_fixtures() {
	t := &testing.T{}

	// Use a predefined fixture for consistency
	db := testutil.SetupTestDBWithBuilder(t, func(b categories.Builder) categories.Builder {
		return b.WithFixture(categories.FixtureStandard)
	})

	// All standard categories are available
	_ = db.Storage
}

// Example_compositeFixtures shows how to combine multiple fixtures.
func Example_compositeFixtures() {
	t := &testing.T{}

	// Create a custom composite fixture
	myFixture := categories.NewCompositeFixture(
		"MyTestFixture",
		"Combines standard categories with test-specific ones",
		categories.FixtureStandard,
		categories.FixtureTestingOnly,
	)

	db := testutil.SetupTestDBWithBuilder(t, func(b categories.Builder) categories.Builder {
		return b.WithFixture(myFixture)
	})

	_ = db.Storage
}

// TestComplexScenario demonstrates a real-world test scenario.
func TestComplexScenario(t *testing.T) {
	// Set up database with all required categories
	db := testutil.SetupTestDBWithBuilder(t, func(b categories.Builder) categories.Builder {
		return b.
			WithFixture(categories.FixtureStandard).     // Common categories
			WithCategory("Special Promotion").           // Test-specific
			WithCategories("Refund", "Cashback")         // Additional test categories
	})

	ctx := context.Background()

	// Create test transactions
	transactions := []model.Transaction{
		{
			ID:           "txn-1",
			Date:         time.Now(),
			Name:         "WHOLE FOODS MARKET",
			MerchantName: "Whole Foods",
			Amount:       125.50,
			AccountID:    "account-1",
		},
		{
			ID:           "txn-2",
			Date:         time.Now(),
			Name:         "STARBUCKS #12345",
			MerchantName: "Starbucks",
			Amount:       5.75,
			AccountID:    "account-1",
		},
	}

	// Save transactions
	for i := range transactions {
		transactions[i].Hash = transactions[i].GenerateHash()
	}
	err := db.Storage.SaveTransactions(ctx, transactions)
	if err != nil {
		t.Fatalf("failed to save transactions: %v", err)
	}

	// Create classifications using our seeded categories
	classifications := []model.Classification{
		{
			Transaction: transactions[0],
			Category:    db.MustGetCategory(categories.CategoryGroceries),
			Status:      model.StatusUserModified,
			Confidence:  1.0,
		},
		{
			Transaction: transactions[1],
			Category:    db.MustGetCategory(categories.CategoryCoffeeDining),
			Status:      model.StatusClassifiedByAI,
			Confidence:  0.95,
		},
	}

	// Save classifications
	for i := range classifications {
		err := db.Storage.SaveClassification(ctx, &classifications[i])
		if err != nil {
			t.Fatalf("failed to save classification: %v", err)
		}
	}

	// Verify classifications
	results, err := db.Storage.GetClassificationsByDateRange(
		ctx,
		time.Now().Add(-1*time.Hour),
		time.Now().Add(1*time.Hour),
	)
	if err != nil {
		t.Fatalf("failed to get classifications: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 classifications, got %d", len(results))
	}
}

// TestTableDrivenWithCategories shows table-driven tests with different category requirements.
func TestTableDrivenWithCategories(t *testing.T) {
	tests := []struct {
		name       string
		categories []categories.CategoryName
		test       func(t *testing.T, storage service.Storage)
	}{
		{
			name: "groceries classification",
			categories: []categories.CategoryName{
				categories.CategoryGroceries,
				categories.CategoryFoodDining,
			},
			test: func(t *testing.T, storage service.Storage) {
				// Test logic using Groceries and Food & Dining categories
			},
		},
		{
			name: "subscription management",
			categories: []categories.CategoryName{
				categories.CategorySubscriptions,
				categories.CategoryEntertainment,
			},
			test: func(t *testing.T, storage service.Storage) {
				// Test logic using subscription categories
			},
		},
		{
			name: "comprehensive test",
			categories: []categories.CategoryName{}, // Will use fixture instead
			test: func(t *testing.T, storage service.Storage) {
				// Test logic needing many categories
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var db *testutil.TestDB
			
			if len(tt.categories) == 0 {
				// Use comprehensive fixture for tests needing many categories
				db = testutil.SetupTestDBWithBuilder(t, func(b categories.Builder) categories.Builder {
					return b.WithFixture(categories.FixtureComprehensive)
				})
			} else {
				// Use specific categories
				db = testutil.SetupTestDBWithBuilder(t, func(b categories.Builder) categories.Builder {
					return b.WithCategories(tt.categories...)
				})
			}

			// Run the test with properly seeded storage
			tt.test(t, db.Storage)
		})
	}
}

// TestParallelExecution demonstrates thread-safe test execution.
func TestParallelExecution(t *testing.T) {
	// Each parallel test gets its own isolated database
	testCases := []string{"test1", "test2", "test3", "test4"}

	for _, tc := range testCases {
		tc := tc // Capture range variable
		t.Run(tc, func(t *testing.T) {
			t.Parallel() // Safe to run in parallel

			db := testutil.SetupTestDBWithBuilder(t, func(b categories.Builder) categories.Builder {
				return b.WithBasicCategories()
			})

			// Each test has its own isolated categories
			ctx := context.Background()
			cats, err := db.Storage.GetCategories(ctx)
			if err != nil {
				t.Fatalf("failed to get categories: %v", err)
			}

			// Modifications don't affect other tests
			_, err = db.Storage.CreateCategory(ctx, tc+"-specific")
			if err != nil {
				t.Fatalf("failed to create category: %v", err)
			}

			// Verify isolation
			if len(cats) < 6 {
				t.Errorf("expected at least 6 basic categories, got %d", len(cats))
			}
		})
	}
}

// TestWithTransaction demonstrates using transactions in tests.
func TestWithTransaction(t *testing.T) {
	db := testutil.SetupTestDBWithBuilder(t, func(b categories.Builder) categories.Builder {
		return b.WithBasicCategories()
	})

	// Use transaction for atomic test operations
	err := db.WithTransaction(func(tx service.Transaction) error {
		ctx := context.Background()

		// All operations within transaction
		_, err := tx.CreateCategory(ctx, "Transaction Test Category")
		if err != nil {
			return err
		}

		// Transaction automatically rolls back after test
		// This is useful for testing rollback scenarios
		return nil
	})

	if err != nil {
		t.Fatalf("transaction failed: %v", err)
	}
}