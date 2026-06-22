package profilepickview

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/undont/jiru/internal/theme"
)

// Model is the profile picker overlay.
type Model struct {
	profiles      []string
	filtered      []string
	activeProfile string
	cursor        int
	selected      string
	dismissed     bool
	newProfile    bool
	filtering     bool
	filter        textinput.Model
	width         int
	height        int

	// renaming holds the in-progress rename of renameTarget.
	renaming     bool
	renameTarget string
	renameInput  textinput.Model
	renamedFrom  string
	renamedTo    string

	// confirmingDelete guards the destructive delete of deleteTarget.
	confirmingDelete bool
	deleteTarget     string
	deleted          string
}

// New creates a new profile picker.
func New(profiles []string, active string) Model {
	ti := textinput.New()
	ti.Placeholder = "Filter profiles"
	ti.CharLimit = 120
	ri := textinput.New()
	ri.Placeholder = "New name"
	ri.CharLimit = 120
	cursor := 0
	for i, p := range profiles {
		if p == active {
			cursor = i
			break
		}
	}
	return Model{
		profiles:      profiles,
		filtered:      append([]string(nil), profiles...),
		activeProfile: active,
		cursor:        cursor,
		filter:        ti,
		renameInput:   ri,
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

// RenameRequest returns a committed rename (once) and clears the sentinel.
// ok is false when no rename is pending.
func (m *Model) RenameRequest() (oldName, newName string, ok bool) {
	if m.renamedFrom == "" {
		return "", "", false
	}
	oldName, newName = m.renamedFrom, m.renamedTo
	m.renamedFrom, m.renamedTo = "", ""
	return oldName, newName, true
}

// Deleted returns the profile the user confirmed for deletion (once).
func (m *Model) Deleted() string {
	d := m.deleted
	m.deleted = ""
	return d
}

// InputActive returns true while the picker is active (suppresses global keys).
func (m Model) InputActive() bool {
	return true
}

// SetSize updates the overlay dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.filter.Width = max(width/2, 20)
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.renaming {
			switch {
			case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
				m.cancelRename()
				return m, nil
			case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
				newName := strings.TrimSpace(m.renameInput.Value())
				switch newName {
				case "":
					// Empty name — ignore, stay in rename mode.
					return m, nil
				case m.renameTarget:
					// Unchanged — treat as cancel.
					m.cancelRename()
					return m, nil
				default:
					m.renamedFrom = m.renameTarget
					m.renamedTo = newName
					m.cancelRename()
					return m, nil
				}
			}
			var cmd tea.Cmd
			m.renameInput, cmd = m.renameInput.Update(msg)
			return m, cmd
		}

		if m.confirmingDelete {
			switch {
			case key.Matches(msg, key.NewBinding(key.WithKeys("y"))):
				m.deleted = m.deleteTarget
				m.confirmingDelete = false
				m.deleteTarget = ""
			case key.Matches(msg, key.NewBinding(key.WithKeys("n", "esc"))):
				m.confirmingDelete = false
				m.deleteTarget = ""
			}
			return m, nil
		}

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
					m.selected = m.filtered[m.cursor]
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
				m.selected = m.filtered[m.cursor]
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("n"))):
			m.newProfile = true
		case key.Matches(msg, key.NewBinding(key.WithKeys("r"))):
			if len(m.filtered) > 0 {
				m.renaming = true
				m.renameTarget = m.filtered[m.cursor]
				m.renameInput.SetValue(m.renameTarget)
				m.renameInput.CursorEnd()
				m.renameInput.Focus()
				return m, textinput.Blink
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("x"))):
			if len(m.filtered) > 0 {
				m.confirmingDelete = true
				m.deleteTarget = m.filtered[m.cursor]
			}
		}
	}

	return m, nil
}

// cancelRename exits rename mode and resets its input.
func (m *Model) cancelRename() {
	m.renaming = false
	m.renameTarget = ""
	m.renameInput.Blur()
	m.renameInput.SetValue("")
}

func (m *Model) applyFilter() {
	query := strings.ToLower(strings.TrimSpace(m.filter.Value()))
	if query == "" {
		m.filtered = append([]string(nil), m.profiles...)
	} else {
		m.filtered = m.filtered[:0]
		for _, profile := range m.profiles {
			if strings.Contains(strings.ToLower(profile), query) {
				m.filtered = append(m.filtered, profile)
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

// View renders the profile picker overlay.
func (m Model) View() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.ColourPrimary).
		MarginBottom(1)

	title := titleStyle.Render("Switch Profile")

	if m.renaming {
		help := theme.StyleHelpKey.Render("enter") + " " + theme.StyleHelpDesc.Render("confirm") + "  " +
			theme.StyleHelpKey.Render("esc") + " " + theme.StyleHelpDesc.Render("cancel")
		content := lipgloss.JoinVertical(
			lipgloss.Left,
			title,
			theme.StyleSubtle.Render("Rename ")+theme.StyleKey.Render(m.renameTarget)+theme.StyleSubtle.Render(" to:"),
			"",
			m.renameInput.View(),
			"",
			help,
		)
		return m.centreBox(content)
	}

	if m.confirmingDelete {
		help := theme.StyleHelpKey.Render("y") + " " + theme.StyleHelpDesc.Render("delete") + "  " +
			theme.StyleHelpKey.Render("n/esc") + " " + theme.StyleHelpDesc.Render("cancel")
		content := lipgloss.JoinVertical(
			lipgloss.Left,
			title,
			theme.StyleSubtle.Render("Delete profile ")+theme.StyleKey.Render(m.deleteTarget)+theme.StyleSubtle.Render("?"),
			theme.StyleSubtle.Render("This removes its config, saved token, filters and recents."),
			"",
			help,
		)
		return m.centreBox(content)
	}

	if len(m.profiles) == 0 {
		content := lipgloss.JoinVertical(
			lipgloss.Left,
			title,
			theme.StyleSubtle.Render("No profiles configured."),
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
		content := lipgloss.JoinVertical(
			lipgloss.Left,
			title,
			filterLine,
			"",
			theme.StyleSubtle.Render("No matches."),
			"",
			help,
		)
		return m.centreBox(content)
	}

	var items strings.Builder
	for i, p := range m.filtered {
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
		items.WriteString(cursor)
		items.WriteString(style.Render(p))
		items.WriteString(marker)
		items.WriteString("\n")
	}

	help := theme.StyleHelpKey.Render("j/k") + " " + theme.StyleHelpDesc.Render("navigate") + "  " +
		theme.StyleHelpKey.Render("/") + " " + theme.StyleHelpDesc.Render("filter") + "  " +
		theme.StyleHelpKey.Render("enter/space") + " " + theme.StyleHelpDesc.Render("select") + "  " +
		theme.StyleHelpKey.Render("n") + " " + theme.StyleHelpDesc.Render("new profile") + "  " +
		theme.StyleHelpKey.Render("esc") + " " + theme.StyleHelpDesc.Render("cancel") + "  " +
		theme.StyleHelpKey.Render("x") + " " + theme.StyleHelpDesc.Render("delete") + "  " +
		theme.StyleHelpKey.Render("r") + " " + theme.StyleHelpDesc.Render("rename")

	parts := []string{title}
	if filterLine != "" {
		parts = append(parts, filterLine)
	}
	parts = append(parts, items.String(), help)

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
