package adf

import (
	"strings"
	"testing"
)

func TestRender_Empty(t *testing.T) {
	if got := Render("", 80); got != "" {
		t.Errorf("Render empty = %q, want empty", got)
	}
}

func TestRender_InvalidJSON(t *testing.T) {
	if got := Render("not json", 80); got != "" {
		t.Errorf("Render invalid = %q, want empty", got)
	}
}

func TestRender_Paragraph(t *testing.T) {
	adf := `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"Hello world"}]}]}`
	got := Render(adf, 80)
	if !strings.Contains(got, "Hello world") {
		t.Errorf("Render paragraph = %q, want to contain 'Hello world'", got)
	}
}

func TestRender_Heading(t *testing.T) {
	adf := `{"type":"doc","version":1,"content":[{"type":"heading","attrs":{"level":1},"content":[{"type":"text","text":"Title"}]}]}`
	got := Render(adf, 80)
	if !strings.Contains(got, "Title") {
		t.Errorf("Render heading = %q, want to contain 'Title'", got)
	}
}

func TestRender_BulletList(t *testing.T) {
	adf := `{"type":"doc","version":1,"content":[{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"Item one"}]}]},{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"Item two"}]}]}]}]}`
	got := Render(adf, 80)
	if !strings.Contains(got, "Item one") || !strings.Contains(got, "Item two") {
		t.Errorf("Render bullet list = %q, want items", got)
	}
}

func TestRender_Table(t *testing.T) {
	adf := `{"type":"doc","version":1,"content":[{"type":"table","content":[{"type":"tableRow","content":[{"type":"tableHeader","content":[{"type":"paragraph","content":[{"type":"text","text":"Col A"}]}]},{"type":"tableHeader","content":[{"type":"paragraph","content":[{"type":"text","text":"Col B"}]}]}]},{"type":"tableRow","content":[{"type":"tableCell","content":[{"type":"paragraph","content":[{"type":"text","text":"val1"}]}]},{"type":"tableCell","content":[{"type":"paragraph","content":[{"type":"text","text":"val2"}]}]}]}]}]}`
	got := Render(adf, 80)
	if !strings.Contains(got, "Col A") || !strings.Contains(got, "val1") {
		t.Errorf("Render table = %q, want table content", got)
	}
}

func TestRender_ZeroWidth(t *testing.T) {
	adf := `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"Test"}]}]}`
	got := Render(adf, 0)
	if !strings.Contains(got, "Test") {
		t.Errorf("Render zero width = %q, want to contain 'Test'", got)
	}
}

func TestExtractPageRefs_InlineCard(t *testing.T) {
	adf := `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"inlineCard","attrs":{"url":"https://example.atlassian.net/wiki/spaces/ENG/pages/12345/My+Page"}}]}]}`
	refs := ExtractPageRefs(adf)
	if len(refs) != 1 {
		t.Fatalf("got %d refs, want 1", len(refs))
	}
	if refs[0].ID != "12345" {
		t.Errorf("ref ID = %q, want 12345", refs[0].ID)
	}
}

func TestExtractPageRefs_NoLinks(t *testing.T) {
	adf := `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"No links here"}]}]}`
	refs := ExtractPageRefs(adf)
	if len(refs) != 0 {
		t.Errorf("got %d refs, want 0", len(refs))
	}
}

func TestExtractPageRefs_Empty(t *testing.T) {
	refs := ExtractPageRefs("")
	if refs != nil {
		t.Errorf("got %v, want nil", refs)
	}
}

func TestExtractPageRefs_Deduplication(t *testing.T) {
	adf := `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"inlineCard","attrs":{"url":"https://example.atlassian.net/wiki/spaces/ENG/pages/12345/A"}},{"type":"inlineCard","attrs":{"url":"https://example.atlassian.net/wiki/spaces/ENG/pages/12345/B"}}]}]}`
	refs := ExtractPageRefs(adf)
	if len(refs) != 1 {
		t.Errorf("got %d refs, want 1 (deduped)", len(refs))
	}
}

func TestExtractIssueKeys_Found(t *testing.T) {
	adf := `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"Fixed in PROJ-123 and TEAM-456"}]}]}`
	keys := ExtractIssueKeys(adf)
	if len(keys) != 2 {
		t.Fatalf("got %d keys, want 2", len(keys))
	}
}

