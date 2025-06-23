package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/tui/themes"
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// StatsPanelModel displays session statistics.
type StatsPanelModel struct {
	theme          themes.Theme
	startTime      time.Time
	lastActionTime time.Time
	categoryStats  map[string]int
	progressBar    progress.Model
	skipped        int
	newCategories  int
	total          int
	userClassified int
	avgTime        time.Duration
	autoClassified int
	classified     int
	width          int
	height         int
	compact        bool
}

// NewStatsPanelModel creates a new stats panel.
func NewStatsPanelModel(theme themes.Theme) StatsPanelModel {
	prog := progress.New(progress.WithDefaultGradient())
	prog.ShowPercentage = false

	return StatsPanelModel{
		categoryStats: make(map[string]int),
		progressBar:   prog,
		theme:         theme,
		startTime:     time.Now(),
	}
}

// Update handles messages.
func (m StatsPanelModel) Update(msg tea.Msg) (StatsPanelModel, tea.Cmd) {
	switch msg := msg.(type) {
	case ClassificationCompleteMsg:
		m.updateStats(msg.Classification)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.progressBar.Width = min(m.width-4, 40)
	}

	return m, nil
}

// View renders the stats panel.
func (m StatsPanelModel) View() string {
	if m.compact {
		return m.renderCompact()
	}
	return m.renderFull()
}

// renderFull renders the full stats view.
func (m StatsPanelModel) renderFull() string {
	sections := []string{
		m.renderProgress(),
		m.renderTimeSaved(),
		m.renderBreakdown(),
	}

	if len(m.categoryStats) > 0 {
		sections = append(sections, m.renderCategoryDistribution())
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		sections...,
	)

	// Return raw content - parent will handle borders
	return content
}

// renderCompact renders a compact stats view.
func (m StatsPanelModel) renderCompact() string {
	progress := m.calculateProgress()

	stats := fmt.Sprintf(
		"Progress: %d/%d (%.0f%%) | Auto: %d | Time saved: %s",
		m.classified,
		m.total,
		progress*100,
		m.autoClassified,
		m.calculateTimeSaved(),
	)

	return m.theme.Box.Render(stats)
}

// renderProgress renders the progress section.
func (m StatsPanelModel) renderProgress() string {
	progress := m.calculateProgress()

	title := m.theme.Subtitle.Render("Progress")

	// Progress bar
	m.progressBar.SetPercent(progress)
	bar := m.progressBar.View()

	// Stats
	stats := fmt.Sprintf("%d/%d transactions (%.0f%%)",
		m.classified,
		m.total,
		progress*100,
	)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		bar,
		m.theme.Normal.Render(stats),
	)
}

// renderTimeSaved renders time savings.
func (m StatsPanelModel) renderTimeSaved() string {
	saved := m.calculateTimeSaved()
	elapsed := time.Since(m.startTime)

	title := m.theme.Subtitle.Render("Time")

	stats := []string{
		fmt.Sprintf("Elapsed:    %s", formatDuration(elapsed)),
		fmt.Sprintf("Saved:      %s", saved),
		fmt.Sprintf("Avg/txn:    %s", m.calculateAvgTime()),
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		m.theme.Normal.Render(strings.Join(stats, "\n")),
	)
}

// renderBreakdown renders classification breakdown.
func (m StatsPanelModel) renderBreakdown() string {
	title := m.theme.Subtitle.Render("Classification Breakdown")

	items := []struct {
		style lipgloss.Style
		label string
		count int
	}{
		{style: m.theme.StatusSuccess, label: "Auto-classified", count: m.autoClassified},
		{style: m.theme.StatusInfo, label: "User-modified", count: m.userClassified},
		{style: m.theme.StatusWarning, label: "Skipped", count: m.skipped},
		{style: m.theme.StatusInfo, label: "New categories", count: m.newCategories},
	}

	var lines []string
	for _, item := range items {
		if item.count > 0 {
			line := fmt.Sprintf("%-15s %s",
				item.label+":",
				item.style.Render(fmt.Sprintf("%d", item.count)),
			)
			lines = append(lines, line)
		}
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		m.theme.Normal.Render(strings.Join(lines, "\n")),
	)
}

// renderCategoryDistribution renders category distribution.
func (m StatsPanelModel) renderCategoryDistribution() string {
	if len(m.categoryStats) == 0 {
		return ""
	}

	title := m.theme.Subtitle.Render("Top Categories")

	// Get top categories
	type catStat struct {
		name  string
		count int
	}

	var stats []catStat
	for cat, count := range m.categoryStats {
		stats = append(stats, catStat{cat, count})
	}

	// Sort by count
	for i := 0; i < len(stats); i++ {
		for j := i + 1; j < len(stats); j++ {
			if stats[j].count > stats[i].count {
				stats[i], stats[j] = stats[j], stats[i]
			}
		}
	}

	// Render top 5
	var lines []string
	maxCount := stats[0].count
	for i, stat := range stats[:min(5, len(stats))] {
		icon := themes.GetCategoryIcon(stat.name)
		barLen := int(float64(stat.count) / float64(maxCount) * 15)
		bar := strings.Repeat("█", barLen)

		line := fmt.Sprintf("%s %-12s %s %d",
			icon,
			truncate(stat.name, 12),
			lipgloss.NewStyle().Foreground(m.theme.Primary).Render(bar),
			stat.count,
		)

		lines = append(lines, line)
		if i >= 4 {
			break
		}
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		m.theme.Normal.Render(strings.Join(lines, "\n")),
	)
}

// Helper methods

func (m *StatsPanelModel) updateStats(classification model.Classification) {
	m.classified++
	m.lastActionTime = time.Now()

	switch classification.Status {
	case model.StatusClassifiedByAI:
		m.autoClassified++
	case model.StatusUserModified:
		m.userClassified++
	case model.StatusUnclassified:
		m.skipped++
	}

	if classification.Category != "" {
		m.categoryStats[classification.Category]++
	}

	// Update average time
	if m.classified > 1 {
		totalTime := time.Since(m.startTime)
		m.avgTime = totalTime / time.Duration(m.classified)
	}
}

func (m StatsPanelModel) calculateProgress() float64 {
	if m.total == 0 {
		return 0
	}
	return float64(m.classified) / float64(m.total)
}

func (m StatsPanelModel) calculateTimeSaved() string {
	// Assume manual classification takes 15 seconds per transaction
	manualTime := time.Duration(m.autoClassified) * 15 * time.Second
	return formatDuration(manualTime)
}

func (m StatsPanelModel) calculateAvgTime() string {
	if m.avgTime == 0 {
		return "N/A"
	}
	return formatDuration(m.avgTime)
}

// SetTotal sets the total number of transactions.
func (m *StatsPanelModel) SetTotal(total int) {
	m.total = total
}

// SetCompact sets compact mode.
func (m *StatsPanelModel) SetCompact(compact bool) {
	m.compact = compact
}

// Resize updates the component size.
func (m *StatsPanelModel) Resize(width, height int) {
	m.width = width
	m.height = height
	m.progressBar.Width = min(width-4, 40)
}

// ClassificationCompleteMsg is sent when a classification is complete.
type ClassificationCompleteMsg struct {
	Classification model.Classification
}

// Utility functions

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
}
