package issuepickview

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/seanhalberthal/jiru/internal/theme"
	"github.com/seanhalberthal/jiru/internal/ui/issueview"
)

// Model is the issue picker overlay.
type Model struct {
	refs      []issueview.IssueRef
	title     string
	cursor    int
	offset    int // first visible item index for scrolling
	selected  *issueview.IssueRef
	dismissed bool
	width     int
	height    int
}

// New creates a new issue picker from the given refs.
func New(refs []issueview.IssueRef) Model {
	return Model{refs: refs, title: "Go to Issue"}
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
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
			m.dismissed = true
		case key.Matches(msg, key.NewBinding(key.WithKeys("k", "up"))):
			if m.cursor > 0 {
				m.cursor--
				m.ensureVisible()
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("j", "down"))):
			if m.cursor < len(m.refs)-1 {
				m.cursor++
				m.ensureVisible()
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
			if len(m.refs) > 0 {
				r := m.refs[m.cursor]
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

// View renders the issue picker overlay.
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

	vis := m.maxVisible()
	end := min(m.offset+vis, len(m.refs))
	visible := m.refs[m.offset:end]

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
	remaining := len(m.refs) - end
	if remaining > 0 {
		b.WriteString(theme.StyleSubtle.Render(fmt.Sprintf("  ↓ %d more", remaining)))
		b.WriteByte('\n')
	}

	help := theme.StyleHelpKey.Render("j/k") + " " + theme.StyleHelpDesc.Render("navigate") + "  " +
		theme.StyleHelpKey.Render("enter") + " " + theme.StyleHelpDesc.Render("select") + "  " +
		theme.StyleHelpKey.Render("esc") + " " + theme.StyleHelpDesc.Render("cancel")

	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		b.String(),
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
