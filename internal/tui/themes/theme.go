package themes

import "github.com/charmbracelet/lipgloss"

// Theme defines the visual style for the TUI.
type Theme struct {
	ProgressBar   lipgloss.Style
	Selected      lipgloss.Style
	CategoryIcon  lipgloss.Style
	StatusPending lipgloss.Style
	StatusInfo    lipgloss.Style
	StatusError   lipgloss.Style
	StatusWarning lipgloss.Style
	StatusSuccess lipgloss.Style
	ProgressFull  lipgloss.Style
	Italic        lipgloss.Style
	Title         lipgloss.Style
	Subtitle      lipgloss.Style
	Normal        lipgloss.Style
	Bold          lipgloss.Style
	Code          lipgloss.Style
	RoundedBox    lipgloss.Style
	ProgressEmpty lipgloss.Style
	Highlighted   lipgloss.Style
	Box           lipgloss.Style
	BorderedBox   lipgloss.Style
	Secondary     lipgloss.Color
	Primary       lipgloss.Color
	Muted         lipgloss.Color
	Border        lipgloss.Color
	Foreground    lipgloss.Color
	Background    lipgloss.Color
	Info          lipgloss.Color
	Error         lipgloss.Color
	Warning       lipgloss.Color
	Success       lipgloss.Color
}

// Default is the default theme.
var Default = Theme{
	// Colors
	Primary:    lipgloss.Color("#7c3aed"),
	Secondary:  lipgloss.Color("#a78bfa"),
	Success:    lipgloss.Color("#10b981"),
	Warning:    lipgloss.Color("#f59e0b"),
	Error:      lipgloss.Color("#ef4444"),
	Info:       lipgloss.Color("#3b82f6"),
	Background: lipgloss.Color("#1a1a1a"),
	Foreground: lipgloss.Color("#fafafa"),
	Border:     lipgloss.Color("#404040"),
	Muted:      lipgloss.Color("#737373"),

	// Text styles
	Title: lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#fafafa")).
		MarginBottom(1),
	Subtitle: lipgloss.NewStyle().
		Foreground(lipgloss.Color("#a3a3a3")).
		MarginBottom(1),
	Normal: lipgloss.NewStyle().
		Foreground(lipgloss.Color("#fafafa")),
	Bold: lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#fafafa")),
	Italic: lipgloss.NewStyle().
		Italic(true).
		Foreground(lipgloss.Color("#fafafa")),
	Code: lipgloss.NewStyle().
		Background(lipgloss.Color("#262626")).
		Foreground(lipgloss.Color("#e5e5e5")).
		Padding(0, 1),
	Selected: lipgloss.NewStyle().
		Background(lipgloss.Color("#7c3aed")).
		Foreground(lipgloss.Color("#fafafa")).
		Bold(true),
	Highlighted: lipgloss.NewStyle().
		Background(lipgloss.Color("#404040")).
		Foreground(lipgloss.Color("#fafafa")),

	// Component styles
	Box: lipgloss.NewStyle().
		Padding(1, 2),
	BorderedBox: lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#404040")).
		Padding(1, 2),
	RoundedBox: lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#404040")).
		Padding(1, 2),
	ProgressBar: lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7c3aed")),
	ProgressEmpty: lipgloss.NewStyle().
		Foreground(lipgloss.Color("#404040")),
	ProgressFull: lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7c3aed")),

	// Status styles
	StatusSuccess: lipgloss.NewStyle().
		Foreground(lipgloss.Color("#10b981")).
		Bold(true),
	StatusWarning: lipgloss.NewStyle().
		Foreground(lipgloss.Color("#f59e0b")).
		Bold(true),
	StatusError: lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ef4444")).
		Bold(true),
	StatusInfo: lipgloss.NewStyle().
		Foreground(lipgloss.Color("#3b82f6")).
		Bold(true),
	StatusPending: lipgloss.NewStyle().
		Foreground(lipgloss.Color("#737373")).
		Italic(true),

	// Category icon styles
	CategoryIcon: lipgloss.NewStyle().
		Width(3).
		Align(lipgloss.Center),
}

