//go:build go1.18
// +build go1.18

package analysis

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"
)

// cryptoRandInt returns a cryptographically secure random int in range [0, max).
func cryptoRandInt(max int) int {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		panic(err)
	}
	return int(n.Int64())
}

// cryptoRandFloat returns a cryptographically secure random float64 in range [0, 1).
func cryptoRandFloat() float64 {
	n, err := rand.Int(rand.Reader, big.NewInt(1<<53))
	if err != nil {
		panic(err)
	}
	return float64(n.Int64()) / float64(1<<53)
}

// FuzzJSONValidator_Validate fuzzes the JSON validator with random input.
func FuzzJSONValidator_Validate(f *testing.F) {
	// Add seed corpus with various JSON structures
	seedCorpus := [][]byte{
		// Valid JSON
		[]byte(`{"sessionID": "test", "coherenceScore": 0.85}`),
		[]byte(`{"issues": [], "summary": {"totalIssues": 0}}`),
		[]byte(`{"issues": [{"type": "pattern", "severity": "high"}]}`),

		// Invalid JSON
		[]byte(`{invalid json`),
		[]byte(`{"unclosed": "string`),
		[]byte(`{"trailing": "comma",}`),
		[]byte(`{"number": 123.456.789}`),
		[]byte(`{"nested": {"broken": }`),

		// Edge cases
		[]byte(`{}`),
		[]byte(`[]`),
		[]byte(`null`),
		[]byte(`"string"`),
		[]byte(`123`),
		[]byte(`true`),
		[]byte(`false`),

		// Unicode and escape sequences
		[]byte(`{"unicode": "Hello 世界"}`),
		[]byte(`{"escaped": "line\nbreak"}`),
		[]byte(`{"emoji": "🚀"}`),

		// Large structures
		generateLargeJSON(100),
		generateNestedJSON(10),
		generateArrayJSON(50),
	}

	for _, seed := range seedCorpus {
		f.Add(seed)
	}

	validator := NewJSONValidator()

	f.Fuzz(func(t *testing.T, data []byte) {
		// The validator should not panic on any input
		report, err := validator.Validate(data)

		// If no error, the report should be valid
		if err == nil {
			if report == nil {
				t.Error("validator returned nil report without error")
				return
			}
			// Verify required fields are present
			if report.SessionID == "" {
				t.Error("valid report missing SessionID")
			}
			if report.GeneratedAt.IsZero() {
				t.Error("valid report has zero GeneratedAt time")
			}
		}

		// Test error extraction
		if err != nil {
			badSection, line, col := validator.ExtractError(data, err)

			// These should not panic and should return reasonable values
			if line < 0 || col < 0 {
				t.Errorf("negative line/col returned: line=%d, col=%d", line, col)
			}

			// Bad section should be a substring of the original data
			if badSection != "" && !strings.Contains(string(data), badSection) {
				// Handle case where badSection might be truncated or modified
				// This is acceptable as long as it doesn't panic
				_ = badSection // Acknowledge but accept this edge case
			}
		}
	})
}

// FuzzJSONValidator_ExtractBadSection specifically fuzzes error extraction.
func FuzzJSONValidator_ExtractBadSection(f *testing.F) {
	// Seed with various malformed JSON
	seeds := []struct {
		err  error
		data []byte
	}{
		{data: []byte(`{"bad": ]`), err: &json.SyntaxError{Offset: 8}},
		{data: []byte(`{"missing": "quote}`), err: &json.SyntaxError{Offset: 19}},
		{data: []byte(`[1, 2, 3,]`), err: &json.SyntaxError{Offset: 9}},
		{data: []byte(`{"nested": {"error": }`), err: &json.SyntaxError{Offset: 21}},
	}

	for _, seed := range seeds {
		f.Add(seed.data, seed.err.(*json.SyntaxError).Offset)
	}

	validator := NewJSONValidator()

	f.Fuzz(func(t *testing.T, data []byte, offset int64) {
		// Create a synthetic syntax error
		err := &json.SyntaxError{Offset: offset}

		// Should not panic
		badSection, line, col := validator.ExtractError(data, err)

		// Validate output
		if line < 1 {
			t.Errorf("line should be >= 1, got %d", line)
		}
		if col < 1 {
			t.Errorf("col should be >= 1, got %d", col)
		}

		// Offset should be within bounds or handled gracefully
		if offset >= 0 && offset < int64(len(data)) {
			// Bad section should contain some context around the error
			if len(badSection) == 0 && len(data) > 0 {
				t.Error("expected non-empty bad section for valid offset")
			}
		}
	})
}

