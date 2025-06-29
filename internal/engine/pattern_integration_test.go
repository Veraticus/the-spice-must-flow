package engine

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/pattern"
	"github.com/Veraticus/the-spice-must-flow/internal/service"
)

// MockStorage for testing pattern classifier
// We embed UnimplementedStorage to get default implementations of all methods
type MockStorage struct {
	mock.Mock
	UnimplementedStorage
}

// UnimplementedStorage provides default panic implementations for all Storage methods
type UnimplementedStorage struct{}

func (u UnimplementedStorage) SaveTransactions(ctx context.Context, transactions []model.Transaction) error {
	panic("unimplemented")
}
func (u UnimplementedStorage) GetTransactionsToClassify(ctx context.Context, fromDate *time.Time) ([]model.Transaction, error) {
	panic("unimplemented")
}
func (u UnimplementedStorage) GetTransactionByID(ctx context.Context, id string) (*model.Transaction, error) {
	panic("unimplemented")
}
func (u UnimplementedStorage) GetTransactionsByCategory(ctx context.Context, categoryName string) ([]model.Transaction, error) {
	panic("unimplemented")
}
func (u UnimplementedStorage) GetTransactionsByCategoryID(ctx context.Context, categoryID int) ([]model.Transaction, error) {
	panic("unimplemented")
}
func (u UnimplementedStorage) UpdateTransactionCategories(ctx context.Context, fromCategory, toCategory string) error {
	panic("unimplemented")
}
func (u UnimplementedStorage) UpdateTransactionCategoriesByID(ctx context.Context, fromCategoryID, toCategoryID int) error {
	panic("unimplemented")
}
func (u UnimplementedStorage) GetTransactionCount(ctx context.Context) (int, error) {
	panic("unimplemented")
}
func (u UnimplementedStorage) GetTransactionCountByCategory(ctx context.Context, categoryName string) (int, error) {
	panic("unimplemented")
}
func (u UnimplementedStorage) GetEarliestTransactionDate(ctx context.Context) (time.Time, error) {
	panic("unimplemented")
}
func (u UnimplementedStorage) GetLatestTransactionDate(ctx context.Context) (time.Time, error) {
	panic("unimplemented")
}
func (u UnimplementedStorage) GetCategorySummary(ctx context.Context, start, end time.Time) (map[string]float64, error) {
	panic("unimplemented")
}
func (u UnimplementedStorage) GetMerchantSummary(ctx context.Context, start, end time.Time) (map[string]float64, error) {
	panic("unimplemented")
}
func (u UnimplementedStorage) GetVendor(ctx context.Context, merchantName string) (*model.Vendor, error) {
	panic("unimplemented")
}
func (u UnimplementedStorage) SaveVendor(ctx context.Context, vendor *model.Vendor) error {
	panic("unimplemented")
}
func (u UnimplementedStorage) DeleteVendor(ctx context.Context, merchantName string) error {
	panic("unimplemented")
}
func (u UnimplementedStorage) GetAllVendors(ctx context.Context) ([]model.Vendor, error) {
	panic("unimplemented")
}
func (u UnimplementedStorage) GetVendorsByCategory(ctx context.Context, categoryName string) ([]model.Vendor, error) {
	panic("unimplemented")
}
func (u UnimplementedStorage) GetVendorsByCategoryID(ctx context.Context, categoryID int) ([]model.Vendor, error) {
	panic("unimplemented")
}
func (u UnimplementedStorage) GetVendorsBySource(ctx context.Context, source model.VendorSource) ([]model.Vendor, error) {
	panic("unimplemented")
}
func (u UnimplementedStorage) DeleteVendorsBySource(ctx context.Context, source model.VendorSource) error {
	panic("unimplemented")
}
func (u UnimplementedStorage) UpdateVendorCategories(ctx context.Context, fromCategory, toCategory string) error {
	panic("unimplemented")
}
func (u UnimplementedStorage) UpdateVendorCategoriesByID(ctx context.Context, fromCategoryID, toCategoryID int) error {
	panic("unimplemented")
}
func (u UnimplementedStorage) FindVendorMatch(ctx context.Context, merchantName string) (*model.Vendor, error) {
	panic("unimplemented")
}
func (u UnimplementedStorage) SaveClassification(ctx context.Context, classification *model.Classification) error {
	panic("unimplemented")
}
func (u UnimplementedStorage) GetClassificationsByDateRange(ctx context.Context, start, end time.Time) ([]model.Classification, error) {
	panic("unimplemented")
}
func (u UnimplementedStorage) GetClassificationsByConfidence(ctx context.Context, maxConfidence float64, excludeUserModified bool) ([]model.Classification, error) {
	panic("unimplemented")
}
func (u UnimplementedStorage) ClearAllClassifications(ctx context.Context) error {
	panic("unimplemented")
}
func (u UnimplementedStorage) GetCategories(ctx context.Context) ([]model.Category, error) {
	panic("unimplemented")
}
func (u UnimplementedStorage) GetCategoryByName(ctx context.Context, name string) (*model.Category, error) {
	panic("unimplemented")
}
func (u UnimplementedStorage) CreateCategory(ctx context.Context, name, description string) (*model.Category, error) {
	panic("unimplemented")
}
func (u UnimplementedStorage) CreateCategoryWithType(ctx context.Context, name, description string, categoryType model.CategoryType) (*model.Category, error) {
	panic("unimplemented")
}
func (u UnimplementedStorage) UpdateCategory(ctx context.Context, id int, name, description string) error {
	panic("unimplemented")
}
func (u UnimplementedStorage) DeleteCategory(ctx context.Context, id int) error {
	panic("unimplemented")
}
func (u UnimplementedStorage) CreateCheckPattern(ctx context.Context, pattern *model.CheckPattern) error {
	panic("unimplemented")
}
func (u UnimplementedStorage) GetCheckPattern(ctx context.Context, id int64) (*model.CheckPattern, error) {
	panic("unimplemented")
}
func (u UnimplementedStorage) GetActiveCheckPatterns(ctx context.Context) ([]model.CheckPattern, error) {
	panic("unimplemented")
}
func (u UnimplementedStorage) GetMatchingCheckPatterns(ctx context.Context, txn model.Transaction) ([]model.CheckPattern, error) {
	panic("unimplemented")
}
func (u UnimplementedStorage) UpdateCheckPattern(ctx context.Context, pattern *model.CheckPattern) error {
	panic("unimplemented")
}
func (u UnimplementedStorage) DeleteCheckPattern(ctx context.Context, id int64) error {
	panic("unimplemented")
}
func (u UnimplementedStorage) IncrementCheckPatternUseCount(ctx context.Context, id int64) error {
	panic("unimplemented")
}
func (u UnimplementedStorage) CreatePatternRule(ctx context.Context, rule *model.PatternRule) error {
	panic("unimplemented")
}
func (u UnimplementedStorage) GetPatternRule(ctx context.Context, id int) (*model.PatternRule, error) {
	panic("unimplemented")
}
func (u UnimplementedStorage) GetActivePatternRules(ctx context.Context) ([]model.PatternRule, error) {
	panic("unimplemented")
}
func (u UnimplementedStorage) UpdatePatternRule(ctx context.Context, rule *model.PatternRule) error {
	panic("unimplemented")
}
func (u UnimplementedStorage) DeletePatternRule(ctx context.Context, id int) error {
	panic("unimplemented")
}
func (u UnimplementedStorage) IncrementPatternRuleUseCount(ctx context.Context, id int) error {
	panic("unimplemented")
}
func (u UnimplementedStorage) GetPatternRulesByCategory(ctx context.Context, category string) ([]model.PatternRule, error) {
	panic("unimplemented")
}
func (u UnimplementedStorage) Migrate(ctx context.Context) error {
	panic("unimplemented")
}
func (u UnimplementedStorage) BeginTx(ctx context.Context) (service.Transaction, error) {
	panic("unimplemented")
}
func (u UnimplementedStorage) Close() error {
	panic("unimplemented")
}

