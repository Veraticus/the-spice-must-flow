package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/cli"
	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/service"
	"github.com/Veraticus/the-spice-must-flow/internal/storage"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func recategorizeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "recategorize",
		Short: "Bulk update transaction categorizations",
		Long: `Recategorize transactions in bulk with pattern matching and direction fixes.

This command allows you to:
- Fix transaction directions based on patterns
- Move transactions between categories
- Review and update categorizations interactively`,
		RunE: runRecategorize,
	}

	// Flags
	cmd.Flags().Bool("fix-direction", false, "Fix transaction directions based on patterns")
	cmd.Flags().String("from-category", "", "Source category to move transactions from")
	cmd.Flags().String("to-category", "", "Target category to move transactions to")
	cmd.Flags().BoolP("interactive", "i", false, "Review changes interactively")
	cmd.Flags().Bool("dry-run", false, "Preview changes without saving")
	cmd.Flags().IntP("year", "y", time.Now().Year(), "Year to recategorize")
	cmd.Flags().StringP("month", "m", "", "Specific month to recategorize (format: 2024-01)")

	// Bind to viper
	_ = viper.BindPFlag("recategorize.fix_direction", cmd.Flags().Lookup("fix-direction"))
	_ = viper.BindPFlag("recategorize.from_category", cmd.Flags().Lookup("from-category"))
	_ = viper.BindPFlag("recategorize.to_category", cmd.Flags().Lookup("to-category"))
	_ = viper.BindPFlag("recategorize.interactive", cmd.Flags().Lookup("interactive"))
	_ = viper.BindPFlag("recategorize.dry_run", cmd.Flags().Lookup("dry-run"))
	_ = viper.BindPFlag("recategorize.year", cmd.Flags().Lookup("year"))
	_ = viper.BindPFlag("recategorize.month", cmd.Flags().Lookup("month"))

	return cmd
}

func runRecategorize(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	fixDirection := viper.GetBool("recategorize.fix_direction")
	fromCategory := viper.GetString("recategorize.from_category")
	toCategory := viper.GetString("recategorize.to_category")
	interactive := viper.GetBool("recategorize.interactive")
	dryRun := viper.GetBool("recategorize.dry_run")
	year := viper.GetInt("recategorize.year")
	month := viper.GetString("recategorize.month")

	// Validate flags
	if !fixDirection && fromCategory == "" {
		return fmt.Errorf("specify either --fix-direction or --from-category")
	}

	if fromCategory != "" && toCategory == "" {
		return fmt.Errorf("--to-category is required when using --from-category")
	}

	// Set up interrupt handling
	interruptHandler := cli.NewInterruptHandler(nil)
	ctx = interruptHandler.HandleInterrupts(ctx, true)

	slog.Info("Starting recategorization")

	// Initialize storage
	dbPath := viper.GetString("database.path")
	if dbPath == "" {
		dbPath = "$HOME/.local/share/spice/spice.db"
	}
	dbPath = os.ExpandEnv(dbPath)

	db, err := storage.NewSQLiteStorage(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			slog.Error("Failed to close database", "error", closeErr)
		}
	}()

	// Run migrations
	if migrateErr := db.Migrate(ctx); migrateErr != nil {
		return fmt.Errorf("failed to run migrations: %w", migrateErr)
	}

	// Calculate date range
	var start, end time.Time
	if month != "" {
		parsed, err := time.Parse("2006-01", month)
		if err != nil {
			return fmt.Errorf("invalid month format '%s', expected YYYY-MM: %w", month, err)
		}
		start = parsed
		end = parsed.AddDate(0, 1, -1).Add(23*time.Hour + 59*time.Minute + 59*time.Second)
	} else {
		start = time.Date(year, 1, 1, 0, 0, 0, 0, time.Local)
		end = time.Date(year, 12, 31, 23, 59, 59, 999999999, time.Local)
	}

	if fixDirection {
		return fixTransactionDirections(ctx, db, start, end, interactive, dryRun)
	}

	return recategorizeTransactions(ctx, db, fromCategory, toCategory, start, end, interactive, dryRun)
}

