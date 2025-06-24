// Package integration provides testing utilities for the TUI components.
package integration

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/tui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Assertions provides type-safe verification methods for TUI testing.
type Assertions struct {
	harness *Harness
}

// NewAssertions creates a new assertion helper.
func NewAssertions(h *Harness) *Assertions {
	return &Assertions{harness: h}
}

// AssertCurrentView verifies the TUI is in the expected view.
func (a *Assertions) AssertCurrentView(t *testing.T, expected tui.View) {
	t.Helper()

	// Get current screen and check for view indicators
	screen := a.harness.GetCurrentScreen()

	switch expected {
	case tui.ViewTransactions:
		assert.Contains(t, screen, "Transactions", "should be in transactions view")
	case tui.ViewMerchantGroups:
		assert.Contains(t, screen, "Merchant Groups", "should be in merchant groups view")
	case tui.ViewCalendar:
		assert.Contains(t, screen, "Calendar", "should be in calendar view")
	case tui.ViewStats:
		assert.Contains(t, screen, "Statistics", "should be in stats view")
	default:
		t.Errorf("unknown view: %v", expected)
	}
}

// AssertCurrentState verifies the TUI is in the expected state.
func (a *Assertions) AssertCurrentState(t *testing.T, expected tui.State) {
	t.Helper()

	// Wait for the state with a short timeout
	err := a.harness.WaitForState(expected, 500*time.Millisecond)
	require.NoError(t, err, "TUI should reach state %v", expected)
}

// AssertTransactionCount verifies the number of transactions displayed.
func (a *Assertions) AssertTransactionCount(t *testing.T, expected int) {
	t.Helper()

	screen := a.harness.GetCurrentScreen()

	// Count transaction lines (simplified - would need more sophisticated parsing)
	lines := strings.Split(screen, "\n")
	transactionCount := 0

	for _, line := range lines {
		// Look for transaction patterns (date, amount, merchant)
		if strings.Contains(line, "$") && strings.Contains(line, "/") {
			transactionCount++
		}
	}

	assert.Equal(t, expected, transactionCount, "should have %d transactions visible", expected)
}

// AssertSelectedTransaction verifies a specific transaction is selected.
func (a *Assertions) AssertSelectedTransaction(t *testing.T, txnID string) {
	t.Helper()

	// Find the transaction in test data
	var expectedTxn *model.Transaction
	for _, txn := range a.harness.testData.Transactions {
		if txn.ID == txnID {
			expectedTxn = &txn
			break
		}
	}

	require.NotNil(t, expectedTxn, "transaction %s should exist in test data", txnID)

	screen := a.harness.GetCurrentScreen()

	// Check for selection indicators (assumes highlighted line contains merchant name)
	// Handle truncated merchant names by checking for the first part of the name
	merchantPrefix := expectedTxn.MerchantName
	if len(merchantPrefix) > 15 {
		merchantPrefix = merchantPrefix[:15] // Match first 15 chars which should be visible
	}
	assert.Contains(t, screen, merchantPrefix, "selected transaction should be visible")
}

// AssertTransactionClassified verifies a transaction has been classified.
func (a *Assertions) AssertTransactionClassified(t *testing.T, txnID, expectedCategory string) {
	t.Helper()

	// Check in mock storage
	classification, ok := a.harness.storage.GetClassification(txnID)
	require.True(t, ok, "transaction %s should be classified", txnID)
	assert.Equal(t, expectedCategory, classification.Category, "transaction should have correct category")
}

// AssertNotification verifies a notification is displayed.
func (a *Assertions) AssertNotification(t *testing.T, msgType, content string) {
	t.Helper()

	screen := a.harness.GetCurrentScreen()

	// Check for notification indicators
	switch msgType {
	case "success":
		assert.Contains(t, screen, "✓", "should show success indicator")
	case "error":
		assert.Contains(t, screen, "✗", "should show error indicator")
	case "info":
		assert.Contains(t, screen, "ℹ", "should show info indicator")
	}

	assert.Contains(t, screen, content, "notification should contain expected content")
}

// AssertNoErrors verifies no error messages are displayed.
func (a *Assertions) AssertNoErrors(t *testing.T) {
	t.Helper()

	screen := a.harness.GetCurrentScreen()

	// Check for common error indicators
	assert.NotContains(t, screen, "Error:", "should not show error messages")
	assert.NotContains(t, screen, "Failed", "should not show failure messages")
	assert.NotContains(t, screen, "✗", "should not show error indicators")
}

// AssertCategoryListVisible verifies the category list is shown.
func (a *Assertions) AssertCategoryListVisible(t *testing.T) {
	t.Helper()

	screen := a.harness.GetCurrentScreen()

	// Check for at least some categories
	foundCategories := 0
	testDataCategories := a.harness.GetTestData().Categories

	// Debug output
	if len(testDataCategories) == 0 {
		t.Logf("WARNING: No categories in test data!")
	}

	for _, cat := range testDataCategories {
		if strings.Contains(screen, cat.Name) {
			foundCategories++
		}
	}

	assert.Greater(t, foundCategories, 0, "should show at least one category (checked %d categories)", len(testDataCategories))
}

// AssertCategorySuggestion verifies a specific category is suggested.
func (a *Assertions) AssertCategorySuggestion(t *testing.T, categoryName string, position int) {
	t.Helper()

	screen := a.harness.GetCurrentScreen()

	// Look for numbered suggestions with the new format [1] or in the content
	suggestionPattern1 := fmt.Sprintf("[%d]", position)
	suggestionPattern2 := categoryName

	// Check if both the number and category name appear
	assert.Contains(t, screen, suggestionPattern1, "should have position [%d]", position)
	assert.Contains(t, screen, suggestionPattern2, "category %s should be suggested", categoryName)
}

