package pattern

import (
	"context"
	"testing"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestMatcher_Match(t *testing.T) {
	ctx := context.Background()

	// Helper function to create float64 pointer
	floatPtr := func(f float64) *float64 { return &f }
	dirPtr := func(d model.TransactionDirection) *model.TransactionDirection { return &d }

	tests := []struct {
		name    string
		rules   []Rule
		wantIDs []int
		txn     model.Transaction
		wantErr bool
	}{
		{
			name: "exact merchant match",
			rules: []Rule{
				{
					ID:              1,
					MerchantPattern: "Amazon",
					AmountCondition: "any",
					DefaultCategory: "Shopping",
					Confidence:      0.9,
					Priority:        10,
					IsActive:        true,
				},
			},
			txn: model.Transaction{
				MerchantName: "Amazon",
				Amount:       50.00,
			},
			wantIDs: []int{1},
		},
		{
			name: "case insensitive merchant match",
			rules: []Rule{
				{
					ID:              1,
					MerchantPattern: "amazon",
					AmountCondition: "any",
					DefaultCategory: "Shopping",
					IsActive:        true,
				},
			},
			txn: model.Transaction{
				MerchantName: "AMAZON",
				Amount:       50.00,
			},
			wantIDs: []int{1},
		},
		{
			name: "regex merchant match",
			rules: []Rule{
				{
					ID:              1,
					MerchantPattern: ".*coffee.*",
					IsRegex:         true,
					AmountCondition: "any",
					DefaultCategory: "Dining",
					IsActive:        true,
				},
			},
			txn: model.Transaction{
				MerchantName: "Starbucks Coffee",
				Amount:       5.00,
			},
			wantIDs: []int{1},
		},
		{
			name: "amount less than condition",
			rules: []Rule{
				{
					ID:              1,
					MerchantPattern: "Amazon",
					AmountCondition: "lt",
					AmountValue:     floatPtr(20.0),
					DefaultCategory: "Shopping - Small",
					IsActive:        true,
				},
			},
			txn: model.Transaction{
				MerchantName: "Amazon",
				Amount:       15.00,
			},
			wantIDs: []int{1},
		},
		{
			name: "amount range condition",
			rules: []Rule{
				{
					ID:              1,
					MerchantPattern: "Restaurant",
					AmountCondition: "range",
					AmountMin:       floatPtr(10.0),
					AmountMax:       floatPtr(50.0),
					DefaultCategory: "Dining",
					IsActive:        true,
				},
			},
			txn: model.Transaction{
				MerchantName: "Restaurant",
				Amount:       25.00,
			},
			wantIDs: []int{1},
		},
		{
			name: "direction filter - income",
			rules: []Rule{
				{
					ID:              1,
					MerchantPattern: "Employer",
					AmountCondition: "any",
					Direction:       dirPtr(model.DirectionIncome),
					DefaultCategory: "Salary",
					IsActive:        true,
				},
			},
			txn: model.Transaction{
				MerchantName: "Employer",
				Amount:       5000.00,
				Direction:    model.DirectionIncome,
			},
			wantIDs: []int{1},
		},
		{
			name: "direction filter - no match",
			rules: []Rule{
				{
					ID:              1,
					MerchantPattern: "Amazon",
					AmountCondition: "any",
					Direction:       dirPtr(model.DirectionIncome),
					DefaultCategory: "Refund",
					IsActive:        true,
				},
			},
			txn: model.Transaction{
				MerchantName: "Amazon",
				Amount:       50.00,
				Direction:    model.DirectionExpense,
			},
			wantIDs: []int{},
		},
		{
			name: "multiple matches sorted by priority",
			rules: []Rule{
				{
					ID:              1,
					MerchantPattern: "Amazon",
					AmountCondition: "any",
					DefaultCategory: "Shopping",
					Priority:        5,
					IsActive:        true,
				},
				{
					ID:              2,
					MerchantPattern: "Amazon",
					AmountCondition: "lt",
					AmountValue:     floatPtr(10.0),
					DefaultCategory: "Shopping - Small",
					Priority:        10,
					IsActive:        true,
				},
			},
			txn: model.Transaction{
				MerchantName: "Amazon",
				Amount:       5.00,
			},
			wantIDs: []int{2, 1}, // Higher priority first
		},
		{
			name: "inactive rule ignored",
			rules: []Rule{
				{
					ID:              1,
					MerchantPattern: "Amazon",
					AmountCondition: "any",
					DefaultCategory: "Shopping",
					IsActive:        false,
				},
			},
			txn: model.Transaction{
				MerchantName: "Amazon",
				Amount:       50.00,
			},
			wantIDs: []int{},
		},
		{
			name: "empty merchant pattern matches all",
			rules: []Rule{
				{
					ID:              1,
					MerchantPattern: "",
					AmountCondition: "gt",
					AmountValue:     floatPtr(100.0),
					DefaultCategory: "Large Purchase",
					IsActive:        true,
				},
			},
			txn: model.Transaction{
				MerchantName: "Any Store",
				Amount:       150.00,
			},
			wantIDs: []int{1},
		},
		{
			name: "use transaction name when merchant name empty",
			rules: []Rule{
				{
					ID:              1,
					MerchantPattern: "Store",
					AmountCondition: "any",
					DefaultCategory: "Shopping",
					IsActive:        true,
				},
			},
			txn: model.Transaction{
				Name:   "Store Purchase",
				Amount: 50.00,
			},
			wantIDs: []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher := NewMatcher(tt.rules)
			matches, err := matcher.Match(ctx, tt.txn)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)

			// Extract rule IDs from matches
			gotIDs := make([]int, len(matches))
			for i, match := range matches {
				gotIDs[i] = match.ID
			}

			assert.Equal(t, tt.wantIDs, gotIDs)
		})
	}
}

