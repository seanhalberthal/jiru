package profilepickview

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/seanhalberthal/jiru/internal/theme"
)

// Model is the profile picker overlay.
type Model struct {
	profiles      []string
	activeProfile string
	cursor        int
	selected      string
	dismissed     bool
	newProfile    bool
	width         int
	height        int
}

// New creates a new profile picker.
func New(profiles []string, active string) Model {
	cursor := 0
	for i, p := range profiles {
		if p == active {
			cursor = i
			break
		}
	}
	return Model{
		profiles:      profiles,
		activeProfile: active,
		cursor:        cursor,
	}
}

// Selected returns the chosen profile name (once) and clears the sentinel.
func (m *Model) Selected() string {
	s := m.selected
	m.selected = ""
	return s
}

// Dismissed returns true (once) if the user cancelled.
func (m *Model) Dismissed() bool {
	d := m.dismissed
	m.dismissed = false
	return d
}

// NewProfile returns true (once) if the user wants to create a new profile.
func (m *Model) NewProfile() bool {
	n := m.newProfile
	m.newProfile = false
	return n
}

// InputActive returns true while the picker is active (suppresses global keys).
func (m Model) InputActive() bool {
	return true
}

// SetSize updates the overlay dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
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
			if m.cursor < len(m.profiles)-1 {
				m.cursor++
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
			if len(m.profiles) > 0 {
				m.selected = m.profiles[m.cursor]
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("n"))):
			m.newProfile = true
		}
	}

	return m, nil
}

// View renders the profile picker overlay.
func (m Model) View() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.ColourPrimary).
		MarginBottom(1)

	title := titleStyle.Render("Switch Profile")

	if len(m.profiles) == 0 {
		content := lipgloss.JoinVertical(lipgloss.Left,
			title,
			theme.StyleSubtle.Render("No profiles configured."),
			"",
			theme.StyleHelpKey.Render("esc")+" "+theme.StyleHelpDesc.Render("close"),
		)
		return m.centreBox(content)
	}

	var items string
	for i, p := range m.profiles {
		cursor := "  "
		style := lipgloss.NewStyle()
		if i == m.cursor {
			cursor = theme.StyleKey.Render("> ")
			style = style.Bold(true)
		}
		marker := ""
		if p == m.activeProfile {
			marker = theme.StyleSubtle.Render(" (active)")
		}
		items += cursor + style.Render(p) + marker + "\n"
	}

	help := theme.StyleHelpKey.Render("j/k") + " " + theme.StyleHelpDesc.Render("navigate") + "  " +
		theme.StyleHelpKey.Render("enter") + " " + theme.StyleHelpDesc.Render("select") + "  " +
		theme.StyleHelpKey.Render("n") + " " + theme.StyleHelpDesc.Render("new profile") + "  " +
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
