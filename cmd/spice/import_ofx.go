package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/classification"
	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/ofx"
	"github.com/spf13/cobra"
)

func init() {
	importOFXCmd := &cobra.Command{
		Use:   "import-ofx [files...]",
		Short: "Import transactions from OFX/QFX files",
		Long: `Import financial transactions from OFX or QFX (Quicken) files exported from your bank.

Examples:
  # Import single file
  spice import-ofx ~/Downloads/chase_jan_2024.qfx
  
  # Import multiple files
  spice import-ofx ~/Downloads/chase_*.qfx
  
  # Import all QFX files in a directory
  spice import-ofx ~/Downloads/*.qfx
  
  # Import from multiple directories
  spice import-ofx ~/Downloads/Chase/*.qfx ~/Downloads/Ally/*.qfx`,
		Args: cobra.MinimumNArgs(1),
		RunE: runImportOFX,
	}

	importOFXCmd.Flags().BoolP("dry-run", "d", false, "Preview import without saving")
	importOFXCmd.Flags().BoolP("verbose", "v", false, "Show detailed transaction data")

	rootCmd.AddCommand(importOFXCmd)
}

func runImportOFX(cmd *cobra.Command, args []string) error {
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	verbose, _ := cmd.Flags().GetBool("verbose")

	// Expand globs and collect all files
	var allFiles []string
	for _, pattern := range args {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return fmt.Errorf("invalid pattern %s: %w", pattern, err)
		}
		if len(matches) == 0 {
			// If no glob matches, check if it's a direct file
			if _, err := os.Stat(pattern); err == nil {
				allFiles = append(allFiles, pattern)
			} else {
				slog.Warn("No files found matching pattern", "pattern", pattern)
			}
		} else {
			allFiles = append(allFiles, matches...)
		}
	}

	if len(allFiles) == 0 {
		return fmt.Errorf("no files found to import")
	}

	slog.Info("🌶️  Importing OFX files...",
		"file_count", len(allFiles),
		"dry_run", dryRun)

	// Track all transactions across files
	var allTransactions []model.Transaction
	transactionMap := make(map[string]bool) // For deduplication
	fileResults := make(map[string]int)

	parser := ofx.NewParser()
	ctx := context.Background()

	// Process each file
	for _, filePath := range allFiles {
		slog.Info("Processing file", "file", filepath.Base(filePath))

		// Open file
		// Validate path
		cleanPath := filepath.Clean(filePath)
		if !filepath.IsAbs(cleanPath) {
			cleanPath, _ = filepath.Abs(cleanPath)
		}
		// #nosec G304 - filePath comes from command line args and is cleaned
		f, err := os.Open(cleanPath)
		if err != nil {
			slog.Error("Failed to open file",
				"file", filePath,
				"error", err)
			continue
		}

		// Parse OFX
		transactions, err := parser.ParseFile(ctx, f)
		if closeErr := f.Close(); closeErr != nil {
			slog.Error("failed to close file", "error", closeErr, "file", filePath)
		}

		if err != nil {
			slog.Error("Failed to parse OFX file",
				"file", filePath,
				"error", err)
			continue
		}

		if len(transactions) == 0 {
			slog.Warn("No transactions found in file",
				"file", filepath.Base(filePath))
			continue
		}

		// Add transactions with deduplication
		addedCount := 0
		for _, tx := range transactions {
			if !transactionMap[tx.Hash] {
				transactionMap[tx.Hash] = true
				allTransactions = append(allTransactions, tx)
				addedCount++
			}
		}

		fileResults[filepath.Base(filePath)] = addedCount
		slog.Info("Processed file",
			"file", filepath.Base(filePath),
			"transactions_found", len(transactions),
			"added", addedCount,
			"duplicates", len(transactions)-addedCount)
	}

	if len(allTransactions) == 0 {
		slog.Warn("No transactions found in any file")
		return nil
	}

	// Show summary
	if _, err := fmt.Fprintln(os.Stdout, "\n📁 File import summary:"); err != nil {
		slog.Error("failed to write output", "error", err)
	}
	for file, count := range fileResults {
		if _, err := fmt.Fprintf(os.Stdout, "  - %s: %d transactions\n", file, count); err != nil {
			slog.Error("failed to write output", "error", err)
		}
	}

	// Analyze combined data
	analyzeTransactions(allTransactions, verbose)

	if !dryRun {
		// Initialize storage
		ctx := context.Background()
		storageService, err := initStorage(ctx)
		if err != nil {
			return fmt.Errorf("failed to initialize storage: %w", err)
		}
		defer func() {
			if err := storageService.Close(); err != nil {
				slog.Error("failed to close storage", "error", err)
			}
		}()

		// Initialize pattern detector for direction detection
		detector, err := classification.NewPatternDetector(classification.DefaultPatterns())
		if err != nil {
			return fmt.Errorf("failed to initialize pattern detector: %w", err)
		}

		// Detect direction for each transaction
		for i := range allTransactions {
			match, err := detector.Classify(ctx, allTransactions[i])
			if err != nil {
				slog.Warn("Failed to detect direction for transaction",
					"transaction_id", allTransactions[i].ID,
					"error", err)
				continue
			}

			if match != nil && match.Confidence >= 0.75 {
				// High confidence match - set direction automatically
				switch match.Type {
				case classification.PatternTypeIncome:
					allTransactions[i].Direction = model.DirectionIncome
				case classification.PatternTypeExpense:
					allTransactions[i].Direction = model.DirectionExpense
				case classification.PatternTypeTransfer:
					allTransactions[i].Direction = model.DirectionTransfer
				}

				if verbose {
					slog.Info("Detected transaction direction",
						"transaction", allTransactions[i].MerchantName,
						"direction", allTransactions[i].Direction,
						"confidence", match.Confidence)
				}
			} else {
				// Low confidence or no match - default to expense for negative amounts, income for positive
				if allTransactions[i].Amount < 0 {
					allTransactions[i].Direction = model.DirectionExpense
				} else {
					allTransactions[i].Direction = model.DirectionIncome
				}
			}
		}

		// Save transactions
		if err := storageService.SaveTransactions(ctx, allTransactions); err != nil {
			return fmt.Errorf("failed to save transactions: %w", err)
		}

		slog.Info("💾 Successfully saved transactions to database",
			"total_count", len(allTransactions),
			"unique_count", len(transactionMap))
	} else {
		slog.Info("🔍 Dry run complete - no data saved")
	}

	return nil
}

