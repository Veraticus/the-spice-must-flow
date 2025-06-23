package components

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/engine"
	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/tui/themes"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ClassifierModel manages the classification interface.
type ClassifierModel struct {
	theme            themes.Theme
	error            error
	classifier       engine.Classifier
	result           *model.Classification
	rankings         model.CategoryRankings
	categories       []model.Category
	sortedCategories []model.Category // Sorted by AI confidence then alphabetically
	customInput      textinput.Model
	spinner          spinner.Model
	transaction      model.Transaction
	pending          model.PendingClassification
	mode             ClassifierMode
	cursor           int
	categoryCursor   int // Cursor for category selection
	categoryOffset   int // Offset for scrolling through categories
	width            int
	height           int
	loading          bool
	complete         bool
}

// ClassifierMode represents the current mode.
type ClassifierMode int

const (
	ModeSelectingSuggestion ClassifierMode = iota
	ModeEnteringCustom
	ModeSelectingCategory
	ModeConfirming
)

// AIClassificationMsg is sent when AI completes classification.
type AIClassificationMsg struct {
	Error    error
	Rankings model.CategoryRankings
}

// NewClassifierModel creates a new classifier.
func NewClassifierModel(pending model.PendingClassification, categories []model.Category, theme themes.Theme, classifier engine.Classifier) ClassifierModel {
	// Setup custom input
	customInput := textinput.New()
	customInput.Placeholder = "Enter custom category..."
	customInput.CharLimit = 50

	// Setup spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(theme.Primary)

	m := ClassifierModel{
		transaction: pending.Transaction,
		pending:     pending,
		rankings:    pending.CategoryRankings,
		categories:  categories,
		customInput: customInput,
		spinner:     s,
		theme:       theme,
		classifier:  classifier,
		mode:        ModeSelectingSuggestion,
	}

	// If we don't have rankings yet, start loading
	if len(m.rankings) == 0 && classifier != nil {
		m.loading = true
	}

	return m
}

// Init returns initial commands.
func (m ClassifierModel) Init() tea.Cmd {
	cmds := []tea.Cmd{m.spinner.Tick}

	// Start AI classification if needed
	if m.loading && m.classifier != nil {
		cmds = append(cmds, m.classifyWithAI())
	}

	return tea.Batch(cmds...)
}

// Update handles messages.
func (m ClassifierModel) Update(msg tea.Msg) (ClassifierModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case AIClassificationMsg:
		m.loading = false
		if msg.Error != nil {
			m.error = msg.Error
		} else {
			m.rankings = msg.Rankings
		}

	case tea.KeyMsg:
		switch m.mode {
		case ModeSelectingSuggestion:
			cmd := m.handleSuggestionMode(msg)
			cmds = append(cmds, cmd)

		case ModeEnteringCustom:
			cmd := m.handleCustomMode(msg)
			cmds = append(cmds, cmd)

		case ModeSelectingCategory:
			cmd := m.handleCategoryMode(msg)
			cmds = append(cmds, cmd)
		}

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	return m, tea.Batch(cmds...)
}

// handleSuggestionMode handles key presses when selecting suggestions.
func (m *ClassifierModel) handleSuggestionMode(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "j", "down":
		m.cursor = min(m.cursor+1, len(m.rankings)-1)

	case "k", "up":
		m.cursor = max(m.cursor-1, 0)

	case "enter", "a":
		// Accept selected suggestion
		if m.cursor < len(m.rankings) {
			return m.confirmCategory(m.rankings[m.cursor])
		}

	case "c":
		// Show category picker
		m.mode = ModeSelectingCategory
		m.prepareCategoryList()
		m.categoryCursor = 0
		m.categoryOffset = 0

	case "s", "space":
		// Skip
		m.result = &model.Classification{
			Transaction:  m.transaction,
			Status:       model.StatusUnclassified,
			ClassifiedAt: time.Now(),
		}
		m.complete = true

	case "1", "2", "3", "4", "5":
		// Quick select by number
		idx := int(msg.String()[0] - '1')
		if idx < len(m.rankings) {
			return m.confirmCategory(m.rankings[idx])
		}
	}

	return nil
}

