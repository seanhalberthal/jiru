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
	IssueType:   "Story",
	StoryPoints: floatPtr(5.5),
	FixVersions: []string{"v1.0", "v1.1"},
}

func floatPtr(f float64) *float64 { return &f }

var testPriorities = []string{"Highest", "High", "Medium", "Low", "Lowest"}
var testIssueTypes = []string{"Bug", "Story", "Task", "Sub-task"}

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
	m.SetIssue(testIssue, testPriorities, testIssueTypes)

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

	if m.origIssueType != "Story" {
		t.Errorf("origIssueType = %q, want Story", m.origIssueType)
	}
	if m.issueTypeCursor != 1 {
		t.Errorf("issueTypeCursor = %d, want 1 (index of Story)", m.issueTypeCursor)
	}
	if *m.origStoryPoints != 5.5 {
		t.Errorf("origStoryPoints = %f, want 5.5", *m.origStoryPoints)
	}
	if m.storyPoints.Value() != "5.5" {
		t.Errorf("storyPoints.Value() = %q, want 5.5", m.storyPoints.Value())
	}
	if m.fixVersions.Value() != "v1.0, v1.1" {
		t.Errorf("fixVersions.Value() = %q, want 'v1.0, v1.1'", m.fixVersions.Value())
	}
}

func TestJCyclesForwardThroughAllFields(t *testing.T) {
	m := New("PROJ-1")
	m.SetIssue(testIssue, testPriorities, testIssueTypes)

	jKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}

	// Start at summary (0).
	if m.activeField != fieldSummary {
		t.Fatalf("expected to start at fieldSummary, got %d", m.activeField)
	}

	// j to issueType (1).
	m, _ = m.Update(jKey)
	if m.activeField != fieldIssueType {
		t.Errorf("after 1 j: activeField = %d, want %d (fieldIssueType)", m.activeField, fieldIssueType)
	}

	// j to priority (2).
	m, _ = m.Update(jKey)
	if m.activeField != fieldPriority {
		t.Errorf("after 2 j: activeField = %d, want %d (fieldPriority)", m.activeField, fieldPriority)
	}

	// j to storyPoints (3).
	m, _ = m.Update(jKey)
	if m.activeField != fieldStoryPoints {
		t.Errorf("after 3 j: activeField = %d, want %d (fieldStoryPoints)", m.activeField, fieldStoryPoints)
	}

	// j to labels (4).
	m, _ = m.Update(jKey)
	if m.activeField != fieldLabels {
		t.Errorf("after 4 j: activeField = %d, want %d (fieldLabels)", m.activeField, fieldLabels)
	}

	// j to fixVersions (5).
	m, _ = m.Update(jKey)
	if m.activeField != fieldFixVersions {
		t.Errorf("after 5 j: activeField = %d, want %d (fieldFixVersions)", m.activeField, fieldFixVersions)
	}

	// j to description (6).
	m, _ = m.Update(jKey)
	if m.activeField != fieldDescription {
		t.Errorf("after 6 j: activeField = %d, want %d (fieldDescription)", m.activeField, fieldDescription)
	}

	// j wraps back to summary (0).
	m, _ = m.Update(jKey)
	if m.activeField != fieldSummary {
		t.Errorf("after 7 j: activeField = %d, want %d (fieldSummary, wrap)", m.activeField, fieldSummary)
	}
}

