package searchview

import (
	"fmt"
	"strings"

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
	dismissed    bool // true when user closes search without entering a query

	submitKeys key.Binding
	closeKeys  key.Binding
	openKeys   key.Binding

	// Completion popup state.
	completions []CompletionItem // Current matching items.
	compIndex   int              // Selected index in completions list (-1 = none).
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
		compIndex:  -1,
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
	m.completions = nil
	m.compIndex = -1
}

func (m *Model) Hide() {
	m.visible = false
	m.input.Blur()
	m.completions = nil
	m.compIndex = -1
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

// Dismissed returns true (once) when the user closed search without entering a query.
func (m *Model) Dismissed() bool {
	d := m.dismissed
	m.dismissed = false
	return d
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.visible {
		return m, nil
	}

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch m.state {
		case stateInput:
			// Esc: dismiss completions first, then dismiss search.
			if key.Matches(keyMsg, m.closeKeys) {
				if len(m.completions) > 0 {
					m.completions = nil
					m.compIndex = -1
					return m, nil
				}
				if m.input.Value() == "" {
					m.dismissed = true
				}
				m.Hide()
				return m, nil
			}

			// Tab: accept first/selected completion.
			if keyMsg.String() == "tab" {
				if len(m.completions) > 0 {
					if m.compIndex < 0 {
						m.compIndex = 0
					}
					m.acceptCompletion()
					return m, nil
				}
			}

			// Down / Up: navigate completions.
			if keyMsg.String() == "down" {
				if len(m.completions) > 0 {
					m.compIndex = (m.compIndex + 1) % len(m.completions)
					return m, nil
				}
			}
			if keyMsg.String() == "shift+tab" || keyMsg.String() == "up" {
				if len(m.completions) > 0 {
					m.compIndex--
					if m.compIndex < 0 {
						m.compIndex = len(m.completions) - 1
					}
					return m, nil
				}
			}

			// Enter: accept selected completion, or submit query.
			if key.Matches(keyMsg, m.submitKeys) {
				if m.compIndex >= 0 && m.compIndex < len(m.completions) {
					m.acceptCompletion()
					return m, nil
				}
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
		word, _ := currentWord(m.input.Value(), m.input.Position())
		m.completions = matchCompletions(word)
		m.compIndex = -1
	case stateResults:
		m.results, cmd = m.results.Update(msg)
	}
	return m, cmd
}

func (m *Model) acceptCompletion() {
	if m.compIndex < 0 || m.compIndex >= len(m.completions) {
		return
	}
	item := m.completions[m.compIndex]
	value := m.input.Value()
	cursor := m.input.Position()
	_, start := currentWord(value, cursor)

	insertText := item.String()
	newValue := value[:start] + insertText + value[cursor:]
	newCursor := start + len(insertText)

	m.input.SetValue(newValue)
	m.input.SetCursor(newCursor)

	m.completions = nil
	m.compIndex = -1
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
		hint := theme.StyleSubtle.Render("Enter to search \u00b7 Tab to complete \u00b7 Esc to close")
		content := fmt.Sprintf("%s\n\n%s\n\n%s", header, m.input.View(), hint)

		if len(m.completions) > 0 {
			popup := m.renderCompletions()
			content = fmt.Sprintf("%s\n%s", content, popup)
		}

		return border.Width(m.width - 4).Render(content)
	case stateResults:
		return m.results.View()
	default:
		return ""
	}
}

func (m Model) renderCompletions() string {
	normalStyle := lipgloss.NewStyle().
		PaddingLeft(1).
		PaddingRight(1)

	selectedStyle := lipgloss.NewStyle().
		PaddingLeft(1).
		PaddingRight(1).
		Background(theme.ColourPrimary).
		Foreground(lipgloss.Color("#000000"))

	kindStyle := lipgloss.NewStyle().
		Foreground(theme.ColourSubtle).
		PaddingRight(1)

	detailStyle := lipgloss.NewStyle().
		Foreground(theme.ColourSubtle)

	var rows []string
	for i, item := range m.completions {
		kind := kindStyle.Render(item.Kind.KindLabel())
		detail := detailStyle.Render(item.Detail)
		line := fmt.Sprintf("%s %-20s %s", kind, item.Label, detail)

		if i == m.compIndex {
			rows = append(rows, selectedStyle.Render(line))
		} else {
			rows = append(rows, normalStyle.Render(line))
		}
	}

	popup := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColourSubtle).
		Render(strings.Join(rows, "\n"))

	return popup
}

type issueItem struct {
	issue jira.Issue
}

func (i issueItem) Title() string { return fmt.Sprintf("%s  %s", i.issue.Key, i.issue.Summary) }
func (i issueItem) Description() string {
	return fmt.Sprintf("%s \u00b7 %s \u00b7 %s", i.issue.IssueType, i.issue.Status, i.issue.Assignee)
}
func (i issueItem) FilterValue() string { return i.issue.Key + " " + i.issue.Summary }
