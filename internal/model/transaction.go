package model

import (
	"crypto/sha256"
	"fmt"
	"time"
)

// TransactionDirection indicates whether a transaction is income, expense, or transfer.
type TransactionDirection string

const (
	// DirectionIncome represents money coming into an account.
	DirectionIncome TransactionDirection = "income"
	// DirectionExpense represents money leaving an account.
	DirectionExpense TransactionDirection = "expense"
	// DirectionTransfer represents money moving between accounts.
	DirectionTransfer TransactionDirection = "transfer"
)

// Transaction represents a single financial transaction from any source.
type Transaction struct {
	Date           time.Time
	Type           string
	Name           string
	MerchantName   string
	AccountID      string
	Hash           string
	ID             string
	CheckNumber    string
	Direction      TransactionDirection
	RefundCategory string
	Category       []string
	Amount         float64
	IsRefund       bool
}

// GenerateHash creates a unique hash for duplicate detection.
func (t *Transaction) GenerateHash() string {
	data := fmt.Sprintf("%s:%.2f:%s:%s",
		t.Date.Format("2006-01-02"),
		t.Amount,
		t.MerchantName,
		t.AccountID)
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash)
}
