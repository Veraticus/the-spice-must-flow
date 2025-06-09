package main

import (
	"log/slog"
	"strings"

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
		RunE: func(_ *cobra.Command, _ []string) error {
			table := `┌─────────────────────────┬──────────────────────┬───────────┬──────────┐
│ Merchant                │ Category             │ Used      │ Updated  │
├─────────────────────────┼──────────────────────┼───────────┼──────────┤
│ (No vendors yet)        │                      │           │          │
└─────────────────────────┴──────────────────────┴───────────┴──────────┘`
			slog.Info(table)
			slog.Warn("Vendor management not yet implemented")
			return nil
		},
	}
}

func vendorsSearchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "search <query>",
		Short: "Search vendor rules",
		Long:  `Search for vendor rules by merchant name.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			query := strings.ToLower(args[0])
			slog.Info("Searching for vendors", "query", query)
			slog.Warn("Vendor search not yet implemented")
			return nil
		},
	}
}

func vendorsEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit <merchant>",
		Short: "Edit a vendor rule",
		Long:  `Edit the category for a vendor rule.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			merchant := args[0]
			slog.Info("Editing vendor rule", "merchant", merchant)
			slog.Warn("Vendor editing not yet implemented")
			return nil
		},
	}
}

func vendorsDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <merchant>",
		Short: "Delete a vendor rule",
		Long:  `Delete a vendor categorization rule.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			merchant := args[0]
			slog.Info("Deleting vendor rule", "merchant", merchant)
			slog.Warn("Vendor deletion not yet implemented")
			return nil
		},
	}
}
