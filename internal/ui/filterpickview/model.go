package filterpickview

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/seanhalberthal/jiru/internal/filters"
	"github.com/seanhalberthal/jiru/internal/jira"
	"github.com/seanhalberthal/jiru/internal/jql"
	"github.com/seanhalberthal/jiru/internal/theme"
)

type state int

const (
	stateList state = iota
	stateEditName
	stateEditQuery
	stateConfirmDelete
)

// Model is the saved-filter manager overlay.
type Model struct {
	filters   []jira.SavedFilter
	cursor    int
	state     state
	width     int
	height    int
	nameInput textinput.Model
	jqlInput  textarea.Model

	// editID is non-empty when editing an existing filter.
	editID string

	// JQL completion state (active during stateEditQuery).
	values      *jql.ValueProvider
	completions []jql.Item
	compIndex   int

	// Sentinels — read-once by the parent.
	applied            *jira.SavedFilter
	saveRequested      *saveRequest
	deleteRequested    string
	favouriteID        string
	duplicateRequested string // ID of filter to duplicate.
	copyJQLRequested   string // JQL string to copy to clipboard.
	dismissed          bool
}

type saveRequest struct {
	id   string // Empty for new filters.
	name string
	jql  string
}

// New creates an empty filter manager.
func New() Model {
	ni := textinput.New()
	ni.Placeholder = "Filter name"
	ni.CharLimit = filters.MaxFilterNameLen

	ji := textarea.New()
	ji.Placeholder = "JQL query"
	ji.CharLimit = 500
	ji.ShowLineNumbers = false
	ji.SetHeight(3)
	ji.Blur() // Must not start focused — prevents stale cursor blink state.

	return Model{
		nameInput: ni,
		jqlInput:  ji,
		compIndex: -1,
	}
}

// SetFilters replaces the displayed filter list.
func (m *Model) SetFilters(filters []jira.SavedFilter) {
	m.filters = filters
	if m.cursor >= len(m.filters) {
		m.cursor = max(0, len(m.filters)-1)
	}
}

// SetValues installs the JQL value provider for autocompletion.
func (m *Model) SetValues(vp *jql.ValueProvider) {
	m.values = vp
}

// StartAdd pre-fills the JQL input for saving from search results.
// The view opens directly to the name-entry step.
func (m *Model) StartAdd(query string) {
	m.editID = ""
	m.nameInput.SetValue("")
	m.jqlInput.SetValue(query)
	m.state = stateEditName
	m.nameInput.Focus()
}

// SetSize updates the overlay dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.nameInput.Width = width*3/4 - 6
	m.jqlInput.SetWidth(width*3/4 - 6)
}

// InputActive returns true whenever a text input is focused (suppresses global keys).
func (m Model) InputActive() bool {
	return m.state == stateEditName || m.state == stateEditQuery || m.state == stateConfirmDelete
}

// EditingName returns true when the name input step is active.
func (m Model) EditingName() bool { return m.state == stateEditName }

// EditingQuery returns true when the JQL input step is active.
func (m Model) EditingQuery() bool { return m.state == stateEditQuery }

// ConfirmingDelete returns true when the delete confirmation is active.
func (m Model) ConfirmingDelete() bool { return m.state == stateConfirmDelete }

// Applied returns the filter the user chose to apply (once).
func (m *Model) Applied() *jira.SavedFilter {
	f := m.applied
	m.applied = nil
	return f
}

// SaveRequested returns a pending save (add or edit) and clears it.
func (m *Model) SaveRequested() (id, name, query string, ok bool) {
	if m.saveRequested == nil {
		return "", "", "", false
	}
	r := m.saveRequested
	m.saveRequested = nil
	return r.id, r.name, r.jql, true
}

// DeleteRequested returns the ID of a filter to delete (once).
func (m *Model) DeleteRequested() string {
	id := m.deleteRequested
	m.deleteRequested = ""
	return id
}