// AssertKeyboardShortcutHint verifies keyboard shortcuts are displayed.
func (a *Assertions) AssertKeyboardShortcutHint(t *testing.T, key, action string) {
	t.Helper()

	screen := a.harness.GetCurrentScreen()

	// Check for keyboard hint patterns
	hints := []string{
		fmt.Sprintf("[%s] %s", key, action),
		fmt.Sprintf("%s: %s", key, action),
		fmt.Sprintf("(%s) %s", key, action),
	}

	found := false
	for _, hint := range hints {
		if strings.Contains(screen, hint) {
			found = true
			break
		}
	}

	assert.True(t, found, "should show keyboard shortcut %s for %s", key, action)
}

// AssertLoadingIndicator verifies a loading indicator is shown.
func (a *Assertions) AssertLoadingIndicator(t *testing.T) {
	t.Helper()

	screen := a.harness.GetCurrentScreen()

	// Check for common loading indicators
	loadingIndicators := []string{
		"Loading",
		"Please wait",
		"...",
		"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏", // Spinner characters
	}

	found := false
	for _, indicator := range loadingIndicators {
		if strings.Contains(screen, indicator) {
			found = true
			break
		}
	}

	assert.True(t, found, "should show loading indicator")
}

// AssertBatchModeActive verifies batch classification mode is active.
func (a *Assertions) AssertBatchModeActive(t *testing.T, expectedCount int) {
	t.Helper()

	screen := a.harness.GetCurrentScreen()

	// Check for batch mode indicators
	assert.Contains(t, screen, "Batch", "should indicate batch mode")
	assert.Contains(t, screen, fmt.Sprintf("%d", expectedCount), "should show transaction count")
}

// AssertProgressBar verifies a progress bar is displayed correctly.
func (a *Assertions) AssertProgressBar(t *testing.T, completed, total int) {
	t.Helper()

	screen := a.harness.GetCurrentScreen()

	// Check for progress indicators
	progressText := fmt.Sprintf("%d/%d", completed, total)
	assert.Contains(t, screen, progressText, "should show progress as %s", progressText)

	// Could also check for visual progress bar characters
	percentage := float64(completed) / float64(total) * 100
	percentText := fmt.Sprintf("%.0f%%", percentage)
	assert.Contains(t, screen, percentText, "should show percentage")
}

// AssertHelpMenuVisible verifies the help menu is displayed.
func (a *Assertions) AssertHelpMenuVisible(t *testing.T) {
	t.Helper()

	screen := a.harness.GetCurrentScreen()

	// Check for help menu indicators
	assert.Contains(t, screen, "Help", "should show help title")
	assert.Contains(t, screen, "Navigation", "should show navigation section")
	assert.Contains(t, screen, "Commands", "should show commands section")
}

// AssertSearchMode verifies search mode is active.
func (a *Assertions) AssertSearchMode(t *testing.T, query string) {
	t.Helper()

	screen := a.harness.GetCurrentScreen()

	// Check for search mode indicators
	assert.Contains(t, screen, "Search:", "should show search prompt")
	if query != "" {
		assert.Contains(t, screen, query, "should show search query")
	}
}

// AssertVisualMode verifies visual selection mode is active.
func (a *Assertions) AssertVisualMode(t *testing.T, selectedCount int) {
	t.Helper()

	screen := a.harness.GetCurrentScreen()

	// Check for visual mode indicators
	assert.Contains(t, screen, "Visual", "should indicate visual mode")
	if selectedCount > 0 {
		assert.Contains(t, screen, fmt.Sprintf("%d selected", selectedCount), "should show selection count")
	}
}

// AssertConfirmationPrompt verifies a confirmation prompt is shown.
func (a *Assertions) AssertConfirmationPrompt(t *testing.T, message string) {
	t.Helper()

	screen := a.harness.GetCurrentScreen()

	// Check for confirmation prompt
	assert.Contains(t, screen, message, "should show confirmation message")
	assert.Contains(t, screen, "y/n", "should show yes/no prompt")
}

// AssertStatsPanel verifies the stats panel shows correct information.
func (a *Assertions) AssertStatsPanel(t *testing.T, stats StatsExpectation) {
	t.Helper()

	screen := a.harness.GetCurrentScreen()

	// Check for stats values
	if stats.TotalTransactions > 0 {
		assert.Contains(t, screen, fmt.Sprintf("Total: %d", stats.TotalTransactions), "should show total transactions")
	}

	if stats.ClassifiedCount > 0 {
		assert.Contains(t, screen, fmt.Sprintf("Classified: %d", stats.ClassifiedCount), "should show classified count")
	}

	if stats.PendingCount > 0 {
		assert.Contains(t, screen, fmt.Sprintf("Pending: %d", stats.PendingCount), "should show pending count")
	}
}

// StatsExpectation defines expected values for stats panel assertions.
type StatsExpectation struct {
	TotalTransactions int
	ClassifiedCount   int
	PendingCount      int
	AccuracyRate      float64
}

// AssertScreenMatches performs a fuzzy match against expected screen content.
func (a *Assertions) AssertScreenMatches(t *testing.T, expectedPatterns ...string) {
	t.Helper()

	screen := a.harness.GetCurrentScreen()

	for _, pattern := range expectedPatterns {
		assert.Contains(t, screen, pattern, "screen should contain pattern: %s", pattern)
	}
}

// AssertScreenDoesNotMatch verifies patterns are not present.
func (a *Assertions) AssertScreenDoesNotMatch(t *testing.T, unexpectedPatterns ...string) {
	t.Helper()

	screen := a.harness.GetCurrentScreen()

	for _, pattern := range unexpectedPatterns {
		assert.NotContains(t, screen, pattern, "screen should not contain pattern: %s", pattern)
	}
}
