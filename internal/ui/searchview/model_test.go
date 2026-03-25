package searchview

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/seanhalberthal/jiru/internal/jira"
	"github.com/seanhalberthal/jiru/internal/jql"
	"github.com/seanhalberthal/jiru/internal/ui/issuedelegate"
)

func TestSetResults_TransitionsToResultsState(t *testing.T) {
	m := New()
	m.Show()
	m = m.SetSize(80, 24)

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
	m = m.SetSize(80, 24)

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
	m = m.SetSize(80, 24)

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
	m = m.SetSize(80, 24)

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
	m = m.SetSize(80, 24)

	// Type something first.
	m.input.SetValue("some query")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.Dismissed() {
		t.Error("expected not dismissed when input has content")
	}
}

func TestEscDismissCompletions_ThenEscClosesSearch(t *testing.T) {
	m := New()
	m.Show()
	m = m.SetSize(80, 24)
	m = m.SetMetadata(&jira.JQLMetadata{
		Statuses: []string{"Done", "To Do", "In Progress"},
	})

	// Simulate typing "status = D" with completions showing.
	m.input.SetValue("status = D")
	m.input.SetCursor(10)
	ctx := jql.Parse(m.input.Value(), m.input.Position())
	m.completions = jql.Match(ctx, m.values)
	if len(m.completions) == 0 {
		t.Fatal("expected completions for 'D' prefix")
	}

	// First esc: dismiss completions.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if len(m.completions) != 0 {
		t.Error("expected completions cleared after first esc")
	}
	if m.Visible() == false {
		t.Error("expected search still visible after first esc")
	}

	// Second esc: should close search (not reshow completions).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.Visible() {
		t.Error("expected search hidden after second esc")
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
	m = m.SetMetadata(meta)

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
	m = m.SetMetadata(nil)
	if m.values != nil {
		t.Error("expected values to remain nil after SetMetadata(nil)")
	}
}

func TestSetUserResults_PopulatesUsers(t *testing.T) {
	m := New()
	m = m.SetUserResults([]string{"Alice", "Bob"})

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
	m = m.SetMetadata(&jira.JQLMetadata{
		Statuses: []string{"Done"},
	})
	m = m.SetUserResults([]string{"Alice"})

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
	m = m.SetSize(80, 24)
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
	m = m.SetSize(80, 24)
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
	m = m.SetSize(80, 24)
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
	m = m.SetSize(80, 24)
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

func TestAcceptCompletion_ClearsCompletions(t *testing.T) {
	m := New()
	m.Show()
	m = m.SetSize(80, 24)
	m = m.SetMetadata(&jira.JQLMetadata{
		Statuses: []string{"Done", "To Do", "In Progress"},
	})

	// Set up input and manually compute completions to simulate typing "status = D".
	m.input.SetValue("status = D")
	m.input.SetCursor(10)
	ctx := jql.Parse(m.input.Value(), m.input.Position())
	m.completions = jql.Match(ctx, m.values)
	if len(m.completions) == 0 {
		t.Fatal("expected completions for 'D' prefix")
	}

	// Accept the first completion (e.g., "Done").
	m.compIndex = 0
	m.acceptCompletion()
	if len(m.completions) != 0 {
		t.Error("expected completions cleared after acceptance")
	}
}

func TestAcceptCompletion_CompletionsReappearAfterSpace(t *testing.T) {
	m := New()
	m.Show()
	m = m.SetSize(80, 24)
	m = m.SetMetadata(&jira.JQLMetadata{
		Statuses: []string{"Done", "To Do", "In Progress"},
	})

	// Set up state with completions and accept one.
	m.input.SetValue("status = D")
	m.input.SetCursor(10)
	ctx := jql.Parse(m.input.Value(), m.input.Position())
	m.completions = jql.Match(ctx, m.values)
	m.compIndex = 0
	m.acceptCompletion()

	// Type space — completions should be recalculated.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	// After space, completions should be recalculated (keyword context: AND/OR/NOT/ORDER BY).
	if len(m.completions) == 0 {
		t.Error("expected completions to reappear after space")
	}
}

func TestAcceptCompletion_BackspaceRecalculatesCompletions(t *testing.T) {
	m := New()
	m.Show()
	m = m.SetSize(80, 24)
	m = m.SetMetadata(&jira.JQLMetadata{
		Statuses: []string{"Done", "To Do", "In Progress"},
	})

	// Set up state with completions and accept one.
	m.input.SetValue("status = D")
	m.input.SetCursor(10)
	ctx := jql.Parse(m.input.Value(), m.input.Position())
	m.completions = jql.Match(ctx, m.values)
	m.compIndex = 0
	m.acceptCompletion()

	// Backspace — completions should recalculate.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	// After backspace, "status = Don" should match "Done" completion.
	if len(m.completions) == 0 {
		t.Error("expected completions to reappear after backspace editing")
	}
}

func TestArrowsCycleThroughCompletions_TabAccepts(t *testing.T) {
	m := New()
	m.Show()
	m = m.SetSize(80, 24)
	m = m.SetMetadata(&jira.JQLMetadata{
		Statuses: []string{"Done", "Draft", "In Progress"},
	})

	// Set up input with completions — prefix "D" matches "Done" and "Draft".
	m.input.SetValue("status = D")
	m.input.SetCursor(10)
	ctx := jql.Parse(m.input.Value(), m.input.Position())
	m.completions = jql.Match(ctx, m.values)
	if len(m.completions) < 2 {
		t.Fatalf("expected at least 2 completions for 'D' prefix, got %d", len(m.completions))
	}

	// Down arrow: should select index 0.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.compIndex != 0 {
		t.Errorf("expected compIndex 0 after down, got %d", m.compIndex)
	}
	if len(m.completions) == 0 {
		t.Error("expected completions still showing after down")
	}

	// Down arrow again: should move to index 1.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.compIndex != 1 {
		t.Errorf("expected compIndex 1 after second down, got %d", m.compIndex)
	}

	// Tab: should accept the selected completion (index 1 = "Draft").
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if len(m.completions) != 0 {
		t.Error("expected completions cleared after tab acceptance")
	}
}

