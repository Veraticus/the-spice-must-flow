package testing

import (
	"fmt"
	"strings"
)

// StateMatcher provides methods for asserting component state through public interfaces.
type StateMatcher struct {
	failures []string
}

// NewStateMatcher creates a new state matcher.
func NewStateMatcher() *StateMatcher {
	return &StateMatcher{
		failures: make([]string, 0),
	}
}

// ViewContains asserts that the view contains the expected string.
func (m *StateMatcher) ViewContains(view, expected string) *StateMatcher {
	if !strings.Contains(view, expected) {
		m.failures = append(m.failures, fmt.Sprintf("view does not contain '%s'", expected))
	}
	return m
}

// ViewNotContains asserts that the view does not contain the unexpected string.
func (m *StateMatcher) ViewNotContains(view, unexpected string) *StateMatcher {
	if strings.Contains(view, unexpected) {
		m.failures = append(m.failures, fmt.Sprintf("view contains unexpected '%s'", unexpected))
	}
	return m
}

// ViewMatches asserts that the view matches a pattern after stripping ANSI codes.
func (m *StateMatcher) ViewMatches(view, pattern string) *StateMatcher {
	stripped := StripANSI(view)
	normalized := NormalizeWhitespace(stripped)
	expectedNormalized := NormalizeWhitespace(pattern)

	if normalized != expectedNormalized {
		m.failures = append(m.failures, fmt.Sprintf("view does not match pattern:\nGot: %s\nWant: %s", normalized, expectedNormalized))
	}
	return m
}

// LineCount asserts the number of lines in the view.
func (m *StateMatcher) LineCount(view string, expected int) *StateMatcher {
	lines := strings.Split(strings.TrimRight(view, "\n"), "\n")
	actual := len(lines)

	if actual != expected {
		m.failures = append(m.failures, fmt.Sprintf("line count mismatch: got %d, want %d", actual, expected))
	}
	return m
}

// LineContains asserts that a specific line contains the expected text.
func (m *StateMatcher) LineContains(view string, lineNum int, expected string) *StateMatcher {
	lines := strings.Split(view, "\n")

	if lineNum < 0 || lineNum >= len(lines) {
		m.failures = append(m.failures, fmt.Sprintf("line %d out of bounds (total lines: %d)", lineNum, len(lines)))
		return m
	}

	line := StripANSI(lines[lineNum])
	if !strings.Contains(line, expected) {
		m.failures = append(m.failures, fmt.Sprintf("line %d does not contain '%s': '%s'", lineNum, expected, line))
	}
	return m
}

// HasSelection verifies that text appears to be selected (usually indicated by specific styling).
func (m *StateMatcher) HasSelection(view string, text string) *StateMatcher {
	// This is a simplified check - in practice, you'd look for specific ANSI codes
	// or patterns that indicate selection in your TUI framework
	if !strings.Contains(view, text) {
		m.failures = append(m.failures, fmt.Sprintf("selection '%s' not found", text))
	}
	return m
}

// CommandCount verifies the expected number of commands were generated.
func (m *StateMatcher) CommandCount(renderer *TestRenderer, expected int) *StateMatcher {
	actual := len(renderer.Commands)
	if actual != expected {
		m.failures = append(m.failures, fmt.Sprintf("command count mismatch: got %d, want %d", actual, expected))
	}
	return m
}

// NoCommands verifies that no commands were generated.
func (m *StateMatcher) NoCommands(renderer *TestRenderer) *StateMatcher {
	return m.CommandCount(renderer, 0)
}

// HasCommand verifies that at least one command was generated.
func (m *StateMatcher) HasCommand(renderer *TestRenderer) *StateMatcher {
	if len(renderer.Commands) == 0 {
		m.failures = append(m.failures, "expected at least one command, but none were generated")
	}
	return m
}

// Check returns an error if any assertions failed.
func (m *StateMatcher) Check() error {
	if len(m.failures) > 0 {
		return fmt.Errorf("state assertions failed:\n%s", strings.Join(m.failures, "\n"))
	}
	return nil
}

// MustCheck calls Check and panics if there are failures.
func (m *StateMatcher) MustCheck() {
	if err := m.Check(); err != nil {
		panic(err)
	}
}

// TableMatcher provides specialized assertions for table components.
type TableMatcher struct {
	*StateMatcher
}

// NewTableMatcher creates a new table matcher.
func NewTableMatcher() *TableMatcher {
	return &TableMatcher{
		StateMatcher: NewStateMatcher(),
	}
}

// RowCount verifies the number of visible rows in a table.
func (m *TableMatcher) RowCount(view string, expected int) *TableMatcher {
	// Count lines that look like table rows (contain │ or similar separators)
	lines := strings.Split(view, "\n")
	rowCount := 0

	for _, line := range lines {
		if strings.Contains(line, "│") && !strings.Contains(line, "─") {
			rowCount++
		}
	}

	// Subtract header row if present
	if rowCount > 0 && expected > 0 {
		rowCount--
	}

	if rowCount != expected {
		m.failures = append(m.failures, fmt.Sprintf("table row count mismatch: got %d, want %d", rowCount, expected))
	}
	return m
}

// CellContains verifies that a specific cell contains the expected text.
func (m *TableMatcher) CellContains(view string, row, col int, expected string) *TableMatcher {
	lines := strings.Split(view, "\n")
	tableRows := make([]string, 0)

	// Extract table rows
	for _, line := range lines {
		if strings.Contains(line, "│") && !strings.Contains(line, "─") {
			tableRows = append(tableRows, line)
		}
	}

	// Skip header row
	if len(tableRows) > 0 {
		tableRows = tableRows[1:]
	}

	if row < 0 || row >= len(tableRows) {
		m.failures = append(m.failures, fmt.Sprintf("table row %d out of bounds (total rows: %d)", row, len(tableRows)))
		return m
	}

	// Split by cell separator
	cells := strings.Split(tableRows[row], "│")

	if col < 0 || col >= len(cells)-2 { // -2 because split includes empty strings at start/end
		m.failures = append(m.failures, fmt.Sprintf("table column %d out of bounds (total columns: %d)", col, len(cells)-2))
		return m
	}

	cell := strings.TrimSpace(StripANSI(cells[col+1])) // +1 to skip empty string at start
	if !strings.Contains(cell, expected) {
		m.failures = append(m.failures, fmt.Sprintf("table cell [%d,%d] does not contain '%s': '%s'", row, col, expected, cell))
	}

	return m
}
