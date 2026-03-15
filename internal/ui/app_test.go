package ui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/seanhalberthal/jiru/internal/client"
	"github.com/seanhalberthal/jiru/internal/config"
	"github.com/seanhalberthal/jiru/internal/jira"
	"github.com/seanhalberthal/jiru/internal/ui/boardview"
)

// --- Stub client ---

type stubClient struct {
	cfg          *config.Config
	meName       string
	meErr        error
	sprint       *jira.Sprint
	sprintErr    error
	sprintIssues []jira.Issue
	sprintIssErr error
	issue        *jira.Issue
	issueErr     error
	boards       []jira.Board
	boardsErr    error
	boardSprints []jira.Sprint
	boardSprtErr error
	searchIssues []jira.Issue
	searchErr    error
	boardIssues  []jira.Issue
	boardIssErr  error
	epicIssues   []jira.Issue
	epicIssErr   error
	parentMap    map[string]client.ParentInfo
	statsOpen    int
	statsInProg  int
	statsDone    int
	statsTotal   int
	statsErr     error
}

func (s *stubClient) Me() (string, error)                 { return s.meName, s.meErr }
func (s *stubClient) Config() *config.Config              { return s.cfg }
func (s *stubClient) ActiveSprint() (*jira.Sprint, error) { return s.sprint, s.sprintErr }
func (s *stubClient) SprintIssues(_ int) ([]jira.Issue, error) {
	return s.sprintIssues, s.sprintIssErr
}
func (s *stubClient) GetIssue(_ string) (*jira.Issue, error) { return s.issue, s.issueErr }
func (s *stubClient) IssueURL(key string) string {
	return fmt.Sprintf("https://test.atlassian.net/browse/%s", key)
}
func (s *stubClient) Boards(_ string) ([]jira.Board, error) { return s.boards, s.boardsErr }
func (s *stubClient) BoardSprints(_ int, _ string) ([]jira.Sprint, error) {
	return s.boardSprints, s.boardSprtErr
}
func (s *stubClient) SearchJQL(_ string, _ uint) ([]jira.Issue, error) {
	return s.searchIssues, s.searchErr
}
func (s *stubClient) SprintIssueStats(_ int) (int, int, int, int, error) {
	return s.statsOpen, s.statsInProg, s.statsDone, s.statsTotal, s.statsErr
}
func (s *stubClient) ResolveParents(_ []jira.Issue) map[string]client.ParentInfo {
	return s.parentMap
}
func (s *stubClient) BoardIssues(_ string, _ ...string) ([]jira.Issue, error) {
	return s.boardIssues, s.boardIssErr
}
func (s *stubClient) EpicIssues(_ string) ([]jira.Issue, error) {
	return s.epicIssues, s.epicIssErr
}
func (s *stubClient) Projects() ([]jira.Project, error) {
	return nil, nil
}
func (s *stubClient) JQLMetadata() (*jira.JQLMetadata, error) {
	return &jira.JQLMetadata{}, nil
}
func (s *stubClient) SearchUsers(_, _ string) ([]client.UserInfo, error) {
	return nil, nil
}
func (s *stubClient) CreateIssue(_ *client.CreateIssueRequest) (*client.CreateIssueResponse, error) {
	return nil, nil
}
func (s *stubClient) IssueTypes(_ string) ([]string, error) {
	return nil, nil
}

func defaultStub() *stubClient {
	return &stubClient{
		cfg:    &config.Config{Domain: "test.atlassian.net", User: "alice", APIToken: "tok", AuthType: "basic"},
		meName: "Alice",
	}
}

// newTestApp creates an App with the given stub and sets a reasonable size.
func newTestApp(c *stubClient, directIssue string) App {
	app := NewApp(c, directIssue, nil, nil, "")
	// Simulate initial WindowSizeMsg so views are sized.
	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return model.(App)
}

// --- Pure function tests ---

