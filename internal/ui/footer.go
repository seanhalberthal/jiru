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
		topBottom := footerBinding{"g/G", "top/bottom"}
		columns := footerBinding{"h/l", "columns"}
		open := footerBinding{"enter", "open"}
		sel := footerBinding{"enter", "select"}
		back := footerBinding{"esc", "back"}
		jql := footerBinding{"s", "JQL"}
		help := footerBinding{"?", "help"}
		filter := footerBinding{"/", "filter"}
		refresh := footerBinding{"r", "refresh"}
		browser := footerBinding{"o", "browser"}
		copyURL := footerBinding{"x", "copy url"}
		quit := footerBinding{"q", "quit"}
		board := footerBinding{"b", "board view"}
		listView := footerBinding{"b", "list view"}
		create := footerBinding{"c", "create"}
		comment := footerBinding{"c", "comment"}
		comments := footerBinding{"c", "view comments"}
		inlineNav := footerBinding{"]/[", "next/prev inline"}
		filters := footerBinding{"f", "filters"}
		move := footerBinding{"m", "move"}
		assign := footerBinding{"a", "assign"}
		edit := footerBinding{"e", "edit"}
		link := footerBinding{"L", "link"}
		del := footerBinding{"D", "delete"}
		parent := footerBinding{"p", "parent"}
		issuePick := footerBinding{"i", "issues"}
		issuesPages := footerBinding{"i", "issues/pages"}
		watch := footerBinding{"w", "watch"}
		branch := footerBinding{"n", "branch"}
		wiki := footerBinding{"tab", "wiki"}
		jira := footerBinding{"tab", "jira"}
		home := footerBinding{"H", "home"}
		boards := footerBinding{"B", "boards"}
		profile := footerBinding{"P", "profile"}
		setup := footerBinding{"S", "setup"}
		submit := footerBinding{"ctrl+s", "submit"}
		switchField := footerBinding{"tab", "switch field"}
		copy := footerBinding{"enter", "copy"}

		switch active {
		case viewSpaces:
			bindings = []footerBinding{nav, open, back, filter, jira, home, help, quit}
		case viewConfluence:
			bindings = []footerBinding{nav, scroll, topBottom, back, comments, inlineNav, issuesPages, browser, refresh, help}
		case viewSprint:
			bindings = []footerBinding{nav, scroll, open, back, filter, board, move, link, copyURL, jql, filters, create, wiki, refresh, boards, profile, setup, help}
		case viewBoard:
			bindings = []footerBinding{nav, scroll, columns, open, back, move, link, copyURL}
			bindings = append(bindings, extra...)
			bindings = append(bindings, listView, jql, filters, create, wiki, refresh, home, boards, profile, setup, help)
		case viewSearchBoard:
			bindings = []footerBinding{nav, scroll, columns, open, back, move, link, copyURL, listView, filters, refresh, home, help}
		case viewIssue:
			bindings = []footerBinding{nav, scroll, topBottom, back, parent, issuePick, edit, assign, move, link, comment, watch, browser, copyURL, branch, del, refresh, jql, home, help}
		case viewIssuePick:
			bindings = []footerBinding{nav, sel, back}
		case viewBranch:
			bindings = []footerBinding{switchField, copy, back}
		case viewSearch:
			bindings = append(bindings, extra...)
		case viewTransition:
			bindings = []footerBinding{nav, sel, back}
		case viewComment:
			bindings = []footerBinding{submit, back}
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
