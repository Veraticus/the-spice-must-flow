package classification

import (
	"context"
	"testing"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPatternDetector(t *testing.T) {
	tests := []struct {
		name     string
		errMsg   string
		patterns []Pattern
		wantErr  bool
	}{
		{
			name: "valid patterns",
			patterns: []Pattern{
				{
					Name:       "Direct Deposit",
					Type:       PatternTypeIncome,
					Regex:      `DIRECTDEP`,
					Priority:   100,
					Confidence: 0.95,
				},
				{
					Name:       "ATM",
					Type:       PatternTypeExpense,
					Regex:      `ATM`,
					Priority:   50,
					Confidence: 0.80,
				},
			},
			wantErr: false,
		},
		{
			name: "invalid regex",
			patterns: []Pattern{
				{
					Name:       "Bad Pattern",
					Type:       PatternTypeIncome,
					Regex:      `[invalid regex`,
					Priority:   100,
					Confidence: 0.95,
				},
			},
			wantErr: true,
			errMsg:  "failed to compile pattern",
		},
		{
			name:     "empty patterns",
			patterns: []Pattern{},
			wantErr:  false,
		},
		{
			name: "patterns sorted by priority",
			patterns: []Pattern{
				{
					Name:     "Low Priority",
					Type:     PatternTypeExpense,
					Regex:    `LOW`,
					Priority: 10,
				},
				{
					Name:     "High Priority",
					Type:     PatternTypeIncome,
					Regex:    `HIGH`,
					Priority: 100,
				},
				{
					Name:     "Medium Priority",
					Type:     PatternTypeTransfer,
					Regex:    `MEDIUM`,
					Priority: 50,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pd, err := NewPatternDetector(tt.patterns)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				assert.Nil(t, pd)
			} else {
				require.NoError(t, err)
				require.NotNil(t, pd)
				assert.Equal(t, len(tt.patterns), pd.GetPatternCount())

				// Verify patterns are sorted by priority
				if len(pd.patterns) > 1 {
					for i := 0; i < len(pd.patterns)-1; i++ {
						assert.GreaterOrEqual(t, pd.patterns[i].Priority, pd.patterns[i+1].Priority)
					}
				}
			}
		})
	}
}

func TestPatternDetector_Classify(t *testing.T) {
	patterns := []Pattern{
		{
			Name:       "Direct Deposit",
			Type:       PatternTypeIncome,
			Regex:      `\b(DIRECTDEP|DIRECT\s*DEP|PAYROLL)\b`,
			Priority:   100,
			Confidence: 0.95,
		},
		{
			Name:       "Interest Income",
			Type:       PatternTypeIncome,
			Regex:      `\b(INTEREST|INT\s*EARNED)\b`,
			Priority:   95,
			Confidence: 0.90,
		},
		{
			Name:       "Transfer",
			Type:       PatternTypeTransfer,
			Regex:      `\b(TRANSFER|XFER)\b`,
			Priority:   80,
			Confidence: 0.85,
		},
		{
			Name:       "ATM",
			Type:       PatternTypeExpense,
			Regex:      `\bATM\b`,
			Priority:   50,
			Confidence: 0.80,
		},
	}

	pd, err := NewPatternDetector(patterns)
	require.NoError(t, err)

	tests := []struct {
		wantMatch   *Match
		name        string
		transaction model.Transaction
	}{
		{
			name: "direct deposit match",
			transaction: model.Transaction{
				ID:           "1",
				Name:         "EMPLOYER DIRECTDEP",
				MerchantName: "",
				Type:         "CREDIT",
				Amount:       2500.00,
			},
			wantMatch: &Match{
				PatternName: "Direct Deposit",
				Type:        PatternTypeIncome,
				Confidence:  1.0, // 0.95 + 0.05 for long pattern
			},
		},
		{
			name: "case insensitive match",
			transaction: model.Transaction{
				ID:           "2",
				Name:         "employer direct dep",
				MerchantName: "",
				Type:         "CREDIT",
				Amount:       2500.00,
			},
			wantMatch: &Match{
				PatternName: "Direct Deposit",
				Type:        PatternTypeIncome,
				Confidence:  1.0, // 0.95 + 0.05 for long pattern
			},
		},
		{
			name: "interest income match",
			transaction: model.Transaction{
				ID:           "3",
				Name:         "SAVINGS INTEREST EARNED",
				MerchantName: "BANK OF AMERICA",
				Type:         "CREDIT",
				Amount:       12.34,
			},
			wantMatch: &Match{
				PatternName: "Interest Income",
				Type:        PatternTypeIncome,
				Confidence:  0.95, // 0.90 + 0.05 for long pattern
			},
		},
		{
			name: "transfer match",
			transaction: model.Transaction{
				ID:           "4",
				Name:         "ONLINE TRANSFER TO SAVINGS",
				MerchantName: "",
				Type:         "DEBIT",
				Amount:       -500.00,
			},
			wantMatch: &Match{
				PatternName: "Transfer",
				Type:        PatternTypeTransfer,
				Confidence:  0.95, // 0.85 + 0.10 for exact name match "Transfer"
			},
		},
		{
			name: "atm match",
			transaction: model.Transaction{
				ID:           "5",
				Name:         "ATM WITHDRAWAL",
				MerchantName: "CHASE ATM",
				Type:         "DEBIT",
				Amount:       -100.00,
			},
			wantMatch: &Match{
				PatternName: "ATM",
				Type:        PatternTypeExpense,
				Confidence:  0.90, // 0.80 + 0.10 for exact name match "ATM"
			},
		},
		{
			name: "no match",
			transaction: model.Transaction{
				ID:           "6",
				Name:         "STARBUCKS COFFEE",
				MerchantName: "STARBUCKS",
				Type:         "DEBIT",
				Amount:       -5.75,
			},
			wantMatch: nil,
		},
		{
			name: "priority order - direct deposit wins over transfer",
			transaction: model.Transaction{
				ID:           "7",
				Name:         "DIRECTDEP TRANSFER FROM EMPLOYER",
				MerchantName: "",
				Type:         "CREDIT",
				Amount:       3000.00,
			},
			wantMatch: &Match{
				PatternName: "Direct Deposit",
				Type:        PatternTypeIncome,
				Confidence:  1.0, // 0.95 + 0.05 for long pattern
			},
		},
		{
			name: "merchant name match",
			transaction: model.Transaction{
				ID:           "8",
				Name:         "DEBIT CARD PURCHASE",
				MerchantName: "ATM SERVICE FEE",
				Type:         "DEBIT",
				Amount:       -3.00,
			},
			wantMatch: &Match{
				PatternName: "ATM",
				Type:        PatternTypeExpense,
				Confidence:  0.90, // 0.80 + 0.10 for exact name match "ATM"
			},
		},
		{
			name: "exact match confidence boost",
			transaction: model.Transaction{
				ID:           "9",
				Name:         "Direct dep from work",
				MerchantName: "",
				Type:         "CREDIT",
				Amount:       2000.00,
			},
			wantMatch: &Match{
				PatternName: "Direct Deposit",
				Type:        PatternTypeIncome,
				Confidence:  1.0, // 0.95 + 0.05 for long pattern
			},
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match, err := pd.Classify(ctx, tt.transaction)
			require.NoError(t, err)

			if tt.wantMatch == nil {
				assert.Nil(t, match)
			} else {
				require.NotNil(t, match)
				assert.Equal(t, tt.wantMatch.PatternName, match.PatternName)
				assert.Equal(t, tt.wantMatch.Type, match.Type)
				assert.InDelta(t, tt.wantMatch.Confidence, match.Confidence, 0.01)
			}
		})
	}
}

