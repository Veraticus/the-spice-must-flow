package llm

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/service"
)

// mockRankingClient is a test client that returns predefined rankings.
type mockRankingClient struct {
	err           error
	rankings      []CategoryRanking
	classifyCount int
	rankingCount  int
}

func (m *mockRankingClient) Classify(_ context.Context, _ string) (ClassificationResponse, error) {
	m.classifyCount++
	if m.err != nil {
		return ClassificationResponse{}, m.err
	}

	// Return the top ranking as a classification
	if len(m.rankings) > 0 {
		top := m.rankings[0]
		return ClassificationResponse{
			Category:            top.Category,
			Confidence:          top.Score,
			IsNew:               top.IsNew,
			CategoryDescription: top.Description,
		}, nil
	}

	return ClassificationResponse{}, errors.New("no rankings available")
}

func (m *mockRankingClient) ClassifyWithRankings(_ context.Context, _ string) (RankingResponse, error) {
	m.rankingCount++
	if m.err != nil {
		return RankingResponse{}, m.err
	}
	return RankingResponse{Rankings: m.rankings}, nil
}

func (m *mockRankingClient) GenerateDescription(_ context.Context, _ string) (DescriptionResponse, error) {
	return DescriptionResponse{Description: "Test description"}, nil
}

func (m *mockRankingClient) ClassifyDirection(_ context.Context, _ string) (DirectionResponse, error) {
	if m.err != nil {
		return DirectionResponse{}, m.err
	}
	return DirectionResponse{
		Direction:  model.DirectionExpense,
		Confidence: 0.95,
		Reasoning:  "Test reasoning",
	}, nil
}

func TestClassifier_SuggestCategoryRankings(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	tests := []struct {
		mockErr       error
		name          string
		wantTopCat    string
		categories    []model.Category
		checkPatterns []model.CheckPattern
		mockRankings  []CategoryRanking
		transaction   model.Transaction
		wantTopScore  float64
		wantCount     int
		wantErr       bool
	}{
		{
			name: "successful ranking with multiple categories",
			transaction: model.Transaction{
				ID:           "123",
				MerchantName: "Starbucks",
				Amount:       5.50,
				Date:         time.Now(),
			},
			categories: []model.Category{
				{Name: "Food & Dining", Description: "Restaurants and dining"},
				{Name: "Coffee Shops", Description: "Coffee and tea establishments"},
				{Name: "Shopping", Description: "General retail"},
			},
			mockRankings: []CategoryRanking{
				{Category: "Coffee Shops", Score: 0.95, IsNew: false},
				{Category: "Food & Dining", Score: 0.85, IsNew: false},
				{Category: "Shopping", Score: 0.10, IsNew: false},
			},
			wantErr:      false,
			wantTopCat:   "Coffee Shops",
			wantTopScore: 0.95,
			wantCount:    3,
		},
		{
			name: "ranking with new category suggestion",
			transaction: model.Transaction{
				ID:           "124",
				MerchantName: "Peloton",
				Amount:       49.99,
				Date:         time.Now(),
			},
			categories: []model.Category{
				{Name: "Shopping", Description: "General retail"},
				{Name: "Entertainment", Description: "Entertainment services"},
			},
			mockRankings: []CategoryRanking{
				{Category: "Fitness & Health", Score: 0.75, IsNew: true, Description: "Fitness and health services"},
				{Category: "Shopping", Score: 0.40, IsNew: false},
				{Category: "Entertainment", Score: 0.20, IsNew: false},
			},
			wantErr:      false,
			wantTopCat:   "Fitness & Health",
			wantTopScore: 0.75,
			wantCount:    3,
		},
		{
			name: "check transaction with pattern boosts",
			transaction: model.Transaction{
				ID:          "125",
				Name:        "Check Paid #1234",
				Amount:      100.00,
				Date:        time.Now(),
				Type:        "CHECK",
				CheckNumber: "1234",
			},
			categories: []model.Category{
				{Name: "Home Services", Description: "Home maintenance and services"},
				{Name: "Personal Care", Description: "Personal services"},
			},
			checkPatterns: []model.CheckPattern{
				{
					PatternName:     "Monthly cleaning",
					Category:        "Home Services",
					ConfidenceBoost: 0.3,
					Active:          true,
				},
			},
			mockRankings: []CategoryRanking{
				{Category: "Home Services", Score: 0.50, IsNew: false},
				{Category: "Personal Care", Score: 0.40, IsNew: false},
			},
			wantErr:      false,
			wantTopCat:   "Home Services",
			wantTopScore: 0.80, // 0.50 + 0.30 boost
			wantCount:    2,
		},
		{
			name: "error from LLM client",
			transaction: model.Transaction{
				ID:           "126",
				MerchantName: "Test Merchant",
				Amount:       10.00,
				Date:         time.Now(),
			},
			categories: []model.Category{
				{Name: "Shopping", Description: "General retail"},
			},
			mockErr: errors.New("LLM service unavailable"),
			wantErr: true,
		},
		{
			name: "empty categories list",
			transaction: model.Transaction{
				ID:           "127",
				MerchantName: "Test Merchant",
				Amount:       10.00,
				Date:         time.Now(),
			},
			categories: []model.Category{},
			mockRankings: []CategoryRanking{
				{Category: "New Category", Score: 0.80, IsNew: true, Description: "A new category"},
			},
			wantErr:      false,
			wantTopCat:   "New Category",
			wantTopScore: 0.80,
			wantCount:    1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockRankingClient{
				rankings: tt.mockRankings,
				err:      tt.mockErr,
			}

			classifier := &Classifier{
				client:      mockClient,
				cache:       newSuggestionCache(time.Hour),
				logger:      logger,
				retryOpts:   service.RetryOptions{MaxAttempts: 1, InitialDelay: time.Millisecond},
				rateLimiter: newRateLimiter(100),
			}
			defer func() { _ = classifier.Close() }()

			rankings, err := classifier.SuggestCategoryRankings(
				context.Background(),
				tt.transaction,
				tt.categories,
				tt.checkPatterns,
			)

			if (err != nil) != tt.wantErr {
				t.Errorf("SuggestCategoryRankings() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return // Skip further checks if we expected an error
			}

			if len(rankings) != tt.wantCount {
				t.Errorf("SuggestCategoryRankings() returned %d rankings, want %d", len(rankings), tt.wantCount)
			}

			top := rankings.Top()
			if top == nil {
				t.Fatal("SuggestCategoryRankings() returned no top ranking")
			}

			if top.Category != tt.wantTopCat {
				t.Errorf("Top category = %v, want %v", top.Category, tt.wantTopCat)
			}

			if top.Score != tt.wantTopScore {
				t.Errorf("Top score = %v, want %v", top.Score, tt.wantTopScore)
			}

			// Verify ranking was called
			if mockClient.rankingCount != 1 {
				t.Errorf("ClassifyWithRankings called %d times, want 1", mockClient.rankingCount)
			}
		})
	}
}

