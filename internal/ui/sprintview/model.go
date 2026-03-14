package sprintview

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/seanhalberthal/jiru/internal/jira"
	"github.com/seanhalberthal/jiru/internal/theme"
	"github.com/seanhalberthal/jiru/internal/ui/issuedelegate"
)

// Model is the sprint issue list view.
type Model struct {
	list     list.Model
	issues   []jira.Issue
	width    int
	height   int
	selected *jira.Issue // set when user presses enter.
	openKeys key.Binding
}

// New creates a new sprint view model.
func New() Model {
	delegate := issuedelegate.Delegate{}
	l := list.New(nil, delegate, 0, 0)
	l.Title = "Issues"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false) // We handle help ourselves.
	l.Styles.Title = theme.StyleTitle

	return Model{
		list: l,
		openKeys: key.NewBinding(
			key.WithKeys("enter"),
		),
	}
}

// SetSize updates the dimensions.
func (m Model) SetSize(width, height int) Model {
	m.width = width
	m.height = height
	m.list.SetSize(width, height)
	return m
}

// SetIssues populates the list with issues.
func (m Model) SetIssues(issues []jira.Issue) Model {
	m.issues = issues
	m.list.SetItems(issuedelegate.ToItems(issues))
	m.list.Title = fmt.Sprintf("Issues (%d)", len(issues))
	return m
}

// SelectedIssue returns the issue the user selected (if any) and resets the selection.
func (m *Model) SelectedIssue() (jira.Issue, bool) {
	if m.selected == nil {
		return jira.Issue{}, false
	}
	iss := *m.selected
	m.selected = nil
	return iss, true
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Don't handle keys when filtering.
		if m.list.FilterState() == list.Filtering {
			break
		}

		if key.Matches(msg, m.openKeys) {
			if item, ok := m.list.SelectedItem().(issueItem); ok {
				m.selected = &item.Issue
				return m, nil
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// Filtering returns true when the list filter input is active.
func (m Model) Filtering() bool {
	return m.list.FilterState() == list.Filtering
}

// View renders the sprint view.
func (m Model) View() string {
	return m.list.View()
}
