package analysis

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// CLIFormatter implements ReportFormatter for terminal display.
type CLIFormatter struct {
	styles *Styles
}

// NewCLIFormatter creates a new CLI formatter with default styles.
func NewCLIFormatter() *CLIFormatter {
	return &CLIFormatter{
		styles: NewStyles(),
	}
}

// FormatSummary creates a high-level summary of the analysis report.
func (f *CLIFormatter) FormatSummary(report *Report) string {
	if report == nil {
		return f.styles.Error.Render("No report available")
	}

	var sections []string

	// Header
	header := f.formatHeader(report)
	sections = append(sections, header)

	// Coherence Score
	scoreSection := f.formatCoherenceScore(report.CoherenceScore)
	sections = append(sections, scoreSection)

	// Issues Summary
	issuesSection := f.formatIssuesSummary(report.Issues)
	sections = append(sections, issuesSection)

	// Category Summary
	if len(report.CategorySummary) > 0 {
		categorySection := f.formatCategorySummary(report.CategorySummary)
		sections = append(sections, categorySection)
	}

	// Insights
	if len(report.Insights) > 0 {
		insightsSection := f.formatInsights(report.Insights)
		sections = append(sections, insightsSection)
	}

	// Suggested Patterns
	if len(report.SuggestedPatterns) > 0 {
		patternsSection := f.formatSuggestedPatternsSummary(report.SuggestedPatterns)
		sections = append(sections, patternsSection)
	}

	return strings.Join(sections, "\n\n")
}

// FormatIssue formats a single issue for detailed display.
func (f *CLIFormatter) FormatIssue(issue Issue) string {
	var parts []string

	// Issue header with severity indicator
	header := f.formatIssueHeader(issue)
	parts = append(parts, header)

	// Description
	desc := f.styles.Normal.Render(issue.Description)
	parts = append(parts, desc)

	// Metadata
	meta := f.formatIssueMetadata(issue)
	parts = append(parts, meta)

	// Category changes if applicable
	if issue.Type == IssueTypeMiscategorized && issue.CurrentCategory != nil && issue.SuggestedCategory != nil {
		change := f.formatCategoryChange(*issue.CurrentCategory, *issue.SuggestedCategory)
		parts = append(parts, change)
	}

	// Fix information if available
	if issue.Fix != nil {
		fix := f.formatFix(issue.Fix)
		parts = append(parts, fix)
	}

	// Affected transactions summary
	if len(issue.TransactionIDs) > 0 {
		txnInfo := f.formatAffectedTransactions(issue)
		parts = append(parts, txnInfo)
	}

	return strings.Join(parts, "\n")
}

// FormatInteractive creates an interactive menu for report navigation.
func (f *CLIFormatter) FormatInteractive(report *Report) string {
	if report == nil {
		return f.styles.Error.Render("No report available")
	}

	var sections []string

	// Header with navigation hint
	header := f.styles.Title.Render("📊 Analysis Report - Interactive View")
	navHint := f.styles.Subtle.Render("Use arrow keys to navigate, Enter to select, q to quit")
	sections = append(sections, header, navHint, "")

	// Quick stats
	stats := f.formatQuickStats(report)
	sections = append(sections, stats, "")

	// Menu options
	menu := f.formatInteractiveMenu(report)
	sections = append(sections, menu)

	return strings.Join(sections, "\n")
}

// formatHeader creates the report header section.
func (f *CLIFormatter) formatHeader(report *Report) string {
	title := f.styles.Title.Render("📊 Transaction Analysis Report")

	period := fmt.Sprintf("Period: %s to %s",
		report.PeriodStart.Format("Jan 2, 2006"),
		report.PeriodEnd.Format("Jan 2, 2006"))
	periodStyled := f.styles.Subtitle.Render(period)

	generated := fmt.Sprintf("Generated: %s",
		report.GeneratedAt.Format(time.RFC3339))
	generatedStyled := f.styles.Subtle.Render(generated)

	return fmt.Sprintf("%s\n%s\n%s", title, periodStyled, generatedStyled)
}

