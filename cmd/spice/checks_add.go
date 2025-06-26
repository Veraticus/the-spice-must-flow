package main

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/Veraticus/the-spice-must-flow/internal/cli"
	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/spf13/cobra"
)

func checksAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Create a new check pattern",
		Long: `Interactively create a new check pattern for automatic categorization.

You'll be prompted to enter:
  - Pattern name and category
  - Amount matching criteria (exact, range, or multiple)
  - Optional day-of-month restrictions
  - Notes for future reference`,
		RunE: runChecksAdd,
	}

	return cmd
}

func runChecksAdd(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

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

	// Start interactive pattern creation
	fmt.Println(cli.FormatTitle("🌶️  Create Check Pattern")) //nolint:forbidigo // User-facing output
	fmt.Println()                                            //nolint:forbidigo // User-facing output

	reader := bufio.NewReader(os.Stdin)

	// Get pattern name
	patternName, err := promptString(reader, "Pattern name")
	if err != nil {
		return fmt.Errorf("failed to get pattern name: %w", err)
	}
	if patternName == "" {
		return fmt.Errorf("please provide a name for this check pattern")
	}

	// Get category - validate it exists
	categories, err := storage.GetCategories(ctx)
	if err != nil {
		return fmt.Errorf("failed to get categories: %w", err)
	}

	fmt.Println("\nAvailable categories:") //nolint:forbidigo // User-facing output
	for i, cat := range categories {
		fmt.Printf("  [%d] %s\n", i+1, cat.Name) //nolint:forbidigo // User-facing output
	}

	var category string
	for {
		categoryInput, err := promptString(reader, "\nCategory (enter number)")
		if err != nil {
			return fmt.Errorf("failed to get category: %w", err)
		}

		// Try to parse as number first
		if num, parseErr := strconv.Atoi(categoryInput); parseErr == nil {
			if num >= 1 && num <= len(categories) {
				category = categories[num-1].Name
				break
			}
			fmt.Println(cli.FormatError(fmt.Sprintf("Please enter a number between 1 and %d", len(categories)))) //nolint:forbidigo // User-facing output
			continue
		}

		// Fall back to name matching for backwards compatibility
		found := false
		for _, cat := range categories {
			if strings.EqualFold(cat.Name, categoryInput) {
				category = cat.Name // Use exact case
				found = true
				break
			}
		}

		if found {
			break
		}

		fmt.Println(cli.FormatError("Invalid selection. Please enter a number from the list above.")) //nolint:forbidigo // User-facing output
	}

	// Get amount matching type
	fmt.Println("\nAmount matching:")     //nolint:forbidigo // User-facing output
	fmt.Println("  [1] Exact amount")     //nolint:forbidigo // User-facing output
	fmt.Println("  [2] Range")            //nolint:forbidigo // User-facing output
	fmt.Println("  [3] Multiple amounts") //nolint:forbidigo // User-facing output

	amountType, err := promptChoice(reader, "Choice", []string{"1", "2", "3"})
	if err != nil {
		return fmt.Errorf("failed to get amount type: %w", err)
	}

	pattern := model.CheckPattern{
		PatternName:     patternName,
		Category:        category,
		ConfidenceBoost: 0.3, // Default confidence boost
	}

	// Handle amount based on type
	switch amountType {
	case "1": // Exact amount
		amount, errAmt := promptAmount(reader, "Amount")
		if errAmt != nil {
			return fmt.Errorf("failed to get amount: %w", errAmt)
		}
		pattern.AmountMin = &amount

	case "2": // Range
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

	case "3": // Multiple amounts
		amounts, errAmts := promptMultipleAmounts(reader)
		if errAmts != nil {
			return fmt.Errorf("failed to get amounts: %w", errAmts)
		}

		// Store all amounts in the single pattern
		pattern.Amounts = amounts
	}

	// Day of month restriction
	useDayRestriction, err := promptYesNo(reader, "Would you like to restrict this pattern to specific days of the month?")
	if err != nil {
		return fmt.Errorf("failed to get day restriction choice: %w", err)
	}

	if useDayRestriction {
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

	// Optional notes
	notes, err := promptString(reader, "\nNotes (optional)")
	if err != nil {
		return fmt.Errorf("failed to get notes: %w", err)
	}
	if notes != "" {
		pattern.Notes = notes
	}

	// Create the pattern
	if err := storage.CreateCheckPattern(ctx, &pattern); err != nil {
		return fmt.Errorf("failed to create pattern: %w", err)
	}

	// Success message
	fmt.Println()                                                                                 //nolint:forbidigo // User-facing output
	fmt.Println(cli.FormatSuccess(fmt.Sprintf("✓ Pattern created: \"%s\"", pattern.PatternName))) //nolint:forbidigo // User-facing output

	amountStr := formatPatternAmounts(pattern)
	fmt.Printf("  Matches checks for %s → %s\n", amountStr, pattern.Category) //nolint:forbidigo // User-facing output

	if pattern.DayOfMonthMin != nil && pattern.DayOfMonthMax != nil {
		fmt.Printf("  Only on days %d-%d of the month\n", *pattern.DayOfMonthMin, *pattern.DayOfMonthMax) //nolint:forbidigo // User-facing output
	}

	return nil
}

func promptString(reader *bufio.Reader, prompt string) (string, error) {
	fmt.Printf("%s: ", cli.FormatPrompt(prompt)) //nolint:forbidigo // User-facing output

	input, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(input), nil
}

func promptChoice(reader *bufio.Reader, prompt string, validChoices []string) (string, error) {
	for {
		fmt.Printf("%s: ", cli.FormatPrompt(prompt)) //nolint:forbidigo // User-facing output

		input, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}

		choice := strings.ToLower(strings.TrimSpace(input))

		for _, valid := range validChoices {
			if choice == valid {
				return choice, nil
			}
		}

		fmt.Println(cli.FormatError("Invalid choice. Please try again.")) //nolint:forbidigo // User-facing output
	}
}

