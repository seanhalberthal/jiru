package filterview

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/seanhalberthal/jiru/internal/jira"
	"github.com/seanhalberthal/jiru/internal/jql"
)

func key(k string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
}

func keyType(t tea.KeyType) tea.KeyMsg {
	return tea.KeyMsg{Type: t}
}

func sampleFilters() []jira.SavedFilter {
	return []jira.SavedFilter{
		{ID: "aaa", Name: "My Bugs", JQL: "type = Bug AND assignee = me"},
		{ID: "bbb", Name: "Open Tasks", JQL: "status = Open", Favourite: true},
	}
}

func TestNew_StartsInListState(t *testing.T) {
	m := New()
	if m.state != stateList {
		t.Errorf("expected stateList, got %d", m.state)
	}
}

func TestDismissed_OnEsc(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetFilters(sampleFilters())

	m, _ = m.Update(keyType(tea.KeyEscape))
	if !m.Dismissed() {
		t.Error("expected Dismissed() after esc")
	}
	// Should be one-shot.
	if m.Dismissed() {
		t.Error("Dismissed() should be false on second call")
	}
}

func TestDismissed_OnQ(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetFilters(sampleFilters())

	m, _ = m.Update(key("q"))
	if !m.Dismissed() {
		t.Error("expected Dismissed() after q")
	}
}

func TestApplied_OnEnter(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetFilters(sampleFilters())

	m, _ = m.Update(keyType(tea.KeyEnter))
	f := m.Applied()
	if f == nil {
		t.Fatal("expected Applied() to return a filter")
	}
	if f.ID != "aaa" {
		t.Errorf("expected filter ID 'aaa', got %q", f.ID)
	}
	// One-shot.
	if m.Applied() != nil {
		t.Error("Applied() should be nil on second call")
	}
}

func TestApplied_SecondItem(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetFilters(sampleFilters())

	// Move down to second item.
	m, _ = m.Update(key("j"))
	m, _ = m.Update(keyType(tea.KeyEnter))

	f := m.Applied()
	if f == nil {
		t.Fatal("expected Applied() to return a filter")
	}
	if f.ID != "bbb" {
		t.Errorf("expected filter ID 'bbb', got %q", f.ID)
	}
}

func TestNewFilter_Flow(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetFilters(nil)

	// Press 'n' to start new filter.
	m, _ = m.Update(key("n"))
	if m.state != stateEditName {
		t.Fatalf("expected stateEditName, got %d", m.state)
	}

	// Type a name.
	for _, c := range "Test Filter" {
		m, _ = m.Update(key(string(c)))
	}
	// Press enter to go to JQL input.
	m, _ = m.Update(keyType(tea.KeyEnter))
	if m.state != stateEditQuery {
		t.Fatalf("expected stateEditQuery, got %d", m.state)
	}

	// Type a query.
	for _, c := range "status = Open" {
		m, _ = m.Update(key(string(c)))
	}
	// Press enter to save.
	m, _ = m.Update(keyType(tea.KeyEnter))

	id, name, jql, ok := m.SaveRequested()
	if !ok {
		t.Fatal("expected SaveRequested() to return true")
	}
	if id != "" {
		t.Errorf("expected empty ID for new filter, got %q", id)
	}
	if name != "Test Filter" {
		t.Errorf("expected name 'Test Filter', got %q", name)
	}
	if jql != "status = Open" {
		t.Errorf("expected JQL 'status = Open', got %q", jql)
	}

	// One-shot.
	if _, _, _, ok := m.SaveRequested(); ok {
		t.Error("SaveRequested() should be false on second call")
	}
}

func TestEditFilter_Flow(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetFilters(sampleFilters())

	// Press 'e' to edit selected filter.
	m, _ = m.Update(key("e"))
	if m.state != stateEditName {
		t.Fatalf("expected stateEditName, got %d", m.state)
	}
	if m.editID != "aaa" {
		t.Errorf("expected editID 'aaa', got %q", m.editID)
	}

	// Press enter to go to JQL (name is pre-filled).
	m, _ = m.Update(keyType(tea.KeyEnter))
	if m.state != stateEditQuery {
		t.Fatalf("expected stateEditQuery, got %d", m.state)
	}

	// Press enter to save (JQL is pre-filled).
	m, _ = m.Update(keyType(tea.KeyEnter))

	id, name, jql, ok := m.SaveRequested()
	if !ok {
		t.Fatal("expected SaveRequested() to return true")
	}
	if id != "aaa" {
		t.Errorf("expected ID 'aaa', got %q", id)
	}
	if name != "My Bugs" {
		t.Errorf("expected pre-filled name, got %q", name)
	}
	if jql != "type = Bug AND assignee = me" {
		t.Errorf("expected pre-filled JQL, got %q", jql)
	}
}

