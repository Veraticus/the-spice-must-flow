// Package main contains the spice CLI commands.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/joshsymonds/the-spice-must-flow/internal/cli"
	"github.com/joshsymonds/the-spice-must-flow/internal/engine"
	"github.com/joshsymonds/the-spice-must-flow/internal/service"
	"github.com/joshsymonds/the-spice-must-flow/internal/storage"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func classifyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "classify",
		Short: "Categorize transactions",
		Long: `Categorize financial transactions with AI assistance and smart batching.
		
This command fetches transactions from Plaid, groups them by merchant,
and guides you through categorization with minimal effort.`,
		RunE: runClassify,
	}

	// Flags
	cmd.Flags().IntP("year", "y", time.Now().Year(), "Year to classify transactions for")
	cmd.Flags().StringP("month", "m", "", "Specific month to classify (format: 2024-01)")
	cmd.Flags().BoolP("resume", "r", false, "Resume from previous session")
	cmd.Flags().Bool("dry-run", false, "Preview without saving changes")

	// Bind to viper (errors are rare and can be ignored in practice)
	_ = viper.BindPFlag("classification.year", cmd.Flags().Lookup("year"))
	_ = viper.BindPFlag("classification.month", cmd.Flags().Lookup("month"))
	_ = viper.BindPFlag("classification.resume", cmd.Flags().Lookup("resume"))
	_ = viper.BindPFlag("classification.dry_run", cmd.Flags().Lookup("dry-run"))

	return cmd
}

func runClassify(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	year := viper.GetInt("classification.year")
	month := viper.GetString("classification.month")
	resume := viper.GetBool("classification.resume")
	dryRun := viper.GetBool("classification.dry_run")

	fmt.Println(cli.StyleTitle("🌶️  Starting transaction categorization..."))

	// Initialize storage
	dbPath := viper.GetString("database.path")
	if dbPath == "" {
		dbPath = "~/.local/share/spice/spice.db"
	}
	dbPath = expandPath(dbPath)

	db, err := storage.NewSQLiteStorage(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Run migrations
	if err := db.Migrate(ctx); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	fmt.Println(cli.StyleSuccess("✓ Connected to database"))

	// For now, use mock implementations
	// TODO: Replace with real implementations in later phases
	var llm service.LLMClassifier
	var prompter service.UserPrompter

	if dryRun {
		fmt.Println(cli.StyleInfo("ℹ️  Running in dry-run mode - using mock components"))
		llm = engine.NewMockLLMClassifier()
		prompter = engine.NewMockUserPrompter(true) // Auto-accept in dry-run
	} else {
		// TODO: Initialize real LLM and prompter
		fmt.Println(cli.StyleWarning("⚠️  Using mock components (real implementations coming soon)"))
		llm = engine.NewMockLLMClassifier()
		prompter = engine.NewMockUserPrompter(false)
	}

	// Create classification engine
	classificationEngine := engine.New(db, llm, prompter)

	// Determine date range
	var fromDate *time.Time
	if !resume {
		if month != "" {
			// Parse month
			parsedMonth, err := time.Parse("2006-01", month)
			if err != nil {
				return fmt.Errorf("invalid month format (use YYYY-MM): %w", err)
			}
			startDate := parsedMonth
			fromDate = &startDate
		} else {
			// Use beginning of year
			startDate := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
			fromDate = &startDate
		}
	}

	// Run classification
	if err := classificationEngine.ClassifyTransactions(ctx, fromDate); err != nil {
		if err == context.Canceled {
			fmt.Println(cli.StyleWarning("\n⚠️  Classification interrupted"))
			fmt.Println(cli.StyleInfo("ℹ️  Progress saved. Resume with: spice classify --resume"))
			return nil
		}
		return fmt.Errorf("classification failed: %w", err)
	}

	// Show completion stats
	stats := prompter.GetCompletionStats()
	showCompletionStats(stats)

	return nil
}

func showCompletionStats(stats service.CompletionStats) {
	fmt.Println(cli.StyleSuccess("\n✅ Classification complete!"))

	fmt.Printf("\n%s\n", cli.StyleTitle("Summary"))
	fmt.Printf("Total transactions:      %d\n", stats.TotalTransactions)

	if stats.TotalTransactions > 0 {
		autoPercent := float64(stats.AutoClassified) / float64(stats.TotalTransactions) * 100
		userPercent := float64(stats.UserClassified) / float64(stats.TotalTransactions) * 100

		fmt.Printf("Auto-classified:         %d (%.1f%%)\n", stats.AutoClassified, autoPercent)
		fmt.Printf("User-classified:         %d (%.1f%%)\n", stats.UserClassified, userPercent)
	}

	fmt.Printf("\nNew vendor rules:        %d\n", stats.NewVendorRules)
	fmt.Printf("Time taken:              %s\n", stats.Duration.Round(time.Second))

	if stats.TotalTransactions > 0 {
		// Estimate time saved (30 seconds per manual transaction)
		timeSaved := time.Duration(stats.AutoClassified) * 30 * time.Second
		fmt.Printf("Time saved:              ~%s (estimated)\n", timeSaved.Round(time.Minute))
	}

	fmt.Printf("\n%s\n", cli.StyleInfo("Ready for export: spice flow --export"))
}

func expandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err == nil {
			path = home + path[1:]
		}
	}
	return path
}
