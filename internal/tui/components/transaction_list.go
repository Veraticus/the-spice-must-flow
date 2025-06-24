package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/tui/themes"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TransactionListModel manages the transaction list view.
type TransactionListModel struct {
	theme        themes.Theme
	selected     map[string]bool
	search       string
	lastKey      string
	filtered     []model.Transaction
	sort         SortConfig
	transactions []model.Transaction
	searchInput  textinput.Model
	table        table.Model
	visualStart  int
	mode         ListMode
	width        int
	height       int
	cursor       int
}

// ListMode represents the current mode of the list.
type ListMode int

// List modes.
const (
	ModeNormal ListMode = iota
	ModeVisual
	ModeSearch
	ModeFilter
)

// TransactionGroup groups transactions by merchant.
type TransactionGroup struct {
	Merchant     string
	Transactions []model.Transaction
	Total        float64
	Collapsed    bool
}

// FilterConfig holds filter settings.
type FilterConfig struct {
	DateRange  *DateRange
	MinAmount  *float64
	MaxAmount  *float64
	Direction  *model.TransactionDirection
	Categories []string
}

// DateRange represents a date range.
type DateRange struct {
	Start time.Time
	End   time.Time
}

// SortConfig holds sort settings.
type SortConfig struct {
	Field     string
	Ascending bool
}

// TransactionSelectedMsg is sent when a transaction is selected.
type TransactionSelectedMsg struct {
	Transaction model.Transaction
	Index       int
}

// FocusTransactionMsg scrolls to a specific transaction.
type FocusTransactionMsg struct {
	Index int
}

// NewTransactionList creates a new transaction list.
func NewTransactionList(transactions []model.Transaction, theme themes.Theme) TransactionListModel {
	// Setup table
	columns := []table.Column{
		{Title: "Date", Width: 10},
		{Title: "Merchant", Width: 25},
		{Title: "Amount", Width: 12},
		{Title: "Category", Width: 20},
		{Title: "Status", Width: 10},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(20),
	)

	// Apply theme
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(theme.Border).
		BorderBottom(true).
		Bold(false)
	s.Selected = theme.Selected
	t.SetStyles(s)

	// Setup search input
	searchInput := textinput.New()
	searchInput.Placeholder = "Search transactions..."
	searchInput.CharLimit = 50

	model := TransactionListModel{
		transactions: transactions,
		filtered:     transactions,
		selected:     make(map[string]bool),
		table:        t,
		searchInput:  searchInput,
		mode:         ModeNormal,
		theme:        theme,
		width:        80,
		height:       24,
		sort: SortConfig{
			Field:     "date",
			Ascending: false,
		},
	}

	// Set initial column widths
	model.updateColumnWidths()

	return model
}

// Update handles messages.
func (m TransactionListModel) Update(msg tea.Msg) (TransactionListModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.mode {
		case ModeNormal:
			cmd := m.handleNormalMode(msg)
			cmds = append(cmds, cmd)
		case ModeVisual:
			cmd := m.handleVisualMode(msg)
			cmds = append(cmds, cmd)
		case ModeSearch:
			cmd := m.handleSearchMode(msg)
			cmds = append(cmds, cmd)
		}

		m.lastKey = msg.String()

	case FocusTransactionMsg:
		m.cursor = msg.Index
		cmds = append(cmds, m.ensureVisible())

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table.SetHeight(m.height - 4)
	}

	// Update table
	newTable, cmd := m.table.Update(msg)
	m.table = newTable
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// handleNormalMode handles key presses in normal mode.
func (m *TransactionListModel) handleNormalMode(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "j", "down":
		m.cursor = min(m.cursor+1, len(m.filtered)-1)
		return m.ensureVisible()

	case "k", "up":
		m.cursor = max(m.cursor-1, 0)
		return m.ensureVisible()

	case "G":
		m.cursor = len(m.filtered) - 1
		return m.ensureVisible()

	case "g":
		if m.lastKey == "g" {
			m.cursor = 0
			return m.ensureVisible()
		}

	case "v":
		m.mode = ModeVisual
		m.visualStart = m.cursor

	case "/":
		m.mode = ModeSearch
		m.searchInput.Focus()
		return textinput.Blink

	case "enter":
		if m.cursor < len(m.filtered) {
			return func() tea.Msg {
				return TransactionSelectedMsg{
					Transaction: m.filtered[m.cursor],
					Index:       m.cursor,
				}
			}
		}

	case "x":
		// Toggle selection
		if m.cursor < len(m.filtered) {
			txn := m.filtered[m.cursor]
			m.selected[txn.ID] = !m.selected[txn.ID]
		}

	case "ctrl+a":
		// Select all
		for _, txn := range m.filtered {
			m.selected[txn.ID] = true
		}

	case "ctrl+d":
		// Deselect all
		m.selected = make(map[string]bool)
	}

	return nil
}

