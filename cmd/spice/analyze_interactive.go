package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Veraticus/the-spice-must-flow/internal/analysis"
	"github.com/Veraticus/the-spice-must-flow/internal/cli"
)

// showInteractiveAnalysis displays an interactive menu for the analysis report.
func showInteractiveAnalysis(ctx context.Context, reader io.Reader, writer io.Writer, report *analysis.Report, engine *analysis.Engine, dryRun bool) error {
	if reader == nil {
		reader = os.Stdin
	}
	if writer == nil {
		writer = os.Stdout
	}

	br := bufio.NewReader(reader)
	formatter := analysis.NewCLIFormatter()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Clear screen and show menu
		if err := clearScreen(writer); err != nil {
			return err
		}

		if _, err := fmt.Fprintln(writer, formatter.FormatInteractive(report)); err != nil {
			return fmt.Errorf("failed to display menu: %w", err)
		}

		// Show additional prompt
		if _, err := fmt.Fprintln(writer); err != nil {
			return err
		}

		// Handle auto-apply suggestion if there are actionable issues
		if report.HasActionableIssues() {
			if _, err := fmt.Fprintln(writer, cli.FormatInfo("To apply recommended fixes, run:")); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(writer, "  spice analyze --auto-apply --session-id %s\n\n", report.SessionID); err != nil {
				return err
			}
		}

		if _, err := fmt.Fprint(writer, cli.FormatPrompt("Select option (1-6, q to quit): ")); err != nil {
			return err
		}

		// Read user input
		input, err := br.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		choice := strings.TrimSpace(strings.ToLower(input))

		// Handle quit
		if choice == "q" || choice == "quit" {
			return nil
		}

		// Handle menu options
		switch choice {
		case "1":
			if err := showIssues(ctx, br, writer, formatter, report); err != nil {
				return err
			}
		case "2":
			if err := showCategoryAnalysis(ctx, br, writer, report); err != nil {
				return err
			}
		case "3":
			if err := showSuggestedPatterns(ctx, br, writer, report); err != nil {
				return err
			}
		case "4":
			if err := showInsights(ctx, br, writer, report); err != nil {
				return err
			}
		case "5":
			if err := showApplyFixes(ctx, br, writer, report, engine, dryRun); err != nil {
				return err
			}
		case "6":
			if err := exportReportInteractive(ctx, br, writer, report); err != nil {
				return err
			}
		default:
			if _, err := fmt.Fprintln(writer, cli.FormatError("Invalid option. Please select 1-6 or q to quit.")); err != nil {
				return err
			}
			if err := waitForEnter(br, writer); err != nil {
				return err
			}
		}
	}
}

// clearScreen clears the terminal screen.
func clearScreen(writer io.Writer) error {
	// Use ANSI escape codes for clearing screen
	if _, err := fmt.Fprint(writer, "\033[H\033[2J"); err != nil {
		// If ANSI codes fail, just add some newlines
		if _, err := fmt.Fprintln(writer, strings.Repeat("\n", 50)); err != nil {
			return err
		}
	}
	return nil
}

// waitForEnter waits for the user to press Enter.
func waitForEnter(reader *bufio.Reader, writer io.Writer) error {
	if _, err := fmt.Fprintln(writer); err != nil {
		return err
	}
	if _, err := fmt.Fprint(writer, cli.SubtleStyle.Render("Press Enter to continue...")); err != nil {
		return err
	}
	_, err := reader.ReadString('\n')
	return err
}

