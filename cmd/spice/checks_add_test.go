package main

import (
	"bufio"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPromptAmount(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedError string
		expectedValue float64
	}{
		{
			name:          "valid amount",
			input:         "100.50\n",
			expectedValue: 100.50,
		},
		{
			name:          "amount with dollar sign",
			input:         "$250.00\n",
			expectedValue: 250.00,
		},
		{
			name:          "zero amount",
			input:         "0\n100\n", // First attempt fails, second succeeds
			expectedValue: 100.00,
		},
		{
			name:          "negative amount",
			input:         "-50\n50\n", // First attempt fails, second succeeds
			expectedValue: 50.00,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bufio.NewReader(strings.NewReader(tt.input))
			result, err := promptAmount(reader, "Amount")

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedValue, result)
			}
		})
	}
}

func TestPromptMultipleAmounts(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedError  string
		expectedValues []float64
	}{
		{
			name:           "single amount",
			input:          "100\n",
			expectedValues: []float64{100.00},
		},
		{
			name:           "multiple amounts",
			input:          "100, 200, 300\n",
			expectedValues: []float64{100.00, 200.00, 300.00},
		},
		{
			name:           "amounts with dollar signs",
			input:          "$100, $200, $300\n",
			expectedValues: []float64{100.00, 200.00, 300.00},
		},
		{
			name:           "mixed formats",
			input:          "100, $200, 300.50\n",
			expectedValues: []float64{100.00, 200.00, 300.50},
		},
		{
			name:          "invalid amount",
			input:         "100, abc, 300\n",
			expectedError: "invalid amount 'abc'",
		},
		{
			name:          "negative amount",
			input:         "100, -200, 300\n",
			expectedError: "amount must be greater than 0",
		},
		{
			name:          "empty input",
			input:         "\n",
			expectedError: "no amounts provided",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bufio.NewReader(strings.NewReader(tt.input))
			result, err := promptMultipleAmounts(reader)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedValues, result)
			}
		})
	}
}

func TestPromptInt(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		min           int
		max           int
		expectedValue int
	}{
		{
			name:          "valid day of month",
			input:         "15\n",
			min:           1,
			max:           31,
			expectedValue: 15,
		},
		{
			name:          "minimum value",
			input:         "1\n",
			min:           1,
			max:           31,
			expectedValue: 1,
		},
		{
			name:          "maximum value",
			input:         "31\n",
			min:           1,
			max:           31,
			expectedValue: 31,
		},
		{
			name:          "out of range then valid",
			input:         "0\n32\n15\n", // Two invalid attempts, then valid
			min:           1,
			max:           31,
			expectedValue: 15,
		},
		{
			name:          "non-numeric then valid",
			input:         "abc\n10\n",
			min:           1,
			max:           31,
			expectedValue: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bufio.NewReader(strings.NewReader(tt.input))
			result, err := promptInt(reader, "Day", tt.min, tt.max)

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedValue, result)
		})
	}
}

func TestPromptYesNo(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "yes lowercase",
			input:    "y\n",
			expected: true,
		},
		{
			name:     "yes full word",
			input:    "yes\n",
			expected: true,
		},
		{
			name:     "yes uppercase",
			input:    "Y\n",
			expected: true,
		},
		{
			name:     "no lowercase",
			input:    "n\n",
			expected: false,
		},
		{
			name:     "no full word",
			input:    "no\n",
			expected: false,
		},
		{
			name:     "empty input defaults to no",
			input:    "\n",
			expected: false,
		},
		{
			name:     "invalid input defaults to no",
			input:    "maybe\n",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bufio.NewReader(strings.NewReader(tt.input))
			result, err := promptYesNo(reader, "Continue?")

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPromptChoice(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedValue string
		validChoices  []string
	}{
		{
			name:          "valid choice first try",
			input:         "1\n",
			validChoices:  []string{"1", "2", "3"},
			expectedValue: "1",
		},
		{
			name:          "invalid then valid",
			input:         "4\n2\n",
			validChoices:  []string{"1", "2", "3"},
			expectedValue: "2",
		},
		{
			name:          "case insensitive",
			input:         "A\n",
			validChoices:  []string{"a", "b", "c"},
			expectedValue: "a",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bufio.NewReader(strings.NewReader(tt.input))
			result, err := promptChoice(reader, "Choice", tt.validChoices)

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedValue, result)
		})
	}
}
