package integration_test

import (
	"strings"
	"testing"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/tui/integration"
	"github.com/stretchr/testify/require"
)

// TestDebugClassification debugs the classification workflow.
func TestDebugClassification(t *testing.T) {
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

	t.Log("Initial screen:")
	t.Log(harness.GetCurrentScreen())

	// Navigate to first transaction
	harness.SendKeys("g") // Go to top
	time.Sleep(100 * time.Millisecond)

	t.Log("\nAfter navigation:")
	screen := harness.GetCurrentScreen()
	t.Log(screen)
	t.Logf("Screen contains 'Test Merchant': %v", strings.Contains(screen, "Test Merchant"))

	// Press enter to start classification
	t.Log("\nPressing Enter to start classification...")
	harness.SendKeys("enter")

	// Wait for state change
	err := harness.WaitForState(1, 2*time.Second)
	if err != nil {
		t.Logf("Error waiting for state: %v", err)
		t.Log("Current screen:")
		t.Log(harness.GetCurrentScreen())
		t.Fatal("Failed to enter classification state")
	}

	t.Log("SUCCESS: Entered classification state!")
	finalScreen := harness.GetCurrentScreen()
	t.Log("Final screen:")
	t.Log(finalScreen)
	t.Logf("Screen contains 'Analyzing': %v", strings.Contains(finalScreen, "Analyzing"))
}
