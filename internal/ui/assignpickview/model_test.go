package assignpickview

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/seanhalberthal/jiru/internal/jira"
)

var testUsers = []jira.UserInfo{
	{AccountID: "user-1", DisplayName: "Alice Smith"},
	{AccountID: "user-2", DisplayName: "Bob Jones"},
	{AccountID: "user-3", DisplayName: "Charlie Brown"},
}

func TestNew_Initialises(t *testing.T) {
	m := New("PROJ-42", "Alice Smith")

	if m.issueKey != "PROJ-42" {
		t.Errorf("issueKey = %q, want %q", m.issueKey, "PROJ-42")
	}
	if m.currentAssignee != "Alice Smith" {
		t.Errorf("currentAssignee = %q, want %q", m.currentAssignee, "Alice Smith")
	}
	if !m.InputActive() {
		t.Error("InputActive() should always return true")
	}
}

func TestNew_Unassigned(t *testing.T) {
	m := New("PROJ-1", "")

	if m.currentAssignee != "" {
		t.Errorf("currentAssignee = %q, want empty", m.currentAssignee)
	}
}

func TestSelectedAssignee_FixedOption_AssignToMe(t *testing.T) {
	m := New("PROJ-1", "")
	m.SetSize(80, 24)

	// Cursor starts at 0 = "Assign to me".
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	sel := m.SelectedAssignee()
	if sel == nil {
		t.Fatal("SelectedAssignee() should not be nil after Enter")
	}
	if sel.AccountID != "default" {
		t.Errorf("AccountID = %q, want %q", sel.AccountID, "default")
	}
	if sel.DisplayName != "Assign to me" {
		t.Errorf("DisplayName = %q, want %q", sel.DisplayName, "Assign to me")
	}

	// Sentinel should clear after first read.
	if m.SelectedAssignee() != nil {
		t.Error("SelectedAssignee() second call should return nil")
	}
}

func TestSelectedAssignee_FixedOption_Unassign(t *testing.T) {
	m := New("PROJ-1", "Alice")
	m.SetSize(80, 24)

	// Move down to "Unassign" (index 1).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	sel := m.SelectedAssignee()
	if sel == nil {
		t.Fatal("SelectedAssignee() should not be nil")
	}
	if sel.AccountID != "none" {
		t.Errorf("AccountID = %q, want %q", sel.AccountID, "none")
	}
	if sel.DisplayName != "Unassign" {
		t.Errorf("DisplayName = %q, want %q", sel.DisplayName, "Unassign")
	}
}

func TestSelectedAssignee_UserResult(t *testing.T) {
	m := New("PROJ-1", "")
	m.SetSize(80, 24)
	m.SetUsers(testUsers)

	// Fixed options: 0=Assign to me, 1=Unassign. Users start at index 2.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown}) // 1
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown}) // 2 (Alice)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown}) // 3 (Bob)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	sel := m.SelectedAssignee()
	if sel == nil {
		t.Fatal("SelectedAssignee() should not be nil")
	}
	if sel.AccountID != "user-2" {
		t.Errorf("AccountID = %q, want %q", sel.AccountID, "user-2")
	}
	if sel.DisplayName != "Bob Jones" {
		t.Errorf("DisplayName = %q, want %q", sel.DisplayName, "Bob Jones")
	}
}

func TestDismissed_OnEsc(t *testing.T) {
	m := New("PROJ-1", "")
	m.SetSize(80, 24)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if !m.Dismissed() {
		t.Error("Dismissed() should return true after Esc")
	}

	// Sentinel should clear after first read.
	if m.Dismissed() {
		t.Error("Dismissed() second call should return false")
	}
}

func TestCursorNavigation_UpDown(t *testing.T) {
	m := New("PROJ-1", "")
	m.SetSize(80, 24)
	m.SetUsers(testUsers)

	// Total items = 2 (fixed) + 3 (users) = 5.
	if m.totalItems() != 5 {
		t.Fatalf("totalItems() = %d, want 5", m.totalItems())
	}

	// Cursor starts at 0. Move up — should stay at 0.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sel := m.SelectedAssignee()
	if sel == nil || sel.AccountID != "default" {
		t.Errorf("cursor should be clamped at top, got %v", sel)
	}
}