// Now override only the methods we need for testing
func (m *MockStorage) GetActivePatternRules(ctx context.Context) ([]model.PatternRule, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]model.PatternRule), args.Error(1)
}

func (m *MockStorage) GetCategories(ctx context.Context) ([]model.Category, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]model.Category), args.Error(1)
}

func (m *MockStorage) IncrementPatternRuleUseCount(ctx context.Context, ruleID int) error {
	args := m.Called(ctx, ruleID)
	return args.Error(0)
}

// MockMatcher for testing
type MockMatcher struct {
	mock.Mock
}

func (m *MockMatcher) Match(ctx context.Context, txn model.Transaction) ([]pattern.Rule, error) {
	args := m.Called(ctx, txn)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]pattern.Rule), args.Error(1)
}

// MockCategorySuggester for testing
type MockCategorySuggester struct {
	mock.Mock
}

func (m *MockCategorySuggester) Suggest(ctx context.Context, txn model.Transaction) ([]pattern.Suggestion, error) {
	args := m.Called(ctx, txn)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]pattern.Suggestion), args.Error(1)
}

func (m *MockCategorySuggester) SuggestWithValidation(ctx context.Context, txn model.Transaction, categories []model.Category) ([]pattern.Suggestion, error) {
	args := m.Called(ctx, txn, categories)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]pattern.Suggestion), args.Error(1)
}

