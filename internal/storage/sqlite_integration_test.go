package storage

import (
	"context"
	"testing"
	"time"

	"github.com/joshsymonds/the-spice-must-flow/internal/model"
)

func TestSQLiteStorage_FullWorkflow(t *testing.T) {
	store, cleanup := createTestStorage(t)
	defer cleanup()
	ctx := context.Background()

	// Step 1: Import transactions
	t.Log("Step 1: Importing transactions")
	transactions := []model.Transaction{
		{
			ID:            "workflow-1",
			Date:          time.Now().Add(-48 * time.Hour),
			Name:          "STARBUCKS STORE #1234",
			MerchantName:  "Starbucks",
			Amount:        5.75,
			AccountID:     "checking",
			PlaidCategory: "Food and Drink > Coffee Shop",
		},
		{
			ID:            "workflow-2",
			Date:          time.Now().Add(-36 * time.Hour),
			Name:          "AMAZON.COM PURCHASE",
			MerchantName:  "Amazon",
			Amount:        49.99,
			AccountID:     "credit",
			PlaidCategory: "Shops > Digital Purchase",
		},
		{
			ID:            "workflow-3",
			Date:          time.Now().Add(-24 * time.Hour),
			Name:          "STARBUCKS STORE #5678",
			MerchantName:  "Starbucks",
			Amount:        6.25,
			AccountID:     "checking",
			PlaidCategory: "Food and Drink > Coffee Shop",
		},
		{
			ID:            "workflow-4",
			Date:          time.Now().Add(-12 * time.Hour),
			Name:          "UBER TRIP",
			MerchantName:  "Uber",
			Amount:        15.50,
			AccountID:     "credit",
			PlaidCategory: "Travel > Taxi",
		},
	}

	// Generate hashes and save
	for i := range transactions {
		transactions[i].Hash = transactions[i].GenerateHash()
	}
	if err := store.SaveTransactions(ctx, transactions); err != nil {
		t.Fatalf("Failed to save transactions: %v", err)
	}

	// Step 2: Get unclassified transactions
	t.Log("Step 2: Getting unclassified transactions")
	unclassified, err := store.GetTransactionsToClassify(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to get unclassified: %v", err)
	}
	if len(unclassified) != 4 {
		t.Errorf("Expected 4 unclassified, got %d", len(unclassified))
	}

	// Step 3: Start classification session
	t.Log("Step 3: Starting classification session")
	progress := &model.ClassificationProgress{
		StartedAt:      time.Now(),
		TotalProcessed: 0,
	}
	if err2 := store.SaveProgress(ctx, progress); err2 != nil {
		t.Fatalf("Failed to save initial progress: %v", err2)
	}

	// Step 4: Classify first Starbucks transaction (user classification)
	t.Log("Step 4: User classifies first Starbucks transaction")
	classification := &model.Classification{
		Transaction: unclassified[0], // First Starbucks
		Category:    "Coffee & Dining",
		Status:      model.StatusUserModified,
		Confidence:  1.0,
	}
	if err2 := store.SaveClassification(ctx, classification); err2 != nil {
		t.Fatalf("Failed to save classification: %v", err2)
	}

	// Update progress
	progress.LastProcessedID = unclassified[0].ID
	progress.LastProcessedDate = unclassified[0].Date
	progress.TotalProcessed = 1
	if err2 := store.SaveProgress(ctx, progress); err2 != nil {
		t.Fatalf("Failed to update progress: %v", err2)
	}

	// Step 5: Check vendor was created
	t.Log("Step 5: Verifying vendor rule was created")
	vendor, err := store.GetVendor(ctx, "Starbucks")
	if err != nil {
		t.Fatalf("Failed to get vendor: %v", err)
	}
	if vendor == nil || vendor.Category != "Coffee & Dining" {
		t.Errorf("Vendor not created correctly: %+v", vendor)
	}

	// Step 6: Apply vendor rule to second Starbucks transaction
	t.Log("Step 6: Applying vendor rule to second Starbucks")
	// Find the second Starbucks transaction
	var secondStarbucks *model.Transaction
	for _, txn := range unclassified {
		if txn.ID == "workflow-3" {
			secondStarbucks = &txn
			break
		}
	}
	if secondStarbucks == nil {
		t.Fatal("Could not find second Starbucks transaction")
	}

	classification = &model.Classification{
		Transaction: *secondStarbucks,
		Category:    "Coffee & Dining",
		Status:      model.StatusClassifiedByRule,
		Confidence:  1.0,
	}
	if err2 := store.SaveClassification(ctx, classification); err2 != nil {
		t.Fatalf("Failed to save rule-based classification: %v", err2)
	}

	// Step 7: AI classifies remaining transactions
	t.Log("Step 7: AI classifies remaining transactions")
	aiClassifications := []struct {
		id         string
		category   string
		confidence float64
	}{
		{"workflow-2", "Shopping", 0.92},
		{"workflow-4", "Transportation", 0.88},
	}

	for _, ac := range aiClassifications {
		// Find transaction
		var txn *model.Transaction
		for _, t := range unclassified {
			if t.ID == ac.id {
				txn = &t
				break
			}
		}
		if txn == nil {
			continue
		}

		classification := &model.Classification{
			Transaction: *txn,
			Category:    ac.category,
			Status:      model.StatusClassifiedByAI,
			Confidence:  ac.confidence,
		}
		if err2 := store.SaveClassification(ctx, classification); err2 != nil {
			t.Fatalf("Failed to save AI classification: %v", err2)
		}
	}

	// Step 8: Generate report
	t.Log("Step 8: Generating report")
	start := time.Now().Add(-72 * time.Hour)
	end := time.Now().Add(24 * time.Hour)

	classifications, err := store.GetClassificationsByDateRange(ctx, start, end)
	if err != nil {
		t.Fatalf("Failed to get classifications: %v", err)
	}

	if len(classifications) != 4 {
		t.Errorf("Expected 4 classifications, got %d", len(classifications))
	}

	// Verify category distribution
	categoryStats := make(map[string]struct {
		count  int
		amount float64
	})

	for _, c := range classifications {
		stats := categoryStats[c.Category]
		stats.count++
		stats.amount += c.Transaction.Amount
		categoryStats[c.Category] = stats
	}

	// Verify expected categories
	expectedCategories := map[string]struct {
		count  int
		amount float64
	}{
		"Coffee & Dining": {count: 2, amount: 12.00}, // 5.75 + 6.25
		"Shopping":        {count: 1, amount: 49.99},
		"Transportation":  {count: 1, amount: 15.50},
	}

	for category, expected := range expectedCategories {
		got := categoryStats[category]
		if got.count != expected.count {
			t.Errorf("Category %s: got %d transactions, want %d",
				category, got.count, expected.count)
		}
		if got.amount != expected.amount {
			t.Errorf("Category %s: got amount %.2f, want %.2f",
				category, got.amount, expected.amount)
		}
	}

	// Step 9: Verify vendor stats
	t.Log("Step 9: Verifying vendor statistics")
	vendors, err := store.GetAllVendors(ctx)
	if err != nil {
		t.Fatalf("Failed to get vendors: %v", err)
	}

	// Should have vendors created from classifications
	if len(vendors) < 1 {
		t.Error("Expected at least 1 vendor")
	}

	// Check Starbucks usage count
	starbucksVendor, err := store.GetVendor(ctx, "Starbucks")
	if err != nil {
		t.Fatalf("Failed to get Starbucks vendor: %v", err)
	}
	if starbucksVendor.UseCount != 2 {
		t.Errorf("Starbucks UseCount = %d, want 2", starbucksVendor.UseCount)
	}

	// Step 10: Verify no unclassified transactions remain
	t.Log("Step 10: Verifying all transactions classified")
	remaining, err := store.GetTransactionsToClassify(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to get remaining: %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("Expected 0 unclassified transactions, got %d", len(remaining))
	}
}