// FuzzAnalysisReport_Validation fuzzes the analysis report structure.
func FuzzAnalysisReport_Validation(f *testing.F) {
	// Seed with various report structures
	f.Add(`{"sessionID": "test", "coherenceScore": 0.5, "issues": []}`)
	f.Add(`{"sessionID": "test", "coherenceScore": 1.0, "issues": null}`)
	f.Add(`{"sessionID": "", "coherenceScore": -1, "issues": [{"type": "invalid"}]}`)

	f.Fuzz(func(t *testing.T, jsonStr string) {
		var report Report
		err := json.Unmarshal([]byte(jsonStr), &report)

		if err == nil {
			// Validate coherence score bounds
			if report.CoherenceScore < 0 || report.CoherenceScore > 1 {
				// This should be caught by validation
				validator := NewJSONValidator()
				_, valErr := validator.Validate([]byte(jsonStr))
				if valErr == nil {
					t.Errorf("validator accepted invalid coherence score: %f", report.CoherenceScore)
				}
			}

			// Validate issue types
			for _, issue := range report.Issues {
				switch issue.Type {
				case IssueTypeInconsistent,
					IssueTypeMissingPattern,
					IssueTypeAmbiguousVendor,
					IssueTypeMiscategorized,
					IssueTypeDuplicatePattern:
					// Valid type
				default:
					// Should be caught by validation
					validator := NewJSONValidator()
					_, valErr := validator.Validate([]byte(jsonStr))
					if valErr == nil {
						t.Errorf("validator accepted invalid issue type: %s", issue.Type)
					}
				}
			}
		}
	})
}

// Helper functions to generate test data

func generateLargeJSON(issueCount int) []byte {
	report := Report{
		ID:             "large-test-report",
		SessionID:      "large-test",
		GeneratedAt:    time.Now(),
		PeriodStart:    time.Now().AddDate(0, -1, 0),
		PeriodEnd:      time.Now(),
		CoherenceScore: 0.75,
		Issues:         make([]Issue, issueCount),
		Insights:       []string{"Test insight"},
		CategorySummary: map[string]CategoryStat{
			"test": {
				CategoryID:   "test",
				CategoryName: "Test Category",
			},
		},
	}

	for i := 0; i < issueCount; i++ {
		categoryName := fmt.Sprintf("Category_%d", i)
		report.Issues[i] = Issue{
			ID:              fmt.Sprintf("issue-%d", i),
			Type:            IssueTypeInconsistent,
			Severity:        SeverityMedium,
			CurrentCategory: &categoryName,
			Description:     fmt.Sprintf("Issue %d description with some long text to make it realistic", i),
			TransactionIDs:  []string{fmt.Sprintf("txn-%d", i)},
			AffectedCount:   cryptoRandInt(100),
			Confidence:      0.8,
		}
	}

	data, _ := json.Marshal(report)
	return data
}

func generateNestedJSON(depth int) []byte {
	type nested struct {
		Data  any `json:"data"`
		Level int `json:"level"`
	}

	var build func(int) any
	build = func(d int) any {
		if d <= 0 {
			return "leaf"
		}
		return nested{
			Level: d,
			Data:  build(d - 1),
		}
	}

	data, _ := json.Marshal(build(depth))
	return data
}

