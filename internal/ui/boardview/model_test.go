package boardview

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/seanhalberthal/jiru/internal/jira"
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

func TestSmallHeightShowsHeadersAndCards(t *testing.T) {
	m := New()
	m.SetIssues(testIssues(), "Test Board")

	// Test a range of small heights — none should panic or lose headers.
	for _, h := range []int{5, 8, 10, 12, 15} {
		m.SetSize(120, h)
		view := m.View()
		if view == "" {
			t.Errorf("height %d: expected non-empty view", h)
		}
		// Title must always be present.
		if !strings.Contains(view, "Test Board") {
			t.Errorf("height %d: title 'Test Board' missing from view", h)
		}
		// At least one column header must be present.
		hasHeader := strings.Contains(view, "To Do") ||
			strings.Contains(view, "In Progress") ||
			strings.Contains(view, "Done")
		if !hasHeader {
			t.Errorf("height %d: no column header found in view", h)
		}
	}
}

func TestSmallWidthShowsContent(t *testing.T) {
	m := New()
	m.SetIssues(testIssues(), "Test Board")

	// Test narrow widths — columns should clamp to minimum width.
	for _, w := range []int{20, 30, 40, 50} {
		m.SetSize(w, 40)
		view := m.View()
		if view == "" {
			t.Errorf("width %d: expected non-empty view", w)
		}
		// Column widths should be at least 12.
		for i, col := range m.columns {
			if col.width < 12 {
				t.Errorf("width %d, col %d: width %d below minimum 12", w, i, col.width)
			}
		}
	}
}

func TestMinimumColumnHeight(t *testing.T) {
	m := New()
	m.SetIssues(testIssues(), "Test Board")

	// Very small height — column height should clamp to minimum 7.
	m.SetSize(120, 3)
	for i, col := range m.columns {
		if col.height < 7 {
			t.Errorf("col %d: height %d below minimum 7", i, col.height)
		}
	}
}

func TestViewHeightConstrainedToAvailable(t *testing.T) {
	m := New()
	m.SetSize(80, 15)
	m.SetIssues(testIssues(), "Test Board")

	view := m.View()
	viewHeight := lipgloss.Height(view)
	if viewHeight > 15 {
		t.Errorf("view height %d exceeds available height 15", viewHeight)
	}
}

func TestResizeFromLargeToSmall(t *testing.T) {
	m := New()
	m.SetSize(120, 40)
	m.SetIssues(testIssues(), "Test Board")

	// Resize to small — should still render correctly.
	m.SetSize(40, 10)
	view := m.View()
	if !strings.Contains(view, "Test Board") {
		t.Error("title missing after resize to small")
	}
	viewHeight := lipgloss.Height(view)
	if viewHeight > 10 {
		t.Errorf("view height %d exceeds available height 10 after resize", viewHeight)
	}
}

func TestAppendIssues_AddsToBoard(t *testing.T) {
	m := New()
	m.SetSize(120, 40)
	m.SetIssues([]jira.Issue{
		{Key: "A-1", Status: "To Do", Summary: "First"},
	}, "Board")

	// Append more issues.
	m.AppendIssues([]jira.Issue{
		{Key: "A-2", Status: "In Progress", Summary: "Second"},
		{Key: "A-3", Status: "To Do", Summary: "Third"},
	})

	if len(m.allIssues) != 3 {
		t.Errorf("expected 3 issues after append, got %d", len(m.allIssues))
	}

	// Verify columns were rebuilt — should now have two columns.
	if len(m.columns) != 2 {
		t.Errorf("expected 2 columns (To Do + In Progress), got %d", len(m.columns))
	}
}

func TestAppendIssues_DeduplicatesByKey(t *testing.T) {
	m := New()
	m.SetSize(120, 40)
	m.SetIssues([]jira.Issue{
		{Key: "A-1", Status: "To Do", Summary: "First"},
		{Key: "A-2", Status: "In Progress", Summary: "Second"},
	}, "Board")

	// Append overlapping page — A-2 already exists.
	m.AppendIssues([]jira.Issue{
		{Key: "A-2", Status: "In Progress", Summary: "Second"},
		{Key: "A-3", Status: "To Do", Summary: "Third"},
	})

	if len(m.allIssues) != 3 {
		t.Errorf("expected 3 issues (no duplicates), got %d", len(m.allIssues))
	}

	// Verify each key appears exactly once across columns.
	keys := make(map[string]int)
	for _, col := range m.columns {
		for _, iss := range col.issues {
			keys[iss.Key]++
		}
	}
	for key, count := range keys {
		if count != 1 {
			t.Errorf("issue %s appears %d times, expected 1", key, count)
		}
	}
}

func TestAppendIssues_PreservesCursorPosition(t *testing.T) {
	m := New()
	m.SetSize(120, 40)
	m.SetIssues([]jira.Issue{
		{Key: "A-1", Status: "To Do", Summary: "First"},
		{Key: "A-2", Status: "To Do", Summary: "Second"},
		{Key: "A-3", Status: "In Progress", Summary: "Third"},
	}, "Board")

	// Move cursor down in "To Do" column and switch to "In Progress".
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}) // cursor=1 in To Do
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")}) // switch to In Progress

	if m.activeCol != 1 {
		t.Fatalf("expected activeCol 1, got %d", m.activeCol)
	}
	if m.columns[0].cursor != 1 {
		t.Fatalf("expected To Do cursor 1, got %d", m.columns[0].cursor)
	}

	// Append new issues — cursor positions should be preserved.
	m.AppendIssues([]jira.Issue{
		{Key: "A-4", Status: "To Do", Summary: "Fourth"},
		{Key: "A-5", Status: "In Progress", Summary: "Fifth"},
	})

	if m.activeCol != 1 {
		t.Errorf("expected activeCol 1 after append, got %d", m.activeCol)
	}
	if m.columns[0].cursor != 1 {
		t.Errorf("expected To Do cursor 1 after append, got %d", m.columns[0].cursor)
	}
}
