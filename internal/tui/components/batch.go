// Package components provides reusable UI components for the TUI.
package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/tui/themes"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// BatchViewModel manages batch classification.
type BatchViewModel struct {
	theme          themes.Theme
	pending        []model.PendingClassification
	results        []model.Classification
	groups         []BatchGroup
	currentIndex   int
	currentGroup   int
	mode           BatchMode
	selectedAction BatchAction
	cursor         int
	width          int
	height         int
	groupMode      bool
}

// BatchMode represents the current batch mode.
type BatchMode int

// Batch mode constants.
const (
	// BatchModeReview shows transactions to review.
	BatchModeReview BatchMode = iota
	// BatchModeApply applies actions to transactions.
	BatchModeApply
	// BatchModeConfirm confirms batch operations.
	BatchModeConfirm
)

// BatchAction represents an action to apply to a batch.
type BatchAction int

// Batch action constants.
const (
	// BatchActionAcceptAll accepts all suggestions.
	BatchActionAcceptAll BatchAction = iota
	// BatchActionSkipAll skips all transactions.
	BatchActionSkipAll
	// BatchActionReviewEach reviews each transaction.
	BatchActionReviewEach
	// BatchActionApplyCategory applies a category.
	BatchActionApplyCategory
)

// BatchGroup groups similar transactions.
type BatchGroup struct {
	Key               string
	SuggestedCategory string
	Transactions      []model.PendingClassification
	Confidence        float64
}

// NewBatchViewModel creates a new batch view model.
func NewBatchViewModel(pending []model.PendingClassification, theme themes.Theme) BatchViewModel {
	groups := groupPendingTransactions(pending)

	return BatchViewModel{
		pending: pending,
		results: make([]model.Classification, 0, len(pending)),
		groups:  groups,
		theme:   theme,
		mode:    BatchModeReview,
	}
}

// Update handles messages.
func (m BatchViewModel) Update(msg tea.Msg) (BatchViewModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.mode {
		case BatchModeReview:
			return m.handleReviewMode(msg)
		case BatchModeApply:
			return m.handleApplyMode(msg)
		case BatchModeConfirm:
			return m.handleConfirmMode(msg)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	return m, nil
}

// handleReviewMode handles key presses in review mode.
func (m BatchViewModel) handleReviewMode(msg tea.KeyMsg) (BatchViewModel, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.groupMode {
			m.currentGroup = min(m.currentGroup+1, len(m.groups)-1)
		} else {
			m.cursor = min(m.cursor+1, len(m.pending)-1)
		}

	case "k", "up":
		if m.groupMode {
			m.currentGroup = max(m.currentGroup-1, 0)
		} else {
			m.cursor = max(m.cursor-1, 0)
		}

	case "g":
		// Toggle group mode
		m.groupMode = !m.groupMode

	case "a":
		// Accept all in group/selection
		m.selectedAction = BatchActionAcceptAll
		m.mode = BatchModeConfirm

	case "s":
		// Skip all
		m.selectedAction = BatchActionSkipAll
		m.mode = BatchModeConfirm

	case "r":
		// Review each
		m.selectedAction = BatchActionReviewEach
		m.mode = BatchModeApply
		m.currentIndex = 0

	case "c":
		// Apply category to all
		m.selectedAction = BatchActionApplyCategory
		m.mode = BatchModeApply
	}

	return m, nil
}

// handleApplyMode handles applying actions.
func (m BatchViewModel) handleApplyMode(msg tea.KeyMsg) (BatchViewModel, tea.Cmd) {
	switch m.selectedAction {
	case BatchActionReviewEach:
		// Handle individual review
		switch msg.String() {
		case "a", "y":
			// Accept current
			m.acceptCurrent()
			m.nextTransaction()

		case "s", "n":
			// Skip current
			m.skipCurrent()
			m.nextTransaction()

		case "esc":
			m.mode = BatchModeReview
		}

	case BatchActionApplyCategory:
		// Handle category selection
		// This would show a category picker
		m.mode = BatchModeReview
	}

	return m, nil
}

// handleConfirmMode handles confirmation.
func (m BatchViewModel) handleConfirmMode(msg tea.KeyMsg) (BatchViewModel, tea.Cmd) {
	switch msg.String() {
	case "y", "enter":
		// Confirm action
		m.applySelectedAction()
		return m, tea.Quit

	case "n", "esc":
		// Cancel
		m.mode = BatchModeReview
	}

	return m, nil
}

