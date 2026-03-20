package markup

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/pgavlin/mermaid-ascii/pkg/diagram"
	"github.com/pgavlin/mermaid-ascii/pkg/render"
	"github.com/seanhalberthal/jiru/internal/theme"
)

// --- Styles for markup rendering ---

var (
	styleBold          = lipgloss.NewStyle().Bold(true)
	styleItalic        = lipgloss.NewStyle().Italic(true)
	styleUnderline     = lipgloss.NewStyle().Underline(true)
	styleStrikethrough = lipgloss.NewStyle().Strikethrough(true)
	styleMonospace     = lipgloss.NewStyle().Foreground(theme.ColourWarning)
	styleSuperscript   = lipgloss.NewStyle().Foreground(theme.ColourSubtle)
	styleSubscript     = lipgloss.NewStyle().Foreground(theme.ColourSubtle)
	styleLink          = lipgloss.NewStyle().Foreground(theme.ColourPrimary).Underline(true)
	styleLinkURL       = lipgloss.NewStyle().Foreground(theme.ColourSubtle)
	styleHeading       = lipgloss.NewStyle().Bold(true).Foreground(theme.ColourPrimary)
	styleBlockquote    = lipgloss.NewStyle().Foreground(theme.ColourSubtle).Italic(true)
	styleCodeBlock     = lipgloss.NewStyle().Foreground(theme.ColourWarning)
	stylePanel         = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(theme.ColourSubtle).Padding(0, 1)
	styleHRule         = lipgloss.NewStyle().Foreground(theme.ColourSubtle)
	styleCitation      = lipgloss.NewStyle().Italic(true).Foreground(theme.ColourSubtle)
	styleBullet        = lipgloss.NewStyle().Foreground(theme.ColourPrimary).Bold(true)
	styleImage         = lipgloss.NewStyle().Foreground(theme.ColourSubtle).Italic(true)
)

// --- Inline patterns ---
// Go's regexp doesn't support lookbehinds, so we capture boundary characters
// and re-emit them alongside the styled content.

var inlinePatterns = []struct {
	re      *regexp.Regexp
	replace func(matches []string) string
}{
	// Monospace {{text}} — must come before other inline patterns.
	{
		re: regexp.MustCompile(`\{\{(.+?)\}\}`),
		replace: func(m []string) string {
			return styleMonospace.Render(m[1])
		},
	},
	// Colour {color:name}text{color}.
	{
		re: regexp.MustCompile(`\{color:(\w+)\}(.*?)\{color\}`),
		replace: func(m []string) string {
			colour := mapColour(m[1])
			return lipgloss.NewStyle().Foreground(colour).Render(m[2])
		},
	},
	// Links [alias|url] or [url].
	{
		re: regexp.MustCompile(`\[([^\]\|]+?)(?:\|([^\]]+?))?\]`),
		replace: func(m []string) string {
			return renderLink(m[1], m[2])
		},
	},
	// Images !url! or !file|params!.
	{
		re: regexp.MustCompile(`!([^!\s]+?)(?:\|[^!]*)?\s*!`),
		replace: func(m []string) string {
			return styleImage.Render(fmt.Sprintf("[image: %s]", m[1]))
		},
	},
	// Bold *text* — boundary-aware (start of line or whitespace/punct before, same after).
	{
		re: regexp.MustCompile(`(^|[\s.,;:!?()\[\]])\*([^\s*](?:[^*]*[^\s*])?)\*([\s.,;:!?()\[\]]|$)`),
		replace: func(m []string) string {
			return m[1] + styleBold.Render(m[2]) + m[3]
		},
	},
	// Italic _text_ — boundary-aware.
	{
		re: regexp.MustCompile(`(^|[\s.,;:!?()\[\]])_([^\s_](?:[^_]*[^\s_])?)_([\s.,;:!?()\[\]]|$)`),
		replace: func(m []string) string {
			return m[1] + styleItalic.Render(m[2]) + m[3]
		},
	},
	// Strikethrough -text- — boundary-aware.
	{
		re: regexp.MustCompile(`(^|[\s.,;:!?()\[\]])-([^\s\-](?:[^-]*[^\s\-])?)-(?:[\s.,;:!?()\[\]]|$)`),
		replace: func(m []string) string {
			return m[1] + styleStrikethrough.Render(m[2])
		},
	},
	// Underline +text+ — boundary-aware.
	{
		re: regexp.MustCompile(`(^|[\s.,;:!?()\[\]])\+([^\s+](?:[^+]*[^\s+])?)\+([\s.,;:!?()\[\]]|$)`),
		replace: func(m []string) string {
			return m[1] + styleUnderline.Render(m[2]) + m[3]
		},
	},
	// Superscript ^text^.
	{
		re: regexp.MustCompile(`\^(.+?)\^`),
		replace: func(m []string) string {
			return styleSuperscript.Render("^" + m[1])
		},
	},
	// Subscript ~text~.
	{
		re: regexp.MustCompile(`~(.+?)~`),
		replace: func(m []string) string {
			return styleSubscript.Render("_" + m[1])
		},
	},
	// Citation ??text??.
	{
		re: regexp.MustCompile(`\?\?(.+?)\?\?`),
		replace: func(m []string) string {
			return styleCitation.Render("— " + m[1])
		},
	},
}

