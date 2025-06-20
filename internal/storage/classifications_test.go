package storage

import (
	"context"
	"testing"
	"time"

	"github.com/joshsymonds/the-spice-must-flow/internal/model"
)

// Helper function to create test storage with categories.
func createTestStorageWithCategories(t *testing.T, categories ...string) (*SQLiteStorage, func()) {
	t.Helper()
	store, cleanup := createTestStorage(t)
	ctx := context.Background()

	// Seed categories
	for _, cat := range categories {
		if _, err := store.CreateCategory(ctx, cat); err != nil {
			cleanup()
			t.Fatalf("Failed to create category %q: %v", cat, err)
		}
	}

	return store, cleanup
}

func TestSQLiteStorage_ClassificationHistory(t *testing.T) {
	store, cleanup := createTestStorageWithCategories(t,
		"Initial Category",
		"User Corrected",
		"Final Category",
	)
	defer cleanup()
	ctx := context.Background()

	// Create and save a transaction
	txn := model.Transaction{
		ID:           "hist-test-1",
		Date:         time.Now(),
		Name:         "TEST TRANSACTION",
		MerchantName: "Test Merchant",
		Amount:       50.00,
		AccountID:    "acc1",
	}
	txn.Hash = txn.GenerateHash()

	if err := store.SaveTransactions(ctx, []model.Transaction{txn}); err != nil {
		t.Fatalf("Failed to save transaction: %v", err)
	}

	// Create multiple classifications for the same transaction
	// (simulating reclassification)
	classifications := []model.Classification{
		{
			Transaction: txn,
			Category:    "Initial Category",
			Status:      model.StatusClassifiedByAI,
			Confidence:  0.75,
		},
		{
			Transaction: txn,
			Category:    "User Corrected",
			Status:      model.StatusUserModified,
			Confidence:  1.0,
		},
		{
			Transaction: txn,
			Category:    "Final Category",
			Status:      model.StatusUserModified,
			Confidence:  1.0,
		},
	}

	// Save each classification
	for i, c := range classifications {
		if err := store.SaveClassification(ctx, &c); err != nil {
			t.Fatalf("Failed to save classification %d: %v", i, err)
		}
		// Add small delay to ensure different timestamps
		time.Sleep(10 * time.Millisecond)
	}

	// Verify transaction is no longer in unclassified list
	unclassified, err := store.GetTransactionsToClassify(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to get unclassified: %v", err)
	}

	for _, u := range unclassified {
		if u.ID == txn.ID {
			t.Error("Transaction still appears as unclassified after classification")
		}
	}

	// Get classifications by date range
	start := time.Now().Add(-1 * time.Hour)
	end := time.Now().Add(1 * time.Hour)

	results, err := store.GetClassificationsByDateRange(ctx, start, end)
	if err != nil {
		t.Fatalf("Failed to get classifications: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 classification (latest), got %d", len(results))
	}

	if results[0].Category != "Final Category" {
		t.Errorf("Expected latest classification, got category: %s", results[0].Category)
	}
}

func TestSQLiteStorage_ClassificationWithVendorRule(t *testing.T) {
	store, cleanup := createTestStorageWithCategories(t, "Subscription Services")
	defer cleanup()
	ctx := context.Background()

	// Create transactions from same merchant
	merchant := "RECURRING MERCHANT"
	transactions := []model.Transaction{
		{
			ID:           "vendor-rule-1",
			Date:         time.Now(),
			Name:         merchant + " #001",
			MerchantName: merchant,
			Amount:       25.00,
			AccountID:    "acc1",
		},
		{
			ID:           "vendor-rule-2",
			Date:         time.Now().Add(1 * time.Hour),
			Name:         merchant + " #002",
			MerchantName: merchant,
			Amount:       30.00,
			AccountID:    "acc1",
		},
	}

	// Generate hashes and save
	for i := range transactions {
		transactions[i].Hash = transactions[i].GenerateHash()
	}
	if err := store.SaveTransactions(ctx, transactions); err != nil {
		t.Fatalf("Failed to save transactions: %v", err)
	}

	// Classify first transaction (should create vendor rule)
	classification1 := &model.Classification{
		Transaction: transactions[0],
		Category:    "Subscription Services",
		Status:      model.StatusUserModified,
		Confidence:  1.0,
	}
	if err := store.SaveClassification(ctx, classification1); err != nil {
		t.Fatalf("Failed to save first classification: %v", err)
	}

	// Check vendor was created
	vendor, err := store.GetVendor(ctx, merchant)
	if err != nil {
		t.Fatalf("Failed to get vendor: %v", err)
	}
	if vendor == nil {
		t.Fatal("Vendor not created after user classification")
	}
	if vendor.Category != "Subscription Services" {
		t.Errorf("Vendor category = %s, want Subscription Services", vendor.Category)
	}
	if vendor.UseCount != 1 {
		t.Errorf("Vendor UseCount = %d, want 1", vendor.UseCount)
	}

	// Classify second transaction with same category
	classification2 := &model.Classification{
		Transaction: transactions[1],
		Category:    "Subscription Services",
		Status:      model.StatusClassifiedByRule,
		Confidence:  1.0,
	}
	if err := store.SaveClassification(ctx, classification2); err != nil {
		t.Fatalf("Failed to save second classification: %v", err)
	}

	// Check vendor use count increased
	vendor, err2 := store.GetVendor(ctx, merchant)
	if err2 != nil {
		t.Fatalf("Failed to get vendor after second classification: %v", err2)
	}
	if vendor.UseCount != 2 {
		t.Errorf("Vendor UseCount = %d, want 2", vendor.UseCount)
	}
}