// FavouriteRequested returns the ID of a filter whose favourite flag should be toggled (once).
func (m *Model) FavouriteRequested() string {
	id := m.favouriteID
	m.favouriteID = ""
	return id
}

// DuplicateRequested returns the ID of a filter to duplicate (once).
func (m *Model) DuplicateRequested() string {
	id := m.duplicateRequested
	m.duplicateRequested = ""
	return id
}

// CopyJQLRequested returns the JQL to copy to clipboard (once).
func (m *Model) CopyJQLRequested() string {
	jql := m.copyJQLRequested
	m.copyJQLRequested = ""
	return jql
}

// Dismissed returns true (once) when the user closed the view without acting.
func (m *Model) Dismissed() bool {
	d := m.dismissed
	m.dismissed = false
	return d
}

// Reset restores the model to a clean list state.
// Called by the parent when entering the filter view to prevent stale state.
func (m *Model) Reset() {
	m.state = stateList
	m.editID = ""
	m.nameInput.SetValue("")
	m.nameInput.Blur()
	m.jqlInput.SetValue("")
	m.jqlInput.Blur()
	m.completions = nil
	m.compIndex = -1
	m.applied = nil
	m.saveRequested = nil
	m.deleteRequested = ""
	m.favouriteID = ""
	m.duplicateRequested = ""
	m.copyJQLRequested = ""
	m.dismissed = false
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	keyMsg, isKey := msg.(tea.KeyMsg)
	if !isKey {
		// Forward to focused inputs.
		var cmd tea.Cmd
		switch m.state {
		case stateEditName:
			m.nameInput, cmd = m.nameInput.Update(msg)
		case stateEditQuery:
			m.jqlInput, cmd = m.jqlInput.Update(msg)
		}
		return m, cmd
	}

	switch m.state {
	case stateList:
		return m.updateList(keyMsg)
	case stateEditName:
		return m.updateEditName(keyMsg)
	case stateEditQuery:
		return m.updateEditQuery(keyMsg)
	case stateConfirmDelete:
		return m.updateConfirmDelete(keyMsg)
	}
	return m, nil
}

func (m Model) updateList(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.dismissed = true
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "j", "down":
		if m.cursor < len(m.filters)-1 {
			m.cursor++
		}
	case "enter":
		if len(m.filters) > 0 {
			f := m.filters[m.cursor]
			m.applied = &f
		}
	case "n":
		m.editID = ""
		m.nameInput.SetValue("")
		m.jqlInput.SetValue("")
		m.state = stateEditName
		m.nameInput.Focus()
		return m, textinput.Blink
	case "e":
		if len(m.filters) > 0 {
			f := m.filters[m.cursor]
			m.editID = f.ID
			m.nameInput.SetValue(f.Name)
			m.jqlInput.SetValue(f.JQL)
			m.state = stateEditName
			m.nameInput.Focus()
			return m, textinput.Blink
		}
	case "f":
		if len(m.filters) > 0 {
			m.favouriteID = m.filters[m.cursor].ID
		}
	case "d":
		if len(m.filters) > 0 {
			m.duplicateRequested = m.filters[m.cursor].ID
		}
	case "x":
		if len(m.filters) > 0 {
			m.copyJQLRequested = m.filters[m.cursor].JQL
		}
	case "D":
		if len(m.filters) > 0 {
			m.state = stateConfirmDelete
		}
	}
	return m, nil
}