// renderInline applies all inline formatting patterns to a line of text.
//
// Monospace ({{...}}) and colour ({color:...}...{color}) are rendered first
// and their output is replaced with placeholders. This prevents subsequent
// patterns (especially links [..]) from matching characters inside already-
// styled spans (e.g., the brackets in {{h-[55px]}}). After all patterns
// have run, placeholders are swapped back to the styled text.
func renderInline(line string) string {
	// Handle explicit line breaks (\\).
	line = strings.ReplaceAll(line, `\\`, "\n")

	// Phase 1: render monospace and colour, stash output in placeholders.
	var stash []string
	placeholder := func(styled string) string {
		idx := len(stash)
		stash = append(stash, styled)
		return fmt.Sprintf("\x00PH%d\x00", idx)
	}

	for _, p := range inlinePatterns[:2] { // monospace + colour only
		line = p.re.ReplaceAllStringFunc(line, func(s string) string {
			matches := p.re.FindStringSubmatch(s)
			if matches == nil {
				return s
			}
			return placeholder(p.replace(matches))
		})
	}

	// Phase 2: apply remaining patterns (links, images, bold, etc.).
	for _, p := range inlinePatterns[2:] {
		line = p.re.ReplaceAllStringFunc(line, func(s string) string {
			matches := p.re.FindStringSubmatch(s)
			if matches == nil {
				return s
			}
			return p.replace(matches)
		})
	}

	// Phase 3: restore placeholders.
	for i, styled := range stash {
		line = strings.ReplaceAll(line, fmt.Sprintf("\x00PH%d\x00", i), styled)
	}

	return line
}

// renderLink formats a wiki markup link for terminal display.
func renderLink(first, second string) string {
	if second == "" {
		// [url] or [page] — single-arg link.
		if strings.HasPrefix(first, "http") || strings.HasPrefix(first, "mailto:") {
			return styleLink.Render(first)
		}
		// [~username] — user mention.
		if strings.HasPrefix(first, "~") {
			return styleBold.Render("@" + first[1:])
		}
		// [page] — internal page link, just show the text.
		return styleLink.Render(first)
	}
	// [alias|url] — show alias, with URL in subtle.
	if strings.HasPrefix(second, "http") || strings.HasPrefix(second, "mailto:") {
		return styleLink.Render(first) + " " + styleLinkURL.Render("("+second+")")
	}
	// [alias|page] — just show the alias.
	return styleLink.Render(first)
}

// mapColour converts a wiki markup colour name to a lipgloss colour.
func mapColour(name string) lipgloss.Color {
	colours := map[string]string{
		"red":    "#f7768e",
		"green":  "#9ece6a",
		"blue":   "#7aa2f7",
		"yellow": "#e0af68",
		"orange": "#ff9e64",
		"purple": "#bb9af7",
		"white":  "#c0caf5",
		"black":  "#414868",
		"gray":   "#565f89",
		"grey":   "#565f89",
	}
	if c, ok := colours[strings.ToLower(name)]; ok {
		return lipgloss.Color(c)
	}
	// Try using the name as a hex colour directly.
	if strings.HasPrefix(name, "#") {
		return lipgloss.Color(name)
	}
	return lipgloss.Color("")
}

