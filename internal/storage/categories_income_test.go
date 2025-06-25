package storage

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
)

func TestCreateCategoryWithType(t *testing.T) {
	ctx := context.Background()

	t.Run("create income category", func(t *testing.T) {
		store, cleanup := createTestStorage(t)
		defer cleanup()

		// Create an income category
		cat, err := store.CreateCategoryWithType(ctx, "Salary", "Monthly salary income", model.CategoryTypeIncome)
		require.NoError(t, err)
		assert.Equal(t, "Salary", cat.Name)
		assert.Equal(t, "Monthly salary income", cat.Description)
		assert.Equal(t, model.CategoryTypeIncome, cat.Type)
		assert.True(t, cat.IsActive)

		// Verify it was saved correctly
		retrieved, err := store.GetCategoryByName(ctx, "Salary")
		require.NoError(t, err)
		assert.Equal(t, model.CategoryTypeIncome, retrieved.Type)
	})

	t.Run("create expense category", func(t *testing.T) {
		store, cleanup := createTestStorage(t)
		defer cleanup()

		// Create an expense category
		cat, err := store.CreateCategoryWithType(ctx, "Groceries", "Food and household items", model.CategoryTypeExpense)
		require.NoError(t, err)
		assert.Equal(t, "Groceries", cat.Name)
		assert.Equal(t, model.CategoryTypeExpense, cat.Type)
	})

	t.Run("create system category", func(t *testing.T) {
		store, cleanup := createTestStorage(t)
		defer cleanup()

		// Create a system category
		cat, err := store.CreateCategoryWithType(ctx, "Transfer", "Internal transfers", model.CategoryTypeSystem)
		require.NoError(t, err)
		assert.Equal(t, "Transfer", cat.Name)
		assert.Equal(t, model.CategoryTypeSystem, cat.Type)
	})

	t.Run("reactivate existing category preserves new type", func(t *testing.T) {
		store, cleanup := createTestStorage(t)
		defer cleanup()

		// Create a category as expense
		cat1, err := store.CreateCategoryWithType(ctx, "Consulting", "Consulting income", model.CategoryTypeExpense)
		require.NoError(t, err)
		assert.Equal(t, model.CategoryTypeExpense, cat1.Type)

		// Delete it
		err = store.DeleteCategory(ctx, cat1.ID)
		require.NoError(t, err)

		// Recreate it as income
		cat2, err := store.CreateCategoryWithType(ctx, "Consulting", "Consulting income", model.CategoryTypeIncome)
		require.NoError(t, err)
		assert.Equal(t, cat1.ID, cat2.ID)                    // Same ID, reactivated
		assert.Equal(t, model.CategoryTypeIncome, cat2.Type) // New type applied
	})
}

func TestCreateCategoryBackwardCompatibility(t *testing.T) {
	ctx := context.Background()
	store, cleanup := createTestStorage(t)
	defer cleanup()

	// The old CreateCategory should default to expense type
	cat, err := store.CreateCategory(ctx, "Shopping", "Retail purchases")
	require.NoError(t, err)
	assert.Equal(t, model.CategoryTypeExpense, cat.Type)
}
