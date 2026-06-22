package issuepickview

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/undont/jiru/internal/theme"
	"github.com/undont/jiru/internal/ui/issueview"
)

// Model is the issues overlay.
type Model struct {
	refs      []issueview.IssueRef
	filtered  []issueview.IssueRef
	title     string
	cursor    int
	offset    int // first visible item index for scrolling
	selected  *issueview.IssueRef
	dismissed bool
	filtering bool
	filter    textinput.Model
	width     int
	height    int
}

// New creates a new issues from the given refs.
func New(refs []issueview.IssueRef) Model {
	ti := textinput.New()
	ti.Placeholder = "Filter issues or pages"
	ti.CharLimit = 120

	m := Model{
		refs:     refs,
		filtered: refs,
		title:    "Go to Issue",
		filter:   ti,
	}
	return m
}

// SetTitle sets the picker overlay title.
func (m *Model) SetTitle(title string) {
	m.title = title
}

// Selected returns the chosen ref (once) and clears the sentinel.
func (m *Model) Selected() *issueview.IssueRef {
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

// SetSize updates the overlay dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.filter.Width = max(width/2, 20)
}

// maxVisible returns the maximum number of items that fit in the overlay.
// Accounts for border (2), padding (2), title + margin (2), help line (1), blank before help (1).
const boxChrome = 8 // border top/bottom + padding top/bottom + title + title margin + help + blank line

func (m Model) maxVisible() int {
	// Use at most 70% of terminal height for the box.
	maxBoxHeight := m.height * 7 / 10
	return max(maxBoxHeight-boxChrome, 1)
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.filtering {
			switch {
			case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
				m.filtering = false
				m.filter.Blur()
				if m.filter.Value() == "" {
					m.applyFilter()
				}
				return m, nil
			case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
				m.filtering = false
				m.filter.Blur()
				if len(m.filtered) > 0 {
					r := m.filtered[m.cursor]
					m.selected = &r
				}
				return m, nil
			}

			var cmd tea.Cmd
			m.filter, cmd = m.filter.Update(msg)
			m.applyFilter()
			return m, cmd
		}

		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("/"))):
			m.filtering = true
			m.filter.Focus()
			return m, textinput.Blink
		case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
			m.dismissed = true
		case key.Matches(msg, key.NewBinding(key.WithKeys("k", "up"))):
			if m.cursor > 0 {
				m.cursor--
				m.ensureVisible()
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("j", "down"))):
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
				m.ensureVisible()
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("d"))):
			step := max(m.maxVisible()/2, 1)
			m.cursor = min(m.cursor+step, len(m.filtered)-1)
			m.ensureVisible()
		case key.Matches(msg, key.NewBinding(key.WithKeys("u"))):
			step := max(m.maxVisible()/2, 1)
			m.cursor = max(m.cursor-step, 0)
			m.ensureVisible()
		case key.Matches(msg, key.NewBinding(key.WithKeys("g"))):
			m.cursor = 0
			m.ensureVisible()
		case key.Matches(msg, key.NewBinding(key.WithKeys("G"))):
			if len(m.filtered) > 0 {
				m.cursor = len(m.filtered) - 1
				m.ensureVisible()
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("enter", " "))):
			if len(m.filtered) > 0 {
				r := m.filtered[m.cursor]
				m.selected = &r
			}
		}
	}

	return m, nil
}

// ensureVisible adjusts the scroll offset so the cursor is within the visible window.
func (m *Model) ensureVisible() {
	vis := m.maxVisible()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+vis {
		m.offset = m.cursor - vis + 1
	}
}

