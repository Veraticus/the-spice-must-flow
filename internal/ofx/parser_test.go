package ofx

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/aclindsa/ofxgo"
	"github.com/joshsymonds/the-spice-must-flow/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Sample OFX data for testing.
const sampleBankOFX = `OFXHEADER:100
DATA:OFXSGML
VERSION:102
SECURITY:NONE
ENCODING:USASCII
CHARSET:1252
COMPRESSION:NONE
OLDFILEUID:NONE
NEWFILEUID:NONE

<OFX>
<SIGNONMSGSRSV1>
<SONRS>
<STATUS>
<CODE>0
<SEVERITY>INFO
</STATUS>
<DTSERVER>20240315120000[0:GMT]
<LANGUAGE>ENG
</SONRS>
</SIGNONMSGSRSV1>
<BANKMSGSRSV1>
<STMTTRNRS>
<TRNUID>1
<STATUS>
<CODE>0
<SEVERITY>INFO
</STATUS>
<STMTRS>
<CURDEF>USD
<BANKACCTFROM>
<BANKID>123456789
<ACCTID>1234567890
<ACCTTYPE>CHECKING
</BANKACCTFROM>
<BANKTRANLIST>
<DTSTART>20240101120000[0:GMT]
<DTEND>20240131120000[0:GMT]
<STMTTRN>
<TRNTYPE>DEBIT
<DTPOSTED>20240115120000[0:GMT]
<TRNAMT>-25.50
<FITID>2024011501
<NAME>STARBUCKS STORE #1234
</STMTTRN>
<STMTTRN>
<TRNTYPE>DEBIT
<DTPOSTED>20240120120000[0:GMT]
<TRNAMT>-125.00
<FITID>2024012001
<NAME>Whole Foods Market
</STMTTRN>
<STMTTRN>
<TRNTYPE>CHECK
<DTPOSTED>20240125120000[0:GMT]
<TRNAMT>-500.00
<FITID>2024012501
<CHECKNUM>1234
<NAME>CHECK #1234
</STMTTRN>
</BANKTRANLIST>
<LEDGERBAL>
<BALAMT>1000.00
<DTASOF>20240131120000[0:GMT]
</LEDGERBAL>
</STMTRS>
</STMTTRNRS>
</BANKMSGSRSV1>
</OFX>`

const sampleCreditCardOFX = `OFXHEADER:100
DATA:OFXSGML
VERSION:102
SECURITY:NONE
ENCODING:USASCII
CHARSET:1252
COMPRESSION:NONE
OLDFILEUID:NONE
NEWFILEUID:NONE

<OFX>
<SIGNONMSGSRSV1>
<SONRS>
<STATUS>
<CODE>0
<SEVERITY>INFO
</STATUS>
<DTSERVER>20240315120000[0:GMT]
<LANGUAGE>ENG
</SONRS>
</SIGNONMSGSRSV1>
<CREDITCARDMSGSRSV1>
<CCSTMTTRNRS>
<TRNUID>1
<STATUS>
<CODE>0
<SEVERITY>INFO
</STATUS>
<CCSTMTRS>
<CURDEF>USD
<CCACCTFROM>
<ACCTID>4111111111111111
</CCACCTFROM>
<BANKTRANLIST>
<DTSTART>20240101120000[0:GMT]
<DTEND>20240131120000[0:GMT]
<STMTTRN>
<TRNTYPE>DEBIT
<DTPOSTED>20240110120000[0:GMT]
<TRNAMT>-45.99
<FITID>CC2024011001
<NAME>AMAZON.COM*RT4Y7HG2
</STMTTRN>
<STMTTRN>
<TRNTYPE>DEBIT
<DTPOSTED>20240115120000[0:GMT]
<TRNAMT>-15.00
<FITID>CC2024011501
<NAME>NETFLIX.COM
</STMTTRN>
</BANKTRANLIST>
<LEDGERBAL>
<BALAMT>-500.00
<DTASOF>20240131120000[0:GMT]
</LEDGERBAL>
</CCSTMTRS>
</CCSTMTTRNRS>
</CREDITCARDMSGSRSV1>
</OFX>`

func TestParseFile(t *testing.T) {
	tests := []struct {
		name          string
		ofxData       string
		expectedCount int
		expectedError bool
	}{
		{
			name:          "valid bank statement",
			ofxData:       sampleBankOFX,
			expectedCount: 3,
			expectedError: false,
		},
		{
			name:          "valid credit card statement",
			ofxData:       sampleCreditCardOFX,
			expectedCount: 2,
			expectedError: false,
		},
		{
			name:          "invalid OFX data",
			ofxData:       "not valid OFX",
			expectedCount: 0,
			expectedError: true,
		},
		{
			name:          "empty OFX",
			ofxData:       "",
			expectedCount: 0,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()
			reader := strings.NewReader(tt.ofxData)

			transactions, err := parser.ParseFile(context.Background(), reader)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, transactions, tt.expectedCount)
			}
		})
	}
}

