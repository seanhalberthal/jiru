package markup

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/seanhalberthal/jiru/internal/theme"
)

// parseTable parses consecutive table rows (lines starting with | or ||)
// and renders them as a formatted text table.
func parseTable(lines []string, start int, width int) (string, int) {
	var rows []tableRow
	end := start

	for end < len(lines) {
		trimmed := strings.TrimSpace(lines[end])
		if !strings.HasPrefix(trimmed, "|") {
			break
		}
		rows = append(rows, parseTableRow(trimmed))
		end++
	}

	if len(rows) == 0 {
		return "", 0
	}

	return renderTable(rows, width), end - start
}

type tableRow struct {
	cells    []string
	isHeader bool
}

// parseTableRow parses a single table row.
// ||cell|| = header row, |cell| = data row.
func parseTableRow(line string) tableRow {
	isHeader := strings.HasPrefix(line, "||")

	separator := "|"
	if isHeader {
		separator = "||"
	}

	// Remove leading and trailing separators.
	line = strings.TrimPrefix(line, separator)
	line = strings.TrimSuffix(line, separator)

	cells := strings.Split(line, separator)
	for i := range cells {
		cells[i] = strings.TrimSpace(cells[i])
	}

	return tableRow{cells: cells, isHeader: isHeader}
}

// renderTable renders parsed table rows as styled text.
func renderTable(rows []tableRow, width int) string {
	if len(rows) == 0 {
		return ""
	}

	// Calculate column widths.
	numCols := 0
	for _, row := range rows {
		if len(row.cells) > numCols {
			numCols = len(row.cells)
		}
	}

	colWidths := make([]int, numCols)
	for _, row := range rows {
		for i, cell := range row.cells {
			if lipgloss.Width(cell) > colWidths[i] {
				colWidths[i] = lipgloss.Width(cell)
			}
		}
	}

	// Cap total width if it exceeds available width.
	totalWidth := numCols + 1 // separators
	for _, w := range colWidths {
		totalWidth += w + 2 // padding
	}
	if width > 0 && totalWidth > width {
		available := width - numCols - 1
		for i := range colWidths {
			colWidths[i] = max(4, available*colWidths[i]/totalWidth)
		}
	}

	var b strings.Builder
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(theme.ColourPrimary)

	for _, row := range rows {
		b.WriteString("│")
		for i := 0; i < numCols; i++ {
			cell := ""
			if i < len(row.cells) {
				cell = renderInline(row.cells[i])
			}

			padded := padOrTruncate(cell, colWidths[i])

			if row.isHeader {
				b.WriteString(" " + headerStyle.Render(padded) + " │")
			} else {
				b.WriteString(" " + padded + " │")
			}
		}
		b.WriteString("\n")

		// Add separator after header rows.
		if row.isHeader {
			b.WriteString("├")
			for i := 0; i < numCols; i++ {
				b.WriteString(strings.Repeat("─", colWidths[i]+2))
				if i < numCols-1 {
					b.WriteString("┼")
				}
			}
			b.WriteString("┤\n")
		}
	}

	return strings.TrimRight(b.String(), "\n")
}

// padOrTruncate pads a string with spaces or truncates it to fit width.
func padOrTruncate(s string, width int) string {
	w := lipgloss.Width(s)
	if w > width && width > 3 {
		// Truncation of styled text is tricky — fall back to simple truncation.
		stripped := stripANSI(s)
		if len(stripped) > width-3 {
			return stripped[:width-3] + "..."
		}
		return s
	}
	if w < width {
		return s + strings.Repeat(" ", width-w)
	}
	return s
}
