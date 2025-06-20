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
		Use:   "import-ofx [file]",
		Short: "Import transactions from OFX/QFX file",
		Long:  `Import financial transactions from OFX or QFX (Quicken) files exported from your bank`,
		Args:  cobra.ExactArgs(1),
		RunE:  runImportOFX,
	}

	importOFXCmd.Flags().BoolP("dry-run", "d", false, "Preview import without saving")
	importOFXCmd.Flags().BoolP("verbose", "v", false, "Show detailed transaction data")

	rootCmd.AddCommand(importOFXCmd)
}

func runImportOFX(cmd *cobra.Command, args []string) error {
	filePath := args[0]
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	verbose, _ := cmd.Flags().GetBool("verbose")

	// Check file exists
	if _, err := os.Stat(filePath); err != nil {
		return fmt.Errorf("file not found: %s", filePath)
	}

	slog.Info("🌶️  Importing OFX file...",
		"file", filepath.Base(filePath),
		"dry_run", dryRun)

	// Open file
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	// Parse OFX
	parser := ofx.NewParser()
	transactions, err := parser.ParseFile(context.Background(), f)
	if err != nil {
		return fmt.Errorf("failed to parse OFX file: %w", err)
	}

	if len(transactions) == 0 {
		slog.Warn("No transactions found in file")
		return nil
	}

	// Analyze the data
	analyzeTransactions(transactions, verbose)

	if !dryRun {
		// TODO: Save to database
		slog.Info("💾 Would save transactions to database",
			"count", len(transactions))
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