func TestPatternDetector_ClassifyBatch(t *testing.T) {
	patterns := []Pattern{
		{
			Name:       "Direct Deposit",
			Type:       PatternTypeIncome,
			Regex:      `DIRECTDEP`,
			Priority:   100,
			Confidence: 0.95,
		},
		{
			Name:       "ATM",
			Type:       PatternTypeExpense,
			Regex:      `ATM`,
			Priority:   50,
			Confidence: 0.80,
		},
	}

	pd, err := NewPatternDetector(patterns)
	require.NoError(t, err)

	transactions := []model.Transaction{
		{
			ID:   "1",
			Name: "EMPLOYER DIRECTDEP",
			Type: "CREDIT",
		},
		{
			ID:   "2",
			Name: "ATM WITHDRAWAL",
			Type: "DEBIT",
		},
		{
			ID:   "3",
			Name: "GROCERY STORE",
			Type: "DEBIT",
		},
		{
			ID:   "4",
			Name: "DIRECTDEP BONUS",
			Type: "CREDIT",
		},
	}

	ctx := context.Background()
	results, err := pd.ClassifyBatch(ctx, transactions)
	require.NoError(t, err)

	// Should have 3 matches (2 direct deposits, 1 ATM)
	assert.Len(t, results, 3)

	// Verify specific matches
	assert.NotNil(t, results["1"])
	assert.Equal(t, PatternTypeIncome, results["1"].Type)

	assert.NotNil(t, results["2"])
	assert.Equal(t, PatternTypeExpense, results["2"].Type)

	assert.Nil(t, results["3"]) // No match

	assert.NotNil(t, results["4"])
	assert.Equal(t, PatternTypeIncome, results["4"].Type)
}

