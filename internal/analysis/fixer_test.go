package analysis

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/service"
)

// mockTransaction for testing transaction interface.
type mockTransaction struct {
	service.Transaction
	mock.Mock
}

func (m *mockTransaction) BeginTx(ctx context.Context) (service.Transaction, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	if v, ok := args.Get(0).(service.Transaction); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockTransaction) Commit() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockTransaction) Rollback() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockTransaction) CreatePatternRule(ctx context.Context, rule *model.PatternRule) error {
	args := m.Called(ctx, rule)
	return args.Error(0)
}

func (m *mockTransaction) GetCategories(ctx context.Context) ([]model.Category, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	if v, ok := args.Get(0).([]model.Category); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockTransaction) GetTransactionByID(ctx context.Context, id string) (*model.Transaction, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	if v, ok := args.Get(0).(*model.Transaction); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockTransaction) SaveClassification(ctx context.Context, classification *model.Classification) error {
	args := m.Called(ctx, classification)
	return args.Error(0)
}

// Extended mockStorage for fixer tests.
type mockStorageForFixer struct {
	service.Storage
	mock.Mock
}

func (m *mockStorageForFixer) BeginTx(ctx context.Context) (service.Transaction, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	if v, ok := args.Get(0).(service.Transaction); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockStorageForFixer) GetTransactionByID(ctx context.Context, id string) (*model.Transaction, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	if v, ok := args.Get(0).(*model.Transaction); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}

func TestTransactionalFixApplier_ApplyPatternFixes(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		setupMocks    func(*mockStorageForFixer, *mockTransaction)
		name          string
		errorContains string
		patterns      []SuggestedPattern
		wantErr       bool
	}{
		{
			name:     "empty patterns list",
			patterns: []SuggestedPattern{},
			setupMocks: func(_ *mockStorageForFixer, _ *mockTransaction) {
				// No calls expected
			},
			wantErr: false,
		},
		{
			name: "successful pattern creation",
			patterns: []SuggestedPattern{
				{
					ID:          "pattern-1",
					Name:        "Starbucks Pattern",
					Description: "Matches Starbucks transactions",
					Pattern: model.PatternRule{
						Name:            "Starbucks Pattern",
						MerchantPattern: "STARBUCKS",
						DefaultCategory: "Dining",
						Confidence:      0.95,
						Priority:        10,
						IsActive:        true,
					},
					Confidence: 0.95,
				},
			},
			setupMocks: func(storage *mockStorageForFixer, tx *mockTransaction) {
				storage.On("BeginTx", ctx).Return(tx, nil)

				categories := []model.Category{
					{Name: "Dining", Type: model.CategoryTypeExpense, IsActive: true},
					{Name: "Groceries", Type: model.CategoryTypeExpense, IsActive: true},
				}
				tx.On("GetCategories", ctx).Return(categories, nil)
				tx.On("CreatePatternRule", ctx, mock.MatchedBy(func(rule *model.PatternRule) bool {
					return rule.Name == "Starbucks Pattern" &&
						rule.MerchantPattern == "STARBUCKS" &&
						rule.DefaultCategory == "Dining"
				})).Return(nil)
				tx.On("Commit").Return(nil)
				tx.On("Rollback").Return(nil)

			},
			wantErr: false,
		},
		{
			name: "transaction begin error",
			patterns: []SuggestedPattern{
				{
					Pattern: model.PatternRule{
						Name:            "Test Pattern",
						MerchantPattern: "TEST",
						DefaultCategory: "Test",
					},
				},
			},
			setupMocks: func(storage *mockStorageForFixer, _ *mockTransaction) {
				storage.On("BeginTx", ctx).Return(nil, errors.New("database error"))
			},
			wantErr:       true,
			errorContains: "failed to begin transaction",
		},
		{
			name: "invalid pattern rule",
			patterns: []SuggestedPattern{
				{
					Pattern: model.PatternRule{
						Name:            "", // Invalid: empty name
						MerchantPattern: "TEST",
						DefaultCategory: "Test",
					},
				},
			},
			setupMocks: func(storage *mockStorageForFixer, tx *mockTransaction) {
				storage.On("BeginTx", ctx).Return(tx, nil)
				tx.On("Rollback").Return(nil)
			},
			wantErr:       true,
			errorContains: "pattern name is required",
		},
		{
			name: "category does not exist",
			patterns: []SuggestedPattern{
				{
					Pattern: model.PatternRule{
						Name:            "Test Pattern",
						MerchantPattern: "TEST",
						DefaultCategory: "NonExistent",
						Confidence:      0.8,
					},
				},
			},
			setupMocks: func(storage *mockStorageForFixer, tx *mockTransaction) {
				storage.On("BeginTx", ctx).Return(tx, nil)

				categories := []model.Category{
					{Name: "Dining", Type: model.CategoryTypeExpense, IsActive: true},
				}
				tx.On("GetCategories", ctx).Return(categories, nil)
				tx.On("Rollback").Return(nil)
			},
			wantErr:       true,
			errorContains: "category \"NonExistent\" does not exist or is inactive",
		},
		{
			name: "commit error",
			patterns: []SuggestedPattern{
				{
					Pattern: model.PatternRule{
						Name:            "Test Pattern",
						MerchantPattern: "TEST",
						DefaultCategory: "Dining",
						Confidence:      0.8,
					},
				},
			},
			setupMocks: func(storage *mockStorageForFixer, tx *mockTransaction) {
				storage.On("BeginTx", ctx).Return(tx, nil)

				categories := []model.Category{
					{Name: "Dining", Type: model.CategoryTypeExpense, IsActive: true},
				}
				tx.On("GetCategories", ctx).Return(categories, nil)
				tx.On("CreatePatternRule", ctx, mock.Anything).Return(nil)
				tx.On("Commit").Return(errors.New("commit failed"))
				tx.On("Rollback").Return(nil)
			},
			wantErr:       true,
			errorContains: "failed to commit pattern fixes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := new(mockStorageForFixer)
			tx := new(mockTransaction)
			tt.setupMocks(storage, tx)

			// Pass nil pattern engine for tests
			fixer := NewTransactionalFixApplier(storage, nil)

			err := fixer.ApplyPatternFixes(ctx, tt.patterns)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}

			storage.AssertExpectations(t)
			tx.AssertExpectations(t)
		})
	}
}

