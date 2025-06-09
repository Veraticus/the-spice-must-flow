package storage

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	"github.com/joshsymonds/the-spice-must-flow/internal/model"
)

func TestSQLiteStorage_VendorCache(t *testing.T) {
	store, cleanup := createTestStorage(t)
	defer cleanup()
	ctx := context.Background()

	// Test cache warming
	vendors := []*model.Vendor{
		{Name: "Cached1", Category: "Cat1", UseCount: 10},
		{Name: "Cached2", Category: "Cat2", UseCount: 20},
		{Name: "Cached3", Category: "Cat3", UseCount: 30},
	}

	// Save vendors to database
	for _, v := range vendors {
		if err := store.SaveVendor(ctx, v); err != nil {
			t.Fatalf("Failed to save vendor: %v", err)
		}
	}

	// Clear cache (simulate fresh start)
	store.vendorCache = make(map[string]*model.Vendor)

	// Warm cache
	if err := store.WarmVendorCache(ctx); err != nil {
		t.Fatalf("Failed to warm cache: %v", err)
	}

	// Verify all vendors are cached
	for _, v := range vendors {
		cached := store.getCachedVendor(v.Name)
		if cached == nil {
			t.Errorf("Vendor %s not in cache after warming", v.Name)
		} else if cached.Category != v.Category {
			t.Errorf("Cached vendor %s has wrong category: got %s, want %s",
				v.Name, cached.Category, v.Category)
		}
	}
}

func TestSQLiteStorage_VendorCacheInvalidation(t *testing.T) {
	store, cleanup := createTestStorage(t)
	defer cleanup()
	ctx := context.Background()

	// Save initial vendor
	vendor := &model.Vendor{
		Name:     "TestVendor",
		Category: "InitialCategory",
		UseCount: 1,
	}
	if err := store.SaveVendor(ctx, vendor); err != nil {
		t.Fatalf("Failed to save vendor: %v", err)
	}

	// Get vendor (should cache it)
	cached, err := store.GetVendor(ctx, "TestVendor")
	if err != nil {
		t.Fatalf("Failed to get vendor: %v", err)
	}
	if cached.Category != "InitialCategory" {
		t.Errorf("Initial category wrong: %s", cached.Category)
	}

	// Update vendor
	vendor.Category = "UpdatedCategory"
	vendor.UseCount = 5
	if err := store.SaveVendor(ctx, vendor); err != nil {
		t.Fatalf("Failed to update vendor: %v", err)
	}

	// Cache should be updated
	cached = store.getCachedVendor("TestVendor")
	if cached == nil {
		t.Fatal("Vendor not in cache after update")
	}
	if cached.Category != "UpdatedCategory" {
		t.Errorf("Cache not invalidated: got %s, want UpdatedCategory", cached.Category)
	}
	if cached.UseCount != 5 {
		t.Errorf("Cache not invalidated: got UseCount %d, want 5", cached.UseCount)
	}
}

func TestSQLiteStorage_VendorUsageTracking(t *testing.T) {
	store, cleanup := createTestStorage(t)
	defer cleanup()
	ctx := context.Background()

	// Create transactions with different merchants
	transactions := []model.Transaction{
		{
			ID:           "txn1",
			MerchantName: "Starbucks",
			Name:         "STARBUCKS #123",
			Amount:       5.00,
		},
		{
			ID:           "txn2",
			MerchantName: "Starbucks",
			Name:         "STARBUCKS #456",
			Amount:       6.00,
		},
		{
			ID:           "txn3",
			MerchantName: "Amazon",
			Name:         "AMAZON.COM",
			Amount:       50.00,
		},
	}

	// Generate hashes and save transactions
	for i := range transactions {
		transactions[i].Date = makeTestTime()
		transactions[i].AccountID = "acc1"
		transactions[i].Hash = transactions[i].GenerateHash()
	}
	if err := store.SaveTransactions(ctx, transactions); err != nil {
		t.Fatalf("Failed to save transactions: %v", err)
	}

	// Classify transactions (should create/update vendors)
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

	// Check vendor usage counts
	starbucks, err := store.GetVendor(ctx, "Starbucks")
	if err != nil {
		t.Fatalf("Failed to get Starbucks vendor: %v", err)
	}
	if starbucks.UseCount != 2 {
		t.Errorf("Starbucks UseCount = %d, want 2", starbucks.UseCount)
	}

	amazon, err := store.GetVendor(ctx, "Amazon")
	if err != nil {
		t.Fatalf("Failed to get Amazon vendor: %v", err)
	}
	if amazon.UseCount != 1 {
		t.Errorf("Amazon UseCount = %d, want 1", amazon.UseCount)
	}
}

