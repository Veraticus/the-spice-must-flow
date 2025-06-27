package storage

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
)

func TestSQLiteStorage_VendorCache(t *testing.T) {
	store, cleanup := createTestStorageWithCategories(t, "Cat1", "Cat2", "Cat3")
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
	store, cleanup := createTestStorageWithCategories(t, "InitialCategory", "UpdatedCategory")
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
	store, cleanup := createTestStorageWithCategories(t, "TestCategory")
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
			Category:    "TestCategory",
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

	// Create required category
	if _, err := store.CreateCategory(ctx, "Test", "Test category description"); err != nil {
		t.Fatalf("Failed to create Test category: %v", err)
	}

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

	// Pre-create categories for concurrent test
	for i := 0; i < 5; i++ {
		categoryName := makeTestName("Category", i)
		if _, err := store.CreateCategory(ctx, categoryName, "Test description for "+categoryName); err != nil {
			t.Fatalf("Failed to create category %q: %v", categoryName, err)
		}
	}

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

	// Create required categories
	categories := []string{"Cat1", "Cat2", "Cat3", "Cat4"}
	for _, cat := range categories {
		if _, err := store.CreateCategory(ctx, cat, "Test description for "+cat); err != nil {
			t.Fatalf("Failed to create category %q: %v", cat, err)
		}
	}

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

// TestSQLiteStorage_DeleteVendorRaceCondition tests that vendor deletion is thread-safe.
func TestSQLiteStorage_DeleteVendorRaceCondition(t *testing.T) {
	store, cleanup := createTestStorage(t)
	defer cleanup()
	ctx := context.Background()

	// Create required categories
	categories := []string{"TestCategory", "UpdatedCategory"}
	for _, cat := range categories {
		if _, err := store.CreateCategory(ctx, cat, "Test description for "+cat); err != nil {
			t.Fatalf("Failed to create category %q: %v", cat, err)
		}
	}

	// Create multiple vendors
	vendorCount := 20
	for i := 0; i < vendorCount; i++ {
		vendor := &model.Vendor{
			Name:     makeTestName("RaceVendor", i),
			Category: "TestCategory",
			UseCount: i,
		}
		if err := store.SaveVendor(ctx, vendor); err != nil {
			t.Fatalf("Failed to save vendor: %v", err)
		}
	}

	// Warm the cache
	if err := store.WarmVendorCache(ctx); err != nil {
		t.Fatalf("Failed to warm cache: %v", err)
	}

	// Run concurrent operations
	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Goroutines that delete vendors
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			vendorName := makeTestName("RaceVendor", id)
			if err := store.DeleteVendor(ctx, vendorName); err != nil {
				errors <- err
			}
		}(i)
	}

	// Goroutines that read from cache
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				vendorName := makeTestName("RaceVendor", id%vendorCount)
				// This should not panic even if vendor is being deleted
				_ = store.getCachedVendor(vendorName)
			}
		}(i)
	}

	// Goroutines that save/update vendors
	for i := 10; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			vendor := &model.Vendor{
				Name:     makeTestName("RaceVendor", id),
				Category: "UpdatedCategory",
				UseCount: id * 2,
			}
			if err := store.SaveVendor(ctx, vendor); err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for any errors
	errorCount := 0
	for err := range errors {
		if err != nil {
			errorCount++
			t.Logf("Concurrent operation error: %v", err)
		}
	}

	// Some errors are expected (deleting non-existent vendors), but should be minimal
	if errorCount > vendorCount/2 {
		t.Errorf("Too many errors during concurrent operations: %d", errorCount)
	}

	// The test passes if we didn't panic (which would happen with the race condition)
	t.Logf("Successfully completed concurrent vendor operations without panic")
}

// Helper function to create a consistent test time.
func makeTestTime() time.Time {
	return time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
}

