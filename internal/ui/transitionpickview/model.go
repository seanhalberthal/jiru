package transitionpickview

import (
	"fmt"
	"sort"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/seanhalberthal/jiru/internal/jira"
	"github.com/seanhalberthal/jiru/internal/theme"
)

// Model is the transition picker overlay.
type Model struct {
	issueKey    string
	transitions []jira.Transition
	cursor      int
	loading     bool
	selected    *jira.Transition
	dismissed   bool
	width       int
	height      int
}

// New creates a new transition picker for the given issue key.
func New(issueKey string) Model {
	return Model{
		issueKey: issueKey,
		loading:  true,
	}
}

// SetTransitions populates the picker with available transitions.
// Forward transitions (in-progress, done) appear first; regressive
// and cancelled transitions are sorted to the bottom.
func (m *Model) SetTransitions(transitions []jira.Transition) {
	sort.SliceStable(transitions, func(i, j int) bool {
		return transitionOrder(transitions[i].ToStatus) < transitionOrder(transitions[j].ToStatus)
	})
	m.transitions = transitions
	m.loading = false
	m.cursor = 0
}

// transitionOrder returns a sort key that groups forward transitions first
// (in-progress → done) and regressive/cancelled transitions last.
func transitionOrder(toStatus string) int {
	switch theme.StatusCategory(toStatus) {
	case 1: // in-progress
		return 0
	case 2: // done
		return 1
	case 0: // todo (regressive)
		return 2
	case 3: // cancelled
		return 3
	default:
		return 2
	}
}

// Selected returns the chosen transition, or nil.
// Selected returns the chosen transition (once) and clears the sentinel.
func (m *Model) Selected() *jira.Transition {
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

// InputActive returns true while the picker is active (suppresses global keys).
func (m Model) InputActive() bool {
	return true
}

// IssueKey returns the issue key this picker is for.
func (m Model) IssueKey() string {
	return m.issueKey
}

// SetSize updates the overlay dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if m.loading {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
			m.dismissed = true
		case key.Matches(msg, key.NewBinding(key.WithKeys("k", "up"))):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("j", "down"))):
			if m.cursor < len(m.transitions)-1 {
				m.cursor++
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("enter", " "))):
			if len(m.transitions) > 0 {
				t := m.transitions[m.cursor]
				m.selected = &t
			}
		}
	}

	return m, nil
}

// View renders the transition picker overlay.
func (m Model) View() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.ColourPrimary).
		MarginBottom(1)

	title := titleStyle.Render(fmt.Sprintf("Move %s", m.issueKey))

	if m.loading {
		content := lipgloss.JoinVertical(lipgloss.Left,
			title,
			theme.StyleSubtle.Render("Loading transitions..."),
		)
		return m.centreBox(content)
	}

	if len(m.transitions) == 0 {
		content := lipgloss.JoinVertical(lipgloss.Left,
			title,
			theme.StyleSubtle.Render("No transitions available."),
			"",
			theme.StyleHelpKey.Render("esc")+" "+theme.StyleHelpDesc.Render("close"),
		)
		return m.centreBox(content)
	}

	var items string
	for i, t := range m.transitions {
		cursor := "  "
		style := lipgloss.NewStyle()
		if i == m.cursor {
			cursor = theme.StyleKey.Render("> ")
			style = style.Bold(true)
		}
		items += cursor + style.Render(t.Name) + "\n"
	}

	help := theme.StyleHelpKey.Render("j/k") + " " + theme.StyleHelpDesc.Render("navigate") + "  " +
		theme.StyleHelpKey.Render("enter/space") + " " + theme.StyleHelpDesc.Render("select") + "  " +
		theme.StyleHelpKey.Render("esc") + " " + theme.StyleHelpDesc.Render("cancel")

	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
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
