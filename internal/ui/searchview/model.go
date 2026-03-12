package searchview

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/seanhalberthal/jiratui/internal/jira"
	"github.com/seanhalberthal/jiratui/internal/theme"
)

type state int

const (
	stateInput state = iota
	stateResults
)

type Model struct {
	input        textinput.Model
	results      list.Model
	state        state
	width        int
	height       int
	selected     *jira.Issue
	query        string
	visible      bool
	pendingQuery string

	submitKeys key.Binding
	closeKeys  key.Binding
	openKeys   key.Binding
}

func New() Model {
	ti := textinput.New()
	ti.Placeholder = "Enter JQL query (e.g. assignee = currentUser() AND status != Done)"
	ti.CharLimit = 500
	ti.Width = 80

	l := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Search Results"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)

	return Model{
		input:      ti,
		results:    l,
		state:      stateInput,
		submitKeys: key.NewBinding(key.WithKeys("enter")),
		closeKeys:  key.NewBinding(key.WithKeys("esc")),
		openKeys:   key.NewBinding(key.WithKeys("enter", "l")),
	}
}

func (m *Model) Show() {
	m.visible = true
	m.state = stateInput
	m.input.SetValue("")
	m.input.Focus()
}

func (m *Model) Hide() {
	m.visible = false
	m.input.Blur()
}

func (m Model) Visible() bool {
	return m.visible
}

func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.input.Width = width - 4
	m.results.SetSize(width, height-4)
}

func (m *Model) SetResults(issues []jira.Issue, query string) {
	m.query = query
	items := make([]list.Item, len(issues))
	for i, iss := range issues {
		items[i] = issueItem{issue: iss}
	}
	m.results.SetItems(items)
	m.results.Title = fmt.Sprintf("Results for: %s (%d)", query, len(issues))
	m.state = stateResults
}

func (m *Model) SelectedIssue() *jira.Issue {
	iss := m.selected
	m.selected = nil
	return iss
}

func (m *Model) SubmittedQuery() string {
	q := m.pendingQuery
	m.pendingQuery = ""
	return q
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.visible {
		return m, nil
	}

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch m.state {
		case stateInput:
			if key.Matches(keyMsg, m.closeKeys) {
				m.Hide()
				return m, nil
			}
			if key.Matches(keyMsg, m.submitKeys) {
				q := m.input.Value()
				if q != "" {
					m.pendingQuery = q
					m.query = q
				}
				return m, nil
			}
		case stateResults:
			if key.Matches(keyMsg, m.closeKeys) {
				m.state = stateInput
				m.input.Focus()
				return m, nil
			}
			if key.Matches(keyMsg, m.openKeys) {
				if item, ok := m.results.SelectedItem().(issueItem); ok {
					iss := item.issue
					m.selected = &iss
					return m, nil
				}
			}
		}
	}

	var cmd tea.Cmd
	switch m.state {
	case stateInput:
		m.input, cmd = m.input.Update(msg)
	case stateResults:
		m.results, cmd = m.results.Update(msg)
	}
	return m, cmd
}

func (m Model) View() string {
	if !m.visible {
		return ""
	}

	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColourPrimary).
		Padding(0, 1)

	switch m.state {
	case stateInput:
		header := theme.StyleTitle.Render("Search Issues (JQL)")
		hint := theme.StyleSubtle.Render("Enter to search · Esc to close")
		content := fmt.Sprintf("%s\n\n%s\n\n%s", header, m.input.View(), hint)
		return border.Width(m.width - 4).Render(content)
	case stateResults:
		return m.results.View()
	default:
		return ""
	}
}

type issueItem struct {
	issue jira.Issue
}

func (i issueItem) Title() string { return fmt.Sprintf("%s  %s", i.issue.Key, i.issue.Summary) }
func (i issueItem) Description() string {
	return fmt.Sprintf("%s · %s · %s", i.issue.IssueType, i.issue.Status, i.issue.Assignee)
}
func (i issueItem) FilterValue() string { return i.issue.Key + " " + i.issue.Summary }