func TestCycleParentFilter_EmptyGroups(t *testing.T) {
	got := cycleParentFilter(nil, "")
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestCycleParentFilter_FirstFromEmpty(t *testing.T) {
	groups := []boardview.ParentGroup{
		{Key: "A-1"}, {Key: "A-2"},
	}
	got := cycleParentFilter(groups, "")
	if got != "A-1" {
		t.Errorf("expected A-1, got %q", got)
	}
}

func TestCycleParentFilter_Progression(t *testing.T) {
	groups := []boardview.ParentGroup{
		{Key: "A-1"}, {Key: "A-2"}, {Key: "A-3"},
	}
	got := cycleParentFilter(groups, "A-1")
	if got != "A-2" {
		t.Errorf("expected A-2, got %q", got)
	}
	got = cycleParentFilter(groups, "A-2")
	if got != "A-3" {
		t.Errorf("expected A-3, got %q", got)
	}
}

func TestCycleParentFilter_WrapAround(t *testing.T) {
	groups := []boardview.ParentGroup{
		{Key: "A-1"}, {Key: "A-2"},
	}
	got := cycleParentFilter(groups, "A-2")
	if got != "" {
		t.Errorf("expected empty (wrap around), got %q", got)
	}
}

func TestCycleParentFilter_UnknownCurrent(t *testing.T) {
	groups := []boardview.ParentGroup{
		{Key: "A-1"},
	}
	got := cycleParentFilter(groups, "UNKNOWN-99")
	if got != "" {
		t.Errorf("expected empty for unknown current, got %q", got)
	}
}

func TestIsHTTPS(t *testing.T) {
	tests := []struct {
		url    string
		reject bool
	}{
		{"https://jira.example.com/browse/PROJ-1", false},
		{"http://jira.example.com/browse/PROJ-1", true},
		{"file:///etc/passwd", true},
		{"javascript:alert(1)", true},
		{"", true},
	}

	for _, tt := range tests {
		shouldReject := !isHTTPS(tt.url)
		if shouldReject != tt.reject {
			t.Errorf("isHTTPS(%q) = %v, want reject=%v", tt.url, !shouldReject, tt.reject)
		}
	}
}

func TestIsHTTPS_PartialPrefix(t *testing.T) {
	if isHTTPS("https:") {
		t.Error("expected false for partial https prefix")
	}
	if isHTTPS("https:/") {
		t.Error("expected false for partial https prefix")
	}
	if !isHTTPS("https://valid.example.com") {
		t.Error("expected true for valid https URL")
	}
}

// --- App state transition tests ---

func TestApp_NewApp_StartsInLoading(t *testing.T) {
	c := defaultStub()
	app := NewApp(c, "", nil, nil, "")
	if app.active != viewLoading {
		t.Errorf("expected viewLoading, got %d", app.active)
	}
}

func TestApp_WindowSizeMsg_UpdatesDimensions(t *testing.T) {
	c := defaultStub()
	app := NewApp(c, "", nil, nil, "")

	model, cmd := app.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	a := model.(App)

	if a.width != 100 || a.height != 50 {
		t.Errorf("expected 100x50, got %dx%d", a.width, a.height)
	}
	if cmd != nil {
		t.Error("expected nil cmd from WindowSizeMsg")
	}
}

func TestApp_QuitKey_ReturnsQuit(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")

	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("expected non-nil cmd for quit")
	}

	// Execute the cmd and check it returns a quit message.
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestApp_ClientReadyMsg_NoBoardID_FetchesBoards(t *testing.T) {
	c := defaultStub()
	// BoardID = 0, no directIssue → should transition toward fetching boards.
	app := newTestApp(c, "")

	model, cmd := app.Update(ClientReadyMsg{Client: c, DisplayName: "Alice"})
	a := model.(App)

	if a.statusMsg != "Authenticated as Alice" {
		t.Errorf("unexpected status: %q", a.statusMsg)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd (fetchBoards)")
	}
}

func TestApp_ClientReadyMsg_WithBoardID_FetchesSprint(t *testing.T) {
	c := defaultStub()
	c.cfg.BoardID = 42
	app := newTestApp(c, "")

	model, cmd := app.Update(ClientReadyMsg{Client: c, DisplayName: "Bob"})
	a := model.(App)

	if a.boardID != 42 {
		t.Errorf("expected boardID 42, got %d", a.boardID)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd (fetchActiveSprint)")
	}
}

func TestApp_ClientReadyMsg_DirectIssue_FetchesDetail(t *testing.T) {
	c := defaultStub()
	c.issue = &jira.Issue{Key: "PROJ-1", Summary: "Direct"}
	app := newTestApp(c, "PROJ-1")

	_, cmd := app.Update(ClientReadyMsg{Client: c, DisplayName: "Alice"})
	if cmd == nil {
		t.Fatal("expected non-nil cmd (fetchIssueDetail)")
	}

	// Execute the command — should return IssueDetailMsg.
	msg := cmd()
	detail, ok := msg.(IssueDetailMsg)
	if !ok {
		t.Fatalf("expected IssueDetailMsg, got %T", msg)
	}
	if detail.Issue.Key != "PROJ-1" {
		t.Errorf("expected PROJ-1, got %s", detail.Issue.Key)
	}
}

func TestApp_IssuesLoadedMsg_TransitionsToSprint(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")

	issues := []jira.Issue{
		{Key: "PROJ-1", Summary: "Task 1", Status: "To Do"},
		{Key: "PROJ-2", Summary: "Task 2", Status: "Done"},
	}

	model, _ := app.Update(IssuesLoadedMsg{Issues: issues, Title: "Sprint 5"})
	a := model.(App)

	if a.active != viewSprint {
		t.Errorf("expected viewSprint, got %d", a.active)
	}
	if len(a.currentIssues) != 2 {
		t.Errorf("expected 2 cached issues, got %d", len(a.currentIssues))
	}
	if a.boardTitle != "Sprint 5" {
		t.Errorf("expected title 'Sprint 5', got %q", a.boardTitle)
	}
}

func TestApp_SprintLoadedMsg_FetchesIssues(t *testing.T) {
	c := defaultStub()
	c.sprintIssues = []jira.Issue{{Key: "X-1", Summary: "Fetched"}}
	app := newTestApp(c, "")

	sprint := &jira.Sprint{ID: 99, Name: "Sprint 99"}
	model, cmd := app.Update(SprintLoadedMsg{Sprint: sprint})
	a := model.(App)

	if a.statusMsg != "Sprint 99" {
		t.Errorf("expected status 'Sprint 99', got %q", a.statusMsg)
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd (fetchSprintIssues)")
	}

	// Execute the command.
	msg := cmd()
	loaded, ok := msg.(IssuesLoadedMsg)
	if !ok {
		t.Fatalf("expected IssuesLoadedMsg, got %T", msg)
	}
	if loaded.Title != "Sprint 99" {
		t.Errorf("expected title 'Sprint 99', got %q", loaded.Title)
	}
}

func TestApp_IssueSelectedMsg_TransitionsToIssue(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")

	iss := jira.Issue{Key: "PROJ-5", Summary: "Selected", Status: "To Do"}
	model, _ := app.Update(IssueSelectedMsg{Issue: iss})
	a := model.(App)

	if a.active != viewIssue {
		t.Errorf("expected viewIssue, got %d", a.active)
	}
}

func TestApp_IssueDetailMsg_UpdatesIssueView(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")

	// First move to issue view.
	model, _ := app.Update(IssueSelectedMsg{Issue: jira.Issue{Key: "PROJ-1", Summary: "Init"}})
	a := model.(App)

	// Then send detail.
	detail := jira.Issue{Key: "PROJ-1", Summary: "Full details", Description: "Full desc"}
	model, _ = a.Update(IssueDetailMsg{Issue: &detail})
	a = model.(App)

	if a.active != viewIssue {
		t.Errorf("expected viewIssue, got %d", a.active)
	}
}

func TestApp_IssueDetailMsg_IgnoredWhenNotInIssueView(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	// App is in viewLoading, not viewIssue.

	detail := jira.Issue{Key: "PROJ-1", Summary: "Late arrival"}
	model, _ := app.Update(IssueDetailMsg{Issue: &detail})
	a := model.(App)

	// Should not have changed the view.
	if a.active != viewLoading {
		t.Errorf("expected viewLoading unchanged, got %d", a.active)
	}
}

func TestApp_BoardsLoadedMsg_TransitionsToHome(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")

	boards := []jira.BoardStats{
		{Board: jira.Board{ID: 1, Name: "Alpha"}},
	}
	model, _ := app.Update(BoardsLoadedMsg{Boards: boards})
	a := model.(App)

	if a.active != viewHome {
		t.Errorf("expected viewHome, got %d", a.active)
	}
	if a.statusMsg != "" {
		t.Errorf("expected empty statusMsg, got %q", a.statusMsg)
	}
}

func TestApp_SearchResultsMsg_TransitionsToSearch(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	// Need to be in a non-loading view for search to work.
	model, _ := app.Update(IssuesLoadedMsg{Issues: nil, Title: "Sprint"})
	a := model.(App)

	issues := []jira.Issue{{Key: "PROJ-1", Summary: "Found"}}
	model, _ = a.Update(SearchResultsMsg{Issues: issues, Query: "status = Open"})
	a = model.(App)

	if a.active != viewSearch {
		t.Errorf("expected viewSearch, got %d", a.active)
	}
}

func TestApp_ErrMsg_SetsErrorAndTransitionsToSprint(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")

	model, _ := app.Update(ErrMsg{Err: errors.New("something broke")})
	a := model.(App)

	if a.active != viewSprint {
		t.Errorf("expected viewSprint on error, got %d", a.active)
	}
	if a.err == nil || a.err.Error() != "something broke" {
		t.Errorf("expected error 'something broke', got %v", a.err)
	}
}

func TestApp_OpenURLMsg_DoesNotPanic(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")

	// Should not panic even with non-HTTPS URL (guard rejects it).
	model, _ := app.Update(OpenURLMsg{URL: "file:///etc/passwd"})
	_ = model.(App) // just verify no panic
}

// --- Key navigation tests ---

func TestApp_SearchKey_FromSprint(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")

	// Move to sprint view first.
	model, _ := app.Update(IssuesLoadedMsg{Issues: nil, Title: "Sprint"})
	a := model.(App)

	// Press '?' to open search.
	model, cmd := a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	a = model.(App)

	if a.active != viewSearch {
		t.Errorf("expected viewSearch, got %d", a.active)
	}
	if a.previousView != viewSprint {
		t.Errorf("expected previousView viewSprint, got %d", a.previousView)
	}
	if cmd == nil {
		t.Error("expected blink cmd")
	}
}

func TestApp_SearchKey_IgnoredDuringLoading(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")

	// Still in viewLoading — search key should be ignored.
	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	a := model.(App)

	if a.active != viewLoading {
		t.Errorf("expected viewLoading (search ignored), got %d", a.active)
	}
}

func TestApp_HomeKey_FromSprint(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")

	// Move to sprint view.
	model, _ := app.Update(IssuesLoadedMsg{Issues: nil, Title: "Sprint"})
	a := model.(App)

	// Press 'H' for home.
	model, _ = a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("H")})
	a = model.(App)

	if a.active != viewHome {
		t.Errorf("expected viewHome, got %d", a.active)
	}
}

