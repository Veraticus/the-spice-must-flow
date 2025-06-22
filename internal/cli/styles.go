// Package cli provides styled terminal output using lipgloss.
package cli

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// PrimaryColor is the main theme color (spicy red).
	PrimaryColor = lipgloss.Color("#FF6B6B")
	// SuccessColor indicates successful operations.
	SuccessColor = lipgloss.Color("#4ECDC4") // Teal
	// WarningColor indicates warnings or caution messages.
	WarningColor = lipgloss.Color("#FFE66D") // Yellow
	// ErrorColor indicates errors or failure messages.
	ErrorColor = lipgloss.Color("#FF6B6B") // Red
	// InfoColor indicates informational messages.
	InfoColor = lipgloss.Color("#95E1D3") // Light teal
	// SubtleColor indicates less prominent UI elements.
	SubtleColor = lipgloss.Color("#666666") // Gray

	// TitleStyle is used for section titles.
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(PrimaryColor).
			MarginBottom(1)

	// SubtitleStyle is used for secondary headings.
	SubtitleStyle = lipgloss.NewStyle().
			Foreground(SubtleColor).
			MarginBottom(1)

	// SuccessStyle formats success messages.
	SuccessStyle = lipgloss.NewStyle().
			Foreground(SuccessColor)

	// WarningStyle formats warning messages.
	WarningStyle = lipgloss.NewStyle().
			Foreground(WarningColor)

	// ErrorStyle formats error messages.
	ErrorStyle = lipgloss.NewStyle().
			Foreground(ErrorColor)

	// InfoStyle formats informational messages.
	InfoStyle = lipgloss.NewStyle().
			Foreground(InfoColor)

	// SubtleStyle formats less prominent text.
	SubtleStyle = lipgloss.NewStyle().
			Foreground(SubtleColor)

	// BoldStyle makes text bold.
	BoldStyle = lipgloss.NewStyle().
			Bold(true)

		// BoxStyle is used for bordered content boxes.
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#333")).
			Padding(1, 2)

		// ProgressStyle is used for progress indicators.
	ProgressStyle = lipgloss.NewStyle().
			Foreground(PrimaryColor)

		// TableHeaderStyle is used for table headers.
	TableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				BorderStyle(lipgloss.NormalBorder()).
				BorderBottom(true).
				BorderForeground(lipgloss.Color("#333"))

	// TableCellStyle formats table cells with appropriate padding.
	TableCellStyle = lipgloss.NewStyle().
			PaddingRight(2)

		// PromptStyle is used for user prompts.
	PromptStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(PrimaryColor)

		// IconStyle is used for icons in the UI.
	IconStyle = lipgloss.NewStyle().
			Bold(true).
			MarginRight(1)
)

// Icons.
const (
	SuccessIcon = "✓"
	ErrorIcon   = "✗"
	WarningIcon = "⚠️"
	InfoIcon    = "ℹ️"
	SpiceIcon   = "🌶️"
	RobotIcon   = "🤖"
	ChartIcon   = "📊"
	FolderIcon  = "🗄️"
	CheckIcon   = "✅"
)

// FormatSuccess formats a success message with icon.
func FormatSuccess(message string) string {
	return SuccessStyle.Render(SuccessIcon + " " + message)
}

// FormatError formats an error message with icon.
func FormatError(message string) string {
	return ErrorStyle.Render(ErrorIcon + " " + message)
}

// FormatWarning formats a warning message with icon.
func FormatWarning(message string) string {
	return WarningStyle.Render(WarningIcon + " " + message)
}

// FormatInfo formats an info message with icon.
func FormatInfo(message string) string {
	return InfoStyle.Render(InfoIcon + " " + message)
}

// FormatTitle formats a title with the spice icon.
func FormatTitle(title string) string {
	return TitleStyle.Render(SpiceIcon + " " + title)
}

// FormatPrompt formats a prompt message.
func FormatPrompt(prompt string) string {
	return PromptStyle.Render(prompt + " → ")
}

// RenderBox renders content in a styled box.
func RenderBox(title, content string) string {
	boxTitle := TitleStyle.
		UnsetMargins().
		Render(title)

	boxContent := lipgloss.JoinVertical(
		lipgloss.Left,
		boxTitle,
		content,
	)

	return BoxStyle.Render(boxContent)
}

// StyleTitle formats text as a title.
func StyleTitle(text string) string {
	return TitleStyle.Render(text)
}

// StyleSuccess formats text as success message.
func StyleSuccess(text string) string {
	return SuccessStyle.Render(text)
}

// StyleWarning formats text as warning message.
func StyleWarning(text string) string {
	return WarningStyle.Render(text)
}

// StyleError formats text as error message.
func StyleError(text string) string {
	return ErrorStyle.Render(text)
}

// StyleInfo formats text as info message.
func StyleInfo(text string) string {
	return InfoStyle.Render(text)
}
