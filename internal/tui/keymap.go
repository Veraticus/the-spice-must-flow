package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines all keyboard shortcuts.
type KeyMap struct {
	// Navigation
	Up       key.Binding
	Down     key.Binding
	Left     key.Binding
	Right    key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Home     key.Binding
	End      key.Binding

	// Actions
	Select key.Binding
	Accept key.Binding
	Reject key.Binding
	Skip   key.Binding
	Custom key.Binding
	Undo   key.Binding
	Redo   key.Binding

	// View modes
	ToggleView   key.Binding
	ToggleHelp   key.Binding
	ToggleStats  key.Binding
	ToggleSearch key.Binding
	ToggleFilter key.Binding

	// Selection
	SelectAll    key.Binding
	DeselectAll  key.Binding
	ToggleSelect key.Binding
	VisualMode   key.Binding

	// Application
	Quit        key.Binding
	ForceQuit   key.Binding
	Help        key.Binding
	Refresh     key.Binding
	ClearScreen key.Binding
}

// DefaultKeyMap returns the default key bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		// Navigation
		Up: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("↓/j", "down"),
		),
		Left: key.NewBinding(
			key.WithKeys("h", "left"),
			key.WithHelp("←/h", "left"),
		),
		Right: key.NewBinding(
			key.WithKeys("l", "right"),
			key.WithHelp("→/l", "right"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "ctrl+b"),
			key.WithHelp("PgUp/Ctrl+B", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", "ctrl+f"),
			key.WithHelp("PgDn/Ctrl+F", "page down"),
		),
		Home: key.NewBinding(
			key.WithKeys("home", "g"),
			key.WithHelp("Home/g", "go to start"),
		),
		End: key.NewBinding(
			key.WithKeys("end", "G"),
			key.WithHelp("End/G", "go to end"),
		),

		// Actions
		Select: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("Enter", "select/classify"),
		),
		Accept: key.NewBinding(
			key.WithKeys("a", "y"),
			key.WithHelp("a/y", "accept suggestion"),
		),
		Reject: key.NewBinding(
			key.WithKeys("r", "n"),
			key.WithHelp("r/n", "reject suggestion"),
		),
		Skip: key.NewBinding(
			key.WithKeys("s", "space"),
			key.WithHelp("s/Space", "skip transaction"),
		),
		Custom: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "custom category"),
		),
		Undo: key.NewBinding(
			key.WithKeys("u", "ctrl+z"),
			key.WithHelp("u/Ctrl+Z", "undo"),
		),
		Redo: key.NewBinding(
			key.WithKeys("ctrl+r", "ctrl+y"),
			key.WithHelp("Ctrl+R", "redo"),
		),

		// View modes
		ToggleView: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("Tab", "cycle views"),
		),
		ToggleHelp: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "toggle help"),
		),
		ToggleStats: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("Ctrl+S", "toggle stats"),
		),
		ToggleSearch: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
		ToggleFilter: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "filter"),
		),

		// Selection
		SelectAll: key.NewBinding(
			key.WithKeys("ctrl+a"),
			key.WithHelp("Ctrl+A", "select all"),
		),
		DeselectAll: key.NewBinding(
			key.WithKeys("ctrl+d"),
			key.WithHelp("Ctrl+D", "deselect all"),
		),
		ToggleSelect: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "toggle selection"),
		),
		VisualMode: key.NewBinding(
			key.WithKeys("v"),
			key.WithHelp("v", "visual mode"),
		),

		// Application
		Quit: key.NewBinding(
			key.WithKeys("q", "esc"),
			key.WithHelp("q/Esc", "quit"),
		),
		ForceQuit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("Ctrl+C", "force quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("ctrl+r"),
			key.WithHelp("Ctrl+R", "refresh"),
		),
		ClearScreen: key.NewBinding(
			key.WithKeys("ctrl+l"),
			key.WithHelp("Ctrl+L", "clear screen"),
		),
	}
}

// ShortHelp returns key bindings for the short help view.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Select, k.Quit}
}

// FullHelp returns all key bindings for the full help view.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right},
		{k.PageUp, k.PageDown, k.Home, k.End},
		{k.Select, k.Accept, k.Reject, k.Skip},
		{k.Custom, k.Undo, k.ToggleSearch, k.ToggleFilter},
		{k.Help, k.Quit},
	}
}
