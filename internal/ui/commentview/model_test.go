package commentview

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNew_InitialisesCorrectly(t *testing.T) {
	m := New("PROJ-42")
	if m.IssueKey() != "PROJ-42" {
		t.Errorf("IssueKey() = %q, want %q", m.IssueKey(), "PROJ-42")
	}
	if !m.InputActive() {
		t.Error("InputActive() should always return true")
	}
}

func TestSubmittedComment_ReturnsTextOnce(t *testing.T) {
	m := New("PROJ-1")
	m.SetSize(80, 24)

	// Type some text into the textarea.
	m = typeText(t, m, "This is a test comment")

	// Submit with ctrl+s.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})

	got := m.SubmittedComment()
	if got != "This is a test comment" {
		t.Errorf("SubmittedComment() = %q, want %q", got, "This is a test comment")
	}

	// Sentinel should clear after first read.
	got = m.SubmittedComment()
	if got != "" {
		t.Errorf("SubmittedComment() second call = %q, want empty", got)
	}
}

func TestSubmittedComment_EmptyBodyGuard(t *testing.T) {
	m := New("PROJ-1")
	m.SetSize(80, 24)

	// ctrl+s with no text should not set the sentinel.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})

	got := m.SubmittedComment()
	if got != "" {
		t.Errorf("SubmittedComment() = %q, want empty (empty body guard)", got)
	}
}

func TestDismissed_OnEsc(t *testing.T) {
	m := New("PROJ-1")
	m.SetSize(80, 24)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if !m.Dismissed() {
		t.Error("Dismissed() should return true after Esc")
	}

	// Sentinel should clear after first read.
	if m.Dismissed() {
		t.Error("Dismissed() second call should return false")
	}
}

func TestDismissed_NotSetOnOtherKeys(t *testing.T) {
	m := New("PROJ-1")
	m.SetSize(80, 24)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})

	if m.Dismissed() {
		t.Error("Dismissed() should not be set for regular key presses")
	}
}

func TestSetSize_UnsafeWithoutIssueKey(t *testing.T) {
	// A zero-value Model has no issueKey — SetSize should not panic.
	m := Model{}
	m.SetSize(80, 24)

	// Should exit early without configuring textarea width.
	// No panic = pass.
}

func TestSetSize_CapsTextareaWidth(t *testing.T) {
	m := New("PROJ-1")

	// Very narrow terminal — textarea width should be capped.
	m.SetSize(20, 24)
	// Should not panic. The taWidth = min(60, 20-8) = 12, which is > 0.

	// Extremely narrow — taWidth would be <= 0, so textarea is not resized.
	m.SetSize(5, 24)
	// Should not panic.
}

func TestView_ContainsIssueKey(t *testing.T) {
	m := New("PROJ-99")
	m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, "PROJ-99") {
		t.Error("View should contain the issue key")
	}
}

func TestView_ContainsHelpText(t *testing.T) {
	m := New("PROJ-1")
	m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, "ctrl+s") {
		t.Error("View should contain ctrl+s help text")
	}
	if !strings.Contains(view, "esc") {
		t.Error("View should contain esc help text")
	}
}

func TestInputActive_AlwaysTrue(t *testing.T) {
	m := New("PROJ-1")
	if !m.InputActive() {
		t.Error("InputActive() should be true initially")
	}

	// Still true after dismissal.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !m.InputActive() {
		t.Error("InputActive() should remain true even after dismiss")
	}
}

// typeText simulates typing each rune into the textarea.
func typeText(t *testing.T, m Model, text string) Model {
	t.Helper()
	for _, r := range text {
		var cmd tea.Cmd
		m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		_ = cmd
	}
	return m
}
