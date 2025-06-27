package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
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
	cmd.AddCommand(vendorsCreateCmd())
	cmd.AddCommand(vendorsEditCmd())
	cmd.AddCommand(vendorsDeleteCmd())
	cmd.AddCommand(vendorsDeleteAllCmd())

	return cmd
}

func vendorsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List vendor rules",
		Long:  `List vendor categorization rules with their usage statistics and source.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			// Get database connection
			db, cleanup, err := getDatabase()
			if err != nil {
				return err
			}
			defer cleanup()

			// Get source filter
			sourceFilter, _ := cmd.Flags().GetString("source")

			// Fetch vendors based on filter
			var vendors []model.Vendor
			if sourceFilter != "" && sourceFilter != "all" {
				// Map filter to VendorSource
				var source model.VendorSource
				switch sourceFilter {
				case "manual":
					source = model.SourceManual
				case "auto":
					source = model.SourceAuto
				case "confirmed":
					source = model.SourceAutoConfirmed
				default:
					return fmt.Errorf("invalid source filter: %s (valid options: manual, auto, confirmed, all)", sourceFilter)
				}
				vendors, err = db.GetVendorsBySource(ctx, source)
			} else {
				vendors, err = db.GetAllVendors(ctx)
			}

			if err != nil {
				return fmt.Errorf("failed to get vendors: %w", err)
			}

			if len(vendors) == 0 {
				slog.Info("No vendor rules found")
				return nil
			}

			// Display vendors in a table
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "MERCHANT\tCATEGORY\tSOURCE\tTYPE\tUSE COUNT\tLAST UPDATED")
			_, _ = fmt.Fprintln(w, "────────\t────────\t──────\t────\t─────────\t────────────")

			for _, vendor := range vendors {
				vendorType := "exact"
				if vendor.IsRegex {
					vendorType = "regex"
				}
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%s\n",
					vendor.Name,
					vendor.Category,
					vendor.Source,
					vendorType,
					vendor.UseCount,
					vendor.LastUpdated.Format("2006-01-02"))
			}

			return w.Flush()
		},
	}

	cmd.Flags().StringP("source", "s", "", "Filter by source (manual|auto|confirmed|all)")
	return cmd
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
						_, _ = fmt.Fprintln(w, "MERCHANT\tCATEGORY\tSOURCE\tTYPE\tUSE COUNT\tLAST UPDATED")
						_, _ = fmt.Fprintln(w, "────────\t────────\t──────\t────\t─────────\t────────────")
						found = true
					}

					vendorType := "exact"
					if vendor.IsRegex {
						vendorType = "regex"
					}
					_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%s\n",
						vendor.Name,
						vendor.Category,
						vendor.Source,
						vendorType,
						vendor.UseCount,
						vendor.LastUpdated.Format("2006-01-02"))
				}
			}

			if !found {
				slog.Info(fmt.Sprintf("No vendor rules found matching '%s'. Try a different search term", query))
				return nil
			}

			return w.Flush()
		},
	}
}

func vendorsCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <merchant>",
		Short: "Create a vendor rule",
		Long:  `Create a new vendor categorization rule manually.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			merchant := args[0]

			// Get category from flag
			category, _ := cmd.Flags().GetString("category")
			if category == "" {
				return fmt.Errorf("category is required (use --category flag)")
			}

			// Get regex flag
			isRegex, _ := cmd.Flags().GetBool("regex")

			// Get database connection
			db, cleanup, err := getDatabase()
			if err != nil {
				return err
			}
			defer cleanup()

			// Check if vendor already exists
			existing, _ := db.GetVendor(ctx, merchant)
			if existing != nil {
				return fmt.Errorf("vendor rule for '%s' already exists with category '%s'", merchant, existing.Category)
			}

			// Create new vendor with manual source
			vendor := &model.Vendor{
				Name:        merchant,
				Category:    category,
				Source:      model.SourceManual,
				UseCount:    0,
				LastUpdated: time.Now(),
				IsRegex:     isRegex,
			}

			if err := db.SaveVendor(ctx, vendor); err != nil {
				return fmt.Errorf("failed to create vendor: %w", err)
			}

			ruleType := "exact match"
			if isRegex {
				ruleType = "regex"
			}
			slog.Info("✓ Vendor rule created successfully",
				"merchant", merchant,
				"category", category,
				"source", "MANUAL",
				"type", ruleType)

			return nil
		},
	}

	cmd.Flags().StringP("category", "c", "", "Category for the vendor (required)")
	cmd.Flags().BoolP("regex", "r", false, "Treat the merchant name as a regular expression pattern")
	if err := cmd.MarkFlagRequired("category"); err != nil {
		slog.Error("failed to mark flag as required", "error", err)
	}
	return cmd
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
			slog.Info("  Merchant", "name", vendor.Name)
			slog.Info("  Category", "category", vendor.Category)
			slog.Info("  Use Count", "count", vendor.UseCount)
			slog.Info("  Last Updated", "date", vendor.LastUpdated.Format("2006-01-02"))

			// Get new category from flag or prompt
			newCategory, _ := cmd.Flags().GetString("category")
			if newCategory == "" {
				slog.Info("Enter new category (or press Enter to cancel): ")
				_, _ = fmt.Scanln(&newCategory)
				if newCategory == "" {
					slog.Info("Edit canceled")
					return nil
				}
			}

			// Update vendor
			vendor.Category = newCategory
			vendor.LastUpdated = time.Now()
			// If vendor was auto-created, mark it as confirmed since user edited it
			if vendor.Source == model.SourceAuto {
				vendor.Source = model.SourceAutoConfirmed
			}

			if err := db.SaveVendor(ctx, vendor); err != nil {
				return fmt.Errorf("failed to update vendor: %w", err)
			}

			slog.Info("✓ Vendor rule updated successfully",
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
			if _, err := fmt.Fprintf(os.Stdout, "About to delete vendor rule:\n"); err != nil {
				slog.Error("failed to write output", "error", err)
			}
			if _, err := fmt.Fprintf(os.Stdout, "  Merchant: %s\n", vendor.Name); err != nil {
				slog.Error("failed to write output", "error", err)
			}
			if _, err := fmt.Fprintf(os.Stdout, "  Category: %s\n", vendor.Category); err != nil {
				slog.Error("failed to write output", "error", err)
			}
			if _, err := fmt.Fprintf(os.Stdout, "  Use Count: %d\n\n", vendor.UseCount); err != nil {
				slog.Error("failed to write output", "error", err)
			}

			// Get confirmation unless --force flag is set
			force, _ := cmd.Flags().GetBool("force")
			if !force {
				slog.Info("Are you sure you want to delete this vendor rule? (y/N): ")
				var response string
				_, _ = fmt.Scanln(&response)
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

func vendorsDeleteAllCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete-all",
		Short: "Delete multiple vendor rules",
		Long:  `Delete vendor rules based on source filter.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			// Get source filter
			sourceFilter, _ := cmd.Flags().GetString("source")
			if sourceFilter == "" {
				return fmt.Errorf("source filter is required (use --source flag)")
			}

			// Map filter to VendorSource
			var source model.VendorSource
			var sourceLabel string
			switch sourceFilter {
			case "manual":
				source = model.SourceManual
				sourceLabel = "manually created"
			case "auto":
				source = model.SourceAuto
				sourceLabel = "auto-created"
			default:
				return fmt.Errorf("invalid source filter: %s (valid options: manual, auto)", sourceFilter)
			}

			// Get database connection
			db, cleanup, err := getDatabase()
			if err != nil {
				return err
			}
			defer cleanup()

			// Get vendors to be deleted
			vendors, err := db.GetVendorsBySource(ctx, source)
			if err != nil {
				return fmt.Errorf("failed to get vendors: %w", err)
			}

			if len(vendors) == 0 {
				slog.Info(fmt.Sprintf("No %s vendor rules found", sourceLabel))
				return nil
			}

			// Show summary
			slog.Info(fmt.Sprintf("About to delete %d %s vendor rules", len(vendors), sourceLabel))

			// Get confirmation unless --force flag is set
			force, _ := cmd.Flags().GetBool("force")
			if !force {
				slog.Info(fmt.Sprintf("Are you sure you want to delete all %s vendor rules? (y/N): ", sourceLabel))
				var response string
				_, _ = fmt.Scanln(&response)
				if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
					slog.Info("Deletion canceled")
					return nil
				}
			}

			// Delete vendors
			if err := db.DeleteVendorsBySource(ctx, source); err != nil {
				return fmt.Errorf("failed to delete vendors: %w", err)
			}

			slog.Info(fmt.Sprintf("✓ Successfully deleted %d %s vendor rules", len(vendors), sourceLabel))
			return nil
		},
	}

	cmd.Flags().StringP("source", "s", "", "Source of vendors to delete (manual|auto)")
	cmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	if err := cmd.MarkFlagRequired("source"); err != nil {
		slog.Error("failed to mark flag as required", "error", err)
	}
	return cmd
}