// handleVisualMode handles key presses in visual mode.
func (m *TransactionListModel) handleVisualMode(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "j", "down":
		m.cursor = min(m.cursor+1, len(m.filtered)-1)
		m.updateVisualSelection()
		return m.ensureVisible()

	case "k", "up":
		m.cursor = max(m.cursor-1, 0)
		m.updateVisualSelection()
		return m.ensureVisible()

	case "esc", "v":
		m.mode = ModeNormal

	case "y":
		// Yank (select) visual selection
		m.mode = ModeNormal
	}

	return nil
}

// handleSearchMode handles key presses in search mode.
func (m *TransactionListModel) handleSearchMode(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "enter":
		m.search = m.searchInput.Value()
		m.applyFilters()
		m.mode = ModeNormal
		m.searchInput.Blur()

	case "esc":
		m.mode = ModeNormal
		m.searchInput.Blur()
		m.searchInput.SetValue("")

	default:
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		return cmd
	}

	return nil
}

// View renders the transaction list.
func (m TransactionListModel) View() string {
	if m.height < 10 {
		return "Terminal too small"
	}

	// Build view based on mode
	switch m.mode {
	case ModeSearch:
		return m.renderSearchView()
	default:
		return m.renderListView()
	}
}

// renderListView renders the main list view.
func (m TransactionListModel) renderListView() string {
	// Build table rows
	rows := m.buildTableRows()
	m.table.SetRows(rows)

	// Layout sections
	header := m.renderHeader()
	tableView := m.table.View()
	footer := m.renderFooter()

	// Combine with proper spacing
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		tableView,
		footer,
	)

	// Return raw content - parent will handle borders
	return content
}

// renderSearchView renders the search interface.
func (m TransactionListModel) renderSearchView() string {
	searchBox := m.theme.BorderedBox.
		Width(60).
		Render(lipgloss.JoinVertical(
			lipgloss.Left,
			m.theme.Title.Render("Search Transactions"),
			m.searchInput.View(),
			lipgloss.NewStyle().Foreground(m.theme.Muted).Render("Press Enter to search, Esc to cancel"),
		))

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		searchBox,
	)
}

// renderHeader renders the list header.
func (m TransactionListModel) renderHeader() string {
	title := m.theme.Title.Render("Transactions")

	status := fmt.Sprintf("%d transactions", len(m.filtered))
	if len(m.selected) > 0 {
		status += fmt.Sprintf(" (%d selected)", len(m.selected))
	}

	if m.search != "" {
		status += fmt.Sprintf(" | Search: %q", m.search)
	}

	subtitle := m.theme.Subtitle.Render(status)

	return lipgloss.JoinVertical(lipgloss.Left, title, subtitle)
}

// renderFooter renders the list footer.
func (m TransactionListModel) renderFooter() string {
	var hints []string

	switch m.mode {
	case ModeNormal:
		hints = []string{
			"[↑↓] Navigate",
			"[Enter] Classify",
			"[/] Search",
			"[v] Visual",
			"[?] Help",
		}
	case ModeVisual:
		hints = []string{
			"[↑↓] Extend selection",
			"[y] Confirm",
			"[Esc] Cancel",
		}
	}

	return lipgloss.NewStyle().Foreground(m.theme.Muted).Render(strings.Join(hints, "  "))
}

