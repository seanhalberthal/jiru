package ui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines global keybindings for the application.
type KeyMap struct {
	Up         key.Binding
	Down       key.Binding
	Top        key.Binding
	Bottom     key.Binding
	Open       key.Binding
	Back       key.Binding
	OpenURL    key.Binding
	Refresh    key.Binding
	Quit       key.Binding
	Search     key.Binding // JQL search
	Help       key.Binding // help overlay
	Board      key.Binding
	Setup      key.Binding
	Branch     key.Binding
	Create     key.Binding
	Transition key.Binding
	Comment    key.Binding
	Filters    key.Binding
	Assign     key.Binding
	Edit       key.Binding
	Link       key.Binding
	Delete     key.Binding
	Parent     key.Binding
	IssuePick  key.Binding
	Profile    key.Binding
	HomeTab    key.Binding
	Home       key.Binding // Go to issue list view.
	Watch      key.Binding // Toggle watch on issue.
	BoardPick  key.Binding // Board switcher.
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
			key.WithKeys("s"),
			key.WithHelp("s", "JQL search"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
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
		Create: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "create issue"),
		),
		Transition: key.NewBinding(
			key.WithKeys("m"),
			key.WithHelp("m", "move"),
		),
		Comment: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "comment"),
		),
		Filters: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "saved filters"),
		),
		Assign: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "assign"),
		),
		Edit: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "edit"),
		),
		Link: key.NewBinding(
			key.WithKeys("l"),
			key.WithHelp("l", "link"),
		),
		Delete: key.NewBinding(
			key.WithKeys("D"),
			key.WithHelp("D", "delete"),
		),
		Parent: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "parent"),
		),
		IssuePick: key.NewBinding(
			key.WithKeys("i"),
			key.WithHelp("i", "issues"),
		),
		Profile: key.NewBinding(
			key.WithKeys("P"),
			key.WithHelp("P", "switch profile"),
		),
		HomeTab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "toggle"),
		),
		Home: key.NewBinding(
			key.WithKeys("H"),
			key.WithHelp("H", "home"),
		),
		Watch: key.NewBinding(
			key.WithKeys("w"),
			key.WithHelp("w", "watch"),
		),
		BoardPick: key.NewBinding(
			key.WithKeys("B"),
			key.WithHelp("B", "boards"),
		),
	}
}
