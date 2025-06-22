package main

import (
	"testing"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestFormatPatternAmounts(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		pattern  model.CheckPattern
	}{
		{
			name: "nil amount min",
			pattern: model.CheckPattern{
				AmountMin: nil,
			},
			expected: "N/A",
		},
		{
			name: "exact amount",
			pattern: model.CheckPattern{
				AmountMin: ptr(100.00),
				AmountMax: nil,
			},
			expected: "$100.00",
		},
		{
			name: "exact amount with same max",
			pattern: model.CheckPattern{
				AmountMin: ptr(100.00),
				AmountMax: ptr(100.00),
			},
			expected: "$100.00",
		},
		{
			name: "range",
			pattern: model.CheckPattern{
				AmountMin: ptr(100.00),
				AmountMax: ptr(200.00),
			},
			expected: "$100.00-$200.00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatPatternAmounts(tt.pattern)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatLastUsed(t *testing.T) {
	tests := []struct {
		updatedAt time.Time
		name      string
		expected  string
		useCount  int
	}{
		{
			name:      "never used",
			updatedAt: time.Now(),
			useCount:  0,
			expected:  "Never",
		},
		{
			name:      "used once",
			updatedAt: time.Date(2024, 12, 15, 0, 0, 0, 0, time.UTC),
			useCount:  1,
			expected:  "2024-12-15",
		},
		{
			name:      "used multiple times",
			updatedAt: time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC),
			useCount:  10,
			expected:  "2024-01-31",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatLastUsed(tt.updatedAt, tt.useCount)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Helper function to create pointers.
func ptr[T any](v T) *T {
	return &v
}
