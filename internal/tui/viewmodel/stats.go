package viewmodel

import "time"

// StatsView represents the statistics display data.
type StatsView struct {
	StartTime             time.Time
	CategoryStats         []CategoryStat
	TotalTransactions     int
	ClassifiedCount       int
	PendingCount          int
	AutoClassifiedCount   int
	ManualClassifiedCount int
	BatchesProcessed      int
	ElapsedTime           time.Duration
	EstimatedTime         time.Duration
	AverageTimePerTx      time.Duration
}

// CategoryStat represents statistics for a single category.
type CategoryStat struct {
	CategoryName     string
	CategoryIcon     string
	TopMerchants     []MerchantStat
	TotalAmount      float64
	Percentage       float64
	TransactionCount int
	IsExpanded       bool
}

// MerchantStat represents statistics for a merchant.
type MerchantStat struct {
	MerchantName     string
	TransactionCount int
	TotalAmount      float64
}

// GetCompletionPercentage returns the percentage of transactions classified.
func (sv StatsView) GetCompletionPercentage() float64 {
	if sv.TotalTransactions == 0 {
		return 0
	}
	return float64(sv.ClassifiedCount) / float64(sv.TotalTransactions) * 100
}

// IsComplete returns true if all transactions have been classified.
func (sv StatsView) IsComplete() bool {
	return sv.TotalTransactions > 0 && sv.ClassifiedCount >= sv.TotalTransactions
}

// HasCategories returns true if there are category statistics to display.
func (sv StatsView) HasCategories() bool {
	return len(sv.CategoryStats) > 0
}

// GetTimeSaved calculates time saved through auto-classification.
func (sv StatsView) GetTimeSaved() time.Duration {
	if sv.AverageTimePerTx == 0 {
		return 0
	}
	return time.Duration(sv.AutoClassifiedCount) * sv.AverageTimePerTx
}
