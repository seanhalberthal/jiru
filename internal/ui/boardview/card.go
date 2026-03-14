package boardview

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/seanhalberthal/jiru/internal/jira"
	"github.com/seanhalberthal/jiru/internal/theme"
)

// renderCard renders an issue as a compact card.
// width is the available content width (excluding borders/padding).
func renderCard(issue jira.Issue, width int, selected bool) string {
	style := theme.StyleCard.Width(width)
	if selected {
		style = theme.StyleCardSelected.Width(width)
	}

	key := theme.StyleKey.Render(issue.Key)

	// Truncate summary to fit card width.
	maxSummary := width - lipgloss.Width(key) - 1
	summary := issue.Summary
	if lipgloss.Width(summary) > maxSummary && maxSummary > 3 {
		summary = summary[:maxSummary-3] + "..."
	}

	line1 := fmt.Sprintf("%s %s", key, summary)

	assignee := issue.Assignee
	if assignee == "" {
		assignee = "Unassigned"
	}
	line2 := theme.StyleSubtle.Render(fmt.Sprintf("%s · %s", issue.IssueType, assignee))

	content := lipgloss.JoinVertical(lipgloss.Left, line1, line2)
	return style.Render(content)
}
