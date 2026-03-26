// Package adf renders Atlassian Document Format (ADF) to styled terminal text.
package adf

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/seanhalberthal/jiru/internal/theme"
)

// --- Styles (shared styles via theme, package-specific styles local) ---

var (
	styleEmoji  = lipgloss.NewStyle()
	styleExpand = lipgloss.NewStyle().Bold(true).Foreground(theme.ColourSubtle)
)

// Render converts an ADF JSON string to styled terminal text.
// width is the available terminal width for wrapping. If width <= 0, no wrapping is applied.
// Returns an empty string on invalid input (intentional graceful degradation for TUI display;
// CLI commands should use ToMarkdown which returns errors).
func Render(adfJSON string, width int) string {
	if adfJSON == "" {
		return ""
	}

	var doc Document
	if err := json.Unmarshal([]byte(adfJSON), &doc); err != nil {
		return ""
	}

	if doc.Type != "doc" || len(doc.Content) == 0 {
		return ""
	}

	var blocks []string
	for _, node := range doc.Content {
		rendered := renderBlock(node, width, 0)
		if rendered != "" {
			blocks = append(blocks, rendered)
		}
	}

	return strings.Join(blocks, "\n\n")
}

// renderBlock renders a top-level/block ADF node.
func renderBlock(node Node, width, indent int) string {
	switch node.Type {
	case "paragraph":
		return renderParagraph(node, width, indent)
	case "heading":
		return renderHeading(node, width)
	case "bulletList":
		return renderBulletList(node, width, indent)
	case "orderedList":
		return renderOrderedList(node, width, indent)
	case "codeBlock":
		return renderCodeBlock(node)
	case "blockquote":
		return renderBlockquote(node, width)
	case "table":
		return renderTable(node, width)
	case "panel":
		return renderPanel(node, width)
	case "rule":
		return renderRule(width)
	case "mediaSingle", "mediaGroup":
		return renderMedia(node)
	case "expand", "nestedExpand":
		return renderExpand(node, width, indent)
	case "taskList":
		return renderTaskList(node, width, indent)
	default:
		// Unknown block — try rendering children as paragraphs.
		return renderChildrenAsBlocks(node, width, indent)
	}
}

// renderParagraph renders inline content as a text block.
func renderParagraph(node Node, width, indent int) string {
	text := renderInlineChildren(node.Content)
	if indent > 0 {
		prefix := strings.Repeat("  ", indent)
		text = prefix + text
	}
	if width > 0 {
		text = theme.WrapStyledText(text, width)
	}
	return text
}

// renderHeading renders a heading node.
func renderHeading(node Node, _ int) string {
	text := renderInlineChildren(node.Content)
	level := 1
	if l, ok := node.Attrs["level"]; ok {
		if lf, ok := l.(float64); ok {
			level = int(lf)
		}
	}

	switch level {
	case 1:
		return theme.StyleHeading.Render("═ " + text + " ═")
	case 2:
		return theme.StyleHeading.Render("─ " + text + " ─")
	case 3:
		return theme.StyleHeading.Render("▸ " + text)
	default:
		return theme.StyleHeading.Render("  " + text)
	}
}

// renderBulletList renders an unordered list.
func renderBulletList(node Node, width, indent int) string {
	var items []string
	for _, item := range node.Content {
		if item.Type == "listItem" {
			prefix := strings.Repeat("  ", indent) + theme.StyleBullet.Render("•") + " "
			prefixWidth := lipgloss.Width(prefix)
			rendered := renderListItemContent(item, width, indent+1, prefixWidth)
			items = append(items, prefix+rendered)
		}
	}
	return strings.Join(items, "\n")
}

// renderOrderedList renders a numbered list.
func renderOrderedList(node Node, width, indent int) string {
	var items []string
	for i, item := range node.Content {
		if item.Type == "listItem" {
			num := i + 1
			if o, ok := node.Attrs["order"]; ok {
				if of, ok := o.(float64); ok {
					num = int(of) + i
				}
			}
			prefix := strings.Repeat("  ", indent) + theme.StyleBullet.Render(fmt.Sprintf("%d.", num)) + " "
			prefixWidth := lipgloss.Width(prefix)
			rendered := renderListItemContent(item, width, indent+1, prefixWidth)
			items = append(items, prefix+rendered)
		}
	}
	return strings.Join(items, "\n")
}