func TestApp_BoardToggle_FromSprint(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")

	issues := []jira.Issue{
		{Key: "PROJ-1", Summary: "Task", Status: "To Do"},
	}
	model, _ := app.Update(IssuesLoadedMsg{Issues: issues, Title: "Sprint"})
	a := model.(App)

	// Press 'b' to switch to board view.
	model, _ = a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	a = model.(App)

	if a.active != viewBoard {
		t.Errorf("expected viewBoard, got %d", a.active)
	}

	// Press 'b' again to switch back.
	model, _ = a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	a = model.(App)

	if a.active != viewSprint {
		t.Errorf("expected viewSprint after toggle back, got %d", a.active)
	}
}

func TestApp_BackKey_FromIssue_ToPreviousView(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")

	// Move to sprint, then issue.
	model, _ := app.Update(IssuesLoadedMsg{Issues: nil, Title: "Sprint"})
	a := model.(App)
	a.previousView = viewSprint
	model, _ = a.Update(IssueSelectedMsg{Issue: jira.Issue{Key: "PROJ-1"}})
	a = model.(App)

	// Press esc (back).
	model, _ = a.Update(tea.KeyMsg{Type: tea.KeyEsc})
	a = model.(App)

	if a.active != viewSprint {
		t.Errorf("expected back to viewSprint, got %d", a.active)
	}
}

