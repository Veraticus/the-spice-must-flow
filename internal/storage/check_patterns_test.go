package storage

import (
	"context"
	"testing"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
)

func TestCheckPatternStorage(t *testing.T) {
	ctx := context.Background()
	storage, cleanup := createTestStorage(t)
	defer cleanup()

	// Helper to create a test pattern
	createTestPattern := func(name string, category string) *model.CheckPattern {
		minAmount := 100.0
		maxAmount := 200.0
		dayMin := 1
		dayMax := 7
		return &model.CheckPattern{
			PatternName:     name,
			AmountMin:       &minAmount,
			AmountMax:       &maxAmount,
			DayOfMonthMin:   &dayMin,
			DayOfMonthMax:   &dayMax,
			Category:        category,
			Notes:           "Test pattern",
			ConfidenceBoost: 0.3,
		}
	}

	t.Run("CreateCheckPattern", func(t *testing.T) {
		pattern := createTestPattern("Test Pattern", "Test Category")

		err := storage.CreateCheckPattern(ctx, pattern)
		if err != nil {
			t.Fatalf("CreateCheckPattern() error = %v", err)
		}

		if pattern.ID == 0 {
			t.Error("CreateCheckPattern() did not set pattern ID")
		}
	})

	t.Run("CreateCheckPattern_ValidationError", func(t *testing.T) {
		pattern := &model.CheckPattern{
			// Missing required fields
		}

		err := storage.CreateCheckPattern(ctx, pattern)
		if err == nil {
			t.Error("CreateCheckPattern() error = nil, want validation error")
		}
	})

	t.Run("GetCheckPattern", func(t *testing.T) {
		// Create a pattern first
		original := createTestPattern("Get Test", "Test Category")
		err := storage.CreateCheckPattern(ctx, original)
		if err != nil {
			t.Fatalf("CreateCheckPattern() error = %v", err)
		}

		// Now retrieve it
		retrieved, err := storage.GetCheckPattern(ctx, original.ID)
		if err != nil {
			t.Fatalf("GetCheckPattern() error = %v", err)
		}

		// Verify fields
		if retrieved.PatternName != original.PatternName {
			t.Errorf("PatternName = %v, want %v", retrieved.PatternName, original.PatternName)
		}
		if retrieved.Category != original.Category {
			t.Errorf("Category = %v, want %v", retrieved.Category, original.Category)
		}
		if *retrieved.AmountMin != *original.AmountMin {
			t.Errorf("AmountMin = %v, want %v", *retrieved.AmountMin, *original.AmountMin)
		}
	})

	t.Run("GetCheckPattern_NotFound", func(t *testing.T) {
		_, err := storage.GetCheckPattern(ctx, 99999)
		if err != ErrCheckPatternNotFound {
			t.Errorf("GetCheckPattern() error = %v, want %v", err, ErrCheckPatternNotFound)
		}
	})

	t.Run("GetActiveCheckPatterns", func(t *testing.T) {
		// Clear any existing patterns
		clearCheckPatterns(t, storage)

		// Create multiple patterns
		patterns := []*model.CheckPattern{
			createTestPattern("Pattern 1", "Category A"),
			createTestPattern("Pattern 2", "Category B"),
			createTestPattern("Pattern 3", "Category C"),
		}

		for _, p := range patterns {
			if err := storage.CreateCheckPattern(ctx, p); err != nil {
				t.Fatalf("CreateCheckPattern() error = %v", err)
			}
		}

		// Get all patterns
		active, err := storage.GetActiveCheckPatterns(ctx)
		if err != nil {
			t.Fatalf("GetActiveCheckPatterns() error = %v", err)
		}

		if len(active) != 3 {
			t.Errorf("GetActiveCheckPatterns() returned %d patterns, want 3", len(active))
		}
	})

	t.Run("GetMatchingCheckPatterns", func(t *testing.T) {
		// Clear patterns
		clearCheckPatterns(t, storage)

		// Create patterns with different criteria
		exactAmount := 150.0
		pattern1 := &model.CheckPattern{
			PatternName:     "Exact Amount",
			AmountMin:       &exactAmount,
			AmountMax:       &exactAmount,
			Category:        "Test",
			ConfidenceBoost: 0.3,
		}

		rangeMin := 100.0
		rangeMax := 200.0
		pattern2 := &model.CheckPattern{
			PatternName:     "Amount Range",
			AmountMin:       &rangeMin,
			AmountMax:       &rangeMax,
			Category:        "Test",
			ConfidenceBoost: 0.3,
		}

		dayMin := 10
		dayMax := 20
		pattern3 := &model.CheckPattern{
			PatternName:     "Day Range",
			DayOfMonthMin:   &dayMin,
			DayOfMonthMax:   &dayMax,
			Category:        "Test",
			ConfidenceBoost: 0.3,
		}

		for _, p := range []*model.CheckPattern{pattern1, pattern2, pattern3} {
			if err := storage.CreateCheckPattern(ctx, p); err != nil {
				t.Fatalf("CreateCheckPattern() error = %v", err)
			}
		}

		// Test transaction that matches pattern 1 and 2
		txn := model.Transaction{
			Type:   "CHECK",
			Amount: 150,
			Date:   time.Date(2024, 12, 5, 0, 0, 0, 0, time.UTC), // Day 5
		}

		matches, err := storage.GetMatchingCheckPatterns(ctx, txn)
		if err != nil {
			t.Fatalf("GetMatchingCheckPatterns() error = %v", err)
		}

		if len(matches) != 2 {
			t.Errorf("GetMatchingCheckPatterns() returned %d patterns, want 2", len(matches))
		}
	})

	t.Run("UpdateCheckPattern", func(t *testing.T) {
		pattern := createTestPattern("Update Test", "Original Category")
		err := storage.CreateCheckPattern(ctx, pattern)
		if err != nil {
			t.Fatalf("CreateCheckPattern() error = %v", err)
		}

		// Update the pattern
		pattern.Category = "Updated Category"
		pattern.Notes = "Updated notes"
		newMin := 500.0
		newMax := 600.0
		pattern.AmountMin = &newMin
		pattern.AmountMax = &newMax

		err = storage.UpdateCheckPattern(ctx, pattern)
		if err != nil {
			t.Fatalf("UpdateCheckPattern() error = %v", err)
		}

		// Verify update
		updated, err := storage.GetCheckPattern(ctx, pattern.ID)
		if err != nil {
			t.Fatalf("GetCheckPattern() error = %v", err)
		}

		if updated.Category != "Updated Category" {
			t.Errorf("Category = %v, want %v", updated.Category, "Updated Category")
		}
		if *updated.AmountMin != 500.0 {
			t.Errorf("AmountMin = %v, want %v", *updated.AmountMin, 500.0)
		}
	})

	t.Run("UpdateCheckPattern_NotFound", func(t *testing.T) {
		pattern := createTestPattern("Nonexistent", "Test")
		pattern.ID = 99999

		err := storage.UpdateCheckPattern(ctx, pattern)
		if err != ErrCheckPatternNotFound {
			t.Errorf("UpdateCheckPattern() error = %v, want %v", err, ErrCheckPatternNotFound)
		}
	})

	t.Run("DeleteCheckPattern", func(t *testing.T) {
		pattern := createTestPattern("Delete Test", "Test Category")
		err := storage.CreateCheckPattern(ctx, pattern)
		if err != nil {
			t.Fatalf("CreateCheckPattern() error = %v", err)
		}

		// Delete the pattern
		err = storage.DeleteCheckPattern(ctx, pattern.ID)
		if err != nil {
			t.Fatalf("DeleteCheckPattern() error = %v", err)
		}

		// Verify it no longer exists
		_, err = storage.GetCheckPattern(ctx, pattern.ID)
		if err != ErrCheckPatternNotFound {
			t.Errorf("GetCheckPattern() after delete error = %v, want %v", err, ErrCheckPatternNotFound)
		}

		// Verify it's not in the patterns list
		active, err := storage.GetActiveCheckPatterns(ctx)
		if err != nil {
			t.Fatalf("GetActiveCheckPatterns() error = %v", err)
		}

		for _, p := range active {
			if p.ID == pattern.ID {
				t.Error("Deleted pattern still appears in active patterns")
			}
		}
	})

	t.Run("DeleteCheckPattern_NotFound", func(t *testing.T) {
		err := storage.DeleteCheckPattern(ctx, 99999)
		if err != ErrCheckPatternNotFound {
			t.Errorf("DeleteCheckPattern() error = %v, want %v", err, ErrCheckPatternNotFound)
		}
	})

	t.Run("IncrementCheckPatternUseCount", func(t *testing.T) {
		pattern := createTestPattern("Use Count Test", "Test Category")
		err := storage.CreateCheckPattern(ctx, pattern)
		if err != nil {
			t.Fatalf("CreateCheckPattern() error = %v", err)
		}

		// Increment use count multiple times
		for i := 0; i < 3; i++ {
			err = storage.IncrementCheckPatternUseCount(ctx, pattern.ID)
			if err != nil {
				t.Fatalf("IncrementCheckPatternUseCount() error = %v", err)
			}
		}

		// Verify count
		updated, err := storage.GetCheckPattern(ctx, pattern.ID)
		if err != nil {
			t.Fatalf("GetCheckPattern() error = %v", err)
		}

		if updated.UseCount != 3 {
			t.Errorf("UseCount = %v, want %v", updated.UseCount, 3)
		}
	})

	t.Run("CheckNumberPattern_Serialization", func(t *testing.T) {
		pattern := createTestPattern("Check Number Test", "Test")
		pattern.CheckNumberPattern = &model.CheckNumberMatcher{
			Modulo: 10,
			Offset: 5,
		}

		err := storage.CreateCheckPattern(ctx, pattern)
		if err != nil {
			t.Fatalf("CreateCheckPattern() error = %v", err)
		}

		retrieved, err := storage.GetCheckPattern(ctx, pattern.ID)
		if err != nil {
			t.Fatalf("GetCheckPattern() error = %v", err)
		}

		if retrieved.CheckNumberPattern == nil {
			t.Fatal("CheckNumberPattern is nil")
		}

		if retrieved.CheckNumberPattern.Modulo != 10 || retrieved.CheckNumberPattern.Offset != 5 {
			t.Errorf("CheckNumberPattern = %+v, want {Modulo:10 Offset:5}", retrieved.CheckNumberPattern)
		}
	})
}

// clearCheckPatterns deletes all check patterns for test isolation.
func clearCheckPatterns(t *testing.T, storage *SQLiteStorage) {
	t.Helper()
	_, err := storage.db.Exec("DELETE FROM check_patterns")
	if err != nil {
		t.Fatalf("Failed to clear check patterns: %v", err)
	}
}