// handleCustomMode handles key presses when entering custom category.
func (m *ClassifierModel) handleCustomMode(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "enter":
		input := strings.TrimSpace(m.customInput.Value())
		if input != "" {
			// Check if input is a number (category ID)
			if categoryID, err := strconv.Atoi(input); err == nil {
				// Find category by ID
				for _, cat := range m.categories {
					if cat.ID == categoryID {
						m.customInput.Blur()
						m.customInput.SetValue("")
						return m.confirmCategoryByName(cat.Name, 1.0)
					}
				}
				// Category ID not found, treat as custom category name
			}
			// Not a number or ID not found, create custom category
			return m.createCustomCategory(input)
		}

	case "esc":
		// Return to previous mode
		if m.categoryCursor >= 0 {
			m.mode = ModeSelectingCategory
		} else {
			m.mode = ModeSelectingSuggestion
		}
		m.customInput.Blur()
		m.customInput.SetValue("")
		return nil

	default:
		var cmd tea.Cmd
		m.customInput, cmd = m.customInput.Update(msg)
		return cmd
	}

	return nil
}

// View renders the classifier interface.
func (m ClassifierModel) View() string {
	if m.loading {
		return m.renderLoading()
	}

	if m.error != nil {
		return m.renderError()
	}

	sections := []string{
		m.renderTransaction(),
	}

	switch m.mode {
	case ModeSelectingSuggestion:
		sections = append(sections, m.renderSuggestions())
	case ModeSelectingCategory:
		sections = append(sections, m.renderCategoryPicker())
	case ModeEnteringCustom:
		sections = append(sections, m.renderCustomInput())
	}

	sections = append(sections, m.renderHelp())

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	// Return raw content - parent will handle borders
	return content
}

// renderLoading renders the loading state.
func (m ClassifierModel) renderLoading() string {
	content := lipgloss.JoinVertical(
		lipgloss.Center,
		m.spinner.View(),
		m.theme.Subtitle.Render("Analyzing transaction..."),
	)

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		m.theme.Box.Render(content),
	)
}

// renderError renders error state.
func (m ClassifierModel) renderError() string {
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		m.theme.StatusError.Render("Error classifying transaction"),
		m.theme.Normal.Render(m.error.Error()),
		"",
		lipgloss.NewStyle().Foreground(m.theme.Muted).Render("Press 's' to skip, 'c' for custom category"),
	)

	// Return raw content - parent will handle borders
	return content
}

// renderTransaction renders transaction details.
func (m ClassifierModel) renderTransaction() string {
	icon := themes.GetCategoryIcon("Other")
	if m.pending.SuggestedCategory != "" {
		icon = themes.GetCategoryIcon(m.pending.SuggestedCategory)
	}

	header := lipgloss.JoinHorizontal(
		lipgloss.Left,
		m.theme.CategoryIcon.Render(icon),
		" ",
		m.theme.Title.Render(m.transaction.MerchantName),
	)

	details := []string{
		fmt.Sprintf("Date: %s", m.transaction.Date.Format("January 2, 2006")),
		fmt.Sprintf("Amount: %s%.2f", m.getAmountPrefix(), m.transaction.Amount),
		fmt.Sprintf("Type: %s", m.transaction.Type),
	}

	if m.transaction.Name != m.transaction.MerchantName {
		details = append(details, fmt.Sprintf("Description: %s", m.transaction.Name))
	}

	detailsStr := m.theme.Normal.Render(strings.Join(details, "\n"))

	// Add AI analysis if available
	var analysis string
	if m.pending.CategoryDescription != "" {
		analysis = m.theme.Box.
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(m.theme.Muted).
			Render(m.theme.Italic.Render("AI: " + m.pending.CategoryDescription))
	}

	sections := []string{header, detailsStr}
	if analysis != "" {
		sections = append(sections, analysis)
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderSuggestions renders category suggestions.
func (m ClassifierModel) renderSuggestions() string {
	if len(m.rankings) == 0 {
		return lipgloss.NewStyle().Foreground(m.theme.Muted).Render("No suggestions available")
	}

	title := m.theme.Subtitle.Render("Suggested Categories:")

	var suggestions []string
	for i, ranking := range m.rankings[:min(5, len(m.rankings))] {
		// Build confidence bar
		confidence := int(ranking.Score * 100)
		bar := m.renderConfidenceBar(confidence)

		// Get category icon
		icon := themes.GetCategoryIcon(ranking.Category)

		// Format line
		prefix := "  "
		if i == m.cursor {
			prefix = lipgloss.NewStyle().Foreground(m.theme.Primary).Render("> ")
		}

		line := fmt.Sprintf("%s%s %s %s %d%%",
			prefix,
			icon,
			ranking.Category,
			bar,
			confidence,
		)

		// Add number hint
		numHint := lipgloss.NewStyle().Foreground(m.theme.Muted).Render(fmt.Sprintf("[%d]", i+1))
		line = numHint + " " + line

		if i == m.cursor {
			line = m.theme.Selected.Render(line)
		}

		suggestions = append(suggestions, line)
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		strings.Join(suggestions, "\n"),
	)
}

// renderConfidenceBar renders a visual confidence indicator.
func (m ClassifierModel) renderConfidenceBar(confidence int) string {
	width := 20
	filled := int(float64(width) * float64(confidence) / 100.0)

	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)

	// Color based on confidence
	var style lipgloss.Style
	if confidence >= 80 {
		style = m.theme.StatusSuccess
	} else if confidence >= 50 {
		style = m.theme.StatusWarning
	} else {
		style = m.theme.StatusError
	}

	return style.Render(bar)
}

// renderCustomInput renders the custom category input.
func (m ClassifierModel) renderCustomInput() string {
	return m.theme.Box.Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			m.theme.Subtitle.Render("Enter Custom Category:"),
			m.customInput.View(),
		),
	)
}

