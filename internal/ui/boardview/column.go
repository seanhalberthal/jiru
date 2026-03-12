package boardview

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/seanhalberthal/jiratui/internal/jira"
	"github.com/seanhalberthal/jiratui/internal/theme"
)

// column represents a single kanban column.
type column struct {
	name   string
	issues []jira.Issue
	cursor int // Selected issue index within column.
	offset int // Scroll offset for visible window.
	width  int
	height int
}

func newColumn(name string, issues []jira.Issue) column {
	return column{
		name:   name,
		issues: issues,
	}
}

func (c *column) setSize(width, height int) {
	c.width = width
	c.height = height
}

func (c *column) moveUp() {
	if c.cursor > 0 {
		c.cursor--
		if c.cursor < c.offset {
			c.offset = c.cursor
		}
	}
}

func (c *column) moveDown() {
	if c.cursor < len(c.issues)-1 {
		c.cursor++
	}
}

// clampCursor ensures the cursor is within bounds (e.g., after filtering).
func (c *column) clampCursor() {
	if len(c.issues) == 0 {
		c.cursor = 0
		c.offset = 0
		return
	}
	if c.cursor >= len(c.issues) {
		c.cursor = len(c.issues) - 1
	}
	if c.offset > c.cursor {
		c.offset = c.cursor
	}
}

func (c *column) selectedIssue() *jira.Issue {
	if len(c.issues) == 0 {
		return nil
	}
	return &c.issues[c.cursor]
}

func (c column) view(active bool) string {
	// Column header: status name + count.
	headerStyle := theme.StyleColumnTitle
	if !active {
		headerStyle = headerStyle.Foreground(theme.ColourSubtle)
	}
	header := headerStyle.Render(fmt.Sprintf("%s (%d)", c.name, len(c.issues)))

	// Card width = column width minus border/padding overhead.
	cardWidth := c.width - 4
	if cardWidth < 10 {
		cardWidth = 10
	}

	// Render visible cards with scrolling.
	// Each card is 5 lines tall (2 content + 2 border + 1 margin).
	// Column header takes 2 lines (text + margin bottom).
	cardHeight := 5
	visibleCards := (c.height - 2) / cardHeight
	if visibleCards < 1 {
		visibleCards = 1
	}

	// Adjust offset so cursor is visible.
	if c.cursor >= c.offset+visibleCards {
		c.offset = c.cursor - visibleCards + 1
	}
	if c.cursor < c.offset {
		c.offset = c.cursor
	}

	cards := ""
	end := c.offset + visibleCards
	if end > len(c.issues) {
		end = len(c.issues)
	}

	for i := c.offset; i < end; i++ {
		selected := active && i == c.cursor
		cards += renderCard(c.issues[i], cardWidth, selected) + "\n"
	}

	if len(c.issues) == 0 {
		cards = theme.StyleSubtle.Render("  No issues")
	}

	content := lipgloss.JoinVertical(lipgloss.Left, header, cards)
	return content
}