func TestCursorNavigation_CtrlPN(t *testing.T) {
	m := New("PROJ-1", "")
	m.SetSize(80, 24)
	m.SetUsers(testUsers)

	// Move down with ctrl+n.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sel := m.SelectedAssignee()
	if sel == nil || sel.AccountID != "none" {
		t.Errorf("after ctrl+n, expected 'Unassign', got %v", sel)
	}
}

func TestCursorClamping_Bottom(t *testing.T) {
	m := New("PROJ-1", "")
	m.SetSize(80, 24)
	m.SetUsers(testUsers)

	// Move down past the end.
	for range 10 {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	sel := m.SelectedAssignee()
	if sel == nil || sel.AccountID != "user-3" {
		t.Errorf("cursor should be clamped at bottom (Charlie Brown), got %v", sel)
	}
}

func TestSetUsers_ResetsCursor(t *testing.T) {
	m := New("PROJ-1", "")
	m.SetSize(80, 24)

	// Move cursor down.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.cursor != 1 {
		t.Fatalf("cursor = %d, want 1 after down", m.cursor)
	}

	// Setting users should reset cursor to 0.
	m.SetUsers(testUsers)
	if m.cursor != 0 {
		t.Errorf("cursor = %d after SetUsers, want 0", m.cursor)
	}
}

func TestSetSize_UpdatesDimensions(t *testing.T) {
	m := New("PROJ-1", "")
	m.SetSize(100, 50)

	if m.width != 100 {
		t.Errorf("width = %d, want 100", m.width)
	}
	if m.height != 50 {
		t.Errorf("height = %d, want 50", m.height)
	}
}

func TestNeedsUserSearch_ClearsAfterRead(t *testing.T) {
	m := New("PROJ-1", "")
	m.needsSearch = "alice"

	got := m.NeedsUserSearch()
	if got != "alice" {
		t.Errorf("NeedsUserSearch() = %q, want %q", got, "alice")
	}

	// Should be cleared after first read.
	if m.NeedsUserSearch() != "" {
		t.Error("NeedsUserSearch() second call should return empty")
	}
}

func TestEnterWithNoItems(t *testing.T) {
	m := New("PROJ-1", "")
	m.SetSize(80, 24)

	// totalItems() = 2 (fixed options) even with no users.
	// Pressing enter should select "Assign to me".
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	sel := m.SelectedAssignee()
	if sel == nil {
		t.Fatal("SelectedAssignee() should not be nil — fixed options always present")
	}
	if sel.AccountID != "default" {
		t.Errorf("AccountID = %q, want %q", sel.AccountID, "default")
	}
}

func TestView_ContainsIssueKey(t *testing.T) {
	m := New("PROJ-42", "")
	m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, "PROJ-42") {
		t.Error("View should contain the issue key")
	}
}

func TestView_ShowsCurrentAssignee(t *testing.T) {
	m := New("PROJ-1", "Alice Smith")
	m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, "Alice Smith") {
		t.Error("View should show the current assignee")
	}
}

func TestView_ShowsUnassignedWhenEmpty(t *testing.T) {
	m := New("PROJ-1", "")
	m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, "unassigned") {
		t.Error("View should show 'unassigned' when no current assignee")
	}
}

func TestView_ShowsFixedOptions(t *testing.T) {
	m := New("PROJ-1", "")
	m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, "Assign to me") {
		t.Error("View should show 'Assign to me' option")
	}
	if !strings.Contains(view, "Unassign") {
		t.Error("View should show 'Unassign' option")
	}
}

func TestView_ShowsUserResults(t *testing.T) {
	m := New("PROJ-1", "")
	m.SetSize(80, 24)
	m.SetUsers(testUsers)

	view := m.View()
	for _, u := range testUsers {
		if !strings.Contains(view, u.DisplayName) {
			t.Errorf("View should contain user %q", u.DisplayName)
		}
	}
}

func TestView_ContainsHelpText(t *testing.T) {
	m := New("PROJ-1", "")
	m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, "navigate") {
		t.Error("View should contain navigation help")
	}
	if !strings.Contains(view, "enter") {
		t.Error("View should contain enter help")
	}
	if !strings.Contains(view, "esc") {
		t.Error("View should contain esc help")
	}
}

func TestInputActive_AlwaysTrue(t *testing.T) {
	m := New("PROJ-1", "")
	if !m.InputActive() {
		t.Error("InputActive() should always return true")
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !m.InputActive() {
		t.Error("InputActive() should remain true even after dismiss")
	}
}