func TestTransactionalFixApplier_ApplyCategoryFixes(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		setupMocks    func(*mockStorageForFixer, *mockTransaction)
		name          string
		errorContains string
		fixes         []Fix
		wantErr       bool
	}{
		{
			name:  "empty fixes list",
			fixes: []Fix{},
			setupMocks: func(_ *mockStorageForFixer, _ *mockTransaction) {
				// No calls expected
			},
			wantErr: false,
		},
		{
			name: "successful category fix",
			fixes: []Fix{
				{
					ID:          "fix-1",
					Type:        "category_update",
					Description: "Update to correct category",
					Data: map[string]any{
						"category":        "Groceries",
						"transaction_ids": []any{"txn-1", "txn-2"},
					},
				},
			},
			setupMocks: func(storage *mockStorageForFixer, tx *mockTransaction) {
				storage.On("BeginTx", ctx).Return(tx, nil)

				categories := []model.Category{
					{Name: "Groceries", Type: model.CategoryTypeExpense, IsActive: true},
					{Name: "Dining", Type: model.CategoryTypeExpense, IsActive: true},
				}
				tx.On("GetCategories", ctx).Return(categories, nil)

				// Mock transaction retrievals
				tx.On("GetTransactionByID", ctx, "txn-1").Return(&model.Transaction{
					ID:     "txn-1",
					Amount: 50.00,
					Name:   "WHOLE FOODS",
				}, nil)
				tx.On("GetTransactionByID", ctx, "txn-2").Return(&model.Transaction{
					ID:     "txn-2",
					Amount: 75.00,
					Name:   "WHOLE FOODS",
				}, nil)

				// Mock classification saves
				tx.On("SaveClassification", ctx, mock.MatchedBy(func(c *model.Classification) bool {
					return c.Category == "Groceries" && c.Status == model.StatusUserModified
				})).Return(nil).Times(2)

				tx.On("Commit").Return(nil)
				tx.On("Rollback").Return(nil)
			},
			wantErr: false,
		},
		{
			name: "fix missing category data",
			fixes: []Fix{
				{
					ID:   "fix-1",
					Type: "category_update",
					Data: map[string]any{
						"transaction_ids": []any{"txn-1"},
						// Missing "category" field
					},
				},
			},
			setupMocks: func(storage *mockStorageForFixer, tx *mockTransaction) {
				storage.On("BeginTx", ctx).Return(tx, nil)

				categories := []model.Category{
					{Name: "Groceries", Type: model.CategoryTypeExpense, IsActive: true},
				}
				tx.On("GetCategories", ctx).Return(categories, nil)
				tx.On("Rollback").Return(nil)
			},
			wantErr:       true,
			errorContains: "missing category data",
		},
		{
			name: "category does not exist",
			fixes: []Fix{
				{
					ID:   "fix-1",
					Type: "category_update",
					Data: map[string]any{
						"category":        "NonExistent",
						"transaction_ids": []any{"txn-1"},
					},
				},
			},
			setupMocks: func(storage *mockStorageForFixer, tx *mockTransaction) {
				storage.On("BeginTx", ctx).Return(tx, nil)

				categories := []model.Category{
					{Name: "Groceries", Type: model.CategoryTypeExpense, IsActive: true},
				}
				tx.On("GetCategories", ctx).Return(categories, nil)
				tx.On("Rollback").Return(nil)
			},
			wantErr:       true,
			errorContains: "category \"NonExistent\" does not exist or is inactive",
		},
		{
			name: "transaction not found",
			fixes: []Fix{
				{
					ID:   "fix-1",
					Type: "category_update",
					Data: map[string]any{
						"category":        "Groceries",
						"transaction_ids": []any{"txn-999"},
					},
				},
			},
			setupMocks: func(storage *mockStorageForFixer, tx *mockTransaction) {
				storage.On("BeginTx", ctx).Return(tx, nil)

				categories := []model.Category{
					{Name: "Groceries", Type: model.CategoryTypeExpense, IsActive: true},
				}
				tx.On("GetCategories", ctx).Return(categories, nil)
				tx.On("GetTransactionByID", ctx, "txn-999").Return(nil, errors.New("not found"))
				tx.On("Rollback").Return(nil)
			},
			wantErr:       true,
			errorContains: "failed to get transaction txn-999",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := new(mockStorageForFixer)
			tx := new(mockTransaction)

			tt.setupMocks(storage, tx)

			fixer := NewTransactionalFixApplier(storage, nil)
			err := fixer.ApplyCategoryFixes(ctx, tt.fixes)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}

			storage.AssertExpectations(t)
			tx.AssertExpectations(t)
		})
	}
}

