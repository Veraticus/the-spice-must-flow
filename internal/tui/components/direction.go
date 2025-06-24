package components

import (
	"fmt"
	"strings"

	"github.com/Veraticus/the-spice-must-flow/internal/engine"
	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/tui/themes"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// DirectionConfirmModel handles transaction direction confirmation.
type DirectionConfirmModel struct {
	theme    themes.Theme
	selected model.TransactionDirection
	pending  engine.PendingDirection
	cursor   int
	width    int
	height   int
	complete bool
}

// NewDirectionConfirmModel creates a new direction confirmation model.
func NewDirectionConfirmModel(pending engine.PendingDirection, theme themes.Theme) DirectionConfirmModel {
	return DirectionConfirmModel{
		pending:  pending,
		selected: pending.SuggestedDirection,
		theme:    theme,
	}
}

// Update handles messages.
func (m DirectionConfirmModel) Update(msg tea.Msg) (DirectionConfirmModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			m.cursor = (m.cursor + 1) % 3

		case "k", "up":
			m.cursor = (m.cursor + 2) % 3

		case "enter", "a":
			// Accept selected direction
			m.complete = true

		case "1":
			m.cursor = 0
			m.complete = true

		case "2":
			m.cursor = 1
			m.complete = true

		case "3":
			m.cursor = 2
			m.complete = true

		case "esc":
			// Use suggested direction
			m.selected = m.pending.SuggestedDirection
			m.complete = true
		}

		// Update selected based on cursor
		switch m.cursor {
		case 0:
			m.selected = model.DirectionExpense
		case 1:
			m.selected = model.DirectionIncome
		case 2:
			m.selected = model.DirectionTransfer
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	return m, nil
}

// View renders the direction confirmation interface.
func (m DirectionConfirmModel) View() string {
	title := m.theme.Title.Render("Confirm Transaction Direction")

	// Merchant info
	merchantInfo := m.renderMerchantInfo()

	// AI suggestion
	suggestion := m.renderSuggestion()

	// Direction options
	options := m.renderOptions()

	// Sample transaction
	sample := m.renderSampleTransaction()

	// Help
	help := lipgloss.NewStyle().Foreground(m.theme.Muted).Render("[↑↓] Navigate | [1-3] Quick select | [Enter] Confirm | [Esc] Use suggestion")

	sections := []string{
		title,
		"",
		merchantInfo,
		"",
		suggestion,
		"",
		options,
		"",
		sample,
		"",
		help,
	}

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		m.theme.BorderedBox.Render(content),
	)
}

// renderMerchantInfo renders merchant information.
func (m DirectionConfirmModel) renderMerchantInfo() string {
	info := fmt.Sprintf("Merchant: %s\nTransactions: %d",
		m.theme.Bold.Render(m.pending.MerchantName),
		m.pending.TransactionCount,
	)

	return m.theme.Box.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(m.theme.Border).
		Render(info)
}

// renderSuggestion renders the AI suggestion.
func (m DirectionConfirmModel) renderSuggestion() string {
	confidence := int(m.pending.Confidence * 100)

	suggestion := fmt.Sprintf("AI Suggestion: %s %s (%d%% confidence)",
		m.getDirectionIcon(m.pending.SuggestedDirection),
		m.theme.StatusInfo.Render(string(m.pending.SuggestedDirection)),
		confidence,
	)

	reasoning := m.theme.Italic.Render("\"" + m.pending.Reasoning + "\"")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		suggestion,
		reasoning,
	)
}

// renderOptions renders direction options.
func (m DirectionConfirmModel) renderOptions() string {
	options := []struct {
		direction model.TransactionDirection
		desc      string
		icon      string
	}{
		{model.DirectionExpense, "Money leaving your accounts", "💸"},
		{model.DirectionIncome, "Money entering your accounts", "💰"},
		{model.DirectionTransfer, "Moving between your accounts", "🔄"},
	}

	// Pre-allocate for 3 options
	lines := make([]string, 0, len(options))
	for i, opt := range options {
		prefix := fmt.Sprintf("[%d] ", i+1)
		if i == m.cursor {
			prefix = lipgloss.NewStyle().Foreground(m.theme.Primary).Render("> ")
		}

		line := fmt.Sprintf("%s%s %s - %s",
			prefix,
			opt.icon,
			opt.direction,
			lipgloss.NewStyle().Foreground(m.theme.Muted).Render(opt.desc),
		)

		if i == m.cursor {
			line = m.theme.Selected.Render(line)
		}

		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// renderSampleTransaction renders a sample transaction.
func (m DirectionConfirmModel) renderSampleTransaction() string {
	t := m.pending.SampleTransaction

	amount := fmt.Sprintf("$%.2f", t.Amount)
	switch m.selected {
	case model.DirectionIncome:
		amount = "+" + amount
	case model.DirectionExpense:
		amount = "-" + amount
	}

	sample := fmt.Sprintf("Sample: %s on %s for %s",
		t.Name,
		t.Date.Format("Jan 2, 2006"),
		m.theme.Bold.Render(amount),
	)

	return m.theme.Box.
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Muted).
		Render(sample)
}

// Helper methods

// IsComplete returns whether the confirmation is complete.
func (m DirectionConfirmModel) IsComplete() bool {
	return m.complete
}

// GetResult returns the selected direction.
func (m DirectionConfirmModel) GetResult() model.TransactionDirection {
	return m.selected
}

// Resize updates the component size.
func (m *DirectionConfirmModel) Resize(width, height int) {
	m.width = width
	m.height = height
}

func (m DirectionConfirmModel) getDirectionIcon(direction model.TransactionDirection) string {
	switch direction {
	case model.DirectionExpense:
		return "💸"
	case model.DirectionIncome:
		return "💰"
	case model.DirectionTransfer:
		return "🔄"
	default:
		return "❓"
	}
}
