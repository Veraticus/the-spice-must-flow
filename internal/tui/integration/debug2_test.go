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

// TestMinimalStandalone tests the absolute minimum to get transactions showing.
func TestMinimalStandalone(t *testing.T) {
	// Single transaction
	txn := model.Transaction{
		ID:           "test_txn_1",
		AccountID:    "test_acc",
		MerchantName: "VISIBLE_TEST_MERCHANT",
		Amount:       -99.99,
		Date:         time.Now(),
		Type:         "PURCHASE",
	}

	// Create harness
	harness := integration.NewHarness(t,
		integration.WithTestData(integration.TestData{
			Transactions: []model.Transaction{txn},
			Categories:   []model.Category{{ID: 1, Name: "Test"}},
		}),
		// 		integration.
	)
	defer harness.Cleanup()

	// Start
	require.NoError(t, harness.Start())

	// Give it more time to load
	for i := 0; i < 10; i++ {
		time.Sleep(200 * time.Millisecond)
		screen := harness.GetCurrentScreen()

		t.Logf("Attempt %d - Screen contains 'Loading': %v", i+1, strings.Contains(screen, "Loading"))
		t.Logf("Screen contains 'VISIBLE_TEST_MERCHANT': %v", strings.Contains(screen, "VISIBLE_TEST_MERCHANT"))
		t.Logf("Screen contains 'VISIBLE_TEST_MERCH': %v", strings.Contains(screen, "VISIBLE_TEST_MERCH"))

		if strings.Contains(screen, "VISIBLE_TEST_MERCH") {
			t.Log("SUCCESS: Transaction is visible!")
			return
		}

		// Try sending a render update
		if i == 5 {
			t.Log("Sending manual window resize to trigger render...")
			harness.GetProgram().Send(tea.WindowSizeMsg{Width: 80, Height: 24})
		}
	}

	// Final screen dump
	screen := harness.GetCurrentScreen()
	t.Logf("Final screen:\n%s", screen)
	t.Fatal("Transaction never became visible")
}
