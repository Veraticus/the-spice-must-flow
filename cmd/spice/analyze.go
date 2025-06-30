package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/analysis"
	"github.com/Veraticus/the-spice-must-flow/internal/cli"
	"github.com/Veraticus/the-spice-must-flow/internal/engine"
	"github.com/Veraticus/the-spice-must-flow/internal/storage"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func analyzeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "Perform AI-powered analysis of transaction categorization",
		Long: `Analyze your transaction data for issues, patterns, and optimization opportunities.

This command examines your categorized transactions using AI to identify:
- Inconsistent categorizations (similar transactions in different categories)
- Missing pattern rules for recurring transactions
- Ambiguous vendor mappings
- Overall categorization coherence

The analysis provides actionable insights and can automatically apply fixes
for high-confidence issues when run with --auto-apply.

Examples:
  # Analyze last 30 days of transactions
  spice analyze

  # Analyze specific date range
  spice analyze --start-date 2024-01-01 --end-date 2024-03-31

  # Focus on pattern effectiveness
  spice analyze --focus patterns

  # Dry run - preview without applying fixes
  spice analyze --dry-run

  # Auto-apply high confidence fixes
  spice analyze --auto-apply

  # Continue a previous session
  spice analyze --session-id abc123`,
		RunE: runAnalyze,
	}

	// Date range flags
	cmd.Flags().String("start-date", "", "Start date for analysis (YYYY-MM-DD)")
	cmd.Flags().String("end-date", "", "End date for analysis (YYYY-MM-DD)")

	// Analysis configuration
	cmd.Flags().String("focus", "all", "Analysis focus area (all, coherence, patterns, categories)")
	cmd.Flags().Int("max-issues", 50, "Maximum number of issues to report")
	cmd.Flags().String("session-id", "", "Continue a previous analysis session")

	// Execution mode
	cmd.Flags().Bool("dry-run", false, "Preview analysis without making changes")
	cmd.Flags().Bool("auto-apply", false, "Automatically apply high-confidence fixes")

	// Output format
	cmd.Flags().String("output", "interactive", "Output format (interactive, summary, json)")

	return cmd
}

func runAnalyze(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	// Set up interrupt handling
	interruptHandler := cli.NewInterruptHandler(nil)
	ctx = interruptHandler.HandleInterrupts(ctx, true)

	// Parse flags
	startDateStr, _ := cmd.Flags().GetString("start-date")
	endDateStr, _ := cmd.Flags().GetString("end-date")
	focusStr, _ := cmd.Flags().GetString("focus")
	maxIssues, _ := cmd.Flags().GetInt("max-issues")
	sessionID, _ := cmd.Flags().GetString("session-id")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	autoApply, _ := cmd.Flags().GetBool("auto-apply")
	outputFormat, _ := cmd.Flags().GetString("output")

	// Parse dates
	var startDate, endDate time.Time
	var err error

	if startDateStr != "" {
		startDate, err = time.Parse("2006-01-02", startDateStr)
		if err != nil {
			return fmt.Errorf("invalid start date format (use YYYY-MM-DD): %w", err)
		}
	} else {
		// Default to 30 days ago
		startDate = time.Now().AddDate(0, 0, -30)
	}

	if endDateStr != "" {
		endDate, err = time.Parse("2006-01-02", endDateStr)
		if err != nil {
			return fmt.Errorf("invalid end date format (use YYYY-MM-DD): %w", err)
		}
	} else {
		// Default to today
		endDate = time.Now()
	}

	// Validate focus
	var focus analysis.Focus
	switch focusStr {
	case "all":
		focus = analysis.FocusAll
	case "coherence":
		focus = analysis.FocusCoherence
	case "patterns":
		focus = analysis.FocusPatterns
	case "categories":
		focus = analysis.FocusCategories
	default:
		return fmt.Errorf("invalid focus: %s (valid options: all, coherence, patterns, categories)", focusStr)
	}

	slog.Info("Starting transaction analysis",
		"start_date", startDate.Format("2006-01-02"),
		"end_date", endDate.Format("2006-01-02"),
		"focus", focusStr)

	// Set up database
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

	// Create LLM client for analysis (uses opus for Claude Code)
	llmClient, err := createAnalysisLLMClient()
	if err != nil {
		return fmt.Errorf("failed to create LLM client: %w", err)
	}

	// Create pattern engine
	patternClassifier, err := engine.NewPatternClassifier(db)
	if err != nil {
		return fmt.Errorf("failed to create pattern classifier: %w", err)
	}

	// Create analysis dependencies
	promptBuilder, err := analysis.NewTemplatePromptBuilder()
	if err != nil {
		return fmt.Errorf("failed to create prompt builder: %w", err)
	}
	validator := analysis.NewJSONValidator()
	formatter := analysis.NewCLIFormatter()
	sessionStore := analysis.NewMemorySessionStore()
	fixApplier := analysis.NewTransactionalFixApplier(db, patternClassifier)
	llmAdapter := analysis.NewLLMAnalysisAdapter(llmClient)

	deps := analysis.Deps{
		Storage:       db,
		LLMClient:     llmAdapter,
		SessionStore:  sessionStore,
		ReportStore:   sessionStore, // MemorySessionStore implements both interfaces
		Validator:     validator,
		FixApplier:    fixApplier,
		PromptBuilder: promptBuilder,
		Formatter:     formatter,
	}

	// Create analysis engine
	analysisEngine, err := analysis.NewEngine(deps)
	if err != nil {
		return fmt.Errorf("failed to create analysis engine: %w", err)
	}

	// Progress callback
	progress := func(stage string, percent int) {
		fmt.Printf("\r%-40s %3d%%", stage, percent)
		if percent == 100 {
			fmt.Println()
		}
	}

	// Create analysis options
	opts := analysis.Options{
		StartDate:    startDate,
		EndDate:      endDate,
		Focus:        focus,
		SessionID:    sessionID,
		MaxIssues:    maxIssues,
		DryRun:       dryRun,
		AutoApply:    autoApply,
		ProgressFunc: progress,
	}

	// Perform analysis
	report, err := analysisEngine.Analyze(ctx, opts)
	if err != nil {
		if err == context.Canceled {
			fmt.Println("\n\nAnalysis canceled by user")
			return nil
		}
		return fmt.Errorf("analysis failed: %w", err)
	}

	// Display results based on output format
	switch outputFormat {
	case "json":
		// Export as JSON for programmatic use
		if err := exportReportJSON(report); err != nil {
			return fmt.Errorf("failed to export report: %w", err)
		}

	case "summary":
		// Display summary only
		fmt.Println("\n" + formatter.FormatSummary(report))

	case "interactive":
		// Interactive report navigation
		fmt.Println("\n" + formatter.FormatInteractive(report))

	default:
		return fmt.Errorf("invalid output format: %s", outputFormat)
	}

	// Show next steps
	if !autoApply && report.HasActionableIssues() {
		fmt.Println("\nTo apply recommended fixes, run:")
		fmt.Printf("  spice analyze --auto-apply --session-id %s\n", report.SessionID)
	}

	return nil
}

// exportReportJSON exports the analysis report as JSON to stdout.
func exportReportJSON(report *analysis.Report) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		return fmt.Errorf("failed to encode report as JSON: %w", err)
	}
	return nil
}
