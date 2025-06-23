package tui

import (
	"fmt"
	"strings"

	"github.com/Veraticus/the-spice-must-flow/internal/tui/components"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// renderLoading renders the loading screen.
func (m Model) renderLoading() string {
	loadingText := m.theme.Title.Render("Loading The Spice Must Flow...")

	spinner := "⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏"
	frame := int(m.sessionStats.TotalTransactions) % len(spinner)

	content := lipgloss.JoinVertical(
		lipgloss.Center,
		loadingText,
		"",
		lipgloss.NewStyle().Foreground(m.theme.Primary).Render(string(spinner[frame])),
		"",
		lipgloss.NewStyle().Foreground(m.theme.Muted).Render("Initializing transaction classifier..."),
	)

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		content,
	)
}

// renderCompactView renders the compact layout for narrow terminals.
func (m Model) renderCompactView() string {
	var content string

	switch m.state {
	case StateList:
		// Account for borders (2) and status bar (1)
		usableWidth := m.width - 2
		usableHeight := m.height - 3

		// Reserve space for stats if enabled
		listHeight := usableHeight
		if m.config.ShowStats {
			listHeight = usableHeight - 3 // 2 for stats + 1 for spacing
		}

		// Full width transaction list
		m.transactionList.Resize(usableWidth, listHeight)
		content = m.transactionList.View()

		// Compact stats at bottom
		if m.config.ShowStats {
			m.statsPanel.SetCompact(true)
			m.statsPanel.Resize(usableWidth, 2)
			stats := m.statsPanel.View()
			content = lipgloss.JoinVertical(
				lipgloss.Left,
				content,
				stats,
			)
		}

	case StateClassifying:
		// Full screen classifier
		// Account for borders (2) and status bar (1)
		usableWidth := m.width - 2
		usableHeight := m.height - 3
		m.classifier.Resize(usableWidth, usableHeight)
		content = m.classifier.View()

	case StateBatch:
		// Full screen batch view
		m.batchView.Resize(m.width-2, m.height-2)
		content = m.batchView.View()

	case StateDirectionConfirm:
		// Full screen direction confirmation
		m.directionView.Resize(m.width-2, m.height-2)
		content = m.directionView.View()

	case StateHelp:
		content = m.renderHelp()
	}

	return m.wrapWithBorder(content)
}

// renderMediumView renders the layout for medium terminals.
func (m Model) renderMediumView() string {
	switch m.state {
	case StateList:
		// Left: Transaction list (70%)
		// Right: Stats panel (30%)
		// Account for: border (2) + separator (3) = 5 total
		totalUsableWidth := m.width - 5
		leftWidth := int(float64(totalUsableWidth) * 0.7)
		rightWidth := totalUsableWidth - leftWidth

		// Account for status bar (1) + borders (2) = 3 total
		usableHeight := m.height - 3
		m.transactionList.Resize(leftWidth, usableHeight)
		left := m.transactionList.View()

		m.statsPanel.SetCompact(false)
		m.statsPanel.Resize(rightWidth, usableHeight)
		right := m.statsPanel.View()

		content := lipgloss.JoinHorizontal(
			lipgloss.Top,
			left,
			m.theme.Normal.Render(" │ "),
			right,
		)

		return m.wrapWithBorder(content)

	case StateClassifying:
		// Full screen with side stats
		// Account for: border (2) + separator (3) = 5 total
		totalUsableWidth := m.width - 5
		mainWidth := int(float64(totalUsableWidth) * 0.75)
		statsWidth := totalUsableWidth - mainWidth

		// Account for status bar (1) + borders (2) = 3 total
		usableHeight := m.height - 3
		m.classifier.Resize(mainWidth, usableHeight)
		main := m.classifier.View()

		m.statsPanel.SetCompact(true)
		m.statsPanel.Resize(statsWidth, usableHeight)
		stats := m.statsPanel.View()

		content := lipgloss.JoinHorizontal(
			lipgloss.Top,
			main,
			m.theme.Normal.Render(" │ "),
			stats,
		)

		return m.wrapWithBorder(content)

	default:
		return m.renderCompactView()
	}
}

