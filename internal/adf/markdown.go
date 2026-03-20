package adf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

// --- ADF → Markdown ---

// ToMarkdown converts an ADF JSON string to CommonMark markdown.
// Returns empty string (not error) for empty or malformed input.
func ToMarkdown(adfJSON string) (string, error) {
	if adfJSON == "" {
		return "", nil
	}

	var doc Document
	if err := json.Unmarshal([]byte(adfJSON), &doc); err != nil {
		return "", fmt.Errorf("parsing ADF: %w", err)
	}

	if doc.Type != "doc" || len(doc.Content) == 0 {
		return "", nil
	}

	var buf strings.Builder
	for i, node := range doc.Content {
		if i > 0 {
			buf.WriteString("\n")
		}
		renderBlockToMarkdown(&buf, node, 0)
	}
	return buf.String(), nil
}

func renderBlockToMarkdown(buf *strings.Builder, node Node, indent int) {
	prefix := strings.Repeat("  ", indent)

	switch node.Type {
	case "paragraph":
		buf.WriteString(prefix)
		renderInlineToMarkdown(buf, node.Content)
		buf.WriteString("\n")

	case "heading":
		level := 1
		if l, ok := node.Attrs["level"]; ok {
			if lf, ok := l.(float64); ok {
				level = int(lf)
			}
		}
		buf.WriteString(prefix)
		buf.WriteString(strings.Repeat("#", level))
		buf.WriteString(" ")
		renderInlineToMarkdown(buf, node.Content)
		buf.WriteString("\n")

	case "bulletList":
		for _, item := range node.Content {
			if item.Type == "listItem" {
				renderListItem(buf, item, indent, "- ")
			}
		}

	case "orderedList":
		for i, item := range node.Content {
			if item.Type == "listItem" {
				renderListItem(buf, item, indent, fmt.Sprintf("%d. ", i+1))
			}
		}

	case "taskList":
		for _, item := range node.Content {
			if item.Type == "taskItem" {
				checked := false
				if s, ok := item.Attrs["state"]; ok && s == "DONE" {
					checked = true
				}
				marker := "- [ ] "
				if checked {
					marker = "- [x] "
				}
				renderListItem(buf, item, indent, marker)
			}
		}

	case "codeBlock":
		lang := ""
		if l, ok := node.Attrs["language"]; ok {
			if ls, ok := l.(string); ok {
				lang = ls
			}
		}
		buf.WriteString(prefix)
		buf.WriteString("```")
		buf.WriteString(lang)
		buf.WriteString("\n")
		for _, child := range node.Content {
			if child.Type == "text" {
				buf.WriteString(child.Text)
			}
		}
		// Ensure code block content ends with newline.
		if s := buf.String(); len(s) > 0 && s[len(s)-1] != '\n' {
			buf.WriteString("\n")
		}
		buf.WriteString(prefix)
		buf.WriteString("```\n")

	case "blockquote":
		var inner strings.Builder
		for i, child := range node.Content {
			if i > 0 {
				inner.WriteString("\n")
			}
			renderBlockToMarkdown(&inner, child, 0)
		}
		for _, line := range strings.Split(strings.TrimRight(inner.String(), "\n"), "\n") {
			buf.WriteString(prefix)
			buf.WriteString("> ")
			buf.WriteString(line)
			buf.WriteString("\n")
		}

	case "panel":
		panelType := "note"
		if t, ok := node.Attrs["panelType"]; ok {
			if ts, ok := t.(string); ok {
				panelType = ts
			}
		}
		// Render as admonition-style blockquote.
		var inner strings.Builder
		for i, child := range node.Content {
			if i > 0 {
				inner.WriteString("\n")
			}
			renderBlockToMarkdown(&inner, child, 0)
		}
		lines := strings.Split(strings.TrimRight(inner.String(), "\n"), "\n")
		for i, line := range lines {
			buf.WriteString(prefix)
			buf.WriteString("> ")
			if i == 0 {
				buf.WriteString("**")
				buf.WriteString(strings.ToUpper(panelType[:1]) + panelType[1:])
				buf.WriteString(":** ")
			}
			buf.WriteString(line)
			buf.WriteString("\n")
		}

	case "rule":
		buf.WriteString(prefix)
		buf.WriteString("---\n")

	case "table":
		renderTableToMarkdown(buf, node)

	case "mediaSingle", "mediaGroup":
		for _, child := range node.Content {
			renderMediaToMarkdown(buf, child, prefix)
		}

	case "expand", "nestedExpand":
		title := ""
		if t, ok := node.Attrs["title"]; ok {
			if ts, ok := t.(string); ok {
				title = ts
			}
		}
		buf.WriteString(prefix)
		buf.WriteString("<details>\n")
		buf.WriteString(prefix)
		buf.WriteString("<summary>")
		buf.WriteString(title)
		buf.WriteString("</summary>\n\n")
		for _, child := range node.Content {
			renderBlockToMarkdown(buf, child, indent)
		}
		buf.WriteString(prefix)
		buf.WriteString("</details>\n")

	default:
		// Unknown block — render content as paragraph fallback.
		if len(node.Content) > 0 {
			buf.WriteString(prefix)
			renderInlineToMarkdown(buf, node.Content)
			buf.WriteString("\n")
		}
	}
}

