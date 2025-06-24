package integration_test

import (
	"testing"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/tui/integration"
	"github.com/stretchr/testify/require"
)

// TestDebugCategories checks why AssertCategoryListVisible is failing.
func TestDebugCategories(t *testing.T) {
	testCategories := []model.Category{
		{ID: 1, Name: "Groceries"},
		{ID: 2, Name: "Shopping"},
	}

	harness := integration.NewHarness(t,
		integration.WithTestData(integration.TestData{
			Transactions: []model.Transaction{
				{ID: "txn_1", MerchantName: "Test", Amount: -50},
			},
			Categories: testCategories,
		}),
	)
	defer harness.Cleanup()

	require.NoError(t, harness.Start())

	// Wait for render
	time.Sleep(200 * time.Millisecond)

	// Press Enter to start classification
	harness.SendKeys("enter")

	// Wait for classification state
	err := harness.WaitForState(1, 2*time.Second)
	require.NoError(t, err)

	// Wait a bit more for UI to render
	time.Sleep(200 * time.Millisecond)

	screen := harness.GetCurrentScreen()
	t.Log("Screen content:")
	t.Log(screen)

	// Check harness test data
	t.Logf("Harness testData categories: %+v", harness.GetTestData().Categories)

	// Run the assertion
	assertions := integration.NewAssertions(harness)
	assertions.AssertCategoryListVisible(t)
}
