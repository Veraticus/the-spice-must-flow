// Package main contains the spice CLI commands.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/config"
	"github.com/Veraticus/the-spice-must-flow/internal/storage"
	"github.com/Veraticus/the-spice-must-flow/internal/tui"
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

	// The TUI handles its own interrupts

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

	// Determine date range filters
	var startDate, endDate string
	if !resume {
		if month != "" {
			// Parse month
			parsedMonth, parseErr := time.Parse("2006-01", month)
			if parseErr != nil {
				return fmt.Errorf("invalid month format (use YYYY-MM): %w", parseErr)
			}
			startDate = parsedMonth.Format("2006-01-02")
			// End date is the last day of the month
			endMonth := parsedMonth.AddDate(0, 1, -1)
			endDate = endMonth.Format("2006-01-02")
		} else if year > 0 {
			// Use beginning and end of specified year
			startDate = fmt.Sprintf("%d-01-01", year)
			endDate = fmt.Sprintf("%d-12-31", year)
		}
		// If year == 0 (default), dates remain empty, which means classify ALL transactions
	}

	if dryRun {
		slog.Info("Running in dry-run mode - using mock components")
		// TODO: Implement dry-run mode with TUI
		return fmt.Errorf("dry-run mode not yet implemented with TUI")
	}

	// Initialize LLM classifier
	classifier, err := createLLMClient()
	if err != nil {
		return fmt.Errorf("failed to create LLM client: %w", err)
	}

	// Create TUI configuration
	cfg := tui.ClassificationConfig{
		Storage:        db,
		Classifier:     classifier,
		StartDate:      startDate,
		EndDate:        endDate,
		OnlyUnreviewed: !resume,
	}

	// Run classification with TUI
	if err := tui.RunClassification(ctx, cfg); err != nil {
		if err == context.Canceled {
			slog.Warn("Classification interrupted")
			slog.Info("Progress saved! Resume where you left off with: spice classify --resume")
			return nil
		}
		return fmt.Errorf("classification failed: %w", err)
	}

	// Note: Stats are now shown within the TUI, so we don't need to show them here
	slog.Info("Classification complete! Ready for export: spice flow --export")

	return nil
}