func TestNeedsUserSearch_PendingBlocksNew(t *testing.T) {
	m := New()
	m.Show()
	m = m.SetSize(80, 24)
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

func TestModel_Visible_ShowHide(t *testing.T) {
	m := New()
	if m.Visible() {
		t.Error("expected model to start hidden")
	}

	m.Show()
	if !m.Visible() {
		t.Error("expected Visible() true after Show()")
	}

	m.Hide()
	if m.Visible() {
		t.Error("expected Visible() false after Hide()")
	}
}

func TestModel_InputActive(t *testing.T) {
	m := New()
	m.Show()
	m = m.SetSize(80, 24)

	// In stateInput, InputActive should be true.
	if !m.InputActive() {
		t.Error("expected InputActive() true in stateInput")
	}

	// Transition to stateResults.
	issues := []jira.Issue{
		{Key: "TEST-1", Summary: "Test", Status: "To Do", IssueType: "Story"},
	}
	m.SetResults(issues, "query")

	// In stateResults with no filtering, InputActive should be false.
	if m.InputActive() {
		t.Error("expected InputActive() false in stateResults without filtering")
	}
}

func TestModel_BackToInput(t *testing.T) {
	m := New()
	m.Show()
	m = m.SetSize(80, 24)

	issues := []jira.Issue{
		{Key: "TEST-1", Summary: "Test", Status: "To Do", IssueType: "Story"},
	}
	m.SetResults(issues, "query")

	// Verify we're in results state.
	if m.InputActive() {
		t.Error("expected to be in results state")
	}

	// Call BackToInput.
	m.BackToInput()

	// Should be back in input state.
	if !m.InputActive() {
		t.Error("expected InputActive() true after BackToInput()")
	}

	view := m.View()
	if !strings.Contains(view, "Search Issues") {
		t.Error("expected input view after BackToInput()")
	}
}

func TestModel_SubmittedQuery(t *testing.T) {
	m := New()
	m.Show()
	m = m.SetSize(80, 24)

	// Type a query.
	m.input.SetValue("status = Done")

	// Press enter to submit.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	q := m.SubmittedQuery()
	if q != "status = Done" {
		t.Errorf("SubmittedQuery() = %q, want %q", q, "status = Done")
	}

	// Sentinel should reset.
	q = m.SubmittedQuery()
	if q != "" {
		t.Errorf("expected empty after sentinel reset, got %q", q)
	}
}

func TestModel_Dismissed_FromInput(t *testing.T) {
	m := New()
	m.Show()
	m = m.SetSize(80, 24)

	// Input is empty, press Esc.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if !m.Dismissed() {
		t.Error("expected Dismissed() true after Esc on empty input")
	}

	// Verify sentinel resets.
	if m.Dismissed() {
		t.Error("expected Dismissed() false after sentinel consumed")
	}
}