func TestApp_BackKey_FromIssue_ToBoard(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")

	// Set up: in issue view with previousView = board.
	app.active = viewIssue
	app.previousView = viewBoard

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	a := model.(App)

	if a.active != viewBoard {
		t.Errorf("expected back to viewBoard, got %d", a.active)
	}
}

func TestApp_BackKey_FromSprint_ToHome_WhenNoBoardID(t *testing.T) {
	c := defaultStub()
	// BoardID = 0, so back from sprint should go to home.
	app := newTestApp(c, "")
	app.active = viewSprint

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	a := model.(App)

	if a.active != viewHome {
		t.Errorf("expected viewHome, got %d", a.active)
	}
}

func TestApp_BackKey_FromSprint_QuitsWhenBoardIDSet(t *testing.T) {
	c := defaultStub()
	c.cfg.BoardID = 42
	app := newTestApp(c, "")
	app.active = viewSprint

	// Sprint is the top-level view when boardID is set — back should quit.
	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected non-nil cmd (quit)")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestApp_QKey_FromIssue_GoesBack(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewIssue
	app.previousView = viewSprint

	// q from issue should go back, not quit.
	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	a := model.(App)

	if a.active != viewSprint {
		t.Errorf("expected viewSprint, got %d", a.active)
	}
}

