package main

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/Veraticus/the-spice-must-flow/internal/engine"
	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/spf13/cobra"
)

func patternsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "patterns",
		Aliases: []string{"pattern"},
		Short:   "Manage pattern-based classification rules",
		Long: `Manage intelligent pattern-based classification rules that consider transaction 
direction, amount conditions, and merchant patterns for accurate categorization.`,
	}

	// Subcommands
	cmd.AddCommand(patternsListCmd())
	cmd.AddCommand(patternsShowCmd())
	cmd.AddCommand(patternsCreateCmd())
	cmd.AddCommand(patternsEditCmd())
	cmd.AddCommand(patternsDeleteCmd())
	cmd.AddCommand(patternsTestCmd())

	return cmd
}

func patternsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List pattern rules",
		Long:  `List all pattern-based classification rules with their conditions and usage.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			// Get database connection
			db, cleanup, err := getDatabase()
			if err != nil {
				return err
			}
			defer cleanup()

			// Get filter flags
			category, _ := cmd.Flags().GetString("category")
			activeOnly, _ := cmd.Flags().GetBool("active")

			// Fetch patterns
			var patterns []model.PatternRule
			switch {
			case category != "":
				patterns, err = db.GetPatternRulesByCategory(ctx, category)
			case !activeOnly:
				// When activeOnly is false, we still get active patterns
				// as there's no separate method for all patterns (active + inactive)
				patterns, err = db.GetActivePatternRules(ctx)
			default:
				patterns, err = db.GetActivePatternRules(ctx)
			}

			if err != nil {
				return fmt.Errorf("failed to get pattern rules: %w", err)
			}

			if len(patterns) == 0 {
				slog.Info("No pattern rules found")
				return nil
			}

			// Display patterns in a table
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "ID\tNAME\tMERCHANT\tAMOUNT\tDIRECTION\tCATEGORY\tCONFIDENCE\tUSE COUNT")
			_, _ = fmt.Fprintln(w, "──\t────\t────────\t──────\t─────────\t────────\t──────────\t─────────")

			for _, pattern := range patterns {
				// Format merchant pattern
				merchant := pattern.MerchantPattern
				if merchant == "" {
					merchant = "any"
				} else if pattern.IsRegex {
					merchant = "/" + merchant + "/"
				}

				// Format amount condition
				amount := formatAmountCondition(pattern)

				// Format direction
				direction := "any"
				if pattern.Direction != nil {
					direction = string(*pattern.Direction)
				}

				_, _ = fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\t%.0f%%\t%d\n",
					pattern.ID,
					truncateString(pattern.Name, 20),
					merchant,
					amount,
					direction,
					pattern.DefaultCategory,
					pattern.Confidence*100,
					pattern.UseCount)
			}

			return w.Flush()
		},
	}

	cmd.Flags().StringP("category", "c", "", "Filter by category")
	cmd.Flags().BoolP("active", "a", true, "Show only active patterns")
	return cmd
}

func patternsShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show pattern rule details",
		Long:  `Display detailed information about a specific pattern rule.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Parse pattern ID
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid pattern ID: %s", args[0])
			}

			// Get database connection
			db, cleanup, err := getDatabase()
			if err != nil {
				return err
			}
			defer cleanup()

			// Get pattern
			pattern, err := db.GetPatternRule(ctx, id)
			if err != nil {
				return fmt.Errorf("pattern rule %d not found", id)
			}

			// Display pattern details
			slog.Info("Pattern Rule Details:")
			slog.Info("  ID", "id", pattern.ID)
			slog.Info("  Name", "name", pattern.Name)
			slog.Info("  Description", "description", pattern.Description)

			if pattern.MerchantPattern != "" {
				if pattern.IsRegex {
					slog.Info("  Merchant Pattern", "pattern", "/"+pattern.MerchantPattern+"/", "type", "regex")
				} else {
					slog.Info("  Merchant Pattern", "pattern", pattern.MerchantPattern, "type", "exact")
				}
			} else {
				slog.Info("  Merchant Pattern", "pattern", "any merchant")
			}

			slog.Info("  Amount Condition", "condition", formatAmountCondition(*pattern))

			if pattern.Direction != nil {
				slog.Info("  Direction", "direction", string(*pattern.Direction))
			} else {
				slog.Info("  Direction", "direction", "any")
			}

			slog.Info("  Default Category", "category", pattern.DefaultCategory)
			slog.Info("  Confidence", "confidence", fmt.Sprintf("%.0f%%", pattern.Confidence*100))
			slog.Info("  Priority", "priority", pattern.Priority)
			slog.Info("  Active", "active", pattern.IsActive)
			slog.Info("  Use Count", "count", pattern.UseCount)
			slog.Info("  Created", "date", pattern.CreatedAt.Format("2006-01-02 15:04:05"))
			slog.Info("  Updated", "date", pattern.UpdatedAt.Format("2006-01-02 15:04:05"))

			return nil
		},
	}
}

func patternsCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a pattern rule",
		Long:  `Create a new pattern-based classification rule.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			// Get required flags
			name, _ := cmd.Flags().GetString("name")
			category, _ := cmd.Flags().GetString("category")

			if name == "" || category == "" {
				return fmt.Errorf("name and category are required")
			}

			// Get optional flags
			description, _ := cmd.Flags().GetString("description")
			merchant, _ := cmd.Flags().GetString("merchant")
			isRegex, _ := cmd.Flags().GetBool("regex")
			amountCond, _ := cmd.Flags().GetString("amount-condition")
			amountValue, _ := cmd.Flags().GetFloat64("amount-value")
			amountMin, _ := cmd.Flags().GetFloat64("amount-min")
			amountMax, _ := cmd.Flags().GetFloat64("amount-max")
			direction, _ := cmd.Flags().GetString("direction")
			confidence, _ := cmd.Flags().GetFloat64("confidence")
			priority, _ := cmd.Flags().GetInt("priority")

			// Validate amount condition
			if amountCond != "" && amountCond != "any" {
				validConditions := []string{"lt", "le", "eq", "ge", "gt", "range"}
				valid := false
				for _, vc := range validConditions {
					if amountCond == vc {
						valid = true
						break
					}
				}
				if !valid {
					return fmt.Errorf("invalid amount condition: %s (valid: lt, le, eq, ge, gt, range, any)", amountCond)
				}

				// Validate required values
				if amountCond == "range" {
					if amountMin == 0 && amountMax == 0 {
						return fmt.Errorf("range condition requires --amount-min and/or --amount-max")
					}
				} else if amountCond != "any" && amountValue == 0 {
					return fmt.Errorf("%s condition requires --amount-value", amountCond)
				}
			}

			// Validate direction
			var directionPtr *model.TransactionDirection
			if direction != "" {
				switch direction {
				case "income":
					d := model.DirectionIncome
					directionPtr = &d
				case "expense":
					d := model.DirectionExpense
					directionPtr = &d
				case "transfer":
					d := model.DirectionTransfer
					directionPtr = &d
				default:
					return fmt.Errorf("invalid direction: %s (valid: income, expense, transfer)", direction)
				}
			}

			// Get database connection
			db, cleanup, err := getDatabase()
			if err != nil {
				return err
			}
			defer cleanup()

			// Create pattern rule
			pattern := &model.PatternRule{
				Name:            name,
				Description:     description,
				MerchantPattern: merchant,
				IsRegex:         isRegex,
				AmountCondition: amountCond,
				Direction:       directionPtr,
				DefaultCategory: category,
				Confidence:      confidence / 100.0, // Convert percentage to decimal
				Priority:        priority,
				IsActive:        true,
			}

			// Set amount values
			if amountCond == "range" {
				if amountMin > 0 {
					pattern.AmountMin = &amountMin
				}
				if amountMax > 0 {
					pattern.AmountMax = &amountMax
				}
			} else if amountCond != "any" && amountCond != "" {
				pattern.AmountValue = &amountValue
			}

			if amountCond == "" {
				pattern.AmountCondition = "any"
			}

			if err := db.CreatePatternRule(ctx, pattern); err != nil {
				return fmt.Errorf("failed to create pattern rule: %w", err)
			}

			slog.Info("✓ Pattern rule created successfully",
				"id", pattern.ID,
				"name", pattern.Name,
				"category", pattern.DefaultCategory)

			// Refresh patterns in classification engine
			if engine := getClassificationEngine(); engine != nil {
				if err := engine.RefreshPatternRules(ctx); err != nil {
					slog.Warn("failed to refresh pattern rules in classification engine", "error", err)
				}
			}

			return nil
		},
	}

	// Required flags
	cmd.Flags().StringP("name", "n", "", "Name for the pattern rule (required)")
	cmd.Flags().StringP("category", "c", "", "Default category for matching transactions (required)")

	// Optional flags
	cmd.Flags().StringP("description", "d", "", "Description of the pattern rule")
	cmd.Flags().StringP("merchant", "m", "", "Merchant pattern to match")
	cmd.Flags().BoolP("regex", "r", false, "Treat merchant pattern as regular expression")
	cmd.Flags().String("amount-condition", "", "Amount condition (lt, le, eq, ge, gt, range)")
	cmd.Flags().Float64("amount-value", 0, "Amount value for comparison")
	cmd.Flags().Float64("amount-min", 0, "Minimum amount for range condition")
	cmd.Flags().Float64("amount-max", 0, "Maximum amount for range condition")
	cmd.Flags().String("direction", "", "Transaction direction (income, expense, transfer)")
	cmd.Flags().Float64("confidence", 80, "Confidence percentage (0-100)")
	cmd.Flags().IntP("priority", "p", 0, "Priority (higher values override lower)")

	if err := cmd.MarkFlagRequired("name"); err != nil {
		slog.Error("failed to mark flag as required", "error", err)
	}
	if err := cmd.MarkFlagRequired("category"); err != nil {
		slog.Error("failed to mark flag as required", "error", err)
	}

	return cmd
}

func patternsEditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit <id>",
		Short: "Edit a pattern rule",
		Long:  `Edit an existing pattern rule.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Parse pattern ID
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid pattern ID: %s", args[0])
			}

			// Get database connection
			db, cleanup, err := getDatabase()
			if err != nil {
				return err
			}
			defer cleanup()

			// Get existing pattern
			pattern, err := db.GetPatternRule(ctx, id)
			if err != nil {
				return fmt.Errorf("pattern rule %d not found", id)
			}

			// Update fields if flags provided
			changed := false

			if name, _ := cmd.Flags().GetString("name"); name != "" {
				pattern.Name = name
				changed = true
			}

			if description, _ := cmd.Flags().GetString("description"); description != "" {
				pattern.Description = description
				changed = true
			}

			if category, _ := cmd.Flags().GetString("category"); category != "" {
				pattern.DefaultCategory = category
				changed = true
			}

			if cmd.Flags().Changed("active") {
				active, _ := cmd.Flags().GetBool("active")
				pattern.IsActive = active
				changed = true
			}

			if cmd.Flags().Changed("priority") {
				priority, _ := cmd.Flags().GetInt("priority")
				pattern.Priority = priority
				changed = true
			}

			if cmd.Flags().Changed("confidence") {
				confidence, _ := cmd.Flags().GetFloat64("confidence")
				pattern.Confidence = confidence / 100.0
				changed = true
			}

			if !changed {
				slog.Info("No changes specified")
				return nil
			}

			// Update pattern
			if err := db.UpdatePatternRule(ctx, pattern); err != nil {
				return fmt.Errorf("failed to update pattern rule: %w", err)
			}

			slog.Info("✓ Pattern rule updated successfully", "id", id)

			// Refresh patterns in classification engine
			if engine := getClassificationEngine(); engine != nil {
				if err := engine.RefreshPatternRules(ctx); err != nil {
					slog.Warn("failed to refresh pattern rules in classification engine", "error", err)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringP("name", "n", "", "New name for the pattern rule")
	cmd.Flags().StringP("description", "d", "", "New description")
	cmd.Flags().StringP("category", "c", "", "New default category")
	cmd.Flags().Bool("active", true, "Set active status")
	cmd.Flags().IntP("priority", "p", 0, "New priority")
	cmd.Flags().Float64("confidence", 0, "New confidence percentage (0-100)")

	return cmd
}

func patternsDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a pattern rule",
		Long:  `Delete a pattern rule.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Parse pattern ID
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid pattern ID: %s", args[0])
			}

			// Get database connection
			db, cleanup, err := getDatabase()
			if err != nil {
				return err
			}
			defer cleanup()

			// Get pattern to show details
			pattern, err := db.GetPatternRule(ctx, id)
			if err != nil {
				return fmt.Errorf("pattern rule %d not found", id)
			}

			// Show pattern info
			_, _ = fmt.Fprintf(os.Stdout, "About to delete pattern rule:\n")
			_, _ = fmt.Fprintf(os.Stdout, "  ID: %d\n", pattern.ID)
			_, _ = fmt.Fprintf(os.Stdout, "  Name: %s\n", pattern.Name)
			_, _ = fmt.Fprintf(os.Stdout, "  Category: %s\n", pattern.DefaultCategory)
			_, _ = fmt.Fprintf(os.Stdout, "  Use Count: %d\n\n", pattern.UseCount)

			// Get confirmation unless --force flag is set
			force, _ := cmd.Flags().GetBool("force")
			if !force {
				slog.Info("Are you sure you want to delete this pattern rule? (y/N): ")
				var response string
				_, _ = fmt.Scanln(&response)
				if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
					slog.Info("Deletion canceled")
					return nil
				}
			}

			// Delete pattern
			if err := db.DeletePatternRule(ctx, id); err != nil {
				return fmt.Errorf("failed to delete pattern rule: %w", err)
			}

			slog.Info("Pattern rule deleted successfully", "id", id)

			// Refresh patterns in classification engine
			if engine := getClassificationEngine(); engine != nil {
				if err := engine.RefreshPatternRules(ctx); err != nil {
					slog.Warn("failed to refresh pattern rules in classification engine", "error", err)
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	return cmd
}

func patternsTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Test pattern rules against a transaction",
		Long:  `Test which pattern rules would match a transaction with given attributes.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Get test parameters
			merchant, _ := cmd.Flags().GetString("merchant")
			amount, _ := cmd.Flags().GetFloat64("amount")
			direction, _ := cmd.Flags().GetString("direction")

			if merchant == "" {
				return fmt.Errorf("merchant name is required")
			}

			// Create test transaction
			txn := model.Transaction{}

			// Set direction
			switch direction {
			case "income":
				txn.Direction = model.DirectionIncome
			case "expense":
				txn.Direction = model.DirectionExpense
			case "transfer":
				txn.Direction = model.DirectionTransfer
			default:
				// Use amount to infer direction if not specified
				if amount < 0 {
					txn.Direction = model.DirectionIncome
				} else {
					txn.Direction = model.DirectionExpense
				}
			}

			// Get database connection
			_, cleanup, err := getDatabase()
			if err != nil {
				return err
			}
			defer cleanup()

			// Get classification engine
			engine := getClassificationEngine()
			if engine == nil {
				return fmt.Errorf("classification engine not available")
			}

			// Test pattern matching
			slog.Info("Testing transaction:",
				"merchant", merchant,
				"amount", fmt.Sprintf("%.2f", amount),
				"direction", txn.Direction)

			// Note: This would need an exported method on ClassificationEngine to test patterns
			// For now, just indicate that testing is not yet implemented
			slog.Info("Pattern testing not yet implemented in CLI")
			slog.Info("Pattern rules will be applied during normal classification process")

			return nil
		},
	}

	cmd.Flags().StringP("merchant", "m", "", "Merchant name to test (required)")
	cmd.Flags().Float64P("amount", "a", 0, "Transaction amount")
	cmd.Flags().StringP("direction", "d", "", "Transaction direction (income, expense, transfer)")

	if err := cmd.MarkFlagRequired("merchant"); err != nil {
		slog.Error("failed to mark flag as required", "error", err)
	}

	return cmd
}