func TestExtractIssueKeys_Deduplication(t *testing.T) {
	adf := `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"See PROJ-123 and also PROJ-123 again"}]}]}`
	keys := ExtractIssueKeys(adf)
	if len(keys) != 1 {
		t.Errorf("got %d keys, want 1 (deduped)", len(keys))
	}
}

func TestExtractIssueKeys_Empty(t *testing.T) {
	keys := ExtractIssueKeys("")
	if keys != nil {
		t.Errorf("got %v, want nil", keys)
	}
}

// --- RenderWithComments tests ---

// adfWithAnnotation builds an ADF doc with a paragraph containing annotated text.
func adfWithAnnotation(text, commentID string) string {
	return `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[` +
		`{"type":"text","text":"` + text + `","marks":[{"type":"annotation","attrs":{"annotationType":"inlineComment","id":"` + commentID + `"}}]}` +
		`]}]}`
}

func TestRenderWithComments_PlacesInline(t *testing.T) {
	doc := adfWithAnnotation("annotated text", "c1")
	comments := map[string]InlineComment{
		"c1": {Author: "Alice", BodyADF: `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"my comment"}]}]}`, Status: "open"},
	}

	rendered, cp := RenderWithComments(doc, 80, comments)

	if !cp.Placed["c1"] {
		t.Error("expected c1 to be placed")
	}
	if !strings.Contains(rendered, "annotated text") {
		t.Error("expected annotated text in output")
	}
	if !strings.Contains(rendered, "Alice") {
		t.Error("expected comment author in output")
	}
	if !strings.Contains(rendered, "my comment") {
		t.Error("expected comment body in output")
	}
}

func TestRenderWithComments_LineOffsets(t *testing.T) {
	doc := adfWithAnnotation("hello", "c1")
	comments := map[string]InlineComment{
		"c1": {Author: "Bob", BodyADF: `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"note"}]}]}`},
	}

	_, cp := RenderWithComments(doc, 80, comments)

	if len(cp.Lines) != 1 {
		t.Fatalf("expected 1 line offset, got %d", len(cp.Lines))
	}
	if cp.Lines[0] < 1 {
		t.Errorf("expected line offset > 0, got %d", cp.Lines[0])
	}
}

func TestRenderWithComments_NoComments(t *testing.T) {
	doc := `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"plain text"}]}]}`

	rendered, cp := RenderWithComments(doc, 80, nil)

	if len(cp.Placed) != 0 {
		t.Error("expected no placed comments")
	}
	if !strings.Contains(rendered, "plain text") {
		t.Errorf("expected plain text, got %q", rendered)
	}
}

func TestRenderWithComments_UnmatchedAnnotation(t *testing.T) {
	doc := adfWithAnnotation("text", "c99")
	comments := map[string]InlineComment{
		"c1": {Author: "Alice", BodyADF: `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"unused"}]}]}`},
	}

	_, cp := RenderWithComments(doc, 80, comments)

	if cp.Placed["c1"] {
		t.Error("c1 should not be placed — no matching annotation")
	}
	if cp.Placed["c99"] {
		t.Error("c99 should not be placed — not in comments map")
	}
}

func TestRenderWithComments_MultipleAnnotations(t *testing.T) {
	// Two paragraphs, each with a different annotation.
	doc := `{"type":"doc","version":1,"content":[` +
		`{"type":"paragraph","content":[{"type":"text","text":"first","marks":[{"type":"annotation","attrs":{"annotationType":"inlineComment","id":"c1"}}]}]},` +
		`{"type":"paragraph","content":[{"type":"text","text":"second","marks":[{"type":"annotation","attrs":{"annotationType":"inlineComment","id":"c2"}}]}]}` +
		`]}`
	comments := map[string]InlineComment{
		"c1": {Author: "Alice", BodyADF: `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"note1"}]}]}`},
		"c2": {Author: "Bob", BodyADF: `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"note2"}]}]}`},
	}

	rendered, cp := RenderWithComments(doc, 80, comments)

	if !cp.Placed["c1"] || !cp.Placed["c2"] {
		t.Error("expected both comments placed")
	}
	if len(cp.Lines) != 2 {
		t.Fatalf("expected 2 line offsets, got %d", len(cp.Lines))
	}
	if cp.Lines[0] >= cp.Lines[1] {
		t.Errorf("expected ascending line offsets, got %v", cp.Lines)
	}
	if !strings.Contains(rendered, "note1") || !strings.Contains(rendered, "note2") {
		t.Error("expected both comment bodies in output")
	}
}

