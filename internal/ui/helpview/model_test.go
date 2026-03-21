package helpview

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNew(t *testing.T) {
	m := New()
	if m.dismissed {
		t.Fatal("new model should not be dismissed")
	}
	if m.built {
		t.Fatal("new model should not be built yet")
	}
}

func TestDismissed_EscKey(t *testing.T) {
	m := New()
	m.SetSize(80, 40)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !m.Dismissed() {
		t.Fatal("esc should dismiss")
	}
}

func TestDismissed_QKey(t *testing.T) {
	m := New()
	m.SetSize(80, 40)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if !m.Dismissed() {
		t.Fatal("q should dismiss")
	}
}

func TestDismissed_QuestionMark(t *testing.T) {
	m := New()
	m.SetSize(80, 40)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	if !m.Dismissed() {
		t.Fatal("? should dismiss")
	}
}

func TestDismissed_ClearsAfterRead(t *testing.T) {
	m := New()
	m.SetSize(80, 40)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	_ = m.Dismissed() // First read: true
	if m.Dismissed() {
		t.Fatal("second Dismissed() call should return false")
	}
}

func TestInputActive(t *testing.T) {
	m := New()
	if !m.InputActive() {
		t.Fatal("help view should always report input active")
	}
}

func TestView_ContainsSections(t *testing.T) {
	m := New()
	m.SetSize(100, 80)
	view := m.View()

	for _, want := range []string{
		"Keyboard Shortcuts",
		"Navigation",
		"Search & Filters",
		"Issue Operations",
		"Views",
		"Filter Manager",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("view should contain %q", want)
		}
	}
}

func TestView_ContainsNewSearchBinding(t *testing.T) {
	m := New()
	m.SetSize(100, 50)
	view := m.View()

	if !strings.Contains(view, "Open JQL search") {
		t.Error("view should reference JQL search")
	}
}

func TestSetSize_RebuildsContent(t *testing.T) {
	m := New()
	m.SetSize(80, 40)
	_ = m.View() // builds
	if !m.built {
		t.Fatal("view should build on first render")
	}
	m.SetSize(120, 60)
	if m.built {
		t.Fatal("SetSize should reset built flag")
	}
}

func TestBuild_SetsBuiltFlag(t *testing.T) {
	m := New()
	m.SetSize(80, 40)
	m.build()
	if !m.built {
		t.Fatal("build should set built flag")
	}
}

func TestBuild_ViewportDimensions(t *testing.T) {
	m := New()
	m.SetSize(80, 40)
	m.build()

	// maxWidth = min(80-4, 72) = 72, viewport width = 72-6 = 66
	if got := m.viewport.Width; got != 66 {
		t.Errorf("viewport width = %d, want 66", got)
	}

	// vpHeight = 40 - 12 = 28
	if got := m.viewport.Height; got != 28 {
		t.Errorf("viewport height = %d, want 28", got)
	}
}

func TestBuild_NarrowTerminal(t *testing.T) {
	m := New()
	m.SetSize(40, 20)
	m.build()

	// maxWidth = min(40-4, 72) = 36, viewport width = 36-6 = 30
	if got := m.viewport.Width; got != 30 {
		t.Errorf("viewport width = %d, want 30", got)
	}
}

func TestBuild_ShortTerminal(t *testing.T) {
	m := New()
	m.SetSize(80, 10)
	m.build()

	// vpHeight = 10 - 12 = -2, clamped to 5
	if got := m.viewport.Height; got != 5 {
		t.Errorf("viewport height = %d, want 5 (minimum)", got)
	}
}

func TestBuild_ContentIncludesAllSections(t *testing.T) {
	m := New()
	m.SetSize(100, 80)
	m.build()

	content := m.viewport.View()
	for _, title := range []string{"Navigation", "Search & Filters", "Issue Operations", "Views", "Confluence Wiki", "Filter Manager"} {
		if !strings.Contains(content, title) {
			t.Errorf("viewport content should contain section %q", title)
		}
	}
}

func TestView_ContainsFooter(t *testing.T) {
	m := New()
	m.SetSize(100, 80)
	view := m.View()

	if !strings.Contains(view, "close") {
		t.Error("view should contain close hint in footer")
	}
	if !strings.Contains(view, "scroll") {
		t.Error("view should contain scroll hint in footer")
	}
}

func TestUpdate_BuildsOnFirstKey(t *testing.T) {
	m := New()
	m.SetSize(80, 40)
	// Send a key without calling View first — Update should build.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if !m.built {
		t.Fatal("Update should build viewport if not yet built")
	}
}
