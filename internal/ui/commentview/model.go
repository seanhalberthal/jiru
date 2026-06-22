package commentview

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/undont/jiru/internal/theme"
)

// Model is the comment input overlay.
type Model struct {
	issueKey  string
	textarea  textarea.Model
	submitted string // non-empty when user submitted a comment
	dismissed bool
	width     int
	height    int
}

// New creates a new comment input for the given issue key.
func New(issueKey string) Model {
	ta := textarea.New()
	ta.Placeholder = "Type your comment..."
	ta.Focus()
	ta.CharLimit = 0 // no limit
	ta.SetWidth(60)
	ta.SetHeight(8)

	return Model{
		issueKey: issueKey,
		textarea: ta,
	}
}

// SubmittedComment returns the comment text (once) if submitted, or empty string.
// Clears the sentinel after reading to prevent duplicate submissions.
func (m *Model) SubmittedComment() string {
	s := m.submitted
	m.submitted = ""
	return s
}

// Dismissed returns true (once) if the user cancelled.
func (m *Model) Dismissed() bool {
	d := m.dismissed
	m.dismissed = false
	return d
}

// InputActive returns true (always suppresses global keys while typing).
func (m Model) InputActive() bool {
	return true
}

// IssueKey returns the issue key.
func (m Model) IssueKey() string {
	return m.issueKey
}

// SetSize updates the overlay dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	if m.issueKey == "" {
		return // Not yet initialised — skip textarea resize.
	}
	taWidth := min(60, width-8)
	if taWidth > 0 {
		m.textarea.SetWidth(taWidth)
	}
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
			m.dismissed = true
			return m, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+s"))):
			text := m.textarea.Value()
			if text != "" {
				m.submitted = text
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
}

// View renders the comment input overlay.
func (m Model) View() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.ColourPrimary).
		MarginBottom(1)

	title := titleStyle.Render(fmt.Sprintf("Comment on %s", m.issueKey))

	help := theme.StyleHelpKey.Render("ctrl+s") + " " + theme.StyleHelpDesc.Render("submit") + "  " +
		theme.StyleHelpKey.Render("esc") + " " + theme.StyleHelpDesc.Render("cancel")

	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		m.textarea.View(),
		"",
		help,
	)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColourPrimary).
		Padding(1, 2)

	box := boxStyle.Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
