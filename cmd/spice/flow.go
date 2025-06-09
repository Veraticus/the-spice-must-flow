package main

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/joshsymonds/the-spice-must-flow/internal/cli"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func flowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "flow",
		Short: "View spending flow reports",
		Long: `Analyze and visualize your financial flow with category breakdowns.
		
This command generates reports showing where your money flows,
with options to export to Google Sheets.`,
		RunE: runFlow,
	}

	// Flags
	cmd.Flags().IntP("year", "y", time.Now().Year(), "Year to analyze")
	cmd.Flags().StringP("month", "m", "", "Specific month to analyze (format: 2024-01)")
	cmd.Flags().Bool("export", false, "Export to Google Sheets")
	cmd.Flags().String("format", "table", "Output format (table, json, csv)")

	// Bind to viper
	_ = viper.BindPFlag("flow.year", cmd.Flags().Lookup("year"))
	_ = viper.BindPFlag("flow.month", cmd.Flags().Lookup("month"))
	_ = viper.BindPFlag("flow.export", cmd.Flags().Lookup("export"))
	_ = viper.BindPFlag("flow.format", cmd.Flags().Lookup("format"))

	return cmd
}

func runFlow(_ *cobra.Command, _ []string) error {
	year := viper.GetInt("flow.year")
	month := viper.GetString("flow.month")
	export := viper.GetBool("flow.export")
	format := viper.GetString("flow.format")

	slog.Info(cli.FormatTitle("Analyzing your financial flow..."))

	// Display period
	period := fmt.Sprintf("%d", year)
	if month != "" {
		period = month
	}

	// Build report content
	content := `Total outflow: $0.00
Transactions: 0
Categories: 0

No data available yet.
Run 'spice classify' to categorize transactions first.`

	// Display styled box
	slog.Info(cli.RenderBox(fmt.Sprintf("%s Financial Flow", period), content))

	if export {
		slog.Warn(cli.FormatWarning("Export to Google Sheets not yet implemented"))
	}

	if format != "table" {
		slog.Warn(cli.FormatWarning(fmt.Sprintf("Output format '%s' not yet implemented", format)))
	}

	return nil
}
