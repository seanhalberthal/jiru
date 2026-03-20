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
	list      list.Model
	issues    []jira.Issue
	listStale bool // True when issues have been updated but not synced to the list widget (e.g. during filtering).
	width     int
	height    int
	selected  *jira.Issue // set when user presses enter.
	openKeys  key.Binding
	loading   bool // True while pages are still being fetched.
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
	l.Styles.StatusBar = l.Styles.StatusBar.Foreground(theme.ColourSubtle)
	l.Styles.StatusBarFilterCount = l.Styles.StatusBarFilterCount.Foreground(theme.ColourSubtle)
	l.Filter = issuedelegate.Filter

	// Override default pagination keys to remove f/d/b/u which conflict
	// with the app's global keybindings (filters, half-page scroll, etc.).
	l.KeyMap.NextPage.SetKeys("right", "l", "pgdown")
	l.KeyMap.PrevPage.SetKeys("left", "h", "pgup")

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

// UpdateIssueStatus updates the status of a specific issue in the list.
func (m Model) UpdateIssueStatus(key, newStatus string) Model {
	for i, iss := range m.issues {
		if iss.Key == key {
			m.issues[i].Status = newStatus
		}
	}
	items := m.list.Items()
	for i, item := range items {
		if it, ok := item.(issuedelegate.Item); ok && it.Key() == key {
			items[i] = it.WithStatus(newStatus)
		}
	}
	idx := m.list.Index()
	m.list.SetItems(items)
	m.list.Select(idx)
	return m
}

// AppendIssues adds more issues to the existing list (for progressive pagination).
// Deduplicates by issue key to handle overlapping API pages.
// When a filter is applied (input closed), items are buffered to avoid disrupting
// the user — they are flushed when the filter is cleared. When the filter input
// is actively open, items are refreshed immediately so newly loaded pages
// participate in the fuzzy match.
func (m Model) AppendIssues(issues []jira.Issue) Model {
	seen := make(map[string]bool, len(m.issues))
	for _, iss := range m.issues {
		seen[iss.Key] = true
	}
	for _, iss := range issues {
		if !seen[iss.Key] {
			m.issues = append(m.issues, iss)
			seen[iss.Key] = true
		}
	}

	if m.list.FilterState() == list.Filtering {
		// Refresh items and re-apply the current filter text so newly loaded
		// pages participate in the fuzzy match. SetFilterText ends in
		// FilterApplied state, so restore Filtering to keep the input open.
		filterText := m.list.FilterValue()
		cursorPos := m.list.FilterInput.Position()
		items := issuedelegate.ToItems(m.issues)
		m.list.SetItems(items)
		m.list.SetFilterText(filterText)
		m.list.SetFilterState(list.Filtering)
		m.list.FilterInput.SetCursor(cursorPos)
		m.list.Title = fmt.Sprintf("Issues (%d) loading...", len(m.issues))
		return m
	}

	if m.list.FilterState() != list.Unfiltered {
		// FilterApplied — don't call SetItems while results are locked in.
		m.listStale = true
		m.list.Title = fmt.Sprintf("Issues (%d) loading...", len(m.issues))
		return m
	}

	items := issuedelegate.ToItems(m.issues)
	m.list.SetItems(items)
	// AppendIssues is only called during progressive loading, so always show
	// the loading indicator. SetLoading(false) clears it when all pages arrive.
	m.list.Title = fmt.Sprintf("Issues (%d) loading...", len(m.issues))
	return m
}

// SetLoading updates the title to show pagination progress.
func (m Model) SetLoading(loading bool) Model {
	m.loading = loading
	if loading {
		m.list.Title = fmt.Sprintf("Issues (%d) loading...", len(m.issues))
	} else {
		m.list.Title = fmt.Sprintf("Issues (%d)", len(m.issues))
	}
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

		// d/u for half-page scrolling (forwarded as pgdown/pgup to the list).
		switch msg.String() {
		case "d":
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(tea.KeyMsg{Type: tea.KeyPgDown})
			return m, cmd
		case "u":
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(tea.KeyMsg{Type: tea.KeyPgUp})
			return m, cmd
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)

	// Flush buffered items once the user clears the filter.
	if m.listStale && m.list.FilterState() == list.Unfiltered {
		m.listStale = false
		items := issuedelegate.ToItems(m.issues)
		m.list.SetItems(items)
		m.list.Title = fmt.Sprintf("Issues (%d)", len(m.issues))
	}

	return m, cmd
}

// Filtering returns true when the list filter input is active.
func (m Model) Filtering() bool {
	return m.list.FilterState() == list.Filtering
}

// Filtered returns true when a filter has been applied (but input is not active).
func (m Model) Filtered() bool {
	return m.list.FilterState() == list.FilterApplied
}

// ResetFilter clears the applied filter.
func (m Model) ResetFilter() Model {
	m.list.ResetFilter()
	return m
}

// View renders the sprint view.
func (m Model) View() string {
	return m.list.View()
}
