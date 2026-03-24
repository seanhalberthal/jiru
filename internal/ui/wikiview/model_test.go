package wikiview

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/seanhalberthal/jiru/internal/confluence"
)

func TestDismissed_SentinelResetsOnRead(t *testing.T) {
	m := New()
	m.dismissed = true
	if !m.Dismissed() {
		t.Error("expected true on first read")
	}
	if m.Dismissed() {
		t.Error("expected false on second read")
	}
}

func TestOpenURL_SentinelResetsOnRead(t *testing.T) {
	m := New()
	m.openURL = "https://example.com"
	url, ok := m.OpenURL()
	if !ok || url != "https://example.com" {
		t.Errorf("first read = (%q, %v), want (url, true)", url, ok)
	}
	_, ok = m.OpenURL()
	if ok {
		t.Error("expected false on second read")
	}
}

func TestSelectedIssue_SentinelResetsOnRead(t *testing.T) {
	m := New()
	m.selIssue = "PROJ-123"
	key, ok := m.SelectedIssue()
	if !ok || key != "PROJ-123" {
		t.Errorf("first read = (%q, %v), want (PROJ-123, true)", key, ok)
	}
	_, ok = m.SelectedIssue()
	if ok {
		t.Error("expected false on second read")
	}
}

func TestSelectedAncestor_SentinelResetsOnRead(t *testing.T) {
	m := New()
	m.selAnc = "456"
	id, ok := m.SelectedAncestor()
	if !ok || id != "456" {
		t.Errorf("first read = (%q, %v), want (456, true)", id, ok)
	}
	_, ok = m.SelectedAncestor()
	if ok {
		t.Error("expected false on second read")
	}
}

func TestSetPage_ViewNonEmpty(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)
	m.SetPage(&confluence.Page{
		ID:      "123",
		Title:   "Test Page",
		BodyADF: `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}]}`,
	})
	view := m.View()
	if view == "" {
		t.Error("View() should not be empty after SetPage")
	}
}

func TestCurrentPage(t *testing.T) {
	m := New()
	if m.CurrentPage() != nil {
		t.Error("CurrentPage should be nil initially")
	}
	page := &confluence.Page{ID: "1", Title: "P"}
	m.SetPage(page)
	if m.CurrentPage() != page {
		t.Error("CurrentPage should return the set page")
	}
}

func TestUpdate_EscDismisses(t *testing.T) {
	m := New()
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !m.Dismissed() {
		t.Error("Esc should set dismissed")
	}
}

// --- Comment tests ---

func TestSetComments_RendersInView(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)
	m.SetPage(&confluence.Page{
		ID:      "1",
		Title:   "Page",
		BodyADF: `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"body"}]}]}`,
	})

	m.SetComments([]confluence.Comment{
		{ID: "c1", Author: "Alice", BodyADF: `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"footer comment"}]}]}`, Created: time.Now()},
	})

	view := m.View()
	if !strings.Contains(view, "Alice") {
		t.Error("expected comment author in view")
	}
}

func TestSetPage_ClearsComments(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)
	m.SetPage(&confluence.Page{ID: "1", Title: "P1", BodyADF: `{"type":"doc","version":1,"content":[]}`})
	m.SetComments([]confluence.Comment{
		{ID: "c1", Author: "Alice"},
	})

	// Setting a new page should clear comments.
	m.SetPage(&confluence.Page{ID: "2", Title: "P2", BodyADF: `{"type":"doc","version":1,"content":[]}`})
	if len(m.comments) != 0 {
		t.Error("expected comments to be cleared after SetPage")
	}
	if m.commentLine != -1 {
		t.Error("expected commentLine to be reset")
	}
	if m.inlineIdx != -1 {
		t.Error("expected inlineIdx to be reset")
	}
	if len(m.inlineLines) != 0 {
		t.Error("expected inlineLines to be cleared")
	}
}

