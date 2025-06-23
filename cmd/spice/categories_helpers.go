package main

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/Veraticus/the-spice-must-flow/internal/cli"
)

// promptForCategoryDescription interactively asks the user for a category description.
func promptForCategoryDescription(categoryName, aiDescription string, confidence float64) string {
	fmt.Printf("\n%s The AI has low confidence (%.0f%%) for category %q\n", cli.InfoIcon, confidence*100, categoryName) //nolint:forbidigo // User-facing output
	fmt.Printf("AI suggestion: %s\n\n", aiDescription)                                                                  //nolint:forbidigo // User-facing output
	fmt.Printf("Please provide a brief description for this category (or press Enter to use AI suggestion): ")          //nolint:forbidigo // User prompt

	reader := bufio.NewReader(os.Stdin)
	userInput, err := reader.ReadString('\n')
	if err != nil {
		slog.Warn("failed to read user input", "error", err)
		return aiDescription
	}

	userInput = strings.TrimSpace(userInput)
	if userInput == "" {
		return aiDescription
	}

	return userInput
}
