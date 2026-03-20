package editview

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/seanhalberthal/jiru/internal/jira"
)

var testIssue = jira.Issue{
	Key:         "PROJ-42",
	Summary:     "Original summary",
	Description: "Original description",
	Priority:    "Medium",
	Labels:      []string{"backend", "urgent"},
}

var testPriorities = []string{"Highest", "High", "Medium", "Low", "Lowest"}

func TestNew_InitialisesWithTextarea(t *testing.T) {
	m := New("PROJ-1")

	if m.issueKey != "PROJ-1" {
		t.Errorf("issueKey = %q, want %q", m.issueKey, "PROJ-1")
	}
	if m.activeField != fieldSummary {
		t.Errorf("activeField = %d, want %d (fieldSummary)", m.activeField, fieldSummary)
	}
	// Description textarea should be initialised with placeholder.
	if m.description.Placeholder == "" {
		t.Error("description textarea should have a placeholder")
	}
	if !m.InputActive() {
		t.Error("InputActive() should always return true")
	}
}

func TestSetIssue_PopulatesDescriptionAndOrigDescription(t *testing.T) {
	m := New("PROJ-42")
	m.SetIssue(testIssue, testPriorities)

	if m.description.Value() != "Original description" {
		t.Errorf("description value = %q, want %q", m.description.Value(), "Original description")
	}
	if m.origDescription != "Original description" {
		t.Errorf("origDescription = %q, want %q", m.origDescription, "Original description")
	}
	// Cursor should be at the start so it's visible without scrolling.
	if row := m.description.Line(); row != 0 {
		t.Errorf("description cursor row = %d, want 0", row)
	}
	if col := m.description.LineInfo().ColumnOffset; col != 0 {
		t.Errorf("description cursor col = %d, want 0", col)
	}
	if m.summary.Value() != "Original summary" {
		t.Errorf("summary value = %q, want %q", m.summary.Value(), "Original summary")
	}
	if m.origSummary != "Original summary" {
		t.Errorf("origSummary = %q, want %q", m.origSummary, "Original summary")
	}
	if m.origPriority != "Medium" {
		t.Errorf("origPriority = %q, want %q", m.origPriority, "Medium")
	}
	if m.priorityCursor != 2 {
		t.Errorf("priorityCursor = %d, want 2 (index of Medium)", m.priorityCursor)
	}
}

func TestTabCyclesForwardThroughAllFields(t *testing.T) {
	m := New("PROJ-1")
	m.SetIssue(testIssue, testPriorities)

	tabKey := tea.KeyMsg{Type: tea.KeyTab}

	// Start at summary (0).
	if m.activeField != fieldSummary {
		t.Fatalf("expected to start at fieldSummary, got %d", m.activeField)
	}

	// Tab to priority (1).
	m, _ = m.Update(tabKey)
	if m.activeField != fieldPriority {
		t.Errorf("after 1 tab: activeField = %d, want %d (fieldPriority)", m.activeField, fieldPriority)
	}

	// Tab to labels (2).
	m, _ = m.Update(tabKey)
	if m.activeField != fieldLabels {
		t.Errorf("after 2 tabs: activeField = %d, want %d (fieldLabels)", m.activeField, fieldLabels)
	}

	// Tab to description (3).
	m, _ = m.Update(tabKey)
	if m.activeField != fieldDescription {
		t.Errorf("after 3 tabs: activeField = %d, want %d (fieldDescription)", m.activeField, fieldDescription)
	}

	// Tab wraps back to summary (0).
	m, _ = m.Update(tabKey)
	if m.activeField != fieldSummary {
		t.Errorf("after 4 tabs: activeField = %d, want %d (fieldSummary, wrap)", m.activeField, fieldSummary)
	}
}

func TestShiftTabCyclesBackwardThroughAllFields(t *testing.T) {
	m := New("PROJ-1")
	m.SetIssue(testIssue, testPriorities)

	shiftTabKey := tea.KeyMsg{Type: tea.KeyShiftTab}

	// Start at summary (0).
	if m.activeField != fieldSummary {
		t.Fatalf("expected to start at fieldSummary, got %d", m.activeField)
	}

	// Shift+Tab wraps to description (3).
	m, _ = m.Update(shiftTabKey)
	if m.activeField != fieldDescription {
		t.Errorf("after 1 shift+tab: activeField = %d, want %d (fieldDescription)", m.activeField, fieldDescription)
	}

	// Shift+Tab to labels (2).
	m, _ = m.Update(shiftTabKey)
	if m.activeField != fieldLabels {
		t.Errorf("after 2 shift+tabs: activeField = %d, want %d (fieldLabels)", m.activeField, fieldLabels)
	}

	// Shift+Tab to priority (1).
	m, _ = m.Update(shiftTabKey)
	if m.activeField != fieldPriority {
		t.Errorf("after 3 shift+tabs: activeField = %d, want %d (fieldPriority)", m.activeField, fieldPriority)
	}

	// Shift+Tab back to summary (0).
	m, _ = m.Update(shiftTabKey)
	if m.activeField != fieldSummary {
		t.Errorf("after 4 shift+tabs: activeField = %d, want %d (fieldSummary)", m.activeField, fieldSummary)
	}
}

