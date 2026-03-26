package wikilistview

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/seanhalberthal/jiru/internal/confluence"
	"github.com/seanhalberthal/jiru/internal/recents"
)

func testSpaces() []confluence.Space {
	return []confluence.Space{
		{ID: "1", Key: "ENG", Name: "Engineering", Type: "global", Description: "Engineering team space"},
		{ID: "2", Key: "DESIGN", Name: "Design", Type: "global"},
		{ID: "3", Key: "~user1", Name: "Personal Space", Type: "personal"},
	}
}

func testPages() []confluence.Page {
	return []confluence.Page{
		{ID: "101", Title: "Getting Started", SpaceID: "1", Version: 3},
		{ID: "102", Title: "Architecture", SpaceID: "1", Version: 7},
	}
}

func testRecents() []recents.Entry {
	return []recents.Entry{
		{PageID: "201", Title: "Recent Page", SpaceKey: "ENG"},
		{PageID: "202", Title: "Another Recent", SpaceKey: "DESIGN"},
	}
}

func sizedModel() Model {
	m := New()
	m = m.SetSize(80, 24)
	return m
}

// --- New / initial state ---

func TestNew_StartsInSpacesState(t *testing.T) {
	m := New()
	if m.state != stateSpaces {
		t.Error("expected initial state to be stateSpaces")
	}
	if m.InPagesState() {
		t.Error("InPagesState() should be false initially")
	}
}

// --- SetSpaces ---

func TestSetSpaces_PopulatesList(t *testing.T) {
	m := sizedModel()
	m = m.SetSpaces(testSpaces())

	if len(m.spaces) != 3 {
		t.Errorf("expected 3 spaces, got %d", len(m.spaces))
	}
}

func TestSetSpaces_GlobalBeforePersonal(t *testing.T) {
	m := sizedModel()
	m = m.SetSpaces(testSpaces())

	items := m.list.Items()
	// Global spaces should come first, personal last.
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	first, ok := items[0].(spaceItem)
	if !ok {
		t.Fatal("first item should be a spaceItem")
	}
	if first.space.Type == "personal" {
		t.Error("first item should be a global space, not personal")
	}
	last, ok := items[len(items)-1].(spaceItem)
	if !ok {
		t.Fatal("last item should be a spaceItem")
	}
	if last.space.Type != "personal" {
		t.Error("last item should be the personal space")
	}
}

// --- SetRecents ---

func TestSetRecents_AddsRecentItemsFirst(t *testing.T) {
	m := sizedModel()
	m = m.SetSpaces(testSpaces())
	m = m.SetRecents(testRecents())

	items := m.list.Items()
	if len(items) != 5 { // 2 recents + 2 global + 1 personal
		t.Fatalf("expected 5 items, got %d", len(items))
	}
	if _, ok := items[0].(recentItem); !ok {
		t.Error("first item should be a recentItem")
	}
	if _, ok := items[1].(recentItem); !ok {
		t.Error("second item should be a recentItem")
	}
}

func TestSetRecents_OnlyRebuildsInSpacesState(t *testing.T) {
	m := sizedModel()
	m = m.SetSpaces(testSpaces())

	// Transition to pages state.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Set pages so we have items.
	m = m.SetPages(testPages())
	itemsBefore := len(m.list.Items())

	// SetRecents should not rebuild the list while in pages state.
	m = m.SetRecents(testRecents())
	itemsAfter := len(m.list.Items())

	if itemsAfter != itemsBefore {
		t.Errorf("SetRecents in pages state should not change list items (before=%d, after=%d)", itemsBefore, itemsAfter)
	}
}

// --- SetPages ---

func TestSetPages_PopulatesList(t *testing.T) {
	m := sizedModel()
	m.spaceKey = "ENG"
	m = m.SetPages(testPages())

	items := m.list.Items()
	if len(items) != 2 {
		t.Errorf("expected 2 page items, got %d", len(items))
	}
}

// --- Open space ---