func TestClassifier_BackwardCompatibility(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	mockClient := &mockRankingClient{
		rankings: []CategoryRanking{
			{Category: "Food & Dining", Score: 0.90, IsNew: false},
			{Category: "Shopping", Score: 0.30, IsNew: false},
		},
	}

	classifier := &Classifier{
		client:      mockClient,
		cache:       newSuggestionCache(time.Hour),
		logger:      logger,
		retryOpts:   service.RetryOptions{MaxAttempts: 1, InitialDelay: time.Millisecond},
		rateLimiter: newRateLimiter(100),
	}
	defer func() { _ = classifier.Close() }()

	transaction := model.Transaction{
		ID:           "test-123",
		MerchantName: "Test Restaurant",
		Amount:       45.00,
		Date:         time.Now(),
		Hash:         "unique-hash",
	}

	categories := []string{"Food & Dining", "Shopping", "Entertainment"}

	// Test that SuggestCategory uses rankings internally
	category, confidence, isNew, _, err := classifier.SuggestCategory(
		context.Background(),
		transaction,
		categories,
	)

	if err != nil {
		t.Fatalf("SuggestCategory() error = %v", err)
	}

	if category != "Food & Dining" {
		t.Errorf("SuggestCategory() category = %v, want Food & Dining", category)
	}

	if confidence != 0.90 {
		t.Errorf("SuggestCategory() confidence = %v, want 0.90", confidence)
	}

	if isNew {
		t.Errorf("SuggestCategory() isNew = true, want false")
	}

	// Verify that the ranking method was called
	if mockClient.rankingCount != 1 {
		t.Errorf("ClassifyWithRankings called %d times, want 1", mockClient.rankingCount)
	}

	// Verify caching works - second call should use cache
	category2, _, _, _, err := classifier.SuggestCategory(
		context.Background(),
		transaction,
		categories,
	)

	if err != nil {
		t.Fatalf("SuggestCategory() second call error = %v", err)
	}

	if category2 != category {
		t.Errorf("Cached category = %v, want %v", category2, category)
	}

	// Should still be 1 call due to caching
	if mockClient.rankingCount != 1 {
		t.Errorf("ClassifyWithRankings called %d times after cache hit, want 1", mockClient.rankingCount)
	}
}

func TestClassifier_PromptBuilding(t *testing.T) {
	classifier := &Classifier{}

	transaction := model.Transaction{
		ID:           "test-123",
		Name:         "Check Paid #1234",
		MerchantName: "",
		Amount:       100.00,
		Date:         time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		Type:         "CHECK",
		CheckNumber:  "1234",
	}

	categories := []model.Category{
		{Name: "Home Services", Description: "Home maintenance and cleaning"},
		{Name: "Personal Care", Description: "Personal services and wellness"},
	}

	checkPatterns := []model.CheckPattern{
		{
			PatternName: "Monthly cleaning",
			Category:    "Home Services",
			UseCount:    10,
			Active:      true,
		},
	}

	prompt := classifier.buildPromptWithRanking(transaction, categories, checkPatterns)

	// Verify prompt contains key elements
	if !containsString(prompt, "Check Paid #1234") {
		t.Error("Prompt missing transaction name")
	}

	if !containsString(prompt, "$100.00") {
		t.Error("Prompt missing amount")
	}

	if !containsString(prompt, "2024-01-15") {
		t.Error("Prompt missing date")
	}

	if !containsString(prompt, "Transaction Type: CHECK") {
		t.Error("Prompt missing transaction type")
	}

	if !containsString(prompt, "Check Number: 1234") {
		t.Error("Prompt missing check number")
	}

	if !containsString(prompt, "Check Pattern Matches:") {
		t.Error("Prompt missing check pattern section")
	}

	if !containsString(prompt, "Monthly cleaning") {
		t.Error("Prompt missing pattern name")
	}

	if !containsString(prompt, "Home Services: Home maintenance and cleaning") {
		t.Error("Prompt missing category with description")
	}

	if !containsString(prompt, "RANKINGS:") {
		t.Error("Prompt missing rankings instruction")
	}
}

// Helper to check if string contains substring.
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr) != -1
}

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
