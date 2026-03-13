package homeview

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/seanhalberthal/jiratui/internal/jira"
)

func testBoards() []jira.BoardStats {
	return []jira.BoardStats{
		{
			Board:        jira.Board{ID: 1, Name: "Team Alpha", Type: "scrum"},
			ActiveSprint: "Sprint 12",
			OpenIssues:   5,
			InProgress:   3,
			DoneIssues:   2,
			TotalIssues:  10,
		},
		{
			Board: jira.Board{ID: 2, Name: "Team Beta", Type: "kanban"},
		},
	}
}

func TestSetBoardsPopulatesList(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetBoards(testBoards())

	if len(m.boards) != 2 {
		t.Errorf("expected 2 boards, got %d", len(m.boards))
	}
}

func TestSelectedBoardSentinelReset(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetBoards(testBoards())

	// Select first board.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	b := m.SelectedBoard()
	if b == nil {
		t.Fatal("expected selected board")
	}
	if b.ID != 1 {
		t.Errorf("expected board ID 1, got %d", b.ID)
	}

	// Sentinel should reset.
	b = m.SelectedBoard()
	if b != nil {
		t.Error("expected nil board after reset")
	}
}

func TestSelectedBoard_LKey(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetBoards(testBoards())

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	b := m.SelectedBoard()
	if b == nil {
		t.Error("expected board selection via 'l' key")
	}
}

func TestNoSelectionOnEmptyBoards(t *testing.T) {
	m := New()
	m.SetSize(80, 24)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	b := m.SelectedBoard()
	if b != nil {
		t.Error("expected no selection on empty boards")
	}
}

func TestBoardItemFilterValue(t *testing.T) {
	item := boardItem{stats: jira.BoardStats{
		Board: jira.Board{ID: 1, Name: "My Board"},
	}}
	fv := item.FilterValue()
	if fv != "My Board" {
		t.Errorf("FilterValue() = %q, want %q", fv, "My Board")
	}
}

func TestView_NonEmpty(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetBoards(testBoards())

	view := m.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}
