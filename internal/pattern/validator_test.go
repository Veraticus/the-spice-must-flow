package pattern

import (
	"context"
	"testing"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestValidator_ValidateDirection(t *testing.T) {
	ctx := context.Background()
	validator := NewValidator()

	tests := []struct {
		name     string
		errMsg   string
		category model.Category
		txn      model.Transaction
		wantErr  bool
	}{
		{
			name: "income transaction with income category",
			txn: model.Transaction{
				Direction: model.DirectionIncome,
				Amount:    100.0,
			},
			category: model.Category{
				Name: "Salary",
				Type: model.CategoryTypeIncome,
			},
			wantErr: false,
		},
		{
			name: "expense transaction with expense category",
			txn: model.Transaction{
				Direction: model.DirectionExpense,
				Amount:    50.0,
			},
			category: model.Category{
				Name: "Shopping",
				Type: model.CategoryTypeExpense,
			},
			wantErr: false,
		},
		{
			name: "transfer transaction with system category",
			txn: model.Transaction{
				Direction: model.DirectionTransfer,
				Amount:    100.0,
			},
			category: model.Category{
				Name: "Transfer",
				Type: model.CategoryTypeSystem,
			},
			wantErr: false,
		},
		{
			name: "income transaction with expense category - error",
			txn: model.Transaction{
				Direction: model.DirectionIncome,
				Amount:    100.0,
			},
			category: model.Category{
				Name: "Shopping",
				Type: model.CategoryTypeExpense,
			},
			wantErr: true,
			errMsg:  `category "Shopping" has type expense but transaction has direction income`,
		},
		{
			name: "expense transaction with income category - error",
			txn: model.Transaction{
				Direction: model.DirectionExpense,
				Amount:    50.0,
			},
			category: model.Category{
				Name: "Salary",
				Type: model.CategoryTypeIncome,
			},
			wantErr: true,
			errMsg:  `category "Salary" has type income but transaction has direction expense`,
		},
		{
			name: "system category with any direction - income",
			txn: model.Transaction{
				Direction: model.DirectionIncome,
				Amount:    100.0,
			},
			category: model.Category{
				Name: "Transfer",
				Type: model.CategoryTypeSystem,
			},
			wantErr: false,
		},
		{
			name: "system category with any direction - expense",
			txn: model.Transaction{
				Direction: model.DirectionExpense,
				Amount:    100.0,
			},
			category: model.Category{
				Name: "Transfer",
				Type: model.CategoryTypeSystem,
			},
			wantErr: false,
		},
		{
			name: "no direction set - positive amount defaults to expense",
			txn: model.Transaction{
				Amount: 50.0,
			},
			category: model.Category{
				Name: "Shopping",
				Type: model.CategoryTypeExpense,
			},
			wantErr: false,
		},
		{
			name: "no direction set - negative amount defaults to income",
			txn: model.Transaction{
				Amount: -50.0,
			},
			category: model.Category{
				Name: "Refund",
				Type: model.CategoryTypeIncome,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateDirection(ctx, tt.txn, tt.category)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Equal(t, tt.errMsg, err.Error())
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidator_ValidateSuggestions(t *testing.T) {
	ctx := context.Background()
	validator := NewValidator()

	categories := []model.Category{
		{Name: "Salary", Type: model.CategoryTypeIncome},
		{Name: "Shopping", Type: model.CategoryTypeExpense},
		{Name: "Transfer", Type: model.CategoryTypeSystem},
	}

	tests := []struct {
		name        string
		errMsg      string
		suggestions []Suggestion
		txn         model.Transaction
		wantErr     bool
	}{
		{
			name: "all valid suggestions",
			txn: model.Transaction{
				Direction: model.DirectionExpense,
				Amount:    50.0,
			},
			suggestions: []Suggestion{
				{Category: "Shopping", Confidence: 0.9},
				{Category: "Transfer", Confidence: 0.5},
			},
			wantErr: false,
		},
		{
			name: "invalid suggestion - wrong category type",
			txn: model.Transaction{
				Direction: model.DirectionExpense,
				Amount:    50.0,
			},
			suggestions: []Suggestion{
				{Category: "Salary", Confidence: 0.9},
			},
			wantErr: true,
			errMsg:  `invalid suggestion: category "Salary" has type income but transaction has direction expense`,
		},
		{
			name: "unknown category in suggestion",
			txn: model.Transaction{
				Direction: model.DirectionExpense,
				Amount:    50.0,
			},
			suggestions: []Suggestion{
				{Category: "UnknownCategory", Confidence: 0.9},
			},
			wantErr: true,
			errMsg:  `suggestion references unknown category "UnknownCategory"`,
		},
		{
			name: "empty suggestions list",
			txn: model.Transaction{
				Direction: model.DirectionExpense,
				Amount:    50.0,
			},
			suggestions: []Suggestion{},
			wantErr:     false,
		},
		{
			name: "mixed valid and invalid suggestions",
			txn: model.Transaction{
				Direction: model.DirectionIncome,
				Amount:    100.0,
			},
			suggestions: []Suggestion{
				{Category: "Salary", Confidence: 0.9},
				{Category: "Shopping", Confidence: 0.5}, // Invalid
			},
			wantErr: true,
			errMsg:  `invalid suggestion: category "Shopping" has type expense but transaction has direction income`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateSuggestions(ctx, tt.txn, tt.suggestions, categories)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Equal(t, tt.errMsg, err.Error())
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