func TestSQLiteStorage_DeleteVendor(t *testing.T) {
	store, cleanup := createTestStorage(t)
	defer cleanup()
	ctx := context.Background()

	// Create vendor
	vendor := &model.Vendor{
		Name:     "ToDelete",
		Category: "Test",
		UseCount: 5,
	}
	if err := store.SaveVendor(ctx, vendor); err != nil {
		t.Fatalf("Failed to save vendor: %v", err)
	}

	// Verify it exists
	found, err := store.GetVendor(ctx, "ToDelete")
	if err != nil {
		t.Fatalf("Failed to get vendor: %v", err)
	}
	if found == nil {
		t.Fatal("Vendor not found after save")
	}

	// Delete vendor
	if err2 := store.DeleteVendor(ctx, "ToDelete"); err2 != nil {
		t.Fatalf("Failed to delete vendor: %v", err2)
	}

	// Verify it's gone
	found, err = store.GetVendor(ctx, "ToDelete")
	if err != sql.ErrNoRows {
		t.Fatalf("Expected sql.ErrNoRows after delete, got: %v", err)
	}
	if found != nil {
		t.Error("Vendor still exists after delete")
	}

	// Verify cache is cleared
	if cached := store.getCachedVendor("ToDelete"); cached != nil {
		t.Error("Vendor still in cache after delete")
	}

	// Delete non-existent vendor should return ErrNotFound
	err = store.DeleteVendor(ctx, "NonExistent")
	if err == nil || err.Error() != "not found" {
		t.Errorf("Delete non-existent vendor should return ErrNotFound, got: %v", err)
	}
}

func TestSQLiteStorage_VendorConcurrency(t *testing.T) {
	store, cleanup := createTestStorage(t)
	defer cleanup()
	ctx := context.Background()

	// Test concurrent vendor operations
	numGoroutines := 10
	numOpsPerGoroutine := 5

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*numOpsPerGoroutine)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < numOpsPerGoroutine; j++ {
				vendorName := makeTestName("ConcurrentVendor", workerID)

				// Save/update vendor
				vendor := &model.Vendor{
					Name:     vendorName,
					Category: makeTestName("Category", j),
					UseCount: j + 1,
				}
				if err := store.SaveVendor(ctx, vendor); err != nil {
					errors <- err
					continue
				}

				// Get vendor
				retrieved, err := store.GetVendor(ctx, vendorName)
				if err != nil {
					errors <- err
					continue
				}
				if retrieved == nil {
					errors <- err
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Concurrent operation failed: %v", err)
	}

	// Verify final state
	allVendors, err := store.GetAllVendors(ctx)
	if err != nil {
		t.Fatalf("Failed to get all vendors: %v", err)
	}

	// Should have at most numGoroutines vendors (some may have same name)
	if len(allVendors) > numGoroutines {
		t.Errorf("Too many vendors created: got %d, want <= %d", len(allVendors), numGoroutines)
	}
}

func TestSQLiteStorage_VendorSorting(t *testing.T) {
	store, cleanup := createTestStorage(t)
	defer cleanup()
	ctx := context.Background()

	// Create vendors with different use counts
	vendors := []*model.Vendor{
		{Name: "LowUse", Category: "Cat1", UseCount: 1},
		{Name: "HighUse", Category: "Cat2", UseCount: 100},
		{Name: "MediumUse", Category: "Cat3", UseCount: 50},
		{Name: "NoUse", Category: "Cat4", UseCount: 0},
	}

	for _, v := range vendors {
		if err := store.SaveVendor(ctx, v); err != nil {
			t.Fatalf("Failed to save vendor %s: %v", v.Name, err)
		}
	}

	// Get all vendors
	allVendors, err := store.GetAllVendors(ctx)
	if err != nil {
		t.Fatalf("Failed to get all vendors: %v", err)
	}

	if len(allVendors) != 4 {
		t.Fatalf("Expected 4 vendors, got %d", len(allVendors))
	}

	// Verify they're sorted by name (alphabetically)
	expectedOrder := []string{"HighUse", "LowUse", "MediumUse", "NoUse"}
	for i, expected := range expectedOrder {
		if allVendors[i].Name != expected {
			t.Errorf("Position %d: expected %s, got %s", i, expected, allVendors[i].Name)
		}
	}
}

// Helper function to create a consistent test time.
func makeTestTime() time.Time {
	return time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
}