func TestSQLiteStorage_ClassificationStatuses(t *testing.T) {
	store, cleanup := createTestStorageWithCategories(t,
		"AI Predicted",
		"User Selected",
		"Rule Applied",
	)
	defer cleanup()
	ctx := context.Background()

	// Create transactions
	transactions := createTestTransactions(3)
	if err := store.SaveTransactions(ctx, transactions); err != nil {
		t.Fatalf("Failed to save transactions: %v", err)
	}

	// Test different classification statuses
	tests := []struct {
		name       string
		status     model.ClassificationStatus
		category   string
		txnIndex   int
		confidence float64
	}{
		{
			name:       "AI classification",
			txnIndex:   0,
			status:     model.StatusClassifiedByAI,
			confidence: 0.85,
			category:   "AI Predicted",
		},
		{
			name:       "User classification",
			txnIndex:   1,
			status:     model.StatusUserModified,
			confidence: 1.0,
			category:   "User Selected",
		},
		{
			name:       "Rule-based classification",
			txnIndex:   2,
			status:     model.StatusClassifiedByRule,
			confidence: 1.0,
			category:   "Rule Applied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			classification := &model.Classification{
				Transaction: transactions[tt.txnIndex],
				Category:    tt.category,
				Status:      tt.status,
				Confidence:  tt.confidence,
			}

			if err := store.SaveClassification(ctx, classification); err != nil {
				t.Fatalf("Failed to save classification: %v", err)
			}

			// Verify classification was saved
			results, err := store.GetClassificationsByDateRange(ctx,
				time.Now().Add(-48*time.Hour), time.Now().Add(24*time.Hour))
			if err != nil {
				t.Fatalf("Failed to get classifications: %v", err)
			}

			// Find our classification
			var found bool
			for _, r := range results {
				if r.Transaction.ID == transactions[tt.txnIndex].ID {
					found = true
					if r.Status != tt.status {
						t.Errorf("Status = %v, want %v", r.Status, tt.status)
					}
					if r.Confidence != tt.confidence {
						t.Errorf("Confidence = %f, want %f", r.Confidence, tt.confidence)
					}
					if r.Category != tt.category {
						t.Errorf("Category = %s, want %s", r.Category, tt.category)
					}
					break
				}
			}
			if !found {
				t.Error("Classification not found in results")
			}
		})
	}
}

func TestSQLiteStorage_ClassificationDateFiltering(t *testing.T) {
	store, cleanup := createTestStorageWithCategories(t, "Test Category")
	defer cleanup()
	ctx := context.Background()

	baseTime := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)

	// Create transactions across different dates
	transactions := []model.Transaction{
		{
			ID:           "jan-1",
			Date:         time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
			Name:         "January 1st Transaction",
			MerchantName: "Merchant 1",
			Amount:       10.00,
			AccountID:    "acc1",
		},
		{
			ID:           "jan-15",
			Date:         baseTime,
			Name:         "January 15th Transaction",
			MerchantName: "Merchant 2",
			Amount:       20.00,
			AccountID:    "acc1",
		},
		{
			ID:           "jan-31",
			Date:         time.Date(2024, 1, 31, 18, 0, 0, 0, time.UTC),
			Name:         "January 31st Transaction",
			MerchantName: "Merchant 3",
			Amount:       30.00,
			AccountID:    "acc1",
		},
		{
			ID:           "feb-1",
			Date:         time.Date(2024, 2, 1, 9, 0, 0, 0, time.UTC),
			Name:         "February 1st Transaction",
			MerchantName: "Merchant 4",
			Amount:       40.00,
			AccountID:    "acc1",
		},
	}

	// Generate hashes and save
	for i := range transactions {
		transactions[i].Hash = transactions[i].GenerateHash()
	}
	if err := store.SaveTransactions(ctx, transactions); err != nil {
		t.Fatalf("Failed to save transactions: %v", err)
	}

	// Classify all transactions
	for i, txn := range transactions {
		classification := &model.Classification{
			Transaction: txn,
			Category:    "Test Category",
			Status:      model.StatusUserModified,
			Confidence:  1.0,
		}
		if err := store.SaveClassification(ctx, classification); err != nil {
			t.Fatalf("Failed to classify transaction %d: %v", i, err)
		}
	}

	tests := []struct {
		name    string
		start   time.Time
		end     time.Time
		wantIDs []string
	}{
		{
			name:    "entire January",
			start:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			end:     time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC),
			wantIDs: []string{"jan-1", "jan-15", "jan-31"},
		},
		{
			name:    "mid-January only",
			start:   time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC),
			end:     time.Date(2024, 1, 20, 0, 0, 0, 0, time.UTC),
			wantIDs: []string{"jan-15"},
		},
		{
			name:    "spanning months",
			start:   time.Date(2024, 1, 30, 0, 0, 0, 0, time.UTC),
			end:     time.Date(2024, 2, 2, 0, 0, 0, 0, time.UTC),
			wantIDs: []string{"jan-31", "feb-1"},
		},
		{
			name:    "no results in range",
			start:   time.Date(2023, 12, 1, 0, 0, 0, 0, time.UTC),
			end:     time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC),
			wantIDs: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := store.GetClassificationsByDateRange(ctx, tt.start, tt.end)
			if err != nil {
				t.Fatalf("Failed to get classifications: %v", err)
			}

			if len(results) != len(tt.wantIDs) {
				t.Errorf("Got %d results, want %d", len(results), len(tt.wantIDs))
			}

			// Check that we got the right transactions
			gotIDs := make(map[string]bool)
			for _, r := range results {
				gotIDs[r.Transaction.ID] = true
			}

			for _, wantID := range tt.wantIDs {
				if !gotIDs[wantID] {
					t.Errorf("Missing expected transaction ID: %s", wantID)
				}
			}
		})
	}
}