// View renders the batch interface.
func (m BatchViewModel) View() string {
	switch m.mode {
	case BatchModeReview:
		return m.renderReviewMode()
	case BatchModeApply:
		return m.renderApplyMode()
	case BatchModeConfirm:
		return m.renderConfirmMode()
	default:
		return ""
	}
}

// renderReviewMode renders the review interface.
func (m BatchViewModel) renderReviewMode() string {
	title := m.theme.Title.Render("Batch Classification")

	subtitle := fmt.Sprintf("%d transactions to classify", len(m.pending))
	if m.groupMode {
		subtitle = fmt.Sprintf("%d groups (%d transactions)", len(m.groups), len(m.pending))
	}

	var content string
	if m.groupMode {
		content = m.renderGroups()
	} else {
		content = m.renderTransactionList()
	}

	help := lipgloss.NewStyle().Foreground(m.theme.Muted).Render(
		"[g] Toggle groups | [a] Accept all | [s] Skip all | [r] Review each | [c] Apply category",
	)

	sections := []string{
		title,
		m.theme.Subtitle.Render(subtitle),
		"",
		content,
		"",
		help,
	}

	return m.theme.BorderedBox.
		Width(m.width).
		MaxHeight(m.height).
		Render(lipgloss.JoinVertical(lipgloss.Left, sections...))
}

