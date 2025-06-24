package viewmodel

// BatchMode represents the mode of the batch classifier.
type BatchMode int

const (
	// BatchModeSelecting indicates the user is selecting transactions to classify.
	BatchModeSelecting BatchMode = iota
	// BatchModeClassifying indicates the classification process is running.
	BatchModeClassifying
	// BatchModeCompleted indicates the batch classification has finished.
	BatchModeCompleted
)

// BatchView represents batch classification display data.
type BatchView struct {
	Groups       []TransactionGroup
	Error        string
	Progress     ProgressView
	Mode         BatchMode
	CurrentGroup int
}

// TransactionGroup represents a group of similar transactions.
type TransactionGroup struct {
	Transactions      []TransactionView
	MerchantName      string
	CategoryName      string
	SuggestedCategory CategoryView
	IsSelected        bool
	IsClassified      bool
}

// ProgressView represents progress information.
type ProgressView struct {
	Status  string
	Current int
	Total   int
}

// IsComplete returns true if progress has reached 100%.
func (p ProgressView) IsComplete() bool {
	return p.Total > 0 && p.Current >= p.Total
}

// HasTransactions returns true if the group contains transactions.
func (tg TransactionGroup) HasTransactions() bool {
	return len(tg.Transactions) > 0
}

// IsEmpty returns true if there are no groups to process.
func (bv BatchView) IsEmpty() bool {
	return len(bv.Groups) == 0
}
