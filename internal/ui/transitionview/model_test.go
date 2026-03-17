package transitionview

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/seanhalberthal/jiru/internal/jira"
)

var testTransitions = []jira.Transition{
	{ID: "11", Name: "To Do"},
	{ID: "21", Name: "In Progress"},
	{ID: "31", Name: "Done"},
}

func TestNew_StartsInLoadingState(t *testing.T) {
	m := New("PROJ-1")
	if m.IssueKey() != "PROJ-1" {
		t.Errorf("IssueKey() = %q, want %q", m.IssueKey(), "PROJ-1")
	}
	if !m.InputActive() {
		t.Error("InputActive() should always return true")
	}
}

func TestLoading_SuppressesAllKeys(t *testing.T) {
	m := New("PROJ-1")
	m.SetSize(80, 24)

	// In loading state, all key presses should be suppressed.
	keys := []tea.KeyMsg{
		{Type: tea.KeyEnter},
		{Type: tea.KeyEsc},
		{Type: tea.KeyRunes, Runes: []rune{'j'}},
		{Type: tea.KeyRunes, Runes: []rune{'k'}},
		{Type: tea.KeyUp},
		{Type: tea.KeyDown},
	}

	for _, k := range keys {
		m, _ = m.Update(k)
	}

	if m.Selected() != nil {
		t.Error("Selected() should be nil in loading state")
	}
	if m.Dismissed() {
		t.Error("Dismissed() should be false in loading state")
	}
}

func TestSetTransitions_ExitsLoadingState(t *testing.T) {
	m := New("PROJ-1")
	m.SetSize(80, 24)
	m.SetTransitions(testTransitions)

	view := m.View()
	if strings.Contains(view, "Loading") {
		t.Error("View should not show loading text after SetTransitions")
	}

	for _, tr := range testTransitions {
		if !strings.Contains(view, tr.Name) {
			t.Errorf("View should contain transition %q", tr.Name)
		}
	}
}

func TestSelected_OnEnter(t *testing.T) {
	m := New("PROJ-1")
	m.SetSize(80, 24)
	m.SetTransitions(testTransitions)

	// Enter should select the first transition (cursor starts at 0).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	sel := m.Selected()
	if sel == nil {
		t.Fatal("Selected() should not be nil after Enter")
	}
	if sel.ID != "11" || sel.Name != "To Do" {
		t.Errorf("Selected() = {%q, %q}, want {\"11\", \"To Do\"}", sel.ID, sel.Name)
	}

	// Sentinel should clear after first read.
	if m.Selected() != nil {
		t.Error("Selected() second call should return nil")
	}
}

func TestSelected_EmptyTransitions(t *testing.T) {
	m := New("PROJ-1")
	m.SetSize(80, 24)
	m.SetTransitions(nil)

	// Enter with no transitions should not set selected.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.Selected() != nil {
		t.Error("Selected() should be nil when no transitions available")
	}
}

func TestDismissed_OnEsc(t *testing.T) {
	m := New("PROJ-1")
	m.SetSize(80, 24)
	m.SetTransitions(testTransitions)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if !m.Dismissed() {
		t.Error("Dismissed() should return true after Esc")
	}

	// Sentinel should clear after first read.
	if m.Dismissed() {
		t.Error("Dismissed() second call should return false")
	}
}

func TestCursorNavigation_JK(t *testing.T) {
	m := New("PROJ-1")
	m.SetSize(80, 24)
	m.SetTransitions(testTransitions)

	// Cursor starts at 0. Move down with j.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sel := m.Selected()
	if sel == nil || sel.Name != "In Progress" {
		t.Errorf("after j, expected 'In Progress', got %v", sel)
	}
}

func TestCursorNavigation_UpDown(t *testing.T) {
	m := New("PROJ-1")
	m.SetSize(80, 24)
	m.SetTransitions(testTransitions)

	// Move down twice with arrow keys.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sel := m.Selected()
	if sel == nil || sel.Name != "Done" {
		t.Errorf("after 2x down, expected 'Done', got %v", sel)
	}
}

func TestCursorClamping_Top(t *testing.T) {
	m := New("PROJ-1")
	m.SetSize(80, 24)
	m.SetTransitions(testTransitions)

	// Move up from position 0 — cursor should stay at 0.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	sel := m.Selected()
	if sel == nil || sel.Name != "To Do" {
		t.Errorf("cursor should be clamped at top, got %v", sel)
	}
}

func TestCursorClamping_Bottom(t *testing.T) {
	m := New("PROJ-1")
	m.SetSize(80, 24)
	m.SetTransitions(testTransitions)

	// Move down well past the end — cursor should clamp at last item.
	for range 10 {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	sel := m.Selected()
	if sel == nil || sel.Name != "Done" {
		t.Errorf("cursor should be clamped at bottom, got %v", sel)
	}
}

func TestCursorNavigation_UpAfterDown(t *testing.T) {
	m := New("PROJ-1")
	m.SetSize(80, 24)
	m.SetTransitions(testTransitions)

	// Down, down, up — should be at index 1.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	sel := m.Selected()
	if sel == nil || sel.Name != "In Progress" {
		t.Errorf("expected 'In Progress' after down-down-up, got %v", sel)
	}
}

func TestView_Loading(t *testing.T) {
	m := New("PROJ-1")
	m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, "Loading") {
		t.Error("View should show loading text when transitions not yet loaded")
	}
	if !strings.Contains(view, "PROJ-1") {
		t.Error("View should contain the issue key")
	}
}

func TestView_EmptyTransitions(t *testing.T) {
	m := New("PROJ-1")
	m.SetSize(80, 24)
	m.SetTransitions(nil)

	view := m.View()
	if !strings.Contains(view, "No transitions") {
		t.Error("View should show 'No transitions' when list is empty")
	}
}

func TestView_ContainsHelpText(t *testing.T) {
	m := New("PROJ-1")
	m.SetSize(80, 24)
	m.SetTransitions(testTransitions)

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

func TestInputActive_AlwaysTrue(t *testing.T) {
	m := New("PROJ-1")
	if !m.InputActive() {
		t.Error("InputActive() should always return true")
	}
}