func (m Model) updateEditName(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		if m.editID == "" && m.jqlInput.Value() != "" {
			// Came from search save — dismiss back to caller.
			m.dismissed = true
		} else {
			m.state = stateList
		}
		m.nameInput.Blur()
	case "enter":
		if strings.TrimSpace(m.nameInput.Value()) != "" {
			m.state = stateEditQuery
			m.nameInput.Blur()
			cmd := m.jqlInput.Focus()
			// Compute initial completions for the JQL input.
			m.recalcCompletions()
			return m, cmd
		}
	default:
		var cmd tea.Cmd
		m.nameInput, cmd = m.nameInput.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) updateEditQuery(msg tea.KeyMsg) (Model, tea.Cmd) {
	// Completion navigation — handled before the default text input update.
	if len(m.completions) > 0 {
		switch msg.String() {
		case "tab":
			if m.compIndex < 0 {
				m.compIndex = 0
			}
			m.acceptCompletion()
			return m, nil
		case "up":
			m.compIndex--
			if m.compIndex < 0 {
				m.compIndex = len(m.completions) - 1
			}
			return m, nil
		case "down":
			m.compIndex = (m.compIndex + 1) % len(m.completions)
			return m, nil
		}
	}

	switch msg.String() {
	case "esc":
		if len(m.completions) > 0 {
			m.completions = nil
			m.compIndex = -1
			return m, nil
		}
		m.state = stateEditName
		m.jqlInput.Blur()
		m.nameInput.Focus()
		m.completions = nil
		m.compIndex = -1
		return m, textinput.Blink
	case "enter":
		if m.compIndex >= 0 && m.compIndex < len(m.completions) {
			m.acceptCompletion()
			return m, nil
		}
		if strings.TrimSpace(m.jqlInput.Value()) != "" {
			m.saveRequested = &saveRequest{
				id:   m.editID,
				name: strings.TrimSpace(m.nameInput.Value()),
				jql:  strings.TrimSpace(m.jqlInput.Value()),
			}
			m.state = stateList
			m.nameInput.Blur()
			m.jqlInput.Blur()
			m.completions = nil
			m.compIndex = -1
		}
	default:
		var cmd tea.Cmd
		m.jqlInput, cmd = m.jqlInput.Update(msg)
		m.recalcCompletions()
		return m, cmd
	}
	return m, nil
}

func (m *Model) recalcCompletions() {
	ctx := jql.Parse(m.jqlInput.Value(), m.jqlInput.LineInfo().CharOffset)
	m.completions = jql.Match(ctx, m.values)
	m.compIndex = -1
}

func (m *Model) acceptCompletion() {
	if m.compIndex < 0 || m.compIndex >= len(m.completions) {
		return
	}
	newValue, newCursor := jql.Accept(m.jqlInput.Value(), m.jqlInput.LineInfo().CharOffset, m.completions[m.compIndex])
	m.jqlInput.SetValue(newValue)
	m.jqlInput.SetCursor(newCursor)
	m.completions = nil
	m.compIndex = -1
}

func (m Model) updateConfirmDelete(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "y", "enter":
		m.deleteRequested = m.filters[m.cursor].ID
		m.state = stateList
		if m.cursor > 0 {
			m.cursor--
		}
	case "n", "esc":
		m.state = stateList
	}
	return m, nil
}

func (m Model) View() string {
	switch m.state {
	case stateEditName:
		return m.renderEditBox("Filter Name", m.nameInput.View(), "",
			"Enter a name for this filter",
			theme.StyleHelpKey.Render("enter")+" "+theme.StyleHelpDesc.Render("next")+"  "+
				theme.StyleHelpKey.Render("esc")+" "+theme.StyleHelpDesc.Render("back"),
		)
	case stateEditQuery:
		var popup string
		if len(m.completions) > 0 {
			popup = jql.RenderPopup(m.completions, m.compIndex)
		}
		return m.renderEditBox("JQL Query", m.jqlInput.View(), popup,
			"Enter or edit the JQL query",
			theme.StyleHelpKey.Render("enter")+" "+theme.StyleHelpDesc.Render("save")+"  "+
				theme.StyleHelpKey.Render("↑↓")+" "+theme.StyleHelpDesc.Render("browse")+"  "+
				theme.StyleHelpKey.Render("tab")+" "+theme.StyleHelpDesc.Render("accept")+"  "+
				theme.StyleHelpKey.Render("esc")+" "+theme.StyleHelpDesc.Render("back"),
		)
	case stateConfirmDelete:
		var name string
		if len(m.filters) > 0 {
			name = m.filters[m.cursor].Name
		}
		content := lipgloss.JoinVertical(lipgloss.Left,
			theme.StyleTitle.Render("Delete Filter"),
			"",
			fmt.Sprintf("Delete %q?", name),
			"",
			theme.StyleHelpKey.Render("y/enter")+" "+theme.StyleHelpDesc.Render("confirm")+"  "+
				theme.StyleHelpKey.Render("n/esc")+" "+theme.StyleHelpDesc.Render("cancel"),
		)
		return m.centreBox(content)
	default:
		return m.renderList()
	}
}

