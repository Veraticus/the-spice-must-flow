package categories_test

import (
	"context"
	"testing"

	"github.com/joshsymonds/the-spice-must-flow/internal/testutil"
	"github.com/joshsymonds/the-spice-must-flow/internal/testutil/categories"
)

func TestBuilder_WithCategory(t *testing.T) {
	db := testutil.SetupTestDBWithBuilder(t, func(b categories.Builder) categories.Builder {
		return b.WithCategory(categories.CategoryGroceries)
	})

	// Verify category was created
	ctx := context.Background()
	cat, err := db.Storage.GetCategoryByName(ctx, "Groceries")
	if err != nil {
		t.Fatalf("failed to get category: %v", err)
	}
	if cat == nil {
		t.Fatal("expected category to exist")
	}
	if cat.Name != "Groceries" {
		t.Errorf("expected name %q, got %q", "Groceries", cat.Name)
	}
}

func TestBuilder_WithMultipleCategories(t *testing.T) {
	db := testutil.SetupTestDBWithBuilder(t, func(b categories.Builder) categories.Builder {
		return b.WithCategories(
			categories.CategoryGroceries,
			categories.CategoryFoodDining,
			categories.CategoryShopping,
		)
	})

	ctx := context.Background()
	cats, err := db.Storage.GetCategories(ctx)
	if err != nil {
		t.Fatalf("failed to get categories: %v", err)
	}

	if len(cats) != 3 {
		t.Errorf("expected 3 categories, got %d", len(cats))
	}

	// Verify all categories exist
	expected := []string{"Food & Dining", "Groceries", "Shopping"}
	for _, name := range expected {
		found := false
		for _, cat := range cats {
			if cat.Name == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected category %q not found", name)
		}
	}
}

func TestBuilder_WithBasicCategories(t *testing.T) {
	db := testutil.SetupTestDBWithBuilder(t, func(b categories.Builder) categories.Builder {
		return b.WithBasicCategories()
	})

	// Basic categories should include at least these
	requiredCategories := []categories.CategoryName{
		categories.CategoryGroceries,
		categories.CategoryFoodDining,
		categories.CategoryShopping,
		categories.CategoryTransportation,
	}

	for _, required := range requiredCategories {
		cat := db.Categories.Find(required)
		if cat == nil {
			t.Errorf("basic categories missing required category %q", required)
		}
	}
}

func TestBuilder_WithFixture(t *testing.T) {
	tests := []struct {
		fixture categories.Fixture
		name    string
		minSize int
	}{
		{
			name:    "minimal fixture",
			fixture: categories.FixtureMinimal,
			minSize: 3,
		},
		{
			name:    "standard fixture",
			fixture: categories.FixtureStandard,
			minSize: 10,
		},
		{
			name:    "comprehensive fixture",
			fixture: categories.FixtureComprehensive,
			minSize: 15,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := testutil.SetupTestDBWithBuilder(t, func(b categories.Builder) categories.Builder {
				return b.WithFixture(tt.fixture)
			})

			ctx := context.Background()
			cats, err := db.Storage.GetCategories(ctx)
			if err != nil {
				t.Fatalf("failed to get categories: %v", err)
			}

			if len(cats) < tt.minSize {
				t.Errorf("expected at least %d categories, got %d", tt.minSize, len(cats))
			}
		})
	}
}

func TestBuilder_ChainedOperations(t *testing.T) {
	db := testutil.SetupTestDBWithBuilder(t, func(b categories.Builder) categories.Builder {
		return b.
			WithBasicCategories().
			WithCategory("Custom Category 1").
			WithCategories("Custom Category 2", "Custom Category 3").
			WithFixture(categories.FixtureTestingOnly)
	})

	// Should have basic + custom + testing categories
	requiredCategories := []string{
		"Groceries",         // from basic
		"Custom Category 1", // custom
		"Custom Category 2", // custom
		"Test Category 1",   // from fixture
		"Initial Category",  // from fixture
	}

	ctx := context.Background()
	for _, name := range requiredCategories {
		cat, err := db.Storage.GetCategoryByName(ctx, name)
		if err != nil {
			t.Errorf("failed to get category %q: %v", name, err)
		}
		if cat == nil {
			t.Errorf("expected category %q to exist", name)
		}
	}
}