// showIssues displays issues grouped by severity.
func showIssues(ctx context.Context, reader *bufio.Reader, writer io.Writer, formatter *analysis.CLIFormatter, report *analysis.Report) error {
	if err := clearScreen(writer); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(writer, cli.TitleStyle.Render("📋 Issues by Severity")); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(writer, strings.Repeat("─", 60)); err != nil {
		return err
	}

	if len(report.Issues) == 0 {
		if _, err := fmt.Fprintln(writer, cli.SuccessStyle.Render("\n✅ No issues found!")); err != nil {
			return err
		}
		return waitForEnter(reader, writer)
	}

	// Group issues by severity
	issuesBySeverity := make(map[analysis.IssueSeverity][]analysis.Issue)
	for _, issue := range report.Issues {
		issuesBySeverity[issue.Severity] = append(issuesBySeverity[issue.Severity], issue)
	}

	// Display in severity order
	severities := []analysis.IssueSeverity{
		analysis.SeverityCritical,
		analysis.SeverityHigh,
		analysis.SeverityMedium,
		analysis.SeverityLow,
	}

	for _, severity := range severities {
		issues, exists := issuesBySeverity[severity]
		if !exists || len(issues) == 0 {
			continue
		}

		// Severity header
		severityStyle := formatter.GetSeverityStyle(severity)
		icon := formatter.GetSeverityIcon(severity)
		header := fmt.Sprintf("\n%s %s Issues (%d)", icon, severity, len(issues))
		if _, err := fmt.Fprintln(writer, severityStyle.Bold(true).Render(header)); err != nil {
			return err
		}

		// Show up to 5 issues per severity
		displayCount := len(issues)
		if displayCount > 5 {
			displayCount = 5
		}

		for i := 0; i < displayCount; i++ {
			issue := issues[i]
			if _, err := fmt.Fprintln(writer); err != nil {
				return err
			}

			// Issue description
			desc := fmt.Sprintf("  • %s", issue.Description)
			if _, err := fmt.Fprintln(writer, desc); err != nil {
				return err
			}

			// Confidence and affected count
			meta := fmt.Sprintf("    Confidence: %.0f%% | Affected: %d transaction(s)",
				issue.Confidence*100, issue.AffectedCount)
			if _, err := fmt.Fprintln(writer, cli.SubtleStyle.Render(meta)); err != nil {
				return err
			}

			// Fix available indicator
			if issue.Fix != nil {
				fix := "    ✓ Fix available"
				if _, err := fmt.Fprintln(writer, cli.SuccessStyle.Render(fix)); err != nil {
					return err
				}
			}
		}

		if len(issues) > displayCount {
			more := fmt.Sprintf("\n  ... and %d more %s issues", len(issues)-displayCount, severity)
			if _, err := fmt.Fprintln(writer, cli.SubtleStyle.Render(more)); err != nil {
				return err
			}
		}
	}

	return waitForEnter(reader, writer)
}

// showCategoryAnalysis displays category statistics.
func showCategoryAnalysis(ctx context.Context, reader *bufio.Reader, writer io.Writer, report *analysis.Report) error {
	if err := clearScreen(writer); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(writer, cli.TitleStyle.Render("📁 Category Analysis")); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(writer, strings.Repeat("─", 60)); err != nil {
		return err
	}

	if len(report.CategorySummary) == 0 {
		if _, err := fmt.Fprintln(writer, cli.InfoStyle.Render("\nNo category data available.")); err != nil {
			return err
		}
		return waitForEnter(reader, writer)
	}

	// Convert map to slice for sorting
	var categories []struct {
		Name string
		Stat analysis.CategoryStat
	}
	for name, stat := range report.CategorySummary {
		categories = append(categories, struct {
			Name string
			Stat analysis.CategoryStat
		}{Name: name, Stat: stat})
	}

	// Sort by transaction count
	for i := 0; i < len(categories)-1; i++ {
		for j := i + 1; j < len(categories); j++ {
			if categories[j].Stat.TransactionCount > categories[i].Stat.TransactionCount {
				categories[i], categories[j] = categories[j], categories[i]
			}
		}
	}

	// Display top categories
	displayCount := len(categories)
	if displayCount > 15 {
		displayCount = 15
	}

	if _, err := fmt.Fprintln(writer); err != nil {
		return err
	}

	for i := 0; i < displayCount; i++ {
		cat := categories[i]
		line := fmt.Sprintf("%-30s %4d transactions", cat.Name, cat.Stat.TransactionCount)

		// Highlight categories with issues
		if cat.Stat.Issues > 0 {
			line += cli.WarningStyle.Render(fmt.Sprintf(" (%d issues)", cat.Stat.Issues))
		}

		if _, err := fmt.Fprintln(writer, line); err != nil {
			return err
		}
	}

	if len(categories) > displayCount {
		more := fmt.Sprintf("\n... and %d more categories", len(categories)-displayCount)
		if _, err := fmt.Fprintln(writer, cli.SubtleStyle.Render(more)); err != nil {
			return err
		}
	}

	// Show summary statistics
	if _, err := fmt.Fprintln(writer); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(writer, cli.SubtitleStyle.Render("Summary:")); err != nil {
		return err
	}

	totalTransactions := 0
	categoriesWithIssues := 0
	for _, cat := range categories {
		totalTransactions += cat.Stat.TransactionCount
		if cat.Stat.Issues > 0 {
			categoriesWithIssues++
		}
	}

	summary := fmt.Sprintf("  Total categories: %d\n", len(categories))
	summary += fmt.Sprintf("  Total transactions: %d\n", totalTransactions)
	summary += fmt.Sprintf("  Categories with issues: %d", categoriesWithIssues)
	if _, err := fmt.Fprintln(writer, summary); err != nil {
		return err
	}

	return waitForEnter(reader, writer)
}

