package pattern

import (
	"context"
	"testing"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestSuggester_Suggest(t *testing.T) {
	ctx := context.Background()

	floatPtr := func(f float64) *float64 { return &f }
	dirPtr := func(d model.TransactionDirection) *model.TransactionDirection { return &d }

	tests := []struct {
		name  string
		rules []Rule
		want  []Suggestion
		txn   model.Transaction
	}{
		{
			name: "single pattern match",
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
				Amount:       50.0,
				Direction:    model.DirectionExpense,
			},
			want: []Suggestion{
				{
					Category:   "Shopping",
					Confidence: 0.9,
					Reason:     "Transactions from Amazon are usually categorized as Shopping",
					RuleID:     intPtr(1),
				},
			},
		},
		{
			name: "pattern with amount condition",
			rules: []Rule{
				{
					ID:              2,
					MerchantPattern: "Coffee Shop",
					AmountCondition: "lt",
					AmountValue:     floatPtr(10.0),
					DefaultCategory: "Dining",
					Confidence:      0.85,
					IsActive:        true,
				},
			},
			txn: model.Transaction{
				MerchantName: "Coffee Shop",
				Amount:       5.0,
				Direction:    model.DirectionExpense,
			},
			want: []Suggestion{
				{
					Category:   "Dining",
					Confidence: 0.85,
					Reason:     "Transactions from Coffee Shop under $10.00 are usually categorized as Dining",
					RuleID:     intPtr(2),
				},
			},
		},
		{
			name: "pattern with range condition",
			rules: []Rule{
				{
					ID:              3,
					MerchantPattern: "Restaurant",
					AmountCondition: "range",
					AmountMin:       floatPtr(20.0),
					AmountMax:       floatPtr(50.0),
					DefaultCategory: "Dining",
					Confidence:      0.9,
					IsActive:        true,
				},
			},
			txn: model.Transaction{
				MerchantName: "Restaurant",
				Amount:       25.0,
				Direction:    model.DirectionExpense,
			},
			want: []Suggestion{
				{
					Category:   "Dining",
					Confidence: 0.9,
					Reason:     "Transactions from Restaurant between $20.00 and $50.00 are usually categorized as Dining",
					RuleID:     intPtr(3),
				},
			},
		},
		{
			name: "pattern with direction",
			rules: []Rule{
				{
					ID:              4,
					MerchantPattern: "Amazon",
					AmountCondition: "any",
					Direction:       dirPtr(model.DirectionIncome),
					DefaultCategory: "Refund",
					Confidence:      0.95,
					IsActive:        true,
				},
			},
			txn: model.Transaction{
				MerchantName: "Amazon",
				Amount:       30.0,
				Direction:    model.DirectionIncome,
			},
			want: []Suggestion{
				{
					Category:   "Refund",
					Confidence: 0.95,
					Reason:     "Transactions from Amazon (income) are usually categorized as Refund",
					RuleID:     intPtr(4),
				},
			},
		},
		{
			name: "multiple patterns - deduplicated",
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
				{
					ID:              2,
					MerchantPattern: "Amazon",
					AmountCondition: "any",
					DefaultCategory: "Shopping", // Same category
					Confidence:      0.85,
					Priority:        5,
					IsActive:        true,
				},
			},
			txn: model.Transaction{
				MerchantName: "Amazon",
				Amount:       50.0,
				Direction:    model.DirectionExpense,
			},
			want: []Suggestion{
				{
					Category:   "Shopping",
					Confidence: 0.9, // From higher priority rule
					Reason:     "Transactions from Amazon are usually categorized as Shopping",
					RuleID:     intPtr(1),
				},
			},
		},
		{
			name: "no matching patterns",
			rules: []Rule{
				{
					ID:              1,
					MerchantPattern: "Amazon",
					AmountCondition: "any",
					DefaultCategory: "Shopping",
					IsActive:        true,
				},
			},
			txn: model.Transaction{
				MerchantName: "Unknown Store",
				Amount:       50.0,
			},
			want: []Suggestion{},
		},
		{
			name: "use transaction name when merchant empty",
			rules: []Rule{
				{
					ID:              1,
					MerchantPattern: "Store Purchase",
					AmountCondition: "any",
					DefaultCategory: "Shopping",
					Confidence:      0.8,
					IsActive:        true,
				},
			},
			txn: model.Transaction{
				Name:   "Store Purchase",
				Amount: 50.0,
			},
			want: []Suggestion{
				{
					Category:   "Shopping",
					Confidence: 0.8,
					Reason:     "Transactions from Store Purchase are usually categorized as Shopping",
					RuleID:     intPtr(1),
				},
			},
		},
		{
			name: "amount greater than condition",
			rules: []Rule{
				{
					ID:              5,
					MerchantPattern: "", // Matches all merchants
					AmountCondition: "gt",
					AmountValue:     floatPtr(1000.0),
					DefaultCategory: "Large Purchase",
					Confidence:      0.7,
					IsActive:        true,
				},
			},
			txn: model.Transaction{
				MerchantName: "Electronics Store",
				Amount:       1500.0,
				Direction:    model.DirectionExpense,
			},
			want: []Suggestion{
				{
					Category:   "Large Purchase",
					Confidence: 0.7,
					Reason:     "Transactions from Electronics Store over $1000.00 are usually categorized as Large Purchase",
					RuleID:     intPtr(5),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher := NewMatcher(tt.rules)
			validator := NewValidator()
			suggester := NewSuggester(matcher, validator)

			got, err := suggester.Suggest(ctx, tt.txn)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSuggester_SuggestWithValidation(t *testing.T) {
	ctx := context.Background()

	categories := []model.Category{
		{Name: "Shopping", Type: model.CategoryTypeExpense},
		{Name: "Salary", Type: model.CategoryTypeIncome},
		{Name: "Refund", Type: model.CategoryTypeIncome},
		{Name: "Transfer", Type: model.CategoryTypeSystem},
		{Name: "Large Purchase", Type: model.CategoryTypeExpense},
	}

	tests := []struct {
		name  string
		rules []Rule
		want  []Suggestion
		txn   model.Transaction
	}{
		{
			name: "all suggestions valid",
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
				{
					ID:              2,
					MerchantPattern: "Amazon",
					AmountCondition: "any",
					DefaultCategory: "Transfer",
					Confidence:      0.5,
					Priority:        5,
					IsActive:        true,
				},
			},
			txn: model.Transaction{
				MerchantName: "Amazon",
				Amount:       50.0,
				Direction:    model.DirectionExpense,
			},
			want: []Suggestion{
				{
					Category:   "Shopping",
					Confidence: 0.9,
					Reason:     "Transactions from Amazon are usually categorized as Shopping",
					RuleID:     intPtr(1),
				},
				{
					Category:   "Transfer",
					Confidence: 0.5,
					Reason:     "Transactions from Amazon are usually categorized as Transfer",
					RuleID:     intPtr(2),
				},
			},
		},
		{
			name: "filter invalid suggestions",
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
				{
					ID:              2,
					MerchantPattern: "Amazon",
					AmountCondition: "any",
					DefaultCategory: "Salary", // Invalid for expense
					Confidence:      0.8,
					Priority:        5,
					IsActive:        true,
				},
			},
			txn: model.Transaction{
				MerchantName: "Amazon",
				Amount:       50.0,
				Direction:    model.DirectionExpense,
			},
			want: []Suggestion{
				{
					Category:   "Shopping",
					Confidence: 0.9,
					Reason:     "Transactions from Amazon are usually categorized as Shopping",
					RuleID:     intPtr(1),
				},
			},
		},
		{
			name: "skip unknown categories",
			rules: []Rule{
				{
					ID:              1,
					MerchantPattern: "Store",
					AmountCondition: "any",
					DefaultCategory: "UnknownCategory",
					Confidence:      0.9,
					Priority:        10,
					IsActive:        true,
				},
				{
					ID:              2,
					MerchantPattern: "Store",
					AmountCondition: "any",
					DefaultCategory: "Shopping",
					Confidence:      0.8,
					Priority:        5,
					IsActive:        true,
				},
			},
			txn: model.Transaction{
				MerchantName: "Store",
				Amount:       50.0,
				Direction:    model.DirectionExpense,
			},
			want: []Suggestion{
				{
					Category:   "Shopping",
					Confidence: 0.8,
					Reason:     "Transactions from Store are usually categorized as Shopping",
					RuleID:     intPtr(2),
				},
			},
		},
		{
			name: "income transaction with income categories",
			rules: []Rule{
				{
					ID:              1,
					MerchantPattern: "Company",
					AmountCondition: "any",
					DefaultCategory: "Salary",
					Confidence:      0.95,
					IsActive:        true,
				},
				{
					ID:              2,
					MerchantPattern: "Company",
					AmountCondition: "any",
					DefaultCategory: "Shopping", // Invalid for income
					Confidence:      0.8,
					IsActive:        true,
				},
			},
			txn: model.Transaction{
				MerchantName: "Company",
				Amount:       3000.0,
				Direction:    model.DirectionIncome,
			},
			want: []Suggestion{
				{
					Category:   "Salary",
					Confidence: 0.95,
					Reason:     "Transactions from Company are usually categorized as Salary",
					RuleID:     intPtr(1),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher := NewMatcher(tt.rules)
			validator := NewValidator()
			suggester := NewSuggester(matcher, validator)

			got, err := suggester.SuggestWithValidation(ctx, tt.txn, categories)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSuggester_GenerateReason(t *testing.T) {
	suggester := &Suggester{}

	floatPtr := func(f float64) *float64 { return &f }
	dirPtr := func(d model.TransactionDirection) *model.TransactionDirection { return &d }

	tests := []struct {
		name string
		want string
		txn  model.Transaction
		rule Rule
	}{
		{
			name: "basic reason",
			txn: model.Transaction{
				MerchantName: "Amazon",
			},
			rule: Rule{
				DefaultCategory: "Shopping",
			},
			want: "Transactions from Amazon are usually categorized as Shopping",
		},
		{
			name: "with amount less than",
			txn: model.Transaction{
				MerchantName: "Coffee Shop",
			},
			rule: Rule{
				AmountCondition: "lt",
				AmountValue:     floatPtr(10.0),
				DefaultCategory: "Dining",
			},
			want: "Transactions from Coffee Shop under $10.00 are usually categorized as Dining",
		},
		{
			name: "with amount greater than",
			txn: model.Transaction{
				MerchantName: "Electronics",
			},
			rule: Rule{
				AmountCondition: "gt",
				AmountValue:     floatPtr(500.0),
				DefaultCategory: "Major Purchase",
			},
			want: "Transactions from Electronics over $500.00 are usually categorized as Major Purchase",
		},
		{
			name: "with amount range",
			txn: model.Transaction{
				MerchantName: "Restaurant",
			},
			rule: Rule{
				AmountCondition: "range",
				AmountMin:       floatPtr(20.0),
				AmountMax:       floatPtr(100.0),
				DefaultCategory: "Dining",
			},
			want: "Transactions from Restaurant between $20.00 and $100.00 are usually categorized as Dining",
		},
		{
			name: "with income direction",
			txn: model.Transaction{
				MerchantName: "Amazon",
			},
			rule: Rule{
				Direction:       dirPtr(model.DirectionIncome),
				DefaultCategory: "Refund",
			},
			want: "Transactions from Amazon (income) are usually categorized as Refund",
		},
		{
			name: "with expense direction",
			txn: model.Transaction{
				MerchantName: "Grocery Store",
			},
			rule: Rule{
				Direction:       dirPtr(model.DirectionExpense),
				DefaultCategory: "Groceries",
			},
			want: "Transactions from Grocery Store (expense) are usually categorized as Groceries",
		},
		{
			name: "use transaction name when merchant empty",
			txn: model.Transaction{
				Name: "Store Purchase",
			},
			rule: Rule{
				DefaultCategory: "Shopping",
			},
			want: "Transactions from Store Purchase are usually categorized as Shopping",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := suggester.generateReason(tt.txn, tt.rule)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Helper function.
func intPtr(i int) *int {
	return &i
}
