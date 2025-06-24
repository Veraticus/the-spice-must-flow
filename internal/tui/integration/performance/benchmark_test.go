package performance_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/tui/integration"
)

// BenchmarkTransactionListScrolling measures scrolling performance with large datasets.
func BenchmarkTransactionListScrolling(b *testing.B) {
	benchmarkSizes := []int{100, 500, 1000, 5000, 10000}

	for _, size := range benchmarkSizes {
		b.Run(fmt.Sprintf("Size%d", size), func(b *testing.B) {
			// Generate test data
			transactions := generateTransactions(size)

			harness := integration.NewHarness(&testing.T{},
				integration.WithTestData(integration.TestData{
					Transactions: transactions,
					Categories:   generateCategories(),
				}),
				integration.WithSize(120, 40), // Standard terminal size
			)
			defer harness.Cleanup()

			if err := harness.Start(); err != nil {
				b.Fatalf("failed to start harness: %v", err)
			}

			// Wait for initial render
			time.Sleep(100 * time.Millisecond)

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				// Scroll to bottom
				harness.SendKeys("G")
				// Scroll to top
				harness.SendKeys("g")
			}

			b.StopTimer()
		})
	}
}

// BenchmarkClassificationWorkflow measures end-to-end classification performance.
func BenchmarkClassificationWorkflow(b *testing.B) {
	transactions := generateTransactions(100)
	categories := generateCategories()

	harness := integration.NewHarness(&testing.T{},
		integration.WithTestData(integration.TestData{
			Transactions: transactions,
			Categories:   categories,
		}),
	)
	defer harness.Cleanup()

	if err := harness.Start(); err != nil {
		b.Fatalf("failed to start harness: %v", err)
	}

	// Set up default AI suggestions
	harness.SetDefaultAISuggestions(model.CategoryRankings{
		model.CategoryRanking{Category: "Groceries", Score: 0.8},
		model.CategoryRanking{Category: "Entertainment", Score: 0.6},
	})

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		txnID := transactions[i%len(transactions)].ID

		// Navigate to transaction
		if err := harness.SelectTransaction(txnID); err != nil {
			b.Fatalf("failed to select transaction: %v", err)
		}

		// Start classification
		harness.SendKeys("enter")
		time.Sleep(50 * time.Millisecond)

		// Select first suggestion
		harness.SendKeys("1")

		// Accept classification
		harness.SendKeys("a")
		time.Sleep(50 * time.Millisecond)
	}

	b.StopTimer()
}

// BenchmarkSearchPerformance measures search functionality performance.
func BenchmarkSearchPerformance(b *testing.B) {
	benchmarkSizes := []int{100, 1000, 5000}
	searchQueries := []string{"Amazon", "Food", "123", "Transport"}

	for _, size := range benchmarkSizes {
		for _, query := range searchQueries {
			b.Run(fmt.Sprintf("Size%d_Query%s", size, query), func(b *testing.B) {
				transactions := generateTransactions(size)

				harness := integration.NewHarness(&testing.T{},
					integration.WithTestData(integration.TestData{
						Transactions: transactions,
						Categories:   generateCategories(),
					}),
				)
				defer harness.Cleanup()

				if err := harness.Start(); err != nil {
					b.Fatalf("failed to start harness: %v", err)
				}

				b.ResetTimer()

				for i := 0; i < b.N; i++ {
					// Start search
					harness.SendKeys("/")
					time.Sleep(20 * time.Millisecond)

					// Type query
					for _, char := range query {
						harness.SendKeys(string(char))
					}

					// Execute search
					harness.SendKeys("enter")
					time.Sleep(50 * time.Millisecond)

					// Clear search
					harness.SendKeys("esc")
				}

				b.StopTimer()
			})
		}
	}
}

