package ofx

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/aclindsa/ofxgo"
	"github.com/joshsymonds/the-spice-must-flow/internal/model"
)

// Parser implements OFX/QFX file parsing
type Parser struct{}

// NewParser creates a new OFX parser
func NewParser() *Parser {
	return &Parser{}
}

// ParseFile parses an OFX/QFX file and returns transactions
func (p *Parser) ParseFile(ctx context.Context, reader io.Reader) ([]model.Transaction, error) {
	// Parse OFX response
	resp, err := ofxgo.ParseResponse(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OFX file: %w", err)
	}

	var transactions []model.Transaction
	var bankStmts, ccStmts int

	// Process bank messages
	for _, msg := range resp.Bank {
		if stmt, ok := msg.(*ofxgo.StatementResponse); ok {
			bankStmts++
			txns, err := p.processBankStatement(stmt)
			if err != nil {
				slog.Warn("Failed to process bank statement",
					"account", stmt.BankAcctFrom.AcctID,
					"error", err)
				continue
			}
			transactions = append(transactions, txns...)
		}
	}

	// Process credit card messages
	for _, msg := range resp.CreditCard {
		if stmt, ok := msg.(*ofxgo.CCStatementResponse); ok {
			ccStmts++
			txns, err := p.processCreditCardStatement(stmt)
			if err != nil {
				slog.Warn("Failed to process credit card statement",
					"account", stmt.CCAcctFrom.AcctID,
					"error", err)
				continue
			}
			transactions = append(transactions, txns...)
		}
	}
	
	slog.Info("Parsed OFX file",
		"total_transactions", len(transactions),
		"bank_statements", bankStmts,
		"cc_statements", ccStmts)

	return transactions, nil
}

// processBankStatement converts OFX bank transactions to our model
func (p *Parser) processBankStatement(stmt *ofxgo.StatementResponse) ([]model.Transaction, error) {
	if stmt.BankTranList == nil {
		return nil, nil
	}

	var transactions []model.Transaction
	accountID := string(stmt.BankAcctFrom.AcctID)

	for _, ofxTx := range stmt.BankTranList.Transactions {
		tx := p.convertTransaction(ofxTx, accountID)
		transactions = append(transactions, tx)
	}

	return transactions, nil
}

// processCreditCardStatement converts OFX credit card transactions to our model
func (p *Parser) processCreditCardStatement(stmt *ofxgo.CCStatementResponse) ([]model.Transaction, error) {
	if stmt.BankTranList == nil {
		return nil, nil
	}

	var transactions []model.Transaction
	accountID := string(stmt.CCAcctFrom.AcctID)

	for _, ofxTx := range stmt.BankTranList.Transactions {
		tx := p.convertTransaction(ofxTx, accountID)
		transactions = append(transactions, tx)
	}

	return transactions, nil
}

// convertTransaction converts an OFX transaction to our model
func (p *Parser) convertTransaction(ofxTx ofxgo.Transaction, accountID string) model.Transaction {
	// Extract clean merchant name
	merchantName := p.extractMerchantName(ofxTx)

	// Convert amount (OFX uses negative for debits)
	// ofxTx.TrnAmt is a big.Rat, convert to float64
	amountFloat, _ := ofxTx.TrnAmt.Float64()
	amount := amountFloat
	if amount < 0 {
		amount = -amount
	}

	// Create transaction
	tx := model.Transaction{
		ID:           string(ofxTx.FiTID),
		Date:         ofxTx.DtPosted.Time,
		Name:         string(ofxTx.Name),
		MerchantName: merchantName,
		Amount:       amount,
		AccountID:    accountID,
	}

	// Generate hash for deduplication
	tx.Hash = tx.GenerateHash()

	return tx
}

// extractMerchantName tries to get a clean merchant name from OFX data
func (p *Parser) extractMerchantName(tx ofxgo.Transaction) string {
	// Prefer PAYEE if available (cleaner merchant name)
	if tx.Payee != nil && tx.Payee.Name != "" {
		return string(tx.Payee.Name)
	}

	// Fall back to NAME field
	name := string(tx.Name)

	// Basic cleanup
	name = strings.TrimSpace(name)

	// Remove common prefixes
	prefixes := []string{
		"POS PURCHASE ",
		"PURCHASE AUTHORIZED ON ",
		"DEBIT CARD PURCHASE ",
		"ACH DEBIT ",
		"CHECK CARD ",
	}

	for _, prefix := range prefixes {
		if strings.HasPrefix(name, prefix) {
			name = strings.TrimPrefix(name, prefix)
			break
		}
	}

	return name
}

// GetAccounts extracts unique account IDs from the OFX file
func (p *Parser) GetAccounts(ctx context.Context, reader io.Reader) ([]string, error) {
	resp, err := ofxgo.ParseResponse(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OFX file: %w", err)
	}

	accountMap := make(map[string]bool)

	// Bank accounts
	for _, msg := range resp.Bank {
		if stmt, ok := msg.(*ofxgo.StatementResponse); ok {
			if stmt.BankAcctFrom.AcctID != "" {
				accountMap[string(stmt.BankAcctFrom.AcctID)] = true
			}
		}
	}

	// Credit card accounts
	for _, msg := range resp.CreditCard {
		if stmt, ok := msg.(*ofxgo.CCStatementResponse); ok {
			if stmt.CCAcctFrom.AcctID != "" {
				accountMap[string(stmt.CCAcctFrom.AcctID)] = true
			}
		}
	}

	// Convert to slice
	var accounts []string
	for acct := range accountMap {
		accounts = append(accounts, acct)
	}

	return accounts, nil
}