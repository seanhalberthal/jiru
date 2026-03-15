package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/seanhalberthal/jiru/internal/theme"
)

// footerBinding is a key + description pair for the footer.
type footerBinding struct {
	Key  string
	Desc string
}

// footerView renders a persistent keybind bar for the given view.
func footerView(active view, width int, version string, extra ...footerBinding) string {
	var bindings []footerBinding

	// Common bindings present in most views.
	nav := footerBinding{"j/k", "navigate"}
	open := footerBinding{"enter", "open"}
	back := footerBinding{"esc", "back"}
	search := footerBinding{"?", "JQL"}
	filter := footerBinding{"/", "filter"}
	refresh := footerBinding{"r", "refresh"}
	quit := footerBinding{"q", "quit"}

	switch active {
	case viewHome:
		bindings = []footerBinding{nav, open, filter, search, {"c", "create"}, {"S", "setup"}, quit}
	case viewSprint:
		bindings = []footerBinding{
			nav, open, back, filter,
			{"b", "board view"},
			search, {"c", "create"}, refresh, {"S", "setup"},
		}
	case viewBoard:
		bindings = []footerBinding{
			nav,
			{"h/l", "columns"},
			open, back,
		}
		bindings = append(bindings, extra...)
		bindings = append(bindings,
			footerBinding{"b", "list view"},
			search, footerBinding{"c", "create"}, refresh, footerBinding{"S", "setup"},
		)
	case viewIssue:
		bindings = []footerBinding{
			nav, back,
			{"o", "browser"},
			{"n", "branch"},
			search,
		}
	case viewBranch:
		bindings = []footerBinding{
			{"tab", "switch field"}, {"enter", "copy"}, back,
		}
	case viewSearch:
		bindings = []footerBinding{
			{"enter", "search"},
			{"tab", "complete"},
			{"esc", "close"},
		}
	case viewLoading:
		bindings = []footerBinding{quit}
	}

	var parts []string
	for _, b := range bindings {
		parts = append(parts, fmt.Sprintf(
			"%s %s",
			theme.StyleHelpKey.Render(b.Key),
			theme.StyleHelpDesc.Render(b.Desc),
		))
	}

	bar := strings.Join(parts, "  ")

	// Append version right-aligned.
	if version != "" {
		ver := theme.StyleSubtle.Render(version)
		barWidth := lipgloss.Width(bar)
		verWidth := lipgloss.Width(ver)
		gap := width - barWidth - verWidth
		if gap >= 2 {
			bar = bar + strings.Repeat(" ", gap) + ver
		}
	}

	// Truncate if wider than terminal.
	if lipgloss.Width(bar) > width {
		bar = bar[:width]
	}

	return bar
}