func (m Model) renderList() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(theme.ColourPrimary).MarginBottom(1)
	title := titleStyle.Render("Saved Filters")

	if len(m.filters) == 0 {
		content := lipgloss.JoinVertical(lipgloss.Left,
			title,
			theme.StyleSubtle.Render("No saved filters yet."),
			"",
			theme.StyleHelpKey.Render("n")+" "+theme.StyleHelpDesc.Render("new filter")+"  "+
				theme.StyleHelpKey.Render("esc")+" "+theme.StyleHelpDesc.Render("close"),
		)
		return m.centreBox(content)
	}

	var rows []string
	for i, f := range m.filters {
		cursor := "  "
		nameStyle := lipgloss.NewStyle()
		if i == m.cursor {
			cursor = theme.StyleKey.Render("> ")
			nameStyle = nameStyle.Bold(true)
		}
		star := " "
		if f.Favourite {
			star = theme.StyleHelpKey.Render("★")
		}
		jqlPreview := f.JQL
		// Box width 3/4, minus border(2) + padding(4) + cursor(2) + star(2) + name(24) + gap(2) = 36.
		maxJQL := m.width*3/4 - 36
		if maxJQL < 10 {
			maxJQL = 10
		}
		if len(jqlPreview) > maxJQL {
			jqlPreview = jqlPreview[:maxJQL] + "…"
		}
		paddedName := fmt.Sprintf("%-24s", f.Name)
		row := fmt.Sprintf("%s%s %s  %s",
			cursor, star,
			nameStyle.Render(paddedName),
			theme.StyleSubtle.Render(jqlPreview),
		)
		rows = append(rows, row)
	}

	help := theme.StyleHelpKey.Render("enter") + " " + theme.StyleHelpDesc.Render("apply") + "  " +
		theme.StyleHelpKey.Render("n") + " " + theme.StyleHelpDesc.Render("new") + "  " +
		theme.StyleHelpKey.Render("e") + " " + theme.StyleHelpDesc.Render("edit") + "  " +
		theme.StyleHelpKey.Render("d") + " " + theme.StyleHelpDesc.Render("duplicate") + "  " +
		theme.StyleHelpKey.Render("f") + " " + theme.StyleHelpDesc.Render("favourite") + "  " +
		theme.StyleHelpKey.Render("D") + " " + theme.StyleHelpDesc.Render("delete") + "  " +
		theme.StyleHelpKey.Render("esc") + " " + theme.StyleHelpDesc.Render("close")

	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		strings.Join(rows, "\n"),
		"",
		help,
	)
	return m.centreBox(content)
}

func (m Model) renderEditBox(title, input, popup, hint, help string) string {
	parts := []string{
		theme.StyleTitle.Render(title),
		"",
		theme.StyleSubtle.Render(hint),
		"",
		input,
	}
	if popup != "" {
		parts = append(parts, popup)
	}
	parts = append(parts, "", help)
	content := lipgloss.JoinVertical(lipgloss.Left, parts...)

	// Use a fixed-height box for edit states so the centered position stays
	// stable regardless of content changes (popup appearing, text wrapping).
	boxHeight := max(m.height*4/10, 12)
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColourPrimary).
		Padding(1, 2).
		Width(m.width * 3 / 4).
		Height(boxHeight)

	box := boxStyle.Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m Model) centreBox(content string) string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColourPrimary).
		Padding(1, 2).
		Width(m.width * 3 / 4)

	box := boxStyle.Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