func TestOpenSpace_TransitionsToPages(t *testing.T) {
	m := sizedModel()
	m = m.SetSpaces(testSpaces())

	// Enter on first space (global "Engineering").
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if !m.InPagesState() {
		t.Error("InPagesState() should be true after opening a space")
	}
	if m.CurrentSpaceID() != "1" {
		t.Errorf("CurrentSpaceID() = %q, want %q", m.CurrentSpaceID(), "1")
	}
}

func TestOpenSpace_SetsFetchSentinel(t *testing.T) {
	m := sizedModel()
	m = m.SetSpaces(testSpaces())

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	id := m.NeedsFetch()
	if id != "1" {
		t.Errorf("NeedsFetch() = %q, want %q", id, "1")
	}
	// Sentinel should clear after read.
	if m.NeedsFetch() != "" {
		t.Error("NeedsFetch() should be empty after second read")
	}
}

// --- Open page ---

func TestOpenPage_SetsSelectedPage(t *testing.T) {
	m := sizedModel()
	m.state = statePages
	m.spaceKey = "ENG"
	m = m.SetPages(testPages())

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	sel := m.SelectedPage()
	if sel == nil {
		t.Fatal("SelectedPage() should not be nil after opening a page")
	}
	if sel.ID != "101" {
		t.Errorf("SelectedPage().ID = %q, want %q", sel.ID, "101")
	}
	if sel.Title != "Getting Started" {
		t.Errorf("SelectedPage().Title = %q, want %q", sel.Title, "Getting Started")
	}
	if sel.SpaceID != "1" {
		t.Errorf("SelectedPage().SpaceID = %q, want %q", sel.SpaceID, "1")
	}
}

func TestSelectedPage_SentinelResets(t *testing.T) {
	m := sizedModel()
	m.state = statePages
	m.spaceKey = "ENG"
	m = m.SetPages(testPages())

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	_ = m.SelectedPage()
	if m.SelectedPage() != nil {
		t.Error("SelectedPage() should be nil on second read")
	}
}

// --- Open recent ---

func TestOpenRecent_SetsSelectedPage(t *testing.T) {
	m := sizedModel()
	m = m.SetRecents(testRecents())
	m = m.SetSpaces(testSpaces())

	// Rebuild puts recents first, so cursor is on the first recent item.
	// We need to re-set recents after spaces so the list is rebuilt with recents on top.
	m = m.SetRecents(testRecents())

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	sel := m.SelectedPage()
	if sel == nil {
		t.Fatal("SelectedPage() should not be nil after opening a recent page")
	}
	if sel.ID != "201" {
		t.Errorf("SelectedPage().ID = %q, want %q", sel.ID, "201")
	}
	if sel.Title != "Recent Page" {
		t.Errorf("SelectedPage().Title = %q, want %q", sel.Title, "Recent Page")
	}
}

// --- Back navigation ---

func TestBack_FromPages_ReturnsToSpaces(t *testing.T) {
	m := sizedModel()
	m = m.SetSpaces(testSpaces())

	// Open a space to enter pages state.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.InPagesState() {
		t.Fatal("should be in pages state")
	}

	// Press back.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.InPagesState() {
		t.Error("should have returned to spaces state")
	}
	if m.Dismissed() {
		t.Error("Dismissed() should be false when going back from pages")
	}
}

func TestBack_FromSpaces_SetsDismissed(t *testing.T) {
	m := sizedModel()
	m = m.SetSpaces(testSpaces())

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if !m.Dismissed() {
		t.Error("Dismissed() should be true after back from spaces")
	}
	// Sentinel should clear.
	if m.Dismissed() {
		t.Error("Dismissed() should be false on second read")
	}
}

func TestBack_Backspace_AlsoWorks(t *testing.T) {
	m := sizedModel()
	m = m.SetSpaces(testSpaces())

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})

	if !m.Dismissed() {
		t.Error("Dismissed() should be true after backspace from spaces")
	}
}

// --- GoToSpaces ---

func TestGoToSpaces_TransitionsBack(t *testing.T) {
	m := sizedModel()
	m = m.SetSpaces(testSpaces())

	// Enter pages state.
	m.state = statePages
	m.GoToSpaces()

	if m.InPagesState() {
		t.Error("should be in spaces state after GoToSpaces()")
	}
}

// --- Empty list ---

