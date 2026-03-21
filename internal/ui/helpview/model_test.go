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
