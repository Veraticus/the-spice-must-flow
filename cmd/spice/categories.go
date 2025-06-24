package main

import (
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
	return &cobra.Command{
		Use:   "list",
		Short: "List all categories",
		Long:  `Display all active expense categories with their descriptions.`,
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
			if _, err := fmt.Fprintf(w, "%s\t%s\t%s\n",
				headerStyle.Render("ID"),
				headerStyle.Render("Name"),
				headerStyle.Render("Description")); err != nil {
				slog.Error("failed to write table header", "error", err)
			}
			if _, err := fmt.Fprintf(w, "%s\t%s\t%s\n",
				strings.Repeat("-", 4),
				strings.Repeat("-", 20),
				strings.Repeat("-", 50)); err != nil {
				slog.Error("failed to write table separator", "error", err)
			}

			// List categories
			for _, cat := range categories {
				desc := cat.Description
				if desc == "" {
					desc = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("(no description)")
				}
				if _, err := fmt.Fprintf(w, "%d\t%s\t%s\n", cat.ID, cat.Name, desc); err != nil {
					slog.Error("failed to write category row", "error", err, "category", cat.Name)
				}
			}

			return nil
		},
	}
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
		Long: `Create one or more categories. AI-generated descriptions will be created automatically for each category.
By default, expense categories are created. Use --income to create income categories.

Examples:
  # Add expense categories (default)
  spice categories add "Travel" "Entertainment" "Dining"
  
  # Add income categories
  spice categories add "Salary" "Freelance" "Investment Returns" --income
  
  # Add multiple categories at once
  spice categories add "Travel" "Entertainment" "Dining" "Healthcare"
  
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
					var confidence float64
					description, confidence, err = classifier.GenerateCategoryDescription(ctx, categoryName)
					if err != nil {
						// Log warning but continue without description
						slog.Warn("failed to generate category description",
							"category", categoryName,
							"error", err)
					} else if confidence < 0.7 {
						// If LLM confidence is low, prompt user for description
						description = promptForCategoryDescription(categoryName, description, confidence)
					}
				}

				// Create category with appropriate type
				categoryType := model.CategoryTypeExpense
				if isIncome {
					categoryType = model.CategoryTypeIncome
				}

				category, err := store.CreateCategory(ctx, categoryName, description, categoryType)
				if err != nil {
					return fmt.Errorf("failed to create category %q: %w", categoryName, err)
				}

				createdCategories = append(createdCategories, *category)
			}

			// Display results
			if len(createdCategories) > 0 {
				categoryTypeStr := "expense"
				if isIncome {
					categoryTypeStr = "income"
				}

				fmt.Println(cli.SuccessStyle.Render(fmt.Sprintf("✓ Created %d %s categor%s:", //nolint:forbidigo
					len(createdCategories),
					categoryTypeStr,
					map[bool]string{true: "y", false: "ies"}[len(createdCategories) == 1])))

				for _, cat := range createdCategories {
					fmt.Printf("  • %s (ID: %d)", cat.Name, cat.ID) //nolint:forbidigo // User-facing output
					if cat.Description != "" {
						fmt.Printf(" - %s", cat.Description) //nolint:forbidigo // User-facing output
					}
					fmt.Println() //nolint:forbidigo // User-facing output
				}
			}

			if len(skippedCategories) > 0 {
				fmt.Printf("\n%s Skipped %d existing categor%s: %s\n", //nolint:forbidigo
					cli.InfoIcon,
					len(skippedCategories),
					map[bool]string{true: "y", false: "ies"}[len(skippedCategories) == 1],
					strings.Join(skippedCategories, ", ")) //nolint:forbidigo // User-facing output
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

				generatedDesc, confidence, err := classifier.GenerateCategoryDescription(ctx, name)
				if err != nil {
					return fmt.Errorf("failed to generate category description: %w", err)
				}
				if confidence < 0.7 {
					// If LLM confidence is low, prompt user for description
					description = promptForCategoryDescription(name, generatedDesc, confidence)
				} else {
					description = generatedDesc
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
  
  # Delete with confirmation skip
  spice categories delete 5 7 12 --force`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			ctx := context.Background()

			// Parse all category IDs
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

			// Get category details for confirmation
			var categoriesToDelete []model.Category
			for _, id := range categoryIDs {
				cat, err := store.GetCategoryByID(ctx, id)
				if err != nil {
					if errors.Is(err, storage.ErrCategoryNotFound) {
						return fmt.Errorf("category with ID %d not found", id)
					}
					return fmt.Errorf("failed to get category %d: %w", id, err)
				}
				categoriesToDelete = append(categoriesToDelete, *cat)
			}

			// Confirm deletion
			if !force {
				fmt.Printf("\n%s About to delete %d categor%s:\n", cli.WarningIcon, //nolint:forbidigo
					len(categoriesToDelete),
					map[bool]string{true: "y", false: "ies"}[len(categoriesToDelete) == 1]) //nolint:forbidigo // User-facing output

				for _, cat := range categoriesToDelete {
					fmt.Printf("  • %s (ID: %d)\n", cat.Name, cat.ID) //nolint:forbidigo // User-facing output
				}

				fmt.Printf("\nAre you sure you want to delete %s? (y/N): ", //nolint:forbidigo
					map[bool]string{true: "this category", false: "these categories"}[len(categoriesToDelete) == 1]) //nolint:forbidigo // User prompt

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

			// Delete all categories
			var deletedCategories []model.Category
			var failedDeletions []string

			for _, cat := range categoriesToDelete {
				if err := store.DeleteCategory(ctx, cat.ID); err != nil {
					failedDeletions = append(failedDeletions, fmt.Sprintf("%s (ID: %d): %v", cat.Name, cat.ID, err))
				} else {
					deletedCategories = append(deletedCategories, cat)
				}
			}

			// Report results
			if len(deletedCategories) > 0 {
				fmt.Println(cli.SuccessStyle.Render(fmt.Sprintf("✓ Deleted %d categor%s:", //nolint:forbidigo
					len(deletedCategories),
					map[bool]string{true: "y", false: "ies"}[len(deletedCategories) == 1])))

				for _, cat := range deletedCategories {
					fmt.Printf("  • %s (ID: %d)\n", cat.Name, cat.ID) //nolint:forbidigo // User-facing output
				}
			}

			if len(failedDeletions) > 0 {
				fmt.Printf("\n%s Failed to delete %d categor%s:\n", cli.ErrorIcon, //nolint:forbidigo
					len(failedDeletions),
					map[bool]string{true: "y", false: "ies"}[len(failedDeletions) == 1])

				for _, failure := range failedDeletions {
					fmt.Printf("  • %s\n", failure) //nolint:forbidigo // User-facing output
				}

				return fmt.Errorf("failed to delete %d categories", len(failedDeletions))
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

			// Get both categories
			fromCat, err := store.GetCategoryByID(ctx, fromCategoryID)
			if err != nil {
				if errors.Is(err, storage.ErrCategoryNotFound) {
					return fmt.Errorf("source category with ID %d not found", fromCategoryID)
				}
				return fmt.Errorf("failed to get source category: %w", err)
			}

			toCat, err := store.GetCategoryByID(ctx, toCategoryID)
			if err != nil {
				if errors.Is(err, storage.ErrCategoryNotFound) {
					return fmt.Errorf("target category with ID %d not found", toCategoryID)
				}
				return fmt.Errorf("failed to get target category: %w", err)
			}

			// Count affected transactions
			txns, err := store.GetTransactionsByCategoryID(ctx, fromCategoryID)
			if err != nil {
				return fmt.Errorf("failed to count affected transactions: %w", err)
			}

			// Count affected vendors
			vendors, err := store.GetVendorsByCategoryID(ctx, fromCategoryID)
			if err != nil {
				return fmt.Errorf("failed to count affected vendors: %w", err)
			}

			// Confirm merge
			if !force {
				fmt.Printf("\n%s Merge Summary:\n", cli.InfoIcon)                              //nolint:forbidigo // User-facing output
				fmt.Printf("  From: %s (ID: %d)\n", fromCat.Name, fromCat.ID)                  //nolint:forbidigo // User-facing output
				fmt.Printf("  To:   %s (ID: %d)\n", toCat.Name, toCat.ID)                      //nolint:forbidigo // User-facing output
				fmt.Printf("\n  This will affect:\n")                                          //nolint:forbidigo // User-facing output
				fmt.Printf("    • %d transactions\n", len(txns))                               //nolint:forbidigo // User-facing output
				fmt.Printf("    • %d vendor rules\n", len(vendors))                            //nolint:forbidigo // User-facing output
				fmt.Printf("\n  The category %q will be deleted after merge.\n", fromCat.Name) //nolint:forbidigo // User-facing output
				fmt.Printf("\nProceed with merge? (y/N): ")                                    //nolint:forbidigo // User prompt

				var response string
				if _, err := fmt.Scanln(&response); err != nil {
					// EOF or empty input is treated as "N"
					response = "n"
				}
				if strings.ToLower(response) != "y" {
					fmt.Println("Merge canceled.") //nolint:forbidigo // User-facing output
					return nil
				}
			}

			// Perform the merge
			fmt.Printf("\n%s Merging categories...\n", cli.SpiceIcon) //nolint:forbidigo // User-facing output

			// Update transactions
			if len(txns) > 0 {
				fmt.Printf("  Updating %d transactions... ", len(txns)) //nolint:forbidigo // User-facing output
				if err := store.UpdateTransactionCategoriesByID(ctx, fromCategoryID, toCategoryID); err != nil {
					fmt.Println(cli.ErrorIcon) //nolint:forbidigo // User-facing output
					return fmt.Errorf("failed to update transactions: %w", err)
				}
				fmt.Println(cli.SuccessIcon) //nolint:forbidigo // User-facing output
			}

			// Update vendor rules
			if len(vendors) > 0 {
				fmt.Printf("  Updating %d vendor rules... ", len(vendors)) //nolint:forbidigo // User-facing output
				if err := store.UpdateVendorCategoriesByID(ctx, fromCategoryID, toCategoryID); err != nil {
					fmt.Println(cli.ErrorIcon) //nolint:forbidigo // User-facing output
					return fmt.Errorf("failed to update vendor rules: %w", err)
				}
				fmt.Println(cli.SuccessIcon) //nolint:forbidigo // User-facing output
			}

			// Delete the source category
			fmt.Printf("  Deleting category %q... ", fromCat.Name) //nolint:forbidigo // User-facing output
			if err := store.DeleteCategory(ctx, fromCat.ID); err != nil {
				fmt.Println(cli.ErrorIcon) //nolint:forbidigo // User-facing output
				return fmt.Errorf("failed to delete source category: %w", err)
			}
			fmt.Println(cli.SuccessIcon) //nolint:forbidigo // User-facing output

			fmt.Printf("\n%s Successfully merged %q into %q\n", cli.SuccessStyle.Render("✓"), fromCat.Name, toCat.Name) //nolint:forbidigo // User-facing output
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompt")

	return cmd
}
