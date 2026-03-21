package profilepickview

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

var testProfiles = []string{"default", "staging", "production"}

func TestNew_InitialisesCorrectly(t *testing.T) {
	m := New(testProfiles, "staging")

	if len(m.profiles) != 3 {
		t.Errorf("profiles count = %d, want 3", len(m.profiles))
	}
	if m.activeProfile != "staging" {
		t.Errorf("activeProfile = %q, want %q", m.activeProfile, "staging")
	}
}

func TestNew_CursorStartsAtActiveProfile(t *testing.T) {
	m := New(testProfiles, "staging")

	// Cursor should be at index 1 (staging).
	// Verify by pressing enter and checking selected.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sel := m.Selected()
	if sel != "staging" {
		t.Errorf("cursor should start at active profile 'staging', got %q", sel)
	}
}

func TestNew_CursorDefaultsToZeroWhenActiveNotFound(t *testing.T) {
	m := New(testProfiles, "nonexistent")

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sel := m.Selected()
	if sel != "default" {
		t.Errorf("cursor should default to 0 when active not found, got %q", sel)
	}
}

func TestNew_EmptyProfiles(t *testing.T) {
	m := New(nil, "anything")

	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0 for empty profiles", m.cursor)
	}
}

func TestCursorNavigation_JK(t *testing.T) {
	m := New(testProfiles, "default")
	m.SetSize(80, 24)

	// Cursor starts at 0. Move down with j.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sel := m.Selected()
	if sel != "staging" {
		t.Errorf("after j, expected 'staging', got %q", sel)
	}
}

func TestCursorNavigation_UpDown(t *testing.T) {
	m := New(testProfiles, "default")
	m.SetSize(80, 24)

	// Move down twice with arrow keys.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sel := m.Selected()
	if sel != "production" {
		t.Errorf("after 2x down, expected 'production', got %q", sel)
	}
}

func TestCursorNavigation_UpAfterDown(t *testing.T) {
	m := New(testProfiles, "default")
	m.SetSize(80, 24)

	// Down, down, up — should be at index 1.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	sel := m.Selected()
	if sel != "staging" {
		t.Errorf("expected 'staging' after down-down-up, got %q", sel)
	}
}

func TestCursorNavigation_KUp(t *testing.T) {
	m := New(testProfiles, "production")
	m.SetSize(80, 24)

	// Cursor starts at 2 (production). Move up with k.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sel := m.Selected()
	if sel != "staging" {
		t.Errorf("after k from production, expected 'staging', got %q", sel)
	}
}

func TestCursorClamping_Top(t *testing.T) {
	m := New(testProfiles, "default")
	m.SetSize(80, 24)

	// Move up from position 0 — cursor should stay at 0.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	sel := m.Selected()
	if sel != "default" {
		t.Errorf("cursor should be clamped at top, got %q", sel)
	}
}

func TestCursorClamping_Bottom(t *testing.T) {
	m := New(testProfiles, "default")
	m.SetSize(80, 24)

	// Move down well past the end — cursor should clamp at last item.
	for range 10 {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	sel := m.Selected()
	if sel != "production" {
		t.Errorf("cursor should be clamped at bottom, got %q", sel)
	}
}

func TestSelected_OnEnter(t *testing.T) {
	m := New(testProfiles, "default")
	m.SetSize(80, 24)

	// Enter should select the first profile (cursor starts at 0).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	sel := m.Selected()
	if sel != "default" {
		t.Errorf("Selected() = %q, want %q", sel, "default")
	}

	// Sentinel should clear after first read.
	if m.Selected() != "" {
		t.Error("Selected() second call should return empty string")
	}
}

func TestSelected_EmptyProfiles(t *testing.T) {
	m := New(nil, "")
	m.SetSize(80, 24)

	// Enter with no profiles should not set selected.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.Selected() != "" {
		t.Error("Selected() should be empty when no profiles available")
	}
}

func TestDismissed_OnEsc(t *testing.T) {
	m := New(testProfiles, "default")
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

func TestInputActive_AlwaysTrue(t *testing.T) {
	m := New(testProfiles, "default")
	if !m.InputActive() {
		t.Error("InputActive() should always return true")
	}

	// Also true with empty profiles.
	m2 := New(nil, "")
	if !m2.InputActive() {
		t.Error("InputActive() should always return true even with empty profiles")
	}
}

func TestSetSize_UpdatesDimensions(t *testing.T) {
	m := New(testProfiles, "default")
	m.SetSize(120, 40)

	if m.width != 120 {
		t.Errorf("width = %d, want 120", m.width)
	}
	if m.height != 40 {
		t.Errorf("height = %d, want 40", m.height)
	}
}

func TestView_ContainsProfileNames(t *testing.T) {
	m := New(testProfiles, "default")
	m.SetSize(80, 24)

	view := m.View()
	for _, p := range testProfiles {
		if !strings.Contains(view, p) {
			t.Errorf("View should contain profile %q", p)
		}
	}
}

func TestView_ContainsTitle(t *testing.T) {
	m := New(testProfiles, "default")
	m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, "Switch Profile") {
		t.Error("View should contain title 'Switch Profile'")
	}
}

func TestView_ContainsActiveMarker(t *testing.T) {
	m := New(testProfiles, "staging")
	m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, "(active)") {
		t.Error("View should contain '(active)' marker for the active profile")
	}
}

func TestView_EmptyProfiles(t *testing.T) {
	m := New(nil, "")
	m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, "No profiles configured") {
		t.Error("View should show 'No profiles configured' when list is empty")
	}
}

func TestView_ContainsHelpText(t *testing.T) {
	m := New(testProfiles, "default")
	m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, "j/k") {
		t.Error("View should contain j/k navigation help")
	}
	if !strings.Contains(view, "enter") {
		t.Error("View should contain enter help")
	}
	if !strings.Contains(view, "esc") {
		t.Error("View should contain esc help")
	}
}

func TestView_EmptyProfilesContainsEscHelp(t *testing.T) {
	m := New(nil, "")
	m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, "esc") {
		t.Error("View for empty profiles should contain esc help")
	}
}

func TestView_CursorIndicator(t *testing.T) {
	m := New(testProfiles, "default")
	m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, ">") {
		t.Error("View should contain cursor indicator '>'")
	}
}

func TestUpdate_UnhandledMessageIsNoop(t *testing.T) {
	m := New(testProfiles, "default")
	m.SetSize(80, 24)

	// A non-key message should not change anything.
	m, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})

	if cmd != nil {
		t.Error("Update should return nil cmd for unhandled message")
	}
	if m.Selected() != "" {
		t.Error("Selected() should be empty after unhandled message")
	}
	if m.Dismissed() {
		t.Error("Dismissed() should be false after unhandled message")
	}
}
