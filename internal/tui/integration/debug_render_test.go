package integration_test

import (
	"strings"
	"testing"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/tui/integration"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
)

// TestDebugRenderAfterClassification debugs why the view doesn't update after state change.
func TestDebugRenderAfterClassification(t *testing.T) {
	// Create test data
	testTransactions := []model.Transaction{
		{ID: "txn_1", MerchantName: "Test Merchant", Amount: -50.00, Date: time.Now()},
	}

	testCategories := []model.Category{
		{ID: 1, Name: "Groceries"},
		{ID: 2, Name: "Shopping"},
	}

	harness := integration.NewHarness(t,
		integration.WithTestData(integration.TestData{
			Transactions: testTransactions,
			Categories:   testCategories,
		}),
		// 		integration.
	)
	defer harness.Cleanup()

	require.NoError(t, harness.Start())

	// Wait for initial render
	time.Sleep(200 * time.Millisecond)

	t.Log("Initial screen (should show transaction list):")
	screen1 := harness.GetCurrentScreen()
	t.Log(screen1)

	// Press enter to start classification
	t.Log("\nPressing Enter to start classification...")
	harness.SendKeys("enter")

	// Wait for state change
	err := harness.WaitForState(1, 2*time.Second)
	require.NoError(t, err, "Failed to enter classification state")

	// Get immediate screen after state change
	screen2 := harness.GetCurrentScreen()
	t.Log("\nScreen immediately after state change:")
	t.Log(screen2)

	// Force a render by sending a resize event
	t.Log("\nSending window resize to force render...")
	harness.GetProgram().Send(tea.WindowSizeMsg{Width: 80, Height: 24})
	time.Sleep(100 * time.Millisecond)

	screen3 := harness.GetCurrentScreen()
	t.Log("\nScreen after resize:")
	t.Log(screen3)

	// Wait a bit more
	time.Sleep(500 * time.Millisecond)

	screen4 := harness.GetCurrentScreen()
	t.Log("\nFinal screen after waiting:")
	t.Log(screen4)

	// Check if any screen shows classification UI
	foundClassifierUI := false
	for i, screen := range []string{screen1, screen2, screen3, screen4} {
		if strings.Contains(screen, "Analyzing") ||
			strings.Contains(screen, "Category") ||
			strings.Contains(screen, "Suggestions") {
			t.Logf("Found classifier UI in screen %d", i+1)
			foundClassifierUI = true
		}
	}

	require.True(t, foundClassifierUI, "Never saw classifier UI despite state change")
}