// renderCategoryPicker renders the full category selection interface.
func (m ClassifierModel) renderCategoryPicker() string {
	title := m.theme.Subtitle.Render("Select Category (All Categories):")

	visibleItems := 10
	var categories []string

	// Calculate visible range
	start := m.categoryOffset
	end := min(start+visibleItems, len(m.sortedCategories))

	for i := start; i < end; i++ {
		cat := m.sortedCategories[i]

		// Build the display line
		prefix := "  "
		if i == m.categoryCursor {
			prefix = lipgloss.NewStyle().Foreground(m.theme.Primary).Render("> ")
		}

		// Get confidence if available
		confidence := 0.0
		for _, ranking := range m.rankings {
			if ranking.Category == cat.Name {
				confidence = ranking.Score
				break
			}
		}

		// Format with category ID, name, and confidence if available
		var line string
		if confidence > 0 {
			bar := m.renderConfidenceBar(int(confidence * 100))
			line = fmt.Sprintf("%s[%d] %s %s %s %.0f%%",
				prefix,
				cat.ID,
				themes.GetCategoryIcon(cat.Name),
				cat.Name,
				bar,
				confidence*100,
			)
		} else {
			line = fmt.Sprintf("%s[%d] %s %s",
				prefix,
				cat.ID,
				themes.GetCategoryIcon(cat.Name),
				cat.Name,
			)
		}

		if i == m.categoryCursor {
			line = m.theme.Selected.Render(line)
		}

		categories = append(categories, line)
	}

	// Add scroll indicators
	if m.categoryOffset > 0 {
		categories = append([]string{
			lipgloss.NewStyle().Foreground(m.theme.Muted).Render("  ↑ More categories above"),
		}, categories...)
	}

	if end < len(m.sortedCategories) {
		categories = append(categories,
			lipgloss.NewStyle().Foreground(m.theme.Muted).Render(
				fmt.Sprintf("  ↓ %d more categories below", len(m.sortedCategories)-end),
			),
		)
	}

	// Add total count
	count := lipgloss.NewStyle().Foreground(m.theme.Muted).Render(
		fmt.Sprintf("Showing %d-%d of %d categories", start+1, end, len(m.sortedCategories)),
	)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		strings.Join(categories, "\n"),
		"",
		count,
	)
}

// renderHelp renders keyboard shortcuts.
func (m ClassifierModel) renderHelp() string {
	var hints []string

	switch m.mode {
	case ModeSelectingSuggestion:
		hints = []string{
			"[↑↓] Navigate",
			"[1-5] Quick select",
			"[Enter/a] Accept",
			"[c] Select from all categories",
			"[s] Skip",
		}
	case ModeSelectingCategory:
		hints = []string{
			"[↑↓] Navigate",
			"[g/G] First/Last",
			"[Enter] Select",
			"[/] Search",
			"[Type ID] Quick select",
			"[Esc] Back",
		}
	case ModeEnteringCustom:
		hints = []string{
			"[Enter] Confirm",
			"[Esc] Cancel",
		}
	}

	return lipgloss.NewStyle().Foreground(m.theme.Muted).Render(strings.Join(hints, "  "))
}

// confirmCategory creates a classification result.
func (m *ClassifierModel) confirmCategory(ranking model.CategoryRanking) tea.Cmd {
	return func() tea.Msg {
		m.result = &model.Classification{
			Transaction:  m.transaction,
			Category:     ranking.Category,
			Status:       model.StatusClassifiedByAI,
			Confidence:   ranking.Score,
			ClassifiedAt: time.Now(),
		}
		m.complete = true
		return nil
	}
}

// createCustomCategory creates a custom category classification.
func (m *ClassifierModel) createCustomCategory(category string) tea.Cmd {
	return func() tea.Msg {
		m.result = &model.Classification{
			Transaction:  m.transaction,
			Category:     category,
			Status:       model.StatusUserModified,
			Confidence:   1.0,
			ClassifiedAt: time.Now(),
			Notes:        "Custom category",
		}
		m.complete = true
		return nil
	}
}

