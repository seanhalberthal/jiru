package markup

import (
	"strings"

	"github.com/undont/jiru/internal/theme"
)

// Render converts Atlassian wiki markup to styled terminal text.
// width is the available terminal width for wrapping. If width <= 0, no wrapping is applied.
func Render(input string, width int) string {
	if input == "" {
		return ""
	}

	// Strip any raw ANSI escape sequences that may have been pasted into the
	// Jira content — they are meaningless in wiki markup and render as garbled
	// text in the terminal. Also strip orphaned bracket sequences where the
	// ESC prefix was lost (e.g. "[38;2;224;175;104m" instead of "\x1b[38;...m").
	input = stripANSI(input)
	input = stripOrphanedANSI(input)

	lines := strings.Split(input, "\n")
	var result []string

	i := 0
	for i < len(lines) {
		// Try block-level elements first (multi-line constructs).
		if block, advance := parseBlock(lines, i, width); advance > 0 {
			result = append(result, block)
			i += advance
			continue
		}

		line := lines[i]

		// Single-line block elements.
		if rendered, ok := renderBlockLine(line, width); ok {
			result = append(result, rendered)
			i++
			continue
		}

		// Plain paragraph text — apply inline formatting and wrap.
		rendered := renderInline(line)
		if width > 0 {
			rendered = theme.WrapStyledText(rendered, width)
		}
		result = append(result, rendered)
		i++
	}

	return strings.Join(result, "\n")
}