func TestRenderWithComments_EmptyADF(t *testing.T) {
	rendered, cp := RenderWithComments("", 80, map[string]InlineComment{"c1": {}})
	if rendered != "" {
		t.Errorf("expected empty, got %q", rendered)
	}
	if len(cp.Placed) != 0 {
		t.Error("expected no placed comments for empty ADF")
	}
}

func TestRenderWithComments_DuplicateAnnotationID(t *testing.T) {
	// Same annotation ID on two text nodes — should only place once.
	doc := `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[` +
		`{"type":"text","text":"a","marks":[{"type":"annotation","attrs":{"annotationType":"inlineComment","id":"c1"}}]},` +
		`{"type":"text","text":"b","marks":[{"type":"annotation","attrs":{"annotationType":"inlineComment","id":"c1"}}]}` +
		`]}]}`
	comments := map[string]InlineComment{
		"c1": {Author: "Alice", BodyADF: `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"note"}]}]}`},
	}

	_, cp := RenderWithComments(doc, 80, comments)

	if len(cp.Lines) != 1 {
		t.Errorf("expected 1 line offset (deduped), got %d", len(cp.Lines))
	}
}

func TestCollectAnnotationIDs(t *testing.T) {
	node := Node{
		Type: "paragraph",
		Content: []Node{
			{Type: "text", Text: "plain"},
			{Type: "text", Text: "annotated", Marks: []Mark{
				{Type: "annotation", Attrs: map[string]any{"annotationType": "inlineComment", "id": "c1"}},
			}},
			{Type: "text", Text: "other mark", Marks: []Mark{
				{Type: "strong"},
			}},
			{Type: "text", Text: "nested", Marks: []Mark{
				{Type: "annotation", Attrs: map[string]any{"annotationType": "inlineComment", "id": "c2"}},
			}},
		},
	}

	ids := collectAnnotationIDs(node)
	if len(ids) != 2 {
		t.Fatalf("expected 2 IDs, got %d: %v", len(ids), ids)
	}
	if ids[0] != "c1" || ids[1] != "c2" {
		t.Errorf("expected [c1, c2], got %v", ids)
	}
}

func TestCollectAnnotationIDs_IgnoresNonInlineComment(t *testing.T) {
	node := Node{
		Type: "paragraph",
		Content: []Node{
			{Type: "text", Text: "x", Marks: []Mark{
				{Type: "annotation", Attrs: map[string]any{"annotationType": "other", "id": "c1"}},
			}},
		},
	}

	ids := collectAnnotationIDs(node)
	if len(ids) != 0 {
		t.Errorf("expected 0 IDs for non-inlineComment annotation, got %v", ids)
	}
}

func TestCollectAnnotationIDs_Empty(t *testing.T) {
	node := Node{Type: "paragraph", Content: []Node{{Type: "text", Text: "no marks"}}}
	ids := collectAnnotationIDs(node)
	if len(ids) != 0 {
		t.Errorf("expected 0 IDs, got %v", ids)
	}
}

func Test_renderBlockquote(t *testing.T) {
	t.Run("single paragraph", func(t *testing.T) {
		node := Node{
			Type: "blockquote",
			Content: []Node{
				{Type: "paragraph", Content: []Node{
					{Type: "text", Text: "quoted text"},
				}},
			},
		}
		got := renderBlockquote(node, 80)
		if !strings.Contains(got, "│") {
			t.Errorf("expected blockquote border, got %q", got)
		}
		if !strings.Contains(got, "quoted text") {
			t.Errorf("expected content 'quoted text', got %q", got)
		}
	})

	t.Run("multiple paragraphs", func(t *testing.T) {
		node := Node{
			Type: "blockquote",
			Content: []Node{
				{Type: "paragraph", Content: []Node{
					{Type: "text", Text: "first"},
				}},
				{Type: "paragraph", Content: []Node{
					{Type: "text", Text: "second"},
				}},
			},
		}
		got := renderBlockquote(node, 80)
		if !strings.Contains(got, "first") || !strings.Contains(got, "second") {
			t.Errorf("expected both paragraphs, got %q", got)
		}
		// Every line should have the border.
		for _, line := range strings.Split(got, "\n") {
			if line != "" && !strings.Contains(line, "│") {
				t.Errorf("line missing border: %q", line)
			}
		}
	})

	t.Run("empty blockquote", func(t *testing.T) {
		node := Node{Type: "blockquote"}
		got := renderBlockquote(node, 80)
		if got != "" {
			t.Errorf("expected empty string for empty blockquote, got %q", got)
		}
	})

	t.Run("narrow width", func(t *testing.T) {
		node := Node{
			Type: "blockquote",
			Content: []Node{
				{Type: "paragraph", Content: []Node{
					{Type: "text", Text: "narrow"},
				}},
			},
		}
		got := renderBlockquote(node, 20)
		if !strings.Contains(got, "narrow") {
			t.Errorf("expected content at narrow width, got %q", got)
		}
	})
}