// formatCoherenceScore creates a visual representation of the coherence score.
func (f *CLIFormatter) formatCoherenceScore(score float64) string {
	percentage := score * 100

	// Determine color based on score
	var scoreStyle lipgloss.Style
	var emoji string
	switch {
	case score >= 0.9:
		scoreStyle = f.styles.Success
		emoji = "🎯"
	case score >= 0.7:
		scoreStyle = f.styles.Warning
		emoji = "⚠️"
	default:
		scoreStyle = f.styles.Error
		emoji = "❌"
	}

	// Create progress bar
	barWidth := 30
	filledWidth := int(float64(barWidth) * score)
	bar := strings.Repeat("█", filledWidth) + strings.Repeat("░", barWidth-filledWidth)

	scoreText := fmt.Sprintf("%s Coherence Score: %.1f%%", emoji, percentage)
	scoreDisplay := scoreStyle.Render(scoreText)
	barDisplay := scoreStyle.Render(bar)

	return fmt.Sprintf("%s\n%s", scoreDisplay, barDisplay)
}

// formatIssuesSummary creates a summary of issues by severity.
func (f *CLIFormatter) formatIssuesSummary(issues []Issue) string {
	// Count issues by severity
	counts := make(map[IssueSeverity]int)
	for _, issue := range issues {
		counts[issue.Severity]++
	}

	title := f.styles.Subtitle.Render("Issues Found:")

	// Create severity breakdown
	var lines []string
	severities := []IssueSeverity{SeverityCritical, SeverityHigh, SeverityMedium, SeverityLow}

	for _, severity := range severities {
		count := counts[severity]
		if count > 0 {
			icon := f.getSeverityIcon(severity)
			style := f.getSeverityStyle(severity)
			line := style.Render(fmt.Sprintf("%s %s: %d", icon, severity, count))
			lines = append(lines, line)
		}
	}

	if len(lines) == 0 {
		return title + "\n" + f.styles.Success.Render("✅ No issues found!")
	}

	return title + "\n" + strings.Join(lines, "\n")
}

// formatCategorySummary creates a table of category statistics.
func (f *CLIFormatter) formatCategorySummary(categories map[string]CategoryStat) string {
	title := f.styles.Subtitle.Render("Category Summary:")

	// Sort categories by transaction count
	stats := make([]CategoryStat, 0, len(categories))
	for _, stat := range categories {
		stats = append(stats, stat)
	}
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].TransactionCount > stats[j].TransactionCount
	})

	// Limit to top 10 categories
	limit := 10
	if len(stats) < limit {
		limit = len(stats)
	}

	// Create manual table
	// Define column widths
	nameWidth := 20
	txnWidth := 12
	totalWidth := 12
	consistencyWidth := 12
	issuesWidth := 8

	// Create header
	headerStyle := f.styles.Subtle.Bold(true)
	header := fmt.Sprintf("%-*s %-*s %-*s %-*s %-*s",
		nameWidth, "Category",
		txnWidth, "Transactions",
		totalWidth, "Total",
		consistencyWidth, "Consistency",
		issuesWidth, "Issues")
	headerLine := headerStyle.Render(header)

	// Create separator
	separator := f.styles.Subtle.Render(strings.Repeat("─", len(header)))

	// Create rows
	rows := []string{headerLine, separator}

	for i := 0; i < limit; i++ {
		stat := stats[i]
		consistency := fmt.Sprintf("%.0f%%", stat.Consistency*100)

		// Color consistency based on value
		var consistencyStyle lipgloss.Style
		switch {
		case stat.Consistency >= 0.9:
			consistencyStyle = f.styles.Success
		case stat.Consistency >= 0.7:
			consistencyStyle = f.styles.Warning
		default:
			consistencyStyle = f.styles.Error
		}

		// Truncate category name if too long
		categoryName := stat.CategoryName
		if len(categoryName) > nameWidth-1 {
			categoryName = categoryName[:nameWidth-4] + "..."
		}

		row := fmt.Sprintf("%-*s %-*s %-*s %-*s %-*s",
			nameWidth, categoryName,
			txnWidth, fmt.Sprintf("%d", stat.TransactionCount),
			totalWidth, fmt.Sprintf("$%.2f", stat.TotalAmount),
			consistencyWidth, consistencyStyle.Render(consistency),
			issuesWidth, fmt.Sprintf("%d", stat.Issues))

		rows = append(rows, row)
	}

	tableStr := strings.Join(rows, "\n")

	if len(stats) > limit {
		more := f.styles.Subtle.Render(fmt.Sprintf("... and %d more categories", len(stats)-limit))
		tableStr += "\n" + more
	}

	return title + "\n" + tableStr
}

