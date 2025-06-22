package engine

import (
	"context"
	"testing"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
)

func TestMockClassifier_SuggestCategoryRankings(t *testing.T) {
	ctx := context.Background()
	classifier := NewMockClassifier()

	tests := []struct {
		name          string
		wantTopCat    string
		categories    []model.Category
		checkPatterns []model.CheckPattern
		transaction   model.Transaction
		wantMinScore  float64
		wantNewCat    bool
	}{
		{
			name: "starbucks transaction",
			transaction: model.Transaction{
				ID:           "test-1",
				MerchantName: "Starbucks",
				Amount:       5.50,
			},
			categories: []model.Category{
				{Name: "Coffee & Tea", Description: "Coffee shops and tea houses"},
				{Name: "Food & Dining", Description: "Restaurants and dining"},
				{Name: "Shopping", Description: "General retail"},
			},
			wantTopCat:   "Coffee & Tea",
			wantMinScore: 0.90,
			wantNewCat:   false,
		},
		{
			name: "amazon transaction",
			transaction: model.Transaction{
				ID:           "test-2",
				MerchantName: "Amazon",
				Amount:       75.00,
			},
			categories: []model.Category{
				{Name: "Shopping", Description: "General retail"},
				{Name: "Office Supplies", Description: "Business supplies"},
				{Name: "Entertainment", Description: "Entertainment services"},
			},
			wantTopCat:   "Shopping",
			wantMinScore: 0.70,
			wantNewCat:   false,
		},
		{
			name: "fitness merchant without fitness category",
			transaction: model.Transaction{
				ID:           "test-3",
				MerchantName: "Peloton",
				Amount:       49.99,
			},
			categories: []model.Category{
				{Name: "Shopping", Description: "General retail"},
				{Name: "Entertainment", Description: "Entertainment services"},
			},
			wantTopCat:   "Fitness & Health",
			wantMinScore: 0.70,
			wantNewCat:   true,
		},
		{
			name: "check transaction with pattern boost",
			transaction: model.Transaction{
				ID:          "test-4",
				Name:        "Check Paid #1234",
				Amount:      100.00,
				Type:        "CHECK",
				CheckNumber: "1234",
			},
			categories: []model.Category{
				{Name: "Home Services", Description: "Home maintenance"},
				{Name: "Personal Care", Description: "Personal services"},
				{Name: "Shopping", Description: "General retail"},
			},
			checkPatterns: []model.CheckPattern{
				{
					PatternName:     "Monthly cleaning",
					Category:        "Home Services",
					ConfidenceBoost: 0.5,
					Active:          true,
				},
			},
			wantTopCat:   "Home Services",
			wantMinScore: 0.50, // Will be boosted by pattern
			wantNewCat:   false,
		},
		{
			name: "unknown merchant",
			transaction: model.Transaction{
				ID:           "test-5",
				MerchantName: "Random Store XYZ",
				Amount:       50.00,
			},
			categories: []model.Category{
				{Name: "Shopping", Description: "General retail"},
				{Name: "Miscellaneous", Description: "Other expenses"},
			},
			wantTopCat:   "Shopping",
			wantMinScore: 0.20,
			wantNewCat:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rankings, err := classifier.SuggestCategoryRankings(ctx, tt.transaction, tt.categories, tt.checkPatterns)
			if err != nil {
				t.Fatalf("SuggestCategoryRankings() error = %v", err)
			}

			if len(rankings) == 0 {
				t.Fatal("SuggestCategoryRankings() returned empty rankings")
			}

			// Check top category
			top := rankings.Top()
			if top.Category != tt.wantTopCat {
				t.Errorf("Top category = %v, want %v", top.Category, tt.wantTopCat)
			}

			if top.Score < tt.wantMinScore {
				t.Errorf("Top score = %v, want at least %v", top.Score, tt.wantMinScore)
			}

			if top.IsNew != tt.wantNewCat {
				t.Errorf("Top IsNew = %v, want %v", top.IsNew, tt.wantNewCat)
			}

			// Verify all categories are ranked
			categoryMap := make(map[string]bool)
			for _, r := range rankings {
				categoryMap[r.Category] = true
			}

			// Check that all provided categories are present (unless a new one was added)
			for _, cat := range tt.categories {
				if !categoryMap[cat.Name] && !tt.wantNewCat {
					t.Errorf("Category %v not found in rankings", cat.Name)
				}
			}

			// Verify scores are sorted in descending order
			for i := 1; i < len(rankings); i++ {
				if rankings[i-1].Score < rankings[i].Score {
					t.Errorf("Rankings not sorted: %v < %v at positions %d and %d",
						rankings[i-1].Score, rankings[i].Score, i-1, i)
				}
			}

			// Verify scores are valid
			for i, r := range rankings {
				if r.Score < 0.0 || r.Score > 1.0 {
					t.Errorf("Invalid score %v for category %v at position %d",
						r.Score, r.Category, i)
				}

				if err := r.Validate(); err != nil {
					t.Errorf("Invalid ranking at position %d: %v", i, err)
				}
			}
		})
	}
}