// renderListItemContent renders the contents of a list item (may contain nested lists).
// prefixWidth is the visual width of the parent's bullet/number prefix, used to
// wrap paragraph text and align continuation lines.
func renderListItemContent(node Node, width, indent, prefixWidth int) string {
	paragraphWidth := width - prefixWidth
	if paragraphWidth < 10 {
		paragraphWidth = 10
	}

	var parts []string
	for _, child := range node.Content {
		switch child.Type {
		case "paragraph":
			text := renderInlineChildren(child.Content)
			if paragraphWidth > 0 {
				text = theme.WrapStyledText(text, paragraphWidth)
				// Indent continuation lines to align with first line's content.
				lines := strings.Split(text, "\n")
				if len(lines) > 1 {
					hangIndent := strings.Repeat(" ", prefixWidth)
					for i := 1; i < len(lines); i++ {
						lines[i] = hangIndent + lines[i]
					}
					text = strings.Join(lines, "\n")
				}
			}
			parts = append(parts, text)
		case "bulletList":
			parts = append(parts, renderBulletList(child, width, indent))
		case "orderedList":
			parts = append(parts, renderOrderedList(child, width, indent))
		default:
			parts = append(parts, renderBlock(child, width, indent))
		}
	}
	return strings.Join(parts, "\n")
}

// renderCodeBlock renders a fenced code block.
func renderCodeBlock(node Node) string {
	var b strings.Builder

	lang := ""
	if l, ok := node.Attrs["language"]; ok {
		if ls, ok := l.(string); ok {
			lang = ls
		}
	}

	if lang != "" {
		b.WriteString(theme.StyleSubtle.Render("── " + lang + " ──"))
		b.WriteString("\n")
	}

	// Code block children are text nodes.
	for _, child := range node.Content {
		if child.Type == "text" {
			b.WriteString(theme.StyleCodeBlock.Render(child.Text))
		}
	}

	return b.String()
}

// renderBlockquote renders a quoted block with border.
func renderBlockquote(node Node, width int) string {
	var lines []string
	for _, child := range node.Content {
		rendered := renderBlock(child, width-4, 0)
		for _, line := range strings.Split(rendered, "\n") {
			lines = append(lines, theme.StyleBlockquote.Render("│ "+line))
		}
	}
	return strings.Join(lines, "\n")
}

// renderTable renders a table from ADF table/tableRow/tableCell/tableHeader nodes.
// Cell content is word-wrapped across multiple lines to avoid truncation.
func renderTable(node Node, width int) string {
	type tableRow struct {
		cells    []string
		isHeader bool
	}

	var rows []tableRow
	for _, row := range node.Content {
		if row.Type != "tableRow" {
			continue
		}
		var cells []string
		isHeader := false
		for _, cell := range row.Content {
			if cell.Type == "tableHeader" {
				isHeader = true
			}
			cellText := renderChildrenInline(cell)
			cells = append(cells, cellText)
		}
		rows = append(rows, tableRow{cells: cells, isHeader: isHeader})
	}

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
			if w := lipgloss.Width(cell); w > colWidths[i] {
				colWidths[i] = w
			}
		}
	}

	// Cap total width to fit terminal.
	needsWrap := false
	if width > 0 {
		// Overhead: numCols borders + 1, plus 2 padding per column.
		overhead := numCols + 1 + numCols*2
		available := width - overhead
		if available < numCols*4 {
			available = numCols * 4
		}

		totalContent := 0
		for _, w := range colWidths {
			totalContent += w
		}

		if totalContent > available {
			needsWrap = true
			// Proportionally scale columns to fit.
			allocated := 0
			for i := range colWidths {
				colWidths[i] = max(4, available*colWidths[i]/totalContent)
				allocated += colWidths[i]
			}
			// Distribute rounding remainder to the widest column so the table
			// doesn't clip the right border.
			if remainder := available - allocated; remainder > 0 {
				widest := 0
				for i := 1; i < len(colWidths); i++ {
					if colWidths[i] > colWidths[widest] {
						widest = i
					}
				}
				colWidths[widest] += remainder
			}
		}
	}

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(theme.ColourPrimary)

	var b strings.Builder
	for _, row := range rows {
		// Wrap each cell's content to its column width.
		cellLines := make([][]string, numCols)
		maxLines := 1
		for i := 0; i < numCols; i++ {
			cell := ""
			if i < len(row.cells) {
				cell = row.cells[i]
			}
			if needsWrap && lipgloss.Width(cell) > colWidths[i] {
				// Strip ANSI before wrapping to avoid splitting escape sequences.
				plain := stripAnsi(cell)
				cellLines[i] = wrapCellText(plain, colWidths[i])
			} else {
				cellLines[i] = []string{cell}
			}
			if len(cellLines[i]) > maxLines {
				maxLines = len(cellLines[i])
			}
		}

		// Render each line of the row.
		for lineIdx := 0; lineIdx < maxLines; lineIdx++ {
			b.WriteString("│")
			for i := 0; i < numCols; i++ {
				line := ""
				if lineIdx < len(cellLines[i]) {
					line = cellLines[i][lineIdx]
				}
				padded := padOrTruncate(line, colWidths[i])
				if row.isHeader {
					b.WriteString(" " + headerStyle.Render(padded) + " │")
				} else {
					b.WriteString(" " + padded + " │")
				}
			}
			b.WriteString("\n")
		}

		if row.isHeader {
			b.WriteString("├")
			for i := 0; i < numCols; i++ {
				b.WriteString(strings.Repeat("─", colWidths[i]+2))
				if i < numCols-1 {
					b.WriteString("┼")
				}
			}
			b.WriteString("┤\n")
		} else if needsWrap && maxLines > 1 {
			// Light separator between multi-line data rows for readability.
			b.WriteString("│")
			for i := 0; i < numCols; i++ {
				b.WriteString(strings.Repeat("·", colWidths[i]+2))
				if i < numCols-1 {
					b.WriteString("│")
				}
			}
			b.WriteString("│\n")
		}
	}

	return strings.TrimRight(b.String(), "\n")
}