func TestDeleteFilter_Confirm(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetFilters(sampleFilters())

	// Press 'd' to start delete.
	m, _ = m.Update(key("d"))
	if m.state != stateConfirmDelete {
		t.Fatalf("expected stateConfirmDelete, got %d", m.state)
	}

	// Confirm with 'y'.
	m, _ = m.Update(key("y"))
	id := m.DeleteRequested()
	if id != "aaa" {
		t.Errorf("expected delete ID 'aaa', got %q", id)
	}
	// One-shot.
	if m.DeleteRequested() != "" {
		t.Error("DeleteRequested() should be empty on second call")
	}
}

func TestDeleteFilter_Cancel(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetFilters(sampleFilters())

	m, _ = m.Update(key("d"))
	m, _ = m.Update(key("n"))
	if m.state != stateList {
		t.Errorf("expected stateList after cancel, got %d", m.state)
	}
	if id := m.DeleteRequested(); id != "" {
		t.Errorf("expected no delete after cancel, got %q", id)
	}
}

func TestFavourite(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetFilters(sampleFilters())

	m, _ = m.Update(key("f"))
	id := m.FavouriteRequested()
	if id != "aaa" {
		t.Errorf("expected favourite ID 'aaa', got %q", id)
	}
	// One-shot.
	if m.FavouriteRequested() != "" {
		t.Error("FavouriteRequested() should be empty on second call")
	}
}

func TestStartAdd_PreFillsJQL(t *testing.T) {
	m := New()
	m.SetSize(80, 24)

	m.StartAdd("project = TEST")
	if m.state != stateEditName {
		t.Fatalf("expected stateEditName, got %d", m.state)
	}
	if m.jqlInput.Value() != "project = TEST" {
		t.Errorf("expected pre-filled JQL, got %q", m.jqlInput.Value())
	}
	if m.nameInput.Value() != "" {
		t.Errorf("expected empty name input, got %q", m.nameInput.Value())
	}
}

func TestInputActive(t *testing.T) {
	m := New()
	m.SetSize(80, 24)

	if m.InputActive() {
		t.Error("InputActive should be false in list state")
	}

	m.SetFilters(nil)
	m, _ = m.Update(key("n"))
	if !m.InputActive() {
		t.Error("InputActive should be true in editName state")
	}
}

func TestCursorNavigation(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetFilters(sampleFilters())

	if m.cursor != 0 {
		t.Errorf("expected cursor at 0, got %d", m.cursor)
	}

	m, _ = m.Update(key("j"))
	if m.cursor != 1 {
		t.Errorf("expected cursor at 1, got %d", m.cursor)
	}

	// Can't go past end.
	m, _ = m.Update(key("j"))
	if m.cursor != 1 {
		t.Errorf("expected cursor still at 1, got %d", m.cursor)
	}

	m, _ = m.Update(key("k"))
	if m.cursor != 0 {
		t.Errorf("expected cursor at 0, got %d", m.cursor)
	}

	// Can't go before start.
	m, _ = m.Update(key("k"))
	if m.cursor != 0 {
		t.Errorf("expected cursor still at 0, got %d", m.cursor)
	}
}

func TestSetFilters_ClampsCursor(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetFilters(sampleFilters())

	m, _ = m.Update(key("j")) // cursor = 1

	// Set a shorter list.
	m.SetFilters([]jira.SavedFilter{{ID: "x", Name: "Only"}})
	if m.cursor != 0 {
		t.Errorf("expected cursor clamped to 0, got %d", m.cursor)
	}
}

func TestEmptyListNoApply(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetFilters(nil)

	m, _ = m.Update(keyType(tea.KeyEnter))
	if m.Applied() != nil {
		t.Error("Applied() should be nil on empty list")
	}
}

