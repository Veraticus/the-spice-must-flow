// Package main contains the spice CLI commands.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/cli"
	"github.com/Veraticus/the-spice-must-flow/internal/config"
	"github.com/Veraticus/the-spice-must-flow/internal/engine"
	"github.com/Veraticus/the-spice-must-flow/internal/service"
	"github.com/Veraticus/the-spice-must-flow/internal/storage"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func classifyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "classify",
		Short: "Categorize transactions",
		Long: `Categorize financial transactions with AI assistance and smart batching.
		
This command processes ALL unclassified transactions by default, grouping them 
by merchant to minimize manual effort. Use --year or --month flags to limit
the date range if needed.

Examples:
  spice classify              # Classify ALL unclassified transactions
  spice classify --year 2024  # Classify only 2024 transactions
  spice classify --month 2024-03  # Classify only March 2024 transactions
  spice classify --resume     # Resume from previous session`,
		RunE: runClassify,
	}

	// Flags
	cmd.Flags().IntP("year", "y", 0, "Year to classify transactions for (0 = all years)")
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

	// Set up interrupt handling
	interruptHandler := cli.NewInterruptHandler(nil)
	ctx = interruptHandler.HandleInterrupts(ctx, true)

	slog.Info("Starting transaction categorization")

	// Initialize storage
	dbPath := viper.GetString("database.path")
	if dbPath == "" {
		dbPath = "$HOME/.local/share/spice/spice.db"
	}
	dbPath = config.ExpandPath(dbPath)

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

	slog.Info("Connected to database successfully")

	// Initialize components
	var classifier engine.Classifier
	var prompter engine.Prompter

	if dryRun {
		slog.Info("Running in dry-run mode - using mock components")
		classifier = engine.NewMockClassifier()
		prompter = engine.NewMockPrompter(true) // Auto-accept in dry-run
	} else {
		// Use real prompter for interactive classification
		cliPrompter := cli.NewCLIPrompter(nil, nil)
		cliPrompter.Start(ctx)
		defer cliPrompter.Close()
		prompter = cliPrompter

		// Initialize real LLM classifier
		var llmErr error
		llmClient, llmErr := createLLMClient()
		if llmErr != nil {
			return fmt.Errorf("failed to create LLM client: %w", llmErr)
		}
		classifier = llmClient
	}

	// Create classification engine
	classificationEngine := engine.New(db, classifier, prompter)

	// Determine date range
	var fromDate *time.Time
	if !resume {
		if month != "" {
			// Parse month
			parsedMonth, parseErr := time.Parse("2006-01", month)
			if parseErr != nil {
				return fmt.Errorf("invalid month format (use YYYY-MM): %w", parseErr)
			}
			startDate := parsedMonth
			fromDate = &startDate
		} else if year > 0 {
			// Use beginning of specified year
			startDate := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
			fromDate = &startDate
		}
		// If year == 0 (default), fromDate remains nil, which means classify ALL transactions
	}

	// Get transaction count for progress tracking
	txns, err := db.GetTransactionsToClassify(ctx, fromDate)
	if err != nil {
		return fmt.Errorf("failed to count transactions: %w", err)
	}

	// Set total for progress tracking
	if cliPrompter, ok := prompter.(*cli.Prompter); ok {
		cliPrompter.SetTotalTransactions(len(txns))
	}

	// Run classification
	if err := classificationEngine.ClassifyTransactions(ctx, fromDate); err != nil {
		if err == context.Canceled {
			slog.Warn("Classification interrupted")
			slog.Info("Progress saved! Resume where you left off with: spice classify --resume")
			return nil
		}
		return fmt.Errorf("classification failed: %w", err)
	}

	// Show completion stats
	if cliPrompter, ok := prompter.(*cli.Prompter); ok {
		cliPrompter.ShowCompletion()
	} else {
		stats := prompter.GetCompletionStats()
		showCompletionStats(stats)
	}

	return nil
}

func showCompletionStats(stats service.CompletionStats) {
	slog.Info("Excellent! All transactions have been categorized", "total_transactions", stats.TotalTransactions)

	if stats.TotalTransactions > 0 {
		autoPercent := float64(stats.AutoClassified) / float64(stats.TotalTransactions) * 100
		userPercent := float64(stats.UserClassified) / float64(stats.TotalTransactions) * 100

		slog.Info("Classification statistics",
			"auto_classified", stats.AutoClassified,
			"auto_percent", autoPercent,
			"user_classified", stats.UserClassified,
			"user_percent", userPercent)
	}

	slog.Info("Classification details",
		"new_vendor_rules", stats.NewVendorRules,
		"duration", stats.Duration.Round(time.Second))

	if stats.TotalTransactions > 0 {
		// Estimate time saved (30 seconds per manual transaction)
		timeSaved := time.Duration(stats.AutoClassified) * 30 * time.Second
		slog.Info("Time saved", "estimated_time", timeSaved.Round(time.Minute))
	}

	slog.Info("Ready for export: spice flow --export")
}
