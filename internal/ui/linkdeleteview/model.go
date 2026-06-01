package linkdeleteview

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/seanhalberthal/jiru/internal/jira"
	"github.com/seanhalberthal/jiru/internal/theme"
)

// DeleteRequest holds the result of the unlink confirmation.
type DeleteRequest struct {
	LinkID    string
	Relation  string
	TargetKey string
}

// Model is the unlink picker overlay: pick an existing link, then confirm.
type Model struct {
	issueKey   string
	links      []jira.LinkedIssue
	cursor     int
	confirming bool
	confirmed  *DeleteRequest
	dismissed  bool
	width      int
	height     int
}

// New creates a new unlink picker for the given issue. Only links carrying a
// link ID (i.e. deletable ones) are included.
func New(issueKey string, links []jira.LinkedIssue) Model {
	deletable := make([]jira.LinkedIssue, 0, len(links))
	for _, l := range links {
		if l.LinkID != "" {
			deletable = append(deletable, l)
		}
	}
	return Model{
		issueKey: issueKey,
		links:    deletable,
	}
}

// HasLinks reports whether there are any deletable links to show.
func (m Model) HasLinks() bool {
	return len(m.links) > 0
}

// Confirmed returns the delete request (once) and clears the sentinel.
func (m *Model) Confirmed() *DeleteRequest {
	c := m.confirmed
	m.confirmed = nil
	return c
}

// Dismissed returns true (once) if the user cancelled out of the picker.
func (m *Model) Dismissed() bool {
	d := m.dismissed
	m.dismissed = false
	return d
}

// InputActive returns true while confirming, so the parent delegates esc to
// this view (cancelling the confirmation) instead of navigating back.
func (m Model) InputActive() bool {
	return m.confirming
}

// SetSize updates the overlay dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	if m.confirming {
		switch {
		case key.Matches(keyMsg, key.NewBinding(key.WithKeys("esc", "n"))):
			m.confirming = false
		case key.Matches(keyMsg, key.NewBinding(key.WithKeys("enter", "y"))):
			if len(m.links) > 0 {
				link := m.links[m.cursor]
				m.confirmed = &DeleteRequest{
					LinkID:    link.LinkID,
					Relation:  link.Relation,
					TargetKey: link.Key,
				}
			}
		}
		return m, nil
	}

	switch {
	case key.Matches(keyMsg, key.NewBinding(key.WithKeys("esc"))):
		m.dismissed = true
	case key.Matches(keyMsg, key.NewBinding(key.WithKeys("k", "up"))):
		if m.cursor > 0 {
			m.cursor--
		}
	case key.Matches(keyMsg, key.NewBinding(key.WithKeys("j", "down"))):
		if m.cursor < len(m.links)-1 {
			m.cursor++
		}
	case key.Matches(keyMsg, key.NewBinding(key.WithKeys("enter", "d"))):
		if len(m.links) > 0 {
			m.confirming = true
		}
	}
	return m, nil
}

// View renders the unlink picker overlay.
func (m Model) View() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.ColourPrimary).
		MarginBottom(1)

	title := titleStyle.Render(fmt.Sprintf("Unlink %s", m.issueKey))

	if len(m.links) == 0 {
		content := lipgloss.JoinVertical(lipgloss.Left,
			title,
			theme.StyleSubtle.Render("No links to delete."),
			"",
			theme.StyleHelpKey.Render("esc")+" "+theme.StyleHelpDesc.Render("close"),
		)
		return m.centreBox(content)
	}

	if m.confirming {
		link := m.links[m.cursor]
		relation := link.Relation
		if relation == "" {
			relation = "relates to"
		}
		warningStyle := lipgloss.NewStyle().Foreground(theme.ColourError)
		warning := warningStyle.Render(fmt.Sprintf("Remove link: %s %s %s?", m.issueKey, relation, link.Key))
		summaryLine := ""
		if link.Summary != "" {
			summaryLine = theme.StyleSubtle.Render(link.Summary)
		}

		help := theme.StyleHelpKey.Render("y/enter") + " " + theme.StyleHelpDesc.Render("delete") + "  " +
			theme.StyleHelpKey.Render("n/esc") + " " + theme.StyleHelpDesc.Render("cancel")

		content := lipgloss.JoinVertical(lipgloss.Left,
			titleStyle.Foreground(theme.ColourError).Render(fmt.Sprintf("Unlink %s", m.issueKey)),
			warning,
			summaryLine,
			"",
			help,
		)
		boxStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.ColourError).
			Padding(1, 2)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, boxStyle.Render(content))
	}

	var items string
	for i, l := range m.links {
		cursor := "  "
		style := lipgloss.NewStyle()
		if i == m.cursor {
			cursor = theme.StyleKey.Render("> ")
			style = style.Bold(true)
		}
		relation := l.Relation
		if relation == "" {
			relation = "relates to"
		}
		row := fmt.Sprintf("%s %s", theme.StyleSubtle.Render(relation), theme.StyleKey.Render(l.Key))
		if l.Summary != "" {
			row += "  " + l.Summary
		}
		items += cursor + style.Render(row) + "\n"
	}

	help := theme.StyleHelpKey.Render("j/k") + " " + theme.StyleHelpDesc.Render("navigate") + "  " +
		theme.StyleHelpKey.Render("enter") + " " + theme.StyleHelpDesc.Render("delete") + "  " +
		theme.StyleHelpKey.Render("esc") + " " + theme.StyleHelpDesc.Render("cancel")

	subtitle := theme.StyleSubtle.Render("Select a link to remove:")

	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		subtitle,
		"",
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
