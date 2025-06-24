package testing

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

var updateGolden = flag.Bool("update-golden", false, "update golden files")

// GoldenFile handles golden file testing for visual regression detection.
type GoldenFile struct {
	t        *testing.T
	basePath string
}

// NewGoldenFile creates a new golden file tester.
func NewGoldenFile(t *testing.T, basePath string) *GoldenFile {
	t.Helper()

	// Ensure the base path exists
	if err := os.MkdirAll(basePath, 0750); err != nil {
		t.Fatalf("failed to create golden file directory: %v", err)
	}

	return &GoldenFile{
		t:        t,
		basePath: basePath,
	}
}

// Assert compares the actual output with the golden file and fails the test if they differ.
func (g *GoldenFile) Assert(name, actual string) {
	g.t.Helper()

	goldenPath := filepath.Join(g.basePath, name+".golden")

	if *updateGolden {
		if err := os.WriteFile(goldenPath, []byte(actual), 0600); err != nil {
			g.t.Fatalf("failed to update golden file: %v", err)
		}
		g.t.Logf("Updated golden file: %s", goldenPath)
		return
	}

	expected, err := os.ReadFile(goldenPath) // #nosec G304 - goldenPath is constructed from controlled inputs
	if err != nil {
		if os.IsNotExist(err) {
			g.t.Fatalf("golden file does not exist: %s\nRun with -update-golden to create it", goldenPath)
		}
		g.t.Fatalf("failed to read golden file: %v", err)
	}

	if string(expected) != actual {
		g.t.Errorf("output does not match golden file: %s", goldenPath)
		g.t.Errorf("Expected:\n%s", expected)
		g.t.Errorf("Actual:\n%s", actual)

		// Show diff if available
		if diff := g.diff(string(expected), actual); diff != "" {
			g.t.Errorf("Diff:\n%s", diff)
		}
	}
}

// AssertStripped compares the actual output with the golden file after stripping ANSI codes.
func (g *GoldenFile) AssertStripped(name, actual string) {
	g.t.Helper()
	stripped := StripANSI(actual)
	g.Assert(name+"_stripped", stripped)
}

// diff computes a simple line-by-line diff between two strings.
func (g *GoldenFile) diff(expected, actual string) string {
	expectedLines := strings.Split(expected, "\n")
	actualLines := strings.Split(actual, "\n")

	var diff strings.Builder
	maxLines := len(expectedLines)
	if len(actualLines) > maxLines {
		maxLines = len(actualLines)
	}

	for i := 0; i < maxLines; i++ {
		var expectedLine, actualLine string

		if i < len(expectedLines) {
			expectedLine = expectedLines[i]
		}
		if i < len(actualLines) {
			actualLine = actualLines[i]
		}

		if expectedLine != actualLine {
			diff.WriteString("- ")
			diff.WriteString(expectedLine)
			diff.WriteString("\n+ ")
			diff.WriteString(actualLine)
			diff.WriteString("\n")
		}
	}

	return diff.String()
}

// GoldenTest provides a convenient way to run golden file tests.
type GoldenTest struct {
	Name   string
	Model  func() tea.Model // Returns tea.Model
	Inputs []tea.Msg        // tea.Msg inputs
}

// RunGoldenTests executes a set of golden file tests.
func RunGoldenTests(t *testing.T, basePath string, tests []GoldenTest) {
	t.Helper()

	golden := NewGoldenFile(t, basePath)

	for _, test := range tests {
		t.Run(test.Name, func(_ *testing.T) {
			// Create the model
			model := test.Model()
			renderer := NewTestRenderer()

			// Apply inputs
			for _, input := range test.Inputs {
				model, _ = renderer.Update(model, input)
			}

			// Get final output
			output := renderer.Render(model)

			// Compare with golden file
			golden.Assert(test.Name, output)
		})
	}
}
