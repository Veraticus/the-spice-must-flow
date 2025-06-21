package main

import (
	"github.com/spf13/cobra"
)

func checksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "checks",
		Short: "Manage check transaction patterns",
		Long: `🌶️  Check Pattern Management

Check patterns help automatically categorize check transactions by matching:
  - Check amounts (exact, range, or multiple values)
  - Day of month restrictions
  - Historical usage patterns

These patterns reduce manual categorization for recurring check payments like
rent, cleaning services, or regular bills.`,
	}

	// Add subcommands
	cmd.AddCommand(checksListCmd())
	cmd.AddCommand(checksAddCmd())
	cmd.AddCommand(checksEditCmd())
	cmd.AddCommand(checksDeleteCmd())
	cmd.AddCommand(checksTestCmd())

	return cmd
}
