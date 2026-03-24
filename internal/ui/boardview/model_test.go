package boardview

import (
	"fmt"
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

func TestColumnNavigationFirstVisitCarriesCursor(t *testing.T) {
	// First visit to a column carries the source cursor (clamped).
	issues := []jira.Issue{
		{Key: "A-1", Status: "To Do"}, {Key: "A-2", Status: "To Do"},
		{Key: "A-3", Status: "To Do"}, {Key: "A-4", Status: "To Do"},
		{Key: "A-5", Status: "To Do"}, {Key: "A-6", Status: "To Do"},
		{Key: "B-1", Status: "In Progress"}, {Key: "B-2", Status: "In Progress"},
		{Key: "B-3", Status: "In Progress"}, {Key: "B-4", Status: "In Progress"},
		{Key: "C-1", Status: "Done"}, {Key: "C-2", Status: "Done"},
	}
	m := New()
	m.SetSize(120, 80)
	m.SetIssues(issues, "Board")

	// Move to card 6 (index 5) in "To Do".
	for i := 0; i < 5; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	}
	if m.columns[0].cursor != 5 {
		t.Fatalf("expected To Do cursor 5, got %d", m.columns[0].cursor)
	}

	// First visit to "In Progress" (4 issues) — carry cursor, clamped to 3.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	if m.columns[1].cursor != 3 {
		t.Errorf("expected In Progress cursor 3 (clamped from 5), got %d", m.columns[1].cursor)
	}

	// First visit to "Done" (2 issues) — carry cursor, clamped to 1.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	if m.columns[2].cursor != 1 {
		t.Errorf("expected Done cursor 1 (clamped from 3), got %d", m.columns[2].cursor)
	}
}

func TestColumnNavigationReturnVisitRestoresCursor(t *testing.T) {
	// Return visit to a previously-visited column restores its saved position.
	issues := []jira.Issue{
		{Key: "A-1", Status: "To Do"}, {Key: "A-2", Status: "To Do"},
		{Key: "A-3", Status: "To Do"}, {Key: "A-4", Status: "To Do"},
		{Key: "A-5", Status: "To Do"}, {Key: "A-6", Status: "To Do"},
		{Key: "B-1", Status: "In Progress"}, {Key: "B-2", Status: "In Progress"},
		{Key: "B-3", Status: "In Progress"}, {Key: "B-4", Status: "In Progress"},
	}
	m := New()
	m.SetSize(120, 80)
	m.SetIssues(issues, "Board")

	// Move to card 6 (index 5) in "To Do".
	for i := 0; i < 5; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	}
	if m.columns[0].cursor != 5 {
		t.Fatalf("expected To Do cursor 5, got %d", m.columns[0].cursor)
	}

	// First visit to "In Progress" — carries cursor, clamped to 3.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	if m.columns[1].cursor != 3 {
		t.Errorf("expected In Progress cursor 3 (clamped), got %d", m.columns[1].cursor)
	}

	// Move down to card 0 in "In Progress" for a distinct position.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	if m.columns[1].cursor != 0 {
		t.Fatalf("expected In Progress cursor 0 after 'g', got %d", m.columns[1].cursor)
	}

	// Return to "To Do" — should restore saved cursor 5 (not carry 0).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	if m.columns[0].cursor != 5 {
		t.Errorf("expected To Do cursor 5 (restored), got %d", m.columns[0].cursor)
	}

	// Return to "In Progress" — should restore saved cursor 0.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	if m.columns[1].cursor != 0 {
		t.Errorf("expected In Progress cursor 0 (restored), got %d", m.columns[1].cursor)
	}
}