func TestCategories_Find(t *testing.T) {
	db := testutil.SetupTestDBWithBuilder(t, func(b categories.Builder) categories.Builder {
		return b.WithCategories(
			categories.CategoryGroceries,
			categories.CategoryFoodDining,
		)
	})

	// Test Find with existing category
	cat := db.Categories.Find(categories.CategoryGroceries)
	if cat == nil {
		t.Error("expected to find Groceries category")
	}
	if cat != nil && cat.Name != "Groceries" {
		t.Errorf("expected name %q, got %q", "Groceries", cat.Name)
	}

	// Test Find with non-existing category
	cat = db.Categories.Find("Non-existent")
	if cat != nil {
		t.Error("expected nil for non-existent category")
	}
}

func TestCategories_MustFind(t *testing.T) {
	db := testutil.SetupTestDBWithBuilder(t, func(b categories.Builder) categories.Builder {
		return b.WithCategory(categories.CategoryGroceries)
	})

	// Should not panic for existing category
	cat := db.Categories.MustFind(t, categories.CategoryGroceries)
	if cat.Name != "Groceries" {
		t.Errorf("expected name %q, got %q", "Groceries", cat.Name)
	}
}

func TestCategoryMap(t *testing.T) {
	ctx := context.Background()

	// Create a simple in-memory database for testing
	db := testutil.SetupTestDBWithBuilder(t, func(b categories.Builder) categories.Builder {
		return b.WithCategories(
			categories.CategoryGroceries,
			categories.CategoryFoodDining,
			categories.CategoryShopping,
		)
	})

	// Build a map using the builder
	builder := categories.NewBuilder(t).
		WithCategories(categories.CategoryGroceries, categories.CategoryFoodDining)

	catMap, err := builder.BuildMap(ctx, db.Storage)
	if err != nil {
		t.Fatalf("failed to build category map: %v", err)
	}

	// Test Get with existing category
	cat, ok := catMap.Get(categories.CategoryGroceries)
	if !ok {
		t.Error("expected to find Groceries in map")
	}
	if cat.Name != "Groceries" {
		t.Errorf("expected name %q, got %q", "Groceries", cat.Name)
	}

	// Test Get with non-existing category
	_, ok = catMap.Get("Non-existent")
	if ok {
		t.Error("expected false for non-existent category")
	}

	// Test MustGet
	cat = catMap.MustGet(t, categories.CategoryFoodDining)
	if cat.Name != "Food & Dining" {
		t.Errorf("expected name %q, got %q", "Food & Dining", cat.Name)
	}
}

func TestDuplicateCategories(t *testing.T) {
	// Test that duplicate categories don't cause errors
	db := testutil.SetupTestDBWithBuilder(t, func(b categories.Builder) categories.Builder {
		return b.
			WithCategory(categories.CategoryGroceries).
			WithCategory(categories.CategoryGroceries). // duplicate
			WithCategories(categories.CategoryGroceries, categories.CategoryFoodDining).
			WithCategory(categories.CategoryFoodDining) // another duplicate
	})

	ctx := context.Background()
	cats, err := db.Storage.GetCategories(ctx)
	if err != nil {
		t.Fatalf("failed to get categories: %v", err)
	}

	// Should only have 2 unique categories
	if len(cats) != 2 {
		t.Errorf("expected 2 unique categories, got %d", len(cats))
	}
}

func TestCompositeFixture(t *testing.T) {
	// Create a composite fixture
	composite := categories.NewCompositeFixture(
		"TestComposite",
		"Combines minimal and testing fixtures",
		categories.FixtureMinimal,
		categories.FixtureTestingOnly,
	)

	db := testutil.SetupTestDBWithBuilder(t, func(b categories.Builder) categories.Builder {
		return b.WithFixture(composite)
	})

	// Should have categories from both fixtures
	requiredCategories := []categories.CategoryName{
		categories.CategoryFoodDining,     // from minimal
		categories.CategoryTransportation, // from minimal
		categories.CategoryTest1,          // from testing
		categories.CategoryInitial,        // from testing
	}

	for _, required := range requiredCategories {
		cat := db.Categories.Find(required)
		if cat == nil {
			t.Errorf("composite fixture missing required category %q", required)
		}
	}
}