func TestKCyclesBackwardThroughAllFields(t *testing.T) {
	m := New("PROJ-1")
	m.SetIssue(testIssue, testPriorities, testIssueTypes)

	kKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}

	// Start at summary (0).
	if m.activeField != fieldSummary {
		t.Fatalf("expected to start at fieldSummary, got %d", m.activeField)
	}

	// k wraps to description (6).
	m, _ = m.Update(kKey)
	if m.activeField != fieldDescription {
		t.Errorf("after 1 k: activeField = %d, want %d (fieldDescription)", m.activeField, fieldDescription)
	}

	// k to fixVersions (5).
	m, _ = m.Update(kKey)
	if m.activeField != fieldFixVersions {
		t.Errorf("after 2 k: activeField = %d, want %d (fieldFixVersions)", m.activeField, fieldFixVersions)
	}

	// k to labels (4).
	m, _ = m.Update(kKey)
	if m.activeField != fieldLabels {
		t.Errorf("after 3 k: activeField = %d, want %d (fieldLabels)", m.activeField, fieldLabels)
	}

	// k to storyPoints (3).
	m, _ = m.Update(kKey)
	if m.activeField != fieldStoryPoints {
		t.Errorf("after 4 k: activeField = %d, want %d (fieldStoryPoints)", m.activeField, fieldStoryPoints)
	}

	// k to priority (2).
	m, _ = m.Update(kKey)
	if m.activeField != fieldPriority {
		t.Errorf("after 5 k: activeField = %d, want %d (fieldPriority)", m.activeField, fieldPriority)
	}

	// k to issueType (1).
	m, _ = m.Update(kKey)
	if m.activeField != fieldIssueType {
		t.Errorf("after 6 k: activeField = %d, want %d (fieldIssueType)", m.activeField, fieldIssueType)
	}

	// k back to summary (0).
	m, _ = m.Update(kKey)
	if m.activeField != fieldSummary {
		t.Errorf("after 7 k: activeField = %d, want %d (fieldSummary)", m.activeField, fieldSummary)
	}
}

func TestDescriptionFieldForwardsMessages(t *testing.T) {
	m := New("PROJ-1")
	m.SetIssue(testIssue, testPriorities, testIssueTypes)
	m.SetSize(80, 24)

	// Navigate to description field.
	jKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	m, _ = m.Update(jKey) // → issueType
	m, _ = m.Update(jKey) // → priority
	m, _ = m.Update(jKey) // → storyPoints
	m, _ = m.Update(jKey) // → labels
	m, _ = m.Update(jKey) // → fixVersions
	m, _ = m.Update(jKey) // → description

	if m.activeField != fieldDescription {
		t.Fatalf("expected fieldDescription, got %d", m.activeField)
	}

	// Enter edit mode, clear the textarea and type new text. The textarea
	// starts with content from SetIssue, so we select all and replace.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
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
	m.SetIssue(testIssue, testPriorities, testIssueTypes)
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
	m.SetIssue(testIssue, testPriorities, testIssueTypes)
	m.SetSize(80, 24)

	// Navigate to description and change it.
	jKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	m, _ = m.Update(jKey) // → issueType
	m, _ = m.Update(jKey) // → priority
	m, _ = m.Update(jKey) // → storyPoints
	m, _ = m.Update(jKey) // → labels
	m, _ = m.Update(jKey) // → fixVersions
	m, _ = m.Update(jKey) // → description

	// Enter edit mode, select all and type new content.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
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
	m.SetIssue(testIssue, testPriorities, testIssueTypes)
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
	if !strings.Contains(view, "j/k") {
		t.Error("View should contain j/k help text")
	}
	if !strings.Contains(view, "h/l") {
		t.Error("View should contain h/l help text")
	}
	if !strings.Contains(view, "enter") {
		t.Error("View should contain enter help text")
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
	// boxWidth = min(76, 180) = 76; contentWidth = 76 - 8 = 68;
	// inputWidth = 68 - 14 (label) - 2 (textinput prompt) = 52.
	if m.summary.Width != 52 {
		t.Errorf("summary.Width = %d, want 52", m.summary.Width)
	}
	if m.labels.Width != 52 {
		t.Errorf("labels.Width = %d, want 52", m.labels.Width)
	}
	// Description textarea width should also be set (guarded by issueKey).
	// No panic = the guard passed.
}

func TestSetSize_NarrowTerminalDoesNotPanic(t *testing.T) {
	m := New("PROJ-1")

	// boxWidth = 1, which is <= boxChromeWidth+labelWidth, so SetSize returns early.
	m.SetSize(5, 10)
	// No panic = pass.
}

func TestCtrlS_SubmitsWithDescriptionDiff(t *testing.T) {
	m := New("PROJ-42")
	m.SetIssue(testIssue, testPriorities, testIssueTypes)
	m.SetSize(80, 24)

	// Navigate to description and change it.
	jKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	m, _ = m.Update(jKey) // → issueType
	m, _ = m.Update(jKey) // → priority
	m, _ = m.Update(jKey) // → storyPoints
	m, _ = m.Update(jKey) // → labels
	m, _ = m.Update(jKey) // → fixVersions
	m, _ = m.Update(jKey) // → description

	// Enter edit mode, select all and type new content.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
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
	m.SetIssue(testIssue, testPriorities, testIssueTypes)
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
	m.SetIssue(testIssue, testPriorities, testIssueTypes)
	m.SetSize(80, 24)

	// Navigate to priority field.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}) // → issueType
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}) // → priority

	if m.activeField != fieldPriority {
		t.Fatalf("expected fieldPriority, got %d", m.activeField)
	}

	// Priority cursor starts at 2 (Medium). Move right.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if m.currentPriority() != "Low" {
		t.Errorf("after l from Medium, priority = %q, want Low", m.currentPriority())
	}

	// Move left.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if m.currentPriority() != "Medium" {
		t.Errorf("after h from Low, priority = %q, want Medium", m.currentPriority())
	}
}

