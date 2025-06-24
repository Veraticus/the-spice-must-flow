package viewmodel

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStatsView_GetCompletionPercentage(t *testing.T) {
	tests := []struct {
		name  string
		stats StatsView
		want  float64
	}{
		{
			name: "half complete",
			stats: StatsView{
				TotalTransactions: 100,
				ClassifiedCount:   50,
			},
			want: 50.0,
		},
		{
			name: "fully complete",
			stats: StatsView{
				TotalTransactions: 100,
				ClassifiedCount:   100,
			},
			want: 100.0,
		},
		{
			name: "no transactions",
			stats: StatsView{
				TotalTransactions: 0,
				ClassifiedCount:   0,
			},
			want: 0.0,
		},
		{
			name: "over classified (shouldn't happen)",
			stats: StatsView{
				TotalTransactions: 100,
				ClassifiedCount:   110,
			},
			want: 110.0,
		},
		{
			name: "fractional percentage",
			stats: StatsView{
				TotalTransactions: 3,
				ClassifiedCount:   1,
			},
			want: 33.333333333333336,
		},
		{
			name: "zero classified",
			stats: StatsView{
				TotalTransactions: 100,
				ClassifiedCount:   0,
			},
			want: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.stats.GetCompletionPercentage()
			assert.InDelta(t, tt.want, got, 0.0001)
		})
	}
}

func TestStatsView_IsComplete(t *testing.T) {
	tests := []struct {
		name  string
		stats StatsView
		want  bool
	}{
		{
			name: "complete",
			stats: StatsView{
				TotalTransactions: 100,
				ClassifiedCount:   100,
			},
			want: true,
		},
		{
			name: "over complete",
			stats: StatsView{
				TotalTransactions: 100,
				ClassifiedCount:   105,
			},
			want: true,
		},
		{
			name: "not complete",
			stats: StatsView{
				TotalTransactions: 100,
				ClassifiedCount:   99,
			},
			want: false,
		},
		{
			name: "no transactions",
			stats: StatsView{
				TotalTransactions: 0,
				ClassifiedCount:   0,
			},
			want: false,
		},
		{
			name: "zero total but some classified",
			stats: StatsView{
				TotalTransactions: 0,
				ClassifiedCount:   10,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.stats.IsComplete()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestStatsView_HasCategories(t *testing.T) {
	tests := []struct {
		name  string
		stats StatsView
		want  bool
	}{
		{
			name: "has categories",
			stats: StatsView{
				CategoryStats: []CategoryStat{
					{CategoryName: "Groceries"},
					{CategoryName: "Transport"},
				},
			},
			want: true,
		},
		{
			name: "single category",
			stats: StatsView{
				CategoryStats: []CategoryStat{
					{CategoryName: "Groceries"},
				},
			},
			want: true,
		},
		{
			name: "no categories",
			stats: StatsView{
				CategoryStats: []CategoryStat{},
			},
			want: false,
		},
		{
			name: "nil categories",
			stats: StatsView{
				CategoryStats: nil,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.stats.HasCategories()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestStatsView_GetTimeSaved(t *testing.T) {
	tests := []struct {
		name  string
		stats StatsView
		want  time.Duration
	}{
		{
			name: "time saved with auto classifications",
			stats: StatsView{
				AutoClassifiedCount: 100,
				AverageTimePerTx:    5 * time.Second,
			},
			want: 500 * time.Second,
		},
		{
			name: "no auto classifications",
			stats: StatsView{
				AutoClassifiedCount: 0,
				AverageTimePerTx:    5 * time.Second,
			},
			want: 0,
		},
		{
			name: "no average time",
			stats: StatsView{
				AutoClassifiedCount: 100,
				AverageTimePerTx:    0,
			},
			want: 0,
		},
		{
			name: "single auto classification",
			stats: StatsView{
				AutoClassifiedCount: 1,
				AverageTimePerTx:    30 * time.Second,
			},
			want: 30 * time.Second,
		},
		{
			name: "fractional seconds",
			stats: StatsView{
				AutoClassifiedCount: 10,
				AverageTimePerTx:    1500 * time.Millisecond,
			},
			want: 15 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.stats.GetTimeSaved()
			assert.Equal(t, tt.want, got)
		})
	}
}