// renderPanel renders a panel (info, warning, etc.) block.
func renderPanel(node Node, width int) string {
	panelType := "info"
	if pt, ok := node.Attrs["panelType"]; ok {
		if pts, ok := pt.(string); ok {
			panelType = pts
		}
	}

	var icon string
	var borderColour lipgloss.AdaptiveColor
	switch panelType {
	case "info":
		icon = "i"
		borderColour = theme.ColourPrimary
	case "warning":
		icon = "!"
		borderColour = theme.ColourWarning
	case "error":
		icon = "✗"
		borderColour = theme.ColourError
	case "success", "tip":
		icon = ">"
		borderColour = theme.ColourSuccess
	case "note":
		icon = "*"
		borderColour = theme.ColourSubtle
	default:
		icon = "i"
		borderColour = theme.ColourSubtle
	}

	var b strings.Builder
	title := theme.StyleBold.Render(strings.ToUpper(panelType[:1]) + panelType[1:])
	fmt.Fprintf(&b, "[%s] %s\n", icon, title)

	for _, child := range node.Content {
		rendered := renderBlock(child, width-4, 0)
		b.WriteString(rendered)
		b.WriteString("\n")
	}

	panelWidth := width - 4
	if panelWidth < 20 {
		panelWidth = 40
	}
	panelStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColour).
		Padding(0, 1)
	return panelStyle.Width(panelWidth).Render(strings.TrimRight(b.String(), "\n"))
}

// renderRule renders a horizontal rule.
func renderRule(width int) string {
	ruleWidth := width
	if ruleWidth <= 0 {
		ruleWidth = 40
	}
	return theme.StyleHRule.Render(strings.Repeat("─", ruleWidth))
}

// renderMedia renders media nodes as placeholder text.
func renderMedia(node Node) string {
	// Walk into mediaSingle → media to find the filename/alt.
	for _, child := range node.Content {
		if child.Type == "media" {
			mediaType := ""
			if t, ok := child.Attrs["type"]; ok {
				if ts, ok := t.(string); ok {
					mediaType = ts
				}
			}

			name := ""
			// Try __fileName first (Confluence media), then alt.
			if fn, ok := child.Attrs["__fileName"]; ok {
				if fns, ok := fn.(string); ok && fns != "" {
					name = fns
				}
			}
			if name == "" {
				if a, ok := child.Attrs["alt"]; ok {
					if as, ok := a.(string); ok && as != "" {
						name = as
					}
				}
			}
			if name == "" {
				name = "attachment"
			}

			if mediaType == "file" {
				return theme.StyleImage.Render(fmt.Sprintf("[file: %s]", name))
			}
			return theme.StyleImage.Render(fmt.Sprintf("[image: %s]", name))
		}
	}
	return theme.StyleImage.Render("[media]")
}

