package main

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/joshsymonds/the-spice-must-flow/internal/cli"
	"github.com/spf13/cobra"
)

func checksEditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit <pattern-id>",
		Short: "Edit an existing check pattern",
		Long: `Edit an existing check pattern. Shows current values as defaults
and allows you to change each field.`,
		Args: cobra.ExactArgs(1),
		RunE: runChecksEdit,
	}

	return cmd
}

func runChecksEdit(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Parse pattern ID
	patternID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid pattern ID: %w", err)
	}

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

	// Get the pattern
	pattern, err := storage.GetCheckPattern(ctx, patternID)
	if err != nil {
		return fmt.Errorf("failed to get pattern: %w", err)
	}

	// Get categories for validation
	categories, err := storage.GetCategories(ctx)
	if err != nil {
		return fmt.Errorf("failed to get categories: %w", err)
	}

	fmt.Println(cli.FormatTitle("🌶️  Edit Check Pattern")) //nolint:forbidigo // User-facing output
	fmt.Println()                                          //nolint:forbidigo // User-facing output

	// Show current pattern details
	fmt.Println("Current pattern:")                              //nolint:forbidigo // User-facing output
	fmt.Printf("  Name: %s\n", pattern.PatternName)              //nolint:forbidigo // User-facing output
	fmt.Printf("  Category: %s\n", pattern.Category)             //nolint:forbidigo // User-facing output
	fmt.Printf("  Amount: %s\n", formatPatternAmounts(*pattern)) //nolint:forbidigo // User-facing output
	if pattern.DayOfMonthMin != nil && pattern.DayOfMonthMax != nil {
		fmt.Printf("  Day restriction: %d-%d\n", *pattern.DayOfMonthMin, *pattern.DayOfMonthMax) //nolint:forbidigo // User-facing output
	}
	if pattern.Notes != "" {
		fmt.Printf("  Notes: %s\n", pattern.Notes) //nolint:forbidigo // User-facing output
	}
	fmt.Println() //nolint:forbidigo // User-facing output

	reader := bufio.NewReader(os.Stdin)

	// Store original values for comparison
	original := *pattern

	// Edit pattern name
	patternName, err := promptStringWithDefault(reader, "Pattern name", pattern.PatternName)
	if err != nil {
		return fmt.Errorf("failed to get pattern name: %w", err)
	}
	if patternName != "" {
		pattern.PatternName = patternName
	}

	// Edit category
	fmt.Println("\nAvailable categories:") //nolint:forbidigo // User-facing output
	for _, cat := range categories {
		fmt.Printf("  - %s\n", cat.Name) //nolint:forbidigo // User-facing output
	}

	for {
		category, errCat := promptStringWithDefault(reader, "\nCategory", pattern.Category)
		if errCat != nil {
			return fmt.Errorf("failed to get category: %w", errCat)
		}

		if category == "" {
			break // Keep existing
		}

		// Validate category exists
		found := false
		for _, cat := range categories {
			if strings.EqualFold(cat.Name, category) {
				pattern.Category = cat.Name // Use exact case
				found = true
				break
			}
		}

		if found {
			break
		}

		fmt.Println(cli.FormatError("Category not found. Please choose from the list above.")) //nolint:forbidigo // User-facing output
	}

	// Edit amount
	fmt.Println("\nAmount matching:")           //nolint:forbidigo // User-facing output
	fmt.Println("  [1] Keep current amount(s)") //nolint:forbidigo // User-facing output
	fmt.Println("  [2] Change to exact amount") //nolint:forbidigo // User-facing output
	fmt.Println("  [3] Change to range")        //nolint:forbidigo // User-facing output

	amountChoice, err := promptChoice(reader, "Choice", []string{"1", "2", "3"})
	if err != nil {
		return fmt.Errorf("failed to get amount choice: %w", err)
	}

	switch amountChoice {
	case "2": // Exact amount
		amount, errAmt := promptAmount(reader, "Amount")
		if errAmt != nil {
			return fmt.Errorf("failed to get amount: %w", errAmt)
		}
		pattern.AmountMin = &amount
		pattern.AmountMax = nil

	case "3": // Range
		minAmount, errMin := promptAmount(reader, "Minimum amount")
		if errMin != nil {
			return fmt.Errorf("failed to get minimum amount: %w", errMin)
		}
		maxAmount, errMax := promptAmount(reader, "Maximum amount")
		if errMax != nil {
			return fmt.Errorf("failed to get maximum amount: %w", errMax)
		}
		if maxAmount <= minAmount {
			return fmt.Errorf("maximum amount must be greater than minimum amount")
		}
		pattern.AmountMin = &minAmount
		pattern.AmountMax = &maxAmount
	}

	// Edit day restriction
	if pattern.DayOfMonthMin != nil && pattern.DayOfMonthMax != nil {
		keepRestriction, errKeep := promptYesNo(reader, "Keep day of month restriction?")
		if errKeep != nil {
			return fmt.Errorf("failed to get day restriction choice: %w", errKeep)
		}
		if !keepRestriction {
			pattern.DayOfMonthMin = nil
			pattern.DayOfMonthMax = nil
		} else {
			// Allow editing the values
			minDay, errMinDay := promptIntWithDefault(reader, "Minimum day of month (1-31)", *pattern.DayOfMonthMin, 1, 31)
			if errMinDay != nil {
				return fmt.Errorf("failed to get minimum day: %w", errMinDay)
			}
			maxDay, errMaxDay := promptIntWithDefault(reader, "Maximum day of month (1-31)", *pattern.DayOfMonthMax, minDay, 31)
			if errMaxDay != nil {
				return fmt.Errorf("failed to get maximum day: %w", errMaxDay)
			}
			pattern.DayOfMonthMin = &minDay
			pattern.DayOfMonthMax = &maxDay
		}
	} else {
		addRestriction, errAdd := promptYesNo(reader, "Add day of month restriction?")
		if errAdd != nil {
			return fmt.Errorf("failed to get day restriction choice: %w", errAdd)
		}
		if addRestriction {
			minDay, errMinDay := promptInt(reader, "Minimum day of month (1-31)", 1, 31)
			if errMinDay != nil {
				return fmt.Errorf("failed to get minimum day: %w", errMinDay)
			}
			maxDay, errMaxDay := promptInt(reader, "Maximum day of month (1-31)", minDay, 31)
			if errMaxDay != nil {
				return fmt.Errorf("failed to get maximum day: %w", errMaxDay)
			}
			pattern.DayOfMonthMin = &minDay
			pattern.DayOfMonthMax = &maxDay
		}
	}

	// Edit notes
	currentNotes := pattern.Notes
	notes, err := promptStringWithDefault(reader, "\nNotes", currentNotes)
	if err != nil {
		return fmt.Errorf("failed to get notes: %w", err)
	}
	if notes != "" {
		pattern.Notes = notes
	}

	// Show changes summary
	fmt.Println("\nChanges summary:") //nolint:forbidigo // User-facing output
	hasChanges := false

	if pattern.PatternName != original.PatternName {
		fmt.Printf("  Name: %s → %s\n", original.PatternName, pattern.PatternName) //nolint:forbidigo // User-facing output
		hasChanges = true
	}
	if pattern.Category != original.Category {
		fmt.Printf("  Category: %s → %s\n", original.Category, pattern.Category) //nolint:forbidigo // User-facing output
		hasChanges = true
	}
	if pattern.AmountMin != original.AmountMin ||
		(pattern.AmountMax == nil && original.AmountMax != nil) ||
		(pattern.AmountMax != nil && original.AmountMax == nil) ||
		(pattern.AmountMax != nil && original.AmountMax != nil && *pattern.AmountMax != *original.AmountMax) {
		fmt.Printf("  Amount: %s → %s\n", formatPatternAmounts(original), formatPatternAmounts(*pattern)) //nolint:forbidigo // User-facing output
		hasChanges = true
	}

	if !hasChanges {
		fmt.Println(cli.InfoStyle.Render("No changes made.")) //nolint:forbidigo // User-facing output
		return nil
	}

	// Confirm changes
	confirm, err := promptYesNo(reader, "\nSave changes?")
	if err != nil {
		return fmt.Errorf("failed to get confirmation: %w", err)
	}

	if !confirm {
		fmt.Println("Changes discarded.") //nolint:forbidigo // User-facing output
		return nil
	}

	// Update the pattern
	if err := storage.UpdateCheckPattern(ctx, pattern); err != nil {
		return fmt.Errorf("failed to update pattern: %w", err)
	}

	fmt.Println(cli.FormatSuccess("✓ Pattern updated successfully")) //nolint:forbidigo // User-facing output
	return nil
}

func promptStringWithDefault(reader *bufio.Reader, prompt, defaultValue string) (string, error) {
	fmt.Printf("%s [%s]: ", cli.FormatPrompt(prompt), defaultValue) //nolint:forbidigo // User-facing output

	input, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return defaultValue, nil
	}

	return input, nil
}

func promptIntWithDefault(reader *bufio.Reader, prompt string, defaultValue, minVal, maxVal int) (int, error) {
	for {
		fmt.Printf("%s [%d]: ", cli.FormatPrompt(prompt), defaultValue) //nolint:forbidigo // User-facing output

		input, err := reader.ReadString('\n')
		if err != nil {
			return 0, err
		}

		input = strings.TrimSpace(input)
		if input == "" {
			return defaultValue, nil
		}

		value, err := strconv.Atoi(input)
		if err != nil {
			fmt.Println(cli.FormatError("Invalid number. Please try again.")) //nolint:forbidigo // User-facing output
			continue
		}

		if value < minVal || value > maxVal {
			fmt.Printf(cli.FormatError("Number must be between %d and %d.\n"), minVal, maxVal) //nolint:forbidigo // User-facing output
			continue
		}

		return value, nil
	}
}