func TestIssueTypeNavigation(t *testing.T) {
	m := New("PROJ-42")
	m.SetIssue(testIssue, testPriorities, testIssueTypes)
	m.SetSize(80, 24)

	// Navigate to issueType field.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}) // → issueType

	if m.activeField != fieldIssueType {
		t.Fatalf("expected fieldIssueType, got %d", m.activeField)
	}

	// IssueType cursor starts at 1 (Story). Move right.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if m.currentIssueType() != "Task" {
		t.Errorf("after l from Story, issueType = %q, want Task", m.currentIssueType())
	}

	// Move left.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if m.currentIssueType() != "Story" {
		t.Errorf("after h from Task, issueType = %q, want Story", m.currentIssueType())
	}
}

func TestStoryPointsValidationAndDoublePointer(t *testing.T) {
	// Case 1: Value changed.
	{
		m := New("PROJ-42")
		m.SetIssue(testIssue, testPriorities, testIssueTypes)
		m.SetSize(80, 24)

		jKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
		m, _ = m.Update(jKey) // → issueType
		m, _ = m.Update(jKey) // → priority
		m, _ = m.Update(jKey) // → storyPoints

		if m.activeField != fieldStoryPoints {
			t.Fatalf("expected fieldStoryPoints, got %d", m.activeField)
		}

		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlA})
		m = typeText(t, m, "8.5")

		req := m.buildRequest()
		if req.StoryPoints == nil || *req.StoryPoints == nil || **req.StoryPoints != 8.5 {
			t.Errorf("expected StoryPoints to be 8.5, got %v", req.StoryPoints)
		}
	}

	// Case 2: Value cleared.
	{
		m := New("PROJ-42")
		m.SetIssue(testIssue, testPriorities, testIssueTypes)
		m.SetSize(80, 24)

		jKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
		m, _ = m.Update(jKey) // → issueType
		m, _ = m.Update(jKey) // → priority
		m, _ = m.Update(jKey) // → storyPoints

		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlA})
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})

		req := m.buildRequest()
		if req.StoryPoints == nil || *req.StoryPoints != nil {
			t.Errorf("expected double-pointer to resolve to nil float64, got %v", req.StoryPoints)
		}
	}

	// Case 3: No change.
	{
		m := New("PROJ-42")
		m.SetIssue(testIssue, testPriorities, testIssueTypes)
		m.SetSize(80, 24)

		req := m.buildRequest()
		if req.StoryPoints != nil {
			t.Errorf("expected nil StoryPoints for no change, got %v", req.StoryPoints)
		}
	}
}