func TestSQLiteStorage_ClassificationAmountAggregation(t *testing.T) {
	store, cleanup := createTestStorageWithCategories(t,
		"Food & Dining",
		"Transportation",
		"Shopping",
	)
	defer cleanup()
	ctx := context.Background()

	// Create transactions with different amounts and categories
	transactions := []model.Transaction{
		{
			ID:           "food-1",
			Date:         time.Now(),
			Name:         "Restaurant 1",
			MerchantName: "Restaurant 1",
			Amount:       25.50,
			AccountID:    "acc1",
		},
		{
			ID:           "food-2",
			Date:         time.Now(),
			Name:         "Restaurant 2",
			MerchantName: "Restaurant 2",
			Amount:       30.25,
			AccountID:    "acc1",
		},
		{
			ID:           "transport-1",
			Date:         time.Now(),
			Name:         "Gas Station",
			MerchantName: "Gas Station",
			Amount:       45.00,
			AccountID:    "acc1",
		},
		{
			ID:           "shopping-1",
			Date:         time.Now(),
			Name:         "Online Store",
			MerchantName: "Online Store",
			Amount:       99.99,
			AccountID:    "acc1",
		},
	}

	// Generate hashes and save
	for i := range transactions {
		transactions[i].Hash = transactions[i].GenerateHash()
	}
	if err := store.SaveTransactions(ctx, transactions); err != nil {
		t.Fatalf("Failed to save transactions: %v", err)
	}

	// Classify with different categories
	classifications := []struct {
		category string
		txnIndex int
	}{
		{txnIndex: 0, category: "Food & Dining"},
		{txnIndex: 1, category: "Food & Dining"},
		{txnIndex: 2, category: "Transportation"},
		{txnIndex: 3, category: "Shopping"},
	}

	for _, c := range classifications {
		classification := &model.Classification{
			Transaction: transactions[c.txnIndex],
			Category:    c.category,
			Status:      model.StatusUserModified,
			Confidence:  1.0,
		}
		if err := store.SaveClassification(ctx, classification); err != nil {
			t.Fatalf("Failed to save classification: %v", err)
		}
	}

	// Get all classifications
	start := time.Now().Add(-1 * time.Hour)
	end := time.Now().Add(1 * time.Hour)
	results, err := store.GetClassificationsByDateRange(ctx, start, end)
	if err != nil {
		t.Fatalf("Failed to get classifications: %v", err)
	}

	// Aggregate by category
	categoryTotals := make(map[string]float64)
	categoryCounts := make(map[string]int)

	for _, r := range results {
		categoryTotals[r.Category] += r.Transaction.Amount
		categoryCounts[r.Category]++
	}

	// Verify aggregations
	expectedTotals := map[string]float64{
		"Food & Dining":  55.75, // 25.50 + 30.25
		"Transportation": 45.00,
		"Shopping":       99.99,
	}

	expectedCounts := map[string]int{
		"Food & Dining":  2,
		"Transportation": 1,
		"Shopping":       1,
	}

	for category, expectedTotal := range expectedTotals {
		if got := categoryTotals[category]; got != expectedTotal {
			t.Errorf("Category %s total = %.2f, want %.2f", category, got, expectedTotal)
		}
		if got := categoryCounts[category]; got != expectedCounts[category] {
			t.Errorf("Category %s count = %d, want %d", category, got, expectedCounts[category])
		}
	}
}
