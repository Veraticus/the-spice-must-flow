package storage

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
)

// Helper function to create test storage.
func createTestStorage(t *testing.T) (*SQLiteStorage, func()) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	ctx := context.Background()
	if err := store.Migrate(ctx); err != nil {
		_ = store.Close()
		t.Fatalf("Failed to migrate: %v", err)
	}

	return store, func() { _ = store.Close() }
}

// Helper function to create test transactions.
func createTestTransactions(count int) []model.Transaction {
	txns := make([]model.Transaction, count)
	baseTime := time.Now().Add(-24 * time.Hour)

	for i := 0; i < count; i++ {
		txns[i] = model.Transaction{
			ID:           makeTestID("txn", i+1),
			Date:         baseTime.Add(time.Duration(i) * time.Hour),
			Name:         makeTestName("Transaction", i+1),
			MerchantName: makeTestName("Merchant", (i%3)+1),
			Amount:       float64(i+1) * 10.50,
			AccountID:    "acc1",
			Category:     []string{"Food", "Restaurants"},
			Direction:    model.DirectionExpense,
		}
		txns[i].Hash = txns[i].GenerateHash()
	}
	return txns
}

func makeTestID(prefix string, num int) string {
	return prefix + "-" + time.Now().Format("20060102") + "-" + string(rune('A'+num-1))
}

func makeTestName(prefix string, num int) string {
	return prefix + " #" + string(rune('0'+num))
}