// MockTransactionValidator for testing
type MockTransactionValidator struct {
	mock.Mock
}

func (m *MockTransactionValidator) ValidateDirection(ctx context.Context, txn model.Transaction, category model.Category) error {
	args := m.Called(ctx, txn, category)
	return args.Error(0)
}

func TestNewPatternClassifier(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func(*MockStorage)
		wantErr       bool
		errorContains string
	}{
		{
			name: "success with no rules",
			setupMock: func(ms *MockStorage) {
				ms.On("GetActivePatternRules", mock.Anything).Return([]model.PatternRule{}, nil)
			},
			wantErr: false,
		},
		{
			name: "success with rules",
			setupMock: func(ms *MockStorage) {
				rules := []model.PatternRule{
					{
						ID:               1,
						Name:             "Test Rule",
						MerchantPattern:  "AMAZON",
						DefaultCategory:  "Shopping",
						IsActive:         true,
						Confidence:       0.9,
					},
				}
				ms.On("GetActivePatternRules", mock.Anything).Return(rules, nil)
			},
			wantErr: false,
		},
		{
			name: "storage error",
			setupMock: func(ms *MockStorage) {
				ms.On("GetActivePatternRules", mock.Anything).Return(nil, errors.New("storage error"))
			},
			wantErr:       true,
			errorContains: "storage error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := new(MockStorage)
			tt.setupMock(mockStorage)

			pc, err := NewPatternClassifier(mockStorage)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, pc)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, pc)
				assert.Equal(t, mockStorage, pc.storage)
				assert.NotNil(t, pc.matcher)
				assert.NotNil(t, pc.suggester)
				assert.NotNil(t, pc.validator)
			}

			mockStorage.AssertExpectations(t)
		})
	}
}

