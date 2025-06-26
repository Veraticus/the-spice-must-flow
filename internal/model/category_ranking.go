package model

import (
	"fmt"
	"sort"
)

// CategoryRanking represents how likely a transaction belongs to a specific category.
type CategoryRanking struct {
	Category    string
	Description string
	Score       float64
	IsNew       bool
}

// Validate ensures the CategoryRanking has valid data.
func (r *CategoryRanking) Validate() error {
	if r.Category == "" {
		return fmt.Errorf("category name is required")
	}

	if r.Score < 0.0 || r.Score > 1.0 {
		return fmt.Errorf("score must be between 0.0 and 1.0, got %.2f", r.Score)
	}

	if r.IsNew && r.Description == "" {
		return fmt.Errorf("new categories must have a description")
	}

	if !r.IsNew && r.Description != "" {
		return fmt.Errorf("existing categories should not have descriptions in rankings")
	}

	return nil
}

// CategoryRankings is a slice of CategoryRanking that supports sorting and utility methods.
type CategoryRankings []CategoryRanking

// Len implements sort.Interface.
func (r CategoryRankings) Len() int {
	return len(r)
}

// Less implements sort.Interface - higher scores come first.
func (r CategoryRankings) Less(i, j int) bool {
	// Sort by score descending (higher scores first)
	if r[i].Score != r[j].Score {
		return r[i].Score > r[j].Score
	}
	// If scores are equal, sort by category name for consistency
	return r[i].Category < r[j].Category
}

// Swap implements sort.Interface.
func (r CategoryRankings) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

// Sort sorts the rankings by score in descending order.
func (r CategoryRankings) Sort() {
	sort.Sort(r)
}

// Top returns the highest-scoring category, or nil if empty.
func (r CategoryRankings) Top() *CategoryRanking {
	if len(r) == 0 {
		return nil
	}
	r.Sort()
	return &r[0]
}

// TopN returns the N highest-scoring categories.
func (r CategoryRankings) TopN(n int) CategoryRankings {
	if n <= 0 {
		return CategoryRankings{}
	}

	r.Sort()

	if n > len(r) {
		n = len(r)
	}

	result := make(CategoryRankings, n)
	copy(result, r[:n])
	return result
}

// AboveThreshold returns all categories with scores above the given threshold.
func (r CategoryRankings) AboveThreshold(threshold float64) CategoryRankings {
	r.Sort()

	var result CategoryRankings
	for _, ranking := range r {
		if ranking.Score >= threshold {
			result = append(result, ranking)
		}
	}
	return result
}

// Validate ensures all rankings in the slice are valid.
func (r CategoryRankings) Validate() error {
	seen := make(map[string]bool)

	for i, ranking := range r {
		// Validate individual ranking
		if err := ranking.Validate(); err != nil {
			return fmt.Errorf("invalid ranking at index %d: %w", i, err)
		}

		// Check for duplicate categories
		if seen[ranking.Category] {
			return fmt.Errorf("duplicate category %q in rankings", ranking.Category)
		}
		seen[ranking.Category] = true
	}

	return nil
}

// ApplyCheckPatternBoosts applies confidence boosts from matching check patterns.
// This modifies scores in-place and re-sorts the rankings.
func (r CategoryRankings) ApplyCheckPatternBoosts(patterns []CheckPattern) {
	// Create a map of category to total boost
	boosts := make(map[string]float64)

	for _, pattern := range patterns {
		boosts[pattern.Category] += pattern.ConfidenceBoost
	}

	// Apply boosts to matching categories
	for i := range r {
		if boost, ok := boosts[r[i].Category]; ok {
			// Apply boost but cap at 1.0
			r[i].Score = minFloat(r[i].Score+boost, 1.0)
		}
	}

	// Re-sort after applying boosts
	r.Sort()
}

// minFloat returns the smaller of two float64 values.
func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