func TestMockClassifier_CheckPatternBoost(t *testing.T) {
	ctx := context.Background()
	classifier := NewMockClassifier()

	transaction := model.Transaction{
		ID:          "test-check",
		Name:        "Check Paid #5000",
		Amount:      200.00,
		Type:        "CHECK",
		CheckNumber: "5000",
	}

	categories := []model.Category{
		{Name: "Home Services", Description: "Home maintenance"},
		{Name: "Personal Care", Description: "Personal services"},
		{Name: "Shopping", Description: "General retail"},
	}

	// First, get rankings without patterns
	rankingsNoPat, err := classifier.SuggestCategoryRankings(ctx, transaction, categories, nil)
	if err != nil {
		t.Fatalf("SuggestCategoryRankings() error = %v", err)
	}

	// Find Home Services score without boost
	var baseScore float64
	for _, r := range rankingsNoPat {
		if r.Category == "Home Services" {
			baseScore = r.Score
			break
		}
	}

	// Now test with pattern boost
	patterns := []model.CheckPattern{
		{
			PatternName:     "Bi-weekly cleaning",
			Category:        "Home Services",
			ConfidenceBoost: 0.4,
			Active:          true,
		},
	}

	rankingsWithPat, err := classifier.SuggestCategoryRankings(ctx, transaction, categories, patterns)
	if err != nil {
		t.Fatalf("SuggestCategoryRankings() with patterns error = %v", err)
	}

	// Find Home Services score with boost
	var boostedScore float64
	for _, r := range rankingsWithPat {
		if r.Category == "Home Services" {
			boostedScore = r.Score
			break
		}
	}

	expectedBoost := baseScore + 0.4
	if expectedBoost > 1.0 {
		expectedBoost = 1.0
	}

	if boostedScore != expectedBoost {
		t.Errorf("Boosted score = %v, want %v (base %v + boost 0.4)",
			boostedScore, expectedBoost, baseScore)
	}

	// Verify Home Services is now the top category
	if rankingsWithPat.Top().Category != "Home Services" {
		t.Errorf("Top category after boost = %v, want Home Services", rankingsWithPat.Top().Category)
	}
}

func TestMockClassifier_BackwardCompatibility(t *testing.T) {
	ctx := context.Background()
	classifier := NewMockClassifier()

	transaction := model.Transaction{
		ID:           "test-compat",
		MerchantName: "Whole Foods",
		Amount:       125.00,
	}

	categories := []string{"Groceries", "Shopping", "Food & Dining"}

	// Test original SuggestCategory method
	category, confidence, isNew, _, err := classifier.SuggestCategory(ctx, transaction, categories)
	if err != nil {
		t.Fatalf("SuggestCategory() error = %v", err)
	}

	if category != "Groceries" {
		t.Errorf("SuggestCategory() category = %v, want Groceries", category)
	}

	if confidence < 0.90 {
		t.Errorf("SuggestCategory() confidence = %v, want >= 0.90", confidence)
	}

	if isNew {
		t.Error("SuggestCategory() isNew = true, want false")
	}

	// Now test that rankings would give same top result
	categoryModels := []model.Category{
		{Name: "Groceries", Description: "Grocery stores"},
		{Name: "Shopping", Description: "General retail"},
		{Name: "Food & Dining", Description: "Restaurants"},
	}

	rankings, err := classifier.SuggestCategoryRankings(ctx, transaction, categoryModels, nil)
	if err != nil {
		t.Fatalf("SuggestCategoryRankings() error = %v", err)
	}

	top := rankings.Top()
	if top.Category != category {
		t.Errorf("Rankings top category = %v, want %v", top.Category, category)
	}

	if top.Score != confidence {
		t.Errorf("Rankings top score = %v, want %v", top.Score, confidence)
	}
}