func (m *Model) applyFilter() {
	query := strings.ToLower(strings.TrimSpace(m.filter.Value()))
	if query == "" {
		m.filtered = append([]issueview.IssueRef(nil), m.refs...)
	} else {
		m.filtered = m.filtered[:0]
		for _, ref := range m.refs {
			if issueRefMatches(ref, query) {
				m.filtered = append(m.filtered, ref)
			}
		}
	}
	if len(m.filtered) == 0 {
		m.cursor = 0
		m.offset = 0
		return
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
	m.ensureVisible()
}

// View renders the issues overlay.
func (m Model) View() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.ColourPrimary).
		MarginBottom(1)

	title := titleStyle.Render(m.title)

	if len(m.refs) == 0 {
		content := lipgloss.JoinVertical(lipgloss.Left,
			title,
			theme.StyleSubtle.Render("No referenced issues found."),
			"",
			theme.StyleHelpKey.Render("esc")+" "+theme.StyleHelpDesc.Render("close"),
		)
		return m.centreBox(content)
	}

	filterLine := ""
	if m.filtering || m.filter.Value() != "" {
		filterLine = theme.StyleSubtle.Render("Filter:") + " " + m.filter.View()
	}

	if len(m.filtered) == 0 {
		help := theme.StyleHelpKey.Render("/") + " " + theme.StyleHelpDesc.Render("filter") + "  " +
			theme.StyleHelpKey.Render("esc") + " " + theme.StyleHelpDesc.Render("cancel")
		content := lipgloss.JoinVertical(lipgloss.Left,
			title,
			filterLine,
			"",
			theme.StyleSubtle.Render("No matches."),
			"",
			help,
		)
		return m.centreBox(content)
	}

	vis := m.maxVisible()
	end := min(m.offset+vis, len(m.filtered))
	visible := m.filtered[m.offset:end]

	var b strings.Builder
	// Scroll indicator — top.
	if m.offset > 0 {
		b.WriteString(theme.StyleSubtle.Render(fmt.Sprintf("  ↑ %d more", m.offset)))
		b.WriteByte('\n')
	}

	// Max label width: box is ~60% of terminal, minus border (2) + padding (4) + cursor (2) + key + gap.
	// Truncate the plain-text label before styling to avoid slicing ANSI escape sequences.
	maxContentWidth := m.width*6/10 - 8

	lastGroup := ""
	for i, r := range visible {
		idx := m.offset + i

		// Group header — render when the group changes.
		if r.Group != "" && r.Group != lastGroup {
			if lastGroup != "" {
				b.WriteByte('\n') // Blank line between groups.
			}
			b.WriteString("  ")
			b.WriteString(theme.StyleTitle.Render(r.Group))
			b.WriteByte('\n')
			lastGroup = r.Group
		}

		cursor := "    "
		style := lipgloss.NewStyle()
		if idx == m.cursor {
			cursor = "  " + theme.StyleKey.Render("> ")
			style = style.Bold(true)
		}

		displayKey := r.Key
		if r.Display != "" {
			displayKey = r.Display
		}

		label := r.Label
		// key + "  " separator; estimate key width from plain text.
		keyWidth := len(displayKey) + 2
		maxLabel := maxContentWidth - keyWidth - 4 // 4 for cursor prefix
		if maxLabel > 0 {
			runes := []rune(label)
			if len(runes) > maxLabel {
				label = string(runes[:maxLabel-1]) + "…"
			}
		}

		var line string
		if label != "" {
			line = cursor + style.Render(fmt.Sprintf("%s  %s", theme.StyleKey.Render(displayKey), theme.StyleSubtle.Render(label)))
		} else {
			line = cursor + style.Render(theme.StyleKey.Render(displayKey))
		}

		b.WriteString(line)
		b.WriteByte('\n')
	}

	// Scroll indicator — bottom.
	remaining := len(m.filtered) - end
	if remaining > 0 {
		b.WriteString(theme.StyleSubtle.Render(fmt.Sprintf("  ↓ %d more", remaining)))
		b.WriteByte('\n')
	}

	help := theme.StyleHelpKey.Render("j/k") + " " + theme.StyleHelpDesc.Render("navigate") + "  " +
		theme.StyleHelpKey.Render("/") + " " + theme.StyleHelpDesc.Render("filter") + "  " +
		theme.StyleHelpKey.Render("enter/space") + " " + theme.StyleHelpDesc.Render("select") + "  " +
		theme.StyleHelpKey.Render("esc") + " " + theme.StyleHelpDesc.Render("cancel")

	parts := []string{title}
	if filterLine != "" {
		parts = append(parts, filterLine)
	}
	parts = append(parts, b.String(), help)

	content := lipgloss.JoinVertical(lipgloss.Left, parts...)

	return m.centreBox(content)
}

func issueRefMatches(ref issueview.IssueRef, query string) bool {
	for _, field := range []string{ref.Key, ref.Display, ref.Label, ref.Group} {
		if strings.Contains(strings.ToLower(field), query) {
			return true
		}
	}
	return false
}

func (m Model) centreBox(content string) string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColourPrimary).
		Padding(1, 2)

	box := boxStyle.Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
