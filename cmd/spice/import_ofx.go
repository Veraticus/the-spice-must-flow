package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/joshsymonds/the-spice-must-flow/internal/model"
	"github.com/joshsymonds/the-spice-must-flow/internal/ofx"
	"github.com/spf13/cobra"
)

func init() {
	importOFXCmd := &cobra.Command{
		Use:   "import-ofx [files...]",
		Short: "Import transactions from OFX/QFX files",
		Long:  `Import financial transactions from OFX or QFX (Quicken) files exported from your bank.

Examples:
  # Import single file
  spice import-ofx ~/Downloads/chase_jan_2024.qfx
  
  # Import multiple files
  spice import-ofx ~/Downloads/chase_*.qfx
  
  # Import all QFX files in a directory
  spice import-ofx ~/Downloads/*.qfx
  
  # Import from multiple directories
  spice import-ofx ~/Downloads/Chase/*.qfx ~/Downloads/Ally/*.qfx`,
		Args:  cobra.MinimumNArgs(1),
		RunE:  runImportOFX,
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
		f, err := os.Open(filePath)
		if err != nil {
			slog.Error("Failed to open file",
				"file", filePath,
				"error", err)
			continue
		}

		// Parse OFX
		transactions, err := parser.ParseFile(ctx, f)
		f.Close()
		
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
	fmt.Println("\n📁 File import summary:")
	for file, count := range fileResults {
		fmt.Printf("  - %s: %d transactions\n", file, count)
	}

	// Analyze combined data
	analyzeTransactions(allTransactions, verbose)

	if !dryRun {
		// TODO: Save to database
		slog.Info("💾 Would save transactions to database",
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
		totalAmount += tx.Amount
	}

	slog.Info("✅ Successfully parsed OFX file",
		"transactions", len(transactions),
		"accounts", len(accountMap),
		"merchants", len(merchantMap))

	fmt.Printf("\n📅 Transaction date range: %s to %s (%d days)\n",
		oldestDate.Format("2006-01-02"),
		newestDate.Format("2006-01-02"),
		int(newestDate.Sub(oldestDate).Hours()/24))

	fmt.Printf("💰 Total amount: $%.2f\n", totalAmount)

	// Show accounts
	fmt.Println("\n🏦 Accounts found:")
	for acct, count := range accountMap {
		fmt.Printf("  - %s (%d transactions)\n", acct, count)
	}

	// Show sample transactions
	fmt.Println("\n📝 Sample transactions (first 5):")
	fmt.Println("──────────────────────────────────────────────────────")
	for i, tx := range transactions {
		if i >= 5 {
			break
		}
		fmt.Printf("Date: %s | Amount: $%.2f | Merchant: %s\n",
			tx.Date.Format("2006-01-02"),
			tx.Amount,
			tx.MerchantName)
		if verbose {
			fmt.Printf("  Raw Name: %s\n", tx.Name)
			fmt.Printf("  Account: %s\n", tx.AccountID)
			fmt.Printf("  ID: %s\n", tx.ID)
		}
		fmt.Println("──────────────────────────────────────────────────────")
	}

	// Show top merchants
	fmt.Println("\n🏪 Top merchants by transaction count:")
	type merchantCount struct {
		name  string
		count int
	}
	var merchants []merchantCount
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
		fmt.Printf("  %2d. %s (%d transactions)\n", i+1, m.name, m.count)
	}
}