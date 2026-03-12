package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/seanhalberthal/jiratui/internal/theme"
)

// footerBinding is a key + description pair for the footer.
type footerBinding struct {
	Key  string
	Desc string
}

// footerView renders a persistent keybind bar for the given view.
func footerView(active view, width int, extra ...footerBinding) string {
	var bindings []footerBinding

	// Common bindings present in most views.
	nav := footerBinding{"j/k", "navigate"}
	open := footerBinding{"enter/l", "open"}
	back := footerBinding{"esc/h", "back"}
	search := footerBinding{"?", "JQL"}
	filter := footerBinding{"/", "filter"}
	refresh := footerBinding{"r", "refresh"}
	quit := footerBinding{"q", "quit"}

	switch active {
	case viewHome:
		bindings = []footerBinding{nav, open, filter, search, quit}
	case viewSprint:
		bindings = []footerBinding{
			nav, open, back, filter,
			{"b", "board view"},
			search, refresh, quit,
		}
	case viewBoard:
		bindings = []footerBinding{
			nav,
			{"h/l", "columns"},
			{"enter/L", "open"},
			{"H/esc", "back"},
		}
		bindings = append(bindings, extra...)
		bindings = append(bindings,
			footerBinding{"b", "list view"},
			search, refresh, quit,
		)
	case viewIssue:
		bindings = []footerBinding{
			nav, back,
			{"o", "browser"},
			search, quit,
		}
	case viewSearch:
		bindings = []footerBinding{
			{"enter", "search"},
			{"tab", "complete"},
			{"esc", "close"},
			quit,
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

	// Truncate if wider than terminal.
	if lipgloss.Width(bar) > width {
		bar = bar[:width]
	}

	return bar
}