func TestModel_SetMetadata(t *testing.T) {
	m := New()
	m.Show()
	m = m.SetSize(80, 24)

	meta := &jira.JQLMetadata{
		Statuses:   []string{"Open", "Closed"},
		IssueTypes: []string{"Bug"},
		Priorities: []string{"High", "Low"},
	}
	m = m.SetMetadata(meta)

	// Verify metadata is stored by triggering value completions.
	// Type "status = " and check completions include our statuses.
	m.input.SetValue("status = O")
	m.input.SetCursor(10)
	ctx := jql.Parse(m.input.Value(), m.input.Position())
	completions := jql.Match(ctx, m.values)

	found := false
	for _, c := range completions {
		if c.Label == "Open" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'Open' in completions after SetMetadata")
	}
}

func TestModel_SetResults_AndSelect(t *testing.T) {
	m := New()
	m.Show()
	m = m.SetSize(80, 24)

	issues := []jira.Issue{
		{Key: "PROJ-1", Summary: "First issue", Status: "To Do", IssueType: "Story"},
		{Key: "PROJ-2", Summary: "Second issue", Status: "Done", IssueType: "Bug"},
	}
	m.SetResults(issues, "project = PROJ")

	// Press enter to select the first result.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	iss := m.SelectedIssue()
	if iss == nil {
		t.Fatal("expected a selected issue")
	}
	if iss.Key != "PROJ-1" {
		t.Errorf("SelectedIssue().Key = %q, want %q", iss.Key, "PROJ-1")
	}

	// Sentinel should reset.
	if m.SelectedIssue() != nil {
		t.Error("expected nil after sentinel consumed")
	}
}

func TestModel_View_InputState(t *testing.T) {
	m := New()
	m.Show()
	m = m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, "Search Issues") {
		t.Error("expected View() in input state to contain 'Search Issues'")
	}
	if !strings.Contains(view, "Enter to search") {
		t.Error("expected View() in input state to contain hint text")
	}
}

func TestModel_View_ResultsState(t *testing.T) {
	m := New()
	m.Show()
	m = m.SetSize(80, 24)

	issues := []jira.Issue{
		{Key: "VIEW-1", Summary: "View test issue", Status: "Open", IssueType: "Task"},
		{Key: "VIEW-2", Summary: "Another issue", Status: "Done", IssueType: "Bug"},
	}
	m.SetResults(issues, "project = VIEW")

	view := m.View()
	if !strings.Contains(view, "VIEW-1") {
		t.Error("expected View() in results state to contain issue key 'VIEW-1'")
	}
	if !strings.Contains(view, "VIEW-2") {
		t.Error("expected View() in results state to contain issue key 'VIEW-2'")
	}
}

func TestModel_View_Hidden(t *testing.T) {
	m := New()
	// Model starts hidden.
	view := m.View()
	if view != "" {
		t.Errorf("expected empty view when hidden, got %q", view)
	}
}

func TestModel_Update_WhenHidden(t *testing.T) {
	m := New()
	// Model is hidden; Update should be a no-op.
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("expected nil cmd when model is hidden")
	}
}

func TestModel_BackToInput_NoopFromInput(t *testing.T) {
	m := New()
	m.Show()
	m = m.SetSize(80, 24)

	// BackToInput when already in input state should be a no-op.
	m.BackToInput()
	if !m.InputActive() {
		t.Error("expected still in input state after BackToInput from input")
	}
}