// isWordBoundary returns true if the character at pos is a word boundary.
func isWordBoundary(s string, pos int) bool {
	if pos < 0 || pos >= len(s) {
		return true // start/end of string is always a boundary
	}
	ch := s[pos]
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '.' ||
		ch == ',' || ch == ';' || ch == ':' || ch == '!' || ch == '?' ||
		ch == '(' || ch == ')' || ch == '[' || ch == ']'
}

// stripANSI removes ANSI escape sequences from text. Handles both proper
// ESC-prefixed sequences (\x1b[...m) and orphaned bracket sequences
// (\[...m) that appear when the ESC character is stripped by an API.
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

// stripOrphanedANSI removes ANSI-like sequences that have lost their ESC
// prefix. Handles two forms:
//   - Bracket-prefixed: [38;2;224;175;104m, [0m, [1;31m
//   - Bare 24-bit colour: 38;2;224;175;104m, 48;2;0;255;128m (specific
//     enough to avoid false positives in normal text)
var orphanedANSIRe = regexp.MustCompile(`\[[0-9;]+m|(?:38|48);2;[0-9]{1,3};[0-9]{1,3};[0-9]{1,3}m`)

func stripOrphanedANSI(s string) string {
	return orphanedANSIRe.ReplaceAllString(s, "")
}

// --- Block-level elements ---

var (
	headingRe        = regexp.MustCompile(`^(h[1-6])\.\s*(.*)$`)
	bulletListRe     = regexp.MustCompile(`^(\*+|-+)\s+(.+)$`)
	numberedListRe   = regexp.MustCompile(`^(#+)\s+(.+)$`)
	codeBlockOpenRe  = regexp.MustCompile(`^\{code(?::([^}]*))?\}`)
	panelBlockOpenRe = regexp.MustCompile(`^\{panel(?::([^}]*))?\}`)
)

// isClosingTag checks whether a line contains a closing macro tag.
// It matches the tag at the start or end of the line (after trimming), or as the entire line.
// It does NOT match opening tags with parameters (e.g. {code:lang} is not a closing {code}).
func isClosingTag(line, tag string) bool {
	trimmed := strings.TrimSpace(line)
	// Exact match (most common case).
	if trimmed == tag {
		return true
	}
	// Tag at start of line (e.g. "{code}some trailing text").
	if strings.HasPrefix(trimmed, tag) {
		return true
	}
	// Tag at end of line (e.g. "some text{code}").
	if strings.HasSuffix(trimmed, tag) {
		return true
	}
	return false
}

// renderBlockLine handles single-line block elements.
// Returns the rendered string and true if the line was a block element, or ("", false) otherwise.
func renderBlockLine(line string, width int) (string, bool) {
	trimmed := strings.TrimSpace(line)

	// Horizontal rule: ----
	if trimmed == "----" {
		ruleWidth := width
		if ruleWidth <= 0 {
			ruleWidth = 40
		}
		return styleHRule.Render(strings.Repeat("─", ruleWidth)), true
	}

	// Headings: h1. through h6.
	if m := headingRe.FindStringSubmatch(trimmed); m != nil {
		text := strings.TrimSpace(m[2])
		if text == "" {
			return "", true // bare "h2." with no text — render as empty line
		}
		level := m[1][1] - '0' // 1-6
		text = renderInline(text)
		return renderHeading(text, int(level)), true
	}

	// Blockquote: bq. text
	if strings.HasPrefix(trimmed, "bq. ") {
		text := renderInline(trimmed[4:])
		return styleBlockquote.Render("│ " + text), true
	}

	// Bulleted lists: * item, ** item, - item, -- item.
	if m := bulletListRe.FindStringSubmatch(line); m != nil {
		depth := len(m[1]) - 1 // 0-based depth
		bullet := "•"
		if m[1][0] == '-' {
			bullet = "▪"
		}
		indent := strings.Repeat("  ", depth)
		text := renderInline(m[2])
		return indent + styleBullet.Render(bullet) + " " + text, true
	}

	// Numbered lists: # item, ## item.
	if m := numberedListRe.FindStringSubmatch(line); m != nil {
		depth := len(m[1]) - 1
		indent := strings.Repeat("  ", depth)
		text := renderInline(m[2])
		marker := styleBullet.Render("•")
		if depth == 0 {
			marker = styleBullet.Render("○")
		}
		return indent + marker + " " + text, true
	}

	return "", false
}