func TestCommentStats(t *testing.T) {
	tests := []struct {
		name     string
		comments []confluence.Comment
		wantSub  []string // substrings that must appear
		wantNot  []string // substrings that must not appear
	}{
		{
			name:    "no comments",
			wantSub: nil,
		},
		{
			name: "footer only",
			comments: []confluence.Comment{
				{ID: "c1", Inline: false},
				{ID: "c2", Inline: false},
			},
			wantSub: []string{"2 comments"},
			wantNot: []string{"unresolved"},
		},
		{
			name: "single footer",
			comments: []confluence.Comment{
				{ID: "c1", Inline: false},
			},
			wantSub: []string{"1 comment"},
		},
		{
			name: "unresolved only",
			comments: []confluence.Comment{
				{ID: "c1", Inline: true, ResolutionStatus: "open"},
			},
			wantSub: []string{"1 unresolved"},
			wantNot: []string{"comment"},
		},
		{
			name: "mixed",
			comments: []confluence.Comment{
				{ID: "c1", Inline: false},
				{ID: "c2", Inline: true, ResolutionStatus: "open"},
				{ID: "c3", Inline: true, ResolutionStatus: "resolved"},
				{ID: "c4", Inline: true, ResolutionStatus: "reopened"},
			},
			wantSub: []string{"1 comment", "2 unresolved"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New()
			m.comments = tt.comments
			got := m.commentStats()

			if len(tt.wantSub) == 0 {
				if got != "" {
					t.Errorf("commentStats() = %q, want empty", got)
				}
				return
			}
			for _, sub := range tt.wantSub {
				if !strings.Contains(got, sub) {
					t.Errorf("commentStats() = %q, want substring %q", got, sub)
				}
			}
			for _, sub := range tt.wantNot {
				if strings.Contains(got, sub) {
					t.Errorf("commentStats() = %q, should not contain %q", got, sub)
				}
			}
		})
	}
}

func TestBuildInlineCommentMap(t *testing.T) {
	m := New()
	m.comments = []confluence.Comment{
		{ID: "c1", Inline: true, MarkerRef: "m1", Author: "Alice", BodyADF: "body1", ResolutionStatus: "open"},
		{ID: "c2", Inline: false, Author: "Bob"}, // footer — excluded
		{ID: "c3", Inline: true, MarkerRef: "m3", Author: "Carol", BodyADF: "body3", ResolutionStatus: "resolved"},
		{ID: "c4", Inline: true, MarkerRef: "", Author: "Dave"}, // no marker ref — excluded
	}

	cm := m.buildInlineCommentMap()
	if len(cm) != 2 {
		t.Fatalf("expected 2 inline comments, got %d", len(cm))
	}
	if cm["m1"].Author != "Alice" || cm["m1"].Status != "open" {
		t.Errorf("m1 = %+v", cm["m1"])
	}
	if cm["m3"].Author != "Carol" {
		t.Errorf("m3 = %+v", cm["m3"])
	}
	if _, ok := cm["c2"]; ok {
		t.Error("footer comment c2 should not be in map")
	}
}

func TestCommentLine_JumpsToFooterComments(t *testing.T) {
	m := New()
	// Use a small viewport so the content is scrollable.
	m = m.SetSize(80, 10)

	// Generate enough body content to exceed viewport height.
	longBody := strings.Repeat("line\\n", 30)
	m.SetPage(&confluence.Page{
		ID:      "1",
		Title:   "P",
		BodyADF: `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"` + longBody + `"}]}]}`,
	})
	m.SetComments([]confluence.Comment{
		{ID: "c1", Inline: false, Author: "Alice", BodyADF: `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"comment"}]}]}`},
	})

	if m.commentLine < 0 {
		t.Fatal("expected commentLine to be set")
	}

	// Press 'c' — should scroll toward the comment line.
	// The viewport clamps to the max scrollable offset, so check we moved.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if m.viewport.YOffset == 0 {
		t.Error("expected viewport to scroll away from top after pressing 'c'")
	}
}

func TestCommentLine_NotSetWithoutFooterComments(t *testing.T) {
	m := New()
	m = m.SetSize(80, 40)
	m.SetPage(&confluence.Page{
		ID:      "1",
		Title:   "P",
		BodyADF: `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"body"}]}]}`,
	})
	// Only inline comments — no footer.
	m.SetComments([]confluence.Comment{
		{ID: "c1", Inline: true, Author: "Alice", ResolutionStatus: "open"},
	})

	if m.commentLine >= 0 {
		t.Errorf("commentLine = %d, should be -1 when no footer comments", m.commentLine)
	}
}