// --- Node type coverage tests ---

func TestRender_Expand(t *testing.T) {
	adf := `{"type":"doc","version":1,"content":[{"type":"expand","attrs":{"title":"Click to expand"},"content":[{"type":"paragraph","content":[{"type":"text","text":"Hidden content here"}]}]}]}`
	got := Render(adf, 80)
	if !strings.Contains(got, "Click to expand") {
		t.Errorf("expand title not rendered: %q", got)
	}
	if !strings.Contains(got, "Hidden content here") {
		t.Errorf("expand content not rendered: %q", got)
	}
}

func TestRender_NestedExpand(t *testing.T) {
	adf := `{"type":"doc","version":1,"content":[{"type":"nestedExpand","attrs":{"title":"Nested"},"content":[{"type":"paragraph","content":[{"type":"text","text":"Nested content"}]}]}]}`
	got := Render(adf, 80)
	if !strings.Contains(got, "Nested") {
		t.Errorf("nestedExpand not rendered: %q", got)
	}
}

func TestRender_BlockCard(t *testing.T) {
	// blockCard is an unhandled node type — should not panic.
	adf := `{"type":"doc","version":1,"content":[{"type":"blockCard","attrs":{"url":"https://example.com/page"}}]}`
	_ = Render(adf, 80) // No panic = pass.
}

func TestRender_EmbedCard(t *testing.T) {
	// embedCard is an unhandled node type — should not panic.
	adf := `{"type":"doc","version":1,"content":[{"type":"embedCard","attrs":{"url":"https://example.com/embed"}}]}`
	_ = Render(adf, 80) // No panic = pass.
}

func TestRender_MediaSingle(t *testing.T) {
	adf := `{"type":"doc","version":1,"content":[{"type":"mediaSingle","content":[{"type":"media","attrs":{"type":"file","id":"abc-123","collection":"attachments"}}]}]}`
	got := Render(adf, 80)
	// Media should render something (placeholder, filename, or attachment indicator).
	if got == "" {
		t.Error("mediaSingle rendered empty output")
	}
}

func TestRender_Mention(t *testing.T) {
	adf := `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"mention","attrs":{"id":"user-123","text":"@alice"}}]}]}`
	got := Render(adf, 80)
	if !strings.Contains(got, "@alice") && !strings.Contains(got, "alice") {
		t.Errorf("mention not rendered: %q", got)
	}
}

func TestRender_Emoji(t *testing.T) {
	adf := `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"emoji","attrs":{"shortName":":thumbsup:","text":"` + "\U0001F44D" + `"}}]}]}`
	got := Render(adf, 80)
	if !strings.Contains(got, "\U0001F44D") && !strings.Contains(got, ":thumbsup:") && got == "" {
		t.Errorf("emoji not rendered: %q", got)
	}
}

func TestRender_EmojiWithFallback(t *testing.T) {
	adf := `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"emoji","attrs":{"shortName":":rocket:"}}]}]}`
	got := Render(adf, 80)
	// Should render either the unicode emoji or the shortName.
	if got == "" {
		t.Error("emoji with fallback rendered empty")
	}
}

func TestRender_Status(t *testing.T) {
	adf := `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"status","attrs":{"text":"In Progress","color":"blue"}}]}]}`
	got := Render(adf, 80)
	if !strings.Contains(got, "In Progress") {
		t.Errorf("status badge not rendered: %q", got)
	}
}

func TestRender_HardBreak(t *testing.T) {
	adf := `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"Line one"},{"type":"hardBreak"},{"type":"text","text":"Line two"}]}]}`
	got := Render(adf, 80)
	if !strings.Contains(got, "Line one") || !strings.Contains(got, "Line two") {
		t.Errorf("hardBreak not rendered correctly: %q", got)
	}
}