func renderListItem(buf *strings.Builder, item Node, indent int, marker string) {
	prefix := strings.Repeat("  ", indent)
	for i, child := range item.Content {
		if i == 0 {
			// First block gets the marker.
			buf.WriteString(prefix)
			buf.WriteString(marker)
			if child.Type == "paragraph" {
				renderInlineToMarkdown(buf, child.Content)
				buf.WriteString("\n")
			} else {
				buf.WriteString("\n")
				renderBlockToMarkdown(buf, child, indent+1)
			}
		} else {
			// Subsequent blocks are indented continuation.
			renderBlockToMarkdown(buf, child, indent+1)
		}
	}
}

func renderTableToMarkdown(buf *strings.Builder, node Node) {
	var rows [][]string
	hasHeader := false

	for _, row := range node.Content {
		if row.Type != "tableRow" {
			continue
		}
		var cells []string
		for _, cell := range row.Content {
			if cell.Type == "tableHeader" {
				hasHeader = true
			}
			var cellBuf strings.Builder
			for _, child := range cell.Content {
				if child.Type == "paragraph" {
					renderInlineToMarkdown(&cellBuf, child.Content)
				}
			}
			cells = append(cells, strings.TrimSpace(cellBuf.String()))
		}
		rows = append(rows, cells)
	}

	if len(rows) == 0 {
		return
	}

	// Calculate column widths.
	colCount := 0
	for _, row := range rows {
		if len(row) > colCount {
			colCount = len(row)
		}
	}

	// Write rows.
	for i, row := range rows {
		buf.WriteString("| ")
		for j := 0; j < colCount; j++ {
			if j < len(row) {
				buf.WriteString(row[j])
			}
			buf.WriteString(" | ")
		}
		buf.WriteString("\n")

		// Write header separator after first row if it contains headers.
		if i == 0 && hasHeader {
			buf.WriteString("| ")
			for j := 0; j < colCount; j++ {
				buf.WriteString("---")
				buf.WriteString(" | ")
			}
			buf.WriteString("\n")
		}
	}

	// GFM tables require a header row; treat first row as header (no further action needed).
	_ = hasHeader
}

func renderMediaToMarkdown(buf *strings.Builder, node Node, prefix string) {
	if node.Type == "media" || node.Type == "mediaInline" {
		alt := ""
		url := ""
		if a, ok := node.Attrs["alt"]; ok {
			if as, ok := a.(string); ok {
				alt = as
			}
		}
		if u, ok := node.Attrs["url"]; ok {
			if us, ok := u.(string); ok {
				url = us
			}
		}
		if url == "" {
			// Attachment — use filename as placeholder.
			if fn, ok := node.Attrs["id"]; ok {
				if fns, ok := fn.(string); ok {
					buf.WriteString(prefix)
					fmt.Fprintf(buf, "[attachment: %s]\n", fns)
					return
				}
			}
		}
		buf.WriteString(prefix)
		fmt.Fprintf(buf, "![%s](%s)\n", alt, url)
	}
}

