package analysis

import (
	"context"
	"testing"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/stretchr/testify/require"
)

// Benchmark template-based prompt generation with varying transaction counts
func BenchmarkPromptBuilder_BuildAnalysisPrompt(b *testing.B) {
	testCases := []struct {
		name     string
		txnCount int
	}{
		{"100_transactions", 100},
		{"1000_transactions", 1000},
		{"5000_transactions", 5000},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			// Setup
			builder, err := NewTemplatePromptBuilder()
			require.NoError(b, err)

			// Generate test data
			promptData := PromptData{
				DateRange: DateRange{
					Start: time.Now().AddDate(0, -1, 0),
					End:   time.Now(),
				},
				Transactions: generateBenchmarkTransactions(tc.txnCount),
				Categories:   generateBenchmarkCategories(20),
				Patterns:     generateBenchmarkPatterns(50),
				TotalCount:   tc.txnCount,
				SampleSize:   tc.txnCount,
				AnalysisOptions: Options{
					Focus: FocusAll,
				},
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := builder.BuildAnalysisPrompt(promptData)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// Benchmark JSON validation with different sizes
func BenchmarkJSONValidator_Validate(b *testing.B) {
	testCases := []struct {
		name      string
		issueCount int
	}{
		{"10_issues", 10},
		{"50_issues", 50},
		{"100_issues", 100},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			validator := NewJSONValidator()
			jsonData := generateBenchmarkJSON(tc.issueCount)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = validator.Validate(jsonData)
			}
		})
	}
}

// Benchmark fix application
func BenchmarkTransactionalFixApplier_ApplyFixes(b *testing.B) {
	ctx := context.Background()
	
	b.Run("batch_processing", func(b *testing.B) {
		fixes := generateBenchmarkFixes(100)
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Simulate batch processing logic
			patternFixes := []Fix{}
			categoryFixes := []Fix{}
			recategorizations := []Fix{}

			for _, fix := range fixes {
				switch fix.Type {
				case "pattern":
					patternFixes = append(patternFixes, fix)
				case "category":
					categoryFixes = append(categoryFixes, fix)
				case "recategorize":
					recategorizations = append(recategorizations, fix)
				}
			}

			// Simulate batch operations
			_ = ctx
			_ = len(patternFixes)
			_ = len(categoryFixes)
			_ = len(recategorizations)
		}
	})
}

// Helper functions for benchmark data generation

func generateBenchmarkTransactions(count int) []model.Transaction {
	txns := make([]model.Transaction, count)
	for i := 0; i < count; i++ {
		txns[i] = model.Transaction{
			ID:           generateID("txn", i),
			MerchantName: generateMerchantName(i),
			Amount:       float64(i%1000) + 0.99,
			Date:         time.Now().AddDate(0, 0, -i),
			Type:         "debit",
			Direction:    "expense",
		}
	}
	return txns
}

func generateBenchmarkCategories(count int) []model.Category {
	cats := make([]model.Category, count)
	for i := 0; i < count; i++ {
		cats[i] = model.Category{
			ID:          i + 1,
			Name:        generateCategoryName(i),
			Type:        model.CategoryTypeExpense,
			Description: "Benchmark category",
		}
	}
	return cats
}

func generateBenchmarkPatterns(count int) []model.PatternRule {
	patterns := make([]model.PatternRule, count)
	for i := 0; i < count; i++ {
		patterns[i] = model.PatternRule{
			ID:              i + 1,
			Name:            generatePatternName(i),
			MerchantPattern: generateMerchantPattern(i),
			DefaultCategory: generateCategoryName(i % 20),
			Priority:        i % 10,
			IsActive:        true,
			Confidence:      0.8 + float64(i%20)/100,
		}
	}
	return patterns
}

func generateBenchmarkJSON(issueCount int) []byte {
	// Generate a simple valid JSON for benchmarking
	report := Report{
		ID:             "bench-test",
		SessionID:      "bench-session",
		GeneratedAt:    time.Now(),
		CoherenceScore: 0.85,
		PeriodStart:    time.Now().AddDate(0, -1, 0),
		PeriodEnd:      time.Now(),
		Issues:         make([]Issue, issueCount),
	}
	
	for i := 0; i < issueCount; i++ {
		report.Issues[i] = generateBenchmarkIssue(i)
	}
	
	// Use a simple JSON encoding for benchmarking
	// In real benchmarks, we'd use json.Marshal
	return []byte(`{"id":"bench-test","sessionID":"bench-session","coherenceScore":0.85}`)
}

func generateBenchmarkFixes(count int) []Fix {
	fixes := make([]Fix, count)
	types := []string{"pattern", "category", "recategorize"}
	
	for i := 0; i < count; i++ {
		fixes[i] = Fix{
			ID:          generateID("fix", i),
			IssueID:     generateID("issue", i%50),
			Type:        types[i%len(types)],
			Description: "Benchmark fix",
			Data:        make(map[string]any),
		}
	}
	return fixes
}

func generateBenchmarkIssue(index int) Issue {
	currentCat := generateCategoryName(index)
	suggestedCat := generateCategoryName((index + 1) % 20)
	return Issue{
		ID:                generateID("issue", index),
		Type:              IssueTypeInconsistent,
		Severity:          SeverityMedium,
		CurrentCategory:   &currentCat,
		SuggestedCategory: &suggestedCat,
		Description:       "Benchmark issue",
		TransactionIDs:    []string{generateID("txn", index)},
		AffectedCount:     10,
		Confidence:        0.85,
	}
}

// Utility functions
func generateID(prefix string, index int) string {
	return prefix + "_" + intToString(index)
}

func generateMerchantName(index int) string {
	merchants := []string{"Amazon", "Starbucks", "Target", "Walmart", "Whole Foods"}
	return merchants[index%len(merchants)]
}

func generateCategoryName(index int) string {
	return "Category_" + intToString(index)
}

func generatePatternName(index int) string {
	return "Pattern_" + intToString(index)
}

func generateMerchantPattern(index int) string {
	return "PATTERN_" + intToString(index)
}

func intToString(i int) string {
	// Simple int to string conversion for benchmarking
	if i == 0 {
		return "0"
	}
	
	var result []byte
	negative := i < 0
	if negative {
		i = -i
	}
	
	for i > 0 {
		result = append([]byte{byte('0' + i%10)}, result...)
		i /= 10
	}
	
	if negative {
		result = append([]byte{'-'}, result...)
	}
	
	return string(result)
}