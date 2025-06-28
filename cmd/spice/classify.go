// Package main contains the spice CLI commands.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/cli"
	"github.com/Veraticus/the-spice-must-flow/internal/engine"
	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/service"
	"github.com/Veraticus/the-spice-must-flow/internal/storage"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func classifyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "classify",
		Short: "Categorize transactions",
		Long: `Categorize financial transactions with AI assistance using efficient batch processing.
		
This command fetches transactions from Plaid, groups them by merchant,
and guides you through categorization with minimal effort using batch LLM calls.

By default, this will classify ALL unclassified transactions. Use --year or --month
to limit the scope to a specific time period.

Examples:
  # Classify all unclassified transactions
  spice classify
  
  # Only auto-accept high confidence items, skip manual review
  spice classify --auto-only
  
  # Force manual review for all items (opposite of --auto-only)
  spice classify --manual-review-all
  
  # Custom auto-accept threshold (default: 95%)
  spice classify --auto-accept-threshold=0.90
  
  # Maximum performance with more parallel workers
  spice classify --auto-only --parallel-workers=10
  
  # Classify only 2024 transactions
  spice classify --year 2024
  
  # Classify specific month
  spice classify --month 2024-03
  
  # Re-classify low confidence transactions
  spice classify --rerank 0.85
  
  # Re-classify with custom auto-accept threshold
  spice classify --rerank 0.80 --auto-accept-threshold=0.90`,
		RunE: runClassify,
	}

	// Flags
	cmd.Flags().IntP("year", "y", 0, "Year to classify transactions for (default: all transactions)")
	cmd.Flags().StringP("month", "m", "", "Specific month to classify (format: 2024-01)")
	cmd.Flags().Bool("dry-run", false, "Preview without saving changes")

	// Batch configuration flags
	cmd.Flags().Float64("auto-accept-threshold", 0.95, "Auto-accept classifications above this confidence (0.0-1.0)")
	cmd.Flags().Int("batch-size", 5, "Number of merchants to process in each LLM batch")
	cmd.Flags().Int("parallel-workers", 5, "Number of parallel workers for batch processing")
	cmd.Flags().Bool("auto-only", false, "Only auto-accept high confidence items, skip manual review")
	cmd.Flags().Bool("manual-review-all", false, "Force manual review for all items, even high confidence ones")

	// Reset flags
	cmd.Flags().Bool("reset", false, "Clear all existing classifications before classifying")
	cmd.Flags().String("reset-vendors", "", "Clear vendor rules when using --reset (auto|all)")

	// Rerank flags
	cmd.Flags().Float64("rerank", 0, "Re-classify transactions with confidence below this threshold (0.0-1.0)")

	// Bind to viper (errors are rare and can be ignored in practice)
	_ = viper.BindPFlag("classification.year", cmd.Flags().Lookup("year"))
	_ = viper.BindPFlag("classification.month", cmd.Flags().Lookup("month"))
	_ = viper.BindPFlag("classification.dry_run", cmd.Flags().Lookup("dry-run"))
	_ = viper.BindPFlag("classification.auto_accept_threshold", cmd.Flags().Lookup("auto-accept-threshold"))
	_ = viper.BindPFlag("classification.batch_size", cmd.Flags().Lookup("batch-size"))
	_ = viper.BindPFlag("classification.parallel_workers", cmd.Flags().Lookup("parallel-workers"))
	_ = viper.BindPFlag("classification.auto_only", cmd.Flags().Lookup("auto-only"))
	_ = viper.BindPFlag("classification.manual_review_all", cmd.Flags().Lookup("manual-review-all"))
	_ = viper.BindPFlag("classification.reset", cmd.Flags().Lookup("reset"))
	_ = viper.BindPFlag("classification.reset_vendors", cmd.Flags().Lookup("reset-vendors"))
	_ = viper.BindPFlag("classification.rerank", cmd.Flags().Lookup("rerank"))

	return cmd
}

