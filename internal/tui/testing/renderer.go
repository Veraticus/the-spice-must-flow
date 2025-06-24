// Package testing provides test utilities for TUI components.
package testing

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// TestRenderer captures the output of a Bubble Tea component without requiring a real terminal.
type TestRenderer struct {
	// Output contains the last rendered view
	Output string

	// Commands contains all commands returned by Update calls
	Commands []tea.Cmd

	// Messages contains all messages sent to the component
	Messages []tea.Msg

	// UpdateCount tracks how many times Update was called
	UpdateCount int
}

// NewTestRenderer creates a new test renderer.
func NewTestRenderer() *TestRenderer {
	return &TestRenderer{
		Commands: make([]tea.Cmd, 0),
		Messages: make([]tea.Msg, 0),
	}
}

// Render renders a component and captures its output.
func (r *TestRenderer) Render(model tea.Model) string {
	r.Output = model.View()
	return r.Output
}

// Update sends a message to the component and captures the result.
func (r *TestRenderer) Update(model tea.Model, msg tea.Msg) (tea.Model, tea.Cmd) {
	r.Messages = append(r.Messages, msg)
	r.UpdateCount++

	newModel, cmd := model.Update(msg)
	if cmd != nil {
		r.Commands = append(r.Commands, cmd)
	}

	// Update the rendered output
	r.Output = newModel.View()

	return newModel, cmd
}

// ProcessCommands executes all pending commands and returns their messages.
// This is useful for testing command chains.
func (r *TestRenderer) ProcessCommands(model tea.Model) []tea.Msg {
	var messages []tea.Msg

	for _, cmd := range r.Commands {
		if cmd == nil {
			continue
		}

		// Execute the command and collect the message
		msg := cmd()
		if msg != nil {
			messages = append(messages, msg)
			// Update the model with the message
			model, _ = r.Update(model, msg)
		}
	}

	// Clear processed commands
	r.Commands = nil

	return messages
}

// LastCommand returns the most recent command, or nil if no commands were generated.
func (r *TestRenderer) LastCommand() tea.Cmd {
	if len(r.Commands) == 0 {
		return nil
	}
	return r.Commands[len(r.Commands)-1]
}

// StripANSI removes ANSI escape codes from the output for content-only testing.
func (r *TestRenderer) StripANSI() string {
	return StripANSI(r.Output)
}

// Lines returns the output split by newlines.
func (r *TestRenderer) Lines() []string {
	return strings.Split(r.Output, "\n")
}

// Reset clears all captured data.
func (r *TestRenderer) Reset() {
	r.Output = ""
	r.Commands = nil
	r.Messages = nil
	r.UpdateCount = 0
}
