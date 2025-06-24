package integration_test

import (
	"testing"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/tui"
	"github.com/Veraticus/the-spice-must-flow/internal/tui/integration"
	"github.com/stretchr/testify/require"
)

func TestBatchModeTransitionDebug(t *testing.T) {
	testTransactions := []model.Transaction{
		{ID: "1", MerchantName: "Test 1", Amount: 10.00},
		{ID: "2", MerchantName: "Test 2", Amount: 20.00},
		{ID: "3", MerchantName: "Test 3", Amount: 30.00},
	}

	harness := integration.NewHarness(t,
		integration.WithTestData(integration.TestData{
			Transactions: testTransactions,
		}),
	)
	defer harness.Cleanup()

	require.NoError(t, harness.Start())

	// Add a small delay to ensure the UI is fully ready
	time.Sleep(100 * time.Millisecond)

	// Log current state before sending key
	t.Logf("Current screen before 'b' key:\n%s", harness.GetCurrentScreen())

	// Send 'b' key to enter batch mode
	harness.SendKeys("b")

	// Add small delay for processing
	time.Sleep(50 * time.Millisecond)

	// Log current state after sending key
	t.Logf("Current screen after 'b' key:\n%s", harness.GetCurrentScreen())

	// Wait for state transition with detailed error
	err := harness.WaitForState(tui.StateBatch, 2*time.Second)
	if err != nil {
		t.Logf("Failed to transition to batch mode. Final screen:\n%s", harness.GetCurrentScreen())
		// Try to understand what state we're in
		for i := 0; i < 10; i++ {
			if stateErr := harness.WaitForState(tui.State(i), 100*time.Millisecond); stateErr == nil {
				t.Logf("Found current state: %d", i)
				break
			}
		}
	}
	require.NoError(t, err, "should transition to batch mode")

	// Verify batch mode is active
	screen := harness.GetCurrentScreen()
	require.Contains(t, screen, "Batch", "screen should show batch mode")
	t.Logf("Successfully entered batch mode. Screen:\n%s", screen)
}

// Test the specific workflow that's failing.
func TestBatchClassifyWorkflowDebug(t *testing.T) {
	// Create test data
	testTransactions := []model.Transaction{
		{
			ID:           "txn1",
			MerchantName: "Coffee Shop",
			Name:         "COFFEE SHOP #123",
			Amount:       5.50,
			Date:         time.Now().AddDate(0, 0, -1),
		},
		{
			ID:           "txn2",
			MerchantName: "Restaurant ABC",
			Name:         "RESTAURANT ABC",
			Amount:       25.00,
			Date:         time.Now().AddDate(0, 0, -2),
		},
		{
			ID:           "txn3",
			MerchantName: "Fast Food Place",
			Name:         "FAST FOOD PLACE",
			Amount:       8.99,
			Date:         time.Now().AddDate(0, 0, -3),
		},
	}

	testCategories := []model.Category{
		{Name: "Coffee"},
		{Name: "Restaurants"},
		{Name: "Fast Food"},
		{Name: "Dining Out"},
		{Name: "Other"},
	}

	harness := integration.NewHarness(t,
		integration.WithTestData(integration.TestData{
			Transactions: testTransactions,
			Categories:   testCategories,
		}),
	)
	defer harness.Cleanup()

	require.NoError(t, harness.Start())

	// Give UI time to initialize
	time.Sleep(200 * time.Millisecond)

	// Log initial state
	t.Logf("Initial screen:\n%s", harness.GetCurrentScreen())

	// Step 1: Press 'b' to enter batch mode
	t.Log("Step 1: Pressing 'b' to enter batch mode")
	harness.SendKeys("b")

	// Wait for batch mode with timeout
	err := harness.WaitForState(tui.StateBatch, 2*time.Second)
	if err != nil {
		screen := harness.GetCurrentScreen()
		t.Fatalf("Failed to enter batch mode: %v\nScreen:\n%s", err, screen)
	}

	t.Log("Successfully entered batch mode")

	// Step 2: Skip all transactions (to test basic batch functionality)
	t.Log("Step 2: Skipping all with 's'")
	harness.SendKeys("s")
	time.Sleep(100 * time.Millisecond)

	// Step 3: Confirm skip
	t.Log("Step 3: Confirming skip with 'y'")
	harness.SendKeys("y")
	time.Sleep(200 * time.Millisecond)

	// Verify results
	finalScreen := harness.GetCurrentScreen()
	t.Logf("Final screen:\n%s", finalScreen)

	// Check that classifications were saved as skipped
	storage := harness.GetStorage()
	for _, txn := range testTransactions {
		classification, ok := storage.GetClassification(txn.ID)
		require.True(t, ok, "transaction %s should be classified", txn.ID)
		require.Equal(t, model.StatusUnclassified, classification.Status, "transaction %s should be unclassified (skipped)", txn.ID)
	}
}

