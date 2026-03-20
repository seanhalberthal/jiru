package linkview

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/seanhalberthal/jiru/internal/jira"
)

var testLinkTypes = []jira.IssueLinkType{
	{ID: "1", Name: "Blocks", Inward: "is blocked by", Outward: "blocks"},
	{ID: "2", Name: "Relates", Inward: "relates to", Outward: "relates to"},
}

func TestNew_StartsInLoadingState(t *testing.T) {
	m := New("PROJ-42")

	if m.issueKey != "PROJ-42" {
		t.Errorf("issueKey = %q, want %q", m.issueKey, "PROJ-42")
	}
	if !m.loading {
		t.Error("should start in loading state")
	}
	if m.InputActive() {
		t.Error("InputActive() should be false initially (step is pickType, not enterKey)")
	}
}

func TestSetLinkTypes_PopulatesEntries(t *testing.T) {
	m := New("PROJ-1")
	m.SetLinkTypes(testLinkTypes)

	if m.loading {
		t.Error("loading should be false after SetLinkTypes")
	}
	// Each link type produces two entries (outward + inward).
	if len(m.entries) != 4 {
		t.Fatalf("entries count = %d, want 4", len(m.entries))
	}

	// Verify order: outward first, then inward for each type.
	expected := []struct {
		label     string
		isOutward bool
	}{
		{"blocks →", true},
		{"is blocked by ←", false},
		{"relates to →", true},
		{"relates to ←", false},
	}

	for i, want := range expected {
		got := m.entries[i]
		if got.label != want.label {
			t.Errorf("entries[%d].label = %q, want %q", i, got.label, want.label)
		}
		if got.isOutward != want.isOutward {
			t.Errorf("entries[%d].isOutward = %v, want %v", i, got.isOutward, want.isOutward)
		}
	}
}

func TestLoading_SuppressesAllKeys(t *testing.T) {
	m := New("PROJ-1")
	m.SetSize(80, 24)

	keys := []tea.KeyMsg{
		{Type: tea.KeyEnter},
		{Type: tea.KeyEsc},
		{Type: tea.KeyRunes, Runes: []rune{'j'}},
		{Type: tea.KeyRunes, Runes: []rune{'k'}},
	}

	for _, k := range keys {
		m, _ = m.Update(k)
	}

	if m.SubmittedLink() != nil {
		t.Error("SubmittedLink() should be nil in loading state")
	}
	if m.Dismissed() {
		t.Error("Dismissed() should be false in loading state")
	}
}

func TestDismissed_OnEsc_StepPickType(t *testing.T) {
	m := New("PROJ-1")
	m.SetSize(80, 24)
	m.SetLinkTypes(testLinkTypes)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if !m.Dismissed() {
		t.Error("Dismissed() should return true after Esc in pick type step")
	}

	// Sentinel should clear after first read.
	if m.Dismissed() {
		t.Error("Dismissed() second call should return false")
	}
}

func TestEsc_StepEnterKey_GoesBackToPickType(t *testing.T) {
	m := New("PROJ-1")
	m.SetSize(80, 24)
	m.SetLinkTypes(testLinkTypes)

	// Enter to advance to step 2.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.step != stepEnterKey {
		t.Fatalf("step = %d, want %d (stepEnterKey)", m.step, stepEnterKey)
	}

	// Esc should go back to step 1, not dismiss.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.step != stepPickType {
		t.Errorf("step = %d, want %d (stepPickType) after esc from step 2", m.step, stepPickType)
	}
	if m.Dismissed() {
		t.Error("Dismissed() should be false — esc from step 2 goes back, not dismiss")
	}
}

func TestCursorNavigation_JK(t *testing.T) {
	m := New("PROJ-1")
	m.SetSize(80, 24)
	m.SetLinkTypes(testLinkTypes)

	// Cursor starts at 0. Move down with j.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.cursor != 1 {
		t.Errorf("cursor = %d after j, want 1", m.cursor)
	}

	// Move up with k.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.cursor != 0 {
		t.Errorf("cursor = %d after k, want 0", m.cursor)
	}
}