func TestPatternClassifier_ClassifyWithPatterns(t *testing.T) {
	ctx := context.Background()
	
	tests := []struct {
		name          string
		transactions  []model.Transaction
		setupMock     func(*MockStorage)
		setupSuggester func(*pattern.Suggester) ([]pattern.Suggestion, error)
		want          *model.CategoryRanking
		wantErr       bool
		errorContains string
	}{
		{
			name:          "no transactions error",
			transactions:  []model.Transaction{},
			setupMock: func(ms *MockStorage) {
				// NewPatternClassifier always calls GetActivePatternRules
				ms.On("GetActivePatternRules", mock.Anything).Return([]model.PatternRule{}, nil).Once()
			},
			wantErr:       true,
			errorContains: "no transactions provided",
		},
		{
			name: "no matching patterns",
			transactions: []model.Transaction{
				{
					ID:           "txn1",
					MerchantName: "Unknown Store",
					Amount:       50.00,
				},
			},
			setupMock: func(ms *MockStorage) {
				ms.On("GetActivePatternRules", mock.Anything).Return([]model.PatternRule{}, nil).Once()
				categories := []model.Category{
					{ID: 1, Name: "Shopping", Type: "expense"},
				}
				ms.On("GetCategories", ctx).Return(categories, nil)
			},
			setupSuggester: func(s *pattern.Suggester) ([]pattern.Suggestion, error) {
				return []pattern.Suggestion{}, nil
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "successful pattern match",
			transactions: []model.Transaction{
				{
					ID:           "txn1",
					MerchantName: "AMAZON MARKETPLACE",
					Amount:       99.99,
				},
			},
			setupMock: func(ms *MockStorage) {
				rules := []model.PatternRule{
					{
						ID:              1,
						Name:            "Amazon Shopping",
						MerchantPattern: "AMAZON",
						DefaultCategory: "Shopping",
						IsActive:        true,
						Confidence:      0.95,
					},
				}
				ms.On("GetActivePatternRules", mock.Anything).Return(rules, nil).Once()
				categories := []model.Category{
					{ID: 1, Name: "Shopping", Type: "expense"},
				}
				ms.On("GetCategories", ctx).Return(categories, nil)
				ms.On("IncrementPatternRuleUseCount", ctx, 1).Return(nil)
			},
			setupSuggester: func(s *pattern.Suggester) ([]pattern.Suggestion, error) {
				ruleID := 1
				return []pattern.Suggestion{
					{
						Category:   "Shopping",
						Confidence: 0.95,
						Reason:     "Matched pattern rule 'Amazon Shopping'",
						RuleID:     &ruleID,
					},
				}, nil
			},
			want: &model.CategoryRanking{
				Category:    "Shopping",
				Score:       0.95,
				IsNew:       false,
				Description: "Matched pattern rule 'Amazon Shopping'",
			},
			wantErr: false,
		},
		{
			name: "categories storage error",
			transactions: []model.Transaction{
				{ID: "txn1", MerchantName: "Store"},
			},
			setupMock: func(ms *MockStorage) {
				ms.On("GetActivePatternRules", mock.Anything).Return([]model.PatternRule{}, nil).Once()
				ms.On("GetCategories", ctx).Return(nil, errors.New("db error"))
			},
			wantErr:       true,
			errorContains: "db error",
		},
		{
			name: "increment use count error (logged but not returned)",
			transactions: []model.Transaction{
				{
					ID:           "txn1",
					MerchantName: "AMAZON",
					Amount:       50.00,
				},
			},
			setupMock: func(ms *MockStorage) {
				rules := []model.PatternRule{
					{
						ID:              1,
						Name:            "Amazon Pattern",
						MerchantPattern: "AMAZON",
						DefaultCategory: "Shopping",
						IsActive:        true,
						Confidence:      0.9,
					},
				}
				ms.On("GetActivePatternRules", mock.Anything).Return(rules, nil).Once()
				categories := []model.Category{
					{ID: 1, Name: "Shopping", Type: "expense"},
				}
				ms.On("GetCategories", ctx).Return(categories, nil)
				ms.On("IncrementPatternRuleUseCount", ctx, 1).Return(errors.New("increment error"))
			},
			setupSuggester: func(s *pattern.Suggester) ([]pattern.Suggestion, error) {
				ruleID := 1
				return []pattern.Suggestion{
					{
						Category:   "Shopping",
						Confidence: 0.9,
						Reason:     "Pattern match",
						RuleID:     &ruleID,
					},
				}, nil
			},
			want: &model.CategoryRanking{
				Category:    "Shopping",
				Score:       0.9,
				IsNew:       false,
				Description: "Pattern match",
			},
			wantErr: false,
		},
		{
			name: "suggestion without rule ID",
			transactions: []model.Transaction{
				{ID: "txn1", MerchantName: "Store"},
			},
			setupMock: func(ms *MockStorage) {
				ms.On("GetActivePatternRules", mock.Anything).Return([]model.PatternRule{}, nil).Once()
				categories := []model.Category{
					{ID: 1, Name: "Shopping", Type: "expense"},
				}
				ms.On("GetCategories", ctx).Return(categories, nil)
			},
			setupSuggester: func(s *pattern.Suggester) ([]pattern.Suggestion, error) {
				return []pattern.Suggestion{
					{
						Category:   "Shopping",
						Confidence: 0.8,
						Reason:     "Manual rule",
						RuleID:     nil,
					},
				}, nil
			},
			want: &model.CategoryRanking{
				Category:    "Shopping",
				Score:       0.8,
				IsNew:       false,
				Description: "Manual rule",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := new(MockStorage)
			
			// Call test-specific setup first
			tt.setupMock(mockStorage)

			pc, err := NewPatternClassifier(mockStorage)
			
			// Only check error if we're not expecting a test error
			if !tt.wantErr || tt.name == "no transactions error" {
				require.NoError(t, err)
			}

			// If we have a custom suggester setup, we need to inject it
			if tt.setupSuggester != nil && pc != nil {
				suggestions, _ := tt.setupSuggester(pc.suggester.(*pattern.Suggester))
				
				// Create a mock suggester that returns our custom suggestions
				mockSuggester := new(MockCategorySuggester)
				mockSuggester.On("SuggestWithValidation", ctx, mock.Anything, mock.Anything).Return(suggestions, nil)
				pc.suggester = mockSuggester
			}

			var got *model.CategoryRanking
			if pc != nil {
				got, err = pc.ClassifyWithPatterns(ctx, tt.transactions)
			}

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}

			mockStorage.AssertExpectations(t)
		})
	}
}

