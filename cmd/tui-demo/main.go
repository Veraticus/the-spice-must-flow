// Package main provides a demo program for the TUI
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/Veraticus/the-spice-must-flow/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// Create TUI with test mode enabled
	ctx := context.Background()

	prompter, err := tui.New(ctx,
		tui.WithTestMode(true),
		tui.WithTestData(100, []string{
			"Whole Foods Market",
			"Amazon.com",
			"Shell Oil",
			"Netflix",
			"Starbucks",
			"Target",
			"Uber",
			"Chipotle",
			"CVS Pharmacy",
			"Home Depot",
		}),
		tui.WithSize(120, 40),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Type assert to get access to the program
	tuiPrompter, ok := prompter.(*tui.Prompter)
	if !ok {
		log.Fatal("unexpected prompter type")
	}

	// Run the TUI program directly
	if _, err := tea.NewProgram(tuiPrompter.Model()).Run(); err != nil {
		// Use explicit error check to satisfy forbidigo
		_, _ = fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}