// showSuggestedPatterns displays suggested check patterns.
func showSuggestedPatterns(ctx context.Context, reader *bufio.Reader, writer io.Writer, report *analysis.Report) error {
	if err := clearScreen(writer); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(writer, cli.TitleStyle.Render("🔍 Suggested Patterns")); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(writer, strings.Repeat("─", 60)); err != nil {
		return err
	}

	if len(report.SuggestedPatterns) == 0 {
		if _, err := fmt.Fprintln(writer, cli.InfoStyle.Render("\nNo pattern suggestions available.")); err != nil {
			return err
		}
		return waitForEnter(reader, writer)
	}

	if _, err := fmt.Fprintln(writer); err != nil {
		return err
	}

	for i, pattern := range report.SuggestedPatterns {
		if i > 0 {
			if _, err := fmt.Fprintln(writer); err != nil {
				return err
			}
		}

		// Pattern name and confidence
		header := fmt.Sprintf("Pattern: %s", cli.InfoStyle.Bold(true).Render(pattern.Name))
		if _, err := fmt.Fprintln(writer, header); err != nil {
			return err
		}

		conf := fmt.Sprintf("  Confidence: %.0f%% | Matches: %d",
			pattern.Confidence*100, pattern.MatchCount)
		if _, err := fmt.Fprintln(writer, cli.SubtleStyle.Render(conf)); err != nil {
			return err
		}

		// Impact description
		if pattern.Impact != "" {
			impact := fmt.Sprintf("  Impact: %s", pattern.Impact)
			if _, err := fmt.Fprintln(writer, impact); err != nil {
				return err
			}
		}

		// Pattern rules
		if pattern.Pattern.AmountMin != nil || pattern.Pattern.AmountMax != nil {
			var amountRule string
			if pattern.Pattern.AmountMin != nil && pattern.Pattern.AmountMax != nil {
				amountRule = fmt.Sprintf("  Amount: $%.2f - $%.2f", *pattern.Pattern.AmountMin, *pattern.Pattern.AmountMax)
			} else if pattern.Pattern.AmountMin != nil {
				amountRule = fmt.Sprintf("  Amount: >= $%.2f", *pattern.Pattern.AmountMin)
			} else {
				amountRule = fmt.Sprintf("  Amount: <= $%.2f", *pattern.Pattern.AmountMax)
			}
			if _, err := fmt.Fprintln(writer, cli.SubtleStyle.Render(amountRule)); err != nil {
				return err
			}
		}

		if pattern.Pattern.MerchantPattern != "" {
			desc := fmt.Sprintf("  Pattern: %s", pattern.Pattern.MerchantPattern)
			if _, err := fmt.Fprintln(writer, cli.SubtleStyle.Render(desc)); err != nil {
				return err
			}
		}
	}

	return waitForEnter(reader, writer)
}

// showInsights displays analysis insights.
func showInsights(ctx context.Context, reader *bufio.Reader, writer io.Writer, report *analysis.Report) error {
	if err := clearScreen(writer); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(writer, cli.TitleStyle.Render("💡 Insights")); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(writer, strings.Repeat("─", 60)); err != nil {
		return err
	}

	if len(report.Insights) == 0 {
		if _, err := fmt.Fprintln(writer, cli.InfoStyle.Render("\nNo insights available.")); err != nil {
			return err
		}
		return waitForEnter(reader, writer)
	}

	if _, err := fmt.Fprintln(writer); err != nil {
		return err
	}

	for i, insight := range report.Insights {
		line := fmt.Sprintf("%d. %s", i+1, insight)
		if _, err := fmt.Fprintln(writer, line); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(writer); err != nil {
			return err
		}
	}

	return waitForEnter(reader, writer)
}

