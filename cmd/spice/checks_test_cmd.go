package main

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/cli"
	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/spf13/cobra"
)

func checksTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test <amount>",
		Short: "Test which patterns match a hypothetical check",
		Long: `Test which check patterns would match a hypothetical check transaction.

This helps you verify that your patterns are working as expected before
encountering real transactions.

Examples:
  spice checks test 100
  spice checks test 100 --date=2024-01-05
  spice checks test 3000 --check-number=1234`,
		Args: cobra.ExactArgs(1),
		RunE: runChecksTest,
	}

	cmd.Flags().String("date", "", "Test with specific date (YYYY-MM-DD)")
	cmd.Flags().String("check-number", "", "Test with specific check number")

	return cmd
}

func runChecksTest(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Parse amount
	amountStr := strings.TrimPrefix(args[0], "$")
	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		return fmt.Errorf("invalid amount: %w", err)
	}

	// Parse optional date
	testDate := time.Now()
	dateStr, _ := cmd.Flags().GetString("date")
	if dateStr != "" {
		parsedDate, errDate := time.Parse("2006-01-02", dateStr)
		if errDate != nil {
			return fmt.Errorf("invalid date format (use YYYY-MM-DD): %w", errDate)
		}
		testDate = parsedDate
	}

	// Parse optional check number
	checkNumberStr, _ := cmd.Flags().GetString("check-number")

	// Initialize storage
	storage, err := initStorage(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer func() {
		if closeErr := storage.Close(); closeErr != nil {
			slog.Error("failed to close storage", "error", closeErr)
		}
	}()

	// Create a test transaction
	testTxn := model.Transaction{
		Amount:      amount,
		Date:        testDate,
		Type:        "CHECK",
		Name:        fmt.Sprintf("Check #%s", checkNumberStr),
		CheckNumber: checkNumberStr,
	}

	// Get matching patterns
	patterns, err := storage.GetMatchingCheckPatterns(ctx, testTxn)
	if err != nil {
		return fmt.Errorf("failed to get matching patterns: %w", err)
	}

	// Display results
	fmt.Println(cli.FormatTitle("🌶️  Check Pattern Test"))                             //nolint:forbidigo // User-facing output
	fmt.Println()                                                                      //nolint:forbidigo // User-facing output
	fmt.Printf("Test transaction:\n")                                                  //nolint:forbidigo // User-facing output
	fmt.Printf("  Amount: $%.2f\n", amount)                                            //nolint:forbidigo // User-facing output
	fmt.Printf("  Date: %s (day %d)\n", testDate.Format("2006-01-02"), testDate.Day()) //nolint:forbidigo // User-facing output
	if checkNumberStr != "" {
		fmt.Printf("  Check number: %s\n", checkNumberStr) //nolint:forbidigo // User-facing output
	}
	fmt.Println() //nolint:forbidigo // User-facing output

	if len(patterns) == 0 {
		fmt.Println(cli.InfoStyle.Render("No patterns match this transaction.")) //nolint:forbidigo // User-facing output
		return nil
	}

	fmt.Printf("Matching patterns (%d):\n\n", len(patterns)) //nolint:forbidigo // User-facing output

	for _, pattern := range patterns {
		// Show pattern details
		fmt.Printf("✓ Pattern: %s (ID: %d)\n", //nolint:forbidigo // User-facing output
			cli.SuccessStyle.Render(pattern.PatternName), pattern.ID)
		fmt.Printf("  Category: %s\n", pattern.Category)                         //nolint:forbidigo // User-facing output
		fmt.Printf("  Confidence boost: +%.1f%%\n", pattern.ConfidenceBoost*100) //nolint:forbidigo // User-facing output
		fmt.Printf("  Previous uses: %d\n", pattern.UseCount)                    //nolint:forbidigo // User-facing output

		// Show why it matched
		fmt.Printf("  Matched because:\n") //nolint:forbidigo // User-facing output

		// Amount match
		if pattern.AmountMax == nil || *pattern.AmountMax == *pattern.AmountMin {
			fmt.Printf("    - Amount equals $%.2f\n", *pattern.AmountMin) //nolint:forbidigo // User-facing output
		} else {
			fmt.Printf("    - Amount in range $%.2f - $%.2f\n", //nolint:forbidigo // User-facing output
				*pattern.AmountMin, *pattern.AmountMax)
		}

		// Day match
		if pattern.DayOfMonthMin != nil && pattern.DayOfMonthMax != nil {
			fmt.Printf("    - Day %d is within range %d-%d\n", //nolint:forbidigo // User-facing output
				testDate.Day(), *pattern.DayOfMonthMin, *pattern.DayOfMonthMax)
		}

		// Check number match
		if pattern.CheckNumberPattern != nil {
			fmt.Printf("    - Check number pattern matched\n") //nolint:forbidigo // User-facing output
		}

		if pattern.Notes != "" {
			fmt.Printf("  Notes: %s\n", pattern.Notes) //nolint:forbidigo // User-facing output
		}

		fmt.Println() //nolint:forbidigo // User-facing output
	}

	// Show recommendation
	if len(patterns) == 1 {
		fmt.Println(cli.FormatInfo(fmt.Sprintf( //nolint:forbidigo // User-facing output
			"This check would be suggested for category: %s",
			patterns[0].Category)))
	} else {
		fmt.Println(cli.FormatInfo( //nolint:forbidigo // User-facing output
			"Multiple patterns match. The LLM would consider all of them when suggesting a category."))
	}

	return nil
}
