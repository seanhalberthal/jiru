package deleteview

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNew_Initialises(t *testing.T) {
	m := New("PROJ-42", "Fix the login bug")

	if m.issueKey != "PROJ-42" {
		t.Errorf("issueKey = %q, want %q", m.issueKey, "PROJ-42")
	}
	if m.summary != "Fix the login bug" {
		t.Errorf("summary = %q, want %q", m.summary, "Fix the login bug")
	}
	if m.cascade {
		t.Error("cascade should default to false")
	}
	if !m.InputActive() {
		t.Error("InputActive() should always return true")
	}
}

func TestConfirmed_OnEnter(t *testing.T) {
	m := New("PROJ-42", "Test issue")
	m.SetSize(80, 24)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	req := m.Confirmed()
	if req == nil {
		t.Fatal("Confirmed() should not be nil after Enter")
	}
	if req.Key != "PROJ-42" {
		t.Errorf("Key = %q, want %q", req.Key, "PROJ-42")
	}
	if req.Cascade {
		t.Error("Cascade should be false by default")
	}

	// Sentinel should clear after first read.
	if m.Confirmed() != nil {
		t.Error("Confirmed() second call should return nil")
	}
}

func TestConfirmed_OnY(t *testing.T) {
	m := New("PROJ-1", "Test")
	m.SetSize(80, 24)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})

	req := m.Confirmed()
	if req == nil {
		t.Fatal("Confirmed() should not be nil after 'y'")
	}
	if req.Key != "PROJ-1" {
		t.Errorf("Key = %q, want %q", req.Key, "PROJ-1")
	}
}

func TestDismissed_OnEsc(t *testing.T) {
	m := New("PROJ-1", "Test")
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

func TestDismissed_OnN(t *testing.T) {
	m := New("PROJ-1", "Test")
	m.SetSize(80, 24)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})

	if !m.Dismissed() {
		t.Error("Dismissed() should return true after 'n'")
	}
}

func TestCascadeToggle_Tab(t *testing.T) {
	m := New("PROJ-1", "Test")
	m.SetSize(80, 24)

	if m.cascade {
		t.Fatal("cascade should start as false")
	}

	// Toggle on.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if !m.cascade {
		t.Error("cascade should be true after first tab")
	}

	// Toggle off.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.cascade {
		t.Error("cascade should be false after second tab")
	}
}

func TestCascadeToggle_ReflectedInConfirmation(t *testing.T) {
	m := New("PROJ-1", "Test")
	m.SetSize(80, 24)

	// Enable cascade, then confirm.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	req := m.Confirmed()
	if req == nil {
		t.Fatal("Confirmed() should not be nil")
	}
	if !req.Cascade {
		t.Error("Cascade should be true after toggling")
	}
}

func TestSetSize_UpdatesDimensions(t *testing.T) {
	m := New("PROJ-1", "Test")
	m.SetSize(100, 50)

	if m.width != 100 {
		t.Errorf("width = %d, want 100", m.width)
	}
	if m.height != 50 {
		t.Errorf("height = %d, want 50", m.height)
	}
}

func TestView_ContainsIssueKey(t *testing.T) {
	m := New("PROJ-42", "Fix bug")
	m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, "PROJ-42") {
		t.Error("View should contain the issue key")
	}
}

func TestView_ContainsSummary(t *testing.T) {
	m := New("PROJ-1", "Fix the login bug")
	m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, "Fix the login bug") {
		t.Error("View should contain the issue summary")
	}
}

func TestView_ContainsDeleteTitle(t *testing.T) {
	m := New("PROJ-1", "Test")
	m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, "Delete Issue") {
		t.Error("View should contain 'Delete Issue' title")
	}
}

func TestView_ShowsCascadeUnchecked(t *testing.T) {
	m := New("PROJ-1", "Test")
	m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, "[ ] Also delete subtasks") {
		t.Error("View should show unchecked cascade option")
	}
}

func TestView_ShowsCascadeChecked(t *testing.T) {
	m := New("PROJ-1", "Test")
	m.SetSize(80, 24)

	// Toggle cascade on.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})

	view := m.View()
	if !strings.Contains(view, "[✓] Also delete subtasks") {
		t.Error("View should show checked cascade option after toggle")
	}
}

func TestView_ContainsHelpText(t *testing.T) {
	m := New("PROJ-1", "Test")
	m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, "confirm") {
		t.Error("View should contain confirm help text")
	}
	if !strings.Contains(view, "toggle subtasks") {
		t.Error("View should contain toggle subtasks help text")
	}
	if !strings.Contains(view, "cancel") {
		t.Error("View should contain cancel help text")
	}
}

func TestInputActive_AlwaysTrue(t *testing.T) {
	m := New("PROJ-1", "Test")
	if !m.InputActive() {
		t.Error("InputActive() should always return true")
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !m.InputActive() {
		t.Error("InputActive() should remain true even after dismiss")
	}
}

func TestUpdate_ReturnsNilCmd(t *testing.T) {
	m := New("PROJ-1", "Test")
	m.SetSize(80, 24)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("Update should return nil cmd")
	}

	m2 := New("PROJ-1", "Test")
	_, cmd = m2.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil {
		t.Error("Update should return nil cmd for esc")
	}

	m3 := New("PROJ-1", "Test")
	_, cmd = m3.Update(tea.KeyMsg{Type: tea.KeyTab})
	if cmd != nil {
		t.Error("Update should return nil cmd for tab")
	}
}