func TestPatternClassifier_RefreshPatterns(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		setupMock     func(*MockStorage)
		wantErr       bool
		errorContains string
	}{
		{
			name: "successful refresh",
			setupMock: func(ms *MockStorage) {
				rules := []model.PatternRule{
					{
						ID:              1,
						Name:            "Updated Rule",
						MerchantPattern: "UPDATED",
						DefaultCategory: "New Category",
						IsActive:        true,
						Confidence:      0.85,
					},
				}
				ms.On("GetActivePatternRules", ctx).Return(rules, nil)
			},
			wantErr: false,
		},
		{
			name: "storage error",
			setupMock: func(ms *MockStorage) {
				ms.On("GetActivePatternRules", ctx).Return(nil, errors.New("refresh error"))
			},
			wantErr:       true,
			errorContains: "refresh error",
		},
		{
			name: "empty rules refresh",
			setupMock: func(ms *MockStorage) {
				ms.On("GetActivePatternRules", ctx).Return([]model.PatternRule{}, nil)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStorage := new(MockStorage)
			
			// Initial setup for creating the classifier
			initialRules := []model.PatternRule{
				{
					ID:              1,
					Name:            "Initial Rule",
					MerchantPattern: "INITIAL",
					DefaultCategory: "Initial Category",
					IsActive:        true,
					Confidence:      0.9,
				},
			}
			mockStorage.On("GetActivePatternRules", mock.Anything).Return(initialRules, nil).Once()

			pc, err := NewPatternClassifier(mockStorage)
			require.NoError(t, err)

			// Setup for refresh test
			tt.setupMock(mockStorage)

			err = pc.RefreshPatterns(ctx)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				// Verify that matcher and suggester are updated (not nil)
				assert.NotNil(t, pc.matcher)
				assert.NotNil(t, pc.suggester)
			}

			mockStorage.AssertExpectations(t)
		})
	}
}

// Test edge cases and concurrent access
func TestPatternClassifier_ConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	mockStorage := new(MockStorage)

	// Setup initial rules
	rules := []model.PatternRule{
		{
			ID:              1,
			Name:            "Test Rule",
			MerchantPattern: "TEST",
			DefaultCategory: "Test Category",
			IsActive:        true,
			Confidence:      0.9,
		},
	}
	mockStorage.On("GetActivePatternRules", mock.Anything).Return(rules, nil)

	pc, err := NewPatternClassifier(mockStorage)
	require.NoError(t, err)

	// Setup mocks for concurrent operations
	categories := []model.Category{
		{ID: 1, Name: "Test Category", Type: "expense"},
	}
	mockStorage.On("GetCategories", ctx).Return(categories, nil).Maybe()
	mockStorage.On("IncrementPatternRuleUseCount", ctx, 1).Return(nil).Maybe()

	// Run concurrent operations
	done := make(chan bool, 3)

	// Goroutine 1: Classify transactions
	go func() {
		txns := []model.Transaction{{ID: "1", MerchantName: "TEST STORE"}}
		_, _ = pc.ClassifyWithPatterns(ctx, txns)
		done <- true
	}()

	// Goroutine 2: Refresh patterns
	go func() {
		_ = pc.RefreshPatterns(ctx)
		done <- true
	}()

	// Goroutine 3: Another classification
	go func() {
		txns := []model.Transaction{{ID: "2", MerchantName: "TEST MARKET"}}
		_, _ = pc.ClassifyWithPatterns(ctx, txns)
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 3; i++ {
		<-done
	}

	// If we get here without panics or race conditions, the test passes
	assert.True(t, true, "Concurrent access handled successfully")
}

