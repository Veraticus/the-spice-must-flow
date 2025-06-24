// Package visual provides visual testing capabilities for TUI components.
package visual

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// GoldenManager handles golden file operations for visual regression testing.
type GoldenManager struct {
	basePath string
	update   bool
}

// NewGoldenManager creates a new golden file manager.
func NewGoldenManager(basePath string, update bool) *GoldenManager {
	return &GoldenManager{
		basePath: basePath,
		update:   update,
	}
}

// Compare compares actual output with golden file.
func (g *GoldenManager) Compare(name string, actual string) error {
	goldenPath := filepath.Join(g.basePath, name)

	if g.update {
		// Ensure directory exists
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0750); err != nil {
			return fmt.Errorf("failed to create golden directory: %w", err)
		}

		// Write golden file
		if err := os.WriteFile(goldenPath, []byte(actual), 0600); err != nil {
			return fmt.Errorf("failed to write golden file: %w", err)
		}

		return nil
	}

	// Read golden file
	cleanPath := filepath.Clean(goldenPath)
	golden, err := os.ReadFile(cleanPath) // #nosec G304 -- test file path from test name
	if err != nil {
		return fmt.Errorf("failed to read golden file: %w", err)
	}

	// Compare
	if string(golden) != actual {
		// Write actual for debugging
		actualPath := goldenPath + ".actual"
		if err := os.WriteFile(actualPath, []byte(actual), 0600); err != nil {
			// Log but don't fail
			log.Printf("Warning: failed to write actual file: %v", err)
		}

		return fmt.Errorf("output does not match golden file %s", name)
	}

	return nil
}

// Diff returns a string showing differences between golden and actual.
func (g *GoldenManager) Diff(name string, actual string) (string, error) {
	goldenPath := filepath.Join(g.basePath, name)

	cleanPath := filepath.Clean(goldenPath)
	golden, err := os.ReadFile(cleanPath) // #nosec G304 -- test file path from test name
	if err != nil {
		return "", fmt.Errorf("failed to read golden file: %w", err)
	}

	return generateDiff(string(golden), actual), nil
}

// generateDiff creates a simple line-by-line diff.
func generateDiff(golden, actual string) string {
	goldenLines := strings.Split(golden, "\n")
	actualLines := strings.Split(actual, "\n")

	var diff strings.Builder
	diff.WriteString("Differences found:\n")

	maxLines := len(goldenLines)
	if len(actualLines) > maxLines {
		maxLines = len(actualLines)
	}

	for i := 0; i < maxLines; i++ {
		var goldenLine, actualLine string

		if i < len(goldenLines) {
			goldenLine = goldenLines[i]
		}
		if i < len(actualLines) {
			actualLine = actualLines[i]
		}

		if goldenLine != actualLine {
			diff.WriteString(fmt.Sprintf("Line %d:\n", i+1))
			diff.WriteString(fmt.Sprintf("- Golden: %s\n", goldenLine))
			diff.WriteString(fmt.Sprintf("+ Actual: %s\n", actualLine))
		}
	}

	return diff.String()
}

// Clean removes .actual files from failed tests.
func (g *GoldenManager) Clean() error {
	return filepath.Walk(g.basePath, func(path string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if strings.HasSuffix(path, ".actual") {
			if err := os.Remove(path); err != nil {
				return err
			}
		}

		return nil
	})
}
