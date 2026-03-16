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
