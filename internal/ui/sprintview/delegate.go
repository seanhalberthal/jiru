package sprintview

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/seanhalberthal/jiratui/internal/jira"
	"github.com/seanhalberthal/jiratui/internal/theme"
)

// issueItem wraps a jira.Issue to implement list.Item.
type issueItem struct {
	issue jira.Issue
}

func (i issueItem) FilterValue() string {
	return i.issue.Key + " " + i.issue.Summary
}

// issueDelegate renders list items with key, summary, status, and assignee.
type issueDelegate struct{}

func (d issueDelegate) Height() int  { return 2 }
func (d issueDelegate) Spacing() int { return 0 }

func (d issueDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d issueDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	i, ok := item.(issueItem)
	if !ok {
		return
	}

	iss := i.issue
	isSelected := index == m.Index()

	keyStyle := theme.StyleKey
	if isSelected {
		keyStyle = keyStyle.Underline(true)
	}

	statusStyle := theme.StatusStyle(iss.Status)

	key := keyStyle.Render(iss.Key)
	status := statusStyle.Render(fmt.Sprintf("[%s]", iss.Status))

	// First line: key + summary (truncated).
	maxSummaryWidth := m.Width() - lipgloss.Width(key) - lipgloss.Width(status) - 4
	summary := iss.Summary
	if lipgloss.Width(summary) > maxSummaryWidth && maxSummaryWidth > 3 {
		summary = summary[:maxSummaryWidth-3] + "..."
	}

	line1 := fmt.Sprintf("%s %s %s", key, summary, status)

	// Second line: assignee + type.
	assignee := iss.Assignee
	if assignee == "" {
		assignee = "Unassigned"
	}
	line2 := theme.StyleSubtle.Render(fmt.Sprintf("  %s · %s", iss.IssueType, assignee))

	if isSelected {
		cursor := lipgloss.NewStyle().
			Foreground(theme.ColourPrimary).
			Bold(true).
			Render("▸ ")
		line1 = cursor + line1
	} else {
		line1 = "  " + line1
	}

	_, _ = fmt.Fprintf(w, "%s\n%s", line1, line2)
}
