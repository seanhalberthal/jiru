package sprintview

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/seanhalberthal/jiru/internal/jira"
)

func testIssues() []jira.Issue {
	return []jira.Issue{
		{Key: "PROJ-1", Summary: "First task", Status: "To Do", IssueType: "Story", Assignee: "Alice"},
		{Key: "PROJ-2", Summary: "Second task", Status: "In Progress", IssueType: "Bug", Assignee: "Bob"},
		{Key: "PROJ-3", Summary: "Third task", Status: "Done", IssueType: "Story", Assignee: "Charlie"},
	}
}

func TestSetIssuesPopulatesList(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)
	m = m.SetIssues(testIssues())

	if len(m.issues) != 3 {
		t.Errorf("expected 3 issues, got %d", len(m.issues))
	}
}

func TestSelectedIssueSentinelReset(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)
	m = m.SetIssues(testIssues())

	// Select first issue.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	iss, ok := m.SelectedIssue()
	if !ok {
		t.Fatal("expected selected issue")
	}
	if iss.Key != "PROJ-1" {
		t.Errorf("expected PROJ-1, got %s", iss.Key)
	}

	// Sentinel should reset.
	_, ok = m.SelectedIssue()
	if ok {
		t.Error("expected no selected issue after reset")
	}
}

func TestSelectedIssue_LKey_DoesNotSelect(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)
	m = m.SetIssues(testIssues())

	// 'l' is no longer an open key — only enter opens.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	_, ok := m.SelectedIssue()
	if ok {
		t.Error("expected no selection via 'l' key")
	}
}

func TestNoSelectionOnEmptyList(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_, ok := m.SelectedIssue()
	if ok {
		t.Error("expected no selection on empty list")
	}
}

func TestFilterValueFormat(t *testing.T) {
	item := issueItem{Issue: jira.Issue{Key: "PROJ-1", Summary: "Test"}}
	want := "PROJ-1 Test"
	if item.FilterValue() != want {
		t.Errorf("FilterValue() = %q, want %q", item.FilterValue(), want)
	}
}

func TestView_NonEmpty(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)
	m = m.SetIssues(testIssues())

	view := m.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestAppendIssues(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)
	m = m.SetIssues([]jira.Issue{{Key: "A-1", Summary: "First"}})

	m = m.AppendIssues([]jira.Issue{{Key: "A-2", Summary: "Second"}, {Key: "A-3", Summary: "Third"}})

	if len(m.issues) != 3 {
		t.Errorf("expected 3 issues after append, got %d", len(m.issues))
	}
	if m.issues[2].Key != "A-3" {
		t.Errorf("expected last issue key 'A-3', got %q", m.issues[2].Key)
	}
}

func TestAppendIssues_RefreshesDuringFiltering(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)
	m = m.SetIssues([]jira.Issue{{Key: "A-1", Summary: "First"}})

	// Activate list filtering by pressing '/'.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	if !m.Filtering() {
		t.Fatal("expected filtering to be active after '/'")
	}

	// Append while filtering — items should be immediately refreshed in the list
	// so newly loaded pages participate in the fuzzy match.
	m = m.AppendIssues([]jira.Issue{{Key: "A-2", Summary: "Second"}})
	if len(m.issues) != 2 {
		t.Errorf("expected 2 issues in backing slice, got %d", len(m.issues))
	}
	// List widget should have both items (re-applied filter with empty query matches all).
	if len(m.list.Items()) != 2 {
		t.Errorf("expected 2 items in list widget during filtering, got %d", len(m.list.Items()))
	}
}

func TestUpdateIssueStatus_MatchingKey(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)
	m = m.SetIssues(testIssues())

	m = m.UpdateIssueStatus("PROJ-2", "Done")

	// The backing slice should reflect the new status.
	found := false
	for _, iss := range m.issues {
		if iss.Key == "PROJ-2" {
			found = true
			if iss.Status != "Done" {
				t.Errorf("expected status 'Done', got %q", iss.Status)
			}
		}
	}
	if !found {
		t.Error("expected PROJ-2 in issues slice")
	}

	// The list widget items should also be updated.
	for _, item := range m.list.Items() {
		if it, ok := item.(issueItem); ok && it.Key() == "PROJ-2" {
			if it.Issue.Status != "Done" {
				t.Errorf("expected list item status 'Done', got %q", it.Issue.Status)
			}
		}
	}
}

func TestUpdateIssueStatus_NonMatchingKey(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)
	m = m.SetIssues(testIssues())

	// Update a key that doesn't exist — should be a no-op.
	m = m.UpdateIssueStatus("NOPE-999", "Done")

	// All statuses should remain unchanged.
	for _, iss := range m.issues {
		switch iss.Key {
		case "PROJ-1":
			if iss.Status != "To Do" {
				t.Errorf("PROJ-1 status changed unexpectedly to %q", iss.Status)
			}
		case "PROJ-2":
			if iss.Status != "In Progress" {
				t.Errorf("PROJ-2 status changed unexpectedly to %q", iss.Status)
			}
		case "PROJ-3":
			if iss.Status != "Done" {
				t.Errorf("PROJ-3 status changed unexpectedly to %q", iss.Status)
			}
		}
	}
}

func TestUpdateIssueStatus_MultipleIssues(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)
	m = m.SetIssues(testIssues())

	// Update two different issues sequentially.
	m = m.UpdateIssueStatus("PROJ-1", "In Progress")
	m = m.UpdateIssueStatus("PROJ-3", "To Do")

	statuses := make(map[string]string)
	for _, iss := range m.issues {
		statuses[iss.Key] = iss.Status
	}
	if statuses["PROJ-1"] != "In Progress" {
		t.Errorf("PROJ-1 status = %q, want 'In Progress'", statuses["PROJ-1"])
	}
	if statuses["PROJ-2"] != "In Progress" {
		t.Errorf("PROJ-2 status should be unchanged, got %q", statuses["PROJ-2"])
	}
	if statuses["PROJ-3"] != "To Do" {
		t.Errorf("PROJ-3 status = %q, want 'To Do'", statuses["PROJ-3"])
	}
}

func TestUpdateIssueStatus_PreservesSelectedIndex(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)
	m = m.SetIssues(testIssues())

	// Move cursor to the second item.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	idxBefore := m.list.Index()
	if idxBefore != 1 {
		t.Fatalf("expected cursor at index 1, got %d", idxBefore)
	}

	// Update the first item's status — cursor should stay at index 1.
	m = m.UpdateIssueStatus("PROJ-1", "Done")
	if m.list.Index() != idxBefore {
		t.Errorf("expected cursor to remain at %d, got %d", idxBefore, m.list.Index())
	}
}

func TestSetLoading_ShowsIndicator(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)
	m = m.SetIssues([]jira.Issue{{Key: "A-1", Summary: "First"}})

	m = m.SetLoading(true)
	if m.list.Title != "Issues (1) loading..." {
		t.Errorf("expected loading title, got %q", m.list.Title)
	}

	m = m.SetLoading(false)
	if m.list.Title != "Issues (1)" {
		t.Errorf("expected normal title, got %q", m.list.Title)
	}
}
