package main

import (
	"context"
	"testing"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckPatternsIntegration(t *testing.T) {
	// Create test database with empty categories
	testDB := testutil.SetupTestDB(t, nil)

	ctx := context.Background()
	store := testDB.Storage

	// Create test categories first
	categories := []struct {
		name        string
		description string
	}{
		{"Home Services", "Cleaning, repairs, etc."},
		{"Housing", "Rent, mortgage, etc."},
		{"Taxes", "Tax payments"},
	}

	for _, cat := range categories {
		_, err := store.CreateCategory(ctx, cat.name, cat.description, model.CategoryTypeExpense)
		require.NoError(t, err)
	}

	t.Run("create and retrieve check patterns", func(t *testing.T) {
		// Create test patterns
		patterns := []model.CheckPattern{
			{
				PatternName: "Monthly cleaning",
				AmountMin:   ptr(100.00),
				AmountMax:   ptr(200.00), // Add upper limit to avoid matching rent amounts
				Category:    "Home Services",
				Notes:       "Cleaning service",
				Active:      true,
			},
			{
				PatternName:   "Rent payment",
				AmountMin:     ptr(3000.00),
				AmountMax:     ptr(3100.00),
				Category:      "Housing",
				DayOfMonthMin: ptr(1),
				DayOfMonthMax: ptr(5),
				Active:        true,
			},
		}

		// Create patterns
		for i := range patterns {
			err := store.CreateCheckPattern(ctx, &patterns[i])
			assert.NoError(t, err)
			assert.NotZero(t, patterns[i].ID)
		}

		// Retrieve all patterns
		retrieved, err := store.GetActiveCheckPatterns(ctx)
		assert.NoError(t, err)
		assert.Len(t, retrieved, 2)

		// Verify pattern details
		pattern1, err := store.GetCheckPattern(ctx, patterns[0].ID)
		assert.NoError(t, err)
		assert.Equal(t, "Monthly cleaning", pattern1.PatternName)
		assert.Equal(t, 100.00, *pattern1.AmountMin)
		assert.Equal(t, 200.00, *pattern1.AmountMax)
	})

	t.Run("match check patterns", func(t *testing.T) {
		// Create test transactions
		transactions := []model.Transaction{
			{
				Amount: 100.00,
				Date:   time.Date(2024, 12, 15, 0, 0, 0, 0, time.UTC),
				Type:   "CHECK",
				Name:   "Check #1234",
			},
			{
				Amount: 3050.00,
				Date:   time.Date(2024, 12, 1, 0, 0, 0, 0, time.UTC),
				Type:   "CHECK",
				Name:   "Check #1235",
			},
			{
				Amount: 3050.00,
				Date:   time.Date(2024, 12, 15, 0, 0, 0, 0, time.UTC), // Wrong day
				Type:   "CHECK",
				Name:   "Check #1236",
			},
		}

		// Test matching
		matches1, err := store.GetMatchingCheckPatterns(ctx, transactions[0])
		assert.NoError(t, err)
		assert.Len(t, matches1, 1)
		assert.Equal(t, "Monthly cleaning", matches1[0].PatternName)

		matches2, err := store.GetMatchingCheckPatterns(ctx, transactions[1])
		assert.NoError(t, err)
		assert.Len(t, matches2, 1)
		assert.Equal(t, "Rent payment", matches2[0].PatternName)

		matches3, err := store.GetMatchingCheckPatterns(ctx, transactions[2])
		assert.NoError(t, err)
		assert.Len(t, matches3, 0) // No match due to day restriction
	})

	t.Run("update check pattern", func(t *testing.T) {
		// Get a pattern
		patterns, err := store.GetActiveCheckPatterns(ctx)
		require.NoError(t, err)
		require.Greater(t, len(patterns), 0)

		pattern := &patterns[0]
		originalName := pattern.PatternName

		// Update it
		pattern.PatternName = "Updated pattern"
		pattern.AmountMin = ptr(150.00)

		err = store.UpdateCheckPattern(ctx, pattern)
		assert.NoError(t, err)

		// Verify update
		updated, err := store.GetCheckPattern(ctx, pattern.ID)
		assert.NoError(t, err)
		assert.Equal(t, "Updated pattern", updated.PatternName)
		assert.Equal(t, 150.00, *updated.AmountMin)
		assert.NotEqual(t, originalName, updated.PatternName)
	})

	t.Run("delete check pattern", func(t *testing.T) {
		// Get a pattern
		patterns, err := store.GetActiveCheckPatterns(ctx)
		require.NoError(t, err)
		originalCount := len(patterns)
		require.Greater(t, originalCount, 0)

		// Delete it
		err = store.DeleteCheckPattern(ctx, patterns[0].ID)
		assert.NoError(t, err)

		// Verify it's gone from active patterns
		remaining, err := store.GetActiveCheckPatterns(ctx)
		assert.NoError(t, err)
		assert.Len(t, remaining, originalCount-1)

		// Verify it still exists but is inactive
		deleted, err := store.GetCheckPattern(ctx, patterns[0].ID)
		assert.NoError(t, err)
		assert.False(t, deleted.Active)
	})

	t.Run("increment use count", func(t *testing.T) {
		// Get an active pattern
		patterns, err := store.GetActiveCheckPatterns(ctx)
		require.NoError(t, err)
		require.Greater(t, len(patterns), 0)

		pattern := patterns[0]
		originalCount := pattern.UseCount

		// Increment use count
		err = store.IncrementCheckPatternUseCount(ctx, pattern.ID)
		assert.NoError(t, err)

		// Verify increment
		updated, err := store.GetCheckPattern(ctx, pattern.ID)
		assert.NoError(t, err)
		assert.Equal(t, originalCount+1, updated.UseCount)
	})
}