func TestFixVersionsDiff(t *testing.T) {
	m := New("PROJ-42")
	m.SetIssue(testIssue, testPriorities, testIssueTypes)
	m.SetSize(80, 24)

	jKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	for i := 0; i < 5; i++ {
		m, _ = m.Update(jKey)
	}

	if m.activeField != fieldFixVersions {
		t.Fatalf("expected fieldFixVersions, got %d", m.activeField)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlA})
	m = typeText(t, m, "v1.1, v2.0")

	req := m.buildRequest()
	if len(req.FixVersions) != 2 {
		t.Fatalf("expected 2 fix version ops, got %v", req.FixVersions)
	}

	hasRemove := false
	hasAdd := false
	for _, op := range req.FixVersions {
		if op == "-v1.0" {
			hasRemove = true
		}
		if op == "v2.0" {
			hasAdd = true
		}
	}
	if !hasRemove {
		t.Error("expected removal op '-v1.0'")
	}
	if !hasAdd {
		t.Error("expected addition op 'v2.0'")
	}
}

func TestLabelsFieldForwardsMessages(t *testing.T) {
	m := New("PROJ-1")
	m.SetSize(80, 24)

	// Navigate to labels field.
	jKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	m, _ = m.Update(jKey) // → issueType
	m, _ = m.Update(jKey) // → priority
	m, _ = m.Update(jKey) // → storyPoints
	m, _ = m.Update(jKey) // → labels

	if m.activeField != fieldLabels {
		t.Fatalf("expected fieldLabels, got %d", m.activeField)
	}

	m = typeText(t, m, "new-label")
	if !strings.Contains(m.labels.Value(), "new-label") {
		t.Errorf("labels.Value() = %q, want it to contain %q", m.labels.Value(), "new-label")
	}
}

