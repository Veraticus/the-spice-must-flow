package viewmodel

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTransactionListView_IsEmpty(t *testing.T) {
	tests := []struct {
		name string
		view TransactionListView
		want bool
	}{
		{
			name: "has transactions",
			view: TransactionListView{
				Transactions: []TransactionItemView{
					{CategoryName: "Groceries"},
					{CategoryName: "Transport"},
				},
			},
			want: false,
		},
		{
			name: "single transaction",
			view: TransactionListView{
				Transactions: []TransactionItemView{
					{CategoryName: "Groceries"},
				},
			},
			want: false,
		},
		{
			name: "empty transactions",
			view: TransactionListView{
				Transactions: []TransactionItemView{},
			},
			want: true,
		},
		{
			name: "nil transactions",
			view: TransactionListView{
				Transactions: nil,
			},
			want: true,
		},
		{
			name: "with filter but no transactions",
			view: TransactionListView{
				Transactions: []TransactionItemView{},
				FilterMode:   FilterPending,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.view.IsEmpty()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTransactionListView_HasFilter(t *testing.T) {
	tests := []struct {
		name string
		view TransactionListView
		want bool
	}{
		{
			name: "no filter",
			view: TransactionListView{
				FilterMode:  FilterNone,
				SearchQuery: "",
			},
			want: false,
		},
		{
			name: "has filter mode",
			view: TransactionListView{
				FilterMode:  FilterPending,
				SearchQuery: "",
			},
			want: true,
		},
		{
			name: "has search query",
			view: TransactionListView{
				FilterMode:  FilterNone,
				SearchQuery: "groceries",
			},
			want: true,
		},
		{
			name: "has both filter and search",
			view: TransactionListView{
				FilterMode:  FilterByCategory,
				SearchQuery: "food",
			},
			want: true,
		},
		{
			name: "filter by category",
			view: TransactionListView{
				FilterMode:  FilterByCategory,
				SearchQuery: "",
			},
			want: true,
		},
		{
			name: "filter by merchant",
			view: TransactionListView{
				FilterMode:  FilterByMerchant,
				SearchQuery: "",
			},
			want: true,
		},
		{
			name: "whitespace search query",
			view: TransactionListView{
				FilterMode:  FilterNone,
				SearchQuery: " ",
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.view.HasFilter()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTransactionListView_GetSelectedCount(t *testing.T) {
	tests := []struct {
		name string
		view TransactionListView
		want int
	}{
		{
			name: "some selected",
			view: TransactionListView{
				Transactions: []TransactionItemView{
					{IsSelected: true},
					{IsSelected: false},
					{IsSelected: true},
					{IsSelected: false},
					{IsSelected: true},
				},
			},
			want: 3,
		},
		{
			name: "all selected",
			view: TransactionListView{
				Transactions: []TransactionItemView{
					{IsSelected: true},
					{IsSelected: true},
					{IsSelected: true},
				},
			},
			want: 3,
		},
		{
			name: "none selected",
			view: TransactionListView{
				Transactions: []TransactionItemView{
					{IsSelected: false},
					{IsSelected: false},
					{IsSelected: false},
				},
			},
			want: 0,
		},
		{
			name: "empty list",
			view: TransactionListView{
				Transactions: []TransactionItemView{},
			},
			want: 0,
		},
		{
			name: "nil list",
			view: TransactionListView{
				Transactions: nil,
			},
			want: 0,
		},
		{
			name: "single selected",
			view: TransactionListView{
				Transactions: []TransactionItemView{
					{IsSelected: false},
					{IsSelected: true},
					{IsSelected: false},
				},
			},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.view.GetSelectedCount()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTransactionItemView_IsClassified(t *testing.T) {
	tests := []struct {
		name string
		item TransactionItemView
		want bool
	}{
		{
			name: "classified",
			item: TransactionItemView{
				CategoryName: "Groceries",
				IsPending:    false,
			},
			want: true,
		},
		{
			name: "pending",
			item: TransactionItemView{
				CategoryName: "",
				IsPending:    true,
			},
			want: false,
		},
		{
			name: "has category but pending",
			item: TransactionItemView{
				CategoryName: "Groceries",
				IsPending:    true,
			},
			want: false,
		},
		{
			name: "not pending but no category",
			item: TransactionItemView{
				CategoryName: "",
				IsPending:    false,
			},
			want: false,
		},
		{
			name: "whitespace category",
			item: TransactionItemView{
				CategoryName: " ",
				IsPending:    false,
			},
			want: true,
		},
		{
			name: "classified with selection",
			item: TransactionItemView{
				CategoryName:  "Transport",
				IsPending:     false,
				IsSelected:    true,
				IsHighlighted: true,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.item.IsClassified()
			assert.Equal(t, tt.want, got)
		})
	}
}