func TestTransactionalFixApplier_ApplyRecategorizations(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		setupMocks    func(*mockStorageForFixer, *mockTransaction)
		name          string
		errorContains string
		issues        []Issue
		wantErr       bool
	}{
		{
			name:   "empty issues list",
			issues: []Issue{},
			setupMocks: func(_ *mockStorageForFixer, _ *mockTransaction) {
				// No calls expected
			},
			wantErr: false,
		},
		{
			name: "successful recategorization",
			issues: []Issue{
				{
					ID:                "issue-1",
					Type:              IssueTypeMiscategorized,
					CurrentCategory:   strPtr("Dining"),
					SuggestedCategory: strPtr("Groceries"),
					TransactionIDs:    []string{"txn-1", "txn-2"},
					Confidence:        0.85,
				},
			},
			setupMocks: func(storage *mockStorageForFixer, tx *mockTransaction) {
				storage.On("BeginTx", ctx).Return(tx, nil)

				categories := []model.Category{
					{Name: "Groceries", Type: model.CategoryTypeExpense, IsActive: true},
					{Name: "Dining", Type: model.CategoryTypeExpense, IsActive: true},
				}
				tx.On("GetCategories", ctx).Return(categories, nil)

				// Mock transaction retrievals
				tx.On("GetTransactionByID", ctx, "txn-1").Return(&model.Transaction{
					ID:     "txn-1",
					Amount: 50.00,
					Name:   "WHOLE FOODS",
				}, nil)
				tx.On("GetTransactionByID", ctx, "txn-2").Return(&model.Transaction{
					ID:     "txn-2",
					Amount: 75.00,
					Name:   "WHOLE FOODS",
				}, nil)

				// Mock classification saves
				tx.On("SaveClassification", ctx, mock.MatchedBy(func(c *model.Classification) bool {
					return c.Category == "Groceries" &&
						c.Status == model.StatusUserModified &&
						c.Confidence == 0.85
				})).Return(nil).Times(2)

				tx.On("Commit").Return(nil)
				tx.On("Rollback").Return(nil)
			},
			wantErr: false,
		},
		{
			name: "skip non-miscategorized issues",
			issues: []Issue{
				{
					ID:                "issue-1",
					Type:              IssueTypeMissingPattern, // Not miscategorized
					SuggestedCategory: strPtr("Groceries"),
					TransactionIDs:    []string{"txn-1"},
				},
			},
			setupMocks: func(storage *mockStorageForFixer, tx *mockTransaction) {
				storage.On("BeginTx", ctx).Return(tx, nil)

				categories := []model.Category{
					{Name: "Groceries", Type: model.CategoryTypeExpense, IsActive: true},
				}
				tx.On("GetCategories", ctx).Return(categories, nil)
				tx.On("Commit").Return(nil)
				tx.On("Rollback").Return(nil)
			},
			wantErr: false,
		},
		{
			name: "skip issues without suggested category",
			issues: []Issue{
				{
					ID:                "issue-1",
					Type:              IssueTypeMiscategorized,
					CurrentCategory:   strPtr("Dining"),
					SuggestedCategory: nil, // No suggestion
					TransactionIDs:    []string{"txn-1"},
				},
			},
			setupMocks: func(storage *mockStorageForFixer, tx *mockTransaction) {
				storage.On("BeginTx", ctx).Return(tx, nil)

				categories := []model.Category{
					{Name: "Dining", Type: model.CategoryTypeExpense, IsActive: true},
				}
				tx.On("GetCategories", ctx).Return(categories, nil)
				tx.On("Commit").Return(nil)
				tx.On("Rollback").Return(nil)
			},
			wantErr: false,
		},
		{
			name: "skip invalid category",
			issues: []Issue{
				{
					ID:                "issue-1",
					Type:              IssueTypeMiscategorized,
					CurrentCategory:   strPtr("Dining"),
					SuggestedCategory: strPtr("NonExistent"),
					TransactionIDs:    []string{"txn-1"},
				},
			},
			setupMocks: func(storage *mockStorageForFixer, tx *mockTransaction) {
				storage.On("BeginTx", ctx).Return(tx, nil)

				categories := []model.Category{
					{Name: "Dining", Type: model.CategoryTypeExpense, IsActive: true},
				}
				tx.On("GetCategories", ctx).Return(categories, nil)
				tx.On("Commit").Return(nil)
				tx.On("Rollback").Return(nil)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := new(mockStorageForFixer)
			tx := new(mockTransaction)

			tt.setupMocks(storage, tx)

			fixer := NewTransactionalFixApplier(storage, nil)
			err := fixer.ApplyRecategorizations(ctx, tt.issues)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}

			storage.AssertExpectations(t)
			tx.AssertExpectations(t)
		})
	}
}