func renderInlineToMarkdown(buf *strings.Builder, nodes []Node) {
	for _, node := range nodes {
		switch node.Type {
		case "text":
			text := node.Text
			text = applyMarksMarkdown(text, node.Marks)
			buf.WriteString(text)

		case "mention":
			name := ""
			if t, ok := node.Attrs["text"]; ok {
				if ts, ok := t.(string); ok {
					name = ts
				}
			}
			if name == "" {
				name = node.Text
			}
			// Strip leading @ if present to avoid @@.
			name = strings.TrimPrefix(name, "@")
			buf.WriteString("@")
			buf.WriteString(name)

		case "emoji":
			if shortName, ok := node.Attrs["shortName"]; ok {
				if s, ok := shortName.(string); ok {
					buf.WriteString(s)
				}
			} else if t, ok := node.Attrs["text"]; ok {
				if ts, ok := t.(string); ok {
					buf.WriteString(ts)
				}
			}

		case "hardBreak":
			buf.WriteString("  \n")

		case "inlineCard":
			url := ""
			if u, ok := node.Attrs["url"]; ok {
				if us, ok := u.(string); ok {
					url = us
				}
			}
			buf.WriteString(url)

		case "status":
			text := ""
			if t, ok := node.Attrs["text"]; ok {
				if ts, ok := t.(string); ok {
					text = ts
				}
			}
			buf.WriteString("**[")
			buf.WriteString(text)
			buf.WriteString("]**")

		case "mediaInline":
			renderMediaToMarkdown(buf, node, "")

		default:
			// Recurse for unknown inline nodes with content.
			if len(node.Content) > 0 {
				renderInlineToMarkdown(buf, node.Content)
			}
		}
	}
}

func applyMarksMarkdown(text string, marks []Mark) string {
	for _, m := range marks {
		switch m.Type {
		case "strong":
			text = "**" + text + "**"
		case "em":
			text = "*" + text + "*"
		case "strike":
			text = "~~" + text + "~~"
		case "code":
			text = "`" + text + "`"
		case "link":
			href := ""
			if h, ok := m.Attrs["href"]; ok {
				if hs, ok := h.(string); ok {
					href = hs
				}
			}
			text = "[" + text + "](" + href + ")"
		case "underline":
			text = "<u>" + text + "</u>"
		case "subsup":
			if t, ok := m.Attrs["type"]; ok {
				if ts, ok := t.(string); ok {
					if ts == "sub" {
						text = "<sub>" + text + "</sub>"
					} else {
						text = "<sup>" + text + "</sup>"
					}
				}
			}
			// textColor — dropped (no markdown equivalent)
		}
	}
	return text
}

// --- Markdown → ADF ---

// FromMarkdown converts CommonMark markdown to an ADF JSON string.
func FromMarkdown(markdown string) (string, error) {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM, // tables, strikethrough, task lists
		),
	)

	source := []byte(markdown)
	reader := text.NewReader(source)
	tree := md.Parser().Parse(reader)

	doc := Document{
		Type:    "doc",
		Version: 1,
		Content: convertChildren(tree, source),
	}

	data, err := json.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("marshalling ADF: %w", err)
	}
	return string(data), nil
}

func convertChildren(parent ast.Node, source []byte) []Node {
	var nodes []Node
	for child := parent.FirstChild(); child != nil; child = child.NextSibling() {
		if n := convertNode(child, source); n != nil {
			nodes = append(nodes, *n)
		}
	}
	return nodes
}

