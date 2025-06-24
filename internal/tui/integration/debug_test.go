package integration_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/tui/integration"
	"github.com/stretchr/testify/require"
)

// TestDebugStandaloneMode helps debug why transactions aren't loading.
func TestDebugStandaloneMode(t *testing.T) {
	testTransactions := []model.Transaction{
		{
			ID:           "txn_1",
			AccountID:    "acc1",
			MerchantName: "Test Store",
			Amount:       -50.00,
			Date:         time.Now(),
			Type:         "PURCHASE",
		},
	}

	testCategories := []model.Category{
		{ID: 1, Name: "Test Category"},
	}

	harness := integration.NewHarness(t,
		integration.WithTestData(integration.TestData{
			Transactions: testTransactions,
			Categories:   testCategories,
		}),
		// 		integration. // Explicitly set standalone mode
	)
	defer harness.Cleanup()

	// Check mock storage has data
	storage := harness.GetStorage()
	t.Logf("Mock storage transaction count: %d", len(testTransactions))

	// Check storage returns transactions
	ctx := context.Background()
	txns, err := storage.GetTransactionsToClassify(ctx, nil)
	require.NoError(t, err)
	t.Logf("GetTransactionsToClassify returned: %d transactions", len(txns))

	// Start the harness
	require.NoError(t, harness.Start())

	// Wait for data to load
	time.Sleep(500 * time.Millisecond)

	// Get the screen output
	screen := harness.GetCurrentScreen()
	t.Logf("Current screen:\n%s", screen)

	// Check what's displayed
	if strings.Contains(screen, "Waiting for transactions") {
		t.Error("TUI is in prompter mode - should be in standalone mode")
	}

	if !strings.Contains(screen, "Test Store") {
		t.Error("Transaction not visible - data may not be loading")
	}
}
