package storage

import (
	"context"
	"testing"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
)

func TestTransaction_GenerateHash(t *testing.T) {
	tests := []struct {
		name     string
		txn1     model.Transaction
		txn2     model.Transaction
		wantSame bool
	}{
		{
			name: "identical transactions have same hash",
			txn1: model.Transaction{
				ID:        "txn1",
				Date:      time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Name:      "STARBUCKS",
				Amount:    5.25,
				AccountID: "acc1",
				Direction: model.DirectionExpense,
			},
			txn2: model.Transaction{
				ID:        "txn1",
				Date:      time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Name:      "STARBUCKS",
				Amount:    5.25,
				AccountID: "acc1",
				Direction: model.DirectionExpense,
			},
			wantSame: true,
		},
		{
			name: "different amounts produce different hashes",
			txn1: model.Transaction{
				ID:        "txn1",
				Date:      time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Name:      "STARBUCKS",
				Amount:    5.25,
				AccountID: "acc1",
				Direction: model.DirectionExpense,
			},
			txn2: model.Transaction{
				ID:        "txn1",
				Date:      time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Name:      "STARBUCKS",
				Amount:    6.25,
				AccountID: "acc1",
				Direction: model.DirectionExpense,
			},
			wantSame: false,
		},
		{
			name: "different dates produce different hashes",
			txn1: model.Transaction{
				ID:        "txn1",
				Date:      time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Name:      "STARBUCKS",
				Amount:    5.25,
				AccountID: "acc1",
				Direction: model.DirectionExpense,
			},
			txn2: model.Transaction{
				ID:        "txn1",
				Date:      time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC),
				Name:      "STARBUCKS",
				Amount:    5.25,
				AccountID: "acc1",
				Direction: model.DirectionExpense,
			},
			wantSame: false,
		},
		{
			name: "different merchant names produce different hashes",
			txn1: model.Transaction{
				ID:           "txn1",
				Date:         time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Name:         "STARBUCKS",
				MerchantName: "Starbucks",
				Amount:       5.25,
				AccountID:    "acc1",
				Direction:    model.DirectionExpense,
			},
			txn2: model.Transaction{
				ID:           "txn1",
				Date:         time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Name:         "STARBUCKS",
				MerchantName: "Coffee Shop",
				Amount:       5.25,
				AccountID:    "acc1",
				Direction:    model.DirectionExpense,
			},
			wantSame: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash1 := tt.txn1.GenerateHash()
			hash2 := tt.txn2.GenerateHash()

			if (hash1 == hash2) != tt.wantSame {
				t.Errorf("Hash comparison failed: hash1=%s, hash2=%s, wantSame=%v",
					hash1, hash2, tt.wantSame)
			}

			// Verify hash is consistent
			if hash1 != tt.txn1.GenerateHash() {
				t.Error("Hash generation is not consistent")
			}
		})
	}
}