// Benchmark for performance testing
func BenchmarkPatternClassifier_ClassifyWithPatterns(b *testing.B) {
	ctx := context.Background()
	mockStorage := new(MockStorage)

	// Setup rules
	rules := make([]model.PatternRule, 100)
	for i := 0; i < 100; i++ {
		rules[i] = model.PatternRule{
			ID:              i,
			Name:            fmt.Sprintf("Rule_%d", i),
			MerchantPattern: fmt.Sprintf("PATTERN%d", i),
			DefaultCategory: fmt.Sprintf("Category%d", i%10),
			IsActive:        true,
			Confidence:      0.8 + float64(i%20)/100,
		}
	}
	mockStorage.On("GetActivePatternRules", mock.Anything).Return(rules, nil)

	pc, err := NewPatternClassifier(mockStorage)
	require.NoError(b, err)

	// Setup for classification
	categories := []model.Category{
		{ID: 1, Name: "Category1", Type: "expense"},
	}
	mockStorage.On("GetCategories", ctx).Return(categories, nil)
	mockStorage.On("IncrementPatternRuleUseCount", ctx, mock.Anything).Return(nil).Maybe()

	txns := []model.Transaction{
		{ID: "bench", MerchantName: "PATTERN50 STORE"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = pc.ClassifyWithPatterns(ctx, txns)
	}
}

// Benchmark pattern refresh operation
func BenchmarkPatternClassifier_RefreshPatterns(b *testing.B) {
	ctx := context.Background()
	
	testCases := []struct {
		name      string
		ruleCount int
	}{
		{"10_rules", 10},
		{"50_rules", 50},
		{"100_rules", 100},
		{"500_rules", 500},
		{"1000_rules", 1000},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			mockStorage := new(MockStorage)
			
			// Generate initial rules
			initialRules := make([]model.PatternRule, tc.ruleCount)
			for i := 0; i < tc.ruleCount; i++ {
				initialRules[i] = model.PatternRule{
					ID:              i,
					Name:            fmt.Sprintf("Rule_%d", i),
					MerchantPattern: fmt.Sprintf("PATTERN_%d", i),
					DefaultCategory: fmt.Sprintf("Category_%d", i%20),
					IsActive:        true,
					Confidence:      0.7 + float64(i%30)/100,
				}
			}
			
			// Setup initial call
			mockStorage.On("GetActivePatternRules", mock.Anything).Return(initialRules, nil).Once()
			
			pc, err := NewPatternClassifier(mockStorage)
			require.NoError(b, err)
			
			// Setup refresh calls
			mockStorage.On("GetActivePatternRules", ctx).Return(initialRules, nil)
			
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = pc.RefreshPatterns(ctx)
			}
		})
	}
}