// renderGroups renders transaction groups.
func (m BatchViewModel) renderGroups() string {
	lines := make([]string, 0, len(m.groups))

	for i, group := range m.groups {
		prefix := "  "
		if i == m.currentGroup {
			prefix = lipgloss.NewStyle().Foreground(m.theme.Primary).Render("> ")
		}

		icon := themes.GetCategoryIcon(group.SuggestedCategory)
		confidence := int(group.Confidence * 100)

		line := fmt.Sprintf("%s%s %-20s (%d txns) → %s %s (%d%%)",
			prefix,
			icon,
			truncate(group.Key, 20),
			len(group.Transactions),
			themes.GetCategoryIcon(group.SuggestedCategory),
			group.SuggestedCategory,
			confidence,
		)

		if i == m.currentGroup {
			line = m.theme.Selected.Render(line)
		}

		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// renderTransactionList renders individual transactions.
func (m BatchViewModel) renderTransactionList() string {
	// Pre-allocate for up to 10 visible items
	lines := make([]string, 0, 10)

	start := max(0, m.cursor-5)
	end := min(len(m.pending), start+10)

	for i := start; i < end; i++ {
		p := m.pending[i]
		prefix := "  "
		if i == m.cursor {
			prefix = lipgloss.NewStyle().Foreground(m.theme.Primary).Render("> ")
		}

		line := fmt.Sprintf("%s%-20s $%-8.2f → %s",
			prefix,
			truncate(p.Transaction.MerchantName, 20),
			p.Transaction.Amount,
			p.SuggestedCategory,
		)

		if i == m.cursor {
			line = m.theme.Selected.Render(line)
		}

		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// renderApplyMode renders the apply interface.
func (m BatchViewModel) renderApplyMode() string {
	switch m.selectedAction {
	case BatchActionReviewEach:
		return m.renderIndividualReview()
	case BatchActionApplyCategory:
		return m.renderCategoryPicker()
	default:
		return ""
	}
}

// renderIndividualReview renders individual transaction review.
func (m BatchViewModel) renderIndividualReview() string {
	if m.currentIndex >= len(m.pending) {
		return m.renderComplete()
	}

	p := m.pending[m.currentIndex]

	title := fmt.Sprintf("Transaction %d of %d", m.currentIndex+1, len(m.pending))

	transaction := fmt.Sprintf("%s\n%s\n$%.2f",
		p.Transaction.MerchantName,
		p.Transaction.Date.Format("Jan 2, 2006"),
		p.Transaction.Amount,
	)

	suggestion := fmt.Sprintf("Suggested: %s %s (%.0f%%)",
		themes.GetCategoryIcon(p.SuggestedCategory),
		p.SuggestedCategory,
		p.Confidence*100,
	)

	help := "[a] Accept | [s] Skip | [Esc] Cancel"

	sections := []string{
		m.theme.Title.Render(title),
		"",
		m.theme.Box.Render(transaction),
		"",
		m.theme.StatusInfo.Render(suggestion),
		"",
		lipgloss.NewStyle().Foreground(m.theme.Muted).Render(help),
	}

	return m.theme.BorderedBox.Render(
		lipgloss.JoinVertical(lipgloss.Left, sections...),
	)
}

// renderCategoryPicker renders category selection.
func (m BatchViewModel) renderCategoryPicker() string {
	// This would show a category picker interface
	return m.theme.BorderedBox.Render("Category picker not implemented")
}

// renderConfirmMode renders confirmation dialog.
func (m BatchViewModel) renderConfirmMode() string {
	var action string
	var details string

	switch m.selectedAction {
	case BatchActionAcceptAll:
		action = "Accept All Suggestions"
		details = fmt.Sprintf("This will classify %d transactions with their AI suggestions.", len(m.pending))
	case BatchActionSkipAll:
		action = "Skip All"
		details = fmt.Sprintf("This will skip %d transactions, leaving them unclassified.", len(m.pending))
	}

	sections := []string{
		m.theme.Title.Render("Confirm Action"),
		"",
		m.theme.StatusWarning.Render(action),
		m.theme.Normal.Render(details),
		"",
		lipgloss.NewStyle().Foreground(m.theme.Muted).Render("[y] Confirm | [n] Cancel"),
	}

	return m.theme.BorderedBox.Render(
		lipgloss.JoinVertical(lipgloss.Left, sections...),
	)
}

// renderComplete renders completion message.
func (m BatchViewModel) renderComplete() string {
	stats := fmt.Sprintf(
		"Completed: %d classified, %d skipped",
		len(m.results),
		len(m.pending)-len(m.results),
	)

	return m.theme.BorderedBox.Render(
		lipgloss.JoinVertical(
			lipgloss.Center,
			m.theme.StatusSuccess.Render("Batch Classification Complete!"),
			m.theme.Normal.Render(stats),
		),
	)
}

// Helper methods

func (m *BatchViewModel) acceptCurrent() {
	if m.currentIndex < len(m.pending) {
		p := m.pending[m.currentIndex]
		classification := model.Classification{
			Transaction:  p.Transaction,
			Category:     p.SuggestedCategory,
			Status:       model.StatusClassifiedByAI,
			Confidence:   p.Confidence,
			ClassifiedAt: time.Now(),
		}
		m.results = append(m.results, classification)
	}
}

func (m *BatchViewModel) skipCurrent() {
	if m.currentIndex < len(m.pending) {
		p := m.pending[m.currentIndex]
		classification := model.Classification{
			Transaction:  p.Transaction,
			Status:       model.StatusUnclassified,
			ClassifiedAt: time.Now(),
		}
		m.results = append(m.results, classification)
	}
}

func (m *BatchViewModel) nextTransaction() {
	m.currentIndex++
	if m.currentIndex >= len(m.pending) {
		// All done
		m.mode = BatchModeReview
	}
}

func (m *BatchViewModel) applySelectedAction() {
	switch m.selectedAction {
	case BatchActionAcceptAll:
		for _, p := range m.pending {
			classification := model.Classification{
				Transaction:  p.Transaction,
				Category:     p.SuggestedCategory,
				Status:       model.StatusClassifiedByAI,
				Confidence:   p.Confidence,
				ClassifiedAt: time.Now(),
			}
			m.results = append(m.results, classification)
		}

	case BatchActionSkipAll:
		for _, p := range m.pending {
			classification := model.Classification{
				Transaction:  p.Transaction,
				Status:       model.StatusUnclassified,
				ClassifiedAt: time.Now(),
			}
			m.results = append(m.results, classification)
		}
	}
}

// GetResults returns the classification results.
func (m BatchViewModel) GetResults() []model.Classification {
	if len(m.results) == len(m.pending) {
		return m.results
	}
	return nil
}

// Resize updates the component size.
func (m *BatchViewModel) Resize(width, height int) {
	m.width = width
	m.height = height
}

// groupPendingTransactions groups transactions by merchant.
func groupPendingTransactions(pending []model.PendingClassification) []BatchGroup {
	groupMap := make(map[string][]model.PendingClassification)

	for _, p := range pending {
		key := p.Transaction.MerchantName
		groupMap[key] = append(groupMap[key], p)
	}

	groups := make([]BatchGroup, 0, len(groupMap))
	for key, transactions := range groupMap {
		// Calculate group confidence as average
		var totalConf float64
		suggestedCat := ""

		for _, t := range transactions {
			totalConf += t.Confidence
			if suggestedCat == "" {
				suggestedCat = t.SuggestedCategory
			}
		}

		groups = append(groups, BatchGroup{
			Key:               key,
			Transactions:      transactions,
			SuggestedCategory: suggestedCat,
			Confidence:        totalConf / float64(len(transactions)),
		})
	}

	return groups
}
