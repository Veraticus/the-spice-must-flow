package analysis

import (
	"github.com/Veraticus/the-spice-must-flow/internal/cli"
	"github.com/charmbracelet/lipgloss"
)

// Styles contains all styling definitions for analysis report formatting.
type Styles struct {
	// Base styles from CLI package
	Title    lipgloss.Style
	Subtitle lipgloss.Style
	Success  lipgloss.Style
	Warning  lipgloss.Style
	Error    lipgloss.Style
	Info     lipgloss.Style
	Subtle   lipgloss.Style
	Normal   lipgloss.Style

	// Analysis-specific styles
	Box           lipgloss.Style
	Score         lipgloss.Style
	Critical      lipgloss.Style
	High          lipgloss.Style
	Medium        lipgloss.Style
	Low           lipgloss.Style
	CategoryBox   lipgloss.Style
	PatternBox    lipgloss.Style
	InsightBox    lipgloss.Style
	ProgressBar   lipgloss.Style
	ProgressFill  lipgloss.Style
	ProgressEmpty lipgloss.Style
	MenuOption    lipgloss.Style
	MenuKey       lipgloss.Style
}

// NewStyles creates a new Styles instance with default styling.
func NewStyles() *Styles {
	s := &Styles{
		// Import base styles from CLI package
		Title:    cli.TitleStyle,
		Subtitle: cli.SubtitleStyle,
		Success:  cli.SuccessStyle,
		Warning:  cli.WarningStyle,
		Error:    cli.ErrorStyle,
		Info:     cli.InfoStyle,
		Subtle:   cli.SubtleStyle,
		Normal:   lipgloss.NewStyle(),
	}

	// Define analysis-specific styles
	s.Box = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cli.SubtleColor).
		Padding(0, 1)

	s.Score = lipgloss.NewStyle().
		Bold(true).
		Foreground(cli.PrimaryColor)

	// Severity-specific styles
	s.Critical = lipgloss.NewStyle().
		Bold(true).
		Foreground(cli.ErrorColor).
		Background(lipgloss.Color("#2D0000"))

	s.High = lipgloss.NewStyle().
		Bold(true).
		Foreground(cli.WarningColor)

	s.Medium = lipgloss.NewStyle().
		Foreground(cli.InfoColor)

	s.Low = lipgloss.NewStyle().
		Foreground(cli.SubtleColor)

	// Category analysis box
	s.CategoryBox = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cli.InfoColor).
		Padding(0, 1).
		MarginTop(1)

	// Pattern suggestion box
	s.PatternBox = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cli.SuccessColor).
		Padding(0, 1).
		MarginTop(1)

	// Insight box
	s.InsightBox = lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(cli.InfoColor).
		Padding(0, 1).
		MarginTop(1)

	// Progress bar styles
	s.ProgressBar = lipgloss.NewStyle().
		Foreground(cli.SubtleColor)

	s.ProgressFill = lipgloss.NewStyle().
		Foreground(cli.SuccessColor)

	s.ProgressEmpty = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#333333"))

	// Menu styles
	s.MenuOption = lipgloss.NewStyle().
		PaddingLeft(2)

	s.MenuKey = lipgloss.NewStyle().
		Bold(true).
		Foreground(cli.InfoColor)

	return s
}

// WithWidth returns a new Styles instance adjusted for the given terminal width.
func (s *Styles) WithWidth(width int) *Styles {
	// Create a copy
	newStyles := *s

	// Adjust box widths if needed
	if width > 0 && width < 100 {
		boxCopy := s.Box
		newStyles.Box = boxCopy.Width(width - 4)
		categoryBoxCopy := s.CategoryBox
		newStyles.CategoryBox = categoryBoxCopy.Width(width - 4)
		patternBoxCopy := s.PatternBox
		newStyles.PatternBox = patternBoxCopy.Width(width - 4)
		insightBoxCopy := s.InsightBox
		newStyles.InsightBox = insightBoxCopy.Width(width - 4)
	}

	return &newStyles
}

// ForSeverity returns the appropriate style for the given severity level.
func (s *Styles) ForSeverity(severity IssueSeverity) lipgloss.Style {
	switch severity {
	case SeverityCritical:
		return s.Critical
	case SeverityHigh:
		return s.High
	case SeverityMedium:
		return s.Medium
	case SeverityLow:
		return s.Low
	default:
		return s.Normal
	}
}

// ForScore returns the appropriate style for the given score value.
func (s *Styles) ForScore(score float64) lipgloss.Style {
	switch {
	case score >= 0.9:
		return s.Success
	case score >= 0.7:
		return s.Warning
	default:
		return s.Error
	}
}

// RenderProgressBar creates a styled progress bar.
func (s *Styles) RenderProgressBar(progress float64, width int) string {
	if width <= 0 {
		width = 30
	}

	filled := int(float64(width) * progress)
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}

	// Return raw characters without styling to ensure correct width
	bar := repeatChar("█", filled) + repeatChar("░", width-filled)

	return bar
}

// RenderBox renders content in a styled box with optional title.
func (s *Styles) RenderBox(content string, title string, style lipgloss.Style) string {
	if title != "" {
		// Since BorderTitle isn't available in lipgloss v1.1.0,
		// we'll add the title as part of the content
		titleStyled := s.Info.Bold(true).Render(" " + title + " ")
		contentWithTitle := titleStyled + "\n" + content
		return style.Render(contentWithTitle)
	}
	return style.Render(content)
}

// repeatChar repeats a character n times.
func repeatChar(char string, n int) string {
	if n <= 0 {
		return ""
	}
	result := ""
	for i := 0; i < n; i++ {
		result += char
	}
	return result
}
