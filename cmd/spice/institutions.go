package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/joshsymonds/the-spice-must-flow/internal/plaid"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func institutionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "institutions",
		Short: "Search and list supported banks",
		Long: `Search for banks supported by Plaid and check their capabilities.

This helps you find banks that don't require OAuth, which can be connected
immediately without configuring redirect URIs in the Plaid dashboard.`,
	}

	cmd.AddCommand(institutionsSearchCmd())

	return cmd
}

func institutionsSearchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search for banks by name",
		Long: `Search for banks by name and see their authentication requirements.

Examples:
  spice institutions search chase
  spice institutions search "bank of america"
  spice institutions search wells

Banks marked with "OAuth: No" can be connected immediately without
configuring redirect URIs in the Plaid dashboard.`,
		Args: cobra.MinimumNArgs(1),
		RunE: runInstitutionsSearch,
	}

	cmd.Flags().String("env", "", "Plaid environment (sandbox/production)")
	cmd.Flags().Int("limit", 10, "Maximum number of results to show")

	return cmd
}

func runInstitutionsSearch(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Get search query
	query := strings.Join(args, " ")

	// Get Plaid configuration
	clientID := viper.GetString("plaid.client_id")
	secret := viper.GetString("plaid.secret")
	environment := viper.GetString("plaid.environment")

	// Override with flag if provided
	if flagEnv, _ := cmd.Flags().GetString("env"); flagEnv != "" {
		environment = flagEnv
	}

	// Check environment variables as fallback
	if clientID == "" {
		clientID = os.Getenv("PLAID_CLIENT_ID")
	}
	if secret == "" {
		secret = os.Getenv("PLAID_SECRET")
	}
	if environment == "" {
		environment = os.Getenv("PLAID_ENV")
		if environment == "" {
			environment = "production"
		}
	}

	if clientID == "" || secret == "" {
		return fmt.Errorf("plaid credentials not found. Please set plaid.client_id and plaid.secret in config")
	}

	limit, _ := cmd.Flags().GetInt("limit")

	slog.Info("Searching for institutions", "query", query, "environment", environment)

	// Create Plaid client
	plaidClient, err := plaid.NewClient(plaid.Config{
		ClientID:    clientID,
		Secret:      secret,
		Environment: environment,
	})
	if err != nil {
		return fmt.Errorf("failed to create Plaid client: %w", err)
	}

	// Search for institutions
	institutions, err := plaidClient.SearchInstitutions(ctx, query, limit)
	if err != nil {
		return fmt.Errorf("failed to search institutions: %w", err)
	}

	if len(institutions) == 0 {
		slog.Info("No institutions found matching your search")
		return nil
	}

	// Display results in a table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "Bank Name\tOAuth Required\tSupports Transactions\tNotes"); err != nil {
		slog.Error("failed to write table header", "error", err)
	}
	if _, err := fmt.Fprintln(w, "─────────\t─────────────\t───────────────────\t─────"); err != nil {
		slog.Error("failed to write table separator", "error", err)
	}

	for _, inst := range institutions {
		oauth := "No"
		if inst.OAuth {
			oauth = "Yes ⚠️"
		}

		transactions := "No"
		if inst.SupportsTransactions {
			transactions = "Yes ✓"
		}

		notes := ""
		if !inst.OAuth && inst.SupportsTransactions {
			notes = "✅ Ready to use!"
		} else if inst.OAuth {
			notes = "Requires redirect URI"
		}

		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			inst.Name,
			oauth,
			transactions,
			notes,
		); err != nil {
			slog.Error("failed to write institution row", "error", err)
		}
	}

	if err := w.Flush(); err != nil {
		slog.Error("failed to flush table writer", "error", err)
	}

	// Show summary
	if _, err := fmt.Fprintln(os.Stdout); err != nil {
		slog.Error("failed to write output", "error", err)
	}
	nonOAuthCount := 0
	for _, inst := range institutions {
		if !inst.OAuth && inst.SupportsTransactions {
			nonOAuthCount++
		}
	}

	if nonOAuthCount > 0 {
		slog.Info(fmt.Sprintf("Found %d banks that work without OAuth configuration", nonOAuthCount))
		slog.Info("You can connect to these banks immediately!")
	} else {
		slog.Info("All matching banks require OAuth configuration")
		slog.Info("You'll need to add https://localhost:8080/ to your Plaid dashboard")
	}

	return nil
}
