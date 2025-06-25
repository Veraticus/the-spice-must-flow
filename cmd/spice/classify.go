// Package main contains the spice CLI commands.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/cli"
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
		
This command fetches transactions from Plaid, groups them by merchant,
and guides you through categorization with minimal effort.

By default, this will classify ALL unclassified transactions. Use --year or --month
to limit the scope to a specific time period.

Examples:
  # Classify all unclassified transactions
  spice classify
  
  # Use batch mode for faster processing (5-10x speedup)
  spice classify --batch
  
  # Batch mode - only auto-accept high confidence items
  spice classify --batch --auto-only
  
  # Batch mode with custom auto-accept threshold
  spice classify --batch --auto-accept-threshold=0.90
  
  # Maximum performance batch mode
  spice classify --batch --auto-only --parallel-workers=10
  
  # Classify only 2024 transactions
  spice classify --year 2024
  
  # Resume from a previous session (sequential mode only)
  spice classify --resume`,
		RunE: runClassify,
	}

	// Flags
	cmd.Flags().IntP("year", "y", 0, "Year to classify transactions for (default: all transactions)")
	cmd.Flags().StringP("month", "m", "", "Specific month to classify (format: 2024-01)")
	cmd.Flags().BoolP("resume", "r", false, "Resume from previous session")
	cmd.Flags().Bool("dry-run", false, "Preview without saving changes")

	// Batch mode flags
	cmd.Flags().BoolP("batch", "b", false, "Use batch mode for faster processing")
	cmd.Flags().Float64("auto-accept-threshold", 0.95, "Auto-accept classifications above this confidence (0.0-1.0)")
	cmd.Flags().Int("batch-size", 20, "Number of merchants to process in each LLM batch")
	cmd.Flags().Int("parallel-workers", 5, "Number of parallel workers for batch processing")
	cmd.Flags().Bool("auto-only", false, "Only auto-accept high confidence items, skip manual review")

	// Bind to viper (errors are rare and can be ignored in practice)
	_ = viper.BindPFlag("classification.year", cmd.Flags().Lookup("year"))
	_ = viper.BindPFlag("classification.month", cmd.Flags().Lookup("month"))
	_ = viper.BindPFlag("classification.resume", cmd.Flags().Lookup("resume"))
	_ = viper.BindPFlag("classification.dry_run", cmd.Flags().Lookup("dry-run"))
	_ = viper.BindPFlag("classification.batch", cmd.Flags().Lookup("batch"))
	_ = viper.BindPFlag("classification.auto_accept_threshold", cmd.Flags().Lookup("auto-accept-threshold"))
	_ = viper.BindPFlag("classification.batch_size", cmd.Flags().Lookup("batch-size"))
	_ = viper.BindPFlag("classification.parallel_workers", cmd.Flags().Lookup("parallel-workers"))
	_ = viper.BindPFlag("classification.auto_only", cmd.Flags().Lookup("auto-only"))

	return cmd
}

func runClassify(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	year := viper.GetInt("classification.year")
	month := viper.GetString("classification.month")
	resume := viper.GetBool("classification.resume")
	dryRun := viper.GetBool("classification.dry_run")
	batchMode := viper.GetBool("classification.batch")
	autoAcceptThreshold := viper.GetFloat64("classification.auto_accept_threshold")
	batchSize := viper.GetInt("classification.batch_size")
	parallelWorkers := viper.GetInt("classification.parallel_workers")
	autoOnly := viper.GetBool("classification.auto_only")

	// Set up interrupt handling
	interruptHandler := cli.NewInterruptHandler(nil)
	ctx = interruptHandler.HandleInterrupts(ctx, true)

	slog.Info("Starting transaction categorization")

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
		prompter = cli.NewCLIPrompter(nil, nil)

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
		} else if year != 0 {
			// Use beginning of specified year
			startDate := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
			fromDate = &startDate
		}
		// If year is 0 and no month specified, fromDate remains nil (classify everything)
	}

	// Run classification based on mode
	if batchMode {
		// Use batch mode for faster processing
		opts := engine.BatchClassificationOptions{
			AutoAcceptThreshold: autoAcceptThreshold,
			BatchSize:           batchSize,
			ParallelWorkers:     parallelWorkers,
			SkipManualReview:    autoOnly,
		}

		// Batch mode doesn't support resume
		if resume {
			slog.Warn("Resume is not supported in batch mode, processing all transactions")
		}

		slog.Info("Starting batch classification",
			"auto_accept_threshold", fmt.Sprintf("%.0f%%", autoAcceptThreshold*100),
			"batch_size", batchSize,
			"parallel_workers", parallelWorkers)

		summary, err := classificationEngine.ClassifyTransactionsBatch(ctx, fromDate, opts)
		if err != nil {
			if err == context.Canceled {
				return nil
			}
			return fmt.Errorf("batch classification failed: %w", err)
		}

		// Show batch summary
		slog.Info(summary.GetDisplay())

	} else {
		// Traditional sequential mode
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
				// The interrupt handler already printed the message
				// Just return nil to exit cleanly
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

func expandPath(path string) string {
	if path != "" && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err == nil {
			path = home + path[1:]
		}
	}
	return path
}