// classifyWithAI starts AI classification.
func (m ClassifierModel) classifyWithAI() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		rankings, err := m.classifier.SuggestCategoryRankings(
			ctx,
			m.transaction,
			m.categories,
			m.pending.CheckPatterns,
		)

		return AIClassificationMsg{
			Rankings: rankings,
			Error:    err,
		}
	}
}

// Helper methods

// IsComplete returns whether classification is complete.
func (m ClassifierModel) IsComplete() bool {
	return m.complete
}

// GetResult returns the classification result.
func (m ClassifierModel) GetResult() model.Classification {
	if m.result != nil {
		return *m.result
	}
	return model.Classification{}
}

// Resize updates the component size.
func (m *ClassifierModel) Resize(width, height int) {
	m.width = width
	m.height = height
}

func (m ClassifierModel) getAmountPrefix() string {
	if m.transaction.Direction == model.DirectionIncome {
		return "+"
	}
	return "-"
}

// prepareCategoryList sorts categories by AI confidence then alphabetically.
func (m *ClassifierModel) prepareCategoryList() {
	// Create a map of categories to their AI confidence scores
	confidenceMap := make(map[string]float64)
	for _, ranking := range m.rankings {
		confidenceMap[ranking.Category] = ranking.Score
	}

	// Copy all categories
	m.sortedCategories = make([]model.Category, len(m.categories))
	copy(m.sortedCategories, m.categories)

	// Sort: first by confidence (descending), then alphabetically
	sort.Slice(m.sortedCategories, func(i, j int) bool {
		confI := confidenceMap[m.sortedCategories[i].Name]
		confJ := confidenceMap[m.sortedCategories[j].Name]

		if confI != confJ {
			return confI > confJ // Higher confidence first
		}
		return m.sortedCategories[i].Name < m.sortedCategories[j].Name // Alphabetical
	})
}

// handleCategoryMode handles key presses when selecting from all categories.
func (m *ClassifierModel) handleCategoryMode(msg tea.KeyMsg) tea.Cmd {
	visibleItems := 10 // Number of categories visible at once

	switch msg.String() {
	case "j", "down":
		m.categoryCursor++
		if m.categoryCursor >= len(m.sortedCategories) {
			m.categoryCursor = len(m.sortedCategories) - 1
		}
		// Adjust scroll offset
		if m.categoryCursor >= m.categoryOffset+visibleItems {
			m.categoryOffset = m.categoryCursor - visibleItems + 1
		}

	case "k", "up":
		m.categoryCursor--
		if m.categoryCursor < 0 {
			m.categoryCursor = 0
		}
		// Adjust scroll offset
		if m.categoryCursor < m.categoryOffset {
			m.categoryOffset = m.categoryCursor
		}

	case "g", "home":
		m.categoryCursor = 0
		m.categoryOffset = 0

	case "G", "end":
		m.categoryCursor = len(m.sortedCategories) - 1
		m.categoryOffset = max(0, len(m.sortedCategories)-visibleItems)

	case "enter":
		// Select the current category
		if m.categoryCursor < len(m.sortedCategories) {
			category := m.sortedCategories[m.categoryCursor]
			// Get confidence score if available
			confidence := 1.0
			for _, ranking := range m.rankings {
				if ranking.Category == category.Name {
					confidence = ranking.Score
					break
				}
			}

			return m.confirmCategoryByName(category.Name, confidence)
		}

	case "esc":
		// Return to suggestion mode
		m.mode = ModeSelectingSuggestion
		m.categoryCursor = 0
		m.categoryOffset = 0

	case "/":
		// Switch to custom entry mode for searching
		m.mode = ModeEnteringCustom
		m.customInput.Focus()
		return textinput.Blink

	default:
		// Handle number input for category ID selection
		if len(msg.String()) == 1 && msg.String()[0] >= '0' && msg.String()[0] <= '9' {
			// Start collecting digits for category ID
			m.mode = ModeEnteringCustom
			m.customInput.SetValue(msg.String())
			m.customInput.Focus()
			return textinput.Blink
		}
	}

	return nil
}

// confirmCategoryByName creates a classification result for a category by name.
func (m *ClassifierModel) confirmCategoryByName(categoryName string, confidence float64) tea.Cmd {
	return func() tea.Msg {
		m.result = &model.Classification{
			Transaction:  m.transaction,
			Category:     categoryName,
			Status:       model.StatusUserModified,
			Confidence:   confidence,
			ClassifiedAt: time.Now(),
		}
		m.complete = true
		return nil
	}
}