func TestApp_QKey_FromBoard_GoesBackToSprint(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewBoard

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	a := model.(App)

	if a.active != viewSprint {
		t.Errorf("expected viewSprint, got %d", a.active)
	}
}

func TestApp_EscKey_FromHome_Quits(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewHome

	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected non-nil cmd (quit)")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestApp_QKey_FromSprint_NoBoardID_GoesHome(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewSprint

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	a := model.(App)

	if a.active != viewHome {
		t.Errorf("expected viewHome, got %d", a.active)
	}
}

func TestApp_CtrlC_AlwaysQuits(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewIssue

	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected non-nil cmd (quit)")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestApp_RefreshKey_FromSprint(t *testing.T) {
	c := defaultStub()
	c.sprint = &jira.Sprint{ID: 1, Name: "Sprint 1"}
	app := newTestApp(c, "")
	app.active = viewSprint

	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	a := model.(App)

	if a.active != viewLoading {
		t.Errorf("expected viewLoading on refresh, got %d", a.active)
	}
	if a.statusMsg != "Refreshing..." {
		t.Errorf("expected 'Refreshing...', got %q", a.statusMsg)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd on refresh")
	}
}

func TestApp_EscFromBoardView_GoesToSprint(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewBoard

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	a := model.(App)

	if a.active != viewSprint {
		t.Errorf("expected viewSprint on esc from board, got %d", a.active)
	}
}

// --- View rendering tests ---

func TestApp_View_Loading(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewLoading

	v := app.View()
	if !strings.Contains(v, "Connecting to Jira...") {
		t.Error("expected loading message in view")
	}
}

func TestApp_View_LoadingWithStatus(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewLoading
	app.statusMsg = "Loading board..."

	v := app.View()
	if !strings.Contains(v, "Loading board...") {
		t.Error("expected custom status in loading view")
	}
}

func TestApp_View_ZeroWidth(t *testing.T) {
	c := defaultStub()
	app := NewApp(c, "", nil, nil, "")

	v := app.View()
	if v != "Loading..." {
		t.Errorf("expected 'Loading...' for zero width, got %q", v)
	}
}

func TestApp_View_Error(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.err = errors.New("test error")

	v := app.View()
	if !strings.Contains(v, "Error") {
		t.Error("expected 'Error' in view")
	}
	if !strings.Contains(v, "test error") {
		t.Error("expected error message in view")
	}
}

func TestApp_View_Sprint(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewSprint

	v := app.View()
	if v == "" {
		t.Error("expected non-empty sprint view")
	}
}

func TestApp_View_Home(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewHome

	v := app.View()
	if v == "" {
		t.Error("expected non-empty home view")
	}
}

func TestApp_View_Issue(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")

	// Set an issue so the view has content.
	model, _ := app.Update(IssueSelectedMsg{Issue: jira.Issue{Key: "PROJ-1", Summary: "Test", Status: "To Do"}})
	a := model.(App)

	v := a.View()
	if !strings.Contains(v, "PROJ-1") {
		t.Error("expected issue key in view")
	}
}

func TestApp_View_Board(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewBoard

	v := app.View()
	if v == "" {
		t.Error("expected non-empty board view")
	}
}