// TestSQLiteStorage_VendorSource tests vendor source tracking functionality.
func TestSQLiteStorage_VendorSource(t *testing.T) {
	store, cleanup := createTestStorageWithCategories(t, "TestCategory")
	defer cleanup()
	ctx := context.Background()

	tests := []struct {
		name        string
		vendor      *model.Vendor
		wantSource  model.VendorSource
		description string
	}{
		{
			name: "auto_vendor_default",
			vendor: &model.Vendor{
				Name:     "AutoVendor",
				Category: "TestCategory",
				UseCount: 0,
			},
			wantSource:  model.SourceAuto,
			description: "Vendor without source should default to AUTO",
		},
		{
			name: "manual_vendor",
			vendor: &model.Vendor{
				Name:     "ManualVendor",
				Category: "TestCategory",
				Source:   model.SourceManual,
				UseCount: 0,
			},
			wantSource:  model.SourceManual,
			description: "Vendor with MANUAL source should preserve it",
		},
		{
			name: "auto_confirmed_vendor",
			vendor: &model.Vendor{
				Name:     "ConfirmedVendor",
				Category: "TestCategory",
				Source:   model.SourceAutoConfirmed,
				UseCount: 10,
			},
			wantSource:  model.SourceAutoConfirmed,
			description: "Vendor with AUTO_CONFIRMED source should preserve it",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save vendor
			if err := store.SaveVendor(ctx, tt.vendor); err != nil {
				t.Fatalf("Failed to save vendor: %v", err)
			}

			// Retrieve vendor
			retrieved, err := store.GetVendor(ctx, tt.vendor.Name)
			if err != nil {
				t.Fatalf("Failed to get vendor: %v", err)
			}

			// Check source
			if retrieved.Source != tt.wantSource {
				t.Errorf("%s: got source %q, want %q", tt.description, retrieved.Source, tt.wantSource)
			}
		})
	}
}

// TestSQLiteStorage_GetVendorsBySource tests filtering vendors by source.
func TestSQLiteStorage_GetVendorsBySource(t *testing.T) {
	store, cleanup := createTestStorageWithCategories(t, "TestCategory")
	defer cleanup()
	ctx := context.Background()

	// Create vendors with different sources
	vendors := []*model.Vendor{
		{Name: "Auto1", Category: "TestCategory", Source: model.SourceAuto, UseCount: 1},
		{Name: "Auto2", Category: "TestCategory", Source: model.SourceAuto, UseCount: 2},
		{Name: "Manual1", Category: "TestCategory", Source: model.SourceManual, UseCount: 3},
		{Name: "Manual2", Category: "TestCategory", Source: model.SourceManual, UseCount: 4},
		{Name: "Confirmed1", Category: "TestCategory", Source: model.SourceAutoConfirmed, UseCount: 5},
	}

	// Save all vendors
	for _, v := range vendors {
		if err := store.SaveVendor(ctx, v); err != nil {
			t.Fatalf("Failed to save vendor %s: %v", v.Name, err)
		}
	}

	// Test filtering by source
	tests := []struct {
		source    model.VendorSource
		wantNames []string
		wantCount int
	}{
		{
			source:    model.SourceAuto,
			wantCount: 2,
			wantNames: []string{"Auto1", "Auto2"},
		},
		{
			source:    model.SourceManual,
			wantCount: 2,
			wantNames: []string{"Manual1", "Manual2"},
		},
		{
			source:    model.SourceAutoConfirmed,
			wantCount: 1,
			wantNames: []string{"Confirmed1"},
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.source), func(t *testing.T) {
			filtered, err := store.GetVendorsBySource(ctx, tt.source)
			if err != nil {
				t.Fatalf("Failed to get vendors by source: %v", err)
			}

			if len(filtered) != tt.wantCount {
				t.Errorf("Got %d vendors, want %d", len(filtered), tt.wantCount)
			}

			// Verify vendor names
			gotNames := make(map[string]bool)
			for _, v := range filtered {
				gotNames[v.Name] = true
			}

			for _, wantName := range tt.wantNames {
				if !gotNames[wantName] {
					t.Errorf("Expected vendor %s not found in results", wantName)
				}
			}
		})
	}
}

