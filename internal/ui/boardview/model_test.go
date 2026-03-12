package boardview

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/seanhalberthal/jiratui/internal/jira"
)

func testIssues() []jira.Issue {
	return []jira.Issue{
		{Key: "PROJ-1", Summary: "First task", Status: "To Do", IssueType: "Story", Assignee: "Alice", ParentKey: "PROJ-100", ParentType: "Epic", ParentSummary: "Auth Overhaul"},
		{Key: "PROJ-2", Summary: "Second task", Status: "In Progress", IssueType: "Bug", Assignee: "Bob", ParentKey: "PROJ-100", ParentType: "Epic", ParentSummary: "Auth Overhaul"},
		{Key: "PROJ-3", Summary: "Third task", Status: "Done", IssueType: "Story", Assignee: "Charlie", ParentKey: "PROJ-101", ParentType: "Epic", ParentSummary: "Search Feature"},
		{Key: "PROJ-4", Summary: "Fourth task", Status: "To Do", IssueType: "Task", Assignee: "", ParentKey: "PROJ-100", ParentType: "Epic", ParentSummary: "Auth Overhaul"},
		{Key: "PROJ-5", Summary: "Fifth task", Status: "In Progress", IssueType: "Story", Assignee: "Alice"},
	}
}

func TestBuildColumnsGroupsByStatus(t *testing.T) {
	m := New()
	m.SetSize(120, 40)
	m.SetIssues(testIssues(), "Test Board")

	if len(m.columns) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(m.columns))
	}

	// Columns should be ordered: To Do → In Progress → Done.
	if m.columns[0].name != "To Do" {
		t.Errorf("expected first column 'To Do', got %q", m.columns[0].name)
	}
	if m.columns[1].name != "In Progress" {
		t.Errorf("expected second column 'In Progress', got %q", m.columns[1].name)
	}
	if m.columns[2].name != "Done" {
		t.Errorf("expected third column 'Done', got %q", m.columns[2].name)
	}
}

func TestColumnIssueCounts(t *testing.T) {
	m := New()
	m.SetSize(120, 40)
	m.SetIssues(testIssues(), "Test Board")

	if len(m.columns[0].issues) != 2 {
		t.Errorf("expected 2 'To Do' issues, got %d", len(m.columns[0].issues))
	}
	if len(m.columns[1].issues) != 2 {
		t.Errorf("expected 2 'In Progress' issues, got %d", len(m.columns[1].issues))
	}
	if len(m.columns[2].issues) != 1 {
		t.Errorf("expected 1 'Done' issue, got %d", len(m.columns[2].issues))
	}
}

func TestEmptyStatusDoesNotCreateColumn(t *testing.T) {
	issues := []jira.Issue{
		{Key: "PROJ-1", Status: "To Do"},
		{Key: "PROJ-2", Status: "To Do"},
	}
	m := New()
	m.SetSize(120, 40)
	m.SetIssues(issues, "Single Status")

	if len(m.columns) != 1 {
		t.Fatalf("expected 1 column, got %d", len(m.columns))
	}
	if m.columns[0].name != "To Do" {
		t.Errorf("expected column 'To Do', got %q", m.columns[0].name)
	}
}

func TestColumnNavigation(t *testing.T) {
	m := New()
	m.SetSize(120, 40)
	m.SetIssues(testIssues(), "Test Board")

	if m.activeCol != 0 {
		t.Fatalf("expected initial column 0, got %d", m.activeCol)
	}

	// Move right.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	if m.activeCol != 1 {
		t.Errorf("expected column 1 after 'l', got %d", m.activeCol)
	}

	// Move right again.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	if m.activeCol != 2 {
		t.Errorf("expected column 2 after second 'l', got %d", m.activeCol)
	}

	// Move right at end — should stay at last column.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	if m.activeCol != 2 {
		t.Errorf("expected column 2 (clamped), got %d", m.activeCol)
	}

	// Move left.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	if m.activeCol != 1 {
		t.Errorf("expected column 1 after 'h', got %d", m.activeCol)
	}
}