// renderContentLine processes a line inside a container block (panel, note, quote).
// It tries block-level rendering first (headings, lists, rules), falling back to inline.
func renderContentLine(line string, width int) string {
	if rendered, ok := renderBlockLine(line, width); ok {
		return rendered
	}
	return renderInline(line)
}

// renderHeading styles a heading based on level (1 = largest).
func renderHeading(text string, level int) string {
	style := styleHeading
	switch level {
	case 1:
		return style.Render("═ " + text + " ═")
	case 2:
		return style.Render("─ " + text + " ─")
	case 3:
		return style.Render("▸ " + text)
	default:
		return style.Render("  " + text)
	}
}

// parseBlock handles multi-line block constructs (code, noformat, panel, table, quote, admonition).
// Returns the rendered block and the number of lines consumed, or ("", 0) if not a block.
func parseBlock(lines []string, start int, width int) (string, int) {
	trimmed := strings.TrimSpace(lines[start])

	// {code} or {code:language} — may have content on the same line.
	if codeBlockOpenRe.MatchString(trimmed) {
		return parseCodeBlock(lines, start, width)
	}

	// {mermaid} — custom macro from Mermaid for Jira apps.
	if trimmed == "{mermaid}" || strings.HasPrefix(trimmed, "{mermaid}") || strings.HasPrefix(trimmed, "{mermaid:") {
		return parseMermaidBlock(lines, start)
	}

	// {noformat} — may have content on the same line.
	if trimmed == "{noformat}" || strings.HasPrefix(trimmed, "{noformat}") {
		return parseNoformatBlock(lines, start, width)
	}

	// {panel} or {panel:title=...} — may have content on the same line.
	if panelBlockOpenRe.MatchString(trimmed) {
		return parsePanelBlock(lines, start, width)
	}

	// {quote} — may have content on the same line.
	if trimmed == "{quote}" || strings.HasPrefix(trimmed, "{quote}") {
		return parseQuoteBlock(lines, start, width)
	}

	// Admonition macros: {info}, {warning}, {tip}, {note}.
	for _, macro := range []string{"info", "warning", "tip", "note"} {
		tag := "{" + macro + "}"
		if trimmed == tag || strings.HasPrefix(trimmed, tag) {
			return parseAdmonitionBlock(lines, start, width, macro)
		}
	}

	// Tables: lines starting with | or ||.
	if strings.HasPrefix(trimmed, "|") || strings.HasPrefix(trimmed, "||") {
		return parseTable(lines, start, width)
	}

	return "", 0
}

// parseCodeBlock extracts content between {code}...{code} tags.
// Handles content on the same line as the opening tag (e.g. {code:csharp}public class Foo).
func parseCodeBlock(lines []string, start int, _ int) (string, int) {
	header := strings.TrimSpace(lines[start])

	// Extract the tag and any trailing content on the same line.
	tagLoc := codeBlockOpenRe.FindStringIndex(header)
	if tagLoc == nil {
		return "", 0
	}
	tag := header[:tagLoc[1]]
	trailing := strings.TrimSpace(header[tagLoc[1]:])

	lang := ""
	if idx := strings.Index(tag, ":"); idx != -1 {
		lang = strings.TrimSuffix(tag[idx+1:], "}")
	}

	var content []string
	if trailing != "" {
		content = append(content, trailing)
	}

	end := start + 1
	for end < len(lines) {
		if isClosingTag(lines[end], "{code}") {
			break
		}
		content = append(content, lines[end])
		end++
	}

	// Try mermaid rendering: explicit language tag or auto-detected content.
	src := strings.Join(content, "\n")
	if strings.EqualFold(lang, "mermaid") || (lang == "" && isMermaidContent(src)) {
		if rendered, err := renderMermaid(src); err == nil {
			consumed := end - start + 1
			if end >= len(lines) {
				consumed = end - start
			}
			return rendered, consumed
		}
		// Fall through to plain code rendering on error.
	}

	var b strings.Builder
	if lang != "" {
		b.WriteString(theme.StyleSubtle.Render("── " + lang + " ──"))
		b.WriteString("\n")
	}
	for _, line := range content {
		b.WriteString(styleCodeBlock.Render(line))
		b.WriteString("\n")
	}

	consumed := end - start + 1
	if end >= len(lines) {
		consumed = end - start // unclosed block
	}
	return strings.TrimRight(b.String(), "\n"), consumed
}

