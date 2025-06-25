package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/tui/themes"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TransactionDetailModel represents the transaction detail view.
type TransactionDetailModel struct {
	classification *model.Classification
	theme          themes.Theme
	transaction    model.Transaction
	width          int
	height         int
	focused        bool
}

type detailKeyMap struct {
	Classify   key.Binding
	AIClassify key.Binding
	Back       key.Binding
	Help       key.Binding
}

var detailKeys = detailKeyMap{
	Classify: key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("c", "classify manually"),
	),
	AIClassify: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "send to AI classifier"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back to list"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
}

// NewTransactionDetailModel creates a new transaction detail model.
func NewTransactionDetailModel(theme themes.Theme) TransactionDetailModel {
	return TransactionDetailModel{
		theme:   theme,
		focused: true,
	}
}

// Init initializes the model.
func (m TransactionDetailModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m TransactionDetailModel) Update(msg tea.Msg) (TransactionDetailModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case TransactionDetailsRequestMsg:
		m.transaction = msg.Transaction
		m.classification = msg.Classification
		return m, nil

	case tea.KeyMsg:
		if !m.focused {
			return m, nil
		}

		switch {
		case key.Matches(msg, detailKeys.Back):
			return m, func() tea.Msg {
				return BackToListMsg{}
			}

		case key.Matches(msg, detailKeys.Classify):
			return m, func() tea.Msg {
				return ManualClassificationRequestMsg{
					Transaction: m.transaction,
				}
			}

		case key.Matches(msg, detailKeys.AIClassify):
			return m, func() tea.Msg {
				return AIClassificationRequestMsg{
					Transaction: m.transaction,
				}
			}

		case key.Matches(msg, detailKeys.Help):
			return m, func() tea.Msg {
				return ShowHelpMsg{}
			}
		}
	}

	return m, nil
}

// View renders the transaction detail view.
func (m TransactionDetailModel) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	titleStyle := m.theme.Title.
		Width(m.width).
		Align(lipgloss.Center)

	sectionStyle := m.theme.Box.
		Width(m.width - 4).
		MarginLeft(2)

	labelStyle := m.theme.Bold.
		Width(20).
		Align(lipgloss.Right)

	valueStyle := m.theme.Normal

	// Build the detail view
	var sections []string

	// Title
	title := titleStyle.Render("Transaction Details")
	sections = append(sections, title)

	// Transaction info
	infoSection := []string{
		lipgloss.JoinHorizontal(lipgloss.Top,
			labelStyle.Render("Date: "),
			valueStyle.Render(m.transaction.Date.Format("Jan 2, 2006")),
		),
		lipgloss.JoinHorizontal(lipgloss.Top,
			labelStyle.Render("Amount: "),
			valueStyle.Render(fmt.Sprintf("$%.2f", m.transaction.Amount)),
		),
		lipgloss.JoinHorizontal(lipgloss.Top,
			labelStyle.Render("Account: "),
			valueStyle.Render(m.transaction.AccountID),
		),
		lipgloss.JoinHorizontal(lipgloss.Top,
			labelStyle.Render("Type: "),
			valueStyle.Render(m.transaction.Type),
		),
	}

	// Add merchant name if available
	if m.transaction.MerchantName != "" {
		infoSection = append(infoSection,
			lipgloss.JoinHorizontal(lipgloss.Top,
				labelStyle.Render("Merchant: "),
				valueStyle.Render(m.transaction.MerchantName),
			),
		)
	}

	// Add check number if available
	if m.transaction.CheckNumber != "" {
		infoSection = append(infoSection,
			lipgloss.JoinHorizontal(lipgloss.Top,
				labelStyle.Render("Check #: "),
				valueStyle.Render(m.transaction.CheckNumber),
			),
		)
	}

	sections = append(sections, sectionStyle.Render(strings.Join(infoSection, "\n")))

	// Classification info
	if m.classification != nil {
		classSection := []string{
			m.theme.Subtitle.Render("Classification"),
			lipgloss.JoinHorizontal(lipgloss.Top,
				labelStyle.Render("Category: "),
				valueStyle.Render(m.classification.Category),
			),
		}

		if m.classification.Confidence > 0 {
			classSection = append(classSection,
				lipgloss.JoinHorizontal(lipgloss.Top,
					labelStyle.Render("Confidence: "),
					valueStyle.Render(fmt.Sprintf("%.0f%%", m.classification.Confidence*100)),
				),
			)
		}

		if m.classification.ClassifiedAt.After(time.Time{}) {
			classSection = append(classSection,
				lipgloss.JoinHorizontal(lipgloss.Top,
					labelStyle.Render("Classified: "),
					valueStyle.Render(m.classification.ClassifiedAt.Format("Jan 2, 2006 3:04 PM")),
				),
			)
		}

		sections = append(sections, sectionStyle.Render(strings.Join(classSection, "\n")))
	} else {
		sections = append(sections, sectionStyle.Render(
			m.theme.Subtitle.Render("Not Classified"),
		))
	}

	// Actions
	actionsText := []string{
		"",
		m.theme.Subtitle.Render("Actions"),
	}

	if m.classification == nil {
		actionsText = append(actionsText,
			"  Press 'c' to classify manually",
			"  Press 'a' to send to AI classifier",
		)
	} else {
		actionsText = append(actionsText,
			"  Press 'c' to reclassify manually",
		)
	}

	actionsText = append(actionsText,
		"  Press 'esc' to return to list",
		"  Press 'q' to quit",
		"  Press '?' for help",
	)

	sections = append(sections, sectionStyle.Render(strings.Join(actionsText, "\n")))

	// Join all sections with spacing
	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	// Center vertically if there's space
	if m.height > strings.Count(content, "\n")+5 {
		verticalPadding := (m.height - strings.Count(content, "\n") - 2) / 2
		content = strings.Repeat("\n", verticalPadding) + content
	}

	return content
}

// SetFocused sets the focus state.
func (m TransactionDetailModel) SetFocused(focused bool) TransactionDetailModel {
	m.focused = focused
	return m
}

// Focused returns the focus state.
func (m TransactionDetailModel) Focused() bool {
	return m.focused
}

// Resize updates the component dimensions.
func (m *TransactionDetailModel) Resize(width, height int) {
	m.width = width
	m.height = height
}