func TestAppendResults_PreservesCursorPosition(t *testing.T) {
	m := New()
	m.Show()
	m = m.SetSize(80, 24)

	m.SetResults([]jira.Issue{
		{Key: "A-1", Summary: "First", Status: "To Do", IssueType: "Story"},
		{Key: "A-2", Summary: "Second", Status: "To Do", IssueType: "Story"},
		{Key: "A-3", Summary: "Third", Status: "To Do", IssueType: "Story"},
	}, "test query")

	// Move cursor to the second item.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	idx := m.results.Index()
	if idx != 1 {
		t.Fatalf("expected cursor at index 1, got %d", idx)
	}

	// Append more results — cursor should stay at index 1.
	m.AppendResults([]jira.Issue{
		{Key: "A-4", Summary: "Fourth", Status: "To Do", IssueType: "Story"},
	})

	if m.results.Index() != 1 {
		t.Errorf("expected cursor still at index 1 after append, got %d", m.results.Index())
	}
	if len(m.results.Items()) != 4 {
		t.Errorf("expected 4 results after append, got %d", len(m.results.Items()))
	}
}

func TestAppendResults_MergesWithExisting(t *testing.T) {
	m := New()
	m.Show()
	m = m.SetSize(80, 24)

	m.SetResults([]jira.Issue{
		{Key: "A-1", Summary: "First"},
	}, "test query")

	m.AppendResults([]jira.Issue{
		{Key: "A-2", Summary: "Second"},
		{Key: "A-3", Summary: "Third"},
	})

	items := m.results.Items()
	if len(items) != 3 {
		t.Errorf("expected 3 results after append, got %d", len(items))
	}
}

func TestTabAcceptsCompletion(t *testing.T) {
	m := New()
	m.Show()
	m = m.SetSize(80, 24)

	for _, ch := range "ass" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}

	if m.input.Value() != "ass" {
		t.Fatalf("after typing: got %q, want %q", m.input.Value(), "ass")
	}
	if len(m.completions) == 0 {
		t.Fatal("expected completions after typing 'ass'")
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})

	got := m.input.Value()
	if got != "assignee" {
		t.Errorf("after Tab: got %q, want %q", got, "assignee")
	}
	if len(m.completions) != 0 {
		t.Errorf("expected completions cleared after accept, got %d", len(m.completions))
	}
}

func TestTabAcceptsThroughAppPattern(t *testing.T) {
	var search Model
	search = New()
	search.Show()
	search = search.SetSize(80, 24)

	for _, ch := range "sta" {
		var cmd tea.Cmd
		search, cmd = search.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		_ = cmd
	}

	search, _ = search.Update(tea.KeyMsg{Type: tea.KeyTab})

	got := search.input.Value()
	if got != "status" {
		t.Errorf("after Tab via app pattern: got %q, want %q", got, "status")
	}
}

func TestSetFilterName_ShowsInTitle(t *testing.T) {
	m := New()
	m.Show()
	m = m.SetSize(120, 24)

	m.SetFilterName("My Bugs")
	issues := []jira.Issue{
		{Key: "PROJ-1", Summary: "Bug", Status: "Open", IssueType: "Bug"},
		{Key: "PROJ-2", Summary: "Bug 2", Status: "Open", IssueType: "Bug"},
	}
	m.SetResults(issues, "type = Bug AND assignee = currentUser()")

	view := m.View()
	if !strings.Contains(view, "Filter: My Bugs") {
		t.Errorf("expected 'Filter: My Bugs' in title, got:\n%s", view)
	}
	if strings.Contains(view, "Results for:") {
		t.Error("expected 'Results for:' prefix to be replaced by 'Filter:'")
	}
}