func TestNextPrevInline_CyclesThrough(t *testing.T) {
	m := New()
	m = m.SetSize(80, 15)

	// ADF with two annotated paragraphs. The annotation "id" values are marker refs
	// (UUIDs in production), which must match the MarkerRef field on the comments.
	doc := `{"type":"doc","version":1,"content":[` +
		`{"type":"paragraph","content":[{"type":"text","text":"first block of text that is long enough to take up space in the viewport for testing purposes","marks":[{"type":"annotation","attrs":{"annotationType":"inlineComment","id":"m1"}}]}]},` +
		`{"type":"paragraph","content":[{"type":"text","text":"second block of text that also needs to be long enough for scrolling purposes in the test","marks":[{"type":"annotation","attrs":{"annotationType":"inlineComment","id":"m2"}}]}]}` +
		`]}`

	// Set page first, then comments (SetComments triggers re-render with the comment map).
	m.SetPage(&confluence.Page{ID: "1", Title: "P", BodyADF: doc})
	m.SetComments([]confluence.Comment{
		{ID: "c1", Inline: true, MarkerRef: "m1", Author: "Alice", BodyADF: `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"note1"}]}]}`, ResolutionStatus: "open"},
		{ID: "c2", Inline: true, MarkerRef: "m2", Author: "Bob", BodyADF: `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"note2"}]}]}`, ResolutionStatus: "open"},
	})

	if len(m.inlineLines) < 2 {
		t.Fatalf("expected at least 2 inline line offsets, got %d", len(m.inlineLines))
	}

	// Press ] — should go to first inline comment.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	if m.inlineIdx != 0 {
		t.Errorf("after first ]: inlineIdx = %d, want 0", m.inlineIdx)
	}

	// Press ] again — should go to second.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	if m.inlineIdx != 1 {
		t.Errorf("after second ]: inlineIdx = %d, want 1", m.inlineIdx)
	}

	// Press ] again — should wrap to first.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	if m.inlineIdx != 0 {
		t.Errorf("after wrap ]: inlineIdx = %d, want 0", m.inlineIdx)
	}

	// Press [ — should go to last.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}})
	if m.inlineIdx != len(m.inlineLines)-1 {
		t.Errorf("after [: inlineIdx = %d, want %d", m.inlineIdx, len(m.inlineLines)-1)
	}
}

func TestNextPrevInline_NoOpWithoutInlineComments(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)
	m.SetPage(&confluence.Page{
		ID:      "1",
		Title:   "P",
		BodyADF: `{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"no annotations"}]}]}`,
	})

	// Press ] with no inline lines — should be a no-op.
	before := m.viewport.YOffset
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	if m.viewport.YOffset != before {
		t.Error("expected no scroll when no inline comments")
	}
	if m.inlineIdx != -1 {
		t.Errorf("inlineIdx = %d, want -1", m.inlineIdx)
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("short", 10); got != "short" {
		t.Errorf("truncate short = %q", got)
	}
	got := truncate("a long string here", 10)
	if runes := []rune(got); len(runes) > 10 {
		t.Errorf("truncate long = %q, rune len %d > 10", got, len(runes))
	}
	if got := truncate("exact len!", 10); got != "exact len!" {
		t.Errorf("truncate exact = %q", got)
	}
}

func TestRenderBreadcrumb_PersonalSpaceKeyOmitted(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)
	m.SetSpaceKey("~user1")
	m.SetPage(&confluence.Page{ID: "1", Title: "Page", BodyADF: `{"type":"doc","version":1,"content":[]}`})
	bc := m.renderBreadcrumb()
	if bc != "" {
		// Personal space keys (starting with ~) should not appear in breadcrumb
		// and with no ancestors, breadcrumb should be empty
		t.Errorf("breadcrumb = %q, want empty for personal space with no ancestors", bc)
	}
}
