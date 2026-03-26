package searchview

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/seanhalberthal/jiru/internal/jira"
	"github.com/seanhalberthal/jiru/internal/jql"
	"github.com/seanhalberthal/jiru/internal/theme"
	"github.com/seanhalberthal/jiru/internal/ui/issuedelegate"
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
	completions []jql.Item // Current matching items.
	compIndex   int        // Selected index in completions list (-1 = none).

	// Dynamic completion values from Jira instance.
	values *jql.ValueProvider
	// User search debounce state.
	userPrefix  string // Last prefix we searched for.
	userPending bool   // Whether a user search is in flight.

	pendingSave string // Non-empty when user pressed 's' on results to save current query.
	filterName  string // When results are from a saved filter, show the filter name in the title.
}

func New() Model {
	ti := textinput.New()
	ti.Placeholder = "Enter JQL query (e.g. assignee = currentUser() AND status != Done)"
	ti.CharLimit = 500
	ti.Width = 80

	l := list.New(nil, issuedelegate.Delegate{}, 0, 0)
	l.Title = "Search Results"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)
	l.Styles.Title = theme.StyleTitle

	// Override default pagination keys to remove f/d/b/u which conflict
	// with the app's global keybindings (filters, half-page scroll, etc.).
	l.KeyMap.NextPage.SetKeys("right", "l", "pgdown")
	l.KeyMap.PrevPage.SetKeys("left", "h", "pgup")

	return Model{
		input:      ti,
		results:    l,
		state:      stateInput,
		compIndex:  -1,
		submitKeys: key.NewBinding(key.WithKeys("enter")),
		closeKeys:  key.NewBinding(key.WithKeys("esc")),
		openKeys:   key.NewBinding(key.WithKeys("enter", " ")),
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

// Reshow restores visibility after returning from an issue detail view,
// preserving the current query and results (unlike Show which resets).
func (m *Model) Reshow() {
	m.visible = true
	m.dismissed = false
	m.selected = nil
}

func (m Model) Visible() bool {
	return m.visible
}

// ShowingResults returns true when the search view is displaying results
// rather than the JQL input.
func (m Model) ShowingResults() bool {
	return m.state == stateResults
}

func (m Model) SetSize(width, height int) Model {
	m.width = width
	m.height = height
	m.input.Width = width - 6 // subtract 2 for the "> " prompt so text fits within the border
	m.results.SetSize(width, height)
	// Re-truncate title if query is set.
	if m.query != "" {
		m.updateTitle(len(m.results.Items()))
	}
	return m
}

func (m *Model) SetResults(issues []jira.Issue, query string) {
	m.query = query
	m.results.SetItems(issuedelegate.ToItems(issues))
	m.updateTitle(len(issues))
	m.state = stateResults
}

// SetFilterName sets the name of the saved filter whose results are being shown.
// Pass "" to clear it (e.g., for ad-hoc JQL searches).
func (m *Model) SetFilterName(name string) {
	m.filterName = name
}

// FilterName returns the saved filter name, or "" for ad-hoc searches.
func (m *Model) FilterName() string {
	return m.filterName
}

// updateTitle sets the results title, truncating the query if needed.
func (m *Model) updateTitle(count int) {
	suffix := fmt.Sprintf(" (%d)", count)
	var prefix, display string
	if m.filterName != "" {
		prefix = "Filter: "
		display = m.filterName
	} else {
		prefix = "Results for: "
		display = m.query
	}
	maxLen := m.width - len(prefix) - len(suffix) - 4 // padding
	if maxLen > 3 && len(display) > maxLen {
		display = display[:maxLen-3] + "..."
	}
	m.results.Title = fmt.Sprintf("%s%s%s", prefix, display, suffix)
}

// UpdateIssueStatus updates the status of an issue in the search results.
// Used to keep results in sync after a status transition.
func (m *Model) UpdateIssueStatus(key, newStatus string) {
	items := m.results.Items()
	for i, item := range items {
		if it, ok := item.(issuedelegate.Item); ok && it.Key() == key {
			items[i] = it.WithStatus(newStatus)
		}
	}
	idx := m.results.Index()
	m.results.SetItems(items)
	m.results.Select(idx)
}

// AppendResults adds more search results to the existing list.
// Preserves the user's cursor position so new pages don't disrupt browsing.
func (m *Model) AppendResults(issues []jira.Issue) {
	existingItems := m.results.Items()
	idx := m.results.Index() // Save cursor position before SetItems resets it.
	newItems := issuedelegate.ToItems(issues)
	allItems := append(existingItems, newItems...)
	m.results.SetItems(allItems)
	m.results.Select(idx) // Restore cursor position — new items are appended at the end.
	m.updateTitle(len(allItems))
}

// HighlightedIssue returns the currently highlighted issue without consuming it.
// Returns false when in input state or when the results list is empty.
func (m Model) HighlightedIssue() (jira.Issue, bool) {
	if m.state != stateResults {
		return jira.Issue{}, false
	}
	if item, ok := m.results.SelectedItem().(issuedelegate.Item); ok {
		return item.Issue, true
	}
	return jira.Issue{}, false
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

// InputActive returns true when the search view has a text input focused
// (JQL input or results list filtering).
func (m Model) InputActive() bool {
	if m.state == stateInput {
		return true
	}
	if m.state == stateResults && m.results.FilterState() == list.Filtering {
		return true
	}
	return false
}

// ResultsFiltered returns true when the results list has a filter applied (but input is not active).
func (m Model) ResultsFiltered() bool {
	return m.state == stateResults && m.results.FilterState() == list.FilterApplied
}

// ResetResultsFilter clears the applied filter on the results list.
func (m *Model) ResetResultsFilter() {
	m.results.ResetFilter()
}

// BackToInput returns from results to the JQL input.
func (m *Model) BackToInput() {
	if m.state == stateResults {
		m.state = stateInput
		m.input.Focus()
	}
}

// SaveFilter returns the JQL query the user wants to save (once), or empty string.
func (m *Model) SaveFilter() string {
	q := m.pendingSave
	m.pendingSave = ""
	return q
}

// Dismissed returns true (once) when the user closed search without entering a query.
func (m *Model) Dismissed() bool {
	d := m.dismissed
	m.dismissed = false
	return d
}

// SetMetadata populates the dynamic completion values from fetched Jira metadata.
func (m Model) SetMetadata(meta *jira.JQLMetadata) Model {
	if meta == nil {
		return m
	}
	m.values = &jql.ValueProvider{
		Statuses:    meta.Statuses,
		IssueTypes:  meta.IssueTypes,
		Priorities:  meta.Priorities,
		Resolutions: meta.Resolutions,
		Projects:    meta.Projects,
		Labels:      meta.Labels,
		Components:  meta.Components,
		Versions:    meta.Versions,
		Sprints:     meta.Sprints,
	}
	return m
}

// SetUserResults updates the assignee/reporter completions from a user search.
func (m Model) SetUserResults(names []string) Model {
	if m.values == nil {
		m.values = &jql.ValueProvider{}
	}
	m.values.Users = names
	m.userPending = false
	return m
}

// NeedsUserSearch returns a prefix if the completion context requires
// a user search that hasn't been done yet. Returns "" if no search needed.
func (m *Model) NeedsUserSearch() string {
	ctx := jql.Parse(m.input.Value(), m.input.Position())
	if ctx.Context != jql.CtxValue {
		return ""
	}
	if ctx.Field != "assignee" && ctx.Field != "reporter" {
		return ""
	}
	if len(ctx.Prefix) < 2 {
		return ""
	}
	if ctx.Prefix == m.userPrefix || m.userPending {
		return ""
	}
	m.userPrefix = ctx.Prefix
	m.userPending = true
	return ctx.Prefix
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

			// Down: cycle forward through completions.
			if keyMsg.String() == "down" {
				if len(m.completions) > 0 {
					m.compIndex = (m.compIndex + 1) % len(m.completions)
					return m, nil
				}
			}
			// Up: cycle backward through completions.
			if keyMsg.String() == "up" {
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
					m.filterName = "" // Clear filter context — this is an ad-hoc search.
					m.pendingQuery = q
					m.query = q
				}
				return m, nil
			}

		case stateResults:
			if key.Matches(keyMsg, m.closeKeys) {
				// If list filter is active, let bubbles/list handle esc to clear it
				// rather than jumping back to JQL input.
				if m.results.FilterState() != list.Unfiltered {
					break
				}
				m.state = stateInput
				m.input.Focus()
				return m, nil
			}
			if keyMsg.String() == "s" && m.query != "" && m.filterName == "" {
				m.pendingSave = m.query
				return m, nil
			}
			if keyMsg.String() == "r" && m.query != "" {
				m.pendingQuery = m.query
				return m, nil
			}
			if m.results.FilterState() != list.Filtering && key.Matches(keyMsg, m.openKeys) {
				if item, ok := m.results.SelectedItem().(issuedelegate.Item); ok {
					iss := item.Issue
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
		// Only recalculate completions on key events — non-key messages
		// (e.g. cursor blink) must not reset the selected completion index.
		if _, ok := msg.(tea.KeyMsg); ok {
			ctx := jql.Parse(m.input.Value(), m.input.Position())
			m.completions = jql.Match(ctx, m.values)
			m.compIndex = -1
		}
	case stateResults:
		m.results, cmd = m.results.Update(msg)
	}
	return m, cmd
}

func (m *Model) acceptCompletion() {
	if m.compIndex < 0 || m.compIndex >= len(m.completions) {
		return
	}
	newValue, newCursor := jql.Accept(m.input.Value(), m.input.Position(), m.completions[m.compIndex])
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
		hint := theme.StyleSubtle.Render("Enter to search \u00b7 \u2191\u2193 browse \u00b7 Tab to accept \u00b7 Esc to close")
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
	return jql.RenderPopup(m.completions, m.compIndex)
}
