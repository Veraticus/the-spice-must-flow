// Package testutil provides sophisticated test utilities for the spice-must-flow project.
// It offers type-safe APIs, proper test isolation, and elegant abstractions for test data management.
package testutil

import (
	"context"
	"fmt"
	"testing"

	"github.com/joshsymonds/the-spice-must-flow/internal/service"
	"github.com/joshsymonds/the-spice-must-flow/internal/storage"
	"github.com/joshsymonds/the-spice-must-flow/internal/testutil/categories"
)

// TestDB represents a test database with associated test utilities.
type TestDB struct {
	Storage    service.Storage
	t          *testing.T
	Categories categories.Categories
}

// SetupTestDB creates a new in-memory test database with the specified categories.
// It automatically handles migrations and cleanup.
//
// Example:
//
//	db := testutil.SetupTestDB(t,
//		testutil.NewCategoryBuilder(t).
//			WithBasicCategories().
//			Build(),
//	)
func SetupTestDB(t *testing.T, cats categories.Categories) *TestDB {
	t.Helper()

	// Create in-memory SQLite storage
	storage, err := storage.NewSQLiteStorage(":memory:")
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	// Run migrations
	ctx := context.Background()
	if err := storage.Migrate(ctx); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Seed categories if provided
	if len(cats) > 0 {
		for _, cat := range cats {
			if _, err := storage.CreateCategory(ctx, cat.Name, cat.Description); err != nil {
				t.Fatalf("failed to seed category %q: %v", cat.Name, err)
			}
		}
	}

	// Register cleanup
	t.Cleanup(func() {
		storage.Close()
	})

	return &TestDB{
		Storage:    storage,
		Categories: cats,
		t:          t,
	}
}

// SetupTestDBWithBuilder creates a test database using a category builder.
// This is a convenience method that combines building and setup.
//
// Example:
//
//	db := testutil.SetupTestDBWithBuilder(t, func(b categories.Builder) categories.Builder {
//		return b.WithBasicCategories().WithCategory("Custom Category")
//	})
func SetupTestDBWithBuilder(t *testing.T, configure func(categories.Builder) categories.Builder) *TestDB {
	t.Helper()

	builder := categories.NewBuilder(t)
	if configure != nil {
		builder = configure(builder)
	}

	// Create in-memory SQLite storage
	storage, err := storage.NewSQLiteStorage(":memory:")
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	// Run migrations
	ctx := context.Background()
	if err := storage.Migrate(ctx); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Build categories
	cats, err := builder.Build(ctx, storage)
	if err != nil {
		t.Fatalf("failed to build categories: %v", err)
	}

	// Register cleanup
	t.Cleanup(func() {
		storage.Close()
	})

	return &TestDB{
		Storage:    storage,
		Categories: cats,
		t:          t,
	}
}

// MustGetCategory returns the category with the given name or fails the test.
func (db *TestDB) MustGetCategory(name categories.CategoryName) string {
	db.t.Helper()
	cat := db.Categories.MustFind(db.t, name)
	return cat.Name
}

// WithTransaction executes the given function within a database transaction.
// The transaction is automatically rolled back after the function completes.
func (db *TestDB) WithTransaction(fn func(tx service.Transaction) error) error {
	ctx := context.Background()
	tx, err := db.Storage.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer tx.Rollback()

	if err := fn(tx); err != nil {
		return err
	}

	return nil
}

// TestDBOptions provides configuration options for test database setup.
type TestDBOptions struct {
	CustomSetup    func(context.Context, service.Storage) error
	Categories     categories.Categories
	SkipMigrations bool
}

// SetupTestDBWithOptions creates a test database with custom options.
func SetupTestDBWithOptions(t *testing.T, opts TestDBOptions) *TestDB {
	t.Helper()

	// Create in-memory SQLite storage
	storage, err := storage.NewSQLiteStorage(":memory:")
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}
	ctx := context.Background()

	// Run migrations unless skipped
	if !opts.SkipMigrations {
		if err := storage.Migrate(ctx); err != nil {
			t.Fatalf("failed to run migrations: %v", err)
		}
	}

	// Seed categories
	if len(opts.Categories) > 0 {
		for _, cat := range opts.Categories {
			if _, err := storage.CreateCategory(ctx, cat.Name, cat.Description); err != nil {
				t.Fatalf("failed to seed category %q: %v", cat.Name, err)
			}
		}
	}

	// Run custom setup
	if opts.CustomSetup != nil {
		if err := opts.CustomSetup(ctx, storage); err != nil {
			t.Fatalf("custom setup failed: %v", err)
		}
	}

	// Register cleanup
	t.Cleanup(func() {
		storage.Close()
	})

	return &TestDB{
		Storage:    storage,
		Categories: opts.Categories,
		t:          t,
	}
}