// formatInsights formats the insights list.
func (f *CLIFormatter) formatInsights(insights []string) string {
	title := f.styles.Subtitle.Render("💡 Key Insights:")

	formatted := make([]string, 0, len(insights))
	for _, insight := range insights {
		// Add bullet point and wrap
		bullet := f.styles.Info.Render("•")
		text := f.styles.Normal.Render(insight)
		formatted = append(formatted, fmt.Sprintf("%s %s", bullet, text))
	}

	return title + "\n" + strings.Join(formatted, "\n")
}

// formatSuggestedPatternsSummary creates a summary of suggested patterns.
func (f *CLIFormatter) formatSuggestedPatternsSummary(patterns []SuggestedPattern) string {
	title := f.styles.Subtitle.Render("🔧 Suggested Pattern Rules:")

	// Sort by match count
	sort.Slice(patterns, func(i, j int) bool {
		return patterns[i].MatchCount > patterns[j].MatchCount
	})

	var lines []string
	limit := 5
	if len(patterns) < limit {
		limit = len(patterns)
	}

	for i := 0; i < limit; i++ {
		pattern := patterns[i]
		confidence := fmt.Sprintf("%.0f%%", pattern.Confidence*100)

		line := fmt.Sprintf("• %s - %d matches (%s confidence)",
			f.styles.Info.Render(pattern.Name),
			pattern.MatchCount,
			f.styles.Subtle.Render(confidence))
		lines = append(lines, line)

		// Add impact description
		impact := f.styles.Subtle.Render("  " + pattern.Impact)
		lines = append(lines, impact)
	}

	if len(patterns) > limit {
		more := f.styles.Subtle.Render(fmt.Sprintf("\n... and %d more patterns", len(patterns)-limit))
		lines = append(lines, more)
	}

	return title + "\n" + strings.Join(lines, "\n")
}

// formatIssueHeader creates the header for an issue display.
func (f *CLIFormatter) formatIssueHeader(issue Issue) string {
	icon := f.getSeverityIcon(issue.Severity)
	style := f.getSeverityStyle(issue.Severity)

	header := fmt.Sprintf("%s %s Issue [%s]", icon, issue.Severity, issue.Type)
	return style.Bold(true).Render(header)
}

// formatIssueMetadata formats issue metadata.
func (f *CLIFormatter) formatIssueMetadata(issue Issue) string {
	var parts []string

	// Affected count
	affected := fmt.Sprintf("Affected: %d transaction(s)", issue.AffectedCount)
	parts = append(parts, f.styles.Subtle.Render(affected))

	// Confidence
	confidence := fmt.Sprintf("Confidence: %.0f%%", issue.Confidence*100)
	parts = append(parts, f.styles.Subtle.Render(confidence))

	// Issue ID
	id := fmt.Sprintf("ID: %s", issue.ID)
	parts = append(parts, f.styles.Subtle.Render(id))

	return strings.Join(parts, " | ")
}

// formatCategoryChange formats a category change suggestion.
func (f *CLIFormatter) formatCategoryChange(current, suggested string) string {
	arrow := f.styles.Info.Render("→")
	currentStyled := f.styles.Error.Render(current)
	suggestedStyled := f.styles.Success.Render(suggested)

	return fmt.Sprintf("Category: %s %s %s", currentStyled, arrow, suggestedStyled)
}

// formatFix formats fix information.
func (f *CLIFormatter) formatFix(fix *Fix) string {
	title := f.styles.Info.Render("🔧 Suggested Fix:")
	desc := f.styles.Normal.Render(fix.Description)

	status := "Not applied"
	statusStyle := f.styles.Subtle
	if fix.Applied {
		status = fmt.Sprintf("Applied at %s", fix.AppliedAt.Format(time.RFC3339))
		statusStyle = f.styles.Success
	}

	return fmt.Sprintf("%s\n%s\n%s", title, desc, statusStyle.Render(status))
}