// TestSQLiteStorage_DeleteVendorsBySource tests deleting vendors by source.
func TestSQLiteStorage_DeleteVendorsBySource(t *testing.T) {
	store, cleanup := createTestStorageWithCategories(t, "TestCategory")
	defer cleanup()
	ctx := context.Background()

	// Create vendors with different sources
	vendors := []*model.Vendor{
		{Name: "AutoToDelete1", Category: "TestCategory", Source: model.SourceAuto, UseCount: 1},
		{Name: "AutoToDelete2", Category: "TestCategory", Source: model.SourceAuto, UseCount: 2},
		{Name: "ManualToKeep", Category: "TestCategory", Source: model.SourceManual, UseCount: 3},
		{Name: "ConfirmedToKeep", Category: "TestCategory", Source: model.SourceAutoConfirmed, UseCount: 4},
	}

	// Save all vendors
	for _, v := range vendors {
		if err := store.SaveVendor(ctx, v); err != nil {
			t.Fatalf("Failed to save vendor %s: %v", v.Name, err)
		}
	}

	// Delete all AUTO vendors
	if err := store.DeleteVendorsBySource(ctx, model.SourceAuto); err != nil {
		t.Fatalf("Failed to delete vendors by source: %v", err)
	}

	// Verify AUTO vendors are deleted
	autoVendors, err := store.GetVendorsBySource(ctx, model.SourceAuto)
	if err != nil {
		t.Fatalf("Failed to get AUTO vendors: %v", err)
	}
	if len(autoVendors) != 0 {
		t.Errorf("Expected 0 AUTO vendors after deletion, got %d", len(autoVendors))
	}

	// Verify other vendors still exist
	allVendors, err := store.GetAllVendors(ctx)
	if err != nil {
		t.Fatalf("Failed to get all vendors: %v", err)
	}
	if len(allVendors) != 2 {
		t.Errorf("Expected 2 vendors remaining, got %d", len(allVendors))
	}

	// Verify specific vendors still exist
	for _, name := range []string{"ManualToKeep", "ConfirmedToKeep"} {
		vendor, err := store.GetVendor(ctx, name)
		if err != nil {
			t.Errorf("Vendor %s should still exist: %v", name, err)
		}
		if vendor == nil {
			t.Errorf("Vendor %s not found after delete by source", name)
		}
	}

	// Verify cache is cleared
	if cached := store.getCachedVendor("AutoToDelete1"); cached != nil {
		t.Error("Deleted vendor still in cache")
	}
}

// TestSQLiteStorage_VendorSourceFromClassification tests vendor source when created from classification.
func TestSQLiteStorage_VendorSourceFromClassification(t *testing.T) {
	store, cleanup := createTestStorageWithCategories(t, "TestCategory")
	defer cleanup()
	ctx := context.Background()

	// Create and save transaction
	txn := model.Transaction{
		ID:           "test-txn-1",
		MerchantName: "NewMerchant",
		Name:         "NEW MERCHANT PURCHASE",
		Amount:       25.00,
		Date:         makeTestTime(),
		AccountID:    "acc1",
	}
	txn.Hash = txn.GenerateHash()

	if err := store.SaveTransactions(ctx, []model.Transaction{txn}); err != nil {
		t.Fatalf("Failed to save transaction: %v", err)
	}

	// Classify transaction (should create vendor with AUTO source)
	classification := &model.Classification{
		Transaction: txn,
		Category:    "TestCategory",
		Status:      model.StatusUserModified,
		Confidence:  0.95,
	}

	if err := store.SaveClassification(ctx, classification); err != nil {
		t.Fatalf("Failed to save classification: %v", err)
	}

	// Check vendor was created with AUTO source
	vendor, err := store.GetVendor(ctx, "NewMerchant")
	if err != nil {
		t.Fatalf("Failed to get vendor: %v", err)
	}
	if vendor == nil {
		t.Fatal("Vendor not created from classification")
	}
	if vendor.Source != model.SourceAuto {
		t.Errorf("Vendor source = %q, want %q", vendor.Source, model.SourceAuto)
	}
	if vendor.UseCount != 1 {
		t.Errorf("Vendor use count = %d, want 1", vendor.UseCount)
	}
}

