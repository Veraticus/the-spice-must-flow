package viewmodel

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProgressView_IsComplete(t *testing.T) {
	tests := []struct {
		name     string
		progress ProgressView
		want     bool
	}{
		{
			name: "complete",
			progress: ProgressView{
				Current: 100,
				Total:   100,
			},
			want: true,
		},
		{
			name: "over complete",
			progress: ProgressView{
				Current: 110,
				Total:   100,
			},
			want: true,
		},
		{
			name: "not complete",
			progress: ProgressView{
				Current: 50,
				Total:   100,
			},
			want: false,
		},
		{
			name: "zero total",
			progress: ProgressView{
				Current: 50,
				Total:   0,
			},
			want: false,
		},
		{
			name: "zero current",
			progress: ProgressView{
				Current: 0,
				Total:   100,
			},
			want: false,
		},
		{
			name: "all zero",
			progress: ProgressView{
				Current: 0,
				Total:   0,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.progress.IsComplete()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTransactionGroup_HasTransactions(t *testing.T) {
	tests := []struct {
		name  string
		group TransactionGroup
		want  bool
	}{
		{
			name: "has transactions",
			group: TransactionGroup{
				MerchantName: "Test Merchant",
				Transactions: []TransactionView{
					{ID: "1", MerchantName: "Test"},
					{ID: "2", MerchantName: "Test"},
				},
			},
			want: true,
		},
		{
			name: "single transaction",
			group: TransactionGroup{
				MerchantName: "Test Merchant",
				Transactions: []TransactionView{
					{ID: "1", MerchantName: "Test"},
				},
			},
			want: true,
		},
		{
			name: "no transactions",
			group: TransactionGroup{
				MerchantName: "Test Merchant",
				Transactions: []TransactionView{},
			},
			want: false,
		},
		{
			name: "nil transactions",
			group: TransactionGroup{
				MerchantName: "Test Merchant",
				Transactions: nil,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.group.HasTransactions()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBatchView_IsEmpty(t *testing.T) {
	tests := []struct {
		name  string
		batch BatchView
		want  bool
	}{
		{
			name: "has groups",
			batch: BatchView{
				Groups: []TransactionGroup{
					{MerchantName: "Merchant 1"},
					{MerchantName: "Merchant 2"},
				},
			},
			want: false,
		},
		{
			name: "single group",
			batch: BatchView{
				Groups: []TransactionGroup{
					{MerchantName: "Merchant 1"},
				},
			},
			want: false,
		},
		{
			name: "no groups",
			batch: BatchView{
				Groups: []TransactionGroup{},
			},
			want: true,
		},
		{
			name: "nil groups",
			batch: BatchView{
				Groups: nil,
			},
			want: true,
		},
		{
			name: "with error but no groups",
			batch: BatchView{
				Groups: []TransactionGroup{},
				Error:  "some error",
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.batch.IsEmpty()
			assert.Equal(t, tt.want, got)
		})
	}
}
