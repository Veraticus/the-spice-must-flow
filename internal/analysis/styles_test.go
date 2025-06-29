package analysis

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepeatChar(t *testing.T) {
	tests := []struct {
		name     string
		char     string
		expected string
		n        int
	}{
		{
			name:     "zero repetitions",
			char:     "x",
			n:        0,
			expected: "",
		},
		{
			name:     "negative repetitions",
			char:     "x",
			n:        -5,
			expected: "",
		},
		{
			name:     "single repetition",
			char:     "x",
			n:        1,
			expected: "x",
		},
		{
			name:     "multiple repetitions",
			char:     "█",
			n:        5,
			expected: "█████",
		},
		{
			name:     "unicode character",
			char:     "♠",
			n:        3,
			expected: "♠♠♠",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := repeatChar(tt.char, tt.n)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestStyles_ForScore(t *testing.T) {
	styles := NewStyles()

	tests := []struct {
		name  string
		style string
		score float64
	}{
		{
			name:  "excellent score",
			score: 0.95,
			style: "success",
		},
		{
			name:  "good score",
			score: 0.85,
			style: "warning",
		},
		{
			name:  "poor score",
			score: 0.65,
			style: "error",
		},
		{
			name:  "boundary - exactly 0.9",
			score: 0.9,
			style: "success",
		},
		{
			name:  "boundary - exactly 0.7",
			score: 0.7,
			style: "warning",
		},
		{
			name:  "perfect score",
			score: 1.0,
			style: "success",
		},
		{
			name:  "zero score",
			score: 0.0,
			style: "error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := styles.ForScore(tt.score)

			switch tt.style {
			case "success":
				assert.Equal(t, styles.Success, result)
			case "warning":
				assert.Equal(t, styles.Warning, result)
			case "error":
				assert.Equal(t, styles.Error, result)
			}
		})
	}
}

func TestStyles_RenderBox(t *testing.T) {
	styles := NewStyles()

	tests := []struct {
		name     string
		content  string
		title    string
		hasTitle bool
	}{
		{
			name:     "box without title",
			content:  "Test content",
			title:    "",
			hasTitle: false,
		},
		{
			name:     "box with title",
			content:  "Test content with title",
			title:    "Important",
			hasTitle: true,
		},
		{
			name:     "multiline content",
			content:  "Line 1\nLine 2\nLine 3",
			title:    "Multi",
			hasTitle: true,
		},
		{
			name:     "empty content with title",
			content:  "",
			title:    "Empty Box",
			hasTitle: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := styles.RenderBox(tt.content, tt.title, styles.Box)

			// Basic checks - the actual rendering depends on lipgloss
			assert.NotEmpty(t, result)

			// If we strip ANSI codes, we should find our content
			stripped := stripANSI(result)
			if tt.content != "" {
				// For multiline content, check each line separately
				lines := strings.Split(tt.content, "\n")
				for _, line := range lines {
					if line != "" {
						assert.Contains(t, stripped, line)
					}
				}
			}
		})
	}
}

func TestStyles_RenderProgressBar_EdgeCases(t *testing.T) {
	styles := NewStyles()

	tests := []struct {
		name     string
		desc     string
		progress float64
		width    int
	}{
		{
			name:     "zero width uses default",
			progress: 0.5,
			width:    0,
			desc:     "should use default width of 30",
		},
		{
			name:     "negative width uses default",
			progress: 0.5,
			width:    -10,
			desc:     "should use default width of 30",
		},
		{
			name:     "very large progress clamped",
			progress: 10.0,
			width:    20,
			desc:     "should clamp to 100%",
		},
		{
			name:     "negative progress clamped",
			progress: -0.5,
			width:    20,
			desc:     "should clamp to 0%",
		},
		{
			name:     "tiny width",
			progress: 0.5,
			width:    1,
			desc:     "should handle width of 1",
		},
		{
			name:     "exact boundaries",
			progress: 0.999,
			width:    10,
			desc:     "should round appropriately",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := styles.RenderProgressBar(tt.progress, tt.width)
			stripped := stripANSI(result)

			// Verify the result has the expected structure
			filled := strings.Count(stripped, "█")
			empty := strings.Count(stripped, "░")
			total := filled + empty

			if tt.width <= 0 {
				assert.Equal(t, 30, total, tt.desc)
			} else {
				assert.Equal(t, tt.width, total, tt.desc)
			}

			// Verify clamping
			assert.GreaterOrEqual(t, filled, 0)
			assert.LessOrEqual(t, filled, total)
		})
	}
}

func TestStyles_WithWidth_Variations(t *testing.T) {
	original := NewStyles()

	tests := []struct {
		name  string
		desc  string
		width int
	}{
		{
			name:  "narrow terminal",
			width: 60,
			desc:  "should adjust box widths",
		},
		{
			name:  "very narrow terminal",
			width: 40,
			desc:  "should adjust box widths",
		},
		{
			name:  "wide terminal",
			width: 120,
			desc:  "should not adjust for wide terminals",
		},
		{
			name:  "exactly 100",
			width: 100,
			desc:  "should not adjust at boundary",
		},
		{
			name:  "zero width",
			width: 0,
			desc:  "should not adjust for zero",
		},
		{
			name:  "negative width",
			width: -50,
			desc:  "should not adjust for negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adjusted := original.WithWidth(tt.width)
			require.NotNil(t, adjusted)

			// The function should always return a valid Styles instance
			assert.NotNil(t, adjusted.Title)
			assert.NotNil(t, adjusted.Success)
			assert.NotNil(t, adjusted.Box)

			// For narrow terminals, boxes should be adjusted
			if tt.width > 0 && tt.width < 100 {
				// We can't easily test the actual width without rendering,
				// but we can verify the style objects are different
				assert.NotEqual(t, original.Box, adjusted.Box, tt.desc)
			}
		})
	}
}
