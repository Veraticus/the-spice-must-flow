package main

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/Veraticus/the-spice-must-flow/internal/cli"
	"github.com/spf13/cobra"
)

func checksDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <pattern-id>",
		Short: "Delete a check pattern",
		Long: `Delete a check pattern (soft delete - can be recovered).

The pattern will be marked as inactive and won't be used for matching,
but the data remains in the database.`,
		Args: cobra.ExactArgs(1),
		RunE: runChecksDelete,
	}

	cmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")

	return cmd
}

func runChecksDelete(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Parse pattern ID
	patternID, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid pattern ID: %w", err)
	}

	force, _ := cmd.Flags().GetBool("force")

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

	// Get the pattern to show details
	pattern, err := storage.GetCheckPattern(ctx, int64(patternID))
	if err != nil {
		return fmt.Errorf("failed to get pattern: %w", err)
	}

	// Show pattern details
	fmt.Println(cli.FormatTitle("🌶️  Delete Check Pattern"))   //nolint:forbidigo // User-facing output
	fmt.Println()                                              //nolint:forbidigo // User-facing output
	fmt.Printf("Pattern ID: %d\n", pattern.ID)                 //nolint:forbidigo // User-facing output
	fmt.Printf("Name: %s\n", pattern.PatternName)              //nolint:forbidigo // User-facing output
	fmt.Printf("Category: %s\n", pattern.Category)             //nolint:forbidigo // User-facing output
	fmt.Printf("Amount: %s\n", formatPatternAmounts(*pattern)) //nolint:forbidigo // User-facing output
	if pattern.DayOfMonthMin != nil && pattern.DayOfMonthMax != nil {
		fmt.Printf("Day restriction: %d-%d\n", *pattern.DayOfMonthMin, *pattern.DayOfMonthMax) //nolint:forbidigo // User-facing output
	}
	fmt.Printf("Uses: %d\n", pattern.UseCount) //nolint:forbidigo // User-facing output
	fmt.Println()                              //nolint:forbidigo // User-facing output

	// Confirm deletion
	if !force {
		fmt.Printf("Are you sure you want to delete pattern %d? (y/N): ", patternID) //nolint:forbidigo // User-facing output
		var response string
		if _, err := fmt.Scanln(&response); err != nil {
			response = "n"
		}
		if strings.ToLower(response) != "y" {
			fmt.Println("Operation canceled.") //nolint:forbidigo // User-facing output
			return nil
		}
	}

	// Delete the pattern
	if err := storage.DeleteCheckPattern(ctx, int64(patternID)); err != nil {
		return fmt.Errorf("failed to delete pattern: %w", err)
	}

	fmt.Println(cli.FormatSuccess(fmt.Sprintf("✓ Pattern %d deleted successfully", patternID)))                 //nolint:forbidigo // User-facing output
	fmt.Println(cli.InfoStyle.Render("Note: This is a soft delete. The pattern data remains in the database.")) //nolint:forbidigo // User-facing output

	return nil
}