func TestTransactionalFixApplier_PreviewFix(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		setupMocks    func(*mockStorageForFixer)
		wantPreview   *FixPreview
		name          string
		errorContains string
		fix           Fix
		wantErr       bool
	}{
		{
			name: "preview category fix",
			fix: Fix{
				ID:   "fix-1",
				Type: "category_update",
				Data: map[string]any{
					"category":        "Groceries",
					"transaction_ids": []any{"txn-1", "txn-2"},
				},
			},
			setupMocks: func(storage *mockStorageForFixer) {
				// Mock transaction retrievals
				storage.On("GetTransactionByID", ctx, "txn-1").Return(&model.Transaction{
					ID:     "txn-1",
					Amount: 50.00,
					Name:   "WHOLE FOODS",
				}, nil)
				storage.On("GetTransactionByID", ctx, "txn-2").Return(&model.Transaction{
					ID:     "txn-2",
					Amount: 75.00,
					Name:   "WHOLE FOODS",
				}, nil)

				// No classification retrieval needed - preview shows all as unclassified
			},
			wantPreview: &FixPreview{
				FixID: "fix-1",
				Changes: []PreviewChange{
					{
						TransactionID: "txn-1",
						FieldName:     "category",
						OldValue:      "unclassified",
						NewValue:      "Groceries",
					},
					{
						TransactionID: "txn-2",
						FieldName:     "category",
						OldValue:      "unclassified",
						NewValue:      "Groceries",
					},
				},
				AffectedCount: 2,
				EstimatedImpact: map[string]float64{
					"total_amount":      125.00,
					"transaction_count": 2,
				},
			},
			wantErr: false,
		},
		{
			name: "preview pattern fix",
			fix: Fix{
				ID:   "fix-2",
				Type: "pattern_creation",
				Data: map[string]any{
					"pattern_name": "Starbucks Pattern",
					"match_count":  float64(15),
				},
			},
			setupMocks: func(_ *mockStorageForFixer) {
				// No storage calls for pattern preview
			},
			wantPreview: &FixPreview{
				FixID: "fix-2",
				Changes: []PreviewChange{
					{
						TransactionID: "",
						FieldName:     "pattern_rule",
						OldValue:      "none",
						NewValue:      "Starbucks Pattern",
					},
				},
				AffectedCount: 15,
				EstimatedImpact: map[string]float64{
					"future_matches": 15,
				},
			},
			wantErr: false,
		},
		{
			name: "unknown fix type",
			fix: Fix{
				ID:   "fix-3",
				Type: "unknown_type",
				Data: map[string]any{},
			},
			setupMocks: func(_ *mockStorageForFixer) {
				// No storage calls expected
			},
			wantErr:       true,
			errorContains: "unknown fix type",
		},
		{
			name: "category fix missing data",
			fix: Fix{
				ID:   "fix-4",
				Type: "category_update",
				Data: map[string]any{
					"transaction_ids": []any{"txn-1"},
					// Missing "category" field
				},
			},
			setupMocks: func(_ *mockStorageForFixer) {
				// No storage calls expected
			},
			wantErr:       true,
			errorContains: "missing category data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := new(mockStorageForFixer)

			tt.setupMocks(storage)

			fixer := NewTransactionalFixApplier(storage, nil)
			preview, err := fixer.PreviewFix(ctx, tt.fix)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantPreview.FixID, preview.FixID)
				assert.Equal(t, tt.wantPreview.AffectedCount, preview.AffectedCount)
				assert.Equal(t, len(tt.wantPreview.Changes), len(preview.Changes))

				// Compare changes
				for i, expectedChange := range tt.wantPreview.Changes {
					assert.Equal(t, expectedChange.TransactionID, preview.Changes[i].TransactionID)
					assert.Equal(t, expectedChange.FieldName, preview.Changes[i].FieldName)
					assert.Equal(t, expectedChange.OldValue, preview.Changes[i].OldValue)
					assert.Equal(t, expectedChange.NewValue, preview.Changes[i].NewValue)
				}

				// Compare estimated impact
				for key, expectedValue := range tt.wantPreview.EstimatedImpact {
					assert.Equal(t, expectedValue, preview.EstimatedImpact[key])
				}
			}

			storage.AssertExpectations(t)
		})
	}
}