func TestSetFilterName_ClearedOnManualSearch(t *testing.T) {
	m := New()
	m.Show()
	m = m.SetSize(120, 24)

	// Set a filter context.
	m.SetFilterName("My Filter")

	// User types a new query and presses enter — should clear the filter name.
	m.input.SetValue("project = PROJ")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	q := m.SubmittedQuery()
	if q != "project = PROJ" {
		t.Errorf("expected submitted query, got %q", q)
	}

	// Now set results — title should show "Results for:" not "Filter:".
	m.SetResults(nil, "project = PROJ")
	view := m.View()
	if strings.Contains(view, "Filter:") {
		t.Error("expected filter name cleared after manual search")
	}
	if !strings.Contains(view, "Results for:") {
		t.Error("expected 'Results for:' prefix after manual search")
	}
}

func TestUpdateIssueStatus_MatchingKey(t *testing.T) {
	m := New()
	m.Show()
	m = m.SetSize(80, 24)

	issues := []jira.Issue{
		{Key: "PROJ-1", Summary: "First", Status: "To Do", IssueType: "Story"},
		{Key: "PROJ-2", Summary: "Second", Status: "In Progress", IssueType: "Bug"},
	}
	m.SetResults(issues, "test query")

	m.UpdateIssueStatus("PROJ-1", "Done")

	// The list item for PROJ-1 should now have status "Done".
	for _, item := range m.results.Items() {
		if it, ok := item.(issuedelegate.Item); ok && it.Key() == "PROJ-1" {
			if it.Issue.Status != "Done" {
				t.Errorf("expected PROJ-1 status 'Done', got %q", it.Issue.Status)
			}
		}
	}
}

func TestUpdateIssueStatus_NonMatchingKey(t *testing.T) {
	m := New()
	m.Show()
	m = m.SetSize(80, 24)

	issues := []jira.Issue{
		{Key: "PROJ-1", Summary: "First", Status: "To Do", IssueType: "Story"},
	}
	m.SetResults(issues, "test query")

	m.UpdateIssueStatus("NOPE-999", "Done")

	// PROJ-1 should remain unchanged.
	for _, item := range m.results.Items() {
		if it, ok := item.(issuedelegate.Item); ok && it.Key() == "PROJ-1" {
			if it.Issue.Status != "To Do" {
				t.Errorf("expected PROJ-1 status to remain 'To Do', got %q", it.Issue.Status)
			}
		}
	}
}

func TestUpdateIssueStatus_PreservesCursorPosition(t *testing.T) {
	m := New()
	m.Show()
	m = m.SetSize(80, 24)

	issues := []jira.Issue{
		{Key: "PROJ-1", Summary: "First", Status: "To Do", IssueType: "Story"},
		{Key: "PROJ-2", Summary: "Second", Status: "To Do", IssueType: "Bug"},
		{Key: "PROJ-3", Summary: "Third", Status: "To Do", IssueType: "Task"},
	}
	m.SetResults(issues, "test query")

	// Move cursor to second item.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	idxBefore := m.results.Index()

	m.UpdateIssueStatus("PROJ-1", "Done")

	if m.results.Index() != idxBefore {
		t.Errorf("cursor moved from %d to %d after UpdateIssueStatus", idxBefore, m.results.Index())
	}
}

func TestSaveFilter_TriggeredFromResults(t *testing.T) {
	m := New()
	m.Show()
	m = m.SetSize(80, 24)

	issues := []jira.Issue{
		{Key: "PROJ-1", Summary: "First", Status: "To Do", IssueType: "Story"},
	}
	m.SetResults(issues, "status = Open")

	// Press 's' to trigger save.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})

	q := m.SaveFilter()
	if q != "status = Open" {
		t.Errorf("SaveFilter() = %q, want %q", q, "status = Open")
	}

	// Sentinel should reset.
	q = m.SaveFilter()
	if q != "" {
		t.Errorf("SaveFilter() should be empty on second call, got %q", q)
	}
}

func TestSaveFilter_NotTriggeredFromInput(t *testing.T) {
	m := New()
	m.Show()
	m = m.SetSize(80, 24)

	// In input state, 's' is just a character — should not trigger save.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})

	q := m.SaveFilter()
	if q != "" {
		t.Errorf("SaveFilter() should be empty in input state, got %q", q)
	}
}

