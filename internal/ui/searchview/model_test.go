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