func analyzeTransactions(transactions []model.Transaction, verbose bool) {
	// Find date range
	var oldestDate, newestDate time.Time
	merchantMap := make(map[string]int)
	accountMap := make(map[string]int)
	directionMap := make(map[model.TransactionDirection]int)
	totalAmount := 0.0

	for i, tx := range transactions {
		if i == 0 || tx.Date.Before(oldestDate) {
			oldestDate = tx.Date
		}
		if i == 0 || tx.Date.After(newestDate) {
			newestDate = tx.Date
		}

		merchantMap[tx.MerchantName]++
		accountMap[tx.AccountID]++
		directionMap[tx.Direction]++
		totalAmount += tx.Amount
	}

	slog.Info("✅ Successfully parsed OFX file",
		"transactions", len(transactions),
		"accounts", len(accountMap),
		"merchants", len(merchantMap))

	if _, err := fmt.Fprintf(os.Stdout, "\n📅 Transaction date range: %s to %s (%d days)\n",
		oldestDate.Format("2006-01-02"),
		newestDate.Format("2006-01-02"),
		int(newestDate.Sub(oldestDate).Hours()/24)); err != nil {
		slog.Error("failed to write output", "error", err)
	}

	if _, err := fmt.Fprintf(os.Stdout, "💰 Total amount: $%.2f\n", totalAmount); err != nil {
		slog.Error("failed to write output", "error", err)
	}

	// Show direction breakdown
	if _, err := fmt.Fprintln(os.Stdout, "\n🔄 Transaction directions:"); err != nil {
		slog.Error("failed to write output", "error", err)
	}
	for direction, count := range directionMap {
		icon := "❓"
		switch direction {
		case model.DirectionIncome:
			icon = "💵"
		case model.DirectionExpense:
			icon = "💸"
		case model.DirectionTransfer:
			icon = "🔁"
		}
		if _, err := fmt.Fprintf(os.Stdout, "  %s %s: %d transactions\n", icon, direction, count); err != nil {
			slog.Error("failed to write output", "error", err)
		}
	}

	// Show accounts
	if _, err := fmt.Fprintln(os.Stdout, "\n🏦 Accounts found:"); err != nil {
		slog.Error("failed to write output", "error", err)
	}
	for acct, count := range accountMap {
		if _, err := fmt.Fprintf(os.Stdout, "  - %s (%d transactions)\n", acct, count); err != nil {
			slog.Error("failed to write output", "error", err)
		}
	}

	// Show sample transactions
	if _, err := fmt.Fprintln(os.Stdout, "\n📝 Sample transactions (first 5):"); err != nil {
		slog.Error("failed to write output", "error", err)
	}
	if _, err := fmt.Fprintln(os.Stdout, "──────────────────────────────────────────────────────"); err != nil {
		slog.Error("failed to write output", "error", err)
	}
	for i, tx := range transactions {
		if i >= 5 {
			break
		}
		dirIcon := "❓"
		switch tx.Direction {
		case model.DirectionIncome:
			dirIcon = "💵"
		case model.DirectionExpense:
			dirIcon = "💸"
		case model.DirectionTransfer:
			dirIcon = "🔁"
		}
		if _, err := fmt.Fprintf(os.Stdout, "%s Date: %s | Amount: $%.2f | Merchant: %s\n",
			dirIcon,
			tx.Date.Format("2006-01-02"),
			tx.Amount,
			tx.MerchantName); err != nil {
			slog.Error("failed to write output", "error", err)
		}
		if verbose {
			if _, err := fmt.Fprintf(os.Stdout, "  Raw Name: %s\n", tx.Name); err != nil {
				slog.Error("failed to write output", "error", err)
			}
			if _, err := fmt.Fprintf(os.Stdout, "  Account: %s\n", tx.AccountID); err != nil {
				slog.Error("failed to write output", "error", err)
			}
			if _, err := fmt.Fprintf(os.Stdout, "  ID: %s\n", tx.ID); err != nil {
				slog.Error("failed to write output", "error", err)
			}
		}
		if _, err := fmt.Fprintln(os.Stdout, "──────────────────────────────────────────────────────"); err != nil {
			slog.Error("failed to write output", "error", err)
		}
	}

	// Show top merchants
	if _, err := fmt.Fprintln(os.Stdout, "\n🏪 Top merchants by transaction count:"); err != nil {
		slog.Error("failed to write output", "error", err)
	}
	type merchantCount struct {
		name  string
		count int
	}
	merchants := make([]merchantCount, 0, len(merchantMap))
	for name, count := range merchantMap {
		merchants = append(merchants, merchantCount{name, count})
	}
	// Simple sort
	for i := 0; i < len(merchants); i++ {
		for j := i + 1; j < len(merchants); j++ {
			if merchants[j].count > merchants[i].count {
				merchants[i], merchants[j] = merchants[j], merchants[i]
			}
		}
	}
	for i, m := range merchants {
		if i >= 10 {
			break
		}
		if _, err := fmt.Fprintf(os.Stdout, "  %2d. %s (%d transactions)\n", i+1, m.name, m.count); err != nil {
			slog.Error("failed to write output", "error", err)
		}
	}
}