func fixTransactionDirections(ctx context.Context, db service.Storage, start, end time.Time, interactive, dryRun bool) error {
	// Get all classifications for the period
	classifications, err := db.GetClassificationsByDateRange(ctx, start, end)
	if err != nil {
		return fmt.Errorf("failed to get classifications: %w", err)
	}

	// Load direction patterns from config
	incomePatterns := viper.GetStringSlice("classification.income_patterns")
	transferPatterns := viper.GetStringSlice("classification.transfer_patterns")

	// If no patterns configured, use defaults
	if len(incomePatterns) == 0 {
		incomePatterns = []string{
			"PAYROLL", "SALARY", "DIRECT DEP", "INTEREST", "DIVIDEND",
			"REIMBURSEMENT", "REFUND", "CASHBACK", "REWARDS",
		}
	}
	if len(transferPatterns) == 0 {
		transferPatterns = []string{
			"TRANSFER FROM", "TRANSFER TO", "XFER",
		}
	}

	// Find transactions that need direction fixes
	var toFix []model.Classification
	directionChanges := make(map[string]model.TransactionDirection)

	for _, class := range classifications {
		txn := class.Transaction
		suggestedDirection := detectDirection(txn, incomePatterns, transferPatterns)

		if suggestedDirection != "" && suggestedDirection != txn.Direction {
			toFix = append(toFix, class)
			directionChanges[txn.ID] = suggestedDirection
		}
	}

	if len(toFix) == 0 {
		fmt.Println(cli.FormatInfo("No transactions need direction fixes.")) //nolint:forbidigo
		return nil
	}

	// Show summary
	fmt.Printf("\n%s Found %d transactions with incorrect directions\n\n", cli.InfoIcon, len(toFix)) //nolint:forbidigo

	// Group by direction change
	changeGroups := make(map[string][]model.Classification)
	for _, class := range toFix {
		newDir := directionChanges[class.Transaction.ID]
		key := fmt.Sprintf("%s → %s", class.Transaction.Direction, newDir)
		changeGroups[key] = append(changeGroups[key], class)
	}

	// Display grouped changes
	for change, items := range changeGroups {
		fmt.Printf("%s %s (%d transactions)\n", cli.CheckIcon, change, len(items)) //nolint:forbidigo

		// Show examples
		for i, class := range items {
			if i >= 3 {
				fmt.Printf("  ... and %d more\n", len(items)-3) //nolint:forbidigo
				break
			}
			fmt.Printf("  • %s - %s ($%.2f)\n", //nolint:forbidigo
				class.Transaction.Date.Format("Jan 2"),
				class.Transaction.MerchantName,
				class.Transaction.Amount)
		}
		fmt.Println() //nolint:forbidigo
	}

	if dryRun {
		fmt.Println(cli.FormatWarning("DRY RUN - No changes will be saved")) //nolint:forbidigo
		return nil
	}

	// Confirm changes
	if interactive {
		fmt.Print(cli.FormatPrompt("Apply these direction fixes? [y/N]")) //nolint:forbidigo

		reader := cli.NewNonBlockingReader(os.Stdin)
		reader.Start(ctx)
		defer reader.Close()

		response, err := reader.ReadLine(ctx)
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}

		if strings.ToLower(response) != "y" {
			fmt.Println(cli.FormatWarning("Canceled")) //nolint:forbidigo
			return nil
		}
	}

	// Apply changes
	successCount := 0
	for txnID, newDirection := range directionChanges {
		if err := db.UpdateTransactionDirection(ctx, txnID, newDirection); err != nil {
			slog.Error("Failed to update transaction direction", "id", txnID, "error", err)
			continue
		}
		successCount++
	}

	fmt.Println(cli.FormatSuccess(fmt.Sprintf("✓ Updated %d transaction directions", successCount))) //nolint:forbidigo

	if successCount < len(directionChanges) {
		fmt.Println(cli.FormatWarning(fmt.Sprintf("⚠ Failed to update %d transactions", len(directionChanges)-successCount))) //nolint:forbidigo
	}

	return nil
}

