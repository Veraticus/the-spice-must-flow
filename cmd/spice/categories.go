package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/charmbracelet/lipgloss"
	"github.com/joshsymonds/the-spice-must-flow/internal/cli"
	"github.com/joshsymonds/the-spice-must-flow/internal/model"
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

	return cmd
}

func listCategoriesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all categories",
		Long:  `Display all active expense categories with their descriptions.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			// Initialize storage with auto-migration
			store, err := initStorage(ctx)
			if err != nil {
				return err
			}
			defer store.Close()

			// Get all categories
			categories, err := store.GetCategories(ctx)
			if err != nil {
				return fmt.Errorf("failed to get categories: %w", err)
			}

			if len(categories) == 0 {
				fmt.Println(cli.InfoStyle.Render("No categories found. Use 'spice categories add' to create one."))
				return nil
			}

			// Create table writer
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			defer w.Flush()

			// Header
			headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
			fmt.Fprintf(w, "%s\t%s\t%s\n", 
				headerStyle.Render("ID"),
				headerStyle.Render("Name"),
				headerStyle.Render("Description"))
			fmt.Fprintf(w, "%s\t%s\t%s\n", 
				strings.Repeat("-", 4),
				strings.Repeat("-", 20),
				strings.Repeat("-", 50))

			// List categories
			for _, cat := range categories {
				desc := cat.Description
				if desc == "" {
					desc = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("(no description)")
				}
				fmt.Fprintf(w, "%d\t%s\t%s\n", cat.ID, cat.Name, desc)
			}

			return nil
		},
	}
}

func addCategoryCmd() *cobra.Command {
	var (
		categoryDescription string
		skipDescription     bool
	)

	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a new category",
		Long:  `Create a new expense category. An AI-generated description will be created automatically.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			categoryName := args[0]

			// Initialize storage with auto-migration
			store, err := initStorage(ctx)
			if err != nil {
				return err
			}
			defer store.Close()

			// Check if category already exists
			existing, err := store.GetCategoryByName(ctx, categoryName)
			if err != nil {
				return fmt.Errorf("failed to check existing category: %w", err)
			}
			if existing != nil {
				return fmt.Errorf("category %q already exists", categoryName)
			}

			// Initialize LLM for description generation
			var description string
			if !skipDescription {
				classifier, err := createLLMClient()
				if err != nil {
					return fmt.Errorf("failed to initialize LLM: %w", err)
				}
				
				// Classifiers from llm package implement Close via embedded Classifier interface
				if closer, ok := classifier.(interface{ Close() error }); ok {
					defer closer.Close()
				}
				
				description, err = classifier.GenerateCategoryDescription(ctx, categoryName)
				if err != nil {
					return fmt.Errorf("failed to generate category description: %w", err)
				}
			}

			// Allow user to edit description if provided
			if categoryDescription != "" {
				description = categoryDescription
			}

			// Create category
			category, err := store.CreateCategory(ctx, categoryName, description)
			if err != nil {
				return fmt.Errorf("failed to create category: %w", err)
			}

			fmt.Println(cli.SuccessStyle.Render(fmt.Sprintf("✓ Created category %q (ID: %d)", category.Name, category.ID)))
			if description != "" {
				fmt.Printf("  Description: %s\n", description)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&categoryDescription, "description", "", "Category description (auto-generated if not provided)")
	cmd.Flags().BoolVar(&skipDescription, "no-description", false, "Skip AI description generation")

	return cmd
}

func updateCategoryCmd() *cobra.Command {
	var (
		categoryName        string
		categoryDescription string
	)

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a category",
		Long:  `Update the name or description of an existing category.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			
			// Parse category ID
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid category ID: %w", err)
			}

			if categoryName == "" && categoryDescription == "" {
				return fmt.Errorf("must specify --name or --description to update")
			}

			// Initialize storage with auto-migration
			store, err := initStorage(ctx)
			if err != nil {
				return err
			}
			defer store.Close()

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
			if categoryDescription != "" {
				description = categoryDescription
			}

			// Update category
			if err := store.UpdateCategory(ctx, id, name, description); err != nil {
				return fmt.Errorf("failed to update category: %w", err)
			}

			fmt.Println(cli.SuccessStyle.Render(fmt.Sprintf("✓ Updated category %d", id)))
			return nil
		},
	}

	cmd.Flags().StringVar(&categoryName, "name", "", "New category name")
	cmd.Flags().StringVar(&categoryDescription, "description", "", "New category description")

	return cmd
}

func deleteCategoryCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a category",
		Long:  `Delete a category. This will fail if any transactions are using the category.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			
			// Parse category ID
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid category ID: %w", err)
			}

			// Initialize storage with auto-migration
			store, err := initStorage(ctx)
			if err != nil {
				return err
			}
			defer store.Close()

			// Confirm deletion
			if !force {
				fmt.Printf("Are you sure you want to delete category %d? (y/N): ", id)
				var response string
				fmt.Scanln(&response)
				if strings.ToLower(response) != "y" {
					fmt.Println("Deletion cancelled.")
					return nil
				}
			}

			// Delete category
			if err := store.DeleteCategory(ctx, id); err != nil {
				return fmt.Errorf("failed to delete category: %w", err)
			}

			fmt.Println(cli.SuccessStyle.Render(fmt.Sprintf("✓ Deleted category %d", id)))
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Skip confirmation prompt")

	return cmd
}