// showApplyFixes displays fix options.
func showApplyFixes(ctx context.Context, reader *bufio.Reader, writer io.Writer, report *analysis.Report, engine *analysis.Engine, dryRun bool) error {
	if err := clearScreen(writer); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(writer, cli.TitleStyle.Render("🔧 Apply Fixes")); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(writer, strings.Repeat("─", 60)); err != nil {
		return err
	}

	actionableCount := 0
	for _, issue := range report.Issues {
		if issue.Fix != nil {
			actionableCount++
		}
	}

	if actionableCount == 0 {
		if _, err := fmt.Fprintln(writer, cli.InfoStyle.Render("\nNo fixes available.")); err != nil {
			return err
		}
		return waitForEnter(reader, writer)
	}

	if _, err := fmt.Fprintln(writer); err != nil {
		return err
	}

	// Show summary of available fixes
	summary := fmt.Sprintf("Found %d actionable issues that can be automatically fixed.", actionableCount)
	if _, err := fmt.Fprintln(writer, summary); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(writer); err != nil {
		return err
	}

	// Group fixes by type
	fixesByType := make(map[string]int)
	for _, issue := range report.Issues {
		if issue.Fix != nil {
			fixesByType[string(issue.Type)]++
		}
	}

	if _, err := fmt.Fprintln(writer, "Fixes by type:"); err != nil {
		return err
	}
	for fixType, count := range fixesByType {
		line := fmt.Sprintf("  • %s: %d", fixType, count)
		if _, err := fmt.Fprintln(writer, line); err != nil {
			return err
		}
	}

	// Ask for confirmation
	if _, err := fmt.Fprintln(writer); err != nil {
		return err
	}

	if dryRun {
		if _, err := fmt.Fprintln(writer, cli.InfoStyle.Render("DRY RUN MODE - No changes will be made")); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(writer); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprint(writer, cli.BoldStyle.Render("Apply these fixes? (y/N): ")); err != nil {
		return err
	}

	answer, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer != "y" && answer != "yes" {
		if _, err := fmt.Fprintln(writer, "\nFix application canceled."); err != nil {
			return err
		}
		return waitForEnter(reader, writer)
	}

	// Apply fixes
	if _, err := fmt.Fprintln(writer, "\n"+cli.InfoStyle.Render("Applying fixes...")); err != nil {
		return err
	}

	if engine != nil && !dryRun {
		if err := engine.ApplyFixesFromReport(ctx, report); err != nil {
			if _, err := fmt.Fprintln(writer, cli.ErrorStyle.Render(fmt.Sprintf("\n❌ Error applying fixes: %v", err))); err != nil {
				return err
			}
		} else {
			if _, err := fmt.Fprintln(writer, cli.SuccessStyle.Render("\n✅ Fixes applied successfully!")); err != nil {
				return err
			}
		}
	} else if dryRun {
		if _, err := fmt.Fprintln(writer, cli.InfoStyle.Render("\n(Dry run - no changes made)")); err != nil {
			return err
		}
	}

	return waitForEnter(reader, writer)
}

// exportReportInteractive exports the report to a file.
func exportReportInteractive(ctx context.Context, reader *bufio.Reader, writer io.Writer, report *analysis.Report) error {
	if err := clearScreen(writer); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(writer, cli.TitleStyle.Render("📤 Export Report")); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(writer, strings.Repeat("─", 60)); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(writer); err != nil {
		return err
	}

	// Generate filename
	filename := fmt.Sprintf("spice-analysis-%s.json", report.GeneratedAt.Format("20060102-150405"))

	// Ask for confirmation
	prompt := fmt.Sprintf("Export report to %s? [Y/n]: ", filename)
	if _, err := fmt.Fprint(writer, cli.FormatPrompt(prompt)); err != nil {
		return err
	}

	input, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	choice := strings.TrimSpace(strings.ToLower(input))
	if choice != "" && choice != "y" && choice != "yes" {
		if _, err := fmt.Fprintln(writer, "Export canceled."); err != nil {
			return err
		}
		return waitForEnter(reader, writer)
	}

	// Export the report
	file, err := os.Create(filename)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to create file: %v", err)
		if _, err := fmt.Fprintln(writer, cli.ErrorStyle.Render(errMsg)); err != nil {
			return err
		}
		return waitForEnter(reader, writer)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		errMsg := fmt.Sprintf("Failed to write report: %v", err)
		if _, err := fmt.Fprintln(writer, cli.ErrorStyle.Render(errMsg)); err != nil {
			return err
		}
		return waitForEnter(reader, writer)
	}

	success := fmt.Sprintf("✓ Report exported to %s", filename)
	if _, err := fmt.Fprintln(writer, cli.SuccessStyle.Render(success)); err != nil {
		return err
	}

	return waitForEnter(reader, writer)
}