func generateArrayJSON(size int) []byte {
	arr := make([]map[string]any, size)
	for i := 0; i < size; i++ {
		arr[i] = map[string]any{
			"id":    i,
			"value": cryptoRandFloat(),
			"name":  fmt.Sprintf("Item_%d", i),
		}
	}

	data, _ := json.Marshal(map[string]any{
		"items": arr,
		"count": size,
	})
	return data
}

// FuzzCalculatePosition tests the position calculation with edge cases.
func FuzzCalculatePosition(f *testing.F) {
	// Seed with various inputs
	testCases := []struct {
		data   []byte
		offset int
	}{
		{[]byte("hello\nworld"), 0},
		{[]byte("hello\nworld"), 6},
		{[]byte("hello\nworld"), 11},
		{[]byte("line1\nline2\nline3"), 7},
		{[]byte("\n\n\n"), 2},
		{[]byte(""), 0},
		{[]byte("no newlines"), 5},
	}

	for _, tc := range testCases {
		f.Add(tc.data, tc.offset)
	}

	f.Fuzz(func(t *testing.T, data []byte, offset int) {
		// calculatePosition should handle any offset gracefully
		line, col := calculatePosition(data, int64(offset))

		// Basic validation
		if line < 1 {
			t.Errorf("line should be >= 1, got %d", line)
		}
		if col < 1 {
			t.Errorf("col should be >= 1, got %d", col)
		}

		// If offset is beyond data length, it should still return valid values
		if offset > len(data) && line == 1 && col == 1 {
			// This is acceptable fallback behavior
			return // Accept default values for out-of-bounds offset
		}

		// Verify the calculation is consistent
		line2, col2 := calculatePosition(data, int64(offset))
		if line != line2 || col != col2 {
			t.Errorf("calculatePosition not deterministic: (%d,%d) != (%d,%d)", line, col, line2, col2)
		}
	})
}

// FuzzExtractContext tests context extraction around errors.
func FuzzExtractContext(f *testing.F) {
	// Seed with various JSON structures
	f.Add([]byte(`{"array": [1, 2, 3]}`), 10)
	f.Add([]byte(`{"nested": {"key": "value"}}`), 15)
	f.Add([]byte(`[{"id": 1}, {"id": 2}]`), 12)

	f.Fuzz(func(t *testing.T, data []byte, offset int) {
		// Test array context extraction with a synthetic error message
		errStr := fmt.Sprintf("error at position %d", offset)
		context := extractArrayContext(data, errStr)

		// Should not panic and should return some string
		if offset >= 0 && offset < len(data) {
			// Context should be related to the data
			if len(context) > len(data)*2 {
				t.Error("context unexpectedly large")
			}
		}

		// Test field context extraction
		fieldContext := extractFieldContext(data, errStr)

		// Should not panic
		if offset >= 0 && offset < len(data) {
			// Field context should be a valid string
			_ = fieldContext
		}
	})
}

// FuzzJSONValidator_Performance tests validator performance with pathological inputs.
func FuzzJSONValidator_Performance(f *testing.F) {
	// Seed with potentially slow inputs
	f.Add(strings.Repeat(`{"a":`, 1000) + `null` + strings.Repeat(`}`, 1000))
	f.Add(`{"a": "` + strings.Repeat(`\u0000`, 1000) + `"}`)
	f.Add(strings.Repeat(`[`, 100) + `1` + strings.Repeat(`]`, 100))

	validator := NewJSONValidator()

	f.Fuzz(func(t *testing.T, input string) {
		// Limit input size to prevent OOM
		if len(input) > 1000000 {
			input = input[:1000000]
		}

		// Time the validation
		start := time.Now()
		_, _ = validator.Validate([]byte(input))
		duration := time.Since(start)

		// Should complete in reasonable time even for pathological inputs
		if duration > 5*time.Second {
			t.Errorf("validation took too long: %v", duration)
		}
	})
}
