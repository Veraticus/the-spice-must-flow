package integration_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/tui/integration"
	"github.com/Veraticus/the-spice-must-flow/internal/tui/integration/workflows"
	"github.com/stretchr/testify/require"
)

func TestClassificationWorkflows(t *testing.T) {
	// Test data shared across subtests
	testTransactions := []model.Transaction{
		{
			ID:           "txn_grocery_1",
			AccountID:    "acc1",
			MerchantName: "Whole Foods Market",
			Amount:       -156.78,
			Date:         time.Now().Add(-24 * time.Hour),
			Type:         "PURCHASE",
		},
		{
			ID:           "txn_entertainment_1",
			AccountID:    "acc1",
			MerchantName: "Netflix",
			Amount:       -15.99,
			Date:         time.Now().Add(-48 * time.Hour),
			Type:         "SUBSCRIPTION",
		},
		{
			ID:           "txn_transport_1",
			AccountID:    "acc1",
			MerchantName: "Uber",
			Amount:       -23.45,
			Date:         time.Now().Add(-72 * time.Hour),
			Type:         "PURCHASE",
		},
		{
			ID:           "txn_dining_1",
			AccountID:    "acc1",
			MerchantName: "Chipotle Mexican Grill",
			Amount:       -12.85,
			Date:         time.Now().Add(-96 * time.Hour),
			Type:         "PURCHASE",
		},
		{
			ID:           "txn_shopping_1",
			AccountID:    "acc1",
			MerchantName: "Amazon.com",
			Amount:       -78.99,
			Date:         time.Now().Add(-120 * time.Hour),
			Type:         "PURCHASE",
		},
	}

	testCategories := []model.Category{
		{ID: 1, Name: "Groceries"},
		{ID: 2, Name: "Entertainment"},
		{ID: 3, Name: "Transportation"},
		{ID: 4, Name: "Dining Out"},
		{ID: 5, Name: "Shopping"},
		{ID: 6, Name: "Healthcare"},
		{ID: 7, Name: "Utilities"},
	}

	t.Run("SingleTransactionClassification", func(t *testing.T) {
		harness := integration.NewHarness(t,
			integration.WithTestData(integration.TestData{
				Transactions: testTransactions,
				Categories:   testCategories,
			}),
		)
		defer harness.Cleanup()

		require.NoError(t, harness.Start())

		// Set up AI suggestions for Whole Foods
		harness.SetAISuggestions("txn_grocery_1", model.CategoryRankings{
			model.CategoryRanking{Category: "Groceries", Score: 0.95},
			model.CategoryRanking{Category: "Shopping", Score: 0.30},
		})

		// Execute workflow
		workflows.NewBuilder(t, harness).
			WithName("Classify Whole Foods transaction").
			NavigateToTransaction("txn_grocery_1").
			StartClassification().
			WaitFor(100*time.Millisecond). // Wait for AI suggestions
			AssertCustom("Verify AI suggestions displayed", func(t *testing.T, a *integration.Assertions) {
				t.Helper()
				a.AssertCategorySuggestion(t, "Groceries", 1)
				a.AssertCategorySuggestion(t, "Shopping", 2)
			}).
			SelectCategorySuggestion(1). // Select Groceries
			AcceptClassification().
			AssertNotification("success", "saved").
			AssertClassificationSaved("txn_grocery_1", "Groceries").
			Execute()

		// Verify save was called
		require.Equal(t, 1, harness.GetStorage().GetSaveCallCount())
	})

	t.Run("CustomCategoryClassification", func(t *testing.T) {
		harness := integration.NewHarness(t,
			integration.WithTestData(integration.TestData{
				Transactions: testTransactions,
				Categories:   testCategories,
			}),
		)
		defer harness.Cleanup()

		require.NoError(t, harness.Start())

		// Execute workflow with custom category
		workflows.NewBuilder(t, harness).
			WithName("Classify with custom category").
			NavigateToTransaction("txn_shopping_1").
			StartClassification().
			CustomClassification("Online Shopping").
			WaitFor(200*time.Millisecond). // Wait for command execution
			AssertNotification("success", "saved").
			AssertClassificationSaved("txn_shopping_1", "Online Shopping").
			Execute()
	})

	t.Run("SkipTransaction", func(t *testing.T) {
		harness := integration.NewHarness(t,
			integration.WithTestData(integration.TestData{
				Transactions: testTransactions,
				Categories:   testCategories,
			}),
		)
		defer harness.Cleanup()

		require.NoError(t, harness.Start())

		// Execute workflow
		workflows.NewBuilder(t, harness).
			WithName("Skip transaction").
			NavigateToTransaction("txn_transport_1").
			StartClassification().
			SkipTransaction().
			AssertCustom("Verify transaction not classified", func(t *testing.T, _ *integration.Assertions) {
				t.Helper()
				// Transaction should remain unclassified
				_, ok := harness.GetStorage().GetClassification("txn_transport_1")
				require.False(t, ok, "transaction should not be classified")
			}).
			Execute()

		// Verify no save was called
		require.Equal(t, 0, harness.GetStorage().GetSaveCallCount())
	})

	t.Run("BatchClassification", func(t *testing.T) {
		harness := integration.NewHarness(t,
			integration.WithTestData(integration.TestData{
				Transactions: testTransactions,
				Categories:   testCategories,
			}),
		)
		defer harness.Cleanup()

		require.NoError(t, harness.Start())

		// Set up AI suggestions for all transactions
		for i, txn := range testTransactions {
			// Use different categories for variety
			categories := []string{"Groceries", "Entertainment", "Transportation", "Dining Out", "Shopping"}
			harness.SetAISuggestions(txn.ID, model.CategoryRankings{
				model.CategoryRanking{Category: categories[i%len(categories)], Score: 0.90},
			})
		}

		// Execute batch workflow
		workflows.NewBuilder(t, harness).
			WithName("Batch accept all AI suggestions").
			StartBatchMode().
			WaitFor(200*time.Millisecond). // Wait for AI suggestions to load
			AssertCustom("Verify batch mode active", func(t *testing.T, a *integration.Assertions) {
				t.Helper()
				// Batch mode should include all 5 unclassified transactions
				a.AssertBatchModeActive(t, 5)
			}).
			Custom("Accept all AI suggestions", func(h *integration.Harness) error {
				h.SendKeys("a") // Press 'a' to accept all AI suggestions
				return nil
			}).
			WaitFor(100*time.Millisecond).
			Custom("Confirm acceptance", func(h *integration.Harness) error {
				h.SendKeys("y") // Press 'y' to confirm
				return nil
			}).
			WaitFor(200*time.Millisecond).
			AssertNotification("success", "5 transactions").
			WaitFor(200*time.Millisecond).
			AssertCustom("Verify all classified with AI suggestions", func(t *testing.T, _ *integration.Assertions) {
				t.Helper()
				// Check all 5 transactions are classified with their AI suggestions
				categories := []string{"Groceries", "Entertainment", "Transportation", "Dining Out", "Shopping"}
				for i := 0; i < 5; i++ {
					txnID := testTransactions[i].ID
					classification, ok := harness.GetStorage().GetClassification(txnID)
					require.True(t, ok, "transaction %s should be classified", txnID)
					require.Equal(t, categories[i], classification.Category)
				}
			}).
			Execute()

		// Verify correct number of saves
		require.Equal(t, 5, harness.GetStorage().GetSaveCallCount())
	})

	t.Run("SearchAndClassify", func(t *testing.T) {
		harness := integration.NewHarness(t,
			integration.WithTestData(integration.TestData{
				Transactions: testTransactions,
				Categories:   testCategories,
			}),
		)
		defer harness.Cleanup()

		require.NoError(t, harness.Start())

		// Set up AI suggestions for Netflix
		harness.SetAISuggestions("txn_entertainment_1", model.CategoryRankings{
			model.CategoryRanking{Category: "Entertainment", Score: 0.98},
		})

		// Execute search workflow
		workflows.NewBuilder(t, harness).
			WithName("Search for Netflix and classify").
			SearchFor("Netflix").
			WaitFor(200*time.Millisecond). // Wait for search results
			AssertCustom("Verify search results", func(t *testing.T, a *integration.Assertions) {
				t.Helper()
				a.AssertScreenMatches(t, "Netflix", "15.99")
			}).
			StartClassification().
			SelectCategorySuggestion(1). // Select Entertainment
			AcceptClassification().
			AssertClassificationSaved("txn_entertainment_1", "Entertainment").
			Execute()
	})

	t.Run("MultiStepClassification", func(t *testing.T) {
		harness := integration.NewHarness(t,
			integration.WithTestData(integration.TestData{
				Transactions: testTransactions,
				Categories:   testCategories,
			}),
		)
		defer harness.Cleanup()

		require.NoError(t, harness.Start())

		// Set up AI suggestions
		harness.SetAISuggestions("txn_grocery_1", model.CategoryRankings{
			model.CategoryRanking{Category: "Groceries", Score: 0.95},
		})
		harness.SetAISuggestions("txn_transport_1", model.CategoryRankings{
			model.CategoryRanking{Category: "Transportation", Score: 0.88},
		})

		// Execute multi-step workflow
		workflows.NewBuilder(t, harness).
			WithName("Classify multiple transactions in sequence").
			// First transaction
			NavigateToTransaction("txn_grocery_1").
			StartClassification().
			SelectCategorySuggestion(1).
			AcceptClassification().
			AssertClassificationSaved("txn_grocery_1", "Groceries").
			WaitFor(100*time.Millisecond).
			// Second transaction
			NavigateToTransaction("txn_transport_1").
			StartClassification().
			SelectCategorySuggestion(1).
			AcceptClassification().
			AssertClassificationSaved("txn_transport_1", "Transportation").
			// Verify final state
			AssertCustom("Verify save count", func(t *testing.T, _ *integration.Assertions) {
				t.Helper()
				require.Equal(t, 2, harness.GetStorage().GetSaveCallCount())
			}).
			Execute()
	})

	t.Run("ClassificationWithUndo", func(t *testing.T) {
		harness := integration.NewHarness(t,
			integration.WithTestData(integration.TestData{
				Transactions: testTransactions,
				Categories:   testCategories,
			}),
		)
		defer harness.Cleanup()

		require.NoError(t, harness.Start())

		// Execute workflow with undo
		workflows.NewBuilder(t, harness).
			WithName("Classify then undo").
			NavigateToTransaction("txn_dining_1").
			StartClassification().
			SelectCategorySuggestion(4). // Select Dining Out
			AcceptClassification().
			AssertClassificationSaved("txn_dining_1", "Dining Out").
			WaitFor(100*time.Millisecond).
			Custom("Undo last classification", func(h *integration.Harness) error {
				h.SendKeys("u") // Undo key
				return nil
			}).
			AssertNotification("info", "Undo").
			AssertCustom("Verify classification removed", func(t *testing.T, _ *integration.Assertions) {
				t.Helper()
				// Should be unclassified again
				_, ok := harness.GetStorage().GetClassification("txn_dining_1")
				require.False(t, ok, "classification should be undone")
			}).
			Execute()
	})

	t.Run("ErrorHandling", func(t *testing.T) {
		harness := integration.NewHarness(t,
			integration.WithTestData(integration.TestData{
				Transactions: testTransactions,
				Categories:   testCategories,
			}),
		)
		defer harness.Cleanup()

		require.NoError(t, harness.Start())

		// Set up AI suggestions for Amazon.com
		harness.SetAISuggestions("txn_shopping_1", model.CategoryRankings{
			model.CategoryRanking{Category: "Shopping", Score: 0.95},
			model.CategoryRanking{Category: "Online Shopping", Score: 0.80},
			model.CategoryRanking{Category: "Retail", Score: 0.60},
			model.CategoryRanking{Category: "E-commerce", Score: 0.50},
			model.CategoryRanking{Category: "General", Score: 0.30},
		})

		// Configure storage to fail
		harness.GetStorage().SetSaveError(fmt.Errorf("database error"))

		// Execute workflow expecting error
		workflows.NewBuilder(t, harness).
			WithName("Handle classification error").
			NavigateToTransaction("txn_shopping_1").
			StartClassification().
			SelectCategorySuggestion(1). // Select Shopping (first suggestion)
			AcceptClassification().
			WaitFor(100*time.Millisecond).
			AssertNotification("error", "database error").
			AssertCustom("Verify no classification saved", func(t *testing.T, _ *integration.Assertions) {
				t.Helper()
				_, ok := harness.GetStorage().GetClassification("txn_shopping_1")
				require.False(t, ok, "classification should not be saved on error")
			}).
			Execute()
	})
}

