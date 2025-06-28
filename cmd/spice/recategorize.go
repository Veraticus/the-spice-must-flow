package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/cli"
	"github.com/Veraticus/the-spice-must-flow/internal/engine"
	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/service"
	"github.com/spf13/cobra"
)

func recategorizeCmd() *cobra.Command {
	var (
		fromDate  string
		toDate    string
		category  string
		merchant  string
		force     bool
		dryRun    bool
		batchSize int
	)

	cmd := &cobra.Command{
		Use:   "recategorize",
		Short: "Bulk recategorize existing transactions",
		Long: `Recategorize existing transactions based on various criteria.
This command allows you to re-run categorization on already classified transactions.

Examples:
  # Recategorize all transactions from 2024
  spice recategorize --from 2024-01-01 --to 2024-12-31
  
  # Recategorize only Miscellaneous transactions
  spice recategorize --category "Miscellaneous"
  
  # Recategorize all Amazon transactions
  spice recategorize --merchant "AMAZON"
  
  # Dry run to see what would be recategorized
  spice recategorize --category "Other" --dry-run
  
  # Force recategorization without confirmation
  spice recategorize --from 2024-01-01 --force`,
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx := context.Background()

			// Validate date inputs
			var fromTime, toTime *time.Time
			if fromDate != "" {
				parsed, err := time.Parse("2006-01-02", fromDate)
				if err != nil {
					return fmt.Errorf("invalid from date format (use YYYY-MM-DD): %w", err)
				}
				fromTime = &parsed
			}
			if toDate != "" {
				parsed, err := time.Parse("2006-01-02", toDate)
				if err != nil {
					return fmt.Errorf("invalid to date format (use YYYY-MM-DD): %w", err)
				}
				// Set to end of day
				endOfDay := parsed.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
				toTime = &endOfDay
			}

			// Ensure from date is before to date
			if fromTime != nil && toTime != nil && fromTime.After(*toTime) {
				return fmt.Errorf("from date must be before to date")
			}

			// Initialize storage
			store, err := initStorage(ctx)
			if err != nil {
				return err
			}
			defer func() {
				if closeErr := store.Close(); closeErr != nil {
					slog.Error("failed to close storage", "error", closeErr)
				}
			}()

			// Find transactions to recategorize
			transactions, err := findTransactionsToRecategorize(ctx, store, fromTime, toTime, category, merchant)
			if err != nil {
				return fmt.Errorf("failed to find transactions: %w", err)
			}

			if len(transactions) == 0 {
				fmt.Println(cli.InfoStyle.Render("No transactions found matching criteria")) //nolint:forbidigo // User-facing output
				return nil
			}

			fmt.Printf("Found %d transactions to recategorize\n", len(transactions)) //nolint:forbidigo // User-facing output

			// Show summary
			showRecategorizationSummary(transactions)

			if dryRun {
				fmt.Println(cli.InfoStyle.Render("\n🔍 Dry run complete - no changes made")) //nolint:forbidigo // User-facing output
				return nil
			}

			// Confirm action
			if !force {
				fmt.Printf("\nAre you sure you want to recategorize %d transactions? (y/N): ", len(transactions)) //nolint:forbidigo // User prompt
				var response string
				if _, scanErr := fmt.Scanln(&response); scanErr != nil {
					response = "n"
				}
				if strings.ToLower(response) != "y" {
					fmt.Println("Recategorization canceled.") //nolint:forbidigo // User-facing output
					return nil
				}
			}

			// Initialize LLM classifier
			classifier, err := createLLMClient()
			if err != nil {
				return fmt.Errorf("failed to initialize LLM: %w", err)
			}
			if closer, ok := classifier.(interface{ Close() error }); ok {
				defer func() {
					if closeErr := closer.Close(); closeErr != nil {
						slog.Error("failed to close LLM client", "error", closeErr)
					}
				}()
			}

			// Initialize prompter
			prompter := cli.NewCLIPrompter(nil, nil)

			// Create classification engine with custom batch size
			engineConfig := engine.DefaultConfig()
			if batchSize > 0 {
				engineConfig.BatchSize = batchSize
			}
			classificationEngine := engine.NewWithConfig(store, classifier, prompter, engineConfig)

			// Process transactions
			fmt.Println(cli.InfoStyle.Render("\n🔄 Starting recategorization...")) //nolint:forbidigo // User-facing output

			// Clear existing classifications for all transactions
			// We do this upfront to ensure they're treated as unclassified
			for _, txn := range transactions {
				// Mark as unclassified by setting empty category
				classification := model.Classification{
					Transaction:  txn,
					Category:     "",
					Status:       model.StatusUnclassified,
					ClassifiedAt: time.Now(),
				}
				if err := store.SaveClassification(ctx, &classification); err != nil {
					slog.Warn("Failed to clear classification",
						"transaction_id", txn.ID,
						"error", err)
				}
			}

			// Run classification engine on ONLY these specific transactions
			fmt.Println(cli.InfoStyle.Render("\n🤖 Running AI classification...")) //nolint:forbidigo // User-facing output

			// Use batch classification options
			batchOpts := engine.BatchClassificationOptions{
				AutoAcceptThreshold: 0.95, // High threshold for auto-acceptance in recategorization
				BatchSize:           batchSize,
				ParallelWorkers:     2,
				SkipManualReview:    false, // Always allow manual review for recategorization
			}

			summary, err := classificationEngine.ClassifySpecificTransactions(ctx, transactions, batchOpts)
			if err != nil {
				return fmt.Errorf("classification failed: %w", err)
			}

			// Show summary
			fmt.Printf("\n📊 Recategorization Summary:\n")                       //nolint:forbidigo // User-facing output
			fmt.Printf("  Total transactions: %d\n", summary.TotalTransactions) //nolint:forbidigo // User-facing output
			fmt.Printf("  Auto-accepted: %d\n", summary.AutoAcceptedTxns)       //nolint:forbidigo // User-facing output
			fmt.Printf("  Manually reviewed: %d\n", summary.NeedsReviewTxns)    //nolint:forbidigo // User-facing output
			if summary.FailedCount > 0 {
				fmt.Printf("  Failed: %d\n", summary.FailedCount) //nolint:forbidigo // User-facing output
			}

			fmt.Println(cli.SuccessStyle.Render("\n✓ Recategorization complete!")) //nolint:forbidigo // User-facing output

			return nil
		},
	}

	cmd.Flags().StringVar(&fromDate, "from", "", "Start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&toDate, "to", "", "End date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&category, "category", "", "Recategorize only transactions in this category")
	cmd.Flags().StringVar(&merchant, "merchant", "", "Recategorize only transactions from this merchant")
	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompt")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without applying them")
	cmd.Flags().IntVar(&batchSize, "batch-size", 50, "Number of transactions to process at once")

	return cmd
}