// renderExpand renders an expand/collapsible section (always expanded in terminal).
func renderExpand(node Node, width, indent int) string {
	title := "Details"
	if t, ok := node.Attrs["title"]; ok {
		if ts, ok := t.(string); ok && ts != "" {
			title = ts
		}
	}

	var b strings.Builder
	b.WriteString(styleExpand.Render("▼ " + title))
	b.WriteString("\n")

	for _, child := range node.Content {
		rendered := renderBlock(child, width, indent)
		b.WriteString(rendered)
		b.WriteString("\n")
	}

	return strings.TrimRight(b.String(), "\n")
}

// renderTaskList renders a task list with checkboxes.
func renderTaskList(node Node, width, indent int) string {
	var items []string
	for _, item := range node.Content {
		if item.Type == "taskItem" {
			checked := false
			if s, ok := item.Attrs["state"]; ok {
				if ss, ok := s.(string); ok && ss == "DONE" {
					checked = true
				}
			}
			checkbox := "[ ]"
			if checked {
				checkbox = "[x]"
			}
			text := renderInlineChildren(item.Content)
			prefix := strings.Repeat("  ", indent) + theme.StyleBullet.Render(checkbox) + " "
			prefWidth := lipgloss.Width(prefix)
			textWidth := width - prefWidth
			if textWidth < 10 {
				textWidth = 10
			}
			if width > 0 {
				text = theme.WrapStyledText(text, textWidth)
				lines := strings.Split(text, "\n")
				if len(lines) > 1 {
					hangIndent := strings.Repeat(" ", prefWidth)
					for i := 1; i < len(lines); i++ {
						lines[i] = hangIndent + lines[i]
					}
					text = strings.Join(lines, "\n")
				}
			}
			items = append(items, prefix+text)
		}
	}
	return strings.Join(items, "\n")
}

// --- Inline rendering ---

// renderInlineChildren renders a list of inline nodes to a single styled string.
func renderInlineChildren(nodes []Node) string {
	var b strings.Builder
	for _, node := range nodes {
		b.WriteString(renderInline(node))
	}
	return b.String()
}

// renderInline renders a single inline node.
func renderInline(node Node) string {
	switch node.Type {
	case "text":
		return applyMarks(node.Text, node.Marks)
	case "mention":
		name := "@unknown"
		if t, ok := node.Attrs["text"]; ok {
			if ts, ok := t.(string); ok {
				name = ts
			}
		}
		return theme.UserStyle(strings.TrimPrefix(name, "@")).Render(name)
	case "emoji":
		shortName := ""
		if sn, ok := node.Attrs["shortName"]; ok {
			if sns, ok := sn.(string); ok {
				shortName = sns
			}
		}
		if shortName != "" {
			return styleEmoji.Render(shortName)
		}
		// Try the text or fallback attributes.
		if t, ok := node.Attrs["text"]; ok {
			if ts, ok := t.(string); ok {
				return ts
			}
		}
		return ""
	case "hardBreak":
		return "\n"
	case "inlineCard":
		url := ""
		if u, ok := node.Attrs["url"]; ok {
			if us, ok := u.(string); ok {
				url = us
			}
		}
		if url != "" {
			return theme.StyleLink.Render(url)
		}
		return ""
	case "mediaInline":
		name := "file"
		if fn, ok := node.Attrs["__fileName"]; ok {
			if fns, ok := fn.(string); ok && fns != "" {
				name = fns
			}
		}
		return theme.StyleImage.Render(fmt.Sprintf("[file: %s]", name))
	case "status":
		text := ""
		if t, ok := node.Attrs["text"]; ok {
			if ts, ok := t.(string); ok {
				text = ts
			}
		}
		if text != "" {
			return theme.StatusStyle(text).Render(text)
		}
		return ""
	default:
		// Unknown inline — try rendering children.
		if len(node.Content) > 0 {
			return renderInlineChildren(node.Content)
		}
		return node.Text
	}
}