// Helper functions

func formatAmountCondition(pattern model.PatternRule) string {
	switch pattern.AmountCondition {
	case "any":
		return "any"
	case "lt":
		if pattern.AmountValue != nil {
			return fmt.Sprintf("< %.2f", *pattern.AmountValue)
		}
	case "le":
		if pattern.AmountValue != nil {
			return fmt.Sprintf("≤ %.2f", *pattern.AmountValue)
		}
	case "eq":
		if pattern.AmountValue != nil {
			return fmt.Sprintf("= %.2f", *pattern.AmountValue)
		}
	case "ge":
		if pattern.AmountValue != nil {
			return fmt.Sprintf("≥ %.2f", *pattern.AmountValue)
		}
	case "gt":
		if pattern.AmountValue != nil {
			return fmt.Sprintf("> %.2f", *pattern.AmountValue)
		}
	case "range":
		parts := []string{}
		if pattern.AmountMin != nil {
			parts = append(parts, fmt.Sprintf("%.2f", *pattern.AmountMin))
		} else {
			parts = append(parts, "∞")
		}
		parts = append(parts, "-")
		if pattern.AmountMax != nil {
			parts = append(parts, fmt.Sprintf("%.2f", *pattern.AmountMax))
		} else {
			parts = append(parts, "∞")
		}
		return strings.Join(parts, "")
	}
	return "?"
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// getClassificationEngine returns the global classification engine instance.
var classificationEngine *engine.ClassificationEngine

func getClassificationEngine() *engine.ClassificationEngine {
	// This would need to be set during application initialization
	// For now, return nil if not available
	return classificationEngine
}