func TestSummaryFieldForwardsMessages(t *testing.T) {
	m := New("PROJ-1")
	m.SetSize(80, 24)

	// Summary field is active by default. Type into it.
	if m.activeField != fieldSummary {
		t.Fatalf("expected fieldSummary, got %d", m.activeField)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = typeText(t, m, "New summary")
	if !strings.Contains(m.summary.Value(), "New summary") {
		t.Errorf("summary.Value() = %q, want it to contain %q", m.summary.Value(), "New summary")
	}
}

// --- parseLabels tests ---

func TestParseLabels_NormalInput(t *testing.T) {
	got := parseLabels("frontend, backend, api")
	want := []string{"frontend", "backend", "api"}
	if len(got) != len(want) {
		t.Fatalf("parseLabels() returned %d items, want %d", len(got), len(want))
	}
	for i, v := range got {
		if v != want[i] {
			t.Errorf("parseLabels()[%d] = %q, want %q", i, v, want[i])
		}
	}
}

func TestParseLabels_EmptyString(t *testing.T) {
	got := parseLabels("")
	if got != nil {
		t.Errorf("parseLabels(\"\") = %v, want nil", got)
	}
}

func TestParseLabels_WhitespaceOnly(t *testing.T) {
	got := parseLabels("   ")
	if got != nil {
		t.Errorf("parseLabels(\"   \") = %v, want nil", got)
	}
}

func TestParseLabels_TrailingCommas(t *testing.T) {
	got := parseLabels("alpha, beta,")
	want := []string{"alpha", "beta"}
	if len(got) != len(want) {
		t.Fatalf("parseLabels() returned %d items, want %d", len(got), len(want))
	}
	for i, v := range got {
		if v != want[i] {
			t.Errorf("parseLabels()[%d] = %q, want %q", i, v, want[i])
		}
	}
}

func TestParseLabels_LeadingCommas(t *testing.T) {
	got := parseLabels(",alpha, beta")
	want := []string{"alpha", "beta"}
	if len(got) != len(want) {
		t.Fatalf("parseLabels() returned %d items, want %d", len(got), len(want))
	}
	for i, v := range got {
		if v != want[i] {
			t.Errorf("parseLabels()[%d] = %q, want %q", i, v, want[i])
		}
	}
}

func TestParseLabels_ExtraWhitespace(t *testing.T) {
	got := parseLabels("  alpha  ,  beta  ,  gamma  ")
	want := []string{"alpha", "beta", "gamma"}
	if len(got) != len(want) {
		t.Fatalf("parseLabels() returned %d items, want %d", len(got), len(want))
	}
	for i, v := range got {
		if v != want[i] {
			t.Errorf("parseLabels()[%d] = %q, want %q", i, v, want[i])
		}
	}
}

func TestParseLabels_SingleLabel(t *testing.T) {
	got := parseLabels("solo")
	if len(got) != 1 || got[0] != "solo" {
		t.Errorf("parseLabels(\"solo\") = %v, want [\"solo\"]", got)
	}
}

// --- computeLabelsDiff tests ---

func TestComputeLabelsDiff_Additions(t *testing.T) {
	ops := computeLabelsDiff([]string{"existing"}, []string{"existing", "new-one"})
	// Only the new label should appear (as a plain string, no prefix).
	if len(ops) != 1 {
		t.Fatalf("expected 1 op, got %d: %v", len(ops), ops)
	}
	if ops[0] != "new-one" {
		t.Errorf("expected addition 'new-one', got %q", ops[0])
	}
}

func TestComputeLabelsDiff_Removals(t *testing.T) {
	ops := computeLabelsDiff([]string{"old-one", "keep"}, []string{"keep"})
	// The removed label should appear with a "-" prefix.
	if len(ops) != 1 {
		t.Fatalf("expected 1 op, got %d: %v", len(ops), ops)
	}
	if ops[0] != "-old-one" {
		t.Errorf("expected removal '-old-one', got %q", ops[0])
	}
}

func TestComputeLabelsDiff_UnchangedExcluded(t *testing.T) {
	ops := computeLabelsDiff([]string{"alpha", "beta"}, []string{"alpha", "beta"})
	if len(ops) != 0 {
		t.Errorf("expected no ops for identical labels, got %v", ops)
	}
}

func TestComputeLabelsDiff_EmptyInputs(t *testing.T) {
	ops := computeLabelsDiff(nil, nil)
	if len(ops) != 0 {
		t.Errorf("expected no ops for nil inputs, got %v", ops)
	}

	ops = computeLabelsDiff([]string{}, []string{})
	if len(ops) != 0 {
		t.Errorf("expected no ops for empty inputs, got %v", ops)
	}
}

func TestComputeLabelsDiff_AllRemoved(t *testing.T) {
	ops := computeLabelsDiff([]string{"a", "b", "c"}, nil)
	if len(ops) != 3 {
		t.Fatalf("expected 3 removal ops, got %d: %v", len(ops), ops)
	}
	for _, op := range ops {
		if !strings.HasPrefix(op, "-") {
			t.Errorf("expected removal prefix '-', got %q", op)
		}
	}
}

func TestComputeLabelsDiff_AllAdded(t *testing.T) {
	ops := computeLabelsDiff(nil, []string{"x", "y"})
	if len(ops) != 2 {
		t.Fatalf("expected 2 addition ops, got %d: %v", len(ops), ops)
	}
	for _, op := range ops {
		if strings.HasPrefix(op, "-") {
			t.Errorf("expected addition (no prefix), got %q", op)
		}
	}
}

func TestComputeLabelsDiff_MixedAdditionsAndRemovals(t *testing.T) {
	ops := computeLabelsDiff([]string{"keep", "remove-me"}, []string{"keep", "add-me"})
	// Should have one removal and one addition, "keep" excluded.
	if len(ops) != 2 {
		t.Fatalf("expected 2 ops, got %d: %v", len(ops), ops)
	}
	hasRemoval := false
	hasAddition := false
	for _, op := range ops {
		if op == "-remove-me" {
			hasRemoval = true
		}
		if op == "add-me" {
			hasAddition = true
		}
	}
	if !hasRemoval {
		t.Error("expected removal '-remove-me' in ops")
	}
	if !hasAddition {
		t.Error("expected addition 'add-me' in ops")
	}
}

// typeText simulates typing each rune into the active input.
func typeText(t *testing.T, m Model, text string) Model {
	t.Helper()
	if !m.editing && isTextField(m.activeField) {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	}
	for _, r := range text {
		var cmd tea.Cmd
		m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		_ = cmd
	}
	return m
}