func TestCursorNavigation_UpDown(t *testing.T) {
	m := New("PROJ-1")
	m.SetSize(80, 24)
	m.SetLinkTypes(testLinkTypes)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.cursor != 2 {
		t.Errorf("cursor = %d after 2x down, want 2", m.cursor)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.cursor != 1 {
		t.Errorf("cursor = %d after up, want 1", m.cursor)
	}
}

func TestCursorClamping_Top(t *testing.T) {
	m := New("PROJ-1")
	m.SetSize(80, 24)
	m.SetLinkTypes(testLinkTypes)

	// Move up from 0 — should stay at 0.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.cursor != 0 {
		t.Errorf("cursor should be clamped at 0, got %d", m.cursor)
	}
}

func TestCursorClamping_Bottom(t *testing.T) {
	m := New("PROJ-1")
	m.SetSize(80, 24)
	m.SetLinkTypes(testLinkTypes)

	// Move down past the end.
	for range 10 {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	if m.cursor != 3 {
		t.Errorf("cursor should be clamped at 3, got %d", m.cursor)
	}
}

func TestSubmitLink_OutwardDirection(t *testing.T) {
	m := New("PROJ-42")
	m.SetSize(80, 24)
	m.SetLinkTypes(testLinkTypes)

	// Select first entry: "blocks →" (outward).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.step != stepEnterKey {
		t.Fatalf("step = %d, want %d (stepEnterKey)", m.step, stepEnterKey)
	}

	// Type the target issue key.
	m = typeText(t, m, "OTHER-123")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	req := m.SubmittedLink()
	if req == nil {
		t.Fatal("SubmittedLink() should not be nil")
	}
	// Outward: this issue (PROJ-42) is the source/outward, target is inward.
	if req.OutwardKey != "PROJ-42" {
		t.Errorf("OutwardKey = %q, want %q", req.OutwardKey, "PROJ-42")
	}
	if req.InwardKey != "OTHER-123" {
		t.Errorf("InwardKey = %q, want %q", req.InwardKey, "OTHER-123")
	}
	if req.LinkType != "Blocks" {
		t.Errorf("LinkType = %q, want %q", req.LinkType, "Blocks")
	}

	// Sentinel should clear after first read.
	if m.SubmittedLink() != nil {
		t.Error("SubmittedLink() second call should return nil")
	}
}

func TestSubmitLink_InwardDirection(t *testing.T) {
	m := New("PROJ-42")
	m.SetSize(80, 24)
	m.SetLinkTypes(testLinkTypes)

	// Select second entry: "is blocked by ←" (inward).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Type the target issue key.
	m = typeText(t, m, "OTHER-456")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	req := m.SubmittedLink()
	if req == nil {
		t.Fatal("SubmittedLink() should not be nil")
	}
	// Inward: this issue (PROJ-42) is the target/inward, other is outward.
	if req.InwardKey != "PROJ-42" {
		t.Errorf("InwardKey = %q, want %q", req.InwardKey, "PROJ-42")
	}
	if req.OutwardKey != "OTHER-456" {
		t.Errorf("OutwardKey = %q, want %q", req.OutwardKey, "OTHER-456")
	}
	if req.LinkType != "Blocks" {
		t.Errorf("LinkType = %q, want %q", req.LinkType, "Blocks")
	}
}

func TestSubmitLink_InvalidKey(t *testing.T) {
	m := New("PROJ-1")
	m.SetSize(80, 24)
	m.SetLinkTypes(testLinkTypes)

	// Advance to step 2.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Type an invalid key.
	m = typeText(t, m, "not-valid")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.SubmittedLink() != nil {
		t.Error("SubmittedLink() should be nil for invalid issue key")
	}
	if m.errMsg == "" {
		t.Error("errMsg should be set for invalid issue key")
	}
}

func TestSubmitLink_EmptyKey(t *testing.T) {
	m := New("PROJ-1")
	m.SetSize(80, 24)
	m.SetLinkTypes(testLinkTypes)

	// Advance to step 2.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Submit without typing anything.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.SubmittedLink() != nil {
		t.Error("SubmittedLink() should be nil for empty key")
	}
}

