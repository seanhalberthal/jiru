package issuepickview

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/undont/jiru/internal/ui/issueview"
)

func makeRefs(n int) []issueview.IssueRef {
	refs := make([]issueview.IssueRef, n)
	for i := range refs {
		refs[i] = issueview.IssueRef{
			Key:   fmt.Sprintf("PROJ-%d", i+1),
			Label: fmt.Sprintf("issue %d", i+1),
		}
	}
	return refs
}

func TestNew_BasicNavigation(t *testing.T) {
	refs := makeRefs(3)
	m := New(refs)
	m.SetSize(120, 40)

	if m.cursor != 0 {
		t.Errorf("expected cursor at 0, got %d", m.cursor)
	}

	// Move down.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m.cursor != 1 {
		t.Errorf("expected cursor at 1, got %d", m.cursor)
	}

	// Move down again.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m.cursor != 2 {
		t.Errorf("expected cursor at 2, got %d", m.cursor)
	}

	// Can't move past end.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m.cursor != 2 {
		t.Errorf("expected cursor clamped at 2, got %d", m.cursor)
	}

	// Move up.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if m.cursor != 1 {
		t.Errorf("expected cursor at 1, got %d", m.cursor)
	}
}

func TestSelected_Sentinel(t *testing.T) {
	refs := makeRefs(3)
	m := New(refs)
	m.SetSize(120, 40)

	// Select first item.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	ref := m.Selected()
	if ref == nil {
		t.Fatal("expected selection")
	}
	if ref.Key != "PROJ-1" {
		t.Errorf("expected PROJ-1, got %s", ref.Key)
	}

	// Should reset after read.
	if m.Selected() != nil {
		t.Error("expected nil after sentinel read")
	}
}

func TestDismissed_Sentinel(t *testing.T) {
	refs := makeRefs(3)
	m := New(refs)
	m.SetSize(120, 40)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !m.Dismissed() {
		t.Error("expected dismissed")
	}
	if m.Dismissed() {
		t.Error("expected dismissed reset after read")
	}
}

func TestScrolling_ManyItems(t *testing.T) {
	// Create more refs than can fit in a small terminal.
	refs := makeRefs(50)
	m := New(refs)
	m.SetSize(120, 30) // Small height — maxVisible will be ~13

	vis := m.maxVisible()
	if vis >= 50 {
		t.Skipf("terminal too tall for scroll test (vis=%d)", vis)
	}

	// Initially offset should be 0.
	if m.offset != 0 {
		t.Errorf("expected offset 0, got %d", m.offset)
	}

	// Move cursor to bottom of visible window.
	for range vis {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	}

	// Cursor should have scrolled.
	if m.offset == 0 {
		t.Error("expected offset to increase after scrolling past visible window")
	}
	if m.cursor < vis {
		t.Errorf("expected cursor >= %d, got %d", vis, m.cursor)
	}

	// Verify the view contains scroll indicators.
	view := m.View()
	if !strings.Contains(view, "↑") {
		t.Error("expected top scroll indicator")
	}
	if !strings.Contains(view, "↓") {
		t.Error("expected bottom scroll indicator")
	}

	// Scroll back up to the top.
	for range m.cursor {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	}
	if m.offset != 0 {
		t.Errorf("expected offset 0 after scrolling back, got %d", m.offset)
	}
	if m.cursor != 0 {
		t.Errorf("expected cursor 0 after scrolling back, got %d", m.cursor)
	}
}

func TestView_FitsInTerminal(t *testing.T) {
	refs := makeRefs(100)
	m := New(refs)
	m.SetSize(80, 24) // Typical small terminal.

	view := m.View()
	lines := strings.Split(view, "\n")

	// The rendered view should not exceed the terminal height.
	if len(lines) > 24 {
		t.Errorf("view has %d lines, exceeds terminal height 24", len(lines))
	}
}

func TestEmpty_NoScrollIndicators(t *testing.T) {
	m := New(nil)
	m.SetSize(120, 40)

	view := m.View()
	if strings.Contains(view, "↑") || strings.Contains(view, "↓") {
		t.Error("expected no scroll indicators for empty list")
	}
}

func TestView_LabelTruncated(t *testing.T) {
	refs := []issueview.IssueRef{{Key: "PROJ-1", Label: strings.Repeat("x", 200)}}
	m := New(refs)
	m.SetSize(40, 40) // Very narrow terminal.

	view := m.View()
	if !strings.Contains(view, "…") {
		t.Error("expected truncated label with ellipsis")
	}
}

func TestInputActive(t *testing.T) {
	m := New(makeRefs(3))
	if !m.InputActive() {
		t.Error("picker must suppress global keys")
	}
}

func TestSmallList_NoScrollIndicators(t *testing.T) {
	refs := makeRefs(3)
	m := New(refs)
	m.SetSize(120, 40)

	view := m.View()
	if strings.Contains(view, "↑") || strings.Contains(view, "↓") {
		t.Error("expected no scroll indicators when all items fit")
	}
}

func TestView_GroupHeaders(t *testing.T) {
	refs := []issueview.IssueRef{
		{Key: "PROJ-1", Label: "parent issue", Group: "Parent"},
		{Key: "PROJ-2", Label: "child one", Group: "To Do (2)"},
		{Key: "PROJ-3", Label: "child two", Group: "To Do (2)"},
		{Key: "PROJ-4", Label: "done child", Group: "Done (1)"},
	}
	m := New(refs)
	m.SetSize(120, 40)

	view := m.View()
	if !strings.Contains(view, "Parent") {
		t.Error("expected 'Parent' group header in view")
	}
	if !strings.Contains(view, "To Do (2)") {
		t.Error("expected 'To Do (2)' group header in view")
	}
	if !strings.Contains(view, "Done (1)") {
		t.Error("expected 'Done (1)' group header in view")
	}
}

func TestFilter_NarrowsResults(t *testing.T) {
	refs := []issueview.IssueRef{
		{Key: "PROJ-1", Label: "alpha work"},
		{Key: "PROJ-2", Label: "beta work"},
	}
	m := New(refs)
	m.SetSize(120, 40)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	if !m.filtering {
		t.Fatal("expected filter mode after '/'")
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("beta")})
	if len(m.filtered) != 1 {
		t.Fatalf("expected 1 filtered ref, got %d", len(m.filtered))
	}
	if m.filtered[0].Key != "PROJ-2" {
		t.Fatalf("expected PROJ-2 match, got %s", m.filtered[0].Key)
	}
}

func TestFilter_EnterSelectsFilteredItem(t *testing.T) {
	refs := []issueview.IssueRef{
		{Key: "PROJ-1", Label: "alpha work"},
		{Key: "PROJ-2", Label: "beta work"},
	}
	m := New(refs)
	m.SetSize(120, 40)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("beta")})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	sel := m.Selected()
	if sel == nil || sel.Key != "PROJ-2" {
		t.Fatalf("expected filtered selection PROJ-2, got %+v", sel)
	}
}