// renderFullView renders the full layout for wide terminals.
func (m Model) renderFullView() string {
	switch m.state {
	case StateList:
		// Three column layout
		// Left: Transaction list (40%)
		// Middle: Preview/Details (35%)
		// Right: Stats & Categories (25%)
		// Account for: border (2) + 2 separators (6) = 8 total
		totalUsableWidth := m.width - 8
		leftWidth := int(float64(totalUsableWidth) * 0.4)
		middleWidth := int(float64(totalUsableWidth) * 0.35)
		rightWidth := totalUsableWidth - leftWidth - middleWidth

		// Account for status bar (1) + borders (2) = 3 total
		usableHeight := m.height - 3
		m.transactionList.Resize(leftWidth, usableHeight)
		left := m.transactionList.View()

		middle := m.renderTransactionPreview(middleWidth)

		m.statsPanel.SetCompact(false)
		m.statsPanel.Resize(rightWidth, usableHeight)
		right := m.statsPanel.View()

		content := lipgloss.JoinHorizontal(
			lipgloss.Top,
			left,
			m.theme.Normal.Render(" │ "),
			middle,
			m.theme.Normal.Render(" │ "),
			right,
		)

		return m.wrapWithBorder(content)

	case StateClassifying:
		// Two column layout with details
		// Account for: border (2) + separator (3) = 5 total
		totalUsableWidth := m.width - 5
		mainWidth := int(float64(totalUsableWidth) * 0.6)
		sideWidth := totalUsableWidth - mainWidth

		// Account for status bar (1) + borders (2) = 3 total
		usableHeight := m.height - 3
		m.classifier.Resize(mainWidth, usableHeight)
		main := m.classifier.View()

		// Right side: transaction history + stats
		// Split height evenly with 1 line for separator
		halfHeight := (usableHeight - 1) / 2
		rightTop := m.renderSimilarTransactions(sideWidth, halfHeight)
		m.statsPanel.Resize(sideWidth, halfHeight)
		rightBottom := m.statsPanel.View()

		right := lipgloss.JoinVertical(
			lipgloss.Left,
			rightTop,
			m.theme.Normal.Render(strings.Repeat("─", sideWidth)),
			rightBottom,
		)

		content := lipgloss.JoinHorizontal(
			lipgloss.Top,
			main,
			m.theme.Normal.Render(" │ "),
			right,
		)

		return m.wrapWithBorder(content)

	default:
		return m.renderMediumView()
	}
}

// renderTransactionPreview renders a preview of the selected transaction.
func (m Model) renderTransactionPreview(width int) string {
	// Get selected transaction from list
	// For now, return placeholder

	title := m.theme.Title.Render("Transaction Details")

	placeholder := lipgloss.NewStyle().Foreground(m.theme.Muted).Render("Select a transaction to view details")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		placeholder,
	)

	// Don't set a fixed height - let content determine height
	return m.theme.Box.
		Width(width).
		MaxWidth(width).
		Render(content)
}

// renderSimilarTransactions renders similar transaction history.
func (m Model) renderSimilarTransactions(width, height int) string {
	title := m.theme.Subtitle.Render("Similar Transactions")

	// Would show previous classifications for this merchant
	placeholder := lipgloss.NewStyle().Foreground(m.theme.Muted).Render("No similar transactions found")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		placeholder,
	)

	return m.theme.Box.
		Width(width).
		MaxWidth(width).
		Height(height).
		Render(content)
}