// --- Command execution tests (verify stub wiring) ---

func TestApp_VerifyAuth_Success(t *testing.T) {
	c := defaultStub()
	app := NewApp(c, "", nil, nil, "")

	cmd := app.verifyAuth()
	msg := cmd()

	ready, ok := msg.(ClientReadyMsg)
	if !ok {
		t.Fatalf("expected ClientReadyMsg, got %T", msg)
	}
	if ready.DisplayName != "Alice" {
		t.Errorf("expected 'Alice', got %q", ready.DisplayName)
	}
}

func TestApp_VerifyAuth_Error(t *testing.T) {
	c := defaultStub()
	c.meErr = errors.New("auth failed")
	app := NewApp(c, "", nil, nil, "")

	cmd := app.verifyAuth()
	msg := cmd()

	errMsg, ok := msg.(ErrMsg)
	if !ok {
		t.Fatalf("expected ErrMsg, got %T", msg)
	}
	if errMsg.Err.Error() != "auth failed" {
		t.Errorf("expected 'auth failed', got %q", errMsg.Err.Error())
	}
}

func TestApp_FetchActiveSprint_Success(t *testing.T) {
	c := defaultStub()
	c.sprint = &jira.Sprint{ID: 42, Name: "Sprint 42"}
	app := NewApp(c, "", nil, nil, "")

	cmd := app.fetchActiveSprint()
	msg := cmd()

	loaded, ok := msg.(SprintLoadedMsg)
	if !ok {
		t.Fatalf("expected SprintLoadedMsg, got %T", msg)
	}
	if loaded.Sprint.ID != 42 {
		t.Errorf("expected sprint ID 42, got %d", loaded.Sprint.ID)
	}
}

func TestApp_FetchActiveSprint_Error(t *testing.T) {
	c := defaultStub()
	c.sprintErr = errors.New("no sprint")
	app := NewApp(c, "", nil, nil, "")

	cmd := app.fetchActiveSprint()
	msg := cmd()

	if _, ok := msg.(ErrMsg); !ok {
		t.Fatalf("expected ErrMsg, got %T", msg)
	}
}

func TestApp_FetchIssueDetail_Success(t *testing.T) {
	c := defaultStub()
	c.issue = &jira.Issue{Key: "PROJ-1", Summary: "Detail", Description: "Full"}
	app := NewApp(c, "", nil, nil, "")

	cmd := app.fetchIssueDetail("PROJ-1")
	msg := cmd()

	detail, ok := msg.(IssueDetailMsg)
	if !ok {
		t.Fatalf("expected IssueDetailMsg, got %T", msg)
	}
	if detail.Issue.Key != "PROJ-1" {
		t.Errorf("expected PROJ-1, got %s", detail.Issue.Key)
	}
}

func TestApp_FetchIssueDetail_Error(t *testing.T) {
	c := defaultStub()
	c.issueErr = errors.New("not found")
	app := NewApp(c, "", nil, nil, "")

	cmd := app.fetchIssueDetail("PROJ-1")
	msg := cmd()

	if _, ok := msg.(ErrMsg); !ok {
		t.Fatalf("expected ErrMsg, got %T", msg)
	}
}

func TestApp_FetchBoards_Success(t *testing.T) {
	c := defaultStub()
	c.boards = []jira.Board{{ID: 1, Name: "Board 1"}}
	c.boardSprints = []jira.Sprint{{ID: 10, Name: "Sprint 10"}}
	c.statsOpen = 3
	c.statsInProg = 2
	c.statsDone = 1
	c.statsTotal = 6
	app := NewApp(c, "", nil, nil, "")

	cmd := app.fetchBoards()
	msg := cmd()

	loaded, ok := msg.(BoardsLoadedMsg)
	if !ok {
		t.Fatalf("expected BoardsLoadedMsg, got %T", msg)
	}
	if len(loaded.Boards) != 1 {
		t.Fatalf("expected 1 board, got %d", len(loaded.Boards))
	}
	if loaded.Boards[0].TotalIssues != 6 {
		t.Errorf("expected 6 total issues, got %d", loaded.Boards[0].TotalIssues)
	}
}

