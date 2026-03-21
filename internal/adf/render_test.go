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