func convertNode(n ast.Node, source []byte) *Node {
	switch node := n.(type) {
	case *ast.Paragraph:
		content := convertInlineChildren(node, source)
		if len(content) == 0 {
			return nil
		}
		return &Node{Type: "paragraph", Content: content}

	case *ast.TextBlock:
		// Tight list items use TextBlock instead of Paragraph. Treat identically.
		content := convertInlineChildren(node, source)
		if len(content) == 0 {
			return nil
		}
		return &Node{Type: "paragraph", Content: content}

	case *ast.Heading:
		return &Node{
			Type:    "heading",
			Attrs:   map[string]any{"level": float64(node.Level)},
			Content: convertInlineChildren(node, source),
		}

	case *ast.FencedCodeBlock:
		lang := ""
		if node.Language(source) != nil {
			lang = string(node.Language(source))
		}
		var textBuf bytes.Buffer
		lines := node.Lines()
		for i := 0; i < lines.Len(); i++ {
			seg := lines.At(i)
			textBuf.Write(seg.Value(source))
		}
		attrs := map[string]any{}
		if lang != "" {
			attrs["language"] = lang
		}
		return &Node{
			Type:  "codeBlock",
			Attrs: attrs,
			Content: []Node{
				{Type: "text", Text: textBuf.String()},
			},
		}

	case *ast.CodeBlock:
		var textBuf bytes.Buffer
		lines := node.Lines()
		for i := 0; i < lines.Len(); i++ {
			seg := lines.At(i)
			textBuf.Write(seg.Value(source))
		}
		return &Node{
			Type: "codeBlock",
			Content: []Node{
				{Type: "text", Text: textBuf.String()},
			},
		}

	case *ast.List:
		items := convertListItems(node, source)
		// If items are task items, wrap in taskList.
		if len(items) > 0 && items[0].Type == "taskItem" {
			return &Node{
				Type:    "taskList",
				Content: items,
			}
		}
		if node.IsOrdered() {
			return &Node{
				Type:    "orderedList",
				Content: items,
			}
		}
		return &Node{
			Type:    "bulletList",
			Content: items,
		}

	case *ast.ListItem:
		return &Node{
			Type:    "listItem",
			Content: convertChildren(node, source),
		}

	case *ast.Blockquote:
		return &Node{
			Type:    "blockquote",
			Content: convertChildren(node, source),
		}

	case *ast.ThematicBreak:
		return &Node{Type: "rule"}

	case *ast.HTMLBlock:
		// Try to detect <details>/<summary> for expand blocks.
		var textBuf bytes.Buffer
		lines := node.Lines()
		for i := 0; i < lines.Len(); i++ {
			seg := lines.At(i)
			textBuf.Write(seg.Value(source))
		}
		raw := textBuf.String()
		if strings.Contains(raw, "<details>") {
			return &Node{
				Type: "paragraph",
				Content: []Node{
					{Type: "text", Text: strings.TrimSpace(raw)},
				},
			}
		}
		return &Node{
			Type: "paragraph",
			Content: []Node{
				{Type: "text", Text: strings.TrimSpace(raw)},
			},
		}

	// GFM extensions
	case *east.Table:
		return convertTable(node, source)

	case *east.TaskCheckBox:
		// Handled within list item conversion.
		return nil

	default:
		// For unrecognised block nodes with children, try converting children.
		children := convertChildren(n, source)
		if len(children) > 0 {
			return &Node{Type: "paragraph", Content: children}
		}
		return nil
	}
}

func convertListItems(list *ast.List, source []byte) []Node {
	var items []Node
	for child := list.FirstChild(); child != nil; child = child.NextSibling() {
		if li, ok := child.(*ast.ListItem); ok {
			// Check for task list items.
			if isTaskItem(li) {
				checked := isTaskChecked(li, source)
				state := "TODO"
				if checked {
					state = "DONE"
				}
				// Get content, skipping the leading TaskCheckBox node.
				content := convertChildren(li, source)
				items = append(items, Node{
					Type:    "taskItem",
					Attrs:   map[string]any{"state": state},
					Content: content,
				})
			} else {
				items = append(items, Node{
					Type:    "listItem",
					Content: convertChildren(li, source),
				})
			}
		}
	}

	// If any task items found, the parent should be a taskList.
	// This is handled by checking at the list level.
	return items
}

func taskItemFirstBlock(li *ast.ListItem) ast.Node {
	first := li.FirstChild()
	if first == nil {
		return nil
	}
	switch first.(type) {
	case *ast.Paragraph, *ast.TextBlock:
		return first
	}
	return nil
}

func isTaskItem(li *ast.ListItem) bool {
	block := taskItemFirstBlock(li)
	if block == nil {
		return false
	}
	if tc := block.FirstChild(); tc != nil {
		if _, ok := tc.(*east.TaskCheckBox); ok {
			return true
		}
	}
	return false
}

func isTaskChecked(li *ast.ListItem, _ []byte) bool {
	block := taskItemFirstBlock(li)
	if block == nil {
		return false
	}
	if tc := block.FirstChild(); tc != nil {
		if cb, ok := tc.(*east.TaskCheckBox); ok {
			return cb.IsChecked
		}
	}
	return false
}

