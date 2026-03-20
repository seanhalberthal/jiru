package adf

import (
	"encoding/json"
	"strings"
	"testing"
)

// --- ToMarkdown tests ---

func TestToMarkdown_EmptyInput(t *testing.T) {
	md, err := ToMarkdown("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if md != "" {
		t.Errorf("expected empty string, got %q", md)
	}
}

func TestToMarkdown_MalformedJSON(t *testing.T) {
	_, err := ToMarkdown("{not json")
	if err == nil {
		t.Error("expected error for malformed JSON")
	}
}

func TestToMarkdown_Paragraph(t *testing.T) {
	adf := makeDoc(Node{
		Type:    "paragraph",
		Content: []Node{{Type: "text", Text: "Hello world"}},
	})
	md, err := ToMarkdown(adf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(md) != "Hello world" {
		t.Errorf("got %q, want %q", strings.TrimSpace(md), "Hello world")
	}
}

func TestToMarkdown_Heading(t *testing.T) {
	for _, tc := range []struct {
		level int
		want  string
	}{
		{1, "# Title"},
		{2, "## Title"},
		{3, "### Title"},
	} {
		adf := makeDoc(Node{
			Type:    "heading",
			Attrs:   map[string]any{"level": float64(tc.level)},
			Content: []Node{{Type: "text", Text: "Title"}},
		})
		md, _ := ToMarkdown(adf)
		if !strings.Contains(md, tc.want) {
			t.Errorf("level %d: got %q, want to contain %q", tc.level, md, tc.want)
		}
	}
}

func TestToMarkdown_Bold(t *testing.T) {
	adf := makeDoc(Node{
		Type: "paragraph",
		Content: []Node{{
			Type:  "text",
			Text:  "bold",
			Marks: []Mark{{Type: "strong"}},
		}},
	})
	md, _ := ToMarkdown(adf)
	if !strings.Contains(md, "**bold**") {
		t.Errorf("got %q, want to contain **bold**", md)
	}
}

func TestToMarkdown_Italic(t *testing.T) {
	adf := makeDoc(Node{
		Type: "paragraph",
		Content: []Node{{
			Type:  "text",
			Text:  "italic",
			Marks: []Mark{{Type: "em"}},
		}},
	})
	md, _ := ToMarkdown(adf)
	if !strings.Contains(md, "*italic*") {
		t.Errorf("got %q, want to contain *italic*", md)
	}
}

func TestToMarkdown_Strikethrough(t *testing.T) {
	adf := makeDoc(Node{
		Type: "paragraph",
		Content: []Node{{
			Type:  "text",
			Text:  "struck",
			Marks: []Mark{{Type: "strike"}},
		}},
	})
	md, _ := ToMarkdown(adf)
	if !strings.Contains(md, "~~struck~~") {
		t.Errorf("got %q, want to contain ~~struck~~", md)
	}
}

func TestToMarkdown_Code(t *testing.T) {
	adf := makeDoc(Node{
		Type: "paragraph",
		Content: []Node{{
			Type:  "text",
			Text:  "code",
			Marks: []Mark{{Type: "code"}},
		}},
	})
	md, _ := ToMarkdown(adf)
	if !strings.Contains(md, "`code`") {
		t.Errorf("got %q, want to contain `code`", md)
	}
}

func TestToMarkdown_Link(t *testing.T) {
	adf := makeDoc(Node{
		Type: "paragraph",
		Content: []Node{{
			Type:  "text",
			Text:  "click",
			Marks: []Mark{{Type: "link", Attrs: map[string]any{"href": "https://example.com"}}},
		}},
	})
	md, _ := ToMarkdown(adf)
	if !strings.Contains(md, "[click](https://example.com)") {
		t.Errorf("got %q, want to contain [click](https://example.com)", md)
	}
}

func TestToMarkdown_BulletList(t *testing.T) {
	adf := makeDoc(Node{
		Type: "bulletList",
		Content: []Node{
			{Type: "listItem", Content: []Node{
				{Type: "paragraph", Content: []Node{{Type: "text", Text: "one"}}},
			}},
			{Type: "listItem", Content: []Node{
				{Type: "paragraph", Content: []Node{{Type: "text", Text: "two"}}},
			}},
		},
	})
	md, _ := ToMarkdown(adf)
	if !strings.Contains(md, "- one") || !strings.Contains(md, "- two") {
		t.Errorf("got %q, want bullet list items", md)
	}
}

func TestToMarkdown_OrderedList(t *testing.T) {
	adf := makeDoc(Node{
		Type: "orderedList",
		Content: []Node{
			{Type: "listItem", Content: []Node{
				{Type: "paragraph", Content: []Node{{Type: "text", Text: "first"}}},
			}},
			{Type: "listItem", Content: []Node{
				{Type: "paragraph", Content: []Node{{Type: "text", Text: "second"}}},
			}},
		},
	})
	md, _ := ToMarkdown(adf)
	if !strings.Contains(md, "1. first") || !strings.Contains(md, "2. second") {
		t.Errorf("got %q, want ordered list items", md)
	}
}

func TestToMarkdown_CodeBlock(t *testing.T) {
	adf := makeDoc(Node{
		Type:  "codeBlock",
		Attrs: map[string]any{"language": "go"},
		Content: []Node{
			{Type: "text", Text: "fmt.Println(\"hello\")"},
		},
	})
	md, _ := ToMarkdown(adf)
	if !strings.Contains(md, "```go") {
		t.Errorf("got %q, want code block with go language", md)
	}
	if !strings.Contains(md, "fmt.Println") {
		t.Errorf("got %q, want code content", md)
	}
}

func TestToMarkdown_Blockquote(t *testing.T) {
	adf := makeDoc(Node{
		Type: "blockquote",
		Content: []Node{
			{Type: "paragraph", Content: []Node{{Type: "text", Text: "quoted"}}},
		},
	})
	md, _ := ToMarkdown(adf)
	if !strings.Contains(md, "> quoted") {
		t.Errorf("got %q, want blockquote", md)
	}
}

func TestToMarkdown_Rule(t *testing.T) {
	adf := makeDoc(Node{Type: "rule"})
	md, _ := ToMarkdown(adf)
	if !strings.Contains(md, "---") {
		t.Errorf("got %q, want horizontal rule", md)
	}
}

func TestToMarkdown_Table(t *testing.T) {
	adf := makeDoc(Node{
		Type: "table",
		Content: []Node{
			{Type: "tableRow", Content: []Node{
				{Type: "tableHeader", Content: []Node{
					{Type: "paragraph", Content: []Node{{Type: "text", Text: "Name"}}},
				}},
				{Type: "tableHeader", Content: []Node{
					{Type: "paragraph", Content: []Node{{Type: "text", Text: "Value"}}},
				}},
			}},
			{Type: "tableRow", Content: []Node{
				{Type: "tableCell", Content: []Node{
					{Type: "paragraph", Content: []Node{{Type: "text", Text: "foo"}}},
				}},
				{Type: "tableCell", Content: []Node{
					{Type: "paragraph", Content: []Node{{Type: "text", Text: "bar"}}},
				}},
			}},
		},
	})
	md, _ := ToMarkdown(adf)
	if !strings.Contains(md, "| Name") || !strings.Contains(md, "| foo") {
		t.Errorf("got %q, want table", md)
	}
	if !strings.Contains(md, "---") {
		t.Errorf("got %q, want header separator", md)
	}
}

func TestToMarkdown_Panel(t *testing.T) {
	adf := makeDoc(Node{
		Type:  "panel",
		Attrs: map[string]any{"panelType": "info"},
		Content: []Node{
			{Type: "paragraph", Content: []Node{{Type: "text", Text: "important"}}},
		},
	})
	md, _ := ToMarkdown(adf)
	if !strings.Contains(md, "**Info:**") {
		t.Errorf("got %q, want panel with Info label", md)
	}
	if !strings.Contains(md, "> ") {
		t.Errorf("got %q, want blockquote-style panel", md)
	}
}

func TestToMarkdown_TaskList(t *testing.T) {
	adf := makeDoc(Node{
		Type: "taskList",
		Content: []Node{
			{Type: "taskItem", Attrs: map[string]any{"state": "TODO"}, Content: []Node{
				{Type: "paragraph", Content: []Node{{Type: "text", Text: "unchecked"}}},
			}},
			{Type: "taskItem", Attrs: map[string]any{"state": "DONE"}, Content: []Node{
				{Type: "paragraph", Content: []Node{{Type: "text", Text: "checked"}}},
			}},
		},
	})
	md, _ := ToMarkdown(adf)
	if !strings.Contains(md, "- [ ] unchecked") {
		t.Errorf("got %q, want unchecked task", md)
	}
	if !strings.Contains(md, "- [x] checked") {
		t.Errorf("got %q, want checked task", md)
	}
}

func TestToMarkdown_Mention(t *testing.T) {
	adf := makeDoc(Node{
		Type: "paragraph",
		Content: []Node{
			{Type: "mention", Attrs: map[string]any{"text": "@jane"}},
		},
	})
	md, _ := ToMarkdown(adf)
	if !strings.Contains(md, "@jane") {
		t.Errorf("got %q, want @jane", md)
	}
}

func TestToMarkdown_HardBreak(t *testing.T) {
	adf := makeDoc(Node{
		Type: "paragraph",
		Content: []Node{
			{Type: "text", Text: "line1"},
			{Type: "hardBreak"},
			{Type: "text", Text: "line2"},
		},
	})
	md, _ := ToMarkdown(adf)
	if !strings.Contains(md, "  \n") {
		t.Errorf("got %q, want hard break (two trailing spaces + newline)", md)
	}
}

func TestToMarkdown_Underline(t *testing.T) {
	adf := makeDoc(Node{
		Type: "paragraph",
		Content: []Node{{
			Type:  "text",
			Text:  "underlined",
			Marks: []Mark{{Type: "underline"}},
		}},
	})
	md, _ := ToMarkdown(adf)
	if !strings.Contains(md, "<u>underlined</u>") {
		t.Errorf("got %q, want <u>underlined</u>", md)
	}
}

func TestToMarkdown_Status(t *testing.T) {
	adf := makeDoc(Node{
		Type: "paragraph",
		Content: []Node{
			{Type: "status", Attrs: map[string]any{"text": "IN PROGRESS"}},
		},
	})
	md, _ := ToMarkdown(adf)
	if !strings.Contains(md, "**[IN PROGRESS]**") {
		t.Errorf("got %q, want **[IN PROGRESS]**", md)
	}
}

// --- FromMarkdown tests ---

func TestFromMarkdown_EmptyInput(t *testing.T) {
	result, err := FromMarkdown("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var doc Document
	if err := json.Unmarshal([]byte(result), &doc); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if doc.Type != "doc" {
		t.Errorf("type = %q, want 'doc'", doc.Type)
	}
}

func TestFromMarkdown_Paragraph(t *testing.T) {
	result, _ := FromMarkdown("Hello world")
	doc := parseDoc(t, result)
	if len(doc.Content) == 0 {
		t.Fatal("expected at least one node")
	}
	if doc.Content[0].Type != "paragraph" {
		t.Errorf("type = %q, want 'paragraph'", doc.Content[0].Type)
	}
}

func TestFromMarkdown_Heading(t *testing.T) {
	result, _ := FromMarkdown("## My Heading")
	doc := parseDoc(t, result)
	found := findNode(doc.Content, "heading")
	if found == nil {
		t.Fatal("expected heading node")
	}
	if level, ok := found.Attrs["level"]; !ok || level != float64(2) {
		t.Errorf("heading level = %v, want 2", level)
	}
}

func TestFromMarkdown_Bold(t *testing.T) {
	result, _ := FromMarkdown("**bold text**")
	doc := parseDoc(t, result)
	text := findTextNode(doc.Content)
	if text == nil {
		t.Fatal("expected text node")
	}
	if !hasMark(text.Marks, "strong") {
		t.Error("expected strong mark on text")
	}
}

func TestFromMarkdown_Italic(t *testing.T) {
	result, _ := FromMarkdown("*italic text*")
	doc := parseDoc(t, result)
	text := findTextNode(doc.Content)
	if text == nil {
		t.Fatal("expected text node")
	}
	if !hasMark(text.Marks, "em") {
		t.Error("expected em mark on text")
	}
}

func TestFromMarkdown_CodeSpan(t *testing.T) {
	result, _ := FromMarkdown("Use `code` here")
	doc := parseDoc(t, result)
	text := findTextWithMark(doc.Content, "code")
	if text == nil {
		t.Fatal("expected text node with code mark")
	}
}

func TestFromMarkdown_Link(t *testing.T) {
	result, _ := FromMarkdown("[click](https://example.com)")
	doc := parseDoc(t, result)
	text := findTextWithMark(doc.Content, "link")
	if text == nil {
		t.Fatal("expected text node with link mark")
	}
}

func TestFromMarkdown_BulletList(t *testing.T) {
	result, _ := FromMarkdown("- one\n- two\n")
	doc := parseDoc(t, result)
	found := findNode(doc.Content, "bulletList")
	if found == nil {
		t.Fatal("expected bulletList node")
	}
	if len(found.Content) != 2 {
		t.Errorf("expected 2 list items, got %d", len(found.Content))
	}
}

func TestFromMarkdown_OrderedList(t *testing.T) {
	result, _ := FromMarkdown("1. first\n2. second\n")
	doc := parseDoc(t, result)
	found := findNode(doc.Content, "orderedList")
	if found == nil {
		t.Fatal("expected orderedList node")
	}
}

func TestFromMarkdown_CodeBlock(t *testing.T) {
	result, _ := FromMarkdown("```go\nfmt.Println(\"hello\")\n```\n")
	doc := parseDoc(t, result)
	found := findNode(doc.Content, "codeBlock")
	if found == nil {
		t.Fatal("expected codeBlock node")
	}
	if lang, ok := found.Attrs["language"]; !ok || lang != "go" {
		t.Errorf("language = %v, want 'go'", lang)
	}
}

func TestFromMarkdown_Blockquote(t *testing.T) {
	result, _ := FromMarkdown("> quoted text\n")
	doc := parseDoc(t, result)
	found := findNode(doc.Content, "blockquote")
	if found == nil {
		t.Fatal("expected blockquote node")
	}
}

func TestFromMarkdown_ThematicBreak(t *testing.T) {
	result, _ := FromMarkdown("---\n")
	doc := parseDoc(t, result)
	found := findNode(doc.Content, "rule")
	if found == nil {
		t.Fatal("expected rule node")
	}
}

func TestFromMarkdown_Table(t *testing.T) {
	md := "| A | B |\n| --- | --- |\n| 1 | 2 |\n"
	result, _ := FromMarkdown(md)
	doc := parseDoc(t, result)
	found := findNode(doc.Content, "table")
	if found == nil {
		t.Fatal("expected table node")
	}
	if len(found.Content) < 2 {
		t.Errorf("expected at least 2 table rows (header + data), got %d", len(found.Content))
	}
}

func TestFromMarkdown_Strikethrough(t *testing.T) {
	result, _ := FromMarkdown("~~struck~~")
	doc := parseDoc(t, result)
	text := findTextWithMark(doc.Content, "strike")
	if text == nil {
		t.Fatal("expected text node with strike mark")
	}
}

func TestFromMarkdown_TaskList(t *testing.T) {
	result, _ := FromMarkdown("- [ ] todo\n- [x] done\n")
	doc := parseDoc(t, result)
	// GFM task lists are rendered as bulletList or taskList depending on detection.
	found := findNode(doc.Content, "taskList")
	if found == nil {
		// Fallback: check if it's a bulletList with taskItem children.
		bl := findNode(doc.Content, "bulletList")
		if bl != nil {
			t.Logf("got bulletList instead of taskList, content types: ")
			for _, c := range bl.Content {
				t.Logf("  %s", c.Type)
			}
		}
		t.Fatalf("expected taskList node, got ADF: %s", result)
	}
	if len(found.Content) != 2 {
		t.Errorf("expected 2 task items, got %d", len(found.Content))
	}
}

// --- Round-trip tests ---

func TestRoundTrip_SimpleParagraph(t *testing.T) {
	original := makeDoc(Node{
		Type:    "paragraph",
		Content: []Node{{Type: "text", Text: "Hello world"}},
	})
	md, err := ToMarkdown(original)
	if err != nil {
		t.Fatalf("ToMarkdown: %v", err)
	}
	result, err := FromMarkdown(md)
	if err != nil {
		t.Fatalf("FromMarkdown: %v", err)
	}
	doc := parseDoc(t, result)
	if len(doc.Content) == 0 {
		t.Fatal("expected content after round-trip")
	}
	// Goldmark may split text across nodes; just check the paragraph exists with text.
	para := findNode(doc.Content, "paragraph")
	if para == nil {
		t.Fatal("expected paragraph after round-trip")
	}
	allText := gatherText(para.Content)
	if !strings.Contains(allText, "Hello") || !strings.Contains(allText, "world") {
		t.Errorf("round-trip lost text content, got %q", allText)
	}
}

func TestRoundTrip_HeadingAndList(t *testing.T) {
	original := makeDoc(
		Node{
			Type:    "heading",
			Attrs:   map[string]any{"level": float64(2)},
			Content: []Node{{Type: "text", Text: "Title"}},
		},
		Node{
			Type: "bulletList",
			Content: []Node{
				{Type: "listItem", Content: []Node{
					{Type: "paragraph", Content: []Node{{Type: "text", Text: "item one"}}},
				}},
				{Type: "listItem", Content: []Node{
					{Type: "paragraph", Content: []Node{{Type: "text", Text: "item two"}}},
				}},
			},
		},
	)
	md, err := ToMarkdown(original)
	if err != nil {
		t.Fatalf("ToMarkdown: %v", err)
	}
	result, err := FromMarkdown(md)
	if err != nil {
		t.Fatalf("FromMarkdown: %v", err)
	}
	doc := parseDoc(t, result)
	heading := findNode(doc.Content, "heading")
	if heading == nil {
		t.Error("lost heading in round-trip")
	}
	list := findNode(doc.Content, "bulletList")
	if list == nil {
		t.Error("lost bullet list in round-trip")
	}
}

// --- Helpers ---

func makeDoc(nodes ...Node) string {
	doc := Document{
		Type:    "doc",
		Version: 1,
		Content: nodes,
	}
	data, _ := json.Marshal(doc)
	return string(data)
}

func parseDoc(t *testing.T, adfJSON string) Document {
	t.Helper()
	var doc Document
	if err := json.Unmarshal([]byte(adfJSON), &doc); err != nil {
		t.Fatalf("invalid ADF JSON: %v\nJSON: %s", err, adfJSON)
	}
	return doc
}

func findNode(nodes []Node, nodeType string) *Node {
	for i := range nodes {
		if nodes[i].Type == nodeType {
			return &nodes[i]
		}
		if found := findNode(nodes[i].Content, nodeType); found != nil {
			return found
		}
	}
	return nil
}

func findTextNode(nodes []Node) *Node {
	return findNode(nodes, "text")
}

func findTextWithMark(nodes []Node, markType string) *Node {
	for i := range nodes {
		if nodes[i].Type == "text" && hasMark(nodes[i].Marks, markType) {
			return &nodes[i]
		}
		if found := findTextWithMark(nodes[i].Content, markType); found != nil {
			return found
		}
	}
	return nil
}

func hasMark(marks []Mark, markType string) bool {
	for _, m := range marks {
		if m.Type == markType {
			return true
		}
	}
	return false
}

func gatherText(nodes []Node) string {
	var buf strings.Builder
	gatherTextRec(&buf, nodes)
	return buf.String()
}

func gatherTextRec(buf *strings.Builder, nodes []Node) {
	for _, n := range nodes {
		if n.Type == "text" {
			buf.WriteString(n.Text)
		}
		gatherTextRec(buf, n.Content)
	}
}
