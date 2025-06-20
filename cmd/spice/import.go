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
	cmd.Flags().Bool("no-checkpoint", false, "Skip creating automatic checkpoint before import")

	// Bind to viper
	_ = viper.BindPFlag("import.start_date", cmd.Flags().Lookup("start-date"))
	_ = viper.BindPFlag("import.end_date", cmd.Flags().Lookup("end-date"))
	_ = viper.BindPFlag("import.days", cmd.Flags().Lookup("days"))
	_ = viper.BindPFlag("import.accounts", cmd.Flags().Lookup("accounts"))
	_ = viper.BindPFlag("import.list_accounts", cmd.Flags().Lookup("list-accounts"))
	_ = viper.BindPFlag("import.dry_run", cmd.Flags().Lookup("dry-run"))
	_ = viper.BindPFlag("import.no_checkpoint", cmd.Flags().Lookup("no-checkpoint"))

	return cmd
}

func runImport(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	// Get base Plaid configuration
	baseConfig := plaid.Config{
		ClientID:    viper.GetString("plaid.client_id"),
		Secret:      viper.GetString("plaid.secret"),
		Environment: viper.GetString("plaid.environment"),
	}

	// Set defaults if not provided
	if baseConfig.Environment == "" {
		baseConfig.Environment = "sandbox"
	}

	// Get all connected banks
	connections := getAllPlaidConnections()
	if len(connections) == 0 {
		return fmt.Errorf("no banks connected. Run 'spice auth plaid' to connect a bank account")
	}

	// Handle list-accounts flag (list from all banks)
	if viper.GetBool("import.list_accounts") {
		return listAccountsFromAllBanks(ctx, baseConfig, connections)
	}

	// Parse date range
	startDate, endDate, err := parseDateRange()
	if err != nil {
		return err
	}

	slog.Info(cli.FormatTitle("Importing transactions from all connected banks"))
	slog.Info("Date range", "start", startDate.Format("2006-01-02"), "end", endDate.Format("2006-01-02"))
	slog.Info(fmt.Sprintf("Connected banks: %d", len(connections)))

	// Fetch transactions from all banks
	var allTransactions []model.Transaction
	totalFetched := 0

	for _, conn := range connections {
		slog.Info(fmt.Sprintf("🏦 Fetching from %s...", conn.InstitutionName))

		// Create client for this bank
		config := baseConfig
		config.AccessToken = conn.AccessToken
		plaidClient, clientErr := plaid.NewClient(config)
		if clientErr != nil {
			slog.Error("Failed to create Plaid client", "bank", conn.InstitutionName, "error", clientErr)
			continue
		}

		// Fetch transactions
		transactions, txnErr := plaidClient.GetTransactions(ctx, startDate, endDate)
		if txnErr != nil {
			slog.Error("Failed to fetch transactions", "bank", conn.InstitutionName, "error", txnErr)
			continue
		}

		allTransactions = append(allTransactions, transactions...)
		totalFetched += len(transactions)
		slog.Info(cli.FormatSuccess(fmt.Sprintf("  ✓ Fetched %d transactions from %s", len(transactions), conn.InstitutionName)))
	}

	slog.Info(cli.FormatSuccess(fmt.Sprintf("✓ Total: %d transactions from %d banks", totalFetched, len(connections))))
	transactions := allTransactions

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
	defer store.Close()

	// Create auto-checkpoint unless disabled
	if !viper.GetBool("import.no_checkpoint") && !viper.GetBool("checkpoint.auto_checkpoint_disabled") {
		slog.Info("🗄️  Creating automatic checkpoint before import...")

		// Create checkpoint manager
		manager, err := store.NewCheckpointManager()
		if err != nil {
			slog.Warn("Failed to create checkpoint manager", "error", err)
		} else {
			if err := manager.AutoCheckpoint(ctx, "import"); err != nil {
				slog.Warn("Failed to create auto-checkpoint", "error", err)
			} else {
				slog.Info(cli.FormatSuccess("✓ Checkpoint created"))
			}
		}
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

// PlaidConnection represents a connected bank.
type PlaidConnection struct {
	AccessToken     string
	InstitutionName string
	ItemID          string
}

// getAllPlaidConnections retrieves all connected banks from config.
func getAllPlaidConnections() []PlaidConnection {
	var connections []PlaidConnection

	// Check for primary/legacy access token
	if token := viper.GetString("plaid.access_token"); token != "" {
		connections = append(connections, PlaidConnection{
			AccessToken:     token,
			InstitutionName: "Primary Bank",
			ItemID:          "primary",
		})
	}

	// Get all connections from config
	connectionsMap := viper.GetStringMap("plaid.connections")
	for itemID, connData := range connectionsMap {
		if connMap, ok := connData.(map[string]any); ok {
			conn := PlaidConnection{
				ItemID: itemID,
			}

			if token, ok := connMap["access_token"].(string); ok {
				conn.AccessToken = token
			}
			if name, ok := connMap["institution_name"].(string); ok {
				conn.InstitutionName = name
			}

			if conn.AccessToken != "" {
				connections = append(connections, conn)
			}
		}
	}

	return connections
}

// listAccountsFromAllBanks lists accounts from all connected banks.
func listAccountsFromAllBanks(ctx context.Context, baseConfig plaid.Config, connections []PlaidConnection) error {
	slog.Info(cli.FormatTitle("Listing accounts from all connected banks"))

	for _, conn := range connections {
		slog.Info(fmt.Sprintf("\n🏦 %s:", conn.InstitutionName))

		config := baseConfig
		config.AccessToken = conn.AccessToken
		plaidClient, clientErr := plaid.NewClient(config)
		if clientErr != nil {
			slog.Error("Failed to create Plaid client", "bank", conn.InstitutionName, "error", clientErr)
			continue
		}

		accounts, err := plaidClient.GetAccounts(ctx)
		if err != nil {
			slog.Error("Failed to fetch accounts", "bank", conn.InstitutionName, "error", err)
			continue
		}

		for _, account := range accounts {
			slog.Info(fmt.Sprintf("  - %s", account))
		}
	}

	return nil
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
