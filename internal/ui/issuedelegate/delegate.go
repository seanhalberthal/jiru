package issuedelegate

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/seanhalberthal/jiru/internal/jira"
	"github.com/seanhalberthal/jiru/internal/theme"
)

// Item wraps a jira.Issue to implement list.Item.
type Item struct {
	Issue jira.Issue
}

func (i Item) FilterValue() string {
	return i.Issue.Key + " " + i.Issue.Summary
}

// ToItems converts a slice of jira.Issue to list items.
func ToItems(issues []jira.Issue) []list.Item {
	items := make([]list.Item, len(issues))
	for i, iss := range issues {
		items[i] = Item{Issue: iss}
	}
	return items
}

// Delegate renders issue list items with key, summary, status badge, and assignee.
type Delegate struct{}

func (d Delegate) Height() int  { return 2 }
func (d Delegate) Spacing() int { return 0 }

func (d Delegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d Delegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	i, ok := item.(Item)
	if !ok {
		return
	}

	iss := i.Issue
	isSelected := index == m.Index()

	keyStyle := theme.StyleKey
	if isSelected {
		keyStyle = keyStyle.Underline(true)
	}

	statusStyle := theme.StatusStyle(iss.Status)

	key := keyStyle.Render(iss.Key)
	status := statusStyle.Render(fmt.Sprintf("[%s]", iss.Status))

	// First line: key + summary (truncated) + status badge.
	maxSummaryWidth := m.Width() - lipgloss.Width(key) - lipgloss.Width(status) - 4
	summary := iss.Summary
	if lipgloss.Width(summary) > maxSummaryWidth && maxSummaryWidth > 3 {
		summary = summary[:maxSummaryWidth-3] + "..."
	}

	line1 := fmt.Sprintf("%s %s %s", key, summary, status)

	// Second line: type + assignee.
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