func promptYesNo(reader *bufio.Reader, prompt string) (bool, error) {
	fmt.Printf("%s [y/N]: ", cli.FormatPrompt(prompt)) //nolint:forbidigo // User-facing output

	input, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}

	response := strings.ToLower(strings.TrimSpace(input))
	return response == "y" || response == "yes", nil
}

func promptAmount(reader *bufio.Reader, prompt string) (float64, error) {
	for {
		input, err := promptString(reader, prompt)
		if err != nil {
			return 0, err
		}

		// Remove $ if present
		input = strings.TrimPrefix(input, "$")

		amount, err := strconv.ParseFloat(input, 64)
		if err != nil {
			fmt.Println(cli.FormatError("Please enter a valid amount (numbers only, no currency symbols needed)")) //nolint:forbidigo // User-facing output
			continue
		}

		if amount <= 0 {
			fmt.Println(cli.FormatError("Please enter an amount greater than $0")) //nolint:forbidigo // User-facing output
			continue
		}

		return amount, nil
	}
}

func promptInt(reader *bufio.Reader, prompt string, minVal, maxVal int) (int, error) {
	for {
		input, err := promptString(reader, prompt)
		if err != nil {
			return 0, err
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

func promptMultipleAmounts(reader *bufio.Reader) ([]float64, error) {
	input, err := promptString(reader, "Enter check amounts separated by commas (e.g., 100, 250, 500)")
	if err != nil {
		return nil, err
	}

	if input == "" {
		return nil, fmt.Errorf("no amounts provided")
	}

	parts := strings.Split(input, ",")
	amounts := make([]float64, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		part = strings.TrimPrefix(part, "$")

		amount, err := strconv.ParseFloat(part, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid amount '%s': %w", part, err)
		}

		if amount <= 0 {
			return nil, fmt.Errorf("amount must be greater than 0: %s", part)
		}

		amounts = append(amounts, amount)
	}

	if len(amounts) == 0 {
		return nil, fmt.Errorf("no amounts provided")
	}

	return amounts, nil
}