func TestParseBankTransactions(t *testing.T) {
	parser := NewParser()
	reader := strings.NewReader(sampleBankOFX)

	transactions, err := parser.ParseFile(context.Background(), reader)
	require.NoError(t, err)
	require.Len(t, transactions, 3)

	// Test first transaction (Starbucks)
	tx1 := transactions[0]
	assert.Equal(t, "2024011501", tx1.ID)
	assert.Equal(t, "STARBUCKS STORE #1234", tx1.Name)
	assert.Equal(t, "STARBUCKS STORE #1234", tx1.MerchantName) // No PAYEE, so uses NAME
	assert.Equal(t, 25.50, tx1.Amount)
	assert.Equal(t, "1234567890", tx1.AccountID)
	// Compare just the date components, ignoring timezone
	assert.Equal(t, 2024, tx1.Date.Year())
	assert.Equal(t, time.January, tx1.Date.Month())
	assert.Equal(t, 15, tx1.Date.Day())

	// Test second transaction (Whole Foods)
	tx2 := transactions[1]
	assert.Equal(t, "2024012001", tx2.ID)
	assert.Equal(t, "Whole Foods Market", tx2.Name)
	assert.Equal(t, "Whole Foods Market", tx2.MerchantName)
	assert.Equal(t, 125.00, tx2.Amount)

	// Test third transaction (Check)
	tx3 := transactions[2]
	assert.Equal(t, "2024012501", tx3.ID)
	assert.Equal(t, "CHECK #1234", tx3.Name)
	assert.Equal(t, 500.00, tx3.Amount)
}

func TestParseCreditCardTransactions(t *testing.T) {
	parser := NewParser()
	reader := strings.NewReader(sampleCreditCardOFX)

	transactions, err := parser.ParseFile(context.Background(), reader)
	require.NoError(t, err)
	require.Len(t, transactions, 2)

	// Test Amazon transaction
	tx1 := transactions[0]
	assert.Equal(t, "CC2024011001", tx1.ID)
	assert.Equal(t, "AMAZON.COM*RT4Y7HG2", tx1.Name)
	assert.Equal(t, 45.99, tx1.Amount)
	assert.Equal(t, "4111111111111111", tx1.AccountID)

	// Test Netflix transaction
	tx2 := transactions[1]
	assert.Equal(t, "CC2024011501", tx2.ID)
	assert.Equal(t, "NETFLIX.COM", tx2.Name)
	assert.Equal(t, 15.00, tx2.Amount)
}

func TestExtractMerchantName(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "remove POS prefix",
			input:    "POS PURCHASE STARBUCKS",
			expected: "STARBUCKS",
		},
		{
			name:     "remove DEBIT CARD prefix",
			input:    "DEBIT CARD PURCHASE WHOLE FOODS",
			expected: "WHOLE FOODS",
		},
		{
			name:     "keep clean name",
			input:    "NETFLIX.COM",
			expected: "NETFLIX.COM",
		},
		{
			name:     "trim whitespace",
			input:    "  AMAZON.COM  ",
			expected: "AMAZON.COM",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock transaction with the test input
			tx := ofxgo.Transaction{
				Name: ofxgo.String(tt.input),
			}
			result := parser.extractMerchantName(tx)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTransactionDeduplication(t *testing.T) {
	// Create two identical transactions
	tx1 := model.Transaction{
		ID:           "TX001",
		Date:         time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		Name:         "STARBUCKS",
		MerchantName: "Starbucks",
		Amount:       25.50,
		AccountID:    "123456",
	}
	tx1.Hash = tx1.GenerateHash()

	tx2 := model.Transaction{
		ID:           "TX002", // Different ID
		Date:         time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		Name:         "STARBUCKS",
		MerchantName: "Starbucks",
		Amount:       25.50,
		AccountID:    "123456",
	}
	tx2.Hash = tx2.GenerateHash()

	// Hashes should be identical for deduplication
	assert.Equal(t, tx1.Hash, tx2.Hash)

	// Different amount should produce different hash
	tx3 := tx1
	tx3.Amount = 30.00
	tx3.Hash = tx3.GenerateHash()
	assert.NotEqual(t, tx1.Hash, tx3.Hash)

	// Different date should produce different hash
	tx4 := tx1
	tx4.Date = time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC)
	tx4.Hash = tx4.GenerateHash()
	assert.NotEqual(t, tx1.Hash, tx4.Hash)
}

func TestGetAccounts(t *testing.T) {
	parser := NewParser()

	// Test with bank statement
	reader := strings.NewReader(sampleBankOFX)
	accounts, err := parser.GetAccounts(context.Background(), reader)
	require.NoError(t, err)
	assert.Contains(t, accounts, "1234567890")

	// Test with credit card statement
	reader = strings.NewReader(sampleCreditCardOFX)
	accounts, err = parser.GetAccounts(context.Background(), reader)
	require.NoError(t, err)
	assert.Contains(t, accounts, "4111111111111111")
}