func TestOpenOnEmptyList_DoesNothing(t *testing.T) {
	m := sizedModel()

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.SelectedPage() != nil {
		t.Error("SelectedPage() should be nil when list is empty")
	}
	if m.InPagesState() {
		t.Error("should not transition to pages when list is empty")
	}
}

// --- Filtering ---

func TestFiltering_InitiallyFalse(t *testing.T) {
	m := New()
	if m.Filtering() {
		t.Error("Filtering() should be false initially")
	}
	if m.Filtered() {
		t.Error("Filtered() should be false initially")
	}
}

// --- Item filter values ---

func TestRecentItem_FilterValue(t *testing.T) {
	item := recentItem{entry: recents.Entry{Title: "My Page", SpaceKey: "ENG"}}
	want := "My Page ENG"
	if got := item.FilterValue(); got != want {
		t.Errorf("FilterValue() = %q, want %q", got, want)
	}
}

func TestSpaceItem_FilterValue(t *testing.T) {
	item := spaceItem{space: confluence.Space{Key: "ENG", Name: "Engineering"}}
	want := "ENG Engineering"
	if got := item.FilterValue(); got != want {
		t.Errorf("FilterValue() = %q, want %q", got, want)
	}
}

func TestPageItem_FilterValue(t *testing.T) {
	item := pageItem{page: confluence.Page{Title: "Architecture"}}
	want := "Architecture"
	if got := item.FilterValue(); got != want {
		t.Errorf("FilterValue() = %q, want %q", got, want)
	}
}

// --- displayKey ---

func TestDisplayKey_GlobalSpace(t *testing.T) {
	s := confluence.Space{Key: "ENG", Name: "Engineering"}
	if got := displayKey(s); got != "ENG" {
		t.Errorf("displayKey() = %q, want %q", got, "ENG")
	}
}

func TestDisplayKey_PersonalSpace(t *testing.T) {
	s := confluence.Space{Key: "~user1", Name: "My Personal"}
	if got := displayKey(s); got != "My Personal" {
		t.Errorf("displayKey() = %q for personal space, want %q", got, "My Personal")
	}
}

// --- truncate ---

func TestTruncate_ShortString(t *testing.T) {
	got := truncate("hello", 10)
	if got != "hello" {
		t.Errorf("truncate() = %q, want %q", got, "hello")
	}
}

func TestTruncate_ExactLength(t *testing.T) {
	got := truncate("hello", 5)
	if got != "hello" {
		t.Errorf("truncate() = %q, want %q", got, "hello")
	}
}

func TestTruncate_LongString(t *testing.T) {
	got := truncate("hello world this is long", 10)
	if got != "hello w..." {
		t.Errorf("truncate() = %q, want %q", got, "hello w...")
	}
}

// --- View ---

func TestView_NonEmpty(t *testing.T) {
	m := sizedModel()
	m = m.SetSpaces(testSpaces())

	view := m.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestView_EmptySpaces(t *testing.T) {
	m := sizedModel()
	m = m.SetSpaces(nil)

	view := m.View()
	if view == "" {
		t.Error("expected non-empty view even with no spaces")
	}
}

// --- Title formatting ---

func TestTitle_WithRecents(t *testing.T) {
	m := sizedModel()
	m = m.SetSpaces(testSpaces())
	m = m.SetRecents(testRecents())

	title := m.list.Title
	if title == "" {
		t.Error("expected non-empty title")
	}
	// Should mention both spaces and recent counts.
	want := "Confluence (3 spaces, 2 recent)"
	if title != want {
		t.Errorf("title = %q, want %q", title, want)
	}
}

func TestTitle_WithoutRecents(t *testing.T) {
	m := sizedModel()
	m = m.SetSpaces(testSpaces())

	want := "Confluence Spaces (3)"
	if m.list.Title != want {
		t.Errorf("title = %q, want %q", m.list.Title, want)
	}
}

func TestTitle_PagesView(t *testing.T) {
	m := sizedModel()
	m.spaceKey = "ENG"
	m = m.SetPages(testPages())

	want := "Pages in ENG (2)"
	if m.list.Title != want {
		t.Errorf("title = %q, want %q", m.list.Title, want)
	}
}