// applyMarks applies formatting marks to text.
func applyMarks(text string, marks []Mark) string {
	result := text
	for _, mark := range marks {
		switch mark.Type {
		case "strong":
			result = theme.StyleBold.Render(result)
		case "em":
			result = theme.StyleItalic.Render(result)
		case "underline":
			result = theme.StyleUnderline.Render(result)
		case "strike":
			result = theme.StyleStrikethrough.Render(result)
		case "code":
			result = theme.StyleCode.Render(result)
		case "link":
			url := ""
			if href, ok := mark.Attrs["href"]; ok {
				if hs, ok := href.(string); ok {
					url = hs
				}
			}
			if url != "" && url != text {
				result = theme.StyleLink.Render(text) + " " + theme.StyleLinkURL.Render("("+url+")")
			} else {
				result = theme.StyleLink.Render(text)
			}
		case "textColor":
			if colour, ok := mark.Attrs["color"]; ok {
				if cs, ok := colour.(string); ok {
					result = lipgloss.NewStyle().Foreground(lipgloss.Color(cs)).Render(result)
				}
			}
		case "subsup":
			if sub, ok := mark.Attrs["type"]; ok {
				if ss, ok := sub.(string); ok && ss == "sub" {
					result = lipgloss.NewStyle().Foreground(theme.ColourSubtle).Render("_" + result)
				} else {
					result = lipgloss.NewStyle().Foreground(theme.ColourSubtle).Render("^" + result)
				}
			}
		}
	}
	return result
}

// InlineComment holds the data needed to render an inline comment within ADF.
type InlineComment struct {
	Author  string // Display name (already resolved).
	BodyADF string // ADF JSON string for the comment body.
	Status  string // "open", "resolved", etc.
}

// CommentPlacement records where an inline comment was rendered.
type CommentPlacement struct {
	Placed map[string]bool // IDs of comments rendered within the body.
	Lines  []int           // Line offsets (0-based) of each placed comment block.
}

// RenderWithComments works like Render but displays inline comments at the
// point in the document where they are annotated. It returns the rendered
// string and placement info (IDs placed inline, plus their line offsets for
// keyboard navigation).
func RenderWithComments(adfJSON string, width int, comments map[string]InlineComment) (string, CommentPlacement) {
	cp := CommentPlacement{Placed: make(map[string]bool)}
	if adfJSON == "" || len(comments) == 0 {
		return Render(adfJSON, width), cp
	}

	var doc Document
	if err := json.Unmarshal([]byte(adfJSON), &doc); err != nil {
		return "", cp
	}
	if doc.Type != "doc" || len(doc.Content) == 0 {
		return "", cp
	}

	var blocks []string
	// lineCount tracks the 0-based line index of the next line after the last block.
	// blocks.Join("\n\n") inserts one empty line between blocks, so each separator
	// advances lineCount by 1 (the empty line), not 2.
	lineCount := 0
	for _, node := range doc.Content {
		rendered := renderBlock(node, width, 0)
		if rendered != "" {
			if len(blocks) > 0 {
				lineCount++ // "\n\n" separator = 1 empty line
			}
			lineCount += strings.Count(rendered, "\n") + 1
			blocks = append(blocks, rendered)
		}

		// Collect annotation IDs from this block's inline nodes.
		ids := collectAnnotationIDs(node)
		for _, id := range ids {
			c, ok := comments[id]
			if !ok || cp.Placed[id] {
				continue
			}
			cp.Placed[id] = true
			commentBlock := renderInlineCommentBlock(c, width)
			if len(blocks) > 0 {
				lineCount++ // "\n\n" separator = 1 empty line
			}
			cp.Lines = append(cp.Lines, lineCount)
			lineCount += strings.Count(commentBlock, "\n") + 1
			blocks = append(blocks, commentBlock)
		}
	}

	return strings.Join(blocks, "\n\n"), cp
}