func TestDescriptionFieldForwardsMessages(t *testing.T) {
	m := New("PROJ-1")
	m.SetIssue(testIssue, testPriorities)
	m.SetSize(80, 24)

	// Navigate to description field.
	tabKey := tea.KeyMsg{Type: tea.KeyTab}
	m, _ = m.Update(tabKey) // → priority
	m, _ = m.Update(tabKey) // → labels
	m, _ = m.Update(tabKey) // → description

	if m.activeField != fieldDescription {
		t.Fatalf("expected fieldDescription, got %d", m.activeField)
	}

	// Clear the textarea and type new text. The textarea starts with content
	// from SetIssue, so we select all and replace.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlA})
	// Type replacement text rune by rune.
	m = typeText(t, m, "New description content")

	got := m.description.Value()
	if !strings.Contains(got, "New description content") {
		t.Errorf("description value = %q, expected it to contain %q", got, "New description content")
	}
}

func TestBuildRequest_OnlyIncludesDescriptionWhenChanged(t *testing.T) {
	m := New("PROJ-42")
	m.SetIssue(testIssue, testPriorities)
	m.SetSize(80, 24)

	// No changes — description should be empty in request.
	req := m.buildRequest()
	if req.Description != "" {
		t.Errorf("buildRequest().Description = %q, want empty (no change)", req.Description)
	}
	if req.Summary != "" {
		t.Errorf("buildRequest().Summary = %q, want empty (no change)", req.Summary)
	}
	if req.Priority != "" {
		t.Errorf("buildRequest().Priority = %q, want empty (no change)", req.Priority)
	}
	if req.Labels != nil {
		t.Errorf("buildRequest().Labels = %v, want nil (no change)", req.Labels)
	}
}

func TestBuildRequest_IncludesDescriptionWhenChanged(t *testing.T) {
	m := New("PROJ-42")
	m.SetIssue(testIssue, testPriorities)
	m.SetSize(80, 24)

	// Navigate to description and change it.
	tabKey := tea.KeyMsg{Type: tea.KeyTab}
	m, _ = m.Update(tabKey) // → priority
	m, _ = m.Update(tabKey) // → labels
	m, _ = m.Update(tabKey) // → description

	// Select all and type new content.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlA})
	m = typeText(t, m, "Updated description")

	req := m.buildRequest()
	if req.Description == "" {
		t.Error("buildRequest().Description should be non-empty when description changed")
	}
	if !strings.Contains(req.Description, "Updated description") {
		t.Errorf("buildRequest().Description = %q, want it to contain %q", req.Description, "Updated description")
	}

	// Other fields should remain unchanged.
	if req.Summary != "" {
		t.Errorf("buildRequest().Summary = %q, want empty (no change)", req.Summary)
	}
	if req.Priority != "" {
		t.Errorf("buildRequest().Priority = %q, want empty (no change)", req.Priority)
	}
}

func TestView_RendersDescriptionField(t *testing.T) {
	m := New("PROJ-42")
	m.SetIssue(testIssue, testPriorities)
	m.SetSize(100, 40)

	view := m.View()
	if !strings.Contains(view, "Desc") {
		t.Error("View should contain the description label 'Desc'")
	}
	if !strings.Contains(view, "PROJ-42") {
		t.Error("View should contain the issue key")
	}
}

func TestView_ContainsHelpText(t *testing.T) {
	m := New("PROJ-1")
	m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, "tab") {
		t.Error("View should contain tab help text")
	}
	if !strings.Contains(view, "ctrl+s") {
		t.Error("View should contain ctrl+s help text")
	}
	if !strings.Contains(view, "esc") {
		t.Error("View should contain esc help text")
	}
}

func TestSetSize_HandlesTextareaWithIssueKeyGuard(t *testing.T) {
	// A zero-value Model has no issueKey — SetSize should not panic on
	// the description textarea.
	m := Model{}
	m.SetSize(80, 24)
	// No panic = pass. The description.SetWidth is guarded by issueKey != "".
}

func TestSetSize_SetsTextareaWidthWhenIssueKeyPresent(t *testing.T) {
	m := New("PROJ-1")

	m.SetSize(80, 24)
	// inputWidth = min(120, 80*4/5) = min(120, 64) = 64
	if m.summary.Width != 64 {
		t.Errorf("summary.Width = %d, want 64", m.summary.Width)
	}
	if m.labels.Width != 64 {
		t.Errorf("labels.Width = %d, want 64", m.labels.Width)
	}
	// Description textarea width should also be set (guarded by issueKey).
	// No panic = the guard passed.
}