// TestClassificationPerformance tests classification performance under load.
func TestClassificationPerformance(t *testing.T) {
	// Generate large dataset
	var transactions []model.Transaction
	for i := 0; i < 1000; i++ {
		transactions = append(transactions, model.Transaction{
			ID:           fmt.Sprintf("txn_%d", i),
			AccountID:    "acc1",
			MerchantName: fmt.Sprintf("Merchant %d", i),
			Amount:       -float64(i),
			Date:         time.Now().Add(-time.Duration(i) * time.Hour),
			Type:         "PURCHASE",
		})
	}

	categories := []model.Category{
		{ID: 1, Name: "Category 1"},
		{ID: 2, Name: "Category 2"},
		{ID: 3, Name: "Category 3"},
	}

	harness := integration.NewHarness(t,
		integration.WithTestData(integration.TestData{
			Transactions: transactions,
			Categories:   categories,
		}),
		integration.WithSize(120, 40), // Larger terminal
	)
	defer harness.Cleanup()

	require.NoError(t, harness.Start())

	// Set default AI rankings
	harness.SetDefaultAISuggestions(model.CategoryRankings{
		model.CategoryRanking{Category: "Category 1", Score: 0.8},
		model.CategoryRanking{Category: "Category 2", Score: 0.6},
		model.CategoryRanking{Category: "Category 3", Score: 0.4},
	})

	// Measure time to classify 10 transactions
	start := time.Now()

	builder := workflows.NewBuilder(t, harness).
		WithName("Performance test").
		WithTimeout(30 * time.Second)

	// Classify first 10 transactions
	for i := 0; i < 10; i++ {
		txnID := fmt.Sprintf("txn_%d", i)
		builder = builder.
			NavigateToTransaction(txnID).
			StartClassification().
			SelectCategorySuggestion(1).
			AcceptClassification()
	}

	builder.Execute()

	elapsed := time.Since(start)
	t.Logf("Classified 10 transactions in %v", elapsed)

	// Should complete in reasonable time
	require.Less(t, elapsed, 5*time.Second, "classification should be fast")
}