// Test accepting all with AI suggestions.
func TestBatchAcceptAllWorkflow(t *testing.T) {
	// Create test data
	testTransactions := []model.Transaction{
		{
			ID:           "txn1",
			MerchantName: "Coffee Shop",
			Name:         "COFFEE SHOP #123",
			Amount:       5.50,
			Date:         time.Now().AddDate(0, 0, -1),
		},
		{
			ID:           "txn2",
			MerchantName: "Restaurant ABC",
			Name:         "RESTAURANT ABC",
			Amount:       25.00,
			Date:         time.Now().AddDate(0, 0, -2),
		},
		{
			ID:           "txn3",
			MerchantName: "Fast Food Place",
			Name:         "FAST FOOD PLACE",
			Amount:       8.99,
			Date:         time.Now().AddDate(0, 0, -3),
		},
	}

	testCategories := []model.Category{
		{Name: "Coffee"},
		{Name: "Restaurants"},
		{Name: "Fast Food"},
		{Name: "Dining Out"},
		{Name: "Other"},
	}

	harness := integration.NewHarness(t,
		integration.WithTestData(integration.TestData{
			Transactions: testTransactions,
			Categories:   testCategories,
		}),
	)
	defer harness.Cleanup()

	// Set up AI suggestions for each transaction
	harness.GetClassifier().SetRankings("txn1", model.CategoryRankings{
		{Category: "Coffee", Score: 0.95},
		{Category: "Dining Out", Score: 0.80},
	})
	harness.GetClassifier().SetRankings("txn2", model.CategoryRankings{
		{Category: "Restaurants", Score: 0.90},
		{Category: "Dining Out", Score: 0.85},
	})
	harness.GetClassifier().SetRankings("txn3", model.CategoryRankings{
		{Category: "Fast Food", Score: 0.88},
		{Category: "Dining Out", Score: 0.75},
	})

	require.NoError(t, harness.Start())

	// Give UI time to initialize
	time.Sleep(200 * time.Millisecond)

	// Step 1: Press 'b' to enter batch mode
	t.Log("Step 1: Pressing 'b' to enter batch mode")
	harness.SendKeys("b")

	// Wait for batch mode with timeout
	err := harness.WaitForState(tui.StateBatch, 2*time.Second)
	require.NoError(t, err, "should enter batch mode")

	// Step 2: Accept all with 'a'
	t.Log("Step 2: Accepting all with 'a'")
	harness.SendKeys("a")
	time.Sleep(100 * time.Millisecond)

	// Step 3: Confirm with 'y'
	t.Log("Step 3: Confirming with 'y'")
	harness.SendKeys("y")
	time.Sleep(200 * time.Millisecond)

	// Verify results
	storage := harness.GetStorage()

	// Check txn1 - should be classified as Coffee
	classification, ok := storage.GetClassification("txn1")
	require.True(t, ok, "txn1 should be classified")
	require.Equal(t, "Coffee", classification.Category, "txn1 should be classified as Coffee")
	require.Equal(t, model.StatusClassifiedByAI, classification.Status)

	// Check txn2 - should be classified as Restaurants
	classification, ok = storage.GetClassification("txn2")
	require.True(t, ok, "txn2 should be classified")
	require.Equal(t, "Restaurants", classification.Category, "txn2 should be classified as Restaurants")
	require.Equal(t, model.StatusClassifiedByAI, classification.Status)

	// Check txn3 - should be classified as Fast Food
	classification, ok = storage.GetClassification("txn3")
	require.True(t, ok, "txn3 should be classified")
	require.Equal(t, "Fast Food", classification.Category, "txn3 should be classified as Fast Food")
	require.Equal(t, model.StatusClassifiedByAI, classification.Status)
}

// Test to understand state values.
func TestStateValues(t *testing.T) {
	states := []struct {
		name  string
		state tui.State
	}{
		{"StateList", tui.StateList},
		{"StateClassifying", tui.StateClassifying},
		{"StateBatch", tui.StateBatch},
		{"StateDirectionConfirm", tui.StateDirectionConfirm},
		{"StateExporting", tui.StateExporting},
		{"StateHelp", tui.StateHelp},
	}

	for _, s := range states {
		t.Logf("%s = %d", s.name, s.state)
	}

	// Verify StateBatch is 2
	require.Equal(t, tui.State(2), tui.StateBatch, "StateBatch should be value 2")
}
