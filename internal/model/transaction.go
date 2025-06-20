package model

import (
	"crypto/sha256"
	"fmt"
	"time"
)

// Transaction represents a single financial transaction from any source.
type Transaction struct {
	Date          time.Time
	ID            string
	Name          string    // Raw transaction description
	MerchantName  string    // Cleaned merchant name
	AccountID     string
	Hash          string
	Amount        float64
	
	// Optional metadata that may be available depending on source
	Category      []string  // Category hints from source (e.g., Plaid categories)
	Type          string    // Transaction type (e.g., DEBIT, CHECK, PAYMENT, ATM)
	CheckNumber   string    // Check number if applicable
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
