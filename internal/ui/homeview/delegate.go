package homeview

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/seanhalberthal/jiratui/internal/jira"
	"github.com/seanhalberthal/jiratui/internal/theme"
)

type boardItem struct {
	stats jira.BoardStats
}

func (i boardItem) FilterValue() string {
	return i.stats.Board.Name
}

type boardDelegate struct{}

func NewDelegate() boardDelegate {
	return boardDelegate{}
}

func (d boardDelegate) Height() int  { return 3 }
func (d boardDelegate) Spacing() int { return 0 }

func (d boardDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd {
	return nil
}

func (d boardDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	bi, ok := item.(boardItem)
	if !ok {
		return
	}

	stats := bi.stats
	isSelected := index == m.Index()
	width := m.Width()

	cursor := "  "
	if isSelected {
		cursor = theme.StyleKey.Render("▸ ")
	}

	name := stats.Board.Name
	boardType := theme.StyleSubtle.Render(fmt.Sprintf("[%s]", stats.Board.Type))
	if isSelected {
		name = theme.StyleKey.Underline(true).Render(name)
	} else {
		name = theme.StyleTitle.Render(name)
	}

	line1 := fmt.Sprintf("%s%s %s", cursor, name, boardType)

	sprintLine := "  "
	if stats.ActiveSprint != "" {
		sprintLine += theme.StyleSubtle.Render(fmt.Sprintf("Sprint: %s", stats.ActiveSprint))
	} else {
		sprintLine += theme.StyleSubtle.Render("No active sprint")
	}

	statsLine := "  "
	if stats.TotalIssues > 0 {
		parts := []string{}
		if stats.OpenIssues > 0 {
			parts = append(parts, theme.StyleStatusOpen.Render(fmt.Sprintf("%d open", stats.OpenIssues)))
		}
		if stats.InProgress > 0 {
			parts = append(parts, theme.StyleStatusInProgress.Render(fmt.Sprintf("%d in progress", stats.InProgress)))
		}
		if stats.DoneIssues > 0 {
			parts = append(parts, theme.StyleStatusDone.Render(fmt.Sprintf("%d done", stats.DoneIssues)))
		}
		statsLine += strings.Join(parts, theme.StyleSubtle.Render(" · "))
		statsLine += theme.StyleSubtle.Render(fmt.Sprintf("  (%d total)", stats.TotalIssues))
	}

	_ = width

	_, _ = fmt.Fprintf(w, "%s\n%s\n%s", line1, sprintLine, statsLine)
}