func TestFindVendorMatch_ExactMatch(t *testing.T) {
	ctx := context.Background()
	store, cleanup := createTestStorage(t)
	defer cleanup()

	// Create a test category
	_, err := store.CreateCategory(ctx, "TestCategory", "Test category")
	if err != nil {
		t.Fatalf("Failed to create category: %v", err)
	}

	// Create an exact match vendor
	vendor := &model.Vendor{
		Name:     "EXACT MERCHANT NAME",
		Category: "TestCategory",
		Source:   model.SourceManual,
		IsRegex:  false,
	}
	err = store.SaveVendor(ctx, vendor)
	if err != nil {
		t.Fatalf("Failed to save vendor: %v", err)
	}

	// Test exact match
	match, err := store.FindVendorMatch(ctx, "EXACT MERCHANT NAME")
	if err != nil {
		t.Fatalf("Failed to find vendor match: %v", err)
	}
	if match == nil {
		t.Fatal("Expected vendor match, got nil")
	}
	if match.Name != "EXACT MERCHANT NAME" {
		t.Errorf("Expected vendor name %q, got %q", "EXACT MERCHANT NAME", match.Name)
	}
	if match.Category != "TestCategory" {
		t.Errorf("Expected category %q, got %q", "TestCategory", match.Category)
	}
	if match.IsRegex {
		t.Error("Expected IsRegex to be false")
	}
}

func TestFindVendorMatch_RegexMatch(t *testing.T) {
	ctx := context.Background()
	store, cleanup := createTestStorage(t)
	defer cleanup()

	// Create a test category
	_, err := store.CreateCategory(ctx, "PayrollCategory", "Payroll income")
	if err != nil {
		t.Fatalf("Failed to create category: %v", err)
	}

	// Create a regex vendor
	vendor := &model.Vendor{
		Name:     "PAYROLL.*COMPANY",
		Category: "PayrollCategory",
		Source:   model.SourceManual,
		IsRegex:  true,
	}
	err = store.SaveVendor(ctx, vendor)
	if err != nil {
		t.Fatalf("Failed to save vendor: %v", err)
	}

	// Test regex match
	testCases := []struct {
		merchantName string
		shouldMatch  bool
	}{
		{"PAYROLL FROM COMPANY", true},
		{"PAYROLL 12345 COMPANY", true},
		{"PAYROLL COMPANY", true},
		{"COMPANY PAYROLL", false},
		{"SOMETHING ELSE", false},
	}

	for _, tc := range testCases {
		t.Run(tc.merchantName, func(t *testing.T) {
			match, err := store.FindVendorMatch(ctx, tc.merchantName)
			if tc.shouldMatch {
				if err != nil {
					t.Fatalf("Failed to find vendor match: %v", err)
				}
				if match == nil {
					t.Fatal("Expected vendor match, got nil")
				}
				if match.Name != "PAYROLL.*COMPANY" {
					t.Errorf("Expected vendor name %q, got %q", "PAYROLL.*COMPANY", match.Name)
				}
				if match.Category != "PayrollCategory" {
					t.Errorf("Expected category %q, got %q", "PayrollCategory", match.Category)
				}
				if !match.IsRegex {
					t.Error("Expected IsRegex to be true")
				}
			} else {
				if err != sql.ErrNoRows {
					t.Errorf("Expected sql.ErrNoRows, got %v", err)
				}
				if match != nil {
					t.Errorf("Expected no match, got %v", match)
				}
			}
		})
	}
}

func TestFindVendorMatch_RegexPriority(t *testing.T) {
	ctx := context.Background()
	store, cleanup := createTestStorage(t)
	defer cleanup()

	// Create test categories
	_, err := store.CreateCategory(ctx, "HighPriority", "High priority category")
	if err != nil {
		t.Fatalf("Failed to create category: %v", err)
	}
	_, err = store.CreateCategory(ctx, "LowPriority", "Low priority category")
	if err != nil {
		t.Fatalf("Failed to create category: %v", err)
	}

	// Create two regex vendors with different use counts
	vendor1 := &model.Vendor{
		Name:     "AMAZON.*",
		Category: "HighPriority",
		Source:   model.SourceManual,
		IsRegex:  true,
		UseCount: 100,
	}
	err = store.SaveVendor(ctx, vendor1)
	if err != nil {
		t.Fatalf("Failed to save vendor1: %v", err)
	}

	vendor2 := &model.Vendor{
		Name:     ".*MARKETPLACE.*",
		Category: "LowPriority",
		Source:   model.SourceManual,
		IsRegex:  true,
		UseCount: 10,
	}
	err = store.SaveVendor(ctx, vendor2)
	if err != nil {
		t.Fatalf("Failed to save vendor2: %v", err)
	}

	// Test that higher use count regex is returned first
	match, err := store.FindVendorMatch(ctx, "AMAZON MARKETPLACE")
	if err != nil {
		t.Fatalf("Failed to find vendor match: %v", err)
	}
	if match == nil {
		t.Fatal("Expected vendor match, got nil")
	}
	if match.Name != "AMAZON.*" {
		t.Errorf("Expected vendor name %q, got %q", "AMAZON.*", match.Name)
	}
	if match.Category != "HighPriority" {
		t.Errorf("Expected category %q, got %q", "HighPriority", match.Category)
	}
}