func TestColumnNavigationFirstVisitExactFit(t *testing.T) {
	// First visit carries cursor position exactly when the column has enough issues.
	issues := []jira.Issue{
		{Key: "A-1", Status: "To Do"}, {Key: "A-2", Status: "To Do"},
		{Key: "A-3", Status: "To Do"},
		{Key: "B-1", Status: "In Progress"}, {Key: "B-2", Status: "In Progress"},
		{Key: "B-3", Status: "In Progress"}, {Key: "B-4", Status: "In Progress"},
		{Key: "B-5", Status: "In Progress"}, {Key: "B-6", Status: "In Progress"},
	}
	m := New()
	m.SetSize(120, 80)
	m.SetIssues(issues, "Board")

	// Move to card 2 (index 1) in "To Do".
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m.columns[0].cursor != 1 {
		t.Fatalf("expected To Do cursor 1, got %d", m.columns[0].cursor)
	}

	// First visit to "In Progress" (6 issues) — cursor should carry exactly at 1.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	if m.columns[1].cursor != 1 {
		t.Errorf("expected In Progress cursor 1 (exact carry), got %d", m.columns[1].cursor)
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

func TestUpdateIssueStatus_MovesCardAndFollowsCursor(t *testing.T) {
	m := New()
	m.SetSize(120, 40)
	m.SetIssues(testIssues(), "Test Board")
	// Columns: To Do (PROJ-1, PROJ-4), In Progress (PROJ-2, PROJ-5), Done (PROJ-3).

	// Select PROJ-1 in "To Do" (cursor 0, col 0).
	m.UpdateIssueStatus("PROJ-1", "In Progress")

	// Card should move to "In Progress" and cursor should follow.
	if m.activeCol >= len(m.columns) {
		t.Fatalf("activeCol %d out of range (columns: %d)", m.activeCol, len(m.columns))
	}
	if m.columns[m.activeCol].name != "In Progress" {
		t.Errorf("expected active column 'In Progress', got %q", m.columns[m.activeCol].name)
	}
	// Cursor should be on the moved card (inserted at top).
	iss := m.columns[m.activeCol].selectedIssue()
	if iss == nil || iss.Key != "PROJ-1" {
		t.Errorf("expected cursor on PROJ-1, got %v", iss)
	}
	// Issue status should be updated.
	if iss != nil && iss.Status != "In Progress" {
		t.Errorf("expected status 'In Progress', got %q", iss.Status)
	}
}

func TestUpdateIssueStatus_RemovesEmptySourceColumn(t *testing.T) {
	issues := []jira.Issue{
		{Key: "A-1", Status: "Review", Summary: "Only review item"},
		{Key: "A-2", Status: "Done", Summary: "Done item"},
	}
	m := New()
	m.SetSize(120, 40)
	m.SetIssues(issues, "Board")

	initialCols := len(m.columns)
	m.UpdateIssueStatus("A-1", "Done")

	// The "Review" column should be removed since it's now empty.
	if len(m.columns) != initialCols-1 {
		t.Errorf("expected %d columns after removing empty, got %d", initialCols-1, len(m.columns))
	}
	// Cursor should be on the moved card in "Done".
	iss := m.columns[m.activeCol].selectedIssue()
	if iss == nil || iss.Key != "A-1" {
		t.Errorf("expected cursor on A-1, got %v", iss)
	}
}

func TestUpdateIssueStatus_CreatesNewColumn(t *testing.T) {
	issues := []jira.Issue{
		{Key: "A-1", Status: "To Do", Summary: "Task one"},
		{Key: "A-2", Status: "To Do", Summary: "Task two"},
	}
	m := New()
	m.SetSize(120, 40)
	m.SetIssues(issues, "Board")

	if len(m.columns) != 1 {
		t.Fatalf("expected 1 column, got %d", len(m.columns))
	}

	// Move to a status that doesn't exist yet.
	m.UpdateIssueStatus("A-1", "In Progress")

	if len(m.columns) != 2 {
		t.Fatalf("expected 2 columns after move, got %d", len(m.columns))
	}
	// Cursor should follow the card.
	iss := m.columns[m.activeCol].selectedIssue()
	if iss == nil || iss.Key != "A-1" {
		t.Errorf("expected cursor on A-1, got %v", iss)
	}
}

func TestUpdateIssueStatus_AllIssuesUpdated(t *testing.T) {
	m := New()
	m.SetSize(120, 40)
	m.SetIssues(testIssues(), "Test Board")

	m.UpdateIssueStatus("PROJ-1", "Done")

	// Verify allIssues is updated.
	for _, iss := range m.allIssues {
		if iss.Key == "PROJ-1" && iss.Status != "Done" {
			t.Errorf("expected PROJ-1 status 'Done' in allIssues, got %q", iss.Status)
		}
	}
}

func TestUpdateIssueStatus_UnknownKeyIsNoOp(t *testing.T) {
	m := New()
	m.SetSize(120, 40)
	m.SetIssues(testIssues(), "Test Board")

	colsBefore := len(m.columns)
	activeBefore := m.activeCol

	m.UpdateIssueStatus("NOPE-999", "Done")

	if len(m.columns) != colsBefore {
		t.Errorf("expected %d columns (unchanged), got %d", colsBefore, len(m.columns))
	}
	if m.activeCol != activeBefore {
		t.Errorf("expected activeCol %d (unchanged), got %d", activeBefore, m.activeCol)
	}
}

func TestUpdateIssueStatus_NewColumnBecomesVisible(t *testing.T) {
	// Create 5+ status columns so not all fit in maxVisibleColumns (4).
	// Then transition a card to a NEW status that sorts to the far right,
	// forcing a column scroll.
	issues := []jira.Issue{
		{Key: "A-1", Status: "Backlog", Summary: "One"},
		{Key: "A-2", Status: "To Do", Summary: "Two"},
		{Key: "A-3", Status: "In Progress", Summary: "Three"},
		{Key: "A-4", Status: "In Review", Summary: "Four"},
		{Key: "A-5", Status: "QA", Summary: "Five"},
	}
	m := New()
	m.SetSize(120, 40)
	m.SetIssues(issues, "Board")

	// Start with cursor on the first column (leftmost).
	m.activeCol = 0
	m.ensureColumnVisible()

	// Move A-1 to a brand new "Done" column — sorts to the right end.
	m.UpdateIssueStatus("A-1", "Done")

	// Active column should be the new column containing A-1.
	iss := m.columns[m.activeCol].selectedIssue()
	if iss == nil || iss.Key != "A-1" {
		t.Fatalf("expected cursor on A-1, got %v", iss)
	}

	// The destination column must be within the visible window.
	start, end := m.visibleColumnRange()
	if m.activeCol < start || m.activeCol >= end {
		t.Errorf("active column %d not in visible range [%d, %d)", m.activeCol, start, end)
	}
}

// --- column.moveHalfPageDown / moveHalfPageUp tests ---

func TestMoveHalfPageDown_BasicScroll(t *testing.T) {
	issues := make([]jira.Issue, 20)
	for i := range issues {
		issues[i] = jira.Issue{Key: fmt.Sprintf("X-%d", i+1), Status: "Open"}
	}
	c := newColumn("Open", issues)
	c.setSize(30, 40) // Visible cards = (40-2)/6 = 6, half = 3.

	c.moveHalfPageDown()
	if c.cursor != 3 {
		t.Errorf("expected cursor 3 after half page down, got %d", c.cursor)
	}

	c.moveHalfPageDown()
	if c.cursor != 6 {
		t.Errorf("expected cursor 6 after second half page down, got %d", c.cursor)
	}
}

func TestMoveHalfPageDown_ClampsAtEnd(t *testing.T) {
	issues := make([]jira.Issue, 5)
	for i := range issues {
		issues[i] = jira.Issue{Key: fmt.Sprintf("X-%d", i+1), Status: "Open"}
	}
	c := newColumn("Open", issues)
	c.setSize(30, 40) // Visible cards = 6, half = 3.

	// First jump: 0 → 3.
	c.moveHalfPageDown()
	if c.cursor != 3 {
		t.Errorf("expected cursor 3, got %d", c.cursor)
	}

	// Second jump: 3+3=6 → clamped to 4 (last index).
	c.moveHalfPageDown()
	if c.cursor != 4 {
		t.Errorf("expected cursor 4 (clamped), got %d", c.cursor)
	}

	// Third jump: already at end, stays at 4.
	c.moveHalfPageDown()
	if c.cursor != 4 {
		t.Errorf("expected cursor 4 (still clamped), got %d", c.cursor)
	}
}

func TestMoveHalfPageDown_EmptyColumn(t *testing.T) {
	c := newColumn("Empty", nil)
	c.setSize(30, 40)

	// Should not panic and cursor should be 0.
	c.moveHalfPageDown()
	if c.cursor != 0 {
		t.Errorf("expected cursor 0 on empty column, got %d", c.cursor)
	}
}

func TestMoveHalfPageDown_SingleIssue(t *testing.T) {
	c := newColumn("Lone", []jira.Issue{{Key: "X-1", Status: "Open"}})
	c.setSize(30, 40)

	c.moveHalfPageDown()
	if c.cursor != 0 {
		t.Errorf("expected cursor 0 with single issue, got %d", c.cursor)
	}
}

func TestMoveHalfPageUp_BasicScroll(t *testing.T) {
	issues := make([]jira.Issue, 20)
	for i := range issues {
		issues[i] = jira.Issue{Key: fmt.Sprintf("X-%d", i+1), Status: "Open"}
	}
	c := newColumn("Open", issues)
	c.setSize(30, 40) // Visible cards = 6, half = 3.

	// Start at cursor 10.
	c.cursor = 10
	c.ensureVisible()

	c.moveHalfPageUp()
	if c.cursor != 7 {
		t.Errorf("expected cursor 7 after half page up, got %d", c.cursor)
	}

	c.moveHalfPageUp()
	if c.cursor != 4 {
		t.Errorf("expected cursor 4 after second half page up, got %d", c.cursor)
	}
}

func TestMoveHalfPageUp_ClampsAtBeginning(t *testing.T) {
	issues := make([]jira.Issue, 10)
	for i := range issues {
		issues[i] = jira.Issue{Key: fmt.Sprintf("X-%d", i+1), Status: "Open"}
	}
	c := newColumn("Open", issues)
	c.setSize(30, 40) // Visible cards = 6, half = 3.

	// Start at cursor 2.
	c.cursor = 2
	c.ensureVisible()

	// 2-3 = -1 → clamped to 0.
	c.moveHalfPageUp()
	if c.cursor != 0 {
		t.Errorf("expected cursor 0 (clamped), got %d", c.cursor)
	}

	// Already at 0, stays at 0.
	c.moveHalfPageUp()
	if c.cursor != 0 {
		t.Errorf("expected cursor 0 (still clamped), got %d", c.cursor)
	}
}

func TestMoveHalfPageUp_EmptyColumn(t *testing.T) {
	c := newColumn("Empty", nil)
	c.setSize(30, 40)

	// Should not panic.
	c.moveHalfPageUp()
	if c.cursor != 0 {
		t.Errorf("expected cursor 0 on empty column, got %d", c.cursor)
	}
}

func TestMoveHalfPageUp_SingleIssue(t *testing.T) {
	c := newColumn("Lone", []jira.Issue{{Key: "X-1", Status: "Open"}})
	c.setSize(30, 40)

	c.moveHalfPageUp()
	if c.cursor != 0 {
		t.Errorf("expected cursor 0 with single issue, got %d", c.cursor)
	}
}

func TestMoveHalfPage_SmallHeight(t *testing.T) {
	// With a very small height, visibleCards() could be 1, so half = 0, jump clamped to 1.
	issues := make([]jira.Issue, 5)
	for i := range issues {
		issues[i] = jira.Issue{Key: fmt.Sprintf("X-%d", i+1), Status: "Open"}
	}
	c := newColumn("Open", issues)
	c.setSize(30, 8) // visibleCards = (8-2)/6 = 1, half = 0, clamped to 1.

	c.moveHalfPageDown()
	if c.cursor != 1 {
		t.Errorf("expected cursor 1 with small height, got %d", c.cursor)
	}

	c.moveHalfPageUp()
	if c.cursor != 0 {
		t.Errorf("expected cursor 0 after half page up with small height, got %d", c.cursor)
	}
}

func TestMoveHalfPageDown_EnsuresVisibility(t *testing.T) {
	issues := make([]jira.Issue, 20)
	for i := range issues {
		issues[i] = jira.Issue{Key: fmt.Sprintf("X-%d", i+1), Status: "Open"}
	}
	c := newColumn("Open", issues)
	c.setSize(30, 20) // visibleCards = (20-2)/6 = 3, half = 1.

	// Jump several times to accumulate offset.
	for i := 0; i < 10; i++ {
		c.moveHalfPageDown()
	}

	// Cursor should be within the visible window.
	vis := c.visibleCards()
	if c.cursor < c.offset || c.cursor >= c.offset+vis {
		t.Errorf("cursor %d not visible (offset=%d, vis=%d)", c.cursor, c.offset, vis)
	}
}

func TestMoveHalfPageUp_EnsuresVisibility(t *testing.T) {
	issues := make([]jira.Issue, 20)
	for i := range issues {
		issues[i] = jira.Issue{Key: fmt.Sprintf("X-%d", i+1), Status: "Open"}
	}
	c := newColumn("Open", issues)
	c.setSize(30, 20)

	// Move to the end.
	c.cursor = 19
	c.ensureVisible()

	// Jump up several times.
	for i := 0; i < 10; i++ {
		c.moveHalfPageUp()
	}

	// Cursor should be within the visible window.
	vis := c.visibleCards()
	if c.cursor < c.offset || c.cursor >= c.offset+vis {
		t.Errorf("cursor %d not visible (offset=%d, vis=%d)", c.cursor, c.offset, vis)
	}
}

func TestKnownStatuses_SortedByCategory(t *testing.T) {
	// Provide known statuses in an order that doesn't match category ordering.
	// The board should reorder columns by category: todo → in progress → done.
	issues := []jira.Issue{
		{Key: "A-1", Status: "Done"},
		{Key: "A-2", Status: "In Progress"},
		{Key: "A-3", Status: "To Do"},
	}
	m := New()
	m.SetSize(120, 40)
	// Known statuses in reverse category order.
	m.SetKnownStatuses([]string{"Done", "In Progress", "To Do"})
	m.SetIssues(issues, "Board")

	if len(m.columns) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(m.columns))
	}
	// Should be sorted by category, not by knownStatuses order.
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
