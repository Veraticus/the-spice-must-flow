package model

import (
	"testing"
)

func TestCategoryRanking_Validate(t *testing.T) {
	tests := []struct {
		name    string
		errMsg  string
		ranking CategoryRanking
		wantErr bool
	}{
		{
			name: "valid existing category",
			ranking: CategoryRanking{
				Category: "Food & Dining",
				Score:    0.85,
				IsNew:    false,
			},
			wantErr: false,
		},
		{
			name: "valid new category",
			ranking: CategoryRanking{
				Category:    "Pet Supplies",
				Score:       0.75,
				IsNew:       true,
				Description: "Purchases for pet care, food, and accessories",
			},
			wantErr: false,
		},
		{
			name: "empty category name",
			ranking: CategoryRanking{
				Score: 0.5,
			},
			wantErr: true,
			errMsg:  "category name is required",
		},
		{
			name: "score too low",
			ranking: CategoryRanking{
				Category: "Shopping",
				Score:    -0.1,
			},
			wantErr: true,
			errMsg:  "score must be between 0.0 and 1.0, got -0.10",
		},
		{
			name: "score too high",
			ranking: CategoryRanking{
				Category: "Shopping",
				Score:    1.1,
			},
			wantErr: true,
			errMsg:  "score must be between 0.0 and 1.0, got 1.10",
		},
		{
			name: "new category without description",
			ranking: CategoryRanking{
				Category: "New Category",
				Score:    0.8,
				IsNew:    true,
			},
			wantErr: true,
			errMsg:  "new categories must have a description",
		},
		{
			name: "existing category with description",
			ranking: CategoryRanking{
				Category:    "Existing",
				Score:       0.9,
				IsNew:       false,
				Description: "Should not have this",
			},
			wantErr: true,
			errMsg:  "existing categories should not have descriptions in rankings",
		},
		{
			name: "edge case - score 0.0",
			ranking: CategoryRanking{
				Category: "Low Confidence",
				Score:    0.0,
			},
			wantErr: false,
		},
		{
			name: "edge case - score 1.0",
			ranking: CategoryRanking{
				Category: "High Confidence",
				Score:    1.0,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ranking.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" && err.Error() != tt.errMsg {
				t.Errorf("Validate() error = %v, want %v", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestCategoryRankings_Sort(t *testing.T) {
	rankings := CategoryRankings{
		{Category: "B", Score: 0.5},
		{Category: "A", Score: 0.8},
		{Category: "D", Score: 0.3},
		{Category: "C", Score: 0.8}, // Same score as A
	}

	rankings.Sort()

	// Check order
	expected := []struct {
		category string
		score    float64
	}{
		{"A", 0.8}, // First by score, then alphabetically
		{"C", 0.8},
		{"B", 0.5},
		{"D", 0.3},
	}

	for i, exp := range expected {
		if rankings[i].Category != exp.category || rankings[i].Score != exp.score {
			t.Errorf("Sort() index %d = %v, want {%s, %.1f}",
				i, rankings[i], exp.category, exp.score)
		}
	}
}

func TestCategoryRankings_Top(t *testing.T) {
	tests := []struct {
		want     *CategoryRanking
		name     string
		rankings CategoryRankings
	}{
		{
			name:     "empty rankings",
			rankings: CategoryRankings{},
			want:     nil,
		},
		{
			name: "single ranking",
			rankings: CategoryRankings{
				{Category: "A", Score: 0.5},
			},
			want: &CategoryRanking{Category: "A", Score: 0.5},
		},
		{
			name: "multiple rankings",
			rankings: CategoryRankings{
				{Category: "B", Score: 0.5},
				{Category: "A", Score: 0.9},
				{Category: "C", Score: 0.3},
			},
			want: &CategoryRanking{Category: "A", Score: 0.9},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.rankings.Top()
			switch {
			case tt.want == nil && got != nil:
				t.Errorf("Top() = %v, want nil", got)
			case tt.want != nil && got == nil:
				t.Errorf("Top() = nil, want %v", tt.want)
			case tt.want != nil && got != nil && (got.Category != tt.want.Category || got.Score != tt.want.Score):
				t.Errorf("Top() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCategoryRankings_TopN(t *testing.T) {
	rankings := CategoryRankings{
		{Category: "A", Score: 0.9},
		{Category: "B", Score: 0.7},
		{Category: "C", Score: 0.5},
		{Category: "D", Score: 0.3},
		{Category: "E", Score: 0.1},
	}

	tests := []struct {
		name  string
		first string
		last  string
		n     int
		count int
	}{
		{name: "zero", n: 0, count: 0, first: "", last: ""},
		{name: "negative", n: -1, count: 0, first: "", last: ""},
		{name: "top 1", n: 1, count: 1, first: "A", last: "A"},
		{name: "top 3", n: 3, count: 3, first: "A", last: "C"},
		{name: "top all", n: 5, count: 5, first: "A", last: "E"},
		{name: "top more than exists", n: 10, count: 5, first: "A", last: "E"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rankings.TopN(tt.n)
			if len(got) != tt.count {
				t.Errorf("TopN(%d) returned %d items, want %d", tt.n, len(got), tt.count)
			}
			if tt.count > 0 {
				if got[0].Category != tt.first {
					t.Errorf("TopN(%d) first = %s, want %s", tt.n, got[0].Category, tt.first)
				}
				if got[len(got)-1].Category != tt.last {
					t.Errorf("TopN(%d) last = %s, want %s", tt.n, got[len(got)-1].Category, tt.last)
				}
			}
		})
	}
}

func TestCategoryRankings_AboveThreshold(t *testing.T) {
	rankings := CategoryRankings{
		{Category: "A", Score: 0.9},
		{Category: "B", Score: 0.7},
		{Category: "C", Score: 0.5},
		{Category: "D", Score: 0.3},
	}

	tests := []struct {
		name      string
		want      []string
		threshold float64
	}{
		{name: "all pass", threshold: 0.0, want: []string{"A", "B", "C", "D"}},
		{name: "high threshold", threshold: 0.85, want: []string{"A"}},
		{name: "medium threshold", threshold: 0.5, want: []string{"A", "B", "C"}},
		{name: "none pass", threshold: 1.1, want: []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rankings.AboveThreshold(tt.threshold)
			if len(got) != len(tt.want) {
				t.Errorf("AboveThreshold(%.1f) returned %d items, want %d",
					tt.threshold, len(got), len(tt.want))
			}
			for i, cat := range tt.want {
				if i < len(got) && got[i].Category != cat {
					t.Errorf("AboveThreshold(%.1f)[%d] = %s, want %s",
						tt.threshold, i, got[i].Category, cat)
				}
			}
		})
	}
}

func TestCategoryRankings_Validate(t *testing.T) {
	tests := []struct {
		name     string
		errMsg   string
		rankings CategoryRankings
		wantErr  bool
	}{
		{
			name: "valid rankings",
			rankings: CategoryRankings{
				{Category: "A", Score: 0.9},
				{Category: "B", Score: 0.7},
			},
			wantErr: false,
		},
		{
			name: "duplicate category",
			rankings: CategoryRankings{
				{Category: "A", Score: 0.9},
				{Category: "A", Score: 0.7},
			},
			wantErr: true,
			errMsg:  "duplicate category",
		},
		{
			name: "invalid ranking",
			rankings: CategoryRankings{
				{Category: "A", Score: 0.9},
				{Category: "", Score: 0.7},
			},
			wantErr: true,
			errMsg:  "invalid ranking at index 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.rankings.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" {
				if !containsString(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %v, want error containing %v", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

// Helper function.
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