// parseNoformatBlock extracts content between {noformat}...{noformat} tags.
// Handles content on the same line as the opening tag (e.g. {noformat}some text).
func parseNoformatBlock(lines []string, start int, _ int) (string, int) {
	header := strings.TrimSpace(lines[start])
	trailing := strings.TrimSpace(strings.TrimPrefix(header, "{noformat}"))

	var content []string
	if trailing != "" {
		content = append(content, trailing)
	}

	end := start + 1
	for end < len(lines) {
		if isClosingTag(lines[end], "{noformat}") {
			break
		}
		content = append(content, lines[end])
		end++
	}

	src := strings.Join(content, "\n")

	// Auto-detect mermaid diagrams inside noformat blocks.
	if isMermaidContent(src) {
		if mermaidOut, err := renderMermaid(src); err == nil {
			consumed := end - start + 1
			if end >= len(lines) {
				consumed = end - start
			}
			return mermaidOut, consumed
		}
	}

	rendered := styleCodeBlock.Render(src)
	consumed := end - start + 1
	if end >= len(lines) {
		consumed = end - start
	}
	return rendered, consumed
}

// parsePanelBlock extracts content between {panel}...{panel} tags.
// Handles content on the same line as the opening tag.
func parsePanelBlock(lines []string, start int, width int) (string, int) {
	header := strings.TrimSpace(lines[start])

	// Extract the tag portion and any trailing content.
	tagLoc := panelBlockOpenRe.FindStringIndex(header)
	if tagLoc == nil {
		return "", 0
	}
	tag := header[:tagLoc[1]]
	trailing := strings.TrimSpace(header[tagLoc[1]:])

	title := ""
	if idx := strings.Index(tag, "title="); idx != -1 {
		rest := tag[idx+6:]
		if pipeIdx := strings.IndexAny(rest, "|}"); pipeIdx != -1 {
			title = rest[:pipeIdx]
		}
	}

	var content []string
	if trailing != "" {
		content = append(content, trailing)
	}

	end := start + 1
	for end < len(lines) {
		if isClosingTag(lines[end], "{panel}") {
			break
		}
		content = append(content, lines[end])
		end++
	}

	var b strings.Builder
	if title != "" {
		b.WriteString(styleBold.Render(title))
		b.WriteString("\n")
	}
	for _, line := range content {
		b.WriteString(renderContentLine(line, width))
		b.WriteString("\n")
	}

	panelWidth := width - 4
	if panelWidth < 20 {
		panelWidth = 40
	}
	rendered := stylePanel.Width(panelWidth).Render(strings.TrimRight(b.String(), "\n"))

	consumed := end - start + 1
	if end >= len(lines) {
		consumed = end - start
	}
	return rendered, consumed
}

// parseQuoteBlock extracts content between {quote}...{quote} tags.
// Handles content on the same line as the opening tag.
func parseQuoteBlock(lines []string, start int, width int) (string, int) {
	header := strings.TrimSpace(lines[start])
	trailing := strings.TrimSpace(strings.TrimPrefix(header, "{quote}"))

	var content []string
	if trailing != "" {
		content = append(content, trailing)
	}

	end := start + 1
	for end < len(lines) {
		if isClosingTag(lines[end], "{quote}") {
			break
		}
		content = append(content, lines[end])
		end++
	}

	var rendered []string
	for _, line := range content {
		rendered = append(rendered, styleBlockquote.Render("│ "+renderContentLine(line, width)))
	}

	consumed := end - start + 1
	if end >= len(lines) {
		consumed = end - start
	}
	return strings.Join(rendered, "\n"), consumed
}

