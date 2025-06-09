// Package plaid provides a client for interacting with the Plaid API.
package plaid

import (
	"context"
	"time"

	"github.com/joshsymonds/the-spice-must-flow/internal/model"
)

// MockClient is a mock implementation of PlaidClient for testing.
type MockClient struct {
	// Functions that can be set by tests to control behavior
	GetTransactionsFn func(ctx context.Context, startDate, endDate time.Time) ([]model.Transaction, error)
	GetAccountsFn     func(ctx context.Context) ([]string, error)

	// Call tracking
	GetTransactionsCalls []GetTransactionsCall
	GetAccountsCalls     int
}

// GetTransactionsCall records the parameters of a GetTransactions call.
type GetTransactionsCall struct {
	StartDate time.Time
	EndDate   time.Time
}

// NewMockClient creates a new mock Plaid client.
func NewMockClient() *MockClient {
	return &MockClient{
		GetTransactionsCalls: []GetTransactionsCall{},
	}
}

// GetTransactions implements PlaidClient.GetTransactions.
func (m *MockClient) GetTransactions(ctx context.Context, startDate, endDate time.Time) ([]model.Transaction, error) {
	m.GetTransactionsCalls = append(m.GetTransactionsCalls, GetTransactionsCall{
		StartDate: startDate,
		EndDate:   endDate,
	})

	if m.GetTransactionsFn != nil {
		return m.GetTransactionsFn(ctx, startDate, endDate)
	}

	// Default behavior: return empty slice
	return []model.Transaction{}, nil
}

// GetAccounts implements PlaidClient.GetAccounts.
func (m *MockClient) GetAccounts(ctx context.Context) ([]string, error) {
	m.GetAccountsCalls++

	if m.GetAccountsFn != nil {
		return m.GetAccountsFn(ctx)
	}

	// Default behavior: return empty slice
	return []string{}, nil
}

// Reset clears all call tracking.
func (m *MockClient) Reset() {
	m.GetTransactionsCalls = []GetTransactionsCall{}
	m.GetAccountsCalls = 0
}

// Ensure MockClient implements PlaidClient interface.
var _ TransactionFetcher = (*MockClient)(nil)