func runClassify(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	year := viper.GetInt("classification.year")
	month := viper.GetString("classification.month")
	dryRun := viper.GetBool("classification.dry_run")
	autoAcceptThreshold := viper.GetFloat64("classification.auto_accept_threshold")
	batchSize := viper.GetInt("classification.batch_size")
	parallelWorkers := viper.GetInt("classification.parallel_workers")
	autoOnly := viper.GetBool("classification.auto_only")
	manualReviewAll := viper.GetBool("classification.manual_review_all")
	reset := viper.GetBool("classification.reset")
	resetVendors := viper.GetString("classification.reset_vendors")
	rerankThreshold := viper.GetFloat64("classification.rerank")

	// Validate flag combinations
	if autoOnly && manualReviewAll {
		return fmt.Errorf("cannot use both --auto-only and --manual-review-all flags")
	}

	// If manual-review-all is set, effectively set auto-accept threshold to 2.0 (impossible)
	if manualReviewAll {
		autoAcceptThreshold = 2.0
	}

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

	// Handle reset if requested
	if reset {
		if resetErr := handleReset(ctx, db, resetVendors); resetErr != nil {
			return fmt.Errorf("failed to reset classifications: %w", resetErr)
		}
	}

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

	// Check if we're doing rerank instead of normal classification
	if rerankThreshold > 0 {
		if rerankThreshold >= 1.0 {
			return fmt.Errorf("rerank threshold must be less than 1.0")
		}

		slog.Info("Starting re-classification of low confidence transactions",
			"confidence_threshold", fmt.Sprintf("%.0f%%", rerankThreshold*100),
			"auto_accept_threshold", fmt.Sprintf("%.0f%%", autoAcceptThreshold*100))

		opts := engine.RerankOptions{
			ConfidenceThreshold: rerankThreshold,
			AutoAcceptThreshold: autoAcceptThreshold,
			BatchSize:           batchSize,
			ParallelWorkers:     parallelWorkers,
			SkipManualReview:    autoOnly,
		}

		summary, rerankErr := classificationEngine.RerankLowConfidenceTransactions(ctx, opts)
		if rerankErr != nil {
			if rerankErr == context.Canceled {
				return nil
			}
			return fmt.Errorf("rerank failed: %w", rerankErr)
		}

		// Show rerank summary
		slog.Info(summary.GetDisplay())

		return nil
	}

	// Normal classification flow
	opts := engine.BatchClassificationOptions{
		AutoAcceptThreshold: autoAcceptThreshold,
		BatchSize:           batchSize,
		ParallelWorkers:     parallelWorkers,
		SkipManualReview:    autoOnly,
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

	// Show batch summary as JSON
	slog.Info(summary.GetDisplay())

	return nil
}

// showCompletionStats displays completion statistics
// nolint:unused // Kept for future use
func showCompletionStats(stats service.CompletionStats) {
	type completionJSON struct {
		Duration          string  `json:"duration"`
		TimeSaved         string  `json:"time_saved,omitempty"`
		Message           string  `json:"message"`
		TotalTransactions int     `json:"total_transactions"`
		AutoClassified    int     `json:"auto_classified"`
		AutoPercent       float64 `json:"auto_percent,omitempty"`
		UserClassified    int     `json:"user_classified"`
		UserPercent       float64 `json:"user_percent,omitempty"`
		NewVendorRules    int     `json:"new_vendor_rules"`
	}

	data := completionJSON{
		TotalTransactions: stats.TotalTransactions,
		AutoClassified:    stats.AutoClassified,
		UserClassified:    stats.UserClassified,
		NewVendorRules:    stats.NewVendorRules,
		Duration:          stats.Duration.Round(time.Second).String(),
		Message:           "Ready for export: spice flow --export",
	}

	if stats.TotalTransactions > 0 {
		data.AutoPercent = float64(stats.AutoClassified) / float64(stats.TotalTransactions) * 100
		data.UserPercent = float64(stats.UserClassified) / float64(stats.TotalTransactions) * 100

		// Estimate time saved (30 seconds per manual transaction)
		timeSaved := time.Duration(stats.AutoClassified) * 30 * time.Second
		data.TimeSaved = timeSaved.Round(time.Minute).String()
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		slog.Error("Failed to marshal completion stats", "error", err)
		return
	}

	slog.Info(string(bytes))
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

func handleReset(ctx context.Context, db service.Storage, resetVendors string) error {
	// Get count of classifications that will be reset
	start := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Now().Add(24 * time.Hour)

	classifications, err := db.GetClassificationsByDateRange(ctx, start, end)
	if err != nil {
		return fmt.Errorf("failed to get classification count: %w", err)
	}

	var vendorCount int
	var vendorLabel string
	if resetVendors != "" {
		if resetVendors != "auto" && resetVendors != "all" {
			return fmt.Errorf("invalid --reset-vendors value: %s (valid options: auto, all)", resetVendors)
		}

		if resetVendors == "auto" {
			vendors, vendorErr := db.GetVendorsBySource(ctx, model.SourceAuto)
			if vendorErr != nil {
				return fmt.Errorf("failed to get auto vendor count: %w", vendorErr)
			}
			vendorCount = len(vendors)
			vendorLabel = "auto-created vendor rules"
		} else { // all
			vendors, vendorErr := db.GetAllVendors(ctx)
			if vendorErr != nil {
				return fmt.Errorf("failed to get vendor count: %w", vendorErr)
			}
			vendorCount = len(vendors)
			vendorLabel = "all vendor rules"
		}
	}

	// Show what will be reset
	fmt.Printf("\n%s\n", cli.WarningStyle.Render("⚠️  Reset Classifications"))                                        //nolint:forbidigo // User-facing output
	fmt.Printf("\nThis will remove:\n")                                                                               //nolint:forbidigo // User-facing output
	fmt.Printf("  • %s transaction classifications\n", cli.BoldStyle.Render(fmt.Sprintf("%d", len(classifications)))) //nolint:forbidigo // User-facing output
	if resetVendors != "" {
		fmt.Printf("  • %s %s\n", cli.BoldStyle.Render(fmt.Sprintf("%d", vendorCount)), vendorLabel) //nolint:forbidigo // User-facing output
	}
	fmt.Printf("\n%s\n", cli.InfoStyle.Render("ℹ️  Your categories and transactions will remain unchanged.")) //nolint:forbidigo // User-facing output

	// Confirm action
	fmt.Printf("\nAre you sure you want to reset? (y/N): ") //nolint:forbidigo // User prompt
	var response string
	if _, scanErr := fmt.Scanln(&response); scanErr != nil {
		response = "n"
	}
	if strings.ToLower(response) != "y" {
		fmt.Println(cli.InfoStyle.Render("Reset canceled.")) //nolint:forbidigo // User-facing output
		return nil
	}

	// Clear classifications
	slog.Info("Clearing classifications...")

	// Use the new ClearAllClassifications method
	if err := db.ClearAllClassifications(ctx); err != nil {
		return fmt.Errorf("failed to clear classifications: %w", err)
	}

	// Clear vendor rules if requested
	if resetVendors != "" {
		slog.Info("Clearing vendor rules...", "type", resetVendors)
		if resetVendors == "auto" {
			if err := db.DeleteVendorsBySource(ctx, model.SourceAuto); err != nil {
				return fmt.Errorf("failed to delete auto vendor rules: %w", err)
			}
		} else { // all
			vendors, vendorErr := db.GetAllVendors(ctx)
			if vendorErr != nil {
				return fmt.Errorf("failed to get vendors: %w", vendorErr)
			}

			for _, vendor := range vendors {
				if err := db.DeleteVendor(ctx, vendor.Name); err != nil {
					slog.Warn("Failed to delete vendor rule",
						"vendor", vendor.Name,
						"error", err)
				}
			}
		}
	}

	fmt.Printf("\n%s\n\n", cli.SuccessStyle.Render("✓ Reset complete! Ready to reclassify.")) //nolint:forbidigo // User-facing output

	return nil
}