// parseAdmonitionBlock handles {info}, {warning}, {tip}, {note} blocks.
func parseAdmonitionBlock(lines []string, start int, width int, macro string) (string, int) {
	header := strings.TrimSpace(lines[start])
	tag := "{" + macro + "}"
	trailing := strings.TrimSpace(strings.TrimPrefix(header, tag))

	var content []string
	if trailing != "" {
		content = append(content, trailing)
	}

	end := start + 1
	for end < len(lines) {
		if isClosingTag(lines[end], tag) {
			break
		}
		content = append(content, lines[end])
		end++
	}

	var icon string
	var borderColour lipgloss.AdaptiveColor
	switch macro {
	case "info":
		icon = "i"
		borderColour = theme.ColourPrimary
	case "warning":
		icon = "!"
		borderColour = theme.ColourWarning
	case "tip":
		icon = ">"
		borderColour = theme.ColourSuccess
	case "note":
		icon = "*"
		borderColour = theme.ColourSubtle
	}

	title := styleBold.Render(strings.ToUpper(macro[:1]) + macro[1:])

	var b strings.Builder
	fmt.Fprintf(&b, "[%s] %s\n", icon, title)
	for _, line := range content {
		b.WriteString(renderContentLine(line, width))
		b.WriteString("\n")
	}

	panelWidth := width - 4
	if panelWidth < 20 {
		panelWidth = 40
	}
	admonitionStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColour).
		Padding(0, 1)
	rendered := admonitionStyle.Width(panelWidth).Render(strings.TrimRight(b.String(), "\n"))

	consumed := end - start + 1
	if end >= len(lines) {
		consumed = end - start
	}
	return rendered, consumed
}

// mermaidKeywords are the diagram type keywords that appear on the first
// non-empty line of a mermaid diagram. Used to auto-detect mermaid content
// inside code/noformat blocks that lack an explicit language tag.
var mermaidKeywords = []string{
	"flowchart", "graph", "sequencediagram", "classdiagram",
	"statediagram", "erdiagram", "gantt", "pie", "mindmap",
	"timeline", "gitgraph", "journey", "quadrantchart",
	"xychart-beta", "c4context", "requirementdiagram",
	"block-beta", "sankey-beta", "packet-beta", "kanban",
	"architecture-beta", "zenuml",
}

// isMermaidContent checks whether text looks like a mermaid diagram by
// inspecting the first non-empty line for a known diagram type keyword.
func isMermaidContent(text string) bool {
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		for _, kw := range mermaidKeywords {
			if strings.HasPrefix(lower, kw) {
				return true
			}
		}
		return false // First non-empty line didn't match.
	}
	return false
}

// parseMermaidBlock extracts content between {mermaid}...{mermaid} tags
// (custom macro from Mermaid for Jira apps).
func parseMermaidBlock(lines []string, start int) (string, int) {
	header := strings.TrimSpace(lines[start])

	// Strip the opening tag and any parameters.
	trailing := header
	if idx := strings.Index(trailing, "}"); idx != -1 {
		trailing = strings.TrimSpace(trailing[idx+1:])
	}

	var content []string
	if trailing != "" {
		content = append(content, trailing)
	}

	end := start + 1
	for end < len(lines) {
		if isClosingTag(lines[end], "{mermaid}") {
			break
		}
		content = append(content, lines[end])
		end++
	}

	src := strings.Join(content, "\n")
	consumed := end - start + 1
	if end >= len(lines) {
		consumed = end - start
	}

	if rendered, err := renderMermaid(src); err == nil {
		return rendered, consumed
	}
	// Fallback: render as plain code.
	return styleCodeBlock.Render(src), consumed
}

// mermaidMarkerRe matches the [xN] spacing markers that mermaid-ascii emits.
var mermaidMarkerRe = regexp.MustCompile(`\[x\d+\]\s*`)

// renderMermaid renders mermaid diagram source to styled Unicode box-drawing art.
func renderMermaid(src string) (string, error) {
	cfg := diagram.DefaultConfig()
	cfg.PaddingBetweenY = 2
	cfg.PaddingBetweenX = 3

	output, err := render.Render(src, cfg)
	if err != nil {
		return "", err
	}

	// Strip the [xN] spacing markers emitted by the renderer.
	output = mermaidMarkerRe.ReplaceAllString(output, "")

	// Strip any ANSI colour codes from the mermaid-ascii output — we apply
	// our own lipgloss styling for consistency with the rest of the UI.
	output = stripANSI(output)

	var b strings.Builder
	b.WriteString(theme.StyleSubtle.Render("── mermaid ──"))
	b.WriteString("\n")
	b.WriteString(styleCodeBlock.Render(output))
	return b.String(), nil
}
