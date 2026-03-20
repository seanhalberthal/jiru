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
// Bindings wrap to multiple rows when they exceed the terminal width.
func footerView(active view, width int, version string, errShowing bool, extra ...footerBinding) string {
	var bindings []footerBinding

	if errShowing {
		bindings = []footerBinding{
			{"esc", "dismiss"},
			{"ctrl+c", "quit"},
		}
	} else {
		// Common bindings present in most views.
		nav := footerBinding{"j/k", "navigate"}
		scroll := footerBinding{"d/u", "½ page"}
		open := footerBinding{"enter", "open"}
		back := footerBinding{"esc", "back"}
		search := footerBinding{"?", "JQL"}
		filter := footerBinding{"/", "filter"}
		refresh := footerBinding{"r", "refresh"}
		quit := footerBinding{"q", "quit"}

		switch active {
		case viewHome:
			bindings = []footerBinding{nav, open, {"/", "filter"}, search, {"f", "filters"}, {"c", "create"}, refresh, {"P", "profile"}, {"S", "setup"}, quit}
		case viewSprint:
			bindings = []footerBinding{
				nav, scroll, open, back, filter,
				{"b", "board view"},
				search, {"f", "filters"}, {"c", "create"}, refresh, {"P", "profile"}, {"S", "setup"},
			}
		case viewBoard:
			bindings = []footerBinding{
				nav, scroll,
				{"h/l", "columns"},
				open, back,
				{"m", "move"},
			}
			bindings = append(bindings, extra...)
			bindings = append(bindings,
				footerBinding{"b", "list view"},
				search, footerBinding{"f", "filters"}, footerBinding{"c", "create"}, refresh, footerBinding{"P", "profile"}, footerBinding{"S", "setup"},
			)
		case viewIssue:
			bindings = []footerBinding{
				nav, scroll, {"g/G", "top/bottom"}, back,
				{"p", "parent"},
				{"i", "go to issue"},
				{"e", "edit"},
				{"a", "assign"},
				{"m", "move"},
				{"l", "link"},
				{"c", "comment"},
				{"o", "browser"},
				{"x", "copy url"},
				{"n", "branch"},
				{"D", "delete"},
				refresh, search,
			}
		case viewIssuePick:
			bindings = []footerBinding{
				nav, {"enter", "select"}, back,
			}
		case viewBranch:
			bindings = []footerBinding{
				{"tab", "switch field"}, {"enter", "copy"}, back,
			}
		case viewSearch:
			bindings = append(bindings, extra...)
		case viewTransition:
			bindings = []footerBinding{
				nav, {"enter", "select"}, back,
			}
		case viewComment:
			bindings = []footerBinding{
				{"ctrl+s", "submit"}, back,
			}
		case viewFilters:
			bindings = append(bindings, extra...)
		case viewLoading:
			bindings = []footerBinding{quit}
		}
	}

	// Render each binding.
	sep := "  "
	var parts []string
	for _, b := range bindings {
		parts = append(parts, fmt.Sprintf(
			"%s %s",
			theme.StyleHelpKey.Render(b.Key),
			theme.StyleHelpDesc.Render(b.Desc),
		))
	}

	// Lay out bindings into rows that fit within the terminal width,
	// reserving space for the version string on the last row.
	verRendered := ""
	verWidth := 0
	if version != "" {
		verRendered = theme.StyleSubtle.Render(version)
		verWidth = lipgloss.Width(verRendered) + 2 // 2 for gap
	}

	var rows []string
	var currentRow []string
	currentWidth := 0
	sepWidth := lipgloss.Width(sep)

	for _, part := range parts {
		partWidth := lipgloss.Width(part)

		const maxPerRow = 11
		if len(currentRow) > 0 && (len(currentRow) >= maxPerRow || currentWidth+sepWidth+partWidth > width) {
			rows = append(rows, strings.Join(currentRow, sep))
			currentRow = nil
			currentWidth = 0
		}

		if len(currentRow) > 0 {
			currentWidth += sepWidth
		}
		currentRow = append(currentRow, part)
		currentWidth += partWidth
	}

	// Flush the last row — append the version right-aligned if it fits.
	if len(currentRow) > 0 {
		lastRow := strings.Join(currentRow, sep)
		lastRowWidth := lipgloss.Width(lastRow)

		if verRendered != "" && lastRowWidth+verWidth <= width {
			gap := width - lastRowWidth - lipgloss.Width(verRendered)
			lastRow = lastRow + strings.Repeat(" ", gap) + verRendered
		} else if verRendered != "" {
			// Version doesn't fit on the last binding row — add its own row.
			rows = append(rows, lastRow)
			lastRow = strings.Repeat(" ", width-lipgloss.Width(verRendered)) + verRendered
		}

		rows = append(rows, lastRow)
	} else if verRendered != "" {
		rows = append(rows, strings.Repeat(" ", width-lipgloss.Width(verRendered))+verRendered)
	}

	return strings.Join(rows, "\n")
}
