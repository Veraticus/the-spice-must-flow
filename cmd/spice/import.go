package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/joshsymonds/the-spice-must-flow/internal/cli"
	"github.com/joshsymonds/the-spice-must-flow/internal/model"
	"github.com/joshsymonds/the-spice-must-flow/internal/plaid"
	"github.com/joshsymonds/the-spice-must-flow/internal/storage"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func importCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import transactions from Plaid",
		Long: `Import financial transactions from your connected Plaid accounts.
		
This command fetches transactions from Plaid and stores them in the local database
for later categorization. Transactions are deduplicated automatically.`,
		RunE: runImport,
	}

	// Date range flags
	cmd.Flags().StringP("start-date", "s", "", "Start date for transaction import (format: 2006-01-02)")
	cmd.Flags().StringP("end-date", "e", "", "End date for transaction import (format: 2006-01-02)")
	cmd.Flags().IntP("days", "d", 30, "Number of days to import (used if start/end dates not specified)")

	// Account filtering
	cmd.Flags().StringSlice("accounts", []string{}, "Filter by specific account IDs (comma-separated)")
	cmd.Flags().Bool("list-accounts", false, "List available accounts without importing")

	// Other options
	cmd.Flags().Bool("dry-run", false, "Show what would be imported without saving")

	// Bind to viper
	_ = viper.BindPFlag("import.start_date", cmd.Flags().Lookup("start-date"))
	_ = viper.BindPFlag("import.end_date", cmd.Flags().Lookup("end-date"))
	_ = viper.BindPFlag("import.days", cmd.Flags().Lookup("days"))
	_ = viper.BindPFlag("import.accounts", cmd.Flags().Lookup("accounts"))
	_ = viper.BindPFlag("import.list_accounts", cmd.Flags().Lookup("list-accounts"))
	_ = viper.BindPFlag("import.dry_run", cmd.Flags().Lookup("dry-run"))

	return cmd
}

func runImport(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	// Get Plaid configuration
	plaidConfig := &plaid.Config{
		ClientID:    viper.GetString("plaid.client_id"),
		Secret:      viper.GetString("plaid.secret"),
		Environment: viper.GetString("plaid.environment"),
		AccessToken: viper.GetString("plaid.access_token"),
	}

	// Set defaults if not provided
	if plaidConfig.Environment == "" {
		plaidConfig.Environment = "sandbox"
	}

	// Create Plaid client
	plaidClient, err := plaid.NewClient(plaidConfig)
	if err != nil {
		return fmt.Errorf("failed to create Plaid client: %w", err)
	}

	// Handle list-accounts flag
	if viper.GetBool("import.list_accounts") {
		return listAccounts(ctx, plaidClient)
	}

	// Parse date range
	startDate, endDate, err := parseDateRange()
	if err != nil {
		return err
	}

	slog.Info(cli.FormatTitle("Importing transactions from Plaid"))
	slog.Info("Date range", "start", startDate.Format("2006-01-02"), "end", endDate.Format("2006-01-02"))

	// Fetch transactions
	slog.Info("🔄 Fetching transactions...")
	transactions, err := plaidClient.GetTransactions(ctx, startDate, endDate)
	if err != nil {
		return fmt.Errorf("failed to fetch transactions: %w", err)
	}

	slog.Info(cli.FormatSuccess(fmt.Sprintf("✓ Fetched %d transactions", len(transactions))))

	// Filter by accounts if specified
	accountFilter := viper.GetStringSlice("import.accounts")
	if len(accountFilter) > 0 {
		filtered := filterTransactionsByAccount(transactions, accountFilter)
		slog.Info(fmt.Sprintf("Filtered to %d transactions from specified accounts", len(filtered)))
		transactions = filtered
	}

	// Check for dry run
	if viper.GetBool("import.dry_run") {
		slog.Info(cli.FormatWarning("Dry run mode - not saving to database"))
		displayTransactionSummary(transactions)
		return nil
	}

	// Initialize storage
	dbPath := filepath.Join(os.Getenv("HOME"), ".config", "spice", "spice.db")
	store, err := storage.NewSQLiteStorage(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Save transactions
	slog.Info("💾 Saving transactions to database...")
	if err := store.SaveTransactions(ctx, transactions); err != nil {
		return fmt.Errorf("failed to save transactions: %w", err)
	}

	slog.Info(cli.FormatSuccess("✓ Import complete!"))
	displayTransactionSummary(transactions)

	return nil
}

func listAccounts(ctx context.Context, client *plaid.Client) error {
	slog.Info(cli.FormatTitle("Fetching accounts from Plaid"))

	accounts, err := client.GetAccounts(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch accounts: %w", err)
	}

	if len(accounts) == 0 {
		slog.Info(cli.FormatWarning("No accounts found"))
		return nil
	}

	content := fmt.Sprintf("Found %d accounts:\n\n", len(accounts))
	for i, accountID := range accounts {
		content += fmt.Sprintf("%d. %s\n", i+1, accountID)
	}

	slog.Info(cli.RenderBox("Available Accounts", content))
	return nil
}

func parseDateRange() (startDate, endDate time.Time, err error) {
	// Check if explicit dates are provided
	startStr := viper.GetString("import.start_date")
	endStr := viper.GetString("import.end_date")

	if startStr != "" && endStr != "" {
		startDate, err = time.Parse("2006-01-02", startStr)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid start date format: %w", err)
		}

		endDate, err = time.Parse("2006-01-02", endStr)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid end date format: %w", err)
		}

		return startDate, endDate, nil
	}

	// Use days flag
	days := viper.GetInt("import.days")
	if days <= 0 {
		days = 30
	}

	endDate = time.Now()
	startDate = endDate.AddDate(0, 0, -days)

	return startDate, endDate, nil
}