func TestSQLiteStorage_SaveTransactions(t *testing.T) {
	tests := []struct {
		setup        func(*SQLiteStorage, context.Context)
		validate     func(*testing.T, *SQLiteStorage, context.Context)
		name         string
		transactions []model.Transaction
		wantErr      bool
	}{
		{
			name:         "save new transactions",
			transactions: createTestTransactions(3),
			wantErr:      false,
			validate: func(t *testing.T, s *SQLiteStorage, ctx context.Context) {
				t.Helper()
				txns, err := s.GetTransactionsToClassify(ctx, nil)
				if err != nil {
					t.Errorf("Failed to get transactions: %v", err)
				}
				if len(txns) != 3 {
					t.Errorf("Expected 3 transactions, got %d", len(txns))
				}
			},
		},
		{
			name:         "handle duplicate transactions",
			transactions: createTestTransactions(2),
			setup: func(s *SQLiteStorage, ctx context.Context) {
				// Save the same transactions first
				txns := createTestTransactions(2)
				_ = s.SaveTransactions(ctx, txns)
			},
			wantErr: false,
			validate: func(t *testing.T, s *SQLiteStorage, ctx context.Context) {
				t.Helper()
				txns, err := s.GetTransactionsToClassify(ctx, nil)
				if err != nil {
					t.Errorf("Failed to get transactions: %v", err)
				}
				// Should still have only 2 transactions (no duplicates)
				if len(txns) != 2 {
					t.Errorf("Expected 2 transactions (no duplicates), got %d", len(txns))
				}
			},
		},
		{
			name:         "save empty list",
			transactions: []model.Transaction{},
			wantErr:      true, // Now we validate that empty slice is not allowed
		},
		{
			name: "save transactions with JSON categories",
			transactions: []model.Transaction{
				{
					ID:           "txn-json-1",
					Date:         time.Now(),
					Name:         "Test Transaction",
					MerchantName: "Test Merchant",
					Amount:       50.00,
					AccountID:    "acc1",
					Category:     []string{"Travel", "Airlines"},
					Direction:    model.DirectionExpense,
				},
			},
			wantErr: false,
			validate: func(t *testing.T, s *SQLiteStorage, ctx context.Context) {
				t.Helper()
				txn, err := s.GetTransactionByID(ctx, "txn-json-1")
				if err != nil {
					t.Fatalf("Failed to get transaction: %v", err)
				}
				if txn == nil {
					t.Fatal("Transaction is nil")
				}
				expectedCategories := []string{"Travel", "Airlines"}
				if len(txn.Category) != len(expectedCategories) {
					t.Errorf("Categories not preserved, expected %v, got %v", expectedCategories, txn.Category)
				} else {
					for i, cat := range expectedCategories {
						if txn.Category[i] != cat {
							t.Errorf("Category[%d] mismatch: expected %s, got %s", i, cat, txn.Category[i])
						}
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, cleanup := createTestStorage(t)
			defer cleanup()
			ctx := context.Background()

			if tt.setup != nil {
				tt.setup(store, ctx)
			}

			// Generate hashes for transactions
			for i := range tt.transactions {
				if tt.transactions[i].Hash == "" {
					tt.transactions[i].Hash = tt.transactions[i].GenerateHash()
				}
			}

			err := store.SaveTransactions(ctx, tt.transactions)
			if (err != nil) != tt.wantErr {
				t.Errorf("SaveTransactions() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.validate != nil {
				tt.validate(t, store, ctx)
			}
		})
	}
}

func TestSQLiteStorage_GetTransactionsToClassify(t *testing.T) {
	tests := []struct {
		setup    func(*SQLiteStorage, context.Context)
		fromDate *time.Time
		name     string
		wantLen  int
		wantErr  bool
	}{
		{
			name: "get all unclassified transactions",
			setup: func(s *SQLiteStorage, ctx context.Context) {
				txns := createTestTransactions(5)
				_ = s.SaveTransactions(ctx, txns)
			},
			fromDate: nil,
			wantLen:  5,
			wantErr:  false,
		},
		{
			name: "filter by date",
			setup: func(s *SQLiteStorage, ctx context.Context) {
				txns := createTestTransactions(5)
				_ = s.SaveTransactions(ctx, txns)
			},
			fromDate: func() *time.Time {
				t := time.Now().Add(-22 * time.Hour) // Include transactions from -21, -20, -19 hours
				return &t
			}(),
			wantLen: 3, // Transactions created at -21, -20, -19 hours from now
			wantErr: false,
		},
		{
			name: "exclude classified transactions",
			setup: func(s *SQLiteStorage, ctx context.Context) {
				txns := createTestTransactions(3)
				_ = s.SaveTransactions(ctx, txns)

				// Create category and classify one transaction
				_, _ = s.CreateCategory(ctx, "Food", "Food and dining expenses", model.CategoryTypeExpense)
				classification := &model.Classification{
					Transaction: txns[0],
					Category:    "Food",
					Status:      model.StatusUserModified,
					Confidence:  1.0,
				}
				_ = s.SaveClassification(ctx, classification)
			},
			fromDate: nil,
			wantLen:  2,
			wantErr:  false,
		},
		{
			name:     "empty database",
			setup:    nil,
			fromDate: nil,
			wantLen:  0,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, cleanup := createTestStorage(t)
			defer cleanup()
			ctx := context.Background()

			if tt.setup != nil {
				tt.setup(store, ctx)
			}

			got, err := store.GetTransactionsToClassify(ctx, tt.fromDate)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetTransactionsToClassify() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if len(got) != tt.wantLen {
				t.Errorf("GetTransactionsToClassify() returned %d transactions, want %d", len(got), tt.wantLen)
			}
		})
	}
}

func TestSQLiteStorage_GetTransactionByID(t *testing.T) {
	tests := []struct {
		setup   func(*SQLiteStorage, context.Context) string
		name    string
		wantNil bool
		wantErr bool
	}{
		{
			name: "find existing transaction",
			setup: func(s *SQLiteStorage, ctx context.Context) string {
				txns := createTestTransactions(1)
				_ = s.SaveTransactions(ctx, txns)
				return txns[0].ID
			},
			wantNil: false,
			wantErr: false,
		},
		{
			name: "transaction not found",
			setup: func(_ *SQLiteStorage, _ context.Context) string {
				return "non-existent-id"
			},
			wantNil: true,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, cleanup := createTestStorage(t)
			defer cleanup()
			ctx := context.Background()

			id := tt.setup(store, ctx)

			got, err := store.GetTransactionByID(ctx, id)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetTransactionByID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if (got == nil) != tt.wantNil {
				t.Errorf("GetTransactionByID() = %v, wantNil %v", got, tt.wantNil)
			}
		})
	}
}

func TestSQLiteStorage_VendorOperations(t *testing.T) {
	tests := []struct {
		vendor   *model.Vendor
		setup    func(*SQLiteStorage, context.Context)
		validate func(*testing.T, *SQLiteStorage, context.Context, *model.Vendor)
		name     string
		wantErr  bool
	}{
		{
			name: "save new vendor",
			vendor: &model.Vendor{
				Name:     "Starbucks",
				Category: "Coffee & Dining",
				UseCount: 1,
			},
			wantErr: false,
			validate: func(t *testing.T, s *SQLiteStorage, ctx context.Context, v *model.Vendor) {
				t.Helper()
				retrieved, err := s.GetVendor(ctx, v.Name)
				if err != nil {
					t.Errorf("Failed to get vendor: %v", err)
				}
				if retrieved == nil || retrieved.Category != v.Category {
					t.Errorf("Retrieved vendor doesn't match: got %+v, want category %s", retrieved, v.Category)
				}
			},
		},
		{
			name: "update existing vendor",
			vendor: &model.Vendor{
				Name:     "Amazon",
				Category: "Online Shopping",
				UseCount: 5,
			},
			setup: func(s *SQLiteStorage, ctx context.Context) {
				// Save initial vendor
				initial := &model.Vendor{
					Name:     "Amazon",
					Category: "Shopping",
					UseCount: 1,
				}
				_ = s.SaveVendor(ctx, initial)
			},
			wantErr: false,
			validate: func(t *testing.T, s *SQLiteStorage, ctx context.Context, v *model.Vendor) {
				t.Helper()
				retrieved, err := s.GetVendor(ctx, v.Name)
				if err != nil {
					t.Errorf("Failed to get vendor: %v", err)
				}
				if retrieved != nil && (retrieved.Category != "Online Shopping" || retrieved.UseCount != 5) {
					t.Errorf("Vendor not updated: got %+v", retrieved)
				}
			},
		},
		{
			name: "vendor caching",
			vendor: &model.Vendor{
				Name:     "CachedVendor",
				Category: "Test",
				UseCount: 1,
			},
			wantErr: false,
			validate: func(t *testing.T, s *SQLiteStorage, ctx context.Context, v *model.Vendor) {
				t.Helper()
				// First retrieval (from DB)
				_, err := s.GetVendor(ctx, v.Name)
				if err != nil {
					t.Errorf("Failed to get vendor: %v", err)
				}

				// Check cache
				cached := s.getCachedVendor(v.Name)
				if cached == nil {
					t.Error("Vendor not cached after retrieval")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, cleanup := createTestStorage(t)
			defer cleanup()
			ctx := context.Background()

			// Seed required categories for vendor tests
			categories := []string{"Coffee & Dining", "Online Shopping", "Shopping", "Test"}
			for _, cat := range categories {
				if _, err := store.CreateCategory(ctx, cat, "Test description for "+cat, model.CategoryTypeExpense); err != nil {
					t.Fatalf("Failed to create category %q: %v", cat, err)
				}
			}

			if tt.setup != nil {
				tt.setup(store, ctx)
			}

			err := store.SaveVendor(ctx, tt.vendor)
			if (err != nil) != tt.wantErr {
				t.Errorf("SaveVendor() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.validate != nil {
				tt.validate(t, store, ctx, tt.vendor)
			}
		})
	}
}

func TestSQLiteStorage_GetAllVendors(t *testing.T) {
	tests := []struct {
		setup   func(*SQLiteStorage, context.Context)
		name    string
		wantLen int
		wantErr bool
	}{
		{
			name: "get multiple vendors",
			setup: func(s *SQLiteStorage, ctx context.Context) {
				// Create categories first
				_, _ = s.CreateCategory(ctx, "Cat1", "Description for Cat1", model.CategoryTypeExpense)
				_, _ = s.CreateCategory(ctx, "Cat2", "Description for Cat2", model.CategoryTypeExpense)
				_, _ = s.CreateCategory(ctx, "Cat3", "Description for Cat3", model.CategoryTypeExpense)

				vendors := []*model.Vendor{
					{Name: "Vendor1", Category: "Cat1", UseCount: 1},
					{Name: "Vendor2", Category: "Cat2", UseCount: 2},
					{Name: "Vendor3", Category: "Cat3", UseCount: 3},
				}
				for _, v := range vendors {
					_ = s.SaveVendor(ctx, v)
				}
			},
			wantLen: 3,
			wantErr: false,
		},
		{
			name:    "empty database",
			setup:   nil,
			wantLen: 0,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, cleanup := createTestStorage(t)
			defer cleanup()
			ctx := context.Background()

			if tt.setup != nil {
				tt.setup(store, ctx)
			}

			got, err := store.GetAllVendors(ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetAllVendors() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if len(got) != tt.wantLen {
				t.Errorf("GetAllVendors() returned %d vendors, want %d", len(got), tt.wantLen)
			}
		})
	}
}

func TestSQLiteStorage_ClassificationOperations(t *testing.T) {
	tests := []struct {
		classification *model.Classification
		setup          func(*SQLiteStorage, context.Context) *model.Transaction
		validate       func(*testing.T, *SQLiteStorage, context.Context)
		name           string
		wantErr        bool
	}{
		{
			name: "save classification with vendor rule",
			classification: &model.Classification{
				Category:   "Food & Dining",
				Status:     model.StatusUserModified,
				Confidence: 1.0,
			},
			setup: func(s *SQLiteStorage, ctx context.Context) *model.Transaction {
				txn := createTestTransactions(1)[0]
				_ = s.SaveTransactions(ctx, []model.Transaction{txn})
				return &txn
			},
			wantErr: false,
			validate: func(t *testing.T, s *SQLiteStorage, ctx context.Context) {
				t.Helper()
				// Check that vendor was created/updated
				vendors, _ := s.GetAllVendors(ctx)
				if len(vendors) != 1 {
					t.Errorf("Expected 1 vendor to be created, got %d", len(vendors))
				}
			},
		},
		{
			name: "save AI classification",
			classification: &model.Classification{
				Category:   "Transportation",
				Status:     model.StatusClassifiedByAI,
				Confidence: 0.85,
			},
			setup: func(s *SQLiteStorage, ctx context.Context) *model.Transaction {
				txn := createTestTransactions(1)[0]
				_ = s.SaveTransactions(ctx, []model.Transaction{txn})
				return &txn
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, cleanup := createTestStorage(t)
			defer cleanup()
			ctx := context.Background()

			// Seed required categories for this test
			if _, err := store.CreateCategory(ctx, "Food & Dining", "Food and dining expenses", model.CategoryTypeExpense); err != nil {
				t.Fatalf("Failed to create Food & Dining category: %v", err)
			}
			if _, err := store.CreateCategory(ctx, "Transportation", "Transportation expenses", model.CategoryTypeExpense); err != nil {
				t.Fatalf("Failed to create Transportation category: %v", err)
			}

			txn := tt.setup(store, ctx)
			tt.classification.Transaction = *txn

			err := store.SaveClassification(ctx, tt.classification)
			if (err != nil) != tt.wantErr {
				t.Errorf("SaveClassification() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.validate != nil {
				tt.validate(t, store, ctx)
			}
		})
	}
}

func TestSQLiteStorage_GetClassificationsByDateRange(t *testing.T) {
	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)
	tomorrow := now.Add(24 * time.Hour)

	tests := []struct {
		start   time.Time
		end     time.Time
		setup   func(*SQLiteStorage, context.Context)
		name    string
		wantLen int
		wantErr bool
	}{
		{
			name: "get classifications in range",
			setup: func(s *SQLiteStorage, ctx context.Context) {
				// Create transactions across different dates
				txns := []model.Transaction{
					{
						ID:           "old-txn",
						Date:         yesterday.Add(-48 * time.Hour),
						Name:         "Old Transaction",
						MerchantName: "Old Merchant",
						Amount:       10.00,
						AccountID:    "acc1",
						Direction:    model.DirectionExpense,
					},
					{
						ID:           "recent-txn-1",
						Date:         now,
						Name:         "Recent Transaction 1",
						MerchantName: "Recent Merchant",
						Amount:       20.00,
						AccountID:    "acc1",
						Direction:    model.DirectionExpense,
					},
					{
						ID:           "recent-txn-2",
						Date:         now.Add(1 * time.Hour),
						Name:         "Recent Transaction 2",
						MerchantName: "Recent Merchant",
						Amount:       30.00,
						AccountID:    "acc1",
						Direction:    model.DirectionExpense,
					},
				}

				// Generate hashes and save
				for i := range txns {
					txns[i].Hash = txns[i].GenerateHash()
				}
				_ = s.SaveTransactions(ctx, txns)

				// Create category and classify all transactions
				_, _ = s.CreateCategory(ctx, "Test Category", "Test category description", model.CategoryTypeExpense)
				for _, txn := range txns {
					classification := &model.Classification{
						Transaction: txn,
						Category:    "Test Category",
						Status:      model.StatusUserModified,
						Confidence:  1.0,
					}
					_ = s.SaveClassification(ctx, classification)
				}
			},
			start:   yesterday,
			end:     tomorrow,
			wantLen: 2, // Only the recent transactions
			wantErr: false,
		},
		{
			name:    "empty date range",
			setup:   nil,
			start:   yesterday,
			end:     tomorrow,
			wantLen: 0,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, cleanup := createTestStorage(t)
			defer cleanup()
			ctx := context.Background()

			if tt.setup != nil {
				tt.setup(store, ctx)
			}

			got, err := store.GetClassificationsByDateRange(ctx, tt.start, tt.end)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetClassificationsByDateRange() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if len(got) != tt.wantLen {
				t.Errorf("GetClassificationsByDateRange() returned %d classifications, want %d", len(got), tt.wantLen)
			}
		})
	}
}

func TestSQLiteStorage_ProgressTracking(t *testing.T) {
	tests := []struct {
		progress *model.ClassificationProgress
		setup    func(*SQLiteStorage, context.Context)
		validate func(*testing.T, *SQLiteStorage, context.Context, *model.ClassificationProgress)
		name     string
		wantErr  bool
	}{
		{
			name: "save new progress",
			progress: &model.ClassificationProgress{
				LastProcessedID:   "txn123",
				LastProcessedDate: time.Now(),
				TotalProcessed:    42,
				StartedAt:         time.Now().Add(-10 * time.Minute),
			},
			wantErr: false,
			validate: func(t *testing.T, s *SQLiteStorage, ctx context.Context, p *model.ClassificationProgress) {
				t.Helper()
				retrieved, err := s.GetLatestProgress(ctx)
				if err != nil {
					t.Errorf("Failed to get progress: %v", err)
				}
				if retrieved == nil || retrieved.TotalProcessed != p.TotalProcessed {
					t.Errorf("Retrieved progress doesn't match: got %+v, want TotalProcessed=%d",
						retrieved, p.TotalProcessed)
				}
			},
		},
		{
			name: "update existing progress",
			progress: &model.ClassificationProgress{
				LastProcessedID:   "txn456",
				LastProcessedDate: time.Now(),
				TotalProcessed:    100,
				StartedAt:         time.Now().Add(-30 * time.Minute),
			},
			setup: func(s *SQLiteStorage, ctx context.Context) {
				// Save initial progress
				initial := &model.ClassificationProgress{
					LastProcessedID:   "txn123",
					LastProcessedDate: time.Now().Add(-1 * time.Hour),
					TotalProcessed:    42,
					StartedAt:         time.Now().Add(-2 * time.Hour),
				}
				_ = s.SaveProgress(ctx, initial)
			},
			wantErr: false,
			validate: func(t *testing.T, s *SQLiteStorage, ctx context.Context, _ *model.ClassificationProgress) {
				t.Helper()
				retrieved, err := s.GetLatestProgress(ctx)
				if err != nil {
					t.Errorf("Failed to get progress: %v", err)
				}
				if retrieved.TotalProcessed != 100 || retrieved.LastProcessedID != "txn456" {
					t.Errorf("Progress not updated: got %+v", retrieved)
				}
			},
		},
		{
			name: "clear progress",
			setup: func(s *SQLiteStorage, ctx context.Context) {
				// Save initial progress
				progress := &model.ClassificationProgress{
					LastProcessedID:   "txn123",
					LastProcessedDate: time.Now(),
					TotalProcessed:    42,
					StartedAt:         time.Now().Add(-10 * time.Minute),
				}
				_ = s.SaveProgress(ctx, progress)
				_ = s.ClearProgress(ctx)
			},
			wantErr: false,
			validate: func(t *testing.T, s *SQLiteStorage, ctx context.Context, _ *model.ClassificationProgress) {
				t.Helper()
				retrieved, err := s.GetLatestProgress(ctx)
				if err != sql.ErrNoRows {
					t.Errorf("Expected sql.ErrNoRows after clear, got: %v", err)
				}
				if retrieved != nil {
					t.Errorf("Expected nil progress after clear, got %+v", retrieved)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, cleanup := createTestStorage(t)
			defer cleanup()
			ctx := context.Background()

			if tt.setup != nil {
				tt.setup(store, ctx)
			}

			if tt.progress != nil {
				err := store.SaveProgress(ctx, tt.progress)
				if (err != nil) != tt.wantErr {
					t.Errorf("SaveProgress() error = %v, wantErr %v", err, tt.wantErr)
				}
			}

			if tt.validate != nil {
				tt.validate(t, store, ctx, tt.progress)
			}
		})
	}
}

func TestSQLiteStorage_Transaction(t *testing.T) {
	tests := []struct {
		txFunc  func(context.Context, *SQLiteStorage) error
		name    string
		wantErr bool
	}{
		{
			name: "successful transaction",
			txFunc: func(ctx context.Context, s *SQLiteStorage) error {
				tx, err := s.BeginTx(ctx)
				if err != nil {
					return err
				}

				// Perform operations within transaction
				txns := createTestTransactions(1)
				if err := tx.SaveTransactions(ctx, txns); err != nil {
					_ = tx.Rollback()
					return err
				}

				return tx.Commit()
			},
			wantErr: false,
		},
		{
			name: "rollback on error",
			txFunc: func(ctx context.Context, s *SQLiteStorage) error {
				tx, err := s.BeginTx(ctx)
				if err != nil {
					return err
				}
				defer func() { _ = tx.Rollback() }()

				// This should cause an error (invalid transaction)
				txns := []model.Transaction{
					{
						ID: "", // Empty ID should cause error
					},
				}
				return tx.SaveTransactions(ctx, txns)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, cleanup := createTestStorage(t)
			defer cleanup()
			ctx := context.Background()

			err := tt.txFunc(ctx, store)
			if (err != nil) != tt.wantErr {
				t.Errorf("Transaction test error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSQLiteStorage_Migrations(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Test initial migration
	store1, err := NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	ctx := context.Background()
	if err2 := store1.Migrate(ctx); err2 != nil {
		t.Fatalf("Initial migration failed: %v", err2)
	}
	_ = store1.Close()

	// Test idempotency - running migrations again should not error
	store2, err := NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer func() { _ = store2.Close() }()

	if err := store2.Migrate(ctx); err != nil {
		t.Fatalf("Repeated migration failed: %v", err)
	}

	// Verify database is functional after migrations
	txns := createTestTransactions(1)
	if err := store2.SaveTransactions(ctx, txns); err != nil {
		t.Errorf("Database not functional after migration: %v", err)
	}
}

func TestSQLiteStorage_ConcurrentAccess(t *testing.T) {
	store, cleanup := createTestStorage(t)
	defer cleanup()
	ctx := context.Background()

	// Warm the vendor cache
	if err := store.WarmVendorCache(ctx); err != nil {
		t.Fatalf("Failed to warm vendor cache: %v", err)
	}

	// Test concurrent reads and writes
	done := make(chan bool)
	errors := make(chan error, 10)

	// Concurrent writers
	for i := 0; i < 5; i++ {
		go func(id int) {
			txn := model.Transaction{
				ID:           makeTestID("concurrent", id),
				Date:         time.Now(),
				Name:         makeTestName("Concurrent", id),
				MerchantName: "TestMerchant",
				Amount:       float64(id) * 10,
				AccountID:    "acc1",
				Direction:    model.DirectionExpense,
			}
			txn.Hash = txn.GenerateHash()

			if err := store.SaveTransactions(ctx, []model.Transaction{txn}); err != nil {
				errors <- err
			}
			done <- true
		}(i)
	}

	// Concurrent readers
	for i := 0; i < 5; i++ {
		go func() {
			if _, err := store.GetTransactionsToClassify(ctx, nil); err != nil {
				errors <- err
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	close(errors)
	for err := range errors {
		t.Errorf("Concurrent access error: %v", err)
	}
}