func convertTable(table *east.Table, source []byte) *Node {
	var rows []Node

	for child := table.FirstChild(); child != nil; child = child.NextSibling() {
		switch row := child.(type) {
		case *east.TableHeader:
			cells := convertTableCells(row, source, "tableHeader")
			rows = append(rows, Node{Type: "tableRow", Content: cells})
		case *east.TableRow:
			cells := convertTableCells(row, source, "tableCell")
			rows = append(rows, Node{Type: "tableRow", Content: cells})
		}
	}

	return &Node{Type: "table", Content: rows}
}

func convertTableCells(row ast.Node, source []byte, cellType string) []Node {
	var cells []Node
	for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
		content := convertInlineChildren(cell, source)
		cells = append(cells, Node{
			Type: cellType,
			Content: []Node{
				{Type: "paragraph", Content: content},
			},
		})
	}
	return cells
}

func convertInlineChildren(parent ast.Node, source []byte) []Node {
	var nodes []Node
	for child := parent.FirstChild(); child != nil; child = child.NextSibling() {
		inlines := convertInline(child, source)
		nodes = append(nodes, inlines...)
	}
	return nodes
}

func convertInline(n ast.Node, source []byte) []Node {
	switch node := n.(type) {
	case *ast.Text:
		t := string(node.Segment.Value(source))
		result := Node{Type: "text", Text: t}
		if node.SoftLineBreak() {
			// Soft breaks become spaces in ADF.
			result.Text = t + " "
		}
		if node.HardLineBreak() {
			return []Node{
				{Type: "text", Text: t},
				{Type: "hardBreak"},
			}
		}
		return []Node{result}

	case *ast.String:
		return []Node{{Type: "text", Text: string(node.Value)}}

	case *ast.CodeSpan:
		var textBuf bytes.Buffer
		for child := node.FirstChild(); child != nil; child = child.NextSibling() {
			if t, ok := child.(*ast.Text); ok {
				textBuf.Write(t.Segment.Value(source))
			}
		}
		return []Node{{
			Type:  "text",
			Text:  textBuf.String(),
			Marks: []Mark{{Type: "code"}},
		}}

	case *ast.Emphasis:
		children := convertInlineChildren(node, source)
		markType := "em"
		if node.Level == 2 {
			markType = "strong"
		}
		// Apply mark to all text children.
		return applyMarkToNodes(children, Mark{Type: markType})

	case *ast.Link:
		children := convertInlineChildren(node, source)
		mark := Mark{
			Type:  "link",
			Attrs: map[string]any{"href": string(node.Destination)},
		}
		return applyMarkToNodes(children, mark)

	case *ast.AutoLink:
		url := string(node.URL(source))
		return []Node{{
			Type:  "text",
			Text:  url,
			Marks: []Mark{{Type: "link", Attrs: map[string]any{"href": url}}},
		}}

	case *ast.Image:
		url := string(node.Destination)
		// Collect alt text from child text nodes (node.Text is deprecated).
		var altBuf strings.Builder
		for child := node.FirstChild(); child != nil; child = child.NextSibling() {
			if t, ok := child.(*ast.Text); ok {
				altBuf.Write(t.Value(source))
			}
		}
		alt := altBuf.String()
		return []Node{{
			Type: "mediaSingle",
			Content: []Node{{
				Type:  "media",
				Attrs: map[string]any{"url": url, "alt": alt, "type": "external"},
			}},
		}}

	case *ast.RawHTML:
		seg := node.Segments.At(0)
		raw := strings.TrimSpace(string(seg.Value(source)))
		// Handle <u>underline</u> and <sub>/<sup> — pass through as text.
		return []Node{{Type: "text", Text: raw}}

	case *east.Strikethrough:
		children := convertInlineChildren(node, source)
		return applyMarkToNodes(children, Mark{Type: "strike"})

	case *east.TaskCheckBox:
		// Skip — handled at the list item level.
		return nil

	default:
		// Recurse for unknown inline nodes.
		children := convertInlineChildren(n, source)
		if len(children) > 0 {
			return children
		}
		return nil
	}
}

// applyMarkToNodes adds a mark to all text nodes in the slice.
func applyMarkToNodes(nodes []Node, mark Mark) []Node {
	for i := range nodes {
		if nodes[i].Type == "text" {
			nodes[i].Marks = append(nodes[i].Marks, mark)
		}
	}
	return nodes
}
