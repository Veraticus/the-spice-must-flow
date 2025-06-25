// Package classification provides transaction classification patterns and detection.
package classification

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
)

// PatternType represents the type of transaction pattern.
type PatternType string

const (
	// PatternTypeIncome represents income transactions.
	PatternTypeIncome PatternType = "income"
	// PatternTypeExpense represents expense transactions.
	PatternTypeExpense PatternType = "expense"
	// PatternTypeTransfer represents transfer transactions.
	PatternTypeTransfer PatternType = "transfer"
)

// Pattern represents a transaction classification pattern.
type Pattern struct {
	Name       string
	Type       PatternType
	Regex      string
	Priority   int     // Higher priority patterns are checked first
	Confidence float64 // Base confidence when pattern matches (0.0-1.0)
}

// CompiledPattern holds a compiled regex pattern with metadata.
type CompiledPattern struct {
	compiledRegex *regexp.Regexp
	Pattern
}

// PatternDetector implements pattern-based transaction classification.
type PatternDetector struct {
	patterns []CompiledPattern
	mu       sync.RWMutex
}

// NewPatternDetector creates a new pattern detector with the given patterns.
func NewPatternDetector(patterns []Pattern) (*PatternDetector, error) {
	compiled := make([]CompiledPattern, 0, len(patterns))

	for _, p := range patterns {
		// Add word boundaries for better matching
		regexStr := p.Regex
		if !strings.HasPrefix(regexStr, "(?i)") {
			regexStr = "(?i)" + regexStr // Make case-insensitive by default
		}

		regex, err := regexp.Compile(regexStr)
		if err != nil {
			return nil, fmt.Errorf("failed to compile pattern %s: %w", p.Name, err)
		}

		compiled = append(compiled, CompiledPattern{
			Pattern:       p,
			compiledRegex: regex,
		})
	}

	// Sort by priority (highest first)
	for i := 0; i < len(compiled)-1; i++ {
		for j := i + 1; j < len(compiled); j++ {
			if compiled[j].Priority > compiled[i].Priority {
				compiled[i], compiled[j] = compiled[j], compiled[i]
			}
		}
	}

	return &PatternDetector{
		patterns: compiled,
	}, nil
}

// Match represents a pattern match result.
type Match struct {
	PatternName string
	Type        PatternType
	Confidence  float64
}

// Classify attempts to classify a transaction based on patterns.
func (pd *PatternDetector) Classify(_ context.Context, txn model.Transaction) (*Match, error) {
	pd.mu.RLock()
	defer pd.mu.RUnlock()

	// Build search text from transaction fields
	searchText := strings.ToLower(fmt.Sprintf("%s %s %s",
		txn.Name,
		txn.MerchantName,
		txn.Type,
	))

	// Check each pattern in priority order
	for _, pattern := range pd.patterns {
		if pattern.compiledRegex.MatchString(searchText) {
			// Calculate confidence based on pattern specificity
			confidence := pattern.Confidence

			// Boost confidence for exact matches
			if strings.Contains(searchText, strings.ToLower(pattern.Name)) {
				confidence = minFloat(confidence+0.1, 1.0)
			}

			// Boost confidence for longer patterns (more specific)
			if len(pattern.Regex) > 20 {
				confidence = minFloat(confidence+0.05, 1.0)
			}

			return &Match{
				PatternName: pattern.Name,
				Type:        pattern.Type,
				Confidence:  confidence,
			}, nil
		}
	}

	// No match found, return nil match with nil error
	return nil, nil //nolint:nilnil // No match is a valid result
}

// ClassifyBatch classifies multiple transactions efficiently.
func (pd *PatternDetector) ClassifyBatch(ctx context.Context, transactions []model.Transaction) (map[string]*Match, error) {
	results := make(map[string]*Match, len(transactions))

	for _, txn := range transactions {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			match, err := pd.Classify(ctx, txn)
			if err != nil {
				return nil, fmt.Errorf("failed to classify transaction %s: %w", txn.ID, err)
			}
			if match != nil {
				results[txn.ID] = match
			}
		}
	}

	return results, nil
}

// UpdatePatterns updates the detector with new patterns.
func (pd *PatternDetector) UpdatePatterns(patterns []Pattern) error {
	compiled := make([]CompiledPattern, 0, len(patterns))

	for _, p := range patterns {
		regexStr := p.Regex
		if !strings.HasPrefix(regexStr, "(?i)") {
			regexStr = "(?i)" + regexStr
		}

		regex, err := regexp.Compile(regexStr)
		if err != nil {
			return fmt.Errorf("failed to compile pattern %s: %w", p.Name, err)
		}

		compiled = append(compiled, CompiledPattern{
			Pattern:       p,
			compiledRegex: regex,
		})
	}

	// Sort by priority
	for i := 0; i < len(compiled)-1; i++ {
		for j := i + 1; j < len(compiled); j++ {
			if compiled[j].Priority > compiled[i].Priority {
				compiled[i], compiled[j] = compiled[j], compiled[i]
			}
		}
	}

	pd.mu.Lock()
	pd.patterns = compiled
	pd.mu.Unlock()

	return nil
}

// GetPatternCount returns the number of loaded patterns.
func (pd *PatternDetector) GetPatternCount() int {
	pd.mu.RLock()
	defer pd.mu.RUnlock()
	return len(pd.patterns)
}

// minFloat returns the minimum of two float64 values.
func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