func TestSQLiteStorage_ResumableSession(t *testing.T) {
	store, cleanup := createTestStorage(t)
	defer cleanup()
	ctx := context.Background()

	// Create transactions
	transactions := createTestTransactions(10)
	if err := store.SaveTransactions(ctx, transactions); err != nil {
		t.Fatalf("Failed to save transactions: %v", err)
	}

	// Start classification session
	startTime := time.Now()
	progress := &model.ClassificationProgress{
		StartedAt:      startTime,
		TotalProcessed: 0,
	}
	if err := store.SaveProgress(ctx, progress); err != nil {
		t.Fatalf("Failed to save progress: %v", err)
	}

	// Classify first 3 transactions
	for i := 0; i < 3; i++ {
		classification := &model.Classification{
			Transaction: transactions[i],
			Category:    "Test Category",
			Status:      model.StatusUserModified,
			Confidence:  1.0,
		}
		if err := store.SaveClassification(ctx, classification); err != nil {
			t.Fatalf("Failed to classify transaction %d: %v", i, err)
		}

		// Update progress
		progress.LastProcessedID = transactions[i].ID
		progress.LastProcessedDate = transactions[i].Date
		progress.TotalProcessed = i + 1
		if err := store.SaveProgress(ctx, progress); err != nil {
			t.Fatalf("Failed to update progress: %v", err)
		}
	}

	// Simulate interruption - get latest progress
	savedProgress, err := store.GetLatestProgress(ctx)
	if err != nil {
		t.Fatalf("Failed to get progress: %v", err)
	}

	if savedProgress.TotalProcessed != 3 {
		t.Errorf("Progress TotalProcessed = %d, want 3", savedProgress.TotalProcessed)
	}
	if savedProgress.LastProcessedID != transactions[2].ID {
		t.Errorf("Progress LastProcessedID = %s, want %s",
			savedProgress.LastProcessedID, transactions[2].ID)
	}

	// Resume session - get remaining transactions
	remaining, err := store.GetTransactionsToClassify(ctx, &savedProgress.LastProcessedDate)
	if err != nil {
		t.Fatalf("Failed to get remaining: %v", err)
	}

	// Should have 7 remaining (10 - 3)
	if len(remaining) != 7 {
		t.Errorf("Expected 7 remaining transactions, got %d", len(remaining))
	}

	// Verify we're starting from the right place
	if remaining[0].Date.Before(savedProgress.LastProcessedDate) {
		t.Error("Remaining transactions include already processed dates")
	}
}

