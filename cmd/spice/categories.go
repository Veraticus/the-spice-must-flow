package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/Veraticus/the-spice-must-flow/internal/cli"
	"github.com/Veraticus/the-spice-must-flow/internal/engine"
	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/storage"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

func categoriesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "categories",
		Short: "Manage expense categories",
		Long:  `List, add, update, and delete expense categories used for transaction classification.`,
	}

	cmd.AddCommand(listCategoriesCmd())
	cmd.AddCommand(addCategoryCmd())
	cmd.AddCommand(updateCategoryCmd())
	cmd.AddCommand(deleteCategoryCmd())
	cmd.AddCommand(mergeCategoriesCmd())

	return cmd
}

func listCategoriesCmd() *cobra.Command {
	var categoryType string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all categories",
		Long:  `Display all active categories with their types and descriptions.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx := context.Background()

			// Initialize storage with auto-migration
			store, err := initStorage(ctx)
			if err != nil {
				return err
			}
			defer func() {
				if closeErr := store.Close(); closeErr != nil {
					slog.Error("failed to close storage", "error", closeErr)
				}
			}()

			// Get all categories
			categories, err := store.GetCategories(ctx)
			if err != nil {
				return fmt.Errorf("failed to get categories: %w", err)
			}

			// Filter by type if specified
			if categoryType != "" {
				var filtered []model.Category
				for _, cat := range categories {
					switch categoryType {
					case "income":
						if cat.Type == model.CategoryTypeIncome {
							filtered = append(filtered, cat)
						}
					case "expense":
						if cat.Type == model.CategoryTypeExpense || cat.Type == "" {
							filtered = append(filtered, cat)
						}
					case "system":
						if cat.Type == model.CategoryTypeSystem {
							filtered = append(filtered, cat)
						}
					}
				}
				categories = filtered
			}

			if len(categories) == 0 {
				fmt.Println(cli.InfoStyle.Render("No categories found. Use 'spice categories add' to create one.")) //nolint:forbidigo // User-facing output
				return nil
			}

			// Create table writer
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			defer func() {
				if flushErr := w.Flush(); flushErr != nil {
					slog.Error("failed to flush table writer", "error", flushErr)
				}
			}()

			// Header
			headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
			if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				headerStyle.Render("ID"),
				headerStyle.Render("Name"),
				headerStyle.Render("Type"),
				headerStyle.Render("Description")); err != nil {
				slog.Error("failed to write table header", "error", err)
			}
			if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				strings.Repeat("-", 4),
				strings.Repeat("-", 20),
				strings.Repeat("-", 10),
				strings.Repeat("-", 50)); err != nil {
				slog.Error("failed to write table separator", "error", err)
			}

			// List categories
			for _, cat := range categories {
				desc := cat.Description
				if desc == "" {
					desc = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("(no description)")
				}

				// Format type with color
				var typeStr string
				switch cat.Type {
				case model.CategoryTypeIncome:
					typeStr = lipgloss.NewStyle().Foreground(lipgloss.Color("82")).Render("Income")
				case model.CategoryTypeExpense:
					typeStr = lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Render("Expense")
				case model.CategoryTypeSystem:
					typeStr = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("System")
				default:
					// Default to expense for legacy categories without type
					typeStr = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("Expense")
				}

				if _, err := fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", cat.ID, cat.Name, typeStr, desc); err != nil {
					slog.Error("failed to write category row", "error", err, "category", cat.Name)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&categoryType, "type", "", "Filter by category type (income, expense, system)")

	return cmd
}

func addCategoryCmd() *cobra.Command {
	var (
		categoryDescription string
		skipDescription     bool
		isIncome            bool
	)

	cmd := &cobra.Command{
		Use:   "add <name> [name2] [name3] ...",
		Short: "Add one or more new categories",
		Long: `Create one or more expense or income categories. AI-generated descriptions will be created automatically for each category.

Examples:
  # Add a single expense category
  spice categories add "Travel"
  
  # Add multiple expense categories at once
  spice categories add "Travel" "Entertainment" "Dining" "Healthcare"
  
  # Add income categories
  spice categories add "Salary" "Freelance" "Investments" --income
  
  # Add categories without AI descriptions
  spice categories add "Travel" "Entertainment" --no-description`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			ctx := context.Background()

			// Initialize storage with auto-migration
			store, err := initStorage(ctx)
			if err != nil {
				return err
			}
			defer func() {
				if closeErr := store.Close(); closeErr != nil {
					slog.Error("failed to close storage", "error", closeErr)
				}
			}()

			// Initialize LLM for description generation if needed
			var classifier engine.Classifier
			if !skipDescription && categoryDescription == "" {
				classifier, err = createLLMClient()
				if err != nil {
					return fmt.Errorf("failed to initialize LLM: %w", err)
				}

				// Classifiers from llm package implement Close via embedded Classifier interface
				if closer, ok := classifier.(interface{ Close() error }); ok {
					defer func() {
						if closeErr := closer.Close(); closeErr != nil {
							slog.Error("failed to close LLM client", "error", closeErr)
						}
					}()
				}
			}

			// Track results
			var createdCategories []model.Category
			var skippedCategories []string

			// Process each category
			for _, categoryName := range args {
				// Check if category already exists
				existing, err := store.GetCategoryByName(ctx, categoryName)
				if err != nil && !errors.Is(err, storage.ErrCategoryNotFound) {
					return fmt.Errorf("failed to check existing category %q: %w", categoryName, err)
				}
				if existing != nil {
					skippedCategories = append(skippedCategories, categoryName)
					continue
				}

				// Generate or use provided description
				var description string
				if categoryDescription != "" {
					// Use the same description for all categories if provided
					description = categoryDescription
				} else if !skipDescription && classifier != nil {
					// Generate unique description for each category
					desc, conf, err := classifier.GenerateCategoryDescription(ctx, categoryName)
					if err != nil {
						slog.Warn("Failed to generate category description",
							"category", categoryName,
							"error", err)
						// Continue without description rather than failing
						description = ""
					} else {
						slog.Debug("Generated category description",
							"category", categoryName,
							"description", desc,
							"confidence", conf)
						// Check confidence level
						if conf < 0.7 {
							// Low confidence - prompt user for description
							fmt.Printf("\n%s\n", cli.WarningStyle.Render(fmt.Sprintf("⚠️  Low confidence (%.2f) for category '%s'", conf, categoryName))) //nolint:forbidigo // User-facing output
							fmt.Printf("AI suggested: %s\n\n", desc)                                                                                      //nolint:forbidigo // User-facing output
							fmt.Print("Please provide a better description (or press Enter to use AI suggestion): ")                                      //nolint:forbidigo // User-facing output

							reader := bufio.NewReader(os.Stdin)
							userDesc, _ := reader.ReadString('\n')
							userDesc = strings.TrimSpace(userDesc)

							if userDesc != "" {
								description = userDesc
							} else {
								description = desc
							}
						} else {
							description = desc
						}
					}
				}

				// Create category with appropriate type
				categoryType := model.CategoryTypeExpense
				if isIncome {
					categoryType = model.CategoryTypeIncome
				}
				category, err := store.CreateCategoryWithType(ctx, categoryName, description, categoryType)
				if err != nil {
					return fmt.Errorf("failed to create category %q: %w", categoryName, err)
				}

				createdCategories = append(createdCategories, *category)
			}

			// Display results
			if len(createdCategories) > 0 {
				fmt.Println(cli.SuccessStyle.Render(fmt.Sprintf("✓ Created %d categories:", len(createdCategories)))) //nolint:forbidigo // User-facing output
				for _, cat := range createdCategories {
					typeDisplay := "expense"
					if cat.Type == model.CategoryTypeIncome {
						typeDisplay = "income"
					}
					fmt.Printf("  • %s (ID: %d, type: %s)", cat.Name, cat.ID, typeDisplay) //nolint:forbidigo // User-facing output
					if cat.Description != "" && !skipDescription {
						fmt.Printf(" - %s", cat.Description) //nolint:forbidigo // User-facing output
					}
					fmt.Println() //nolint:forbidigo // User-facing output
				}
			}

			if len(skippedCategories) > 0 {
				fmt.Println(cli.WarningStyle.Render(fmt.Sprintf("⚠ Skipped %d existing categories:", len(skippedCategories)))) //nolint:forbidigo // User-facing output
				for _, name := range skippedCategories {
					fmt.Printf("  • %s\n", name) //nolint:forbidigo // User-facing output
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&categoryDescription, "description", "", "Category description (auto-generated if not provided)")
	cmd.Flags().BoolVar(&skipDescription, "no-description", false, "Skip AI description generation")
	cmd.Flags().BoolVar(&isIncome, "income", false, "Create income categories instead of expense categories")

	return cmd
}

func updateCategoryCmd() *cobra.Command {
	var (
		categoryName        string
		categoryDescription string
		regenerateDesc      bool
	)

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a category",
		Long:  `Update the name or description of an existing category. Use --regenerate to create a new AI-generated description.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			ctx := context.Background()

			// Parse category ID
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid category ID: %w", err)
			}

			if categoryName == "" && categoryDescription == "" && !regenerateDesc {
				return fmt.Errorf("must specify --name, --description, or --regenerate to update")
			}

			// Initialize storage with auto-migration
			store, err := initStorage(ctx)
			if err != nil {
				return err
			}
			defer func() {
				if closeErr := store.Close(); closeErr != nil {
					slog.Error("failed to close storage", "error", closeErr)
				}
			}()

			// Get current category
			categories, err := store.GetCategories(ctx)
			if err != nil {
				return fmt.Errorf("failed to get categories: %w", err)
			}

			var currentCategory *model.Category
			for _, cat := range categories {
				if cat.ID == id {
					currentCategory = &cat
					break
				}
			}

			if currentCategory == nil {
				return fmt.Errorf("category with ID %d not found", id)
			}

			// Use current values if not specified
			name := currentCategory.Name
			if categoryName != "" {
				name = categoryName
			}

			description := currentCategory.Description
			if regenerateDesc {
				// Generate new description using LLM
				classifier, err := createLLMClient()
				if err != nil {
					return fmt.Errorf("failed to initialize LLM: %w", err)
				}

				// Classifiers from llm package implement Close via embedded Classifier interface
				if closer, ok := classifier.(interface{ Close() error }); ok {
					defer func() {
						if closeErr := closer.Close(); closeErr != nil {
							slog.Error("failed to close LLM client", "error", closeErr)
						}
					}()
				}

				generatedDesc, conf, err := classifier.GenerateCategoryDescription(ctx, name)
				if err != nil {
					return fmt.Errorf("failed to generate category description: %w", err)
				}
				description = generatedDesc
				// Log low confidence descriptions
				if conf < 0.7 {
					slog.Warn("Low confidence category description",
						"category", name,
						"confidence", conf,
						"description", generatedDesc)
				}
			} else if categoryDescription != "" {
				description = categoryDescription
			}

			// Update category
			if err := store.UpdateCategory(ctx, id, name, description); err != nil {
				return fmt.Errorf("failed to update category: %w", err)
			}

			fmt.Println(cli.SuccessStyle.Render(fmt.Sprintf("✓ Updated category %d", id))) //nolint:forbidigo // User-facing output
			if regenerateDesc {
				fmt.Printf("  Description: %s\n", description) //nolint:forbidigo // User-facing output
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&categoryName, "name", "", "New category name")
	cmd.Flags().StringVar(&categoryDescription, "description", "", "New category description")
	cmd.Flags().BoolVar(&regenerateDesc, "regenerate", false, "Regenerate description using AI")

	return cmd
}

func deleteCategoryCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete <id> [id2] [id3] ...",
		Short: "Delete one or more categories",
		Long: `Delete one or more categories. This will fail if any transactions are using the categories.

Examples:
  # Delete a single category
  spice categories delete 5
  
  # Delete multiple categories
  spice categories delete 5 7 12
  
  # Delete without confirmation
  spice categories delete 5 7 --force`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			ctx := context.Background()

			// Parse category IDs
			var categoryIDs []int
			for _, arg := range args {
				id, err := strconv.Atoi(arg)
				if err != nil {
					return fmt.Errorf("invalid category ID %q: %w", arg, err)
				}
				categoryIDs = append(categoryIDs, id)
			}

			// Initialize storage with auto-migration
			store, err := initStorage(ctx)
			if err != nil {
				return err
			}
			defer func() {
				if closeErr := store.Close(); closeErr != nil {
					slog.Error("failed to close storage", "error", closeErr)
				}
			}()

			// Confirm deletion
			if !force {
				if len(categoryIDs) == 1 {
					fmt.Printf("Are you sure you want to delete category %d? (y/N): ", categoryIDs[0]) //nolint:forbidigo // User prompt
				} else {
					fmt.Printf("Are you sure you want to delete %d categories (%v)? (y/N): ", len(categoryIDs), categoryIDs) //nolint:forbidigo // User prompt
				}
				var response string
				if _, err := fmt.Scanln(&response); err != nil {
					// EOF or empty input is treated as "N"
					response = "n"
				}
				if strings.ToLower(response) != "y" {
					if _, err := fmt.Fprintln(os.Stdout, "Deletion canceled."); err != nil {
						slog.Error("failed to write output", "error", err)
					}
					return nil
				}
			}

			// Track results
			var deletedIDs []int
			var failedIDs []int

			// Delete categories
			for _, id := range categoryIDs {
				if err := store.DeleteCategory(ctx, id); err != nil {
					slog.Warn("Failed to delete category",
						"id", id,
						"error", err)
					failedIDs = append(failedIDs, id)
				} else {
					deletedIDs = append(deletedIDs, id)
				}
			}

			// Display results
			if len(deletedIDs) > 0 {
				if len(deletedIDs) == 1 {
					fmt.Println(cli.SuccessStyle.Render(fmt.Sprintf("✓ Deleted category %d", deletedIDs[0]))) //nolint:forbidigo // User-facing output
				} else {
					fmt.Println(cli.SuccessStyle.Render(fmt.Sprintf("✓ Deleted %d categories:", len(deletedIDs)))) //nolint:forbidigo // User-facing output
					for _, id := range deletedIDs {
						fmt.Printf("  • Category %d\n", id) //nolint:forbidigo // User-facing output
					}
				}
			}

			if len(failedIDs) > 0 {
				fmt.Println(cli.ErrorStyle.Render(fmt.Sprintf("✗ Failed to delete %d categories:", len(failedIDs)))) //nolint:forbidigo // User-facing output
				for _, id := range failedIDs {
					fmt.Printf("  • Category %d (likely has associated transactions)\n", id) //nolint:forbidigo // User-facing output
				}
			}

			// Return error if all deletions failed
			if len(deletedIDs) == 0 && len(failedIDs) > 0 {
				return fmt.Errorf("failed to delete any categories")
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompt")

	return cmd
}

func mergeCategoriesCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "merge <from-category-id> <to-category-id>",
		Short: "Merge one category into another",
		Long: `Merge all transactions from one category into another, then delete the source category.
This is useful for consolidating duplicate or similar categories.

Example:
  spice categories merge 5 7
  
This will move all transactions from category ID 5 to category ID 7 and delete category 5.`,
		Args: cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			ctx := context.Background()

			// Parse category IDs
			fromCategoryID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid source category ID: %w", err)
			}

			toCategoryID, err := strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("invalid target category ID: %w", err)
			}

			// Initialize storage with auto-migration
			store, err := initStorage(ctx)
			if err != nil {
				return err
			}
			defer func() {
				if closeErr := store.Close(); closeErr != nil {
					slog.Error("failed to close storage", "error", closeErr)
				}
			}()

			// Get categories to verify they exist
			categories, err := store.GetCategories(ctx)
			if err != nil {
				return fmt.Errorf("failed to get categories: %w", err)
			}

			var fromCategory, toCategory *model.Category
			for _, cat := range categories {
				switch cat.ID {
				case fromCategoryID:
					fromCategory = &cat
				case toCategoryID:
					toCategory = &cat
				}
			}

			if fromCategory == nil {
				return fmt.Errorf("source category with ID %d not found", fromCategoryID)
			}
			if toCategory == nil {
				return fmt.Errorf("target category with ID %d not found", toCategoryID)
			}

			// Get count of transactions and vendors that will be affected
			transactionCount, err := store.GetTransactionCountByCategory(ctx, fromCategory.Name)
			if err != nil {
				return fmt.Errorf("failed to get transaction count: %w", err)
			}

			vendors, err := store.GetVendorsByCategory(ctx, fromCategory.Name)
			if err != nil {
				return fmt.Errorf("failed to get vendor count: %w", err)
			}
			vendorCount := len(vendors)

			// Show merge preview
			fmt.Println(cli.InfoStyle.Render("Merge Preview:"))                     //nolint:forbidigo // User-facing output
			fmt.Printf("  From: %s (ID: %d)\n", fromCategory.Name, fromCategory.ID) //nolint:forbidigo // User-facing output
			fmt.Printf("  To:   %s (ID: %d)\n", toCategory.Name, toCategory.ID)     //nolint:forbidigo // User-facing output
			fmt.Printf("  Transactions to update: %d\n", transactionCount)          //nolint:forbidigo // User-facing output
			fmt.Printf("  Vendors to update: %d\n", vendorCount)                    //nolint:forbidigo // User-facing output
			fmt.Println()                                                           //nolint:forbidigo // User-facing output

			// Confirm merge
			if !force {
				fmt.Printf("Are you sure you want to merge category '%s' into '%s'? (y/N): ", fromCategory.Name, toCategory.Name) //nolint:forbidigo // User prompt
				var response string
				if _, err := fmt.Scanln(&response); err != nil {
					// EOF or empty input is treated as "N"
					response = "n"
				}
				if strings.ToLower(response) != "y" {
					if _, err := fmt.Fprintln(os.Stdout, "Merge canceled."); err != nil {
						slog.Error("failed to write output", "error", err)
					}
					return nil
				}
			}

			// Perform the merge
			fmt.Println(cli.InfoStyle.Render("Performing merge...")) //nolint:forbidigo // User-facing output

			// Update transactions
			if transactionCount > 0 {
				if err := store.UpdateTransactionCategoriesByID(ctx, fromCategoryID, toCategoryID); err != nil {
					return fmt.Errorf("failed to update transactions: %w", err)
				}
				fmt.Printf("  ✓ Updated %d transactions\n", transactionCount) //nolint:forbidigo // User-facing output
			}

			// Update vendors
			if vendorCount > 0 {
				if err := store.UpdateVendorCategoriesByID(ctx, fromCategoryID, toCategoryID); err != nil {
					return fmt.Errorf("failed to update vendors: %w", err)
				}
				fmt.Printf("  ✓ Updated %d vendors\n", vendorCount) //nolint:forbidigo // User-facing output
			}

			// Delete the source category
			if err := store.DeleteCategory(ctx, fromCategoryID); err != nil {
				// If deletion fails, it's still a successful merge
				slog.Warn("Failed to delete source category after merge",
					"category", fromCategory.Name,
					"id", fromCategoryID,
					"error", err)
				fmt.Println(cli.WarningStyle.Render(fmt.Sprintf("⚠ Could not delete source category '%s' (ID: %d) - it may still have references", fromCategory.Name, fromCategoryID))) //nolint:forbidigo // User-facing output
			} else {
				fmt.Printf("  ✓ Deleted source category '%s'\n", fromCategory.Name) //nolint:forbidigo // User-facing output
			}

			fmt.Println()                                                                                                                 //nolint:forbidigo // User-facing output
			fmt.Println(cli.SuccessStyle.Render(fmt.Sprintf("✓ Successfully merged '%s' into '%s'", fromCategory.Name, toCategory.Name))) //nolint:forbidigo // User-facing output

			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompt")

	return cmd
}
