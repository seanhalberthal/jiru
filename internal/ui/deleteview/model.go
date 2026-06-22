package deleteview

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/undont/jiru/internal/theme"
)

// DeleteRequest holds the result of the delete confirmation.
type DeleteRequest struct {
	Key     string
	Cascade bool
}

// Model is the delete confirmation overlay.
type Model struct {
	issueKey  string
	summary   string
	cascade   bool
	confirmed *DeleteRequest
	dismissed bool
	width     int
	height    int
}

// New creates a new delete confirmation for the given issue.
func New(issueKey, summary string) Model {
	return Model{
		issueKey: issueKey,
		summary:  summary,
	}
}

// Confirmed returns the delete request (once) and clears the sentinel.
func (m *Model) Confirmed() *DeleteRequest {
	c := m.confirmed
	m.confirmed = nil
	return c
}

// Dismissed returns true (once) if the user cancelled.
func (m *Model) Dismissed() bool {
	d := m.dismissed
	m.dismissed = false
	return d
}

// InputActive returns true (always captures all keys).
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
		case key.Matches(msg, key.NewBinding(key.WithKeys("esc", "n"))):
			m.dismissed = true
		case key.Matches(msg, key.NewBinding(key.WithKeys("enter", "y"))):
			m.confirmed = &DeleteRequest{
				Key:     m.issueKey,
				Cascade: m.cascade,
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("tab"))):
			m.cascade = !m.cascade
		}
	}
	return m, nil
}

// View renders the delete confirmation overlay.
func (m Model) View() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.ColourError).
		MarginBottom(1)

	title := titleStyle.Render("Delete Issue")

	warningStyle := lipgloss.NewStyle().Foreground(theme.ColourError)
	warning := warningStyle.Render(fmt.Sprintf("Are you sure you want to delete %s?", m.issueKey))

	summaryLine := theme.StyleSubtle.Render(m.summary)

	cascadeLabel := "[ ] Also delete subtasks"
	if m.cascade {
		cascadeLabel = "[✓] Also delete subtasks"
	}
	cascadeLine := lipgloss.NewStyle().Render(cascadeLabel)

	help := theme.StyleHelpKey.Render("y/enter") + " " + theme.StyleHelpDesc.Render("confirm") + "  " +
		theme.StyleHelpKey.Render("tab") + " " + theme.StyleHelpDesc.Render("toggle subtasks") + "  " +
		theme.StyleHelpKey.Render("n/esc") + " " + theme.StyleHelpDesc.Render("cancel")

	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		warning,
		summaryLine,
		"",
		cascadeLine,
		"",
		help,
	)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColourError).
		Padding(1, 2)

	box := boxStyle.Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
