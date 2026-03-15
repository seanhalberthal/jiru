package ui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines global keybindings for the application.
type KeyMap struct {
	Up      key.Binding
	Down    key.Binding
	Top     key.Binding
	Bottom  key.Binding
	Open    key.Binding
	Back    key.Binding
	OpenURL key.Binding
	Refresh key.Binding
	Quit    key.Binding
	Search  key.Binding // JQL search — now "?"
	Home    key.Binding
	Board   key.Binding
	Setup   key.Binding
	Branch  key.Binding
}

// DefaultKeyMap returns the default vim-style keybindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("k/↑", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("j/↓", "down"),
		),
		Top: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("gg", "top"),
		),
		Bottom: key.NewBinding(
			key.WithKeys("G"),
			key.WithHelp("G", "bottom"),
		),
		Open: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "open"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		OpenURL: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "open in browser"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Search: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "JQL search"),
		),
		Home: key.NewBinding(
			key.WithKeys("H"),
			key.WithHelp("H", "home"),
		),
		Board: key.NewBinding(
			key.WithKeys("b"),
			key.WithHelp("b", "board view"),
		),
		Setup: key.NewBinding(
			key.WithKeys("S"),
			key.WithHelp("S", "setup"),
		),
		Branch: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "create branch"),
		),
	}
}