func TestMatcher_AmountConditions(t *testing.T) {
	floatPtr := func(f float64) *float64 { return &f }

	tests := []struct {
		value     *float64
		min       *float64
		max       *float64
		name      string
		condition string
		amount    float64
		want      bool
	}{
		{
			name:      "any amount always matches",
			condition: "any",
			amount:    100.0,
			want:      true,
		},
		{
			name:      "less than - matches",
			condition: "lt",
			value:     floatPtr(50.0),
			amount:    40.0,
			want:      true,
		},
		{
			name:      "less than - no match",
			condition: "lt",
			value:     floatPtr(50.0),
			amount:    60.0,
			want:      false,
		},
		{
			name:      "less equal - exact match",
			condition: "le",
			value:     floatPtr(50.0),
			amount:    50.0,
			want:      true,
		},
		{
			name:      "equal - matches",
			condition: "eq",
			value:     floatPtr(50.0),
			amount:    50.0,
			want:      true,
		},
		{
			name:      "equal - no match",
			condition: "eq",
			value:     floatPtr(50.0),
			amount:    50.01,
			want:      false,
		},
		{
			name:      "greater equal - exact match",
			condition: "ge",
			value:     floatPtr(50.0),
			amount:    50.0,
			want:      true,
		},
		{
			name:      "greater than - matches",
			condition: "gt",
			value:     floatPtr(50.0),
			amount:    60.0,
			want:      true,
		},
		{
			name:      "range - within bounds",
			condition: "range",
			min:       floatPtr(10.0),
			max:       floatPtr(50.0),
			amount:    30.0,
			want:      true,
		},
		{
			name:      "range - below min",
			condition: "range",
			min:       floatPtr(10.0),
			max:       floatPtr(50.0),
			amount:    5.0,
			want:      false,
		},
		{
			name:      "range - above max",
			condition: "range",
			min:       floatPtr(10.0),
			max:       floatPtr(50.0),
			amount:    60.0,
			want:      false,
		},
		{
			name:      "range - only min",
			condition: "range",
			min:       floatPtr(10.0),
			amount:    20.0,
			want:      true,
		},
		{
			name:      "range - only max",
			condition: "range",
			max:       floatPtr(50.0),
			amount:    30.0,
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := Rule{
				ID:              1,
				AmountCondition: tt.condition,
				AmountValue:     tt.value,
				AmountMin:       tt.min,
				AmountMax:       tt.max,
				IsActive:        true,
			}
			matcher := &MatcherImpl{}
			txn := model.Transaction{Amount: tt.amount}

			got := matcher.matchesAmount(txn, rule)
			assert.Equal(t, tt.want, got)
		})
	}
}
