package boardpickview

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/seanhalberthal/jiru/internal/jira"
	"github.com/seanhalberthal/jiru/internal/theme"
)

// Model is the board picker overlay.
type Model struct {
	boards    []jira.Board
	filtered  []jira.Board
	cursor    int
	loading   bool
	selected  *jira.Board
	dismissed bool
	filtering bool
	filter    textinput.Model
	width     int
	height    int
}

// New creates a new board picker.
func New() Model {
	ti := textinput.New()
	ti.Placeholder = "Filter boards"
	ti.CharLimit = 120
	return Model{loading: true, filter: ti}
}

// SetBoards populates the picker with available boards.
func (m Model) SetBoards(boards []jira.Board) Model {
	m.boards = boards
	m.filtered = append([]jira.Board(nil), boards...)
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
	m.filter.Width = max(width/2, 20)
	return m
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if m.loading {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.filtering {
			switch {
			case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
				m.filtering = false
				m.filter.Blur()
				if m.filter.Value() == "" {
					m.applyFilter()
				}
				return m, nil
			case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
				m.filtering = false
				m.filter.Blur()
				if len(m.filtered) > 0 {
					b := m.filtered[m.cursor]
					m.selected = &b
				}
				return m, nil
			}

			var cmd tea.Cmd
			m.filter, cmd = m.filter.Update(msg)
			m.applyFilter()
			return m, cmd
		}

		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("/"))):
			m.filtering = true
			m.filter.Focus()
			return m, textinput.Blink
		case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
			m.dismissed = true
		case key.Matches(msg, key.NewBinding(key.WithKeys("k", "up"))):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("j", "down"))):
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("d"))):
			if len(m.filtered) > 0 {
				step := max(len(m.filtered)/2, 1)
				m.cursor = min(m.cursor+step, len(m.filtered)-1)
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("u"))):
			if len(m.filtered) > 0 {
				step := max(len(m.filtered)/2, 1)
				m.cursor = max(m.cursor-step, 0)
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("g"))):
			m.cursor = 0
		case key.Matches(msg, key.NewBinding(key.WithKeys("G"))):
			if len(m.filtered) > 0 {
				m.cursor = len(m.filtered) - 1
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("enter", " "))):
			if len(m.filtered) > 0 {
				b := m.filtered[m.cursor]
				m.selected = &b
			}
		}
	}

	return m, nil
}

func (m *Model) applyFilter() {
	query := strings.ToLower(strings.TrimSpace(m.filter.Value()))
	if query == "" {
		m.filtered = append([]jira.Board(nil), m.boards...)
	} else {
		m.filtered = m.filtered[:0]
		for _, board := range m.boards {
			haystack := strings.ToLower(board.Name + " " + board.Type)
			if strings.Contains(haystack, query) {
				m.filtered = append(m.filtered, board)
			}
		}
	}
	if len(m.filtered) == 0 {
		m.cursor = 0
		return
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
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

	filterLine := ""
	if m.filtering || m.filter.Value() != "" {
		filterLine = theme.StyleSubtle.Render("Filter:") + " " + m.filter.View()
	}

	if len(m.filtered) == 0 {
		help := theme.StyleHelpKey.Render("/") + " " + theme.StyleHelpDesc.Render("filter") + "  " +
			theme.StyleHelpKey.Render("esc") + " " + theme.StyleHelpDesc.Render("cancel")
		content := lipgloss.JoinVertical(lipgloss.Left,
			title,
			filterLine,
			"",
			theme.StyleSubtle.Render("No matches."),
			"",
			help,
		)
		return m.centreBox(content)
	}

	var items string
	for i, b := range m.filtered {
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
		theme.StyleHelpKey.Render("/") + " " + theme.StyleHelpDesc.Render("filter") + "  " +
		theme.StyleHelpKey.Render("enter/space") + " " + theme.StyleHelpDesc.Render("select") + "  " +
		theme.StyleHelpKey.Render("esc") + " " + theme.StyleHelpDesc.Render("cancel")

	parts := []string{title}
	if filterLine != "" {
		parts = append(parts, filterLine)
	}
	parts = append(parts, items, help)

	content := lipgloss.JoinVertical(lipgloss.Left, parts...)

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
