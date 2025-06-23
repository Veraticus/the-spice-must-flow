// Package main provides a demo of the TUI with test data
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/Veraticus/the-spice-must-flow/internal/tui"
)

func main() {
	ctx := context.Background()

	// Create TUI with test mode enabled
	prompter, err := tui.New(ctx,
		tui.WithTestMode(true),
	)
	if err != nil {
		log.Fatalf("Failed to create TUI: %v", err)
	}

	// Type assert to get access to the TUI implementation
	tuiPrompter, ok := prompter.(*tui.Prompter)
	if !ok {
		log.Fatal("Unexpected prompter type")
	}

	fmt.Println("Starting TUI demo with test data...")
	fmt.Println("Use arrow keys to navigate, Enter to classify, ? for help, q to quit")

	// Run the TUI
	if err := tuiPrompter.Start(); err != nil {
		log.Fatalf("Failed to start TUI: %v", err)
	}
}