func filterTransactionsByAccount(transactions []model.Transaction, accountIDs []string) []model.Transaction {
	accountSet := make(map[string]bool)
	for _, id := range accountIDs {
		accountSet[id] = true
	}

	filtered := make([]model.Transaction, 0)
	for _, tx := range transactions {
		if accountSet[tx.AccountID] {
			filtered = append(filtered, tx)
		}
	}

	return filtered
}

func displayTransactionSummary(transactions []model.Transaction) {
	if len(transactions) == 0 {
		return
	}

	// Calculate summary statistics
	totalAmount := 0.0
	merchants := make(map[string]int)
	accounts := make(map[string]int)

	for _, tx := range transactions {
		totalAmount += tx.Amount
		merchants[tx.MerchantName]++
		accounts[tx.AccountID]++
	}

	content := fmt.Sprintf(`Transactions: %d
Total amount: $%.2f
Unique merchants: %d
Accounts: %d

Top merchants:
`, len(transactions), totalAmount, len(merchants), len(accounts))

	// Show top 5 merchants
	topMerchants := getTopMerchants(merchants, 5)
	for i, m := range topMerchants {
		content += fmt.Sprintf("%d. %s (%d transactions)\n", i+1, m.name, m.count)
	}

	slog.Info(cli.RenderBox("Import Summary", content))
}

type merchantCount struct {
	name  string
	count int
}

func getTopMerchants(merchants map[string]int, limit int) []merchantCount {
	// Convert map to slice for sorting
	counts := make([]merchantCount, 0, len(merchants))
	for name, count := range merchants {
		counts = append(counts, merchantCount{name: name, count: count})
	}

	// Simple bubble sort for top N (efficient for small N)
	for i := 0; i < len(counts) && i < limit; i++ {
		for j := i + 1; j < len(counts); j++ {
			if counts[j].count > counts[i].count {
				counts[i], counts[j] = counts[j], counts[i]
			}
		}
	}

	if len(counts) > limit {
		counts = counts[:limit]
	}

	return counts
}
