package storage

import (
	"context"
	"testing"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
)

// TestMigration14_VendorSource tests the vendor source migration.
func TestMigration14_VendorSource(t *testing.T) {
	// This test validates that the migration properly sets source based on use_count
	// However, since migrations run automatically, we can't test the actual migration
	// Instead, we'll test that the source column exists and works correctly
	store, cleanup := createTestStorage(t)
	defer cleanup()
	ctx := context.Background()

	// Create test categories
	categories := []string{"TestCategory1", "TestCategory2"}
	for _, cat := range categories {
		if _, err := store.CreateCategory(ctx, cat, "Test category"); err != nil {
			t.Fatalf("Failed to create category %s: %v", cat, err)
		}
	}

	// Test that new vendors get AUTO source by default
	vendor := &model.Vendor{
		Name:     "TestVendor",
		Category: "TestCategory1",
		UseCount: 5,
	}
	if err := store.SaveVendor(ctx, vendor); err != nil {
		t.Fatalf("Failed to save vendor: %v", err)
	}

	retrieved, err := store.GetVendor(ctx, vendor.Name)
	if err != nil {
		t.Fatalf("Failed to get vendor: %v", err)
	}
	if retrieved.Source != model.SourceAuto {
		t.Errorf("New vendor has source %q, want %q", retrieved.Source, model.SourceAuto)
	}

	// Verify the source column was added and indexed
	var indexCount int
	err = store.db.QueryRow(`
		SELECT COUNT(*) FROM sqlite_master 
		WHERE type='index' AND name='idx_vendors_source'
	`).Scan(&indexCount)
	if err != nil {
		t.Fatalf("Failed to check index: %v", err)
	}
	if indexCount != 1 {
		t.Error("Source column index was not created")
	}
}

// TestMigration14_NewVendorDefaultSource tests that new vendors get AUTO source by default.
func TestMigration14_NewVendorDefaultSource(t *testing.T) {
	store, cleanup := createTestStorageWithCategories(t, "TestCategory")
	defer cleanup()
	ctx := context.Background()

	// Create a vendor without specifying source
	vendor := &model.Vendor{
		Name:     "NewVendorNoSource",
		Category: "TestCategory",
		UseCount: 0,
	}

	if err := store.SaveVendor(ctx, vendor); err != nil {
		t.Fatalf("Failed to save vendor: %v", err)
	}

	// Retrieve and check source
	retrieved, err := store.GetVendor(ctx, vendor.Name)
	if err != nil {
		t.Fatalf("Failed to get vendor: %v", err)
	}

	if retrieved.Source != model.SourceAuto {
		t.Errorf("New vendor has source %q, want %q", retrieved.Source, model.SourceAuto)
	}
}
