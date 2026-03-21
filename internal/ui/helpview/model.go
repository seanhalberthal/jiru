package helpview

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/seanhalberthal/jiru/internal/theme"
)

// section is a named group of keybindings.
type section struct {
	Title    string
	Bindings []binding
}

type binding struct {
	Key  string
	Desc string
}

// Model is the help overlay.
type Model struct {
	viewport  viewport.Model
	dismissed bool
	width     int
	height    int
	built     bool
}

// New creates a new help overlay.
func New() Model {
	return Model{}
}

// Dismissed returns true (once) if the user closed the overlay.
func (m *Model) Dismissed() bool {
	d := m.dismissed
	m.dismissed = false
	return d
}

// InputActive returns true while the overlay is showing (suppresses global keys).
func (m Model) InputActive() bool {
	return true
}

// SetSize updates the overlay dimensions and rebuilds content.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.built = false
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("esc", "q", "?"))):
			m.dismissed = true
			return m, nil
		}
	}

	if !m.built {
		m.build()
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// View renders the help overlay.
func (m *Model) View() string {
	if !m.built {
		m.build()
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColourPrimary).
		Padding(1, 2)

	// Cap the box width.
	maxWidth := 72
	if m.width-4 < maxWidth {
		maxWidth = m.width - 4
	}

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.ColourPrimary).
		Render("Keyboard Shortcuts")

	footer := theme.StyleHelpKey.Render("esc/q/?") + " " + theme.StyleHelpDesc.Render("close") + "  " +
		theme.StyleHelpKey.Render("j/k") + " " + theme.StyleHelpDesc.Render("scroll")

	inner := lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		m.viewport.View(),
		"",
		footer,
	)

	box := boxStyle.Width(maxWidth).Render(inner)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m *Model) build() {
	sections := allSections()

	keyStyle := lipgloss.NewStyle().
		Foreground(theme.ColourPrimary).
		Bold(true).
		Width(14)

	descStyle := lipgloss.NewStyle()

	sectionTitleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.ColourSubtle).
		MarginTop(1)

	var lines []string
	for i, sec := range sections {
		if i > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, sectionTitleStyle.Render(sec.Title))
		for _, b := range sec.Bindings {
			lines = append(lines, fmt.Sprintf(
				"%s%s",
				keyStyle.Render(b.Key),
				descStyle.Render(b.Desc),
			))
		}
	}

	content := strings.Join(lines, "\n")

	// Calculate available height for viewport (box padding + title + spacing + footer).
	vpHeight := m.height - 12
	if vpHeight < 5 {
		vpHeight = 5
	}

	maxWidth := min(m.width-4, 72)

	m.viewport = viewport.New(maxWidth-6, vpHeight) // account for box border + padding
	m.viewport.SetContent(content)
	m.built = true
}

func allSections() []section {
	return []section{
		{
			Title: "Navigation",
			Bindings: []binding{
				{"j / ↓", "Move down"},
				{"k / ↑", "Move up"},
				{"g / G", "Jump to top / bottom"},
				{"d / u", "Half-page down / up"},
				{"h / l", "Previous / next column (board view)"},
				{"enter", "Open / select"},
				{"esc", "Go back"},
				{"q", "Quit (at top level) / go back"},
			},
		},
		{
			Title: "Search & Filters",
			Bindings: []binding{
				{"s", "Open JQL search"},
				{"/", "Filter current list"},
				{"f", "Saved filters"},
				{"r", "Refresh / re-run query"},
			},
		},
		{
			Title: "Issue Operations",
			Bindings: []binding{
				{"c", "Create issue / add comment"},
				{"e", "Edit issue"},
				{"m", "Move (transition status)"},
				{"a", "Assign issue"},
				{"l", "Link issue"},
				{"w", "Watch / unwatch issue"},
				{"D", "Delete issue"},
				{"n", "Create branch from issue"},
				{"p", "Go to parent issue"},
				{"i", "Go to issue (picker)"},
				{"o", "Open in browser"},
				{"x", "Copy URL to clipboard"},
			},
		},
		{
			Title: "Views",
			Bindings: []binding{
				{"b", "Toggle board / list view"},
				{"B", "Switch board"},
				{"H", "Go home"},
				{"tab", "Toggle Jira / Confluence wiki"},
				{"P", "Switch profile"},
				{"S", "Setup wizard"},
			},
		},
		{
			Title: "Confluence Wiki",
			Bindings: []binding{
				{"tab", "Switch between Jira and Confluence"},
				{"enter", "Open space / page"},
				{"p", "View pages in space"},
				{"/", "Filter spaces or pages"},
				{"o", "Open page in browser"},
				{"esc", "Go back"},
			},
		},
		{
			Title: "Filter Manager",
			Bindings: []binding{
				{"n", "New filter"},
				{"e", "Edit filter"},
				{"d", "Duplicate filter"},
				{"f", "Toggle favourite"},
				{"x", "Copy JQL to clipboard"},
				{"D", "Delete filter"},
			},
		},
	}
}