// formatAffectedTransactions formats affected transaction information.
func (f *CLIFormatter) formatAffectedTransactions(issue Issue) string {
	title := f.styles.Subtle.Render("Affected Transactions:")

	// Show first few transaction IDs
	limit := 3
	shown := issue.TransactionIDs
	if len(shown) > limit {
		shown = shown[:limit]
	}

	// Pre-allocate with extra capacity for "and X more" line
	lines := make([]string, 0, len(shown)+1)
	for _, id := range shown {
		lines = append(lines, f.styles.Subtle.Render("  • "+id))
	}

	if len(issue.TransactionIDs) > limit {
		more := f.styles.Subtle.Render(fmt.Sprintf("  ... and %d more", len(issue.TransactionIDs)-limit))
		lines = append(lines, more)
	}

	return title + "\n" + strings.Join(lines, "\n")
}

// formatQuickStats formats quick statistics for interactive view.
func (f *CLIFormatter) formatQuickStats(report *Report) string {
	// Create a grid of stats
	stats := []struct {
		style lipgloss.Style
		label string
		value string
	}{
		{
			label: "Coherence",
			value: fmt.Sprintf("%.0f%%", report.CoherenceScore*100),
			style: f.getScoreStyle(report.CoherenceScore),
		},
		{
			label: "Issues",
			value: fmt.Sprintf("%d", len(report.Issues)),
			style: f.styles.Info,
		},
		{
			label: "Patterns",
			value: fmt.Sprintf("%d", len(report.SuggestedPatterns)),
			style: f.styles.Info,
		},
		{
			label: "Categories",
			value: fmt.Sprintf("%d", len(report.CategorySummary)),
			style: f.styles.Info,
		},
	}

	parts := make([]string, 0, len(stats))
	for _, stat := range stats {
		label := f.styles.Subtle.Render(stat.label + ":")
		value := stat.style.Render(stat.value)
		parts = append(parts, fmt.Sprintf("%s %s", label, value))
	}

	box := f.styles.Box.Render(strings.Join(parts, "  │  "))
	return box
}

// formatInteractiveMenu formats the interactive menu options.
func (f *CLIFormatter) formatInteractiveMenu(report *Report) string {
	title := f.styles.Subtitle.Render("📋 Menu Options:")

	options := []struct {
		key   string
		label string
		count int
	}{
		{"1", "View Issues by Severity", len(report.Issues)},
		{"2", "View Category Analysis", len(report.CategorySummary)},
		{"3", "View Suggested Patterns", len(report.SuggestedPatterns)},
		{"4", "View Insights", len(report.Insights)},
		{"5", "Apply Fixes", countActionableIssues(report)},
		{"6", "Export Report", 0},
	}

	lines := make([]string, 0, len(options))
	for _, opt := range options {
		key := f.styles.Info.Render(fmt.Sprintf("[%s]", opt.key))
		label := opt.label

		// Add count if relevant
		if opt.count > 0 {
			count := f.styles.Subtle.Render(fmt.Sprintf("(%d)", opt.count))
			label = fmt.Sprintf("%s %s", label, count)
		}

		lines = append(lines, fmt.Sprintf("%s %s", key, label))
	}

	return title + "\n" + strings.Join(lines, "\n")
}

// getSeverityIcon returns the appropriate icon for a severity level.
func (f *CLIFormatter) getSeverityIcon(severity IssueSeverity) string {
	switch severity {
	case SeverityCritical:
		return "🚨"
	case SeverityHigh:
		return "⚠️"
	case SeverityMedium:
		return "⚡"
	case SeverityLow:
		return "💡"
	default:
		return "•"
	}
}

// getSeverityStyle returns the appropriate style for a severity level.
func (f *CLIFormatter) getSeverityStyle(severity IssueSeverity) lipgloss.Style {
	switch severity {
	case SeverityCritical:
		return f.styles.Error
	case SeverityHigh:
		return f.styles.Warning
	case SeverityMedium:
		return f.styles.Info
	case SeverityLow:
		return f.styles.Subtle
	default:
		return f.styles.Normal
	}
}

// getScoreStyle returns the appropriate style for a score value.
func (f *CLIFormatter) getScoreStyle(score float64) lipgloss.Style {
	switch {
	case score >= 0.9:
		return f.styles.Success
	case score >= 0.7:
		return f.styles.Warning
	default:
		return f.styles.Error
	}
}

// countActionableIssues counts issues that have fixes available.
func countActionableIssues(report *Report) int {
	count := 0
	for _, issue := range report.Issues {
		if issue.Fix != nil && !issue.Fix.Applied {
			count++
		}
	}
	return count
}