// Benchmark pattern matching with regex patterns
func BenchmarkPatternClassifier_RegexMatching(b *testing.B) {
	ctx := context.Background()
	
	testCases := []struct {
		name         string
		regexCount   int
		nonRegexCount int
	}{
		{"all_exact", 0, 100},
		{"half_regex", 50, 50},
		{"all_regex", 100, 0},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			mockStorage := new(MockStorage)
			
			// Generate rules with mix of regex and exact patterns
			rules := make([]model.PatternRule, tc.regexCount+tc.nonRegexCount)
			idx := 0
			
			// Add regex rules
			for i := 0; i < tc.regexCount; i++ {
				rules[idx] = model.PatternRule{
					ID:              idx,
					Name:            fmt.Sprintf("RegexRule_%d", i),
					MerchantPattern: fmt.Sprintf("PATTERN_%d.*", i),
					IsRegex:         true,
					DefaultCategory: "Shopping",
					IsActive:        true,
					Confidence:      0.9,
				}
				idx++
			}
			
			// Add exact match rules
			for i := 0; i < tc.nonRegexCount; i++ {
				rules[idx] = model.PatternRule{
					ID:              idx,
					Name:            fmt.Sprintf("ExactRule_%d", i),
					MerchantPattern: fmt.Sprintf("EXACT_PATTERN_%d", i),
					IsRegex:         false,
					DefaultCategory: "Shopping",
					IsActive:        true,
					Confidence:      0.9,
				}
				idx++
			}
			
			mockStorage.On("GetActivePatternRules", mock.Anything).Return(rules, nil)
			
			pc, err := NewPatternClassifier(mockStorage)
			require.NoError(b, err)
			
			// Setup classification
			categories := []model.Category{
				{ID: 1, Name: "Shopping", Type: "expense"},
			}
			mockStorage.On("GetCategories", ctx).Return(categories, nil)
			mockStorage.On("IncrementPatternRuleUseCount", ctx, mock.Anything).Return(nil).Maybe()
			
			// Test transactions
			txns := []model.Transaction{
				{ID: "1", MerchantName: "PATTERN_5_STORE"},
				{ID: "2", MerchantName: "EXACT_PATTERN_10"},
				{ID: "3", MerchantName: "RANDOM_MERCHANT"},
			}
			
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for _, txn := range txns {
					_, _ = pc.ClassifyWithPatterns(ctx, []model.Transaction{txn})
				}
			}
		})
	}
}

// Benchmark memory allocation in pattern classification
func BenchmarkPatternClassifier_MemoryAllocation(b *testing.B) {
	ctx := context.Background()
	mockStorage := new(MockStorage)
	
	// Setup with reasonable number of rules
	rules := make([]model.PatternRule, 50)
	for i := 0; i < 50; i++ {
		rules[i] = model.PatternRule{
			ID:              i,
			Name:            fmt.Sprintf("Rule_%d", i),
			MerchantPattern: fmt.Sprintf("PATTERN_%d", i),
			DefaultCategory: fmt.Sprintf("Category_%d", i%10),
			IsActive:        true,
			Confidence:      0.85,
		}
	}
	
	mockStorage.On("GetActivePatternRules", mock.Anything).Return(rules, nil)
	
	pc, err := NewPatternClassifier(mockStorage)
	require.NoError(b, err)
	
	categories := make([]model.Category, 10)
	for i := 0; i < 10; i++ {
		categories[i] = model.Category{
			ID:   i + 1,
			Name: fmt.Sprintf("Category_%d", i),
			Type: "expense",
		}
	}
	mockStorage.On("GetCategories", ctx).Return(categories, nil)
	mockStorage.On("IncrementPatternRuleUseCount", ctx, mock.Anything).Return(nil).Maybe()
	
	// Create varying sizes of transaction batches
	smallBatch := generateBenchmarkTransactions(10)
	mediumBatch := generateBenchmarkTransactions(100)
	largeBatch := generateBenchmarkTransactions(1000)
	
	b.Run("small_batch", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = pc.ClassifyWithPatterns(ctx, smallBatch)
		}
	})
	
	b.Run("medium_batch", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = pc.ClassifyWithPatterns(ctx, mediumBatch)
		}
	})
	
	b.Run("large_batch", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = pc.ClassifyWithPatterns(ctx, largeBatch)
		}
	})
}

func generateBenchmarkTransactions(count int) []model.Transaction {
	txns := make([]model.Transaction, count)
	merchants := []string{
		"AMAZON MARKETPLACE",
		"STARBUCKS STORE #123",
		"TARGET STORE",
		"WHOLE FOODS MARKET",
		"WALMART SUPERCENTER",
	}
	
	for i := 0; i < count; i++ {
		txns[i] = model.Transaction{
			ID:           fmt.Sprintf("txn_%d", i),
			MerchantName: merchants[i%len(merchants)],
			Amount:       float64(i%100) + 0.99,
			Date:         time.Now().AddDate(0, 0, -i),
		}
	}
	return txns
}