func TestSetSize_NarrowTerminalDoesNotPanic(t *testing.T) {
	m := New("PROJ-1")

	// inputWidth = min(60, 5-12) = -7 which is <= 0, so nothing is set.
	m.SetSize(5, 10)
	// No panic = pass.
}

func TestCtrlS_SubmitsWithDescriptionDiff(t *testing.T) {
	m := New("PROJ-42")
	m.SetIssue(testIssue, testPriorities)
	m.SetSize(80, 24)

	// Navigate to description and change it.
	tabKey := tea.KeyMsg{Type: tea.KeyTab}
	m, _ = m.Update(tabKey) // → priority
	m, _ = m.Update(tabKey) // → labels
	m, _ = m.Update(tabKey) // → description

	// Select all and type new content.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlA})
	m = typeText(t, m, "Changed desc")

	// Submit with ctrl+s.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})

	req := m.SubmittedEdit()
	if req == nil {
		t.Fatal("SubmittedEdit() should not be nil after ctrl+s")
	}
	if !strings.Contains(req.Description, "Changed desc") {
		t.Errorf("SubmittedEdit().Description = %q, want it to contain %q", req.Description, "Changed desc")
	}

	// Sentinel should clear after first read.
	if m.SubmittedEdit() != nil {
		t.Error("SubmittedEdit() second call should return nil")
	}
}

func TestCtrlS_SubmitsWithNoChanges(t *testing.T) {
	m := New("PROJ-42")
	m.SetIssue(testIssue, testPriorities)
	m.SetSize(80, 24)

	// Submit immediately without changing anything.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})

	req := m.SubmittedEdit()
	if req == nil {
		t.Fatal("SubmittedEdit() should not be nil after ctrl+s even with no changes")
	}
	if req.Description != "" {
		t.Errorf("Description = %q, want empty (no change)", req.Description)
	}
	if req.Summary != "" {
		t.Errorf("Summary = %q, want empty (no change)", req.Summary)
	}
	if req.Priority != "" {
		t.Errorf("Priority = %q, want empty (no change)", req.Priority)
	}
	if req.Labels != nil {
		t.Errorf("Labels = %v, want nil (no change)", req.Labels)
	}
}

func TestDismissed_OnEsc(t *testing.T) {
	m := New("PROJ-1")
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

func TestInputActive_AlwaysTrue(t *testing.T) {
	m := New("PROJ-1")
	if !m.InputActive() {
		t.Error("InputActive() should be true initially")
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !m.InputActive() {
		t.Error("InputActive() should remain true even after dismiss")
	}
}

func TestPriorityNavigation_InDescriptionField(t *testing.T) {
	m := New("PROJ-42")
	m.SetIssue(testIssue, testPriorities)
	m.SetSize(80, 24)

	// Navigate to priority field.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab}) // → priority

	if m.activeField != fieldPriority {
		t.Fatalf("expected fieldPriority, got %d", m.activeField)
	}

	// Priority cursor starts at 2 (Medium). Move down.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.currentPriority() != "Low" {
		t.Errorf("after j from Medium, priority = %q, want Low", m.currentPriority())
	}

	// Move up.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.currentPriority() != "Medium" {
		t.Errorf("after k from Low, priority = %q, want Medium", m.currentPriority())
	}
}

func TestSummaryFieldForwardsMessages(t *testing.T) {
	m := New("PROJ-1")
	m.SetSize(80, 24)

	// Summary field is active by default. Type into it.
	if m.activeField != fieldSummary {
		t.Fatalf("expected fieldSummary, got %d", m.activeField)
	}

	m = typeText(t, m, "New summary")
	if !strings.Contains(m.summary.Value(), "New summary") {
		t.Errorf("summary.Value() = %q, want it to contain %q", m.summary.Value(), "New summary")
	}
}

func TestLabelsFieldForwardsMessages(t *testing.T) {
	m := New("PROJ-1")
	m.SetSize(80, 24)

	// Navigate to labels field.
	tabKey := tea.KeyMsg{Type: tea.KeyTab}
	m, _ = m.Update(tabKey) // → priority
	m, _ = m.Update(tabKey) // → labels

	if m.activeField != fieldLabels {
		t.Fatalf("expected fieldLabels, got %d", m.activeField)
	}

	m = typeText(t, m, "new-label")
	if !strings.Contains(m.labels.Value(), "new-label") {
		t.Errorf("labels.Value() = %q, want it to contain %q", m.labels.Value(), "new-label")
	}
}

// typeText simulates typing each rune into the active input.
func typeText(t *testing.T, m Model, text string) Model {
	t.Helper()
	for _, r := range text {
		var cmd tea.Cmd
		m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		_ = cmd
	}
	return m
}
