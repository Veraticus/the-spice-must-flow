package viewmodel

// TransactionListView represents the transaction list display data.
type TransactionListView struct {
	SearchQuery  string
	Transactions []TransactionItemView
	Cursor       int
	TotalCount   int
	FilterMode   FilterMode
	SortBy       SortField
	SortOrder    SortOrder
}

// TransactionItemView represents a single transaction in the list.
type TransactionItemView struct {
	CategoryName  string
	Transaction   TransactionView
	IsSelected    bool
	IsHighlighted bool
	IsPending     bool
}

// FilterMode represents different filtering options.
type FilterMode int

const (
	// FilterNone indicates no filter is applied.
	FilterNone FilterMode = iota
	// FilterPending shows only unclassified transactions.
	FilterPending
	// FilterClassified shows only classified transactions.
	FilterClassified
	// FilterByCategory filters by a specific category.
	FilterByCategory
	// FilterByMerchant filters by a specific merchant.
	FilterByMerchant
)

// SortField represents the field to sort by.
type SortField int

const (
	// SortByDate sorts transactions by date.
	SortByDate SortField = iota
	// SortByAmount sorts transactions by amount.
	SortByAmount
	// SortByMerchant sorts transactions by merchant name.
	SortByMerchant
	// SortByCategory sorts transactions by category.
	SortByCategory
)

// SortOrder represents sort direction.
type SortOrder int

const (
	// SortAscending sorts in ascending order.
	SortAscending SortOrder = iota
	// SortDescending sorts in descending order.
	SortDescending
)

// IsEmpty returns true if there are no transactions in the list.
func (tlv TransactionListView) IsEmpty() bool {
	return len(tlv.Transactions) == 0
}

// HasFilter returns true if any filter is active.
func (tlv TransactionListView) HasFilter() bool {
	return tlv.FilterMode != FilterNone || tlv.SearchQuery != ""
}

// GetSelectedCount returns the number of selected transactions.
func (tlv TransactionListView) GetSelectedCount() int {
	count := 0
	for _, t := range tlv.Transactions {
		if t.IsSelected {
			count++
		}
	}
	return count
}

// IsClassified returns true if the transaction has been classified.
func (tiv TransactionItemView) IsClassified() bool {
	return !tiv.IsPending && tiv.CategoryName != ""
}
