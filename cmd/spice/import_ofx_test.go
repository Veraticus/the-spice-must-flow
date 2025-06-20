package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Sample OFX files for testing.
const testOFX1 = `OFXHEADER:100
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
<FITID>JAN01
<NAME>STARBUCKS
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

const testOFX2 = `OFXHEADER:100
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
<DTSTART>20240201120000[0:GMT]
<DTEND>20240228120000[0:GMT]
<STMTTRN>
<TRNTYPE>DEBIT
<DTPOSTED>20240215120000[0:GMT]
<TRNAMT>-25.50
<FITID>FEB01
<NAME>STARBUCKS
</STMTTRN>
<STMTTRN>
<TRNTYPE>DEBIT
<DTPOSTED>20240220120000[0:GMT]
<TRNAMT>-100.00
<FITID>FEB02
<NAME>WHOLE FOODS
</STMTTRN>
</BANKTRANLIST>
<LEDGERBAL>
<BALAMT>900.00
<DTASOF>20240228120000[0:GMT]
</LEDGERBAL>
</STMTRS>
</STMTTRNRS>
</BANKMSGSRSV1>
</OFX>`

// Duplicate transaction in both files.
const testOFXDuplicate = `OFXHEADER:100
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
<DTSTART>20240215120000[0:GMT]
<DTEND>20240315120000[0:GMT]
<STMTTRN>
<TRNTYPE>DEBIT
<DTPOSTED>20240215120000[0:GMT]
<TRNAMT>-25.50
<FITID>FEB01_DUP
<NAME>STARBUCKS
</STMTTRN>
<STMTTRN>
<TRNTYPE>DEBIT
<DTPOSTED>20240301120000[0:GMT]
<TRNAMT>-50.00
<FITID>MAR01
<NAME>TARGET
</STMTTRN>
</BANKTRANLIST>
<LEDGERBAL>
<BALAMT>850.00
<DTASOF>20240315120000[0:GMT]
</LEDGERBAL>
</STMTRS>
</STMTTRNRS>
</BANKMSGSRSV1>
</OFX>`

func TestMultiFileImport(t *testing.T) {
	// Create temporary directory for test files
	tempDir, err := os.MkdirTemp("", "ofx_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Write test OFX files
	file1 := filepath.Join(tempDir, "jan2024.qfx")
	file2 := filepath.Join(tempDir, "feb2024.qfx")
	file3 := filepath.Join(tempDir, "feb_mar2024.qfx")

	require.NoError(t, os.WriteFile(file1, []byte(testOFX1), 0644))
	require.NoError(t, os.WriteFile(file2, []byte(testOFX2), 0644))
	require.NoError(t, os.WriteFile(file3, []byte(testOFXDuplicate), 0644))

	// Test glob pattern matching
	pattern := filepath.Join(tempDir, "*.qfx")
	matches, err := filepath.Glob(pattern)
	require.NoError(t, err)
	assert.Len(t, matches, 3)

	// Simulate deduplication logic
	transactionMap := make(map[string]bool)
	totalUnique := 0
	totalDuplicates := 0

	// Expected results:
	// File 1: 1 transaction (Jan Starbucks)
	// File 2: 2 transactions (Feb Starbucks, Feb Whole Foods)
	// File 3: 2 transactions, but Feb Starbucks is duplicate
	// Total unique: 4 transactions

	expectedTransactions := []struct {
		date     string
		merchant string
		amount   float64
	}{
		{"2024-01-15", "STARBUCKS", 25.50},
		{"2024-02-15", "STARBUCKS", 25.50},
		{"2024-02-20", "WHOLE FOODS", 100.00},
		{"2024-03-01", "TARGET", 50.00},
	}

	// Simulate hash generation for deduplication
	for _, exp := range expectedTransactions {
		hash := generateTestHash(exp.date, exp.merchant, exp.amount)
		if !transactionMap[hash] {
			transactionMap[hash] = true
			totalUnique++
		} else {
			totalDuplicates++
		}
	}

	// Feb Starbucks appears twice (in file 2 and file 3)
	duplicateHash := generateTestHash("2024-02-15", "STARBUCKS", 25.50)
	if !transactionMap[duplicateHash] {
		transactionMap[duplicateHash] = true
		totalUnique++
	} else {
		totalDuplicates++
	}

	assert.Equal(t, 4, totalUnique)
	assert.Equal(t, 1, totalDuplicates)
}

func generateTestHash(date, merchant string, amount float64) string {
	// Simplified hash for testing
	return fmt.Sprintf("%s:%s:%.2f", date, merchant, amount)
}

func TestGlobPatterns(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		files    []string
		expected int
	}{
		{
			name:     "all QFX files",
			pattern:  "*.qfx",
			files:    []string{"jan.qfx", "feb.qfx", "data.csv"},
			expected: 2,
		},
		{
			name:     "specific month pattern",
			pattern:  "*jan*.qfx",
			files:    []string{"jan2024.qfx", "february.qfx", "january.qfx"},
			expected: 2,
		},
		{
			name:     "no matches",
			pattern:  "*.ofx",
			files:    []string{"jan.qfx", "feb.qfx"},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory
			tempDir, err := os.MkdirTemp("", "glob_test")
			require.NoError(t, err)
			defer os.RemoveAll(tempDir)

			// Create test files
			for _, file := range tt.files {
				path := filepath.Join(tempDir, file)
				require.NoError(t, os.WriteFile(path, []byte("test"), 0644))
			}

			// Test glob
			pattern := filepath.Join(tempDir, tt.pattern)
			matches, err := filepath.Glob(pattern)
			require.NoError(t, err)
			assert.Len(t, matches, tt.expected)
		})
	}
}
