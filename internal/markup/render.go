package markup

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Render converts Atlassian wiki markup to styled terminal text.
// width is the available terminal width for wrapping. If width <= 0, no wrapping is applied.
func Render(input string, width int) string {
	if input == "" {
		return ""
	}

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
			rendered = wrapStyledText(rendered, width)
		}
		result = append(result, rendered)
		i++
	}

	return strings.Join(result, "\n")
}

// wrapStyledText wraps text at the given width, respecting ANSI escape sequences.
// This delegates to lipgloss.Width for accurate width calculation of styled text.
func wrapStyledText(text string, width int) string {
	if width <= 0 {
		return text
	}

	var result strings.Builder
	for _, line := range strings.Split(text, "\n") {
		if lipgloss.Width(line) <= width {
			result.WriteString(line)
			result.WriteString("\n")
			continue
		}

		// For styled text, split on word boundaries while preserving ANSI sequences.
		words := strings.Fields(line)
		current := ""
		for _, word := range words {
			if current == "" {
				current = word
			} else if lipgloss.Width(current+" "+word) <= width {
				current += " " + word
			} else {
				result.WriteString(current)
				result.WriteString("\n")
				current = word
			}
		}
		if current != "" {
			result.WriteString(current)
			result.WriteString("\n")
		}
	}

	return strings.TrimRight(result.String(), "\n")
}