func TestSQLiteStorage_TransactionDeduplication(t *testing.T) {
	store, cleanup := createTestStorage(t)
	defer cleanup()
	ctx := context.Background()

	baseTime := time.Now()

	// Create base transaction
	baseTxn := model.Transaction{
		ID:           "original",
		Date:         baseTime,
		Name:         "DUPLICATE TEST",
		MerchantName: "Test Merchant",
		Amount:       99.99,
		AccountID:    "acc1",
		Direction:    model.DirectionExpense,
	}
	baseTxn.Hash = baseTxn.GenerateHash()

	// First save
	if err := store.SaveTransactions(ctx, []model.Transaction{baseTxn}); err != nil {
		t.Fatalf("Failed to save initial transaction: %v", err)
	}

	// Verify it was saved
	txns, err := store.GetTransactionsToClassify(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to get transactions: %v", err)
	}
	if len(txns) != 1 {
		t.Errorf("Expected 1 transaction, got %d", len(txns))
	}

	// Try to save duplicate with different ID (should be skipped)
	dupTxn := baseTxn
	dupTxn.ID = "duplicate"

	if err2 := store.SaveTransactions(ctx, []model.Transaction{dupTxn}); err2 != nil {
		t.Fatalf("Failed to save duplicate transaction: %v", err2)
	}

	// Should still have only 1 transaction
	txns, err = store.GetTransactionsToClassify(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to get transactions: %v", err)
	}
	if len(txns) != 1 {
		t.Errorf("Expected 1 transaction after duplicate save, got %d", len(txns))
	}

	// Save a slightly different transaction (should be saved)
	diffTxn := baseTxn
	diffTxn.ID = "different"
	diffTxn.Amount = 100.00 // Different amount
	diffTxn.Hash = diffTxn.GenerateHash()

	if err2 := store.SaveTransactions(ctx, []model.Transaction{diffTxn}); err2 != nil {
		t.Fatalf("Failed to save different transaction: %v", err2)
	}

	// Should now have 2 transactions
	txns, err = store.GetTransactionsToClassify(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to get transactions: %v", err)
	}
	if len(txns) != 2 {
		t.Errorf("Expected 2 transactions after different save, got %d", len(txns))
	}
}

func TestSQLiteStorage_TransactionBatchOperations(t *testing.T) {
	store, cleanup := createTestStorage(t)
	defer cleanup()
	ctx := context.Background()

	// Test large batch insert
	batchSize := 100
	txns := make([]model.Transaction, batchSize)

	for i := 0; i < batchSize; i++ {
		txns[i] = model.Transaction{
			ID:           makeTestID("batch", i+1),
			Date:         time.Now().Add(time.Duration(i) * time.Minute),
			Name:         makeTestName("Batch Transaction", i+1),
			MerchantName: makeTestName("Batch Merchant", (i%10)+1),
			Amount:       float64(i+1) * 1.50,
			AccountID:    "acc1",
			Category:     []string{"Batch", "Test"},
			Direction:    model.DirectionExpense,
		}
		txns[i].Hash = txns[i].GenerateHash()
	}

	// Save batch
	start := time.Now()
	if err := store.SaveTransactions(ctx, txns); err != nil {
		t.Fatalf("Failed to save batch: %v", err)
	}
	duration := time.Since(start)

	// Performance check (should be fast even for 100 transactions)
	if duration > 500*time.Millisecond {
		t.Logf("Warning: Batch insert took %v, which may be slow", duration)
	}

	// Verify all were saved
	saved, err := store.GetTransactionsToClassify(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to get transactions: %v", err)
	}
	if len(saved) != batchSize {
		t.Errorf("Expected %d transactions, got %d", batchSize, len(saved))
	}
}

