package assignpickview

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/seanhalberthal/jiru/internal/client"
	"github.com/seanhalberthal/jiru/internal/theme"
)

// AssignRequest holds the result of the assignee picker.
type AssignRequest struct {
	AccountID   string // "none" for unassign, "default" for assign-to-me
	DisplayName string // for status message
}

// fixedOption represents the fixed options at the top of the list.
type fixedOption struct {
	label     string
	accountID string
}

var fixedOptions = []fixedOption{
	{"Assign to me", "default"},
	{"Unassign", "none"},
}

// Model is the assignee search and picker overlay.
type Model struct {
	issueKey        string
	currentAssignee string
	input           textinput.Model
	users           []client.UserInfo
	cursor          int
	selected        *AssignRequest
	dismissed       bool
	needsSearch     string
	lastSearch      string
	lastSearchTime  time.Time
	width           int
	height          int
}

// New creates a new assignee picker for the given issue.
func New(issueKey, currentAssignee string) Model {
	ti := textinput.New()
	ti.Placeholder = "Search users..."
	ti.Focus()
	ti.CharLimit = 100
	ti.Width = 40

	return Model{
		issueKey:        issueKey,
		currentAssignee: currentAssignee,
		input:           ti,
	}
}

// SelectedAssignee returns the selection (once) and clears the sentinel.
func (m *Model) SelectedAssignee() *AssignRequest {
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

// InputActive returns true (always suppresses global keys).
func (m Model) InputActive() bool {
	return true
}

// NeedsUserSearch returns the search prefix when a debounced search should fire.
func (m *Model) NeedsUserSearch() string {
	s := m.needsSearch
	m.needsSearch = ""
	return s
}

// SetUsers populates the search results.
func (m *Model) SetUsers(users []client.UserInfo) {
	m.users = users
	// Reset cursor to first search result (after fixed options).
	m.cursor = 0
}

// SetSize updates the overlay dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// totalItems returns the total number of selectable items.
func (m Model) totalItems() int {
	return len(fixedOptions) + len(m.users)
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
			m.dismissed = true
			return m, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("up", "ctrl+p"))):
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("down", "ctrl+n"))):
			if m.cursor < m.totalItems()-1 {
				m.cursor++
			}
			return m, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
			total := m.totalItems()
			if total == 0 {
				return m, nil
			}
			if m.cursor < len(fixedOptions) {
				opt := fixedOptions[m.cursor]
				m.selected = &AssignRequest{
					AccountID:   opt.accountID,
					DisplayName: opt.label,
				}
			} else {
				idx := m.cursor - len(fixedOptions)
				if idx < len(m.users) {
					u := m.users[idx]
					m.selected = &AssignRequest{
						AccountID:   u.AccountID,
						DisplayName: u.DisplayName,
					}
				}
			}
			return m, nil
		}
	}

	// Update text input.
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)

	// Debounce: trigger search when input changes and enough time has passed.
	val := m.input.Value()
	if val != m.lastSearch && len(val) >= 2 {
		now := time.Now()
		if now.Sub(m.lastSearchTime) > 300*time.Millisecond {
			m.needsSearch = val
			m.lastSearch = val
			m.lastSearchTime = now
		}
	}

	return m, cmd
}

// View renders the assignee picker overlay.
func (m Model) View() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.ColourPrimary).
		MarginBottom(1)

	title := titleStyle.Render(fmt.Sprintf("Assign %s", m.issueKey))

	var currentLine string
	if m.currentAssignee != "" {
		currentLine = theme.StyleSubtle.Render(fmt.Sprintf("Current: %s", m.currentAssignee))
	} else {
		currentLine = theme.StyleSubtle.Render("Currently unassigned")
	}

	var items string
	for i, opt := range fixedOptions {
		cursor := "  "
		style := lipgloss.NewStyle()
		if i == m.cursor {
			cursor = theme.StyleKey.Render("> ")
			style = style.Bold(true)
		}
		items += cursor + style.Render(opt.label) + "\n"
	}

	if len(m.users) > 0 {
		items += "\n"
		for i, u := range m.users {
			idx := i + len(fixedOptions)
			cursor := "  "
			style := lipgloss.NewStyle()
			if idx == m.cursor {
				cursor = theme.StyleKey.Render("> ")
				style = style.Bold(true)
			}
			items += cursor + style.Render(u.DisplayName) + "\n"
		}
	}

	help := theme.StyleHelpKey.Render("↑/↓") + " " + theme.StyleHelpDesc.Render("navigate") + "  " +
		theme.StyleHelpKey.Render("enter") + " " + theme.StyleHelpDesc.Render("select") + "  " +
		theme.StyleHelpKey.Render("esc") + " " + theme.StyleHelpDesc.Render("cancel")

	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		currentLine,
		"",
		m.input.View(),
		"",
		items,
		help,
	)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColourPrimary).
		Padding(1, 2)

	box := boxStyle.Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
