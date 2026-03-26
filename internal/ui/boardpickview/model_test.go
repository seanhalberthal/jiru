package boardpickview

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/seanhalberthal/jiru/internal/jira"
)

func TestNew(t *testing.T) {
	m := New()
	if !m.loading {
		t.Error("expected loading state on init")
	}
	if m.Selected() != nil {
		t.Error("expected no selection on init")
	}
	if m.Dismissed() {
		t.Error("expected not dismissed on init")
	}
}

func TestSetBoards(t *testing.T) {
	m := New()
	m = m.SetBoards([]jira.Board{
		{ID: 1, Name: "Sprint Board", Type: "scrum"},
		{ID: 2, Name: "Kanban Board", Type: "kanban"},
	})
	if m.loading {
		t.Error("expected loading=false after SetBoards")
	}
}

func TestSelectBoard(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)
	m = m.SetBoards([]jira.Board{
		{ID: 1, Name: "Sprint Board", Type: "scrum"},
		{ID: 2, Name: "Kanban Board", Type: "kanban"},
	})

	// Select first board (enter).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	b := m.Selected()
	if b == nil {
		t.Fatal("expected a selected board")
	}
	if b.ID != 1 {
		t.Errorf("expected board ID 1, got %d", b.ID)
	}

	// Sentinel consumed.
	if m.Selected() != nil {
		t.Error("expected sentinel consumed on second call")
	}
}

func TestNavigateAndSelect(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)
	m = m.SetBoards([]jira.Board{
		{ID: 1, Name: "Board A", Type: "scrum"},
		{ID: 2, Name: "Board B", Type: "kanban"},
		{ID: 3, Name: "Board C", Type: "scrum"},
	})

	// Move down twice, select third board.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	b := m.Selected()
	if b == nil || b.ID != 3 {
		t.Errorf("expected board ID 3, got %v", b)
	}
}

func TestDismiss(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)
	m = m.SetBoards([]jira.Board{{ID: 1, Name: "Board", Type: "scrum"}})

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !m.Dismissed() {
		t.Error("expected dismissed after esc")
	}
	// Sentinel consumed.
	if m.Dismissed() {
		t.Error("expected sentinel consumed on second call")
	}
}

func TestVimKeys(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)
	m = m.SetBoards([]jira.Board{
		{ID: 1, Name: "Board A", Type: "scrum"},
		{ID: 2, Name: "Board B", Type: "kanban"},
	})

	// j moves down, enter selects.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	b := m.Selected()
	if b == nil || b.ID != 2 {
		t.Errorf("expected board ID 2 via j key, got %v", b)
	}
}

func TestEmptyBoards(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)
	m = m.SetBoards(nil)

	// Enter on empty list should not crash or select.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.Selected() != nil {
		t.Error("expected no selection on empty board list")
	}
}

func TestLoadingView(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)
	v := m.View()
	if !strings.Contains(v, "Loading") && !strings.Contains(v, "loading") {
		t.Error("expected loading indicator in view")
	}
}

func TestViewRendersBoards(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)
	m = m.SetBoards([]jira.Board{
		{ID: 1, Name: "My Board", Type: "scrum"},
	})
	v := m.View()
	if !strings.Contains(v, "My Board") {
		t.Errorf("expected board name in view, got: %s", v)
	}
}

func TestInputActive(t *testing.T) {
	m := New()
	if !m.InputActive() {
		t.Error("expected InputActive to return true")
	}
}

func TestSpaceSelectsBoard(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)
	m = m.SetBoards([]jira.Board{
		{ID: 1, Name: "Board A", Type: "scrum"},
	})

	// Space key selects.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	b := m.Selected()
	if b == nil || b.ID != 1 {
		t.Errorf("expected board ID 1 via space key, got %v", b)
	}
}

func TestCursorDoesNotOvershoot(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)
	m = m.SetBoards([]jira.Board{
		{ID: 1, Name: "Only Board", Type: "scrum"},
	})

	// Try to move down past the only item.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	b := m.Selected()
	if b == nil || b.ID != 1 {
		t.Errorf("cursor should stay on first item, got %v", b)
	}

	// Try to move up past zero.
	m = m.SetBoards([]jira.Board{
		{ID: 1, Name: "Board A", Type: "scrum"},
		{ID: 2, Name: "Board B", Type: "kanban"},
	})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	b = m.Selected()
	if b == nil || b.ID != 1 {
		t.Errorf("cursor should not go below zero, got %v", b)
	}
}