func TestSQLiteStorage_TransactionFiltering(t *testing.T) {
	store, cleanup := createTestStorage(t)
	defer cleanup()
	ctx := context.Background()

	now := time.Now()

	// Create transactions across different time periods
	transactions := []model.Transaction{
		{
			ID:           "old-1",
			Date:         now.Add(-72 * time.Hour),
			Name:         "Old Transaction 1",
			MerchantName: "Old Merchant",
			Amount:       10.00,
			AccountID:    "acc1",
			Direction:    model.DirectionExpense,
		},
		{
			ID:           "old-2",
			Date:         now.Add(-48 * time.Hour),
			Name:         "Old Transaction 2",
			MerchantName: "Old Merchant",
			Amount:       20.00,
			AccountID:    "acc1",
			Direction:    model.DirectionExpense,
		},
		{
			ID:           "recent-1",
			Date:         now.Add(-12 * time.Hour),
			Name:         "Recent Transaction 1",
			MerchantName: "Recent Merchant",
			Amount:       30.00,
			AccountID:    "acc1",
			Direction:    model.DirectionExpense,
		},
		{
			ID:           "recent-2",
			Date:         now.Add(-6 * time.Hour),
			Name:         "Recent Transaction 2",
			MerchantName: "Recent Merchant",
			Amount:       40.00,
			AccountID:    "acc1",
			Direction:    model.DirectionExpense,
		},
		{
			ID:           "current",
			Date:         now,
			Name:         "Current Transaction",
			MerchantName: "Current Merchant",
			Amount:       50.00,
			AccountID:    "acc1",
			Direction:    model.DirectionExpense,
		},
	}

	// Generate hashes and save
	for i := range transactions {
		transactions[i].Hash = transactions[i].GenerateHash()
	}
	if err := store.SaveTransactions(ctx, transactions); err != nil {
		t.Fatalf("Failed to save transactions: %v", err)
	}

	tests := []struct {
		fromDate *time.Time
		name     string
		wantLen  int
	}{
		{
			name:     "no filter returns all",
			fromDate: nil,
			wantLen:  5,
		},
		{
			name:     "filter last 24 hours",
			fromDate: func() *time.Time { t := now.Add(-24 * time.Hour); return &t }(),
			wantLen:  3,
		},
		{
			name:     "filter last 7 days",
			fromDate: func() *time.Time { t := now.Add(-7 * 24 * time.Hour); return &t }(),
			wantLen:  5,
		},
		{
			name:     "filter last hour",
			fromDate: func() *time.Time { t := now.Add(-1 * time.Hour); return &t }(),
			wantLen:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := store.GetTransactionsToClassify(ctx, tt.fromDate)
			if err != nil {
				t.Fatalf("Failed to get transactions: %v", err)
			}
			if len(got) != tt.wantLen {
				t.Errorf("Expected %d transactions, got %d", tt.wantLen, len(got))
			}
		})
	}
}

func TestSQLiteStorage_TransactionClassificationState(t *testing.T) {
	store, cleanup := createTestStorage(t)
	defer cleanup()
	ctx := context.Background()

	// Seed required categories
	categories := []string{"Food", "Transport", "Shopping"}
	for _, cat := range categories {
		if _, err := store.CreateCategory(ctx, cat, "Test description for "+cat, model.CategoryTypeExpense); err != nil {
			t.Fatalf("Failed to create category %q: %v", cat, err)
		}
	}

	// Create test transactions
	transactions := createTestTransactions(5)
	if err := store.SaveTransactions(ctx, transactions); err != nil {
		t.Fatalf("Failed to save transactions: %v", err)
	}

	// Initially all should be unclassified
	unclassified, err := store.GetTransactionsToClassify(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to get unclassified: %v", err)
	}
	if len(unclassified) != 5 {
		t.Errorf("Expected 5 unclassified, got %d", len(unclassified))
	}

	// Classify some transactions
	classifications := []struct {
		status   model.ClassificationStatus
		category string
		txnIndex int
	}{
		{txnIndex: 0, status: model.StatusUserModified, category: "Food"},
		{txnIndex: 1, status: model.StatusClassifiedByAI, category: "Transport"},
		{txnIndex: 2, status: model.StatusClassifiedByRule, category: "Shopping"},
	}

	for _, c := range classifications {
		classification := &model.Classification{
			Transaction: transactions[c.txnIndex],
			Category:    c.category,
			Status:      c.status,
			Confidence:  1.0,
		}
		if err2 := store.SaveClassification(ctx, classification); err2 != nil {
			t.Fatalf("Failed to save classification: %v", err2)
		}
	}

	// Now should have 2 unclassified
	unclassified, err = store.GetTransactionsToClassify(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to get unclassified: %v", err)
	}
	if len(unclassified) != 2 {
		t.Errorf("Expected 2 unclassified after classification, got %d", len(unclassified))
	}

	// Verify the unclassified ones are the right ones
	unclassifiedIDs := make(map[string]bool)
	for _, txn := range unclassified {
		unclassifiedIDs[txn.ID] = true
	}

	if !unclassifiedIDs[transactions[3].ID] || !unclassifiedIDs[transactions[4].ID] {
		t.Error("Wrong transactions remained unclassified")
	}
}
