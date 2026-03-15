package boardview

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/seanhalberthal/jiru/internal/jira"
	"github.com/seanhalberthal/jiru/internal/theme"
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
		c.ensureVisible()
	}
}

// cardHeight is the total height of one rendered card:
// 2 content lines + 2 border lines + 1 MarginBottom = 5 lines.
const cardHeight = 5

// headerLines is the height of the column header (rendered text).
const headerLines = 1

func (c *column) visibleCards() int {
	vis := (c.height - headerLines) / cardHeight
	if vis < 1 {
		vis = 1
	}
	return vis
}

func (c *column) moveHalfPageDown() {
	jump := c.visibleCards() / 2
	if jump < 1 {
		jump = 1
	}
	c.cursor += jump
	if c.cursor >= len(c.issues) {
		c.cursor = len(c.issues) - 1
	}
	if c.cursor < 0 {
		c.cursor = 0
	}
	c.ensureVisible()
}

func (c *column) moveHalfPageUp() {
	jump := c.visibleCards() / 2
	if jump < 1 {
		jump = 1
	}
	c.cursor -= jump
	if c.cursor < 0 {
		c.cursor = 0
	}
	c.ensureVisible()
}

// ensureVisible adjusts the scroll offset so the cursor is within the visible window.
func (c *column) ensureVisible() {
	vis := c.visibleCards()
	if c.cursor >= c.offset+vis {
		c.offset = c.cursor - vis + 1
	}
	if c.cursor < c.offset {
		c.offset = c.cursor
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
	// Use the same constants as visibleCards() for consistency.
	vis := (c.height - headerLines) / cardHeight
	if vis < 1 {
		vis = 1
	}

	cards := ""
	end := c.offset + vis
	if end > len(c.issues) {
		end = len(c.issues)
	}

	for i := c.offset; i < end; i++ {
		selected := active && i == c.cursor
		cards += renderCard(c.issues[i], cardWidth, selected)
	}

	if len(c.issues) == 0 {
		cards = theme.StyleSubtle.Render("  No issues")
	}

	content := lipgloss.JoinVertical(lipgloss.Left, header, cards)

	// Fix the column to a consistent height so all columns align.
	return lipgloss.NewStyle().Height(c.height).MaxHeight(c.height).Render(content)
}
