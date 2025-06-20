package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/joshsymonds/the-spice-must-flow/internal/simplefin"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	testSimpleFINCmd := &cobra.Command{
		Use:   "test-simplefin",
		Short: "Test SimpleFIN connection and data retrieval",
		Long:  `Test SimpleFIN connection and inspect the data we receive`,
		RunE:  runTestSimpleFIN,
	}

	testSimpleFINCmd.Flags().IntP("days", "d", 30, "Number of days to fetch")
	testSimpleFINCmd.Flags().BoolP("verbose", "v", false, "Show raw transaction data")

	rootCmd.AddCommand(testSimpleFINCmd)
}

func runTestSimpleFIN(cmd *cobra.Command, args []string) error {
	// Get SimpleFIN token from config or environment
	token := viper.GetString("simplefin.token")
	if token == "" {
		// Fallback to environment variable
		token = os.Getenv("SIMPLEFIN_TOKEN")
	}
	if token == "" {
		return fmt.Errorf("SimpleFIN token not found in config or SIMPLEFIN_TOKEN environment variable")
	}

	days, _ := cmd.Flags().GetInt("days")
	verbose, _ := cmd.Flags().GetBool("verbose")

	slog.Info("🌶️  Testing SimpleFIN connection...")
	slog.Debug("Token format", "has_http", strings.HasPrefix(token, "http"), "length", len(token))

	// Create client
	client, err := simplefin.NewClient(token)
	if err != nil {
		return fmt.Errorf("failed to create SimpleFIN client: %w", err)
	}

	// Test account fetching
	ctx := context.Background()
	accounts, err := client.GetAccounts(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch accounts: %w", err)
	}

	slog.Info("✅ Connected to SimpleFIN",
		"accounts_found", len(accounts))

	for i, accountID := range accounts {
		slog.Info(fmt.Sprintf("  Account %d: %s", i+1, accountID))
	}

	// Fetch transactions
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -days)

	slog.Info("📊 Fetching transactions...",
		"start_date", startDate.Format("2006-01-02"),
		"end_date", endDate.Format("2006-01-02"))

	transactions, err := client.GetTransactions(ctx, startDate, endDate)
	if err != nil {
		return fmt.Errorf("failed to fetch transactions: %w", err)
	}

	slog.Info("✅ Retrieved transactions",
		"count", len(transactions))

	// Analyze the data
	if len(transactions) > 0 {
		// Find date range
		var oldestDate, newestDate time.Time
		merchantMap := make(map[string]int)
		accountTxCount := make(map[string]int)

		for i, tx := range transactions {
			if i == 0 || tx.Date.Before(oldestDate) {
				oldestDate = tx.Date
			}
			if i == 0 || tx.Date.After(newestDate) {
				newestDate = tx.Date
			}

			merchantMap[tx.MerchantName]++
			accountTxCount[tx.AccountID]++
		}

		slog.Info("📅 Transaction date range",
			"oldest", oldestDate.Format("2006-01-02"),
			"newest", newestDate.Format("2006-01-02"),
			"days_covered", int(newestDate.Sub(oldestDate).Hours()/24))

		slog.Info("📈 Transaction distribution",
			"unique_merchants", len(merchantMap),
			"accounts", len(accountTxCount))

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
				fmt.Printf("  Hash: %s\n", tx.Hash)
			}
			fmt.Println("──────────────────────────────────────────────────────")
		}

		// Show merchant analysis
		fmt.Println("\n🏪 Top merchants by transaction count:")
		type merchantCount struct {
			name  string
			count int
		}
		var merchants []merchantCount
		for name, count := range merchantMap {
			merchants = append(merchants, merchantCount{name, count})
		}
		// Simple sort for top 10
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

		// If verbose, show raw JSON for one transaction
		if verbose && len(transactions) > 0 {
			fmt.Println("\n🔍 Raw transaction data (first transaction as JSON):")
			data, _ := json.MarshalIndent(transactions[0], "", "  ")
			fmt.Println(string(data))
		}
	}

	// Check how far back we can go
	fmt.Println("\n💡 To check maximum history available:")
	fmt.Println("   Run: spice test-simplefin -d 365")

	return nil
}