func TestEnterWithNoEntries(t *testing.T) {
	m := New("PROJ-1")
	m.SetSize(80, 24)
	m.SetLinkTypes(nil)

	// Enter with no entries should do nothing.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.step != stepPickType {
		t.Errorf("step should remain %d (stepPickType), got %d", stepPickType, m.step)
	}
}

func TestInputActive_StepBased(t *testing.T) {
	m := New("PROJ-1")
	m.SetSize(80, 24)
	m.SetLinkTypes(testLinkTypes)

	// Step 1 (pickType) — input not active.
	if m.InputActive() {
		t.Error("InputActive() should be false in pickType step")
	}

	// Step 2 (enterKey) — input active.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.InputActive() {
		t.Error("InputActive() should be true in enterKey step")
	}
}

func TestSetSize_UpdatesDimensions(t *testing.T) {
	m := New("PROJ-1")
	m.SetSize(100, 50)

	if m.width != 100 {
		t.Errorf("width = %d, want 100", m.width)
	}
	if m.height != 50 {
		t.Errorf("height = %d, want 50", m.height)
	}
}

func TestView_Loading(t *testing.T) {
	m := New("PROJ-42")
	m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, "Loading") {
		t.Error("View should show loading text when link types not yet loaded")
	}
	if !strings.Contains(view, "PROJ-42") {
		t.Error("View should contain the issue key")
	}
}

func TestView_EmptyEntries(t *testing.T) {
	m := New("PROJ-1")
	m.SetSize(80, 24)
	m.SetLinkTypes(nil)

	view := m.View()
	if !strings.Contains(view, "No link types") {
		t.Error("View should show 'No link types' when list is empty")
	}
}

func TestView_PickTypeStep(t *testing.T) {
	m := New("PROJ-1")
	m.SetSize(80, 24)
	m.SetLinkTypes(testLinkTypes)

	view := m.View()
	if !strings.Contains(view, "blocks →") {
		t.Error("View should show outward link label")
	}
	if !strings.Contains(view, "is blocked by ←") {
		t.Error("View should show inward link label")
	}
	if !strings.Contains(view, "Select link type") {
		t.Error("View should contain subtitle 'Select link type'")
	}
}

func TestView_EnterKeyStep(t *testing.T) {
	m := New("PROJ-1")
	m.SetSize(80, 24)
	m.SetLinkTypes(testLinkTypes)

	// Advance to step 2.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	view := m.View()
	if !strings.Contains(view, "Target issue key") {
		t.Error("View should show 'Target issue key' prompt in step 2")
	}
	if !strings.Contains(view, "blocks →") {
		t.Error("View should show the selected link type label")
	}
}

func TestView_ShowsErrorMessage(t *testing.T) {
	m := New("PROJ-1")
	m.SetSize(80, 24)
	m.SetLinkTypes(testLinkTypes)

	// Advance to step 2.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Type invalid key and submit.
	m = typeText(t, m, "bad")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	view := m.View()
	if !strings.Contains(view, "Invalid issue key") {
		t.Error("View should show error message for invalid key")
	}
}

func TestView_ContainsHelpText_PickType(t *testing.T) {
	m := New("PROJ-1")
	m.SetSize(80, 24)
	m.SetLinkTypes(testLinkTypes)

	view := m.View()
	if !strings.Contains(view, "j/k") {
		t.Error("View should contain j/k navigation help")
	}
	if !strings.Contains(view, "enter") {
		t.Error("View should contain enter help")
	}
	if !strings.Contains(view, "esc") {
		t.Error("View should contain esc help")
	}
}

func TestView_ContainsHelpText_EnterKey(t *testing.T) {
	m := New("PROJ-1")
	m.SetSize(80, 24)
	m.SetLinkTypes(testLinkTypes)

	// Advance to step 2.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	view := m.View()
	if !strings.Contains(view, "link") {
		t.Error("View should contain 'link' help text in step 2")
	}
	if !strings.Contains(view, "back") {
		t.Error("View should contain 'back' help text in step 2")
	}
}

// typeText simulates typing each rune into the active text input.
func typeText(t *testing.T, m Model, text string) Model {
	t.Helper()
	for _, r := range text {
		var cmd tea.Cmd
		m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		_ = cmd
	}
	return m
}
