package boardpickview

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/seanhalberthal/jiru/internal/jira"
	"github.com/seanhalberthal/jiru/internal/theme"
)

// Model is the board picker overlay.
type Model struct {
	boards    []jira.Board
	cursor    int
	loading   bool
	selected  *jira.Board
	dismissed bool
	width     int
	height    int
}

// New creates a new board picker.
func New() Model {
	return Model{loading: true}
}

// SetBoards populates the picker with available boards.
func (m Model) SetBoards(boards []jira.Board) Model {
	m.boards = boards
	m.loading = false
	m.cursor = 0
	return m
}

// Selected returns the chosen board (once) and clears the sentinel.
func (m *Model) Selected() *jira.Board {
	s := m.selected
	m.selected = nil
	return s
}

// Dismissed returns true (once) if the user cancelled.
func (m *Model) Dismissed() bool {
	d := m.dismissed
	m.dismissed = false
	return d
}

// InputActive returns true while the picker is active (suppresses global keys).
func (m Model) InputActive() bool {
	return true
}

// SetSize updates the overlay dimensions.
func (m Model) SetSize(width, height int) Model {
	m.width = width
	m.height = height
	return m
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if m.loading {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
			m.dismissed = true
		case key.Matches(msg, key.NewBinding(key.WithKeys("k", "up"))):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("j", "down"))):
			if m.cursor < len(m.boards)-1 {
				m.cursor++
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("enter", " "))):
			if len(m.boards) > 0 {
				b := m.boards[m.cursor]
				m.selected = &b
			}
		}
	}

	return m, nil
}

// View renders the board picker overlay.
func (m Model) View() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.ColourPrimary).
		MarginBottom(1)

	title := titleStyle.Render("Switch Board")

	if m.loading {
		content := lipgloss.JoinVertical(lipgloss.Left,
			title,
			theme.StyleSubtle.Render("Loading boards..."),
		)
		return m.centreBox(content)
	}

	if len(m.boards) == 0 {
		content := lipgloss.JoinVertical(lipgloss.Left,
			title,
			theme.StyleSubtle.Render("No boards found."),
			"",
			theme.StyleHelpKey.Render("esc")+" "+theme.StyleHelpDesc.Render("close"),
		)
		return m.centreBox(content)
	}

	var items string
	for i, b := range m.boards {
		cursor := "  "
		style := lipgloss.NewStyle()
		if i == m.cursor {
			cursor = theme.StyleKey.Render("> ")
			style = style.Bold(true)
		}
		label := b.Name
		if b.Type != "" {
			label += theme.StyleSubtle.Render(fmt.Sprintf("  %s", b.Type))
		}
		items += cursor + style.Render(label) + "\n"
	}

	help := theme.StyleHelpKey.Render("j/k") + " " + theme.StyleHelpDesc.Render("navigate") + "  " +
		theme.StyleHelpKey.Render("enter/space") + " " + theme.StyleHelpDesc.Render("select") + "  " +
		theme.StyleHelpKey.Render("esc") + " " + theme.StyleHelpDesc.Render("cancel")

	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		items,
		help,
	)

	return m.centreBox(content)
}

func (m Model) centreBox(content string) string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColourPrimary).
		Padding(1, 2)

	box := boxStyle.Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
