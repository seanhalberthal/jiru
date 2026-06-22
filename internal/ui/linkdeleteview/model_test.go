package linkdeleteview

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/undont/jiru/internal/jira"
)

var testLinks = []jira.LinkedIssue{
	{LinkID: "20001", Relation: "is blocked by", Key: "PROJ-2", Summary: "Upstream"},
	{LinkID: "20002", Relation: "blocks", Key: "PROJ-3", Summary: "Downstream"},
	{Relation: "relates to", Key: "PROJ-4"}, // No LinkID — not deletable.
}

func keyPress(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func TestNew_FiltersUndeletableLinks(t *testing.T) {
	m := New("PROJ-1", testLinks)
	if len(m.links) != 2 {
		t.Fatalf("links = %d, want 2 (links without a LinkID should be dropped)", len(m.links))
	}
	if !m.HasLinks() {
		t.Error("HasLinks() = false, want true")
	}
}

func TestNew_NoLinks(t *testing.T) {
	m := New("PROJ-1", []jira.LinkedIssue{{Relation: "relates to", Key: "PROJ-9"}})
	if m.HasLinks() {
		t.Error("HasLinks() = true, want false when no links carry an ID")
	}
}

func TestUpdate_PickAndConfirmDelete(t *testing.T) {
	m := New("PROJ-1", testLinks)

	// Move to the second link, then open the confirmation.
	m, _ = m.Update(keyPress("j"))
	m, _ = m.Update(keyPress("enter"))
	if !m.confirming {
		t.Fatal("should be in confirming state after enter")
	}
	if !m.InputActive() {
		t.Error("InputActive() should be true while confirming")
	}

	// Confirm the deletion.
	m, _ = m.Update(keyPress("y"))
	req := m.Confirmed()
	if req == nil {
		t.Fatal("Confirmed() = nil, want a request")
	}
	if req.LinkID != "20002" || req.TargetKey != "PROJ-3" {
		t.Errorf("request = %+v, want LinkID=20002 TargetKey=PROJ-3", req)
	}
	// Sentinel clears after reading.
	if m.Confirmed() != nil {
		t.Error("Confirmed() should return nil on second read")
	}
}

func TestUpdate_CancelConfirmationReturnsToList(t *testing.T) {
	m := New("PROJ-1", testLinks)
	m, _ = m.Update(keyPress("enter"))
	if !m.confirming {
		t.Fatal("should be confirming")
	}
	m, _ = m.Update(keyPress("esc"))
	if m.confirming {
		t.Error("esc should cancel the confirmation, returning to the list")
	}
	if m.Dismissed() {
		t.Error("esc during confirmation should not dismiss the whole view")
	}
}

func TestUpdate_EscFromListDismisses(t *testing.T) {
	m := New("PROJ-1", testLinks)
	m, _ = m.Update(keyPress("esc"))
	if !m.Dismissed() {
		t.Error("esc from the list should dismiss the view")
	}
}