// renderHelp renders the help screen.
func (m Model) renderHelp() string {
	title := m.theme.Title.Render("The Spice Must Flow - Help")

	sections := []struct {
		title string
		items []string
	}{
		{
			"Navigation",
			[]string{
				"↑/k, ↓/j    Move up/down",
				"←/h, →/l    Move left/right",
				"PgUp/PgDn   Page up/down",
				"g/G         Go to start/end",
			},
		},
		{
			"Classification",
			[]string{
				"Enter       Classify transaction",
				"a/y         Accept suggestion",
				"s/Space     Skip transaction",
				"c           Custom category",
				"1-5         Quick select suggestion",
			},
		},
		{
			"Selection",
			[]string{
				"v           Visual mode",
				"x           Toggle selection",
				"Ctrl+A      Select all",
				"Ctrl+D      Deselect all",
			},
		},
		{
			"Views",
			[]string{
				"Tab         Cycle views",
				"/           Search",
				"f           Filter",
				"?           Toggle help",
			},
		},
		{
			"Application",
			[]string{
				"q/Esc       Quit",
				"Ctrl+C      Force quit",
				"Ctrl+L      Clear screen",
			},
		},
	}

	var content []string
	for _, section := range sections {
		sectionTitle := m.theme.Subtitle.Render(section.title)
		content = append(content, sectionTitle)

		for _, item := range section.items {
			parts := strings.SplitN(item, "  ", 2)
			if len(parts) == 2 {
				line := fmt.Sprintf("  %-12s %s",
					lipgloss.NewStyle().Foreground(m.theme.Primary).Render(parts[0]),
					m.theme.Normal.Render(parts[1]),
				)
				content = append(content, line)
			}
		}
		content = append(content, "")
	}

	helpText := lipgloss.JoinVertical(
		lipgloss.Left,
		content...,
	)

	footer := lipgloss.NewStyle().Foreground(m.theme.Muted).Render("Press ? or Esc to close help")

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		m.theme.BorderedBox.
			Width(60).
			MaxHeight(m.height-4).
			Render(
				lipgloss.JoinVertical(
					lipgloss.Left,
					title,
					"",
					helpText,
					footer,
				),
			),
	)
}

// wrapWithBorder adds a border around content.
func (m Model) wrapWithBorder(content string) string {
	// Add status bar at bottom
	statusBar := m.renderStatusBar()

	fullContent := lipgloss.JoinVertical(
		lipgloss.Left,
		content,
		statusBar,
	)

	return m.theme.BorderedBox.
		Width(m.width).
		Height(m.height).
		Render(fullContent)
}

// renderStatusBar renders the bottom status bar.
func (m Model) renderStatusBar() string {
	var left, center, right string

	// Left: current mode/state
	switch m.state {
	case StateList:
		left = "Browse"
	case StateClassifying:
		left = "Classify"
	case StateBatch:
		left = "Batch"
	case StateDirectionConfirm:
		left = "Direction"
	}

	// Center: progress
	if m.sessionStats.TotalTransactions > 0 {
		progress := float64(m.sessionStats.UserClassified+m.sessionStats.AutoClassified) /
			float64(m.sessionStats.TotalTransactions)
		progressBar := m.renderMiniProgressBar(20, progress)
		center = fmt.Sprintf("%s %d/%d",
			progressBar,
			m.sessionStats.UserClassified+m.sessionStats.AutoClassified,
			m.sessionStats.TotalTransactions,
		)
	}

	// Right: help hint
	right = "? Help"

	// Calculate spacing
	totalWidth := m.width - 4 // Account for borders
	leftWidth := len(left)
	centerWidth := len(center)
	rightWidth := len(right)

	spacing := totalWidth - leftWidth - centerWidth - rightWidth
	leftPad := spacing / 2
	rightPad := spacing - leftPad

	status := fmt.Sprintf("%s%s%s%s%s",
		m.theme.StatusInfo.Render(left),
		strings.Repeat(" ", leftPad),
		m.theme.Normal.Render(center),
		strings.Repeat(" ", rightPad),
		lipgloss.NewStyle().Foreground(m.theme.Muted).Render(right),
	)

	return m.theme.Normal.
		Background(m.theme.Border).
		Width(m.width - 2).
		MaxWidth(m.width - 2).
		Render(status)
}

// renderMiniProgressBar renders a small progress bar.
func (m Model) renderMiniProgressBar(width int, progress float64) string {
	filled := int(float64(width) * progress)
	empty := width - filled

	bar := m.theme.StatusSuccess.Render(strings.Repeat("█", filled)) +
		lipgloss.NewStyle().Foreground(m.theme.Muted).Render(strings.Repeat("░", empty))

	return bar
}

// showError displays an error message.
func (m Model) showError(err error) tea.Cmd {
	// This would show a temporary error notification
	return nil
}

// handleDataLoaded processes loaded data.
func (m *Model) handleDataLoaded(msg dataLoadedMsg) {
	// Update UI components with loaded data
	if msg.dataType == "transactions" && len(m.transactions) > 0 {
		m.transactionList = components.NewTransactionList(m.transactions, m.theme)
		m.statsPanel.SetTotal(len(m.transactions))
	}
}

// handleTransactionSelection processes transaction selection.
func (m Model) handleTransactionSelection(msg components.TransactionSelectedMsg) tea.Cmd {
	// This would trigger classification for the selected transaction
	return nil
}
