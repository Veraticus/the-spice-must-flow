package model

import (
	"crypto/sha256"
	"fmt"
	"time"
)

// Transaction represents a single financial transaction from Plaid.
type Transaction struct {
	Date          time.Time
	ID            string
	Name          string
	MerchantName  string
	AccountID     string
	Hash          string
	PlaidCategory string
	Amount        float64
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