func detectDirection(txn model.Transaction, incomePatterns, transferPatterns []string) model.TransactionDirection {
	upperName := strings.ToUpper(txn.Name)
	upperMerchant := strings.ToUpper(txn.MerchantName)

	// Check transfer patterns first
	for _, pattern := range transferPatterns {
		if strings.Contains(upperName, pattern) || strings.Contains(upperMerchant, pattern) {
			return model.DirectionTransfer
		}
	}

	// Check income patterns
	for _, pattern := range incomePatterns {
		if strings.Contains(upperName, pattern) || strings.Contains(upperMerchant, pattern) {
			return model.DirectionIncome
		}
	}

	// Check transaction type hints
	if strings.Contains(txn.Type, "deposit") || strings.Contains(txn.Type, "credit") {
		return model.DirectionIncome
	}

	// Default empty string means no change needed
	return ""
}

func recategorizeTransactions(ctx context.Context, db service.Storage, fromCategory, toCategory string, start, end time.Time, interactive, dryRun bool) error {
	// Get transactions in the source category
	classifications, err := db.GetClassificationsByDateRange(ctx, start, end)
	if err != nil {
		return fmt.Errorf("failed to get classifications: %w", err)
	}

	// Filter to only the source category
	var toMove []model.Classification
	for _, class := range classifications {
		if class.Category == fromCategory {
			toMove = append(toMove, class)
		}
	}

	if len(toMove) == 0 {
		fmt.Println(cli.FormatInfo(fmt.Sprintf("No transactions found in category '%s'", fromCategory))) //nolint:forbidigo
		return nil
	}

	// Show summary
	var totalAmount float64
	for _, class := range toMove {
		totalAmount += class.Transaction.Amount
	}

	content := fmt.Sprintf("Transactions to move: %d\n", len(toMove)) +
		fmt.Sprintf("Total amount: $%.2f\n", totalAmount) +
		fmt.Sprintf("From: %s\n", fromCategory) +
		fmt.Sprintf("To: %s\n\n", toCategory)

	// Show examples
	content += "Sample transactions:\n"
	for i, class := range toMove {
		if i >= 5 {
			content += fmt.Sprintf("... and %d more\n", len(toMove)-5)
			break
		}
		content += fmt.Sprintf("• %s - %s ($%.2f)\n",
			class.Transaction.Date.Format("Jan 2"),
			class.Transaction.MerchantName,
			class.Transaction.Amount)
	}

	fmt.Println(cli.RenderBox("Recategorization Summary", content)) //nolint:forbidigo

	if dryRun {
		fmt.Println(cli.FormatWarning("DRY RUN - No changes will be saved")) //nolint:forbidigo
		return nil
	}

	// Confirm changes
	if interactive {
		fmt.Print(cli.FormatPrompt("Proceed with recategorization? [y/N]")) //nolint:forbidigo

		reader := cli.NewNonBlockingReader(os.Stdin)
		reader.Start(ctx)
		defer reader.Close()

		response, err := reader.ReadLine(ctx)
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}

		if strings.ToLower(response) != "y" {
			fmt.Println(cli.FormatWarning("Canceled")) //nolint:forbidigo
			return nil
		}
	}

	// Apply changes
	if err := db.UpdateTransactionCategories(ctx, fromCategory, toCategory); err != nil {
		return fmt.Errorf("failed to update categories: %w", err)
	}

	fmt.Println(cli.FormatSuccess(fmt.Sprintf("✓ Moved %d transactions from '%s' to '%s'", //nolint:forbidigo
		len(toMove), fromCategory, toCategory)))

	return nil
}
