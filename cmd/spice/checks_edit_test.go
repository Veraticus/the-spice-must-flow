package main

import (
	"bufio"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPromptStringWithDefault(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		defaultValue string
		expected     string
	}{
		{
			name:         "use default with empty input",
			input:        "\n",
			defaultValue: "default value",
			expected:     "default value",
		},
		{
			name:         "override default",
			input:        "new value\n",
			defaultValue: "default value",
			expected:     "new value",
		},
		{
			name:         "whitespace trimmed",
			input:        "  trimmed  \n",
			defaultValue: "default",
			expected:     "trimmed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bufio.NewReader(strings.NewReader(tt.input))
			result, err := promptStringWithDefault(reader, "Enter value", tt.defaultValue)

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPromptIntWithDefault(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		defaultValue int
		min          int
		max          int
		expected     int
	}{
		{
			name:         "use default with empty input",
			input:        "\n",
			defaultValue: 15,
			min:          1,
			max:          31,
			expected:     15,
		},
		{
			name:         "override default",
			input:        "20\n",
			defaultValue: 15,
			min:          1,
			max:          31,
			expected:     20,
		},
		{
			name:         "invalid then use default",
			input:        "abc\n\n",
			defaultValue: 10,
			min:          1,
			max:          31,
			expected:     10,
		},
		{
			name:         "out of range then valid",
			input:        "50\n25\n",
			defaultValue: 15,
			min:          1,
			max:          31,
			expected:     25,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bufio.NewReader(strings.NewReader(tt.input))
			result, err := promptIntWithDefault(reader, "Enter day", tt.defaultValue, tt.min, tt.max)

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
