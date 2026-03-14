package searchview

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/seanhalberthal/jiratui/internal/jira"
)

func TestSetResults_TransitionsToResultsState(t *testing.T) {
	m := New()
	m.Show()
	m.SetSize(80, 24)

	issues := []jira.Issue{
		{Key: "PROJ-1", Summary: "Found issue", Status: "To Do", IssueType: "Story"},
	}
	m.SetResults(issues, "assignee = currentUser()")

	view := m.View()
	if view == "" {
		t.Error("expected non-empty view in results state")
	}
}

func TestResults_EscReturnsToInput(t *testing.T) {
	m := New()
	m.Show()
	m.SetSize(80, 24)

	issues := []jira.Issue{
		{Key: "PROJ-1", Summary: "Found issue", Status: "To Do", IssueType: "Story"},
	}
	m.SetResults(issues, "query")

	// Esc should return to input state.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	view := m.View()
	if !strings.Contains(view, "Search Issues") {
		t.Error("expected input state view after Esc from results")
	}
}

func TestResults_EnterSelectsIssue(t *testing.T) {
	m := New()
	m.Show()
	m.SetSize(80, 24)

	issues := []jira.Issue{
		{Key: "PROJ-1", Summary: "Found issue", Status: "To Do", IssueType: "Story"},
	}
	m.SetResults(issues, "query")

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	iss := m.SelectedIssue()
	if iss == nil {
		t.Fatal("expected selected issue")
	}
	if iss.Key != "PROJ-1" {
		t.Errorf("expected PROJ-1, got %s", iss.Key)
	}

	// Sentinel should reset.
	iss = m.SelectedIssue()
	if iss != nil {
		t.Error("expected nil after sentinel reset")
	}
}

func TestDismissed_OnEscWithEmptyInput(t *testing.T) {
	m := New()
	m.Show()
	m.SetSize(80, 24)

	// Esc on empty input should dismiss.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !m.Dismissed() {
		t.Error("expected dismissed on Esc with empty input")
	}
	// Sentinel should reset.
	if m.Dismissed() {
		t.Error("expected dismissed to reset after read")
	}
}

func TestDismissed_NotOnEscWithContent(t *testing.T) {
	m := New()
	m.Show()
	m.SetSize(80, 24)

	// Type something first.
	m.input.SetValue("some query")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.Dismissed() {
		t.Error("expected not dismissed when input has content")
	}
}

func TestSetMetadata_PopulatesValues(t *testing.T) {
	m := New()
	meta := &jira.JQLMetadata{
		Statuses:    []string{"To Do", "In Progress", "Done"},
		IssueTypes:  []string{"Bug", "Story"},
		Priorities:  []string{"High", "Medium", "Low"},
		Resolutions: []string{"Fixed", "Won't Fix"},
		Projects:    []string{"PROJ", "TEST"},
		Labels:      []string{"frontend", "backend"},
		Components:  []string{"API", "UI"},
		Versions:    []string{"1.0", "2.0"},
		Sprints:     []string{"Sprint 1", "Sprint 2"},
	}
	m.SetMetadata(meta)

	if m.values == nil {
		t.Fatal("expected values to be populated")
	}
	if len(m.values.Statuses) != 3 {
		t.Errorf("expected 3 statuses, got %d", len(m.values.Statuses))
	}
	if len(m.values.IssueTypes) != 2 {
		t.Errorf("expected 2 issue types, got %d", len(m.values.IssueTypes))
	}
	if len(m.values.Projects) != 2 {
		t.Errorf("expected 2 projects, got %d", len(m.values.Projects))
	}
}

func TestSetMetadata_NilIsNoop(t *testing.T) {
	m := New()
	m.SetMetadata(nil)
	if m.values != nil {
		t.Error("expected values to remain nil after SetMetadata(nil)")
	}
}

func TestSetUserResults_PopulatesUsers(t *testing.T) {
	m := New()
	m.SetUserResults([]string{"Alice", "Bob"})

	if m.values == nil {
		t.Fatal("expected values to be created")
	}
	if len(m.values.Users) != 2 {
		t.Errorf("expected 2 users, got %d", len(m.values.Users))
	}
	if m.userPending {
		t.Error("expected userPending to be false after SetUserResults")
	}
}

func TestSetUserResults_WithExistingMetadata(t *testing.T) {
	m := New()
	m.SetMetadata(&jira.JQLMetadata{
		Statuses: []string{"Done"},
	})
	m.SetUserResults([]string{"Alice"})

	// Should not overwrite existing metadata.
	if len(m.values.Statuses) != 1 {
		t.Errorf("expected statuses preserved, got %d", len(m.values.Statuses))
	}
	if len(m.values.Users) != 1 {
		t.Errorf("expected 1 user, got %d", len(m.values.Users))
	}
}

func TestNeedsUserSearch_InAssigneeContext(t *testing.T) {
	m := New()
	m.Show()
	m.SetSize(80, 24)
	m.input.SetValue("assignee = Al")
	m.input.SetCursor(13)

	prefix := m.NeedsUserSearch()
	if prefix != "Al" {
		t.Errorf("expected prefix 'Al', got %q", prefix)
	}
}

func TestNeedsUserSearch_TooShortPrefix(t *testing.T) {
	m := New()
	m.Show()
	m.SetSize(80, 24)
	m.input.SetValue("assignee = A")
	m.input.SetCursor(12)

	prefix := m.NeedsUserSearch()
	if prefix != "" {
		t.Errorf("expected empty (prefix too short), got %q", prefix)
	}
}

func TestNeedsUserSearch_NotInAssigneeContext(t *testing.T) {
	m := New()
	m.Show()
	m.SetSize(80, 24)
	m.input.SetValue("status = Do")
	m.input.SetCursor(11)

	prefix := m.NeedsUserSearch()
	if prefix != "" {
		t.Errorf("expected empty (not assignee field), got %q", prefix)
	}
}

func TestNeedsUserSearch_SamePrefixNotRepeated(t *testing.T) {
	m := New()
	m.Show()
	m.SetSize(80, 24)
	m.input.SetValue("assignee = Al")
	m.input.SetCursor(13)

	prefix := m.NeedsUserSearch()
	if prefix != "Al" {
		t.Fatalf("expected 'Al', got %q", prefix)
	}

	// Same prefix — should not trigger again.
	m.userPending = false // simulate completion
	prefix = m.NeedsUserSearch()
	if prefix != "" {
		t.Errorf("expected empty (same prefix already searched), got %q", prefix)
	}
}

func TestNeedsUserSearch_PendingBlocksNew(t *testing.T) {
	m := New()
	m.Show()
	m.SetSize(80, 24)
	m.input.SetValue("assignee = Ali")
	m.input.SetCursor(14)

	prefix := m.NeedsUserSearch()
	if prefix == "" {
		t.Fatal("expected non-empty prefix on first call")
	}

	// Change prefix but pending is still true.
	m.input.SetValue("assignee = Alic")
	m.input.SetCursor(15)
	prefix = m.NeedsUserSearch()
	if prefix != "" {
		t.Errorf("expected empty (pending blocks new search), got %q", prefix)
	}
}
