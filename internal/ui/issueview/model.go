package issueview

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/seanhalberthal/jiru/internal/jira"
	"github.com/seanhalberthal/jiru/internal/markup"
	"github.com/seanhalberthal/jiru/internal/theme"
)

// Model is the issue detail view.
type Model struct {
	viewport viewport.Model
	issue    *jira.Issue
	issueURL string
	width    int
	height   int
	openURL  bool // set when user presses 'o'.
	openKeys key.Binding
}

// New creates a new issue view model.
func New() Model {
	vp := viewport.New(0, 0)
	return Model{
		viewport: vp,
		openKeys: key.NewBinding(key.WithKeys("o")),
	}
}

// SetSize updates the dimensions.
func (m Model) SetSize(width, height int) Model {
	m.width = width
	m.height = height
	headerHeight := 3 // Title bar.
	m.viewport.Width = width
	m.viewport.Height = height - headerHeight
	if m.issue != nil {
		m.viewport.SetContent(m.renderContent())
	}
	return m
}

// SetIssue sets the issue to display and renders content.
func (m Model) SetIssue(iss jira.Issue) Model {
	m.issue = &iss
	m.viewport.SetContent(m.renderContent())
	m.viewport.GotoTop()
	return m
}

// SetIssueURL sets the browser URL for the current issue.
func (m *Model) SetIssueURL(url string) {
	m.issueURL = url
}

// CurrentIssue returns the currently displayed issue, or nil.
func (m Model) CurrentIssue() *jira.Issue {
	return m.issue
}

// OpenURL returns the URL to open (if requested) and resets the flag.
func (m *Model) OpenURL() (string, bool) {
	if !m.openURL || m.issueURL == "" {
		return "", false
	}
	m.openURL = false
	return m.issueURL, true
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, m.openKeys) && m.issueURL != "" {
			m.openURL = true
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// View renders the issue view.
func (m Model) View() string {
	if m.issue == nil {
		return theme.StyleSubtle.Render("No issue selected.")
	}

	header := m.renderHeader()
	return lipgloss.JoinVertical(lipgloss.Left, header, m.viewport.View())
}

func (m Model) renderHeader() string {
	if m.issue == nil {
		return ""
	}

	iss := m.issue
	keyStyle := theme.StyleKey
	statusStyle := theme.StatusStyle(iss.Status)

	title := fmt.Sprintf("%s  %s  %s",
		keyStyle.Render(iss.Key),
		iss.Summary,
		statusStyle.Render(fmt.Sprintf("[%s]", iss.Status)),
	)

	scrollPct := fmt.Sprintf("%.0f%%", m.viewport.ScrollPercent()*100)
	info := theme.StyleSubtle.Render(scrollPct)

	gap := strings.Repeat(" ", max(0, m.width-lipgloss.Width(title)-lipgloss.Width(info)))

	headerStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(theme.ColourSubtle)

	return headerStyle.Render(title + gap + info)
}

func (m Model) renderContent() string {
	if m.issue == nil {
		return ""
	}

	iss := m.issue
	var b strings.Builder

	// Metadata.
	labelStyle := theme.StyleSubtle
	writeField := func(label, value string) {
		if value == "" {
			value = "—"
		}
		fmt.Fprintf(&b, "%s %s\n", labelStyle.Render(label+":"), value)
	}

	writeField("Type", iss.IssueType)
	writeField("Priority", iss.Priority)
	writeField("Assignee", iss.Assignee)
	writeField("Reporter", iss.Reporter)

	if len(iss.Labels) > 0 {
		writeField("Labels", strings.Join(iss.Labels, ", "))
	}

	// Description.
	b.WriteString("\n")
	b.WriteString(theme.StyleTitle.Render("Description"))
	b.WriteString("\n\n")

	desc := iss.Description
	if desc == "" {
		desc = theme.StyleSubtle.Render("No description.")
	} else {
		desc = markup.Render(desc, m.width-4)
	}
	b.WriteString(desc)

	// Comments.
	if len(iss.Comments) > 0 {
		b.WriteString("\n\n")
		b.WriteString(theme.StyleTitle.Render(fmt.Sprintf("Comments (%d)", len(iss.Comments))))
		b.WriteString("\n")

		// Show most recent comments (last 10).
		start := 0
		if len(iss.Comments) > 10 {
			start = len(iss.Comments) - 10
		}
		for _, c := range iss.Comments[start:] {
			b.WriteString("\n")
			author := theme.StyleKey.Render(c.Author)
			fmt.Fprintf(&b, "%s\n", author)
			body := markup.Render(c.Body, m.width-4)
			b.WriteString(body)
			b.WriteString("\n")
		}
	}

	return b.String()
}

// wrapText wraps text at the given width.
func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}

	var result strings.Builder
	for _, line := range strings.Split(text, "\n") {
		if lipgloss.Width(line) <= width {
			result.WriteString(line)
			result.WriteString("\n")
			continue
		}

		words := strings.Fields(line)
		current := ""
		for _, word := range words {
			if current == "" {
				current = word
			} else if lipgloss.Width(current+" "+word) <= width {
				current += " " + word
			} else {
				result.WriteString(current)
				result.WriteString("\n")
				current = word
			}
		}
		if current != "" {
			result.WriteString(current)
			result.WriteString("\n")
		}
	}

	return strings.TrimRight(result.String(), "\n")
}