// BenchmarkBatchClassification measures batch operation performance.
func BenchmarkBatchClassification(b *testing.B) {
	batchSizes := []int{10, 50, 100}

	for _, batchSize := range batchSizes {
		b.Run(fmt.Sprintf("Batch%d", batchSize), func(b *testing.B) {
			transactions := generateTransactions(batchSize * 2) // Extra for multiple runs

			harness := integration.NewHarness(&testing.T{},
				integration.WithTestData(integration.TestData{
					Transactions: transactions,
					Categories:   generateCategories(),
				}),
			)
			defer harness.Cleanup()

			if err := harness.Start(); err != nil {
				b.Fatalf("failed to start harness: %v", err)
			}

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				// Enter batch mode
				harness.SendKeys("b")
				time.Sleep(50 * time.Millisecond)

				// Select visual range
				harness.SendKeys("v")

				// Select batch size items
				for j := 0; j < batchSize-1; j++ {
					harness.SendKeys("j")
				}

				// Apply category
				harness.SendKeys("1") // First category

				// Confirm
				harness.SendKeys("y")
				time.Sleep(100 * time.Millisecond)
			}

			b.StopTimer()
		})
	}
}

// BenchmarkUIRendering measures rendering performance under various conditions.
func BenchmarkUIRendering(b *testing.B) {
	scenarios := []struct {
		name   string
		width  int
		height int
		txns   int
	}{
		{"Compact_Small", 80, 24, 100},
		{"Compact_Large", 80, 24, 1000},
		{"Medium_Small", 120, 40, 100},
		{"Medium_Large", 120, 40, 1000},
		{"Full_Small", 160, 50, 100},
		{"Full_Large", 160, 50, 1000},
	}

	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			transactions := generateTransactions(scenario.txns)

			harness := integration.NewHarness(&testing.T{},
				integration.WithTestData(integration.TestData{
					Transactions: transactions,
					Categories:   generateCategories(),
				}),
				integration.WithSize(scenario.width, scenario.height),
			)
			defer harness.Cleanup()

			if err := harness.Start(); err != nil {
				b.Fatalf("failed to start harness: %v", err)
			}

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				// Force re-render by navigating
				harness.SendKeys("j")
				harness.SendKeys("k")
			}

			b.StopTimer()
		})
	}
}

// BenchmarkMemoryUsage measures memory allocation during typical operations.
func BenchmarkMemoryUsage(b *testing.B) {
	transactions := generateTransactions(1000)

	b.Run("ScrollingMemory", func(b *testing.B) {
		harness := integration.NewHarness(&testing.T{},
			integration.WithTestData(integration.TestData{
				Transactions: transactions,
				Categories:   generateCategories(),
			}),
		)
		defer harness.Cleanup()

		if err := harness.Start(); err != nil {
			b.Fatalf("failed to start harness: %v", err)
		}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			// Perform typical scrolling operations
			harness.SendKeys("j", "j", "j", "k", "k", "k")
		}
	})

	b.Run("ClassificationMemory", func(b *testing.B) {
		harness := integration.NewHarness(&testing.T{},
			integration.WithTestData(integration.TestData{
				Transactions: transactions,
				Categories:   generateCategories(),
			}),
		)
		defer harness.Cleanup()

		if err := harness.Start(); err != nil {
			b.Fatalf("failed to start harness: %v", err)
		}

		harness.SetDefaultAISuggestions(model.CategoryRankings{
			model.CategoryRanking{Category: "Test", Score: 0.8},
		})

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			// Classify a transaction
			harness.SendKeys("enter", "1", "a")
			time.Sleep(50 * time.Millisecond)
		}
	})
}

// Helper functions

func generateTransactions(count int) []model.Transaction {
	merchants := []string{
		"Amazon", "Whole Foods", "Netflix", "Uber", "Target",
		"Starbucks", "Shell", "CVS", "Home Depot", "Chipotle",
	}

	var transactions []model.Transaction
	baseDate := time.Now()

	for i := 0; i < count; i++ {
		merchant := merchants[i%len(merchants)]
		transactions = append(transactions, model.Transaction{
			ID:           fmt.Sprintf("txn_%d", i),
			AccountID:    "acc1",
			MerchantName: fmt.Sprintf("%s #%d", merchant, i),
			Amount:       -float64(10 + (i % 100)),
			Date:         baseDate.Add(-time.Duration(i) * time.Hour),
			Type:         "PURCHASE",
		})
	}

	return transactions
}

func generateCategories() []model.Category {
	categoryNames := []string{
		"Groceries", "Entertainment", "Transportation", "Dining Out",
		"Shopping", "Healthcare", "Utilities", "Home Supplies",
	}

	categories := make([]model.Category, 0, len(categoryNames))
	for i, name := range categoryNames {
		categories = append(categories, model.Category{
			ID:   i + 1,
			Name: name,
		})
	}

	return categories
}
