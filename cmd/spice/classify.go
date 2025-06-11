// Package main contains the spice CLI commands.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/joshsymonds/the-spice-must-flow/internal/cli"
	"github.com/joshsymonds/the-spice-must-flow/internal/engine"
	"github.com/joshsymonds/the-spice-must-flow/internal/llm"
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
		} else {
			// Use beginning of year
			startDate := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
			fromDate = &startDate
		}
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
			slog.Info("Progress saved. Resume with: spice classify --resume")
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
	slog.Info("Classification complete!", "total_transactions", stats.TotalTransactions)

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

func createLLMClient() (engine.Classifier, error) {
	// Read LLM configuration from viper
	provider := viper.GetString("llm.provider")
	if provider == "" {
		provider = "openai" // default provider
	}

	// Build config from viper settings
	config := llm.Config{
		Provider:    provider,
		Model:       viper.GetString("llm.model"),
		Temperature: viper.GetFloat64("llm.temperature"),
		MaxTokens:   viper.GetInt("llm.max_tokens"),
		MaxRetries:  viper.GetInt("llm.max_retries"),
		RetryDelay:  viper.GetDuration("llm.retry_delay"),
		CacheTTL:    viper.GetDuration("llm.cache_ttl"),
		RateLimit:   viper.GetInt("llm.rate_limit"),
	}

	// Set defaults if not specified
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = time.Second
	}
	if config.CacheTTL == 0 {
		config.CacheTTL = 24 * time.Hour
	}
	if config.RateLimit == 0 {
		config.RateLimit = 1000 // requests per minute
	}

	// Get API key based on provider
	switch provider {
	case "openai":
		// Check viper first, then environment variable
		apiKey := viper.GetString("llm.openai_api_key")
		if apiKey == "" {
			apiKey = os.Getenv("OPENAI_API_KEY")
		}
		if apiKey == "" {
			return nil, fmt.Errorf("OpenAI API key not found in config or OPENAI_API_KEY environment variable")
		}
		config.APIKey = apiKey

		// Set default model if not specified
		if config.Model == "" {
			config.Model = "gpt-4-turbo-preview"
		}

	case "anthropic":
		// Check viper first, then environment variable
		apiKey := viper.GetString("llm.anthropic_api_key")
		if apiKey == "" {
			apiKey = os.Getenv("ANTHROPIC_API_KEY")
		}
		if apiKey == "" {
			return nil, fmt.Errorf("anthropic API key not found in config or ANTHROPIC_API_KEY environment variable")
		}
		config.APIKey = apiKey

		// Set default model if not specified
		if config.Model == "" {
			config.Model = "claude-3-sonnet-20240229"
		}

	case "claudecode":
		// No API key needed for local CLI
		if config.Model == "" {
			config.Model = "sonnet"
		}

	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", provider)
	}

	// Create classifier with all the wrappers
	classifier, err := llm.NewClassifier(config, slog.Default())
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM classifier: %w", err)
	}

	slog.Info("Initialized LLM classifier",
		"provider", provider,
		"model", config.Model,
		"cache_ttl", config.CacheTTL,
		"rate_limit", config.RateLimit)

	return classifier, nil
}