func TestValidatePatternRule(t *testing.T) {
	fixer := &TransactionalFixApplier{}

	tests := []struct {
		rule          *model.PatternRule
		name          string
		errorContains string
		wantErr       bool
	}{
		{
			name: "valid pattern rule",
			rule: &model.PatternRule{
				Name:            "Test Pattern",
				MerchantPattern: "TEST*",
				DefaultCategory: "TestCategory",
				Confidence:      0.85,
				Priority:        10,
			},
			wantErr: false,
		},
		{
			name: "missing name",
			rule: &model.PatternRule{
				Name:            "",
				MerchantPattern: "TEST*",
				DefaultCategory: "TestCategory",
				Confidence:      0.85,
			},
			wantErr:       true,
			errorContains: "pattern name is required",
		},
		{
			name: "missing merchant pattern",
			rule: &model.PatternRule{
				Name:            "Test Pattern",
				MerchantPattern: "",
				DefaultCategory: "TestCategory",
				Confidence:      0.85,
			},
			wantErr:       true,
			errorContains: "merchant pattern is required",
		},
		{
			name: "missing default category",
			rule: &model.PatternRule{
				Name:            "Test Pattern",
				MerchantPattern: "TEST*",
				DefaultCategory: "",
				Confidence:      0.85,
			},
			wantErr:       true,
			errorContains: "default category is required",
		},
		{
			name: "confidence too low",
			rule: &model.PatternRule{
				Name:            "Test Pattern",
				MerchantPattern: "TEST*",
				DefaultCategory: "TestCategory",
				Confidence:      -0.1,
			},
			wantErr:       true,
			errorContains: "confidence must be between 0 and 1",
		},
		{
			name: "confidence too high",
			rule: &model.PatternRule{
				Name:            "Test Pattern",
				MerchantPattern: "TEST*",
				DefaultCategory: "TestCategory",
				Confidence:      1.1,
			},
			wantErr:       true,
			errorContains: "confidence must be between 0 and 1",
		},
		{
			name: "negative priority",
			rule: &model.PatternRule{
				Name:            "Test Pattern",
				MerchantPattern: "TEST*",
				DefaultCategory: "TestCategory",
				Confidence:      0.85,
				Priority:        -5,
			},
			wantErr:       true,
			errorContains: "priority must be non-negative",
		},
		{
			name: "sets defaults for new pattern",
			rule: &model.PatternRule{
				Name:            "Test Pattern",
				MerchantPattern: "TEST*",
				DefaultCategory: "TestCategory",
				Confidence:      0.85,
				Priority:        10,
				// CreatedAt, UpdatedAt, and IsActive not set
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := fixer.validatePatternRule(tt.rule)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				// Check defaults were set
				if tt.name == "sets defaults for new pattern" {
					assert.False(t, tt.rule.CreatedAt.IsZero())
					assert.False(t, tt.rule.UpdatedAt.IsZero())
					assert.True(t, tt.rule.IsActive)
				}
			}
		})
	}
}

func TestValueOrDefault(t *testing.T) {
	tests := []struct {
		name         string
		value        *string
		defaultValue string
		want         string
	}{
		{
			name:         "nil value returns default",
			value:        nil,
			defaultValue: "default",
			want:         "default",
		},
		{
			name:         "non-nil value returns value",
			value:        strPtr("actual"),
			defaultValue: "default",
			want:         "actual",
		},
		{
			name:         "empty string value returns empty string",
			value:        strPtr(""),
			defaultValue: "default",
			want:         "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := valueOrDefault(tt.value, tt.defaultValue)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Helper function to create string pointers.
func strPtr(s string) *string {
	return &s
}