func TestView_EmptyList(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetFilters(nil)

	view := m.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestView_WithFilters(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetFilters(sampleFilters())

	view := m.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestEscFromEditNameWithPrefilledJQL_Dismisses(t *testing.T) {
	m := New()
	m.SetSize(80, 24)

	// Simulate save from search (StartAdd pre-fills JQL).
	m.StartAdd("project = TEST")
	m, _ = m.Update(keyType(tea.KeyEscape))

	if !m.Dismissed() {
		t.Error("expected Dismissed() when escaping from name entry with pre-filled JQL")
	}
}

func TestEscFromEditName_ReturnsToList(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetFilters(sampleFilters())

	// Start new filter (no pre-filled JQL).
	m, _ = m.Update(key("n"))
	m, _ = m.Update(keyType(tea.KeyEscape))

	if m.state != stateList {
		t.Errorf("expected stateList, got %d", m.state)
	}
	if m.Dismissed() {
		t.Error("should not dismiss — just return to list")
	}
}

// queryModel returns a Model in stateEditQuery with the given JQL value,
// optional ValueProvider, and completions cleared.
func queryModel(jqlValue string, vp *jql.ValueProvider) Model {
	m := New()
	m.SetSize(80, 24)
	if vp != nil {
		m.SetValues(vp)
	}
	m.nameInput.SetValue("test")
	m.jqlInput.SetValue(jqlValue)
	m.jqlInput.SetCursor(len(jqlValue))
	m.jqlInput.Focus()
	m.state = stateEditQuery
	m.completions = nil
	m.compIndex = -1
	return m
}

func TestEscFromEditQuery_ReturnsToEditName(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetFilters(nil)

	m, _ = m.Update(key("n"))
	for _, c := range "test" {
		m, _ = m.Update(key(string(c)))
	}
	m, _ = m.Update(keyType(tea.KeyEnter)) // Go to JQL input.

	// First esc dismisses completions (populated on entry).
	if len(m.completions) > 0 {
		m, _ = m.Update(keyType(tea.KeyEscape))
	}
	// Second esc returns to name input.
	m, _ = m.Update(keyType(tea.KeyEscape))

	if m.state != stateEditName {
		t.Errorf("expected stateEditName, got %d", m.state)
	}
}

// --- JQL completion integration tests ---

func TestEditQuery_CompletionsPopulateOnEntry(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetValues(&jql.ValueProvider{
		Statuses: []string{"Done", "To Do", "In Progress"},
	})
	m.SetFilters(nil)

	// n → type name → enter (transitions to query step).
	m, _ = m.Update(key("n"))
	for _, c := range "test" {
		m, _ = m.Update(key(string(c)))
	}
	m, _ = m.Update(keyType(tea.KeyEnter))

	if m.state != stateEditQuery {
		t.Fatalf("expected stateEditQuery, got %d", m.state)
	}
	// Empty JQL input should show field completions.
	if len(m.completions) == 0 {
		t.Error("expected completions populated on query entry")
	}
}

func TestEditQuery_TypingRecalculatesCompletions(t *testing.T) {
	m := queryModel("", nil)

	for _, c := range "sta" {
		m, _ = m.Update(key(string(c)))
	}

	found := false
	for _, item := range m.completions {
		if item.Label == "status" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'status' in completions after typing 'sta'")
	}
}

func TestEditQuery_TabAcceptsCompletion(t *testing.T) {
	m := queryModel("", nil)

	for _, c := range "ass" {
		m, _ = m.Update(key(string(c)))
	}
	if len(m.completions) == 0 {
		t.Fatal("expected completions after typing 'ass'")
	}

	m, _ = m.Update(keyType(tea.KeyTab))

	got := m.jqlInput.Value()
	if got != "assignee" {
		t.Errorf("after tab: got %q, want %q", got, "assignee")
	}
	if len(m.completions) != 0 {
		t.Errorf("expected completions cleared after tab, got %d", len(m.completions))
	}
}

func TestEditQuery_ArrowsCycleThroughCompletions(t *testing.T) {
	vp := &jql.ValueProvider{
		Statuses: []string{"Done", "Draft"},
	}
	m := queryModel("status = D", vp)
	m.recalcCompletions()

	if len(m.completions) < 2 {
		t.Fatalf("expected at least 2 completions for 'D', got %d", len(m.completions))
	}

	// Down: select index 0.
	m, _ = m.Update(keyType(tea.KeyDown))
	if m.compIndex != 0 {
		t.Errorf("expected compIndex 0 after down, got %d", m.compIndex)
	}

	// Down again: index 1.
	m, _ = m.Update(keyType(tea.KeyDown))
	if m.compIndex != 1 {
		t.Errorf("expected compIndex 1 after second down, got %d", m.compIndex)
	}

	// Up: back to 0.
	m, _ = m.Update(keyType(tea.KeyUp))
	if m.compIndex != 0 {
		t.Errorf("expected compIndex 0 after up, got %d", m.compIndex)
	}

	// Tab: accepts selected.
	m, _ = m.Update(keyType(tea.KeyTab))
	if len(m.completions) != 0 {
		t.Error("expected completions cleared after tab")
	}
}

func TestEditQuery_EscDismissesCompletionsThenReturnsToName(t *testing.T) {
	vp := &jql.ValueProvider{
		Statuses: []string{"Done", "To Do"},
	}
	m := queryModel("status = D", vp)
	m.recalcCompletions()

	if len(m.completions) == 0 {
		t.Fatal("expected completions")
	}

	// First esc: dismiss completions.
	m, _ = m.Update(keyType(tea.KeyEscape))
	if len(m.completions) != 0 {
		t.Error("expected completions cleared after first esc")
	}
	if m.state != stateEditQuery {
		t.Error("expected still in stateEditQuery after dismissing completions")
	}

	// Second esc: return to name.
	m, _ = m.Update(keyType(tea.KeyEscape))
	if m.state != stateEditName {
		t.Errorf("expected stateEditName, got %d", m.state)
	}
}

func TestEditQuery_EnterAcceptsSelectedCompletion(t *testing.T) {
	m := queryModel("", nil)

	for _, c := range "ass" {
		m, _ = m.Update(key(string(c)))
	}
	// Select first completion.
	m, _ = m.Update(keyType(tea.KeyDown))
	if m.compIndex != 0 {
		t.Fatalf("expected compIndex 0, got %d", m.compIndex)
	}

	// Enter accepts the completion (doesn't save).
	m, _ = m.Update(keyType(tea.KeyEnter))
	if m.jqlInput.Value() != "assignee" {
		t.Errorf("expected 'assignee', got %q", m.jqlInput.Value())
	}
	if m.state != stateEditQuery {
		t.Error("expected still in stateEditQuery after accepting completion")
	}
}

func TestEditQuery_EnterSavesWhenNoCompletionSelected(t *testing.T) {
	m := queryModel("status = Done", nil)
	// No completions selected (compIndex = -1).

	m, _ = m.Update(keyType(tea.KeyEnter))

	_, name, query, ok := m.SaveRequested()
	if !ok {
		t.Fatal("expected SaveRequested()")
	}
	if name != "test" {
		t.Errorf("expected name 'test', got %q", name)
	}
	if query != "status = Done" {
		t.Errorf("expected query 'status = Done', got %q", query)
	}
}

func TestEditQuery_DynamicValuesFromProvider(t *testing.T) {
	vp := &jql.ValueProvider{
		Statuses: []string{"To Do", "In Progress", "Done"},
	}
	m := queryModel("status = Do", vp)
	m.recalcCompletions()

	found := false
	for _, item := range m.completions {
		if item.Label == "Done" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Done' in completions for status value prefix 'Do'")
	}
}

func TestEditQuery_SetValuesPopulatesProvider(t *testing.T) {
	m := New()
	m.SetSize(80, 24)

	if m.values != nil {
		t.Error("expected nil values initially")
	}

	vp := &jql.ValueProvider{Statuses: []string{"Open"}}
	m.SetValues(vp)

	if m.values == nil {
		t.Error("expected values after SetValues")
	}
	if len(m.values.Statuses) != 1 {
		t.Errorf("expected 1 status, got %d", len(m.values.Statuses))
	}
}

func TestEditQuery_CompletionsReappearAfterSpace(t *testing.T) {
	vp := &jql.ValueProvider{
		Statuses: []string{"Done"},
	}
	m := queryModel("status = D", vp)
	m.recalcCompletions()
	m.compIndex = 0
	m.acceptCompletion()

	// Type space — should get keyword completions (AND/OR/etc.).
	m, _ = m.Update(key(" "))
	if len(m.completions) == 0 {
		t.Error("expected completions to reappear after space")
	}
}

func TestXKey_NoOp(t *testing.T) {
	m := New()
	m.SetSize(80, 24)
	m.SetFilters(sampleFilters())

	m, _ = m.Update(key("x"))
	// Should remain in list state with no side effects.
	if m.state != stateList {
		t.Errorf("expected stateList after x, got %d", m.state)
	}
	if m.Dismissed() {
		t.Error("x should not dismiss")
	}
	if m.Applied() != nil {
		t.Error("x should not apply a filter")
	}
}

func TestEditQuery_BackspaceRecalculatesCompletions(t *testing.T) {
	vp := &jql.ValueProvider{
		Statuses: []string{"Done"},
	}
	m := queryModel("status = D", vp)
	m.recalcCompletions()
	m.compIndex = 0
	m.acceptCompletion()

	// Backspace — should recalculate.
	m, _ = m.Update(keyType(tea.KeyBackspace))
	if len(m.completions) == 0 {
		t.Error("expected completions to reappear after backspace")
	}
}