func TestSaveFilter_EmptyWhenNotTriggered(t *testing.T) {
	m := New()
	m.Show()
	m = m.SetSize(80, 24)

	q := m.SaveFilter()
	if q != "" {
		t.Errorf("SaveFilter() should be empty initially, got %q", q)
	}
}

func TestShowingResults_InInputState(t *testing.T) {
	m := New()
	m.Show()
	m = m.SetSize(80, 24)

	if m.ShowingResults() {
		t.Error("ShowingResults() should be false in input state")
	}
}

func TestShowingResults_InResultsState(t *testing.T) {
	m := New()
	m.Show()
	m = m.SetSize(80, 24)

	issues := []jira.Issue{
		{Key: "PROJ-1", Summary: "First", Status: "To Do", IssueType: "Story"},
	}
	m.SetResults(issues, "test query")

	if !m.ShowingResults() {
		t.Error("ShowingResults() should be true after SetResults")
	}
}

func TestShowingResults_FalseAfterBackToInput(t *testing.T) {
	m := New()
	m.Show()
	m = m.SetSize(80, 24)

	m.SetResults([]jira.Issue{
		{Key: "PROJ-1", Summary: "First", Status: "To Do", IssueType: "Story"},
	}, "test query")
	m.BackToInput()

	if m.ShowingResults() {
		t.Error("ShowingResults() should be false after BackToInput")
	}
}

func TestResultsFiltered_FalseByDefault(t *testing.T) {
	m := New()
	m.Show()
	m = m.SetSize(80, 24)

	if m.ResultsFiltered() {
		t.Error("ResultsFiltered() should be false in input state")
	}
}

func TestResultsFiltered_FalseInResultsWithNoFilter(t *testing.T) {
	m := New()
	m.Show()
	m = m.SetSize(80, 24)

	m.SetResults([]jira.Issue{
		{Key: "PROJ-1", Summary: "First", Status: "To Do", IssueType: "Story"},
	}, "test query")

	if m.ResultsFiltered() {
		t.Error("ResultsFiltered() should be false when no filter is applied")
	}
}

func TestResetResultsFilter_ClearsFilter(t *testing.T) {
	m := New()
	m.Show()
	m = m.SetSize(80, 24)

	m.SetResults([]jira.Issue{
		{Key: "PROJ-1", Summary: "First", Status: "To Do", IssueType: "Story"},
	}, "test query")

	// ResetResultsFilter on an unfiltered list should be a no-op (no panic).
	m.ResetResultsFilter()

	if m.ResultsFiltered() {
		t.Error("ResultsFiltered() should be false after ResetResultsFilter")
	}
}

func TestSetFilterName_Empty_UsesDefault(t *testing.T) {
	m := New()
	m.Show()
	m = m.SetSize(120, 24)

	m.SetFilterName("") // Explicitly empty.
	m.SetResults([]jira.Issue{
		{Key: "PROJ-1", Summary: "Test", Status: "Open", IssueType: "Task"},
	}, "status = Open")

	view := m.View()
	if !strings.Contains(view, "Results for:") {
		t.Error("expected default 'Results for:' when filter name is empty")
	}
}

func TestAppendResults_PreservesFilterName(t *testing.T) {
	m := New()
	m.Show()
	m = m.SetSize(120, 24)

	m.SetFilterName("My Bugs")
	m.SetResults([]jira.Issue{
		{Key: "A-1", Summary: "First", Status: "Open", IssueType: "Bug"},
	}, "assignee = me AND type = Bug")

	// Title should show filter name, not raw JQL.
	if !strings.Contains(m.results.Title, "Filter: My Bugs") {
		t.Errorf("expected 'Filter: My Bugs' in title after SetResults, got %q", m.results.Title)
	}

	// Append more results — filter name should be preserved.
	m.AppendResults([]jira.Issue{
		{Key: "A-2", Summary: "Second", Status: "Open", IssueType: "Bug"},
	})

	if !strings.Contains(m.results.Title, "Filter: My Bugs") {
		t.Errorf("expected 'Filter: My Bugs' preserved after AppendResults, got %q", m.results.Title)
	}
	if strings.Contains(m.results.Title, "assignee") {
		t.Error("AppendResults should not show raw JQL when filter name is set")
	}
}
