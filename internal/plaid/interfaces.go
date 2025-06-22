package plaid

import (
	"context"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
)

// TransactionFetcher defines the contract for fetching transaction data.
// This interface allows for easy mocking in tests and swapping data sources.
type TransactionFetcher interface {
	GetTransactions(ctx context.Context, startDate, endDate time.Time) ([]model.Transaction, error)
	GetAccounts(ctx context.Context) ([]string, error)
}
