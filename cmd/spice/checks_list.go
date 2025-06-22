package main

import (
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/cli"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

func checksListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all check patterns",
		Long: `Display all check patterns with their matching criteria and usage statistics.

Patterns are sorted by use count (most used first) to show the most helpful
patterns at the top.`,
		RunE: runChecksList,
	}

	return cmd
}

func runChecksList(cmd *cobra.Command, _ []string) error {
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

	// Get all active patterns
	patterns, err := storage.GetActiveCheckPatterns(ctx)
	if err != nil {
		return fmt.Errorf("failed to get check patterns: %w", err)
	}

	// Sort by use count descending
	sort.Slice(patterns, func(i, j int) bool {
		return patterns[i].UseCount > patterns[j].UseCount
	})

	// Display patterns
	if len(patterns) == 0 {
		fmt.Println(cli.InfoStyle.Render("No check patterns found. Use 'spice checks add' to create one.")) //nolint:forbidigo // User-facing output
		return nil
	}

	fmt.Println(cli.FormatTitle("🌶️  Check Patterns")) //nolint:forbidigo // User-facing output
	fmt.Println()                                      //nolint:forbidigo // User-facing output

	// Create table writer
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer func() {
		if flushErr := w.Flush(); flushErr != nil {
			slog.Error("failed to flush table writer", "error", flushErr)
		}
	}()

	// Header
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
		headerStyle.Render("ID"),
		headerStyle.Render("Pattern Name"),
		headerStyle.Render("Amount(s)"),
		headerStyle.Render("Category"),
		headerStyle.Render("Uses"),
		headerStyle.Render("Last Used")); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Separator
	if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
		strings.Repeat("─", 4),
		strings.Repeat("─", 20),
		strings.Repeat("─", 15),
		strings.Repeat("─", 15),
		strings.Repeat("─", 5),
		strings.Repeat("─", 12)); err != nil {
		return fmt.Errorf("failed to write separator: %w", err)
	}

	// Data rows
	for _, pattern := range patterns {
		amountStr := formatPatternAmounts(pattern)
		lastUsed := formatLastUsed(pattern.UpdatedAt, pattern.UseCount)

		if _, err := fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%d\t%s\n",
			pattern.ID,
			pattern.PatternName,
			amountStr,
			pattern.Category,
			pattern.UseCount,
			lastUsed); err != nil {
			return fmt.Errorf("failed to write pattern row: %w", err)
		}
	}

	return nil
}

func formatLastUsed(updatedAt time.Time, useCount int) string {
	if useCount == 0 {
		return "Never"
	}
	return updatedAt.Format("2006-01-02")
}