func findTransactionsToRecategorize(ctx context.Context, store service.Storage, fromDate, toDate *time.Time, category, merchant string) ([]model.Transaction, error) {
	var transactions []model.Transaction

	switch {
	case category != "":
		// Get transactions by category
		txns, err := store.GetTransactionsByCategory(ctx, category)
		if err != nil {
			return nil, fmt.Errorf("failed to get transactions by category: %w", err)
		}
		transactions = txns
	case fromDate != nil || toDate != nil:
		// Get transactions by date range
		// Default to all time if dates not specified
		start := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
		end := time.Now().Add(24 * time.Hour)

		if fromDate != nil {
			start = *fromDate
		}
		if toDate != nil {
			end = *toDate
		}

		// Get classifications in date range
		classifications, err := store.GetClassificationsByDateRange(ctx, start, end)
		if err != nil {
			return nil, fmt.Errorf("failed to get classifications by date: %w", err)
		}

		// Extract transactions
		for _, c := range classifications {
			transactions = append(transactions, c.Transaction)
		}
	default:
		// Get all classified transactions
		start := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
		end := time.Now().Add(24 * time.Hour)

		classifications, err := store.GetClassificationsByDateRange(ctx, start, end)
		if err != nil {
			return nil, fmt.Errorf("failed to get all classifications: %w", err)
		}

		for _, c := range classifications {
			transactions = append(transactions, c.Transaction)
		}
	}

	// Filter by merchant if specified
	if merchant != "" {
		filtered := make([]model.Transaction, 0)
		merchantLower := strings.ToLower(merchant)
		for _, txn := range transactions {
			if strings.Contains(strings.ToLower(txn.MerchantName), merchantLower) ||
				strings.Contains(strings.ToLower(txn.Name), merchantLower) {
				filtered = append(filtered, txn)
			}
		}
		transactions = filtered
	}

	// Apply date filters if we got transactions by category
	if category != "" && (fromDate != nil || toDate != nil) {
		filtered := make([]model.Transaction, 0)
		for _, txn := range transactions {
			if fromDate != nil && txn.Date.Before(*fromDate) {
				continue
			}
			if toDate != nil && txn.Date.After(*toDate) {
				continue
			}
			filtered = append(filtered, txn)
		}
		transactions = filtered
	}

	return transactions, nil
}

func showRecategorizationSummary(transactions []model.Transaction) {
	// Calculate statistics
	categoryMap := make(map[string]int)
	merchantMap := make(map[string]int)
	var totalAmount float64
	var oldestDate, newestDate time.Time

	for i, txn := range transactions {
		if len(txn.Category) > 0 {
			categoryMap[txn.Category[0]]++
		}
		merchantMap[txn.MerchantName]++
		totalAmount += txn.Amount

		if i == 0 || txn.Date.Before(oldestDate) {
			oldestDate = txn.Date
		}
		if i == 0 || txn.Date.After(newestDate) {
			newestDate = txn.Date
		}
	}

	fmt.Println("\n📊 Summary of transactions to recategorize:")                                              //nolint:forbidigo // User-facing output
	fmt.Printf("  Date range: %s to %s\n", oldestDate.Format("2006-01-02"), newestDate.Format("2006-01-02")) //nolint:forbidigo // User-facing output
	fmt.Printf("  Total amount: $%.2f\n", totalAmount)                                                       //nolint:forbidigo // User-facing output
	fmt.Printf("  Unique merchants: %d\n", len(merchantMap))                                                 //nolint:forbidigo // User-facing output

	if len(categoryMap) > 0 {
		fmt.Println("\n  Current categories:") //nolint:forbidigo // User-facing output
		for cat, count := range categoryMap {
			fmt.Printf("    - %s: %d transactions\n", cat, count) //nolint:forbidigo // User-facing output
		}
	}
}