func TestPatternDetector_ClassifyBatch_ContextCancellation(t *testing.T) {
	patterns := []Pattern{
		{
			Name:       "Test",
			Type:       PatternTypeIncome,
			Regex:      `TEST`,
			Priority:   100,
			Confidence: 0.95,
		},
	}

	pd, err := NewPatternDetector(patterns)
	require.NoError(t, err)

	// Create many transactions
	transactions := make([]model.Transaction, 1000)
	for i := 0; i < 1000; i++ {
		transactions[i] = model.Transaction{
			ID:   string(rune(i)),
			Name: "TEST TRANSACTION",
		}
	}

	// Cancel context immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = pd.ClassifyBatch(ctx, transactions)
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestPatternDetector_UpdatePatterns(t *testing.T) {
	initialPatterns := []Pattern{
		{
			Name:       "Initial",
			Type:       PatternTypeIncome,
			Regex:      `INITIAL`,
			Priority:   50,
			Confidence: 0.80,
		},
	}

	pd, err := NewPatternDetector(initialPatterns)
	require.NoError(t, err)
	assert.Equal(t, 1, pd.GetPatternCount())

	// Update with new patterns
	newPatterns := []Pattern{
		{
			Name:       "New Pattern 1",
			Type:       PatternTypeExpense,
			Regex:      `NEW1`,
			Priority:   100,
			Confidence: 0.90,
		},
		{
			Name:       "New Pattern 2",
			Type:       PatternTypeTransfer,
			Regex:      `NEW2`,
			Priority:   80,
			Confidence: 0.85,
		},
	}

	err = pd.UpdatePatterns(newPatterns)
	require.NoError(t, err)
	assert.Equal(t, 2, pd.GetPatternCount())

	// Verify new patterns work
	ctx := context.Background()
	txn := model.Transaction{
		ID:   "test",
		Name: "NEW1 TRANSACTION",
	}

	match, err := pd.Classify(ctx, txn)
	require.NoError(t, err)
	require.NotNil(t, match)
	assert.Equal(t, "New Pattern 1", match.PatternName)
	assert.Equal(t, PatternTypeExpense, match.Type)
}

func TestDefaultPatterns(t *testing.T) {
	patterns := DefaultPatterns()

	// Verify we have patterns for all types
	hasIncome := false
	hasExpense := false
	hasTransfer := false

	for _, p := range patterns {
		switch p.Type {
		case PatternTypeIncome:
			hasIncome = true
		case PatternTypeExpense:
			hasExpense = true
		case PatternTypeTransfer:
			hasTransfer = true
		}

		// Verify pattern fields
		assert.NotEmpty(t, p.Name)
		assert.NotEmpty(t, p.Regex)
		assert.Greater(t, p.Priority, 0)
		assert.GreaterOrEqual(t, p.Confidence, 0.0)
		assert.LessOrEqual(t, p.Confidence, 1.0)
	}

	assert.True(t, hasIncome, "Should have income patterns")
	assert.True(t, hasExpense, "Should have expense patterns")
	assert.True(t, hasTransfer, "Should have transfer patterns")

	// Verify all patterns compile
	_, err := NewPatternDetector(patterns)
	assert.NoError(t, err)
}

func TestPatternDetector_EdgeCases(t *testing.T) {
	patterns := []Pattern{
		{
			Name:       "Unicode Pattern",
			Type:       PatternTypeIncome,
			Regex:      `café|naïve|€`,
			Priority:   100,
			Confidence: 0.90,
		},
		{
			Name:       "Special Characters",
			Type:       PatternTypeExpense,
			Regex:      `\$\d+\.\d{2}`,
			Priority:   90,
			Confidence: 0.85,
		},
		{
			Name:       "Long Pattern",
			Type:       PatternTypeTransfer,
			Regex:      `this\sis\sa\svery\slong\spattern\sthat\sshould\sget\sa\sconfidence\sboost`,
			Priority:   80,
			Confidence: 0.80,
		},
	}

	pd, err := NewPatternDetector(patterns)
	require.NoError(t, err)

	tests := []struct {
		name        string
		transaction model.Transaction
		wantMatch   bool
	}{
		{
			name: "unicode match",
			transaction: model.Transaction{
				ID:   "1",
				Name: "Payment to café",
			},
			wantMatch: true,
		},
		{
			name: "special characters match",
			transaction: model.Transaction{
				ID:   "2",
				Name: "Amount: $123.45",
			},
			wantMatch: true,
		},
		{
			name: "empty transaction name",
			transaction: model.Transaction{
				ID:   "3",
				Name: "",
			},
			wantMatch: false,
		},
		{
			name: "very long transaction name",
			transaction: model.Transaction{
				ID:   "4",
				Name: string(make([]byte, 10000)), // 10KB string
			},
			wantMatch: false,
		},
		{
			name: "long pattern match with confidence boost",
			transaction: model.Transaction{
				ID:   "5",
				Name: "this is a very long pattern that should get a confidence boost",
			},
			wantMatch: true,
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match, err := pd.Classify(ctx, tt.transaction)
			require.NoError(t, err)

			if tt.wantMatch {
				assert.NotNil(t, match)

				// Check confidence boost for long pattern
				if tt.transaction.ID == "5" {
					assert.Greater(t, match.Confidence, 0.80)
				}
			} else {
				assert.Nil(t, match)
			}
		})
	}
}

func BenchmarkPatternDetector_Classify(b *testing.B) {
	patterns := DefaultPatterns()
	pd, err := NewPatternDetector(patterns)
	require.NoError(b, err)

	txn := model.Transaction{
		ID:           "bench",
		Name:         "EMPLOYER DIRECTDEP PAYROLL",
		MerchantName: "ACME CORP",
		Type:         "CREDIT",
		Amount:       2500.00,
		Date:         time.Now(),
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := pd.Classify(ctx, txn)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPatternDetector_ClassifyBatch(b *testing.B) {
	patterns := DefaultPatterns()
	pd, err := NewPatternDetector(patterns)
	require.NoError(b, err)

	// Create 1000 transactions
	transactions := make([]model.Transaction, 1000)
	for i := 0; i < 1000; i++ {
		transactions[i] = model.Transaction{
			ID:           string(rune(i)),
			Name:         "VARIOUS TRANSACTION TYPES",
			MerchantName: "MERCHANT",
			Type:         "DEBIT",
			Amount:       -float64(i),
		}
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := pd.ClassifyBatch(ctx, transactions)
		if err != nil {
			b.Fatal(err)
		}
	}
}