func TestApp_FetchBoards_Error(t *testing.T) {
	c := defaultStub()
	c.boardsErr = errors.New("boards failed")
	app := NewApp(c, "", nil, nil, "")

	cmd := app.fetchBoards()
	msg := cmd()

	if _, ok := msg.(ErrMsg); !ok {
		t.Fatalf("expected ErrMsg, got %T", msg)
	}
}

func TestApp_SearchJQL_Success(t *testing.T) {
	c := defaultStub()
	c.searchIssues = []jira.Issue{{Key: "PROJ-1", Summary: "Found"}}
	app := NewApp(c, "", nil, nil, "")

	cmd := app.searchJQL("status = Open")
	msg := cmd()

	results, ok := msg.(SearchResultsMsg)
	if !ok {
		t.Fatalf("expected SearchResultsMsg, got %T", msg)
	}
	if results.Query != "status = Open" {
		t.Errorf("expected query 'status = Open', got %q", results.Query)
	}
	if len(results.Issues) != 1 {
		t.Errorf("expected 1 issue, got %d", len(results.Issues))
	}
}

func TestApp_SearchJQL_Error(t *testing.T) {
	c := defaultStub()
	c.searchErr = errors.New("bad jql")
	app := NewApp(c, "", nil, nil, "")

	cmd := app.searchJQL("invalid query")
	msg := cmd()

	if _, ok := msg.(ErrMsg); !ok {
		t.Fatalf("expected ErrMsg, got %T", msg)
	}
}

func TestApp_FetchSprintIssues_Success(t *testing.T) {
	c := defaultStub()
	c.sprintIssues = []jira.Issue{
		{Key: "PROJ-1", Summary: "A", ParentKey: "PROJ-100"},
	}
	c.parentMap = map[string]client.ParentInfo{
		"PROJ-100": {Key: "PROJ-100", Summary: "Epic", IssueType: "Epic"},
	}
	app := NewApp(c, "", nil, nil, "")

	cmd := app.fetchSprintIssues(99, "Sprint 99")
	msg := cmd()

	loaded, ok := msg.(IssuesLoadedMsg)
	if !ok {
		t.Fatalf("expected IssuesLoadedMsg, got %T", msg)
	}
	if loaded.Title != "Sprint 99" {
		t.Errorf("expected 'Sprint 99', got %q", loaded.Title)
	}
	if len(loaded.Issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(loaded.Issues))
	}
	// Verify parent enrichment was applied.
	if loaded.Issues[0].ParentSummary != "Epic" {
		t.Errorf("expected ParentSummary 'Epic', got %q", loaded.Issues[0].ParentSummary)
	}
}

func TestApp_FetchSprintIssues_Error(t *testing.T) {
	c := defaultStub()
	c.sprintIssErr = errors.New("sprint issues failed")
	app := NewApp(c, "", nil, nil, "")

	cmd := app.fetchSprintIssues(99, "Sprint 99")
	msg := cmd()

	if _, ok := msg.(ErrMsg); !ok {
		t.Fatalf("expected ErrMsg, got %T", msg)
	}
}

func TestApp_FetchActiveSprintForBoard_WithSprint(t *testing.T) {
	c := defaultStub()
	c.boardSprints = []jira.Sprint{{ID: 10, Name: "Sprint 10"}}
	app := NewApp(c, "", nil, nil, "")

	cmd := app.fetchActiveSprintForBoard(1)
	msg := cmd()

	loaded, ok := msg.(SprintLoadedMsg)
	if !ok {
		t.Fatalf("expected SprintLoadedMsg, got %T", msg)
	}
	if loaded.Sprint.Name != "Sprint 10" {
		t.Errorf("expected 'Sprint 10', got %q", loaded.Sprint.Name)
	}
}

func TestApp_FetchActiveSprintForBoard_NoSprint_FallsBackToBoardIssues(t *testing.T) {
	c := defaultStub()
	c.boardSprtErr = errors.New("no sprints")
	c.boardIssues = []jira.Issue{{Key: "KAN-1", Summary: "Kanban task"}}
	app := NewApp(c, "", nil, nil, "")

	cmd := app.fetchActiveSprintForBoard(1)
	msg := cmd()

	loaded, ok := msg.(IssuesLoadedMsg)
	if !ok {
		t.Fatalf("expected IssuesLoadedMsg (kanban fallback), got %T", msg)
	}
	if loaded.Title != "Board" {
		t.Errorf("expected title 'Board', got %q", loaded.Title)
	}
}