// renderInlineCommentBlock renders a single inline comment as a styled block.
func renderInlineCommentBlock(c InlineComment, width int) string {
	var b strings.Builder

	// Comment header: author · status
	var meta []string
	if c.Author != "" {
		meta = append(meta, theme.UserStyle(c.Author).Bold(true).Render(c.Author))
	}
	if c.Status != "" {
		switch strings.ToLower(c.Status) {
		case "resolved":
			meta = append(meta, lipgloss.NewStyle().Foreground(theme.ColourSuccess).Render("✓ resolved"))
		case "open", "reopened":
			meta = append(meta, lipgloss.NewStyle().Foreground(theme.ColourWarning).Render("○ "+strings.ToLower(c.Status)))
		default:
			meta = append(meta, lipgloss.NewStyle().Foreground(theme.ColourSubtle).Render(c.Status))
		}
	}
	header := strings.Join(meta, lipgloss.NewStyle().Foreground(theme.ColourSubtle).Render(" · "))

	// Comment body (render ADF, indented).
	bodyWidth := width - 6
	if bodyWidth < 20 {
		bodyWidth = 20
	}
	body := Render(c.BodyADF, bodyWidth)

	// Assemble: "  💬 author · status\n  body"
	marker := lipgloss.NewStyle().Foreground(theme.ColourSubtle).Render("  ▸ ")
	b.WriteString(marker)
	b.WriteString(header)
	if body != "" {
		b.WriteString("\n")
		for line := range strings.SplitSeq(body, "\n") {
			b.WriteString("    ")
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	return strings.TrimRight(b.String(), "\n")
}

// collectAnnotationIDs recursively extracts inline comment annotation IDs from a node tree.
func collectAnnotationIDs(node Node) []string {
	var ids []string
	seen := make(map[string]bool)

	var walk func(n Node)
	walk = func(n Node) {
		for _, mark := range n.Marks {
			if mark.Type == "annotation" {
				if at, ok := mark.Attrs["annotationType"]; ok {
					if ats, ok := at.(string); ok && ats == "inlineComment" {
						if id, ok := mark.Attrs["id"]; ok {
							if ids2, ok := id.(string); ok && !seen[ids2] {
								seen[ids2] = true
								ids = append(ids, ids2)
							}
						}
					}
				}
			}
		}
		for _, child := range n.Content {
			walk(child)
		}
	}
	walk(node)
	return ids
}

// --- Helper functions ---

// renderChildrenAsBlocks renders node children as block elements.
func renderChildrenAsBlocks(node Node, width, indent int) string {
	var blocks []string
	for _, child := range node.Content {
		rendered := renderBlock(child, width, indent)
		if rendered != "" {
			blocks = append(blocks, rendered)
		}
	}
	return strings.Join(blocks, "\n\n")
}

// renderChildrenInline renders all content of a node as inline text.
// Used for table cells where we want everything on one line.
func renderChildrenInline(node Node) string {
	var parts []string
	for _, child := range node.Content {
		if child.Type == "paragraph" {
			parts = append(parts, renderInlineChildren(child.Content))
		} else if len(child.Content) > 0 {
			parts = append(parts, renderInlineChildren(child.Content))
		} else {
			parts = append(parts, renderInline(child))
		}
	}
	return strings.Join(parts, " ")
}

var issueKeyRe = regexp.MustCompile(`(?:^|[^A-Z0-9-])([A-Z][A-Z][A-Z0-9]*-[0-9]{2,})(?:[^A-Z0-9-]|$)`)
var wikiPageRe = regexp.MustCompile(`/wiki/spaces/[^/]+/pages/(\d+)`)

// PageRef represents a reference to a Confluence page found in ADF content.
type PageRef struct {
	ID    string
	Title string // from link text or URL context
}

// ExtractPageRefs returns all unique Confluence page references found in an ADF document.
func ExtractPageRefs(adfJSON string) []PageRef {
	if adfJSON == "" {
		return nil
	}

	var doc Document
	if err := json.Unmarshal([]byte(adfJSON), &doc); err != nil {
		return nil
	}

	seen := make(map[string]bool)
	var refs []PageRef
	collectPageRefs(&refs, seen, doc.Content)
	return refs
}

// collectPageRefs recursively extracts page references from ADF nodes.
func collectPageRefs(refs *[]PageRef, seen map[string]bool, nodes []Node) {
	for _, node := range nodes {
		// Check inlineCard URLs.
		if node.Type == "inlineCard" {
			if url, ok := node.Attrs["url"]; ok {
				if us, ok := url.(string); ok {
					if m := wikiPageRe.FindStringSubmatch(us); len(m) > 1 && !seen[m[1]] {
						seen[m[1]] = true
						*refs = append(*refs, PageRef{ID: m[1], Title: us})
					}
				}
			}
		}

		// Check link marks on text nodes.
		for _, mark := range node.Marks {
			if mark.Type == "link" {
				if href, ok := mark.Attrs["href"]; ok {
					if hs, ok := href.(string); ok {
						if m := wikiPageRe.FindStringSubmatch(hs); len(m) > 1 && !seen[m[1]] {
							seen[m[1]] = true
							title := node.Text
							if title == "" {
								title = hs
							}
							*refs = append(*refs, PageRef{ID: m[1], Title: title})
						}
					}
				}
			}
		}

		// Check raw text for wiki URLs.
		if node.Text != "" {
			for _, m := range wikiPageRe.FindAllStringSubmatch(node.Text, -1) {
				if len(m) > 1 && !seen[m[1]] {
					seen[m[1]] = true
					*refs = append(*refs, PageRef{ID: m[1], Title: m[0]})
				}
			}
		}

		collectPageRefs(refs, seen, node.Content)
	}
}

// ExtractIssueKeys returns all unique Jira issue keys found in an ADF document.
func ExtractIssueKeys(adfJSON string) []string {
	if adfJSON == "" {
		return nil
	}

	var doc Document
	if err := json.Unmarshal([]byte(adfJSON), &doc); err != nil {
		return nil
	}

	var b strings.Builder
	collectText(&b, doc.Content)

	matches := issueKeyRe.FindAllStringSubmatch(b.String(), -1)
	seen := make(map[string]bool)
	var result []string
	for _, m := range matches {
		key := m[1]
		if !seen[key] {
			seen[key] = true
			result = append(result, key)
		}
	}
	return result
}

// collectText recursively extracts plain text and URL/text attributes from ADF nodes,
// skipping code blocks and inline code where technical strings (e.g. UTF-8) live.
func collectText(b *strings.Builder, nodes []Node) {
	for _, node := range nodes {
		// Skip code contexts — they contain technical strings that look like issue keys.
		switch node.Type {
		case "codeBlock", "code", "codeInline":
			continue
		}
		if node.Text != "" {
			b.WriteString(node.Text)
			b.WriteString(" ")
		}
		if url, ok := node.Attrs["url"]; ok {
			if us, ok := url.(string); ok {
				b.WriteString(us)
				b.WriteString(" ")
			}
		}
		if text, ok := node.Attrs["text"]; ok {
			if ts, ok := text.(string); ok {
				b.WriteString(ts)
				b.WriteString(" ")
			}
		}
		collectText(b, node.Content)
	}
}

// wrapCellText wraps plain text to width, breaking long words at character
// boundaries so nothing is truncated. Used for table cells.
func wrapCellText(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}

	var lines []string
	for _, line := range strings.Split(text, "\n") {
		words := strings.Fields(line)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}

		current := ""
		currentW := 0
		for _, word := range words {
			wordW := lipgloss.Width(word)

			// Break words wider than the column at character boundaries.
			for wordW > width {
				runes := []rune(word)
				// Find the break point that fits within width.
				cut := 0
				cutW := 0
				for _, r := range runes {
					rw := lipgloss.Width(string(r))
					if cutW+rw > width {
						break
					}
					cut++
					cutW += rw
				}
				if cut == 0 {
					cut = 1 // At least one rune per line.
				}
				chunk := string(runes[:cut])
				if current != "" {
					lines = append(lines, current)
					current = ""
					currentW = 0
				}
				lines = append(lines, chunk)
				word = string(runes[cut:])
				wordW = lipgloss.Width(word)
			}

			if wordW == 0 {
				continue
			}

			if current == "" {
				current = word
				currentW = wordW
			} else if currentW+1+wordW <= width {
				current += " " + word
				currentW += 1 + wordW
			} else {
				lines = append(lines, current)
				current = word
				currentW = wordW
			}
		}
		if current != "" {
			lines = append(lines, current)
		}
	}

	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}

// padOrTruncate pads a string with spaces or truncates it to fit width.
// Handles ANSI escape sequences and multi-byte characters safely by
// stripping styles and truncating the plain text, then re-styling.
func padOrTruncate(s string, width int) string {
	w := lipgloss.Width(s)
	if w > width && width > 3 {
		// Strip ANSI, truncate plain text, then style subtle to indicate truncation.
		plain := stripAnsi(s)
		runes := []rune(plain)
		if len(runes) > width-1 {
			return string(runes[:width-1]) + "…"
		}
		return string(runes)
	}
	if w < width {
		return s + strings.Repeat(" ", width-w)
	}
	return s
}

// stripAnsi removes ANSI escape sequences from a string.
func stripAnsi(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		if r == '\033' {
			inEsc = true
			continue
		}
		if inEsc {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
