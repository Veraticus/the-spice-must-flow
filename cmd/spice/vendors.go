package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

func vendorsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vendors",
		Short: "Manage vendor categorization rules",
		Long:  `View, edit, and manage vendor categorization rules.`,
	}

	// Subcommands
	cmd.AddCommand(vendorsListCmd())
	cmd.AddCommand(vendorsSearchCmd())
	cmd.AddCommand(vendorsEditCmd())
	cmd.AddCommand(vendorsDeleteCmd())

	return cmd
}

func vendorsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all vendor rules",
		Long:  `List all vendor categorization rules with their usage statistics.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			// Get database connection
			db, cleanup, err := getDatabase()
			if err != nil {
				return err
			}
			defer cleanup()

			// Fetch all vendors
			vendors, err := db.GetAllVendors(ctx)
			if err != nil {
				return fmt.Errorf("failed to get vendors: %w", err)
			}

			if len(vendors) == 0 {
				slog.Info("No vendor rules found")
				return nil
			}

			// Display vendors in a table
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "MERCHANT\tCATEGORY\tUSE COUNT\tLAST UPDATED")
			fmt.Fprintln(w, "────────\t────────\t─────────\t────────────")

			for _, vendor := range vendors {
				fmt.Fprintf(w, "%s\t%s\t%d\t%s\n",
					vendor.Name,
					vendor.Category,
					vendor.UseCount,
					vendor.LastUpdated.Format("2006-01-02"))
			}

			return w.Flush()
		},
	}
}

func vendorsSearchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "search <query>",
		Short: "Search vendor rules",
		Long:  `Search for vendor rules by merchant name.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			query := strings.ToLower(args[0])

			// Get database connection
			db, cleanup, err := getDatabase()
			if err != nil {
				return err
			}
			defer cleanup()

			// Fetch all vendors and filter by query
			vendors, err := db.GetAllVendors(ctx)
			if err != nil {
				return fmt.Errorf("failed to get vendors: %w", err)
			}

			// Filter vendors matching the query
			var found bool
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

			for _, vendor := range vendors {
				if strings.Contains(strings.ToLower(vendor.Name), query) {
					if !found {
						// Print header on first match
						slog.Info(fmt.Sprintf("Vendors matching '%s':", query))
						fmt.Fprintln(w, "MERCHANT\tCATEGORY\tUSE COUNT\tLAST UPDATED")
						fmt.Fprintln(w, "────────\t────────\t─────────\t────────────")
						found = true
					}

					fmt.Fprintf(w, "%s\t%s\t%d\t%s\n",
						vendor.Name,
						vendor.Category,
						vendor.UseCount,
						vendor.LastUpdated.Format("2006-01-02"))
				}
			}

			if !found {
				slog.Info("No vendors found matching query", "query", query)
				return nil
			}

			return w.Flush()
		},
	}
}

func vendorsEditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit <merchant>",
		Short: "Edit a vendor rule",
		Long:  `Edit the category for a vendor rule.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			merchant := args[0]

			// Get database connection
			db, cleanup, err := getDatabase()
			if err != nil {
				return err
			}
			defer cleanup()

			// Check if vendor exists
			vendor, err := db.GetVendor(ctx, merchant)
			if err != nil {
				return fmt.Errorf("vendor '%s' not found", merchant)
			}

			// Show current vendor info
			slog.Info("Current vendor rule:")
			fmt.Printf("  Merchant: %s\n", vendor.Name)
			fmt.Printf("  Category: %s\n", vendor.Category)
			fmt.Printf("  Use Count: %d\n", vendor.UseCount)
			fmt.Printf("  Last Updated: %s\n\n", vendor.LastUpdated.Format("2006-01-02"))

			// Get new category from flag or prompt
			newCategory, _ := cmd.Flags().GetString("category")
			if newCategory == "" {
				fmt.Print("Enter new category (or press Enter to cancel): ")
				fmt.Scanln(&newCategory)
				if newCategory == "" {
					slog.Info("Edit canceled")
					return nil
				}
			}

			// Update vendor
			vendor.Category = newCategory
			vendor.LastUpdated = time.Now()

			if err := db.SaveVendor(ctx, vendor); err != nil {
				return fmt.Errorf("failed to update vendor: %w", err)
			}

			slog.Info("Vendor rule updated successfully",
				"merchant", merchant,
				"new_category", newCategory)

			return nil
		},
	}

	cmd.Flags().StringP("category", "c", "", "New category for the vendor")
	return cmd
}

func vendorsDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <merchant>",
		Short: "Delete a vendor rule",
		Long:  `Delete a vendor categorization rule.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			merchant := args[0]

			// Get database connection
			db, cleanup, err := getDatabase()
			if err != nil {
				return err
			}
			defer cleanup()

			// Check if vendor exists
			vendor, err := db.GetVendor(ctx, merchant)
			if err != nil {
				return fmt.Errorf("vendor '%s' not found", merchant)
			}

			// Show vendor info
			fmt.Printf("About to delete vendor rule:\n")
			fmt.Printf("  Merchant: %s\n", vendor.Name)
			fmt.Printf("  Category: %s\n", vendor.Category)
			fmt.Printf("  Use Count: %d\n\n", vendor.UseCount)

			// Get confirmation unless --force flag is set
			force, _ := cmd.Flags().GetBool("force")
			if !force {
				fmt.Print("Are you sure you want to delete this vendor rule? (y/N): ")
				var response string
				fmt.Scanln(&response)
				if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
					slog.Info("Deletion canceled")
					return nil
				}
			}

			// Delete vendor
			if err := db.DeleteVendor(ctx, merchant); err != nil {
				return fmt.Errorf("failed to delete vendor: %w", err)
			}

			slog.Info("Vendor rule deleted successfully", "merchant", merchant)
			return nil
		},
	}

	cmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	return cmd
}