func TestApp_FetchActiveSprintForBoard_NoSprint_BoardIssuesError(t *testing.T) {
	c := defaultStub()
	c.boardSprtErr = errors.New("no sprints")
	c.boardIssErr = errors.New("board issues failed")
	app := NewApp(c, "", nil, nil, "")

	cmd := app.fetchActiveSprintForBoard(1)
	msg := cmd()

	if _, ok := msg.(ErrMsg); !ok {
		t.Fatalf("expected ErrMsg, got %T", msg)
	}
}

// --- Create view tests ---

func TestApp_CreateKey_FromSprint(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewSprint

	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	a := model.(App)

	if a.active != viewCreate {
		t.Errorf("expected viewCreate, got %d", a.active)
	}
	if a.previousView != viewSprint {
		t.Errorf("expected previousView viewSprint, got %d", a.previousView)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd (create init)")
	}
}

func TestApp_CreateKey_FromHome(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewHome

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	a := model.(App)

	if a.active != viewCreate {
		t.Errorf("expected viewCreate, got %d", a.active)
	}
	if a.previousView != viewHome {
		t.Errorf("expected previousView viewHome, got %d", a.previousView)
	}
}

func TestApp_CreateKey_FromBoard(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewBoard

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	a := model.(App)

	if a.active != viewCreate {
		t.Errorf("expected viewCreate, got %d", a.active)
	}
	if a.previousView != viewBoard {
		t.Errorf("expected previousView viewBoard, got %d", a.previousView)
	}
}

func TestApp_CreateKey_IgnoredFromIssue(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewIssue

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	a := model.(App)

	if a.active != viewIssue {
		t.Errorf("expected viewIssue unchanged, got %d", a.active)
	}
}

func TestApp_CreateKey_IgnoredWithoutClient(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewSprint
	app.client = nil

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	a := model.(App)

	if a.active != viewSprint {
		t.Errorf("expected viewSprint unchanged, got %d", a.active)
	}
}

func TestApp_BackKey_FromCreate_ReturnsToPreviousView(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewCreate
	app.previousView = viewSprint

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	a := model.(App)

	if a.active != viewSprint {
		t.Errorf("expected viewSprint, got %d", a.active)
	}
}

func TestApp_QKey_FromCreate_ReturnsToPreviousView(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewCreate
	app.previousView = viewHome

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	a := model.(App)

	if a.active != viewHome {
		t.Errorf("expected viewHome, got %d", a.active)
	}
}

// --- Footer tests ---

func TestFooterView_Loading(t *testing.T) {
	v := footerView(viewLoading, 120, "")
	if !strings.Contains(v, "quit") {
		t.Error("expected 'quit' in loading footer")
	}
}

func TestFooterView_Sprint(t *testing.T) {
	v := footerView(viewSprint, 120, "")
	if !strings.Contains(v, "board view") {
		t.Error("expected 'board view' in sprint footer")
	}
	if !strings.Contains(v, "refresh") {
		t.Error("expected 'refresh' in sprint footer")
	}
}

func TestFooterView_Board(t *testing.T) {
	extra := footerBinding{"e", "filter Epic"}
	v := footerView(viewBoard, 120, "", extra)
	if !strings.Contains(v, "filter Epic") {
		t.Error("expected 'filter Epic' in board footer")
	}
	if !strings.Contains(v, "list view") {
		t.Error("expected 'list view' in board footer")
	}
}

func TestFooterView_Issue(t *testing.T) {
	v := footerView(viewIssue, 120, "")
	if !strings.Contains(v, "browser") {
		t.Error("expected 'browser' in issue footer")
	}
}

func TestFooterView_Search(t *testing.T) {
	v := footerView(viewSearch, 120, "")
	if !strings.Contains(v, "complete") {
		t.Error("expected 'complete' in search footer")
	}
}

func TestFooterView_Truncation(t *testing.T) {
	v := footerView(viewSprint, 10, "")
	// Should not exceed the specified width.
	if len(v) > 100 { // generous buffer for ANSI codes
		t.Error("footer should be truncated for narrow width")
	}
}
