package ofx

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strings"

	"github.com/aclindsa/ofxgo"
	"github.com/joshsymonds/the-spice-must-flow/internal/model"
)

// Parser implements OFX/QFX file parsing.
type Parser struct{}

// NewParser creates a new OFX parser.
func NewParser() *Parser {
	return &Parser{}
}

// preprocessOFX fixes common formatting issues in OFX files.
func (p *Parser) preprocessOFX(content string) string {
	// Trim any leading whitespace or blank lines before the header
	content = strings.TrimLeft(content, " \t\r\n")

	// Fix mixed-case SEVERITY values (should be INFO, WARN, or ERROR)
	severityRegex := regexp.MustCompile(`(?i)<SEVERITY>(Info|Warn|Error)</SEVERITY>`)
	content = severityRegex.ReplaceAllStringFunc(content, func(match string) string {
		return strings.ToUpper(match)
	})

	// Fix missing closing angle brackets in SGML-style OFX files
	// Match opening tags that are missing their closing bracket
	// Pattern: <TAGNAME at end of line (no > and no content after tag)
	tagFixRegex := regexp.MustCompile(`(?m)^(\s*<[A-Z][A-Z0-9._]*[A-Z0-9])$`)
	content = tagFixRegex.ReplaceAllString(content, "$1>")

	return content
}

// ParseFile parses an OFX/QFX file and returns transactions.
func (p *Parser) ParseFile(ctx context.Context, reader io.Reader) ([]model.Transaction, error) {
	// Read and preprocess the content
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read OFX file: %w", err)
	}

	processedContent := p.preprocessOFX(string(content))

	// Parse OFX response
	resp, err := ofxgo.ParseResponse(strings.NewReader(processedContent))
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

// processBankStatement converts OFX bank transactions to our model.
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

// processCreditCardStatement converts OFX credit card transactions to our model.
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

// convertTransaction converts an OFX transaction to our model.
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
		Type:         fmt.Sprintf("%v", ofxTx.TrnType), // e.g., DEBIT, CHECK, PAYMENT, ATM
	}

	// Add check number if present
	if ofxTx.CheckNum != "" {
		tx.CheckNumber = string(ofxTx.CheckNum)
	}

	// OFX doesn't provide categories, but we could infer some based on transaction type
	// This is optional and can be expanded later
	switch tx.Type {
	case "INT":
		tx.Category = []string{"Income", "Interest"}
	case "FEE":
		tx.Category = []string{"Bank Fees"}
	case "ATM":
		tx.Category = []string{"Cash & ATM"}
	}

	// Generate hash for deduplication
	tx.Hash = tx.GenerateHash()

	return tx
}

// extractMerchantName tries to get a clean merchant name from OFX data.
func (p *Parser) extractMerchantName(tx ofxgo.Transaction) string {
	// Prefer PAYEE if available (cleaner merchant name)
	if tx.Payee != nil && tx.Payee.Name != "" {
		return string(tx.Payee.Name)
	}

	// Fall back to NAME field
	name := string(tx.Name)

	// Use MEMO field if NAME is generic
	if tx.Memo != "" && isGenericDescription(name) {
		// Sometimes MEMO has better merchant info
		name = string(tx.Memo)
	}

	// Basic cleanup
	name = strings.TrimSpace(name)

	// Remove common prefixes
	prefixes := []string{
		"POS PURCHASE ",
		"PURCHASE AUTHORIZED ON ",
		"DEBIT CARD PURCHASE ",
		"ACH DEBIT ",
		"CHECK CARD ",
		"VISA PURCHASE ",
		"MC PURCHASE ",
		"DEBIT PURCHASE ",
	}

	for _, prefix := range prefixes {
		if strings.HasPrefix(strings.ToUpper(name), prefix) {
			name = name[len(prefix):]
			break
		}
	}

	// Clean up date patterns like "MM/DD" at the beginning
	if len(name) > 5 && name[2] == '/' && name[5] == ' ' {
		name = strings.TrimSpace(name[6:])
	}

	return name
}

// isGenericDescription checks if a transaction name is too generic.
func isGenericDescription(name string) bool {
	generic := []string{
		"DEBIT",
		"CREDIT",
		"PURCHASE",
		"PAYMENT",
		"POS TRANSACTION",
		"CARD PURCHASE",
	}

	upperName := strings.ToUpper(name)
	for _, g := range generic {
		if upperName == g {
			return true
		}
	}
	return false
}

// GetAccounts extracts unique account IDs from the OFX file.
func (p *Parser) GetAccounts(ctx context.Context, reader io.Reader) ([]string, error) {
	// Read and preprocess the content
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read OFX file: %w", err)
	}

	processedContent := p.preprocessOFX(string(content))

	resp, err := ofxgo.ParseResponse(strings.NewReader(processedContent))
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
