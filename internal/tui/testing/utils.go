package testing

import (
	"regexp"
	"strings"
	"time"
)

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// StripANSI removes all ANSI escape codes from a string.
func StripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

// NormalizeWhitespace converts all whitespace sequences to single spaces and trims the result.
func NormalizeWhitespace(s string) string {
	// Replace all whitespace sequences with a single space
	re := regexp.MustCompile(`\s+`)
	normalized := re.ReplaceAllString(s, " ")
	return strings.TrimSpace(normalized)
}

// ExtractLines extracts specific lines from output, useful for testing partial content.
func ExtractLines(output string, start, end int) []string {
	lines := strings.Split(output, "\n")
	if start < 0 {
		start = 0
	}
	if end > len(lines) {
		end = len(lines)
	}
	if start >= end {
		return nil
	}
	return lines[start:end]
}

// ContainsInOrder checks if the output contains all specified strings in order.
func ContainsInOrder(output string, expected ...string) bool {
	lastIndex := 0
	for _, exp := range expected {
		index := strings.Index(output[lastIndex:], exp)
		if index == -1 {
			return false
		}
		lastIndex += index + len(exp)
	}
	return true
}

// TimeController provides deterministic time for testing time-based behaviors.
type TimeController struct {
	current time.Time
}

// NewTimeController creates a new time controller with a fixed starting time.
func NewTimeController(start time.Time) *TimeController {
	return &TimeController{current: start}
}

// Now returns the current controlled time.
func (tc *TimeController) Now() time.Time {
	return tc.current
}

// Advance advances the controlled time by the specified duration.
func (tc *TimeController) Advance(d time.Duration) {
	tc.current = tc.current.Add(d)
}

// Set sets the controlled time to a specific value.
func (tc *TimeController) Set(t time.Time) {
	tc.current = t
}

// Timer creates a timer message for testing time-based updates.
func (tc *TimeController) Timer(id int, d time.Duration) TimerMsg {
	return TimerMsg{
		ID:   id,
		Time: tc.current.Add(d),
	}
}

// TimerMsg represents a timer tick for testing.
type TimerMsg struct {
	Time time.Time
	ID   int
}

// String implements fmt.Stringer for TimerMsg.
func (t TimerMsg) String() string {
	return "timer"
}

// AssertContains checks if the output contains a substring and returns an error message if not.
func AssertContains(output, expected string) string {
	if !strings.Contains(output, expected) {
		return "output does not contain expected string"
	}
	return ""
}

// AssertNotContains checks if the output does not contain a substring.
func AssertNotContains(output, unexpected string) string {
	if strings.Contains(output, unexpected) {
		return "output contains unexpected string"
	}
	return ""
}

// CompareLines compares two multi-line strings and returns a description of differences.
func CompareLines(actual, expected string) string {
	actualLines := strings.Split(actual, "\n")
	expectedLines := strings.Split(expected, "\n")

	var diffs []string
	maxLines := len(actualLines)
	if len(expectedLines) > maxLines {
		maxLines = len(expectedLines)
	}

	for i := 0; i < maxLines; i++ {
		var actualLine, expectedLine string

		if i < len(actualLines) {
			actualLine = actualLines[i]
		}
		if i < len(expectedLines) {
			expectedLine = expectedLines[i]
		}

		if actualLine != expectedLine {
			diffs = append(diffs, "line "+string(rune(i+1))+": got '"+actualLine+"', want '"+expectedLine+"'")
		}
	}

	if len(diffs) > 0 {
		return strings.Join(diffs, "\n")
	}
	return ""
}
