package testing

import (
	tea "github.com/charmbracelet/bubbletea"
)

// KeyPress creates a key press message for testing.
func KeyPress(key string) tea.KeyMsg {
	return tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune(key),
	}
}

// KeyDown creates a down arrow key message.
func KeyDown() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyDown}
}

// KeyUp creates an up arrow key message.
func KeyUp() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyUp}
}

// KeyLeft creates a left arrow key message.
func KeyLeft() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyLeft}
}

// KeyRight creates a right arrow key message.
func KeyRight() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRight}
}

// KeyEnter creates an enter key message.
func KeyEnter() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyEnter}
}

// KeyEsc creates an escape key message.
func KeyEsc() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyEsc}
}

// KeyTab creates a tab key message.
func KeyTab() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyTab}
}

// KeyBackspace creates a backspace key message.
func KeyBackspace() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyBackspace}
}

// KeyCtrl creates a ctrl key combination message.
func KeyCtrl(key string) tea.KeyMsg {
	if len(key) != 1 {
		panic("KeyCtrl requires a single character")
	}

	r := rune(key[0])
	// Convert to control character (Ctrl+A = 1, Ctrl+Z = 26)
	ctrlRune := r - 'a' + 1
	if r >= 'A' && r <= 'Z' {
		ctrlRune = r - 'A' + 1
	}

	return tea.KeyMsg{
		Type:  tea.KeyCtrlC + tea.KeyType(ctrlRune-3), // Adjust for tea.KeyCtrlC offset
		Runes: []rune{ctrlRune},
	}
}

// WindowSize creates a window size message for testing responsive layouts.
func WindowSize(width, height int) tea.WindowSizeMsg {
	return tea.WindowSizeMsg{
		Width:  width,
		Height: height,
	}
}

// MouseClick creates a mouse click message at the specified coordinates.
func MouseClick(x, y int) tea.MouseMsg {
	return tea.MouseMsg{
		X:      x,
		Y:      y,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	}
}

// MouseMotion creates a mouse motion message.
func MouseMotion(x, y int) tea.MouseMsg {
	return tea.MouseMsg{
		X:      x,
		Y:      y,
		Action: tea.MouseActionMotion,
	}
}

// InputSequence represents a sequence of inputs for testing.
type InputSequence struct {
	inputs []tea.Msg
}

// NewInputSequence creates a new input sequence.
func NewInputSequence(inputs ...tea.Msg) *InputSequence {
	return &InputSequence{inputs: inputs}
}

// Add adds an input to the sequence.
func (s *InputSequence) Add(input tea.Msg) *InputSequence {
	s.inputs = append(s.inputs, input)
	return s
}

// Type adds a string of characters to the sequence.
func (s *InputSequence) Type(text string) *InputSequence {
	for _, r := range text {
		s.inputs = append(s.inputs, tea.KeyMsg{
			Type:  tea.KeyRunes,
			Runes: []rune{r},
		})
	}
	return s
}

// Apply applies all inputs in the sequence to a model using a renderer.
func (s *InputSequence) Apply(model tea.Model, renderer *TestRenderer) tea.Model {
	result := model
	for _, input := range s.inputs {
		result, _ = renderer.Update(result, input)
	}
	return result
}

// Messages returns all messages in the sequence.
func (s *InputSequence) Messages() []tea.Msg {
	return s.inputs
}