// buildTableRows builds rows for the table.
func (m TransactionListModel) buildTableRows() []table.Row {
	rows := make([]table.Row, 0, len(m.filtered))

	for _, txn := range m.filtered {
		date := txn.Date.Format("2006-01-02")
		merchant := truncate(txn.MerchantName, 25)
		amount := fmt.Sprintf("$%.2f", txn.Amount)

		category := ""
		if len(txn.Category) > 0 {
			category = txn.Category[0]
		}
		if category == "" {
			category = lipgloss.NewStyle().Foreground(m.theme.Muted).Render("Unclassified")
		}

		status := "?"
		if len(txn.Category) > 0 {
			status = "✓"
		}

		// Apply selection highlighting
		if m.selected[txn.ID] {
			date = m.theme.Highlighted.Render(date)
			merchant = m.theme.Highlighted.Render(merchant)
			amount = m.theme.Highlighted.Render(amount)
			category = m.theme.Highlighted.Render(category)
			status = m.theme.Highlighted.Render(status)
		}

		rows = append(rows, table.Row{
			date,
			merchant,
			amount,
			category,
			status,
		})
	}

	return rows
}

// Helper functions

func (m *TransactionListModel) updateVisualSelection() {
	// Clear previous selections
	m.selected = make(map[string]bool)

	// Select range from visualStart to cursor
	start := min(m.visualStart, m.cursor)
	end := max(m.visualStart, m.cursor)

	for i := start; i <= end && i < len(m.filtered); i++ {
		m.selected[m.filtered[i].ID] = true
	}
}

func (m *TransactionListModel) applyFilters() {
	m.filtered = m.transactions

	// Apply search filter
	if m.search != "" {
		var filtered []model.Transaction
		searchLower := strings.ToLower(m.search)

		for _, txn := range m.filtered {
			categoryMatch := false
			for _, cat := range txn.Category {
				if strings.Contains(strings.ToLower(cat), searchLower) {
					categoryMatch = true
					break
				}
			}

			if strings.Contains(strings.ToLower(txn.MerchantName), searchLower) ||
				strings.Contains(strings.ToLower(txn.Name), searchLower) ||
				categoryMatch {
				filtered = append(filtered, txn)
			}
		}

		m.filtered = filtered
	}

	// Reset cursor if needed
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}

func (m TransactionListModel) ensureVisible() tea.Cmd {
	// This would implement scrolling logic
	// For now, return nil
	return nil
}

// Resize updates the component size.
func (m *TransactionListModel) Resize(width, height int) {
	m.width = width
	m.height = height

	// Account for chrome:
	// - Header: 1 line for title + 1 line spacing + 1 line column headers = 3
	// - Footer: 1 line for controls = 1
	// Total chrome: 4 lines
	tableHeight := max(1, height-4)
	m.table.SetHeight(tableHeight)
	m.updateColumnWidths()
}

// updateColumnWidths dynamically adjusts column widths based on available space.
func (m *TransactionListModel) updateColumnWidths() {
	// Account for borders and padding
	availableWidth := m.width - 4
	if availableWidth < 60 {
		availableWidth = 60 // Minimum width
	}

	// Calculate proportional widths
	columns := []table.Column{
		{Title: "Date", Width: max(10, int(float64(availableWidth)*0.13))},
		{Title: "Merchant", Width: max(15, int(float64(availableWidth)*0.33))},
		{Title: "Amount", Width: max(10, int(float64(availableWidth)*0.15))},
		{Title: "Category", Width: max(12, int(float64(availableWidth)*0.26))},
		{Title: "Status", Width: max(8, int(float64(availableWidth)*0.13))},
	}

	m.table.SetColumns(columns)
}

// Helper to truncate strings.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// Removed minInt and maxInt - using built-in min/max functions