func TestSQLiteStorage_ErrorRecovery(t *testing.T) {
	store, cleanup := createTestStorage(t)
	defer cleanup()
	ctx := context.Background()

	// Test transaction rollback on error
	tx, err := store.BeginTx(ctx)
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	// Save valid transaction
	validTxn := model.Transaction{
		ID:           "valid-txn",
		Date:         time.Now(),
		Name:         "Valid Transaction",
		MerchantName: "Valid Merchant",
		Amount:       50.00,
		AccountID:    "acc1",
	}
	validTxn.Hash = validTxn.GenerateHash()

	if err2 := tx.SaveTransactions(ctx, []model.Transaction{validTxn}); err2 != nil {
		t.Fatalf("Failed to save valid transaction: %v", err2)
	}

	// Try to save invalid transaction (should cause entire tx to fail)
	invalidTxn := model.Transaction{
		ID:   "", // Invalid: empty ID
		Date: time.Now(),
		Name: "Invalid",
	}

	if err2 := tx.SaveTransactions(ctx, []model.Transaction{invalidTxn}); err2 == nil {
		t.Error("Expected error for invalid transaction")
	}

	// Rollback
	if err2 := tx.Rollback(); err2 != nil {
		t.Fatalf("Failed to rollback: %v", err2)
	}

	// Verify nothing was saved
	txns, err := store.GetTransactionsToClassify(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to get transactions: %v", err)
	}
	if len(txns) != 0 {
		t.Errorf("Expected 0 transactions after rollback, got %d", len(txns))
	}
}

func TestSQLiteStorage_DataIntegrity(t *testing.T) {
	store, cleanup := createTestStorage(t)
	defer cleanup()
	ctx := context.Background()

	// Test that classification references are maintained
	txn := model.Transaction{
		ID:           "integrity-test",
		Date:         time.Now(),
		Name:         "Test Transaction",
		MerchantName: "Test Merchant",
		Amount:       100.00,
		AccountID:    "acc1",
	}
	txn.Hash = txn.GenerateHash()

	if err := store.SaveTransactions(ctx, []model.Transaction{txn}); err != nil {
		t.Fatalf("Failed to save transaction: %v", err)
	}

	// Save classification
	classification := &model.Classification{
		Transaction: txn,
		Category:    "Test Category",
		Status:      model.StatusUserModified,
		Confidence:  1.0,
	}
	if err := store.SaveClassification(ctx, classification); err != nil {
		t.Fatalf("Failed to save classification: %v", err)
	}

	// Get classification back
	classifications, err := store.GetClassificationsByDateRange(ctx,
		time.Now().Add(-1*time.Hour), time.Now().Add(1*time.Hour))
	if err != nil {
		t.Fatalf("Failed to get classifications: %v", err)
	}

	if len(classifications) != 1 {
		t.Fatalf("Expected 1 classification, got %d", len(classifications))
	}

	// Verify transaction data is intact
	got := classifications[0].Transaction
	if got.ID != txn.ID {
		t.Errorf("Transaction ID = %s, want %s", got.ID, txn.ID)
	}
	if got.MerchantName != txn.MerchantName {
		t.Errorf("MerchantName = %s, want %s", got.MerchantName, txn.MerchantName)
	}
	if got.Amount != txn.Amount {
		t.Errorf("Amount = %.2f, want %.2f", got.Amount, txn.Amount)
	}
}