func TestVerticalNavigation(t *testing.T) {
	m := New()
	m.SetSize(120, 40)
	m.SetIssues(testIssues(), "Test Board")

	// Column 0 (To Do) has 2 issues.
	if m.columns[0].cursor != 0 {
		t.Fatalf("expected initial cursor 0, got %d", m.columns[0].cursor)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m.columns[0].cursor != 1 {
		t.Errorf("expected cursor 1 after 'j', got %d", m.columns[0].cursor)
	}

	// Move down at end — should stay at last issue.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m.columns[0].cursor != 1 {
		t.Errorf("expected cursor 1 (clamped), got %d", m.columns[0].cursor)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if m.columns[0].cursor != 0 {
		t.Errorf("expected cursor 0 after 'k', got %d", m.columns[0].cursor)
	}
}

func TestIssueSelection(t *testing.T) {
	m := New()
	m.SetSize(120, 40)
	m.SetIssues(testIssues(), "Test Board")

	// Select first issue in first column.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	iss, ok := m.SelectedIssue()
	if !ok {
		t.Fatal("expected a selected issue")
	}
	if iss.Key != "PROJ-1" {
		t.Errorf("expected PROJ-1, got %s", iss.Key)
	}

	// SelectedIssue should reset after reading.
	_, ok = m.SelectedIssue()
	if ok {
		t.Error("expected no selected issue after reset")
	}
}

func TestParentFilterCycling(t *testing.T) {
	m := New()
	m.SetSize(120, 40)
	m.SetIssues(testIssues(), "Test Board")

	groups := m.ParentGroups()
	if len(groups) != 2 {
		t.Fatalf("expected 2 parent groups, got %d", len(groups))
	}

	// Initially no filter.
	if m.ParentFilter() != "" {
		t.Errorf("expected empty filter, got %q", m.ParentFilter())
	}

	// Cycle to first parent.
	m.SetParentFilter(groups[0].Key)
	if m.ParentFilter() != "PROJ-100" {
		t.Errorf("expected filter PROJ-100, got %q", m.ParentFilter())
	}

	// Check filtered issue count (PROJ-100 has 3 issues: PROJ-1, PROJ-2, PROJ-4).
	totalFiltered := 0
	for _, col := range m.columns {
		totalFiltered += len(col.issues)
	}
	if totalFiltered != 3 {
		t.Errorf("expected 3 filtered issues, got %d", totalFiltered)
	}

	// Cycle to second parent.
	m.SetParentFilter(groups[1].Key)
	if m.ParentFilter() != "PROJ-101" {
		t.Errorf("expected filter PROJ-101, got %q", m.ParentFilter())
	}

	// Clear filter.
	m.SetParentFilter("")
	if m.ParentFilter() != "" {
		t.Errorf("expected empty filter after clear, got %q", m.ParentFilter())
	}
	totalAll := 0
	for _, col := range m.columns {
		totalAll += len(col.issues)
	}
	if totalAll != 5 {
		t.Errorf("expected 5 total issues after clearing filter, got %d", totalAll)
	}
}

func TestDynamicParentLabel(t *testing.T) {
	// All same type — should use "Epic".
	m := New()
	m.SetSize(120, 40)
	m.SetIssues(testIssues(), "Test Board")
	if m.ParentLabel() != "Epic" {
		t.Errorf("expected label 'Epic', got %q", m.ParentLabel())
	}

	// Mixed types — should use "Parent".
	mixed := []jira.Issue{
		{Key: "A-1", Status: "To Do", ParentKey: "A-100", ParentType: "Epic"},
		{Key: "A-2", Status: "To Do", ParentKey: "A-200", ParentType: "Feature"},
	}
	m2 := New()
	m2.SetSize(120, 40)
	m2.SetIssues(mixed, "Mixed")
	if m2.ParentLabel() != "Parent" {
		t.Errorf("expected label 'Parent', got %q", m2.ParentLabel())
	}

	// No parents — should use "Parent".
	noParents := []jira.Issue{
		{Key: "B-1", Status: "To Do"},
	}
	m3 := New()
	m3.SetSize(120, 40)
	m3.SetIssues(noParents, "No Parents")
	if m3.ParentLabel() != "Parent" {
		t.Errorf("expected label 'Parent', got %q", m3.ParentLabel())
	}
}

func TestViewTogglePreservesData(t *testing.T) {
	issues := testIssues()
	m := New()
	m.SetSize(120, 40)
	m.SetIssues(issues, "Sprint 12")

	// Verify all issues present.
	total := 0
	for _, col := range m.columns {
		total += len(col.issues)
	}
	if total != 5 {
		t.Errorf("expected 5 issues, got %d", total)
	}

	// Set issues again (simulating toggle back).
	m.SetIssues(issues, "Sprint 12")
	total = 0
	for _, col := range m.columns {
		total += len(col.issues)
	}
	if total != 5 {
		t.Errorf("expected 5 issues after re-set, got %d", total)
	}
}

func TestWindowResizeRedistributesColumns(t *testing.T) {
	m := New()
	m.SetSize(120, 40)
	m.SetIssues(testIssues(), "Test Board")

	nCols := len(m.columns)
	expectedWidth := (120 - (nCols - 1)) / nCols
	for i, col := range m.columns {
		if col.width != expectedWidth {
			t.Errorf("column %d: expected width %d, got %d", i, expectedWidth, col.width)
		}
	}

	// Resize.
	m.SetSize(90, 30)
	expectedWidth = (90 - (nCols - 1)) / nCols
	for i, col := range m.columns {
		if col.width != expectedWidth {
			t.Errorf("column %d after resize: expected width %d, got %d", i, expectedWidth, col.width)
		}
	}
}

func TestNoIssuesView(t *testing.T) {
	m := New()
	m.SetSize(120, 40)
	m.SetIssues(nil, "Empty Board")

	view := m.View()
	if view == "" {
		t.Error("expected non-empty view for empty board")
	}
}