// CatppuccinMocha is the Catppuccin Mocha theme.
var CatppuccinMocha = Theme{
	// Colors
	Primary:    lipgloss.Color("#cba6f7"),
	Secondary:  lipgloss.Color("#f5c2e7"),
	Success:    lipgloss.Color("#a6e3a1"),
	Warning:    lipgloss.Color("#f9e2af"),
	Error:      lipgloss.Color("#f38ba8"),
	Info:       lipgloss.Color("#89dceb"),
	Background: lipgloss.Color("#1e1e2e"),
	Foreground: lipgloss.Color("#cdd6f4"),
	Border:     lipgloss.Color("#45475a"),
	Muted:      lipgloss.Color("#6c7086"),

	// Text styles
	Title: lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#cdd6f4")).
		MarginBottom(1),
	Subtitle: lipgloss.NewStyle().
		Foreground(lipgloss.Color("#a6adc8")).
		MarginBottom(1),
	Normal: lipgloss.NewStyle().
		Foreground(lipgloss.Color("#cdd6f4")),
	Bold: lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#cdd6f4")),
	Italic: lipgloss.NewStyle().
		Italic(true).
		Foreground(lipgloss.Color("#cdd6f4")),
	Code: lipgloss.NewStyle().
		Background(lipgloss.Color("#313244")).
		Foreground(lipgloss.Color("#cdd6f4")).
		Padding(0, 1),
	Selected: lipgloss.NewStyle().
		Background(lipgloss.Color("#cba6f7")).
		Foreground(lipgloss.Color("#1e1e2e")).
		Bold(true),
	Highlighted: lipgloss.NewStyle().
		Background(lipgloss.Color("#45475a")).
		Foreground(lipgloss.Color("#cdd6f4")),

	// Component styles
	Box: lipgloss.NewStyle().
		Padding(1, 2),
	BorderedBox: lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#45475a")).
		Padding(1, 2),
	RoundedBox: lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#45475a")).
		Padding(1, 2),
	ProgressBar: lipgloss.NewStyle().
		Foreground(lipgloss.Color("#cba6f7")),
	ProgressEmpty: lipgloss.NewStyle().
		Foreground(lipgloss.Color("#45475a")),
	ProgressFull: lipgloss.NewStyle().
		Foreground(lipgloss.Color("#cba6f7")),

	// Status styles
	StatusSuccess: lipgloss.NewStyle().
		Foreground(lipgloss.Color("#a6e3a1")).
		Bold(true),
	StatusWarning: lipgloss.NewStyle().
		Foreground(lipgloss.Color("#f9e2af")).
		Bold(true),
	StatusError: lipgloss.NewStyle().
		Foreground(lipgloss.Color("#f38ba8")).
		Bold(true),
	StatusInfo: lipgloss.NewStyle().
		Foreground(lipgloss.Color("#89dceb")).
		Bold(true),
	StatusPending: lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6c7086")).
		Italic(true),

	// Category icon styles
	CategoryIcon: lipgloss.NewStyle().
		Width(3).
		Align(lipgloss.Center),
}

// GetTheme returns a theme by name.
func GetTheme(name string) Theme {
	switch name {
	case "catppuccin-mocha":
		return CatppuccinMocha
	default:
		return Default
	}
}

// CategoryIcons maps categories to emoji icons.
var CategoryIcons = map[string]string{
	"Groceries":      "🥬",
	"Dining":         "🍕",
	"Dining Out":     "🍕",
	"Transportation": "🚗",
	"Entertainment":  "🎬",
	"Shopping":       "🛍️",
	"Healthcare":     "💊",
	"Utilities":      "💡",
	"Home":           "🏠",
	"Home Supplies":  "🏠",
	"Education":      "📚",
	"Travel":         "✈️",
	"Fitness":        "💪",
	"Personal Care":  "💅",
	"Gifts":          "🎁",
	"Subscriptions":  "📱",
	"Insurance":      "🛡️",
	"Taxes":          "📋",
	"Investments":    "📈",
	"Charity":        "❤️",
	"Other":          "📦",
}

// GetCategoryIcon returns an icon for a category.
func GetCategoryIcon(category string) string {
	if icon, ok := CategoryIcons[category]; ok {
		return icon
	}
	return "📦"
}