func TestFindVendorMatch_InvalidRegex(t *testing.T) {
	ctx := context.Background()
	store, cleanup := createTestStorage(t)
	defer cleanup()

	// Create a test category
	_, err := store.CreateCategory(ctx, "TestCategory", "Test category")
	if err != nil {
		t.Fatalf("Failed to create category: %v", err)
	}

	// Create a vendor with invalid regex
	vendor := &model.Vendor{
		Name:     "[invalid(regex",
		Category: "TestCategory",
		Source:   model.SourceManual,
		IsRegex:  true,
	}
	err = store.SaveVendor(ctx, vendor)
	if err != nil {
		t.Fatalf("Failed to save vendor: %v", err)
	}

	// Test that invalid regex is skipped
	match, err := store.FindVendorMatch(ctx, "anything")
	if err != sql.ErrNoRows {
		t.Errorf("Expected sql.ErrNoRows, got %v", err)
	}
	if match != nil {
		t.Errorf("Expected no match, got %v", match)
	}
}

func TestVendorCRUD_WithRegex(t *testing.T) {
	ctx := context.Background()
	store, cleanup := createTestStorage(t)
	defer cleanup()

	// Create a test category
	_, err := store.CreateCategory(ctx, "TestCategory", "Test category")
	if err != nil {
		t.Fatalf("Failed to create category: %v", err)
	}

	// Create a regex vendor
	vendor := &model.Vendor{
		Name:     "TEST.*PATTERN",
		Category: "TestCategory",
		Source:   model.SourceManual,
		IsRegex:  true,
	}
	err = store.SaveVendor(ctx, vendor)
	if err != nil {
		t.Fatalf("Failed to save vendor: %v", err)
	}

	// Retrieve and verify
	retrieved, err := store.GetVendor(ctx, "TEST.*PATTERN")
	if err != nil {
		t.Fatalf("Failed to get vendor: %v", err)
	}
	if retrieved.Name != "TEST.*PATTERN" {
		t.Errorf("Expected vendor name %q, got %q", "TEST.*PATTERN", retrieved.Name)
	}
	if retrieved.Category != "TestCategory" {
		t.Errorf("Expected category %q, got %q", "TestCategory", retrieved.Category)
	}
	if !retrieved.IsRegex {
		t.Error("Expected IsRegex to be true")
	}

	// Update the vendor
	retrieved.Category = "TestCategory"
	retrieved.IsRegex = false
	err = store.SaveVendor(ctx, retrieved)
	if err != nil {
		t.Fatalf("Failed to update vendor: %v", err)
	}

	// Verify update
	updated, err := store.GetVendor(ctx, "TEST.*PATTERN")
	if err != nil {
		t.Fatalf("Failed to get updated vendor: %v", err)
	}
	if updated.IsRegex {
		t.Error("Expected IsRegex to be false after update")
	}

	// List all vendors
	vendors, err := store.GetAllVendors(ctx)
	if err != nil {
		t.Fatalf("Failed to get all vendors: %v", err)
	}
	if len(vendors) != 1 {
		t.Errorf("Expected 1 vendor, got %d", len(vendors))
	}
	if vendors[0].Name != "TEST.*PATTERN" {
		t.Errorf("Expected vendor name %q, got %q", "TEST.*PATTERN", vendors[0].Name)
	}
	if vendors[0].IsRegex {
		t.Error("Expected IsRegex to be false in list")
	}
}
