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
	"github.com/seanhalberthal/jiru/internal/ui/branchview"
	"github.com/seanhalberthal/jiru/internal/ui/commentview"
	"github.com/seanhalberthal/jiru/internal/ui/createview"
	"github.com/seanhalberthal/jiru/internal/ui/issuepickview"
	"github.com/seanhalberthal/jiru/internal/ui/issueview"
	"github.com/seanhalberthal/jiru/internal/ui/transitionview"
)

// findMsgInBatch recursively executes a tea.Cmd tree and returns true
// if the predicate matches any resulting tea.Msg. Handles nested BatchMsg.
func findMsgInBatch(cmd tea.Cmd, match func(tea.Msg) bool) bool {
	if cmd == nil {
		return false
	}
	msg := cmd()
	if match(msg) {
		return true
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			if findMsgInBatch(c, match) {
				return true
			}
		}
	}
	return false
}

// --- Stub client ---

type stubClient struct {
	cfg          *config.Config
	meName       string
	meErr        error
	sprintIssues []jira.Issue
	sprintIssErr error
	sprintTotal  int // When set, SprintIssuesPage reports this as Total (simulates Agile truncation).
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
	transitions  []jira.Transition
	transErr     error
	transIssErr  error
	commentErr   error
}

func (s *stubClient) Me() (string, error)    { return s.meName, s.meErr }
func (s *stubClient) Config() *config.Config { return s.cfg }
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
func (s *stubClient) IssueTypesWithID(_ string) ([]jira.IssueTypeInfo, error) {
	return nil, nil
}
func (s *stubClient) CreateMetaFields(_, _ string) ([]jira.CustomFieldDef, error) {
	return nil, nil
}
func (s *stubClient) Transitions(_ string) ([]jira.Transition, error) {
	return s.transitions, s.transErr
}
func (s *stubClient) TransitionIssue(_, _ string) error {
	return s.transIssErr
}
func (s *stubClient) AddComment(_, _ string) error {
	return s.commentErr
}
func (s *stubClient) ChildIssues(_ string) ([]jira.ChildIssue, error) {
	return nil, nil
}
func (s *stubClient) AssignIssue(_, _ string) error { return nil }
func (s *stubClient) EditIssue(_ string, _ *client.EditIssueRequest) error {
	return nil
}
func (s *stubClient) LinkIssue(_, _, _ string) error                   { return nil }
func (s *stubClient) GetIssueLinkTypes() ([]jira.IssueLinkType, error) { return nil, nil }
func (s *stubClient) DeleteIssue(_ string, _ bool) error               { return nil }
func (s *stubClient) SprintIssuesPage(_ int, from, pageSize int) (*client.PageResult, error) {
	if s.sprintIssErr != nil {
		return nil, s.sprintIssErr
	}
	issues := s.sprintIssues
	if from >= len(issues) {
		total := s.sprintTotal // When set, simulates Agile API reporting more issues than it can return.
		return &client.PageResult{Issues: nil, HasMore: false, From: from, Total: total}, nil
	}
	end := from + pageSize
	if end > len(issues) {
		end = len(issues)
	}
	page := issues[from:end]
	return &client.PageResult{
		Issues:  page,
		HasMore: end < len(issues),
		From:    from,
	}, nil
}
func (s *stubClient) SearchJQLPage(_ string, pageSize int, from int, _ string) (*client.PageResult, error) {
	if s.searchErr != nil {
		return nil, s.searchErr
	}
	issues := s.searchIssues
	if from >= len(issues) {
		return &client.PageResult{Issues: nil, HasMore: false, From: from}, nil
	}
	end := from + pageSize
	if end > len(issues) {
		end = len(issues)
	}
	page := issues[from:end]
	hasMore := end < len(issues)
	token := ""
	if hasMore {
		token = fmt.Sprintf("page-%d", end) // Simulate cursor token.
	}
	return &client.PageResult{
		Issues:    page,
		HasMore:   hasMore,
		From:      from,
		NextToken: token,
	}, nil
}
func (s *stubClient) BoardFilterJQL(_ int) (string, error) { return "", fmt.Errorf("no filter") }
func (s *stubClient) BoardIssuesPage(_ int, from, pageSize int) (*client.PageResult, error) {
	// Reuse searchIssues for board issues fallback tests.
	issues := s.searchIssues
	if s.searchErr != nil {
		return nil, s.searchErr
	}
	if from >= len(issues) {
		return &client.PageResult{Issues: nil, HasMore: false, From: from}, nil
	}
	end := from + pageSize
	if end > len(issues) {
		end = len(issues)
	}
	return &client.PageResult{Issues: issues[from:end], HasMore: end < len(issues), From: from}, nil
}
func (s *stubClient) EpicIssuesPage(_ string, from, pageSize int) (*client.PageResult, error) {
	if s.epicIssErr != nil {
		return nil, s.epicIssErr
	}
	issues := s.epicIssues
	if from >= len(issues) {
		return &client.PageResult{Issues: nil, HasMore: false, From: from}, nil
	}
	end := from + pageSize
	if end > len(issues) {
		end = len(issues)
	}
	page := issues[from:end]
	return &client.PageResult{
		Issues:  page,
		HasMore: end < len(issues),
		From:    from,
	}, nil
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

	// Execute the batch command — one of the results should be IssueDetailMsg.
	// The batch may be nested (fetchIssueBundle wraps multiple cmds), so search recursively.
	if !findMsgInBatch(cmd, func(m tea.Msg) bool {
		if detail, ok := m.(IssueDetailMsg); ok {
			if detail.Issue.Key != "PROJ-1" {
				t.Errorf("expected PROJ-1, got %s", detail.Issue.Key)
			}
			return true
		}
		return false
	}) {
		t.Fatal("expected IssueDetailMsg in batch")
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

func TestApp_ErrMsg_SetsErrorWithoutChangingView(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")

	// Start at viewLoading (default for newTestApp).
	model, _ := app.Update(ErrMsg{Err: errors.New("something broke")})
	a := model.(App)

	if a.active != viewLoading {
		t.Errorf("expected view unchanged (viewLoading), got %d", a.active)
	}
	if a.err == nil || a.err.Error() != "something broke" {
		t.Errorf("expected error 'something broke', got %v", a.err)
	}
}

func TestApp_ErrMsg_PreservesActiveView(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewSearch

	model, _ := app.Update(ErrMsg{Err: errors.New("search failed")})
	a := model.(App)

	if a.active != viewSearch {
		t.Errorf("expected viewSearch preserved on error, got %d", a.active)
	}
}

func TestApp_ErrorDismissal_ClearsErrorAndReturnsToView(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewSprint
	app.err = errors.New("some error")

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	a := model.(App)

	if a.err != nil {
		t.Errorf("expected error cleared after esc, got %v", a.err)
	}
	if a.active != viewSprint {
		t.Errorf("expected to stay at viewSprint after dismiss, got %d", a.active)
	}
}

func TestApp_ErrorDismissal_FromLoading_NavigatesBack(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewLoading
	app.previousView = viewHome
	app.err = errors.New("load failed")

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	a := model.(App)

	if a.err != nil {
		t.Errorf("expected error cleared, got %v", a.err)
	}
	if a.active != viewHome {
		t.Errorf("expected navigateBack to viewHome, got %d", a.active)
	}
}

func TestApp_ErrorDismissal_FromLoading_InitialLoad_Quits(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewLoading
	app.previousView = viewSetup // No meaningful previous view.
	app.err = errors.New("auth failed")

	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if cmd == nil {
		t.Fatal("expected non-nil cmd (tea.Quit)")
	}
}

func TestApp_ErrorOverlay_SwallowsNonBackKeys(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewSprint
	app.err = errors.New("some error")

	// Press a regular key — should be swallowed.
	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	a := model.(App)

	if a.err == nil {
		t.Error("expected error to persist after non-back key")
	}
	if a.active != viewSprint {
		t.Errorf("expected viewSprint unchanged, got %d", a.active)
	}
}

func TestApp_RefreshKey_SetsPreviousView(t *testing.T) {
	c := defaultStub()
	c.boardSprints = []jira.Sprint{{ID: 1, Name: "Sprint 1"}}
	app := newTestApp(c, "")
	app.active = viewSprint

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	a := model.(App)

	if a.previousView != viewSprint {
		t.Errorf("expected previousView viewSprint, got %d", a.previousView)
	}
}

func TestApp_NavigateBack_FromLoading_WithPreviousHome(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewLoading
	app.previousView = viewHome

	model, cmd := app.navigateBack()
	a := model.(App)

	if a.active != viewHome {
		t.Errorf("expected viewHome, got %d", a.active)
	}
	if cmd != nil {
		t.Error("expected nil cmd when navigating back to home")
	}
}

func TestApp_NavigateBack_FromLoading_WithPreviousSprint(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewLoading
	app.previousView = viewSprint

	model, cmd := app.navigateBack()
	a := model.(App)

	if a.active != viewSprint {
		t.Errorf("expected viewSprint, got %d", a.active)
	}
	if cmd != nil {
		t.Error("expected nil cmd when navigating back to sprint")
	}
}

func TestApp_NavigateBack_FromLoading_InitialLoad_Quits(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewLoading
	app.previousView = viewSetup

	_, cmd := app.navigateBack()

	if cmd == nil {
		t.Fatal("expected non-nil cmd (tea.Quit)")
	}
}

func TestApp_SuccessfulResult_ClearsStaleError(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.err = errors.New("stale error")

	model, _ := app.Update(IssuesLoadedMsg{Issues: nil, Title: "Sprint"})
	a := model.(App)

	if a.err != nil {
		t.Errorf("expected stale error cleared on IssuesLoadedMsg, got %v", a.err)
	}
}

func TestFooterView_ErrorState(t *testing.T) {
	v := footerView(viewSprint, 120, "", true)
	if !strings.Contains(v, "dismiss") {
		t.Error("expected 'dismiss' in error footer")
	}
	if strings.Contains(v, "board view") {
		t.Error("expected normal bindings suppressed in error footer")
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

	// Sprint is the top-level view when boardID is set — first esc triggers confirm.
	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	a := model.(App)
	if !a.confirmQuit {
		t.Fatal("expected confirmQuit to be set")
	}
	if cmd != nil {
		t.Error("expected nil cmd on confirm prompt, not immediate quit")
	}

	// Second esc confirms quit.
	_, cmd = a.Update(tea.KeyMsg{Type: tea.KeyEsc})
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

	// First esc triggers confirm prompt.
	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	a := model.(App)
	if !a.confirmQuit {
		t.Fatal("expected confirmQuit to be set")
	}
	if cmd != nil {
		t.Error("expected nil cmd on confirm prompt")
	}

	// Second esc confirms quit.
	_, cmd = a.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected non-nil cmd (quit)")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestApp_QuitConfirm_DismissedByOtherKey(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewHome

	// Trigger confirm prompt.
	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	a := model.(App)
	if !a.confirmQuit {
		t.Fatal("expected confirmQuit to be set")
	}

	// Press a different key — should dismiss, not quit.
	model, cmd := a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	a = model.(App)
	if a.confirmQuit {
		t.Error("expected confirmQuit to be cleared")
	}
	if cmd != nil {
		t.Error("expected nil cmd after dismissing confirm")
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

// --- Issue navigation stack tests ---

func TestApp_ParentKey_PushesStackAndFetches(t *testing.T) {
	c := defaultStub()
	c.issue = &jira.Issue{Key: "PROJ-1", Summary: "Parent Issue", Status: "To Do"}
	app := newTestApp(c, "")
	app.active = viewIssue
	app.previousView = viewSprint

	// Set an issue with a parent.
	childIssue := jira.Issue{
		Key:           "PROJ-2",
		Summary:       "Child Issue",
		Status:        "In Progress",
		ParentKey:     "PROJ-1",
		ParentSummary: "Parent Issue",
	}
	app.issue = app.issue.SetIssue(childIssue)

	// Press 'p' to navigate to parent.
	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	a := model.(App)

	if a.active != viewIssue {
		t.Errorf("expected viewIssue, got %d", a.active)
	}
	if len(a.issueStack) != 1 {
		t.Fatalf("expected 1 item on stack, got %d", len(a.issueStack))
	}
	if a.issueStack[0].Key != "PROJ-2" {
		t.Errorf("expected PROJ-2 on stack, got %s", a.issueStack[0].Key)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd (fetch parent detail)")
	}
	// Current issue should be the placeholder for the parent.
	if iss := a.issue.CurrentIssue(); iss == nil || iss.Key != "PROJ-1" {
		t.Error("expected current issue to be PROJ-1 placeholder")
	}
}

func TestApp_ParentKey_NoParent_NoOp(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewIssue
	app.previousView = viewSprint

	// Issue without parent.
	app.issue = app.issue.SetIssue(jira.Issue{Key: "PROJ-1", Summary: "No parent", Status: "To Do"})

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	a := model.(App)

	if len(a.issueStack) != 0 {
		t.Errorf("expected empty stack, got %d", len(a.issueStack))
	}
	if a.active != viewIssue {
		t.Errorf("expected viewIssue, got %d", a.active)
	}
}

func TestApp_BackKey_FromIssue_PopsStack(t *testing.T) {
	c := defaultStub()
	c.issue = &jira.Issue{Key: "PROJ-2", Summary: "Child", Status: "To Do"}
	app := newTestApp(c, "")
	app.active = viewIssue
	app.previousView = viewSprint

	// Set up the stack: child was pushed when navigating to parent.
	childIssue := jira.Issue{Key: "PROJ-2", Summary: "Child Issue", Status: "In Progress"}
	app.issueStack = []jira.Issue{childIssue}
	app.issue = app.issue.SetIssue(jira.Issue{Key: "PROJ-1", Summary: "Parent", Status: "To Do"})

	// Press esc — should pop back to child, not go to sprint.
	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	a := model.(App)

	if a.active != viewIssue {
		t.Errorf("expected viewIssue (popped from stack), got %d", a.active)
	}
	if len(a.issueStack) != 0 {
		t.Errorf("expected empty stack after pop, got %d", len(a.issueStack))
	}
	if iss := a.issue.CurrentIssue(); iss == nil || iss.Key != "PROJ-2" {
		t.Error("expected current issue to be PROJ-2 after pop")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd (re-fetch child detail)")
	}
}

func TestApp_IssuePickKey_OpensPickerOverlay(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewIssue
	app.previousView = viewSprint

	// Issue with parent and children.
	app.issue = app.issue.SetIssue(jira.Issue{
		Key:       "PROJ-5",
		Summary:   "Main",
		Status:    "To Do",
		ParentKey: "PROJ-1",
	})

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})
	a := model.(App)

	if a.active != viewIssuePick {
		t.Errorf("expected viewIssuePick, got %d", a.active)
	}
}

func TestApp_IssuePickKey_NoRefs_NoOp(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewIssue
	app.previousView = viewSprint

	// Issue with no references at all.
	app.issue = app.issue.SetIssue(jira.Issue{Key: "PROJ-5", Summary: "Lonely", Status: "To Do"})

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})
	a := model.(App)

	if a.active != viewIssue {
		t.Errorf("expected viewIssue (no refs), got %d", a.active)
	}
}

func TestApp_IssuePick_EscDismisses(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewIssue
	app.previousView = viewSprint

	app.issue = app.issue.SetIssue(jira.Issue{
		Key:       "PROJ-5",
		Summary:   "Main",
		Status:    "To Do",
		ParentKey: "PROJ-1",
	})

	// Open picker.
	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})
	a := model.(App)

	// Press esc to dismiss.
	model, _ = a.Update(tea.KeyMsg{Type: tea.KeyEsc})
	a = model.(App)

	if a.active != viewIssue {
		t.Errorf("expected viewIssue after dismiss, got %d", a.active)
	}
}

func TestApp_IssueStack_ClearedOnNewIssueFromList(t *testing.T) {
	c := defaultStub()
	c.issue = &jira.Issue{Key: "PROJ-3", Summary: "New", Status: "To Do"}
	app := newTestApp(c, "")
	app.active = viewIssue
	app.previousView = viewSprint
	app.issueStack = []jira.Issue{
		{Key: "PROJ-1", Summary: "Old", Status: "To Do"},
	}

	// Selecting from list should clear the stack.
	model, _ := app.Update(IssueSelectedMsg{Issue: jira.Issue{Key: "PROJ-3", Summary: "New", Status: "To Do"}})
	a := model.(App)

	if len(a.issueStack) != 0 {
		t.Errorf("expected stack cleared on IssueSelectedMsg, got %d", len(a.issueStack))
	}
}

func TestApp_IssuePick_SelectPushesStackAndNavigates(t *testing.T) {
	c := defaultStub()
	c.issue = &jira.Issue{Key: "PROJ-1", Summary: "Parent", Status: "To Do"}
	app := newTestApp(c, "")

	// Set up: viewing PROJ-5 with picker open showing PROJ-1.
	currentIss := jira.Issue{Key: "PROJ-5", Summary: "Current", Status: "To Do", ParentKey: "PROJ-1"}
	app.issue = app.issue.SetIssue(currentIss)
	app.issuePick = issuepickview.New([]issueview.IssueRef{{Key: "PROJ-1", Label: "parent"}})
	app.issuePick.SetSize(120, 40)
	app.active = viewIssuePick

	// Press enter to select PROJ-1 from picker.
	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	a := model.(App)

	if a.active != viewIssue {
		t.Errorf("expected viewIssue after pick, got %d", a.active)
	}
	if len(a.issueStack) != 1 {
		t.Fatalf("expected 1 item on stack, got %d", len(a.issueStack))
	}
	if a.issueStack[0].Key != "PROJ-5" {
		t.Errorf("expected PROJ-5 on stack, got %s", a.issueStack[0].Key)
	}
	if iss := a.issue.CurrentIssue(); iss == nil || iss.Key != "PROJ-1" {
		t.Error("expected current issue to be PROJ-1 placeholder")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd (fetch detail)")
	}
}

func TestApp_IssuePick_GlobalKeysBlocked(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")

	app.issue = app.issue.SetIssue(jira.Issue{
		Key:       "PROJ-5",
		Summary:   "Main",
		Status:    "To Do",
		ParentKey: "PROJ-1",
	})
	app.issuePick = issuepickview.New([]issueview.IssueRef{{Key: "PROJ-1", Label: "parent"}})
	app.issuePick.SetSize(120, 40)
	app.active = viewIssuePick

	// Press 'p' — should be swallowed by the picker, not navigate to parent.
	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	a := model.(App)

	if a.active != viewIssuePick {
		t.Errorf("expected viewIssuePick (key blocked), got %d", a.active)
	}
	if len(a.issueStack) != 0 {
		t.Error("expected empty stack — 'p' should not have triggered parent navigation")
	}
}

func TestApp_IssueStack_MultiLevelPopOrder(t *testing.T) {
	c := defaultStub()
	c.issue = &jira.Issue{Key: "PROJ-1", Summary: "Grandparent", Status: "To Do"}
	app := newTestApp(c, "")
	app.active = viewIssue
	app.previousView = viewSprint

	// Stack: [grandparent, parent], currently viewing grandchild.
	app.issueStack = []jira.Issue{
		{Key: "PROJ-1", Summary: "Grandparent", Status: "To Do"},
		{Key: "PROJ-2", Summary: "Parent", Status: "To Do"},
	}
	app.issue = app.issue.SetIssue(jira.Issue{Key: "PROJ-3", Summary: "Grandchild", Status: "To Do"})

	// Pop once → PROJ-2 (parent).
	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	a := model.(App)

	if iss := a.issue.CurrentIssue(); iss == nil || iss.Key != "PROJ-2" {
		t.Errorf("expected PROJ-2 after first pop, got %v", a.issue.CurrentIssue())
	}
	if len(a.issueStack) != 1 {
		t.Errorf("expected stack depth 1, got %d", len(a.issueStack))
	}

	// Pop again → PROJ-1 (grandparent).
	model, _ = a.Update(tea.KeyMsg{Type: tea.KeyEsc})
	a = model.(App)

	if iss := a.issue.CurrentIssue(); iss == nil || iss.Key != "PROJ-1" {
		t.Errorf("expected PROJ-1 after second pop, got %v", a.issue.CurrentIssue())
	}
	if len(a.issueStack) != 0 {
		t.Errorf("expected empty stack, got %d", len(a.issueStack))
	}

	// Pop with empty stack → should go back to sprint view.
	model, _ = a.Update(tea.KeyMsg{Type: tea.KeyEsc})
	a = model.(App)

	if a.active != viewSprint {
		t.Errorf("expected viewSprint after exhausting stack, got %d", a.active)
	}
}

func TestApp_InputActive_TrueForIssuePick(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewIssuePick
	app.issuePick = issuepickview.New([]issueview.IssueRef{{Key: "PROJ-1", Label: "parent"}})

	if !app.inputActive() {
		t.Error("expected inputActive() true for viewIssuePick")
	}
}

func TestApp_BranchInfoMsg_UpdatesIssueView(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewIssue
	app.issue = app.issue.SetIssue(jira.Issue{Key: "PROJ-1", Summary: "Test", Status: "To Do"})

	model, _ := app.Update(BranchInfoMsg{
		IssueKey: "PROJ-1",
		Branches: []jira.BranchInfo{
			{Name: "origin/feature/PROJ-1-fix", RemoteCommit: 5},
		},
	})
	a := model.(App)

	view := a.issue.View()
	if !strings.Contains(view, "origin/feature/PROJ-1-fix") {
		t.Error("expected branch name in issue view after BranchInfoMsg")
	}
}

func TestApp_BranchInfoMsg_IgnoredWhenDifferentIssue(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewIssue
	app.issue = app.issue.SetIssue(jira.Issue{Key: "PROJ-2", Summary: "Different", Status: "To Do"})

	model, _ := app.Update(BranchInfoMsg{
		IssueKey: "PROJ-1", // Different from current issue.
		Branches: []jira.BranchInfo{
			{Name: "origin/PROJ-1-fix", RemoteCommit: 3},
		},
	})
	a := model.(App)

	view := a.issue.View()
	if strings.Contains(view, "origin/PROJ-1-fix") {
		t.Error("expected BranchInfoMsg to be ignored for different issue")
	}
}

func TestApp_RefreshKey_FromSprint(t *testing.T) {
	c := defaultStub()
	c.boardSprints = []jira.Sprint{{ID: 1, Name: "Sprint 1"}}
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
	// fetchSprintIssues now uses SearchJQLPage (v3 JQL) instead of SprintIssuesPage (Agile v1).
	c.searchIssues = []jira.Issue{
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
	// Verify JQL and NextToken are populated for cursor-based pagination.
	if loaded.JQL == "" {
		t.Error("expected JQL to be set for v3 search-based sprint loading")
	}
}

func TestApp_FetchSprintIssues_Error(t *testing.T) {
	c := defaultStub()
	// fetchSprintIssues now uses SearchJQLPage (v3 JQL) instead of SprintIssuesPage (Agile v1).
	c.searchErr = errors.New("sprint issues failed")
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
	c.cfg.Project = "KAN"
	c.boardSprtErr = errors.New("no sprints")
	c.searchIssues = []jira.Issue{{Key: "KAN-1", Summary: "Kanban task"}}
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
	c.cfg.Project = "KAN"
	c.boardSprtErr = errors.New("no sprints")
	c.searchErr = errors.New("search failed")
	app := NewApp(c, "", nil, nil, "")

	cmd := app.fetchActiveSprintForBoard(1)
	msg := cmd()

	if _, ok := msg.(ErrMsg); !ok {
		t.Fatalf("expected ErrMsg, got %T", msg)
	}
}

// --- Progressive pagination tests ---

func TestApp_IssuesLoadedMsg_WithHasMore_ChainsNextFetch(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.client = c

	model, cmd := app.Update(IssuesLoadedMsg{
		Issues:  []jira.Issue{{Key: "A-1"}},
		Title:   "Sprint 1",
		HasMore: true,
		Source:  "sprint",
		From:    1,
		Seq:     app.paginationSeq,
	})
	a := model.(App)
	if a.active != viewSprint {
		t.Errorf("expected viewSprint, got %d", a.active)
	}
	if cmd == nil {
		t.Fatal("expected a follow-up command for next page")
	}
}

func TestApp_IssuesLoadedMsg_WithoutHasMore_NoChain(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.client = c

	_, cmd := app.Update(IssuesLoadedMsg{
		Issues: []jira.Issue{{Key: "A-1"}},
		Title:  "Sprint 1",
	})
	if cmd != nil {
		t.Error("expected no follow-up command when HasMore is false")
	}
}

func TestApp_IssuesPageMsg_AppendsToViews(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.client = c
	app.active = viewSprint
	app.currentIssues = []jira.Issue{{Key: "A-1"}}

	model, cmd := app.Update(IssuesPageMsg{
		Issues:  []jira.Issue{{Key: "A-2"}},
		HasMore: false,
		Source:  "sprint",
		From:    2,
		Seq:     app.paginationSeq,
	})
	a := model.(App)
	if len(a.currentIssues) != 2 {
		t.Errorf("expected 2 issues, got %d", len(a.currentIssues))
	}
	if cmd != nil {
		t.Error("expected no follow-up command when HasMore is false")
	}
}

func TestApp_IssuesPageMsg_StaleSeq_Discarded(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.client = c
	app.paginationSeq = 5
	app.currentIssues = []jira.Issue{{Key: "A-1"}}

	model, cmd := app.Update(IssuesPageMsg{
		Issues:  []jira.Issue{{Key: "A-2"}},
		HasMore: true,
		Source:  "sprint",
		From:    2,
		Seq:     3, // Stale — doesn't match current paginationSeq.
	})
	a := model.(App)
	// Should not have appended the stale page.
	if len(a.currentIssues) != 1 {
		t.Errorf("expected 1 issue (stale page discarded), got %d", len(a.currentIssues))
	}
	if cmd != nil {
		t.Error("expected no follow-up command for stale page")
	}
}

func TestApp_IssuesPageMsg_SearchSource_AppendsToSearch(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.client = c
	app.active = viewSearch
	app.search.Show()
	app.search.SetResults([]jira.Issue{{Key: "S-1"}}, "status = Open")

	model, _ := app.Update(IssuesPageMsg{
		Issues:  []jira.Issue{{Key: "S-2"}},
		HasMore: false,
		Source:  "search",
		From:    2,
		Seq:     app.paginationSeq,
	})
	_ = model.(App)
	// No crash — search results were appended.
}

func TestApp_SearchJQL_ReturnsSearchResultsMsg(t *testing.T) {
	c := defaultStub()
	c.searchIssues = []jira.Issue{{Key: "S-1"}, {Key: "S-2"}}
	app := newTestApp(c, "")
	app.client = c

	cmd := app.searchJQL("status = Open")
	msg := cmd()

	result, ok := msg.(SearchResultsMsg)
	if !ok {
		t.Fatalf("expected SearchResultsMsg, got %T", msg)
	}
	if len(result.Issues) != 2 {
		t.Errorf("expected 2 issues, got %d", len(result.Issues))
	}
}

func TestApp_FetchSprintIssues_Progressive(t *testing.T) {
	c := defaultStub()
	// fetchSprintIssues now uses SearchJQLPage (v3 JQL) instead of SprintIssuesPage.
	c.searchIssues = []jira.Issue{{Key: "SP-1"}}
	app := newTestApp(c, "")
	app.client = c

	cmd := app.fetchSprintIssues(1, "Sprint 1")
	msg := cmd()

	loaded, ok := msg.(IssuesLoadedMsg)
	if !ok {
		t.Fatalf("expected IssuesLoadedMsg, got %T", msg)
	}
	if loaded.Source != "sprint" {
		t.Errorf("expected source 'sprint', got %q", loaded.Source)
	}
	if loaded.Title != "Sprint 1" {
		t.Errorf("expected title 'Sprint 1', got %q", loaded.Title)
	}
}

func TestApp_FetchMoreIssues_SprintUsesJQL(t *testing.T) {
	c := defaultStub()
	// Sprint loading now uses v3 JQL search (not Agile v1), so set up searchIssues.
	c.searchIssues = []jira.Issue{
		{Key: "A-1"}, {Key: "A-2"}, {Key: "A-3"}, {Key: "A-4"}, {Key: "A-5"},
	}

	app := newTestApp(c, "")
	app.client = c

	// Simulate: first page loaded 3 issues, now fetching more via JQL.
	cmd := app.fetchMoreIssues(IssuesPageMsg{
		Source:   "sprint",
		From:     3,
		SprintID: 1,
		JQL:      "sprint = 1 ORDER BY updated DESC",
		Seq:      app.paginationSeq,
	})
	msg := cmd()

	page, ok := msg.(IssuesPageMsg)
	if !ok {
		t.Fatalf("expected IssuesPageMsg, got %T", msg)
	}

	// Source should remain "sprint" (routed through SearchJQLPage).
	if page.Source != "sprint" {
		t.Errorf("expected source 'sprint', got %q", page.Source)
	}

	// Should have fetched the remaining issues via JQL.
	if len(page.Issues) == 0 {
		t.Fatal("expected issues from JQL search, got none")
	}

	// JQL should be carried through for subsequent pages.
	if page.JQL == "" {
		t.Error("expected JQL to be set for pagination continuation")
	}
}

func TestApp_SprintIssues_RouteToSprintView(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewSprint
	app.paginationSeq = 1
	app.currentIssues = []jira.Issue{{Key: "A-1"}, {Key: "A-2"}, {Key: "A-3"}}

	// Sprint JQL pages should go to currentIssues (sprint view), NOT the search overlay.
	model, _ := app.Update(IssuesPageMsg{
		Issues:  []jira.Issue{{Key: "A-2"}, {Key: "A-4"}, {Key: "A-5"}},
		HasMore: false,
		Source:  "sprint",
		Seq:     1,
	})
	a := model.(App)

	// A-2 should be deduped, A-4 and A-5 appended.
	if len(a.currentIssues) != 5 {
		t.Errorf("expected 5 issues in currentIssues (deduped), got %d", len(a.currentIssues))
	}

	// Verify the search overlay was not activated.
	if a.search.Visible() {
		t.Error("sprint issues should not activate the search overlay")
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

// --- BranchCreatedMsg tests ---

func TestApp_BranchCreatedMsg_Success_Local(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewBranch

	model, _ := app.Update(BranchCreatedMsg{Name: "feat/test", Mode: "local"})
	a := model.(App)

	if a.active != viewIssue {
		t.Errorf("expected viewIssue after branch creation, got %d", a.active)
	}
	if a.statusMsg == "" {
		t.Error("expected status message after branch creation")
	}
	if !strings.Contains(a.statusMsg, "feat/test") {
		t.Errorf("expected branch name in status, got %q", a.statusMsg)
	}
}

func TestApp_BranchCreatedMsg_Success_Remote(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewBranch

	model, _ := app.Update(BranchCreatedMsg{Name: "feat/test", Mode: "remote"})
	a := model.(App)

	if a.active != viewIssue {
		t.Errorf("expected viewIssue, got %d", a.active)
	}
	if !strings.Contains(a.statusMsg, "Pushed") {
		t.Errorf("expected 'Pushed' in status, got %q", a.statusMsg)
	}
}

func TestApp_BranchCreatedMsg_Success_Both(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewBranch

	model, _ := app.Update(BranchCreatedMsg{Name: "feat/test", Mode: "both"})
	a := model.(App)

	if a.active != viewIssue {
		t.Errorf("expected viewIssue, got %d", a.active)
	}
	if !strings.Contains(a.statusMsg, "pushed") {
		t.Errorf("expected 'pushed' in status, got %q", a.statusMsg)
	}
}

func TestApp_BranchCreatedMsg_Copied(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewBranch

	model, _ := app.Update(BranchCreatedMsg{Name: "feat/test", Copied: true})
	a := model.(App)

	if a.active != viewIssue {
		t.Errorf("expected viewIssue, got %d", a.active)
	}
	if !strings.Contains(a.statusMsg, "Copied") {
		t.Errorf("expected 'Copied' in status, got %q", a.statusMsg)
	}
}

func TestApp_BranchCreatedMsg_Error(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewBranch

	model, _ := app.Update(BranchCreatedMsg{Err: fmt.Errorf("push failed")})
	a := model.(App)

	if a.active != viewIssue {
		t.Errorf("expected viewIssue even on error, got %d", a.active)
	}
	if a.err == nil {
		t.Error("expected error to be set")
	}
}

func TestApp_BranchCreatedMsg_IgnoredWhenNotInBranchView(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewSprint

	model, _ := app.Update(BranchCreatedMsg{Name: "feat/test", Mode: "local"})
	a := model.(App)

	if a.active != viewSprint {
		t.Errorf("expected viewSprint unchanged, got %d", a.active)
	}
}

// --- sanitiseError tests ---

func TestSanitiseError_StripsURLs(t *testing.T) {
	err := fmt.Errorf("failed to call https://my.jira.net/rest/api/2/issue: 403")
	got := sanitiseError(err)
	if strings.Contains(got.Error(), "https://") {
		t.Errorf("expected URL stripped, got %q", got.Error())
	}
	if !strings.Contains(got.Error(), "[url redacted]") {
		t.Errorf("expected [url redacted], got %q", got.Error())
	}
}

func TestSanitiseError_StripsHTTP(t *testing.T) {
	err := fmt.Errorf("failed: http://internal.server:8080/api")
	got := sanitiseError(err)
	if strings.Contains(got.Error(), "http://") {
		t.Errorf("expected http URL stripped, got %q", got.Error())
	}
}

func TestSanitiseError_PreservesNonURLErrors(t *testing.T) {
	err := fmt.Errorf("something broke")
	got := sanitiseError(err)
	if got.Error() != "something broke" {
		t.Errorf("expected unchanged error, got %q", got.Error())
	}
}

func TestSanitiseError_MultipleURLs(t *testing.T) {
	err := fmt.Errorf("tried https://a.com and https://b.com")
	got := sanitiseError(err)
	if strings.Contains(got.Error(), "https://") {
		t.Errorf("expected all URLs stripped, got %q", got.Error())
	}
}

// --- Footer tests ---

func TestFooterView_Loading(t *testing.T) {
	v := footerView(viewLoading, 120, "", false)
	if !strings.Contains(v, "quit") {
		t.Error("expected 'quit' in loading footer")
	}
}

func TestFooterView_Sprint(t *testing.T) {
	v := footerView(viewSprint, 120, "", false)
	if !strings.Contains(v, "board view") {
		t.Error("expected 'board view' in sprint footer")
	}
	if !strings.Contains(v, "refresh") {
		t.Error("expected 'refresh' in sprint footer")
	}
}

func TestFooterView_Board(t *testing.T) {
	v := footerView(viewBoard, 120, "", false)
	if !strings.Contains(v, "list view") {
		t.Error("expected 'list view' in board footer")
	}
}

func TestFooterView_Issue(t *testing.T) {
	v := footerView(viewIssue, 120, "", false)
	if !strings.Contains(v, "browser") {
		t.Error("expected 'browser' in issue footer")
	}
	if !strings.Contains(v, "parent") {
		t.Error("expected 'parent' in issue footer")
	}
	if !strings.Contains(v, "go to issue") {
		t.Error("expected 'go to issue' in issue footer")
	}
}

func TestFooterView_Search(t *testing.T) {
	// Search bindings are now passed dynamically via extra from app.go.
	inputExtras := []footerBinding{
		{"enter", "search"},
		{"↑↓", "browse"},
		{"tab", "accept"},
		{"esc", "close"},
	}
	v := footerView(viewSearch, 120, "", false, inputExtras...)
	if !strings.Contains(v, "accept") {
		t.Error("expected 'accept' in search footer")
	}
}

func TestFooterView_WrapsToMultipleRows(t *testing.T) {
	// Issue view has >10 bindings, so it always wraps (max 10 per row).
	v := footerView(viewIssue, 200, "", false)
	rows := strings.Count(v, "\n") + 1
	if rows < 2 {
		t.Errorf("expected multiple rows for issue view (>10 bindings), got %d", rows)
	}

	// All bindings should still be present — none dropped.
	for _, want := range []string{"parent", "delete", "JQL", "browser", "edit", "assign"} {
		if !strings.Contains(v, want) {
			t.Errorf("expected %q in wrapped footer", want)
		}
	}

	// A view with few bindings should stay on one row.
	loading := footerView(viewLoading, 200, "", false)
	if strings.Contains(loading, "\n") {
		t.Error("expected single row for loading view")
	}
}

func TestFooterView_VersionOnLastRow(t *testing.T) {
	v := footerView(viewLoading, 80, "v1.2.3", false)
	if !strings.Contains(v, "v1.2.3") {
		t.Error("expected version in footer")
	}
	if !strings.Contains(v, "quit") {
		t.Error("expected quit binding in footer")
	}
}

func TestFooterView_Transition(t *testing.T) {
	v := footerView(viewTransition, 120, "", false)
	if !strings.Contains(v, "select") {
		t.Error("expected 'select' in transition footer")
	}
	if !strings.Contains(v, "back") {
		t.Error("expected 'back' in transition footer")
	}
}

func TestFooterView_Comment(t *testing.T) {
	v := footerView(viewComment, 120, "", false)
	if !strings.Contains(v, "submit") {
		t.Error("expected 'submit' in comment footer")
	}
	if !strings.Contains(v, "back") {
		t.Error("expected 'back' in comment footer")
	}
}

func TestFooterView_SearchResults_ShowsSaveFilter(t *testing.T) {
	extras := []footerBinding{
		{"enter", "open"},
		{"s", "save filter"},
		{"/", "filter"},
		{"esc", "back"},
	}
	v := footerView(viewSearch, 120, "", false, extras...)
	if !strings.Contains(v, "save filter") {
		t.Error("expected 'save filter' in search results footer for manual JQL search")
	}
}

func TestFooterView_SearchResults_HidesSaveFilterForSavedFilter(t *testing.T) {
	// When search origin is viewFilters, "save filter" should be omitted.
	extras := []footerBinding{
		{"enter", "open"},
		{"/", "filter"},
		{"esc", "back"},
	}
	v := footerView(viewSearch, 120, "", false, extras...)
	if strings.Contains(v, "save filter") {
		t.Error("should not show 'save filter' when results came from a saved filter")
	}
}

// --- Transition view tests ---

func TestApp_TransitionKey_FromIssue(t *testing.T) {
	c := defaultStub()
	c.transitions = []jira.Transition{{ID: "1", Name: "In Progress"}}
	app := newTestApp(c, "")

	// Move to issue view with an actual issue set.
	model, _ := app.Update(IssueSelectedMsg{Issue: jira.Issue{Key: "PROJ-1", Summary: "Test", Status: "To Do"}})
	a := model.(App)
	a.previousView = viewSprint

	// Press 'm' to open transition view.
	model, cmd := a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("m")})
	a = model.(App)

	if a.active != viewTransition {
		t.Errorf("expected viewTransition, got %d", a.active)
	}
	if a.transitionOrigin != viewIssue {
		t.Errorf("expected transitionOrigin viewIssue, got %d", a.transitionOrigin)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd (fetchTransitions)")
	}
}

func TestApp_TransitionKey_IgnoredFromSprint(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewSprint

	// 'm' from sprint view should not trigger transition.
	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("m")})
	a := model.(App)

	if a.active != viewSprint {
		t.Errorf("expected viewSprint unchanged, got %d", a.active)
	}
}

func TestApp_TransitionKey_IgnoredWhenNoIssue(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	// In issue view but without setting an actual issue.
	app.active = viewIssue

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("m")})
	a := model.(App)

	// Should stay in viewIssue because CurrentIssue() returns nil.
	if a.active != viewIssue {
		t.Errorf("expected viewIssue unchanged, got %d", a.active)
	}
}

func TestApp_CommentKey_FromIssue(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")

	// Move to issue view with an actual issue.
	model, _ := app.Update(IssueSelectedMsg{Issue: jira.Issue{Key: "PROJ-1", Summary: "Test", Status: "To Do"}})
	a := model.(App)
	a.previousView = viewSprint

	// Press 'c' from issue view — should open comment (not create).
	model, _ = a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	a = model.(App)

	if a.active != viewComment {
		t.Errorf("expected viewComment, got %d", a.active)
	}
	// previousView should be preserved (not overwritten by comment overlay).
	if a.previousView != viewSprint {
		t.Errorf("expected previousView viewSprint (preserved), got %d", a.previousView)
	}
}

func TestApp_CommentKey_IgnoredWhenNoIssue(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	// In issue view but without setting an actual issue.
	app.active = viewIssue

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	a := model.(App)

	// Should stay in viewIssue because CurrentIssue() returns nil.
	if a.active != viewIssue {
		t.Errorf("expected viewIssue unchanged, got %d", a.active)
	}
}

func TestApp_TransitionsLoadedMsg_SetsTransitions(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")

	// Set up app in transition view.
	app.transition = transitionview.New("PROJ-1")
	app.transition.SetSize(120, 38)
	app.active = viewTransition

	transitions := []jira.Transition{
		{ID: "1", Name: "In Progress"},
		{ID: "2", Name: "Done"},
	}
	model, _ := app.Update(TransitionsLoadedMsg{Key: "PROJ-1", Transitions: transitions})
	a := model.(App)

	// Should still be in transition view (no panic, no state change).
	if a.active != viewTransition {
		t.Errorf("expected viewTransition, got %d", a.active)
	}
}

func TestApp_TransitionsLoadedMsg_IgnoredWhenNotInTransitionView(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewSprint

	transitions := []jira.Transition{{ID: "1", Name: "In Progress"}}
	model, _ := app.Update(TransitionsLoadedMsg{Key: "PROJ-1", Transitions: transitions})
	a := model.(App)

	if a.active != viewSprint {
		t.Errorf("expected viewSprint unchanged, got %d", a.active)
	}
}

func TestApp_IssueTransitionedMsg_Success(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewTransition
	app.transitionOrigin = viewIssue
	c.issue = &jira.Issue{Key: "PROJ-1", Summary: "Test", Status: "In Progress"}

	model, cmd := app.Update(IssueTransitionedMsg{Key: "PROJ-1", NewStatus: "In Progress"})
	a := model.(App)

	if a.active != viewIssue {
		t.Errorf("expected viewIssue after transition, got %d", a.active)
	}
	if a.statusMsg != "Moved to In Progress" {
		t.Errorf("expected 'Moved to In Progress', got %q", a.statusMsg)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd (fetchIssueDetail to refresh)")
	}
}

func TestApp_IssueTransitionedMsg_Success_FromBoard(t *testing.T) {
	c := defaultStub()
	c.boardSprints = []jira.Sprint{{ID: 10, Name: "Sprint 10"}}
	app := newTestApp(c, "")
	app.active = viewTransition
	app.transitionOrigin = viewBoard
	app.boardID = 42

	model, cmd := app.Update(IssueTransitionedMsg{Key: "PROJ-1", NewStatus: "Done"})
	a := model.(App)

	if a.active != viewBoard {
		t.Errorf("expected viewBoard after transition, got %d", a.active)
	}
	if a.statusMsg != "Moved to Done" {
		t.Errorf("expected 'Moved to Done', got %q", a.statusMsg)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd (refreshCurrentView)")
	}
}

func TestApp_IssueTransitionedMsg_Error(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewTransition
	app.transitionOrigin = viewIssue

	model, _ := app.Update(IssueTransitionedMsg{Key: "PROJ-1", Err: errors.New("transition failed")})
	a := model.(App)

	if a.active != viewIssue {
		t.Errorf("expected viewIssue on error, got %d", a.active)
	}
	if a.err == nil || a.err.Error() != "transition failed" {
		t.Errorf("expected error 'transition failed', got %v", a.err)
	}
}

func TestApp_IssueTransitionedMsg_IgnoredWhenNotInTransitionView(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewSprint

	model, _ := app.Update(IssueTransitionedMsg{Key: "PROJ-1", NewStatus: "Done"})
	a := model.(App)

	if a.active != viewSprint {
		t.Errorf("expected viewSprint unchanged, got %d", a.active)
	}
}

func TestApp_CommentAddedMsg_Success(t *testing.T) {
	c := defaultStub()
	c.issue = &jira.Issue{Key: "PROJ-1", Summary: "Test", Status: "To Do"}
	app := newTestApp(c, "")
	app.active = viewComment
	app.comment = commentview.New("PROJ-1")
	app.comment.SetSize(120, 38)

	model, cmd := app.Update(CommentAddedMsg{Key: "PROJ-1"})
	a := model.(App)

	if a.active != viewIssue {
		t.Errorf("expected viewIssue after comment, got %d", a.active)
	}
	if a.statusMsg != "Comment added" {
		t.Errorf("expected 'Comment added', got %q", a.statusMsg)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd (fetchIssueDetail to refresh)")
	}
}

func TestApp_CommentAddedMsg_Error(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewComment
	app.comment = commentview.New("PROJ-1")
	app.comment.SetSize(120, 38)

	model, _ := app.Update(CommentAddedMsg{Key: "PROJ-1", Err: errors.New("comment failed")})
	a := model.(App)

	if a.active != viewIssue {
		t.Errorf("expected viewIssue on error, got %d", a.active)
	}
	if a.err == nil || a.err.Error() != "comment failed" {
		t.Errorf("expected error 'comment failed', got %v", a.err)
	}
}

func TestApp_CommentAddedMsg_IgnoredWhenNotInCommentView(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewSprint

	model, _ := app.Update(CommentAddedMsg{Key: "PROJ-1"})
	a := model.(App)

	if a.active != viewSprint {
		t.Errorf("expected viewSprint unchanged, got %d", a.active)
	}
}

// --- Back navigation for new views ---

func TestApp_BackKey_FromTransition_ReturnsToPreviousView(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewTransition
	app.transitionOrigin = viewIssue

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	a := model.(App)

	if a.active != viewIssue {
		t.Errorf("expected viewIssue, got %d", a.active)
	}
}

func TestApp_BackKey_FromTransition_ReturnsToBoardView(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewTransition
	app.transitionOrigin = viewBoard

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	a := model.(App)

	if a.active != viewBoard {
		t.Errorf("expected viewBoard, got %d", a.active)
	}
}

func TestApp_BackKey_FromComment_ReturnsToIssue(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewComment
	app.previousView = viewIssue

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	a := model.(App)

	if a.active != viewIssue {
		t.Errorf("expected viewIssue, got %d", a.active)
	}
}

func TestApp_QKey_FromTransition_SuppressedByInputActive(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.transition = transitionview.New("PROJ-1")
	app.transition.SetSize(120, 38)
	app.active = viewTransition
	app.transitionOrigin = viewIssue

	// 'q' is suppressed by inputActive() — stays in transition view.
	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	a := model.(App)

	if a.active != viewTransition {
		t.Errorf("expected viewTransition (q suppressed), got %d", a.active)
	}
}

func TestApp_QKey_FromComment_SuppressedByInputActive(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.comment = commentview.New("PROJ-1")
	app.comment.SetSize(120, 38)
	app.active = viewComment
	app.previousView = viewIssue

	// 'q' is suppressed by inputActive() — stays in comment view.
	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	a := model.(App)

	if a.active != viewComment {
		t.Errorf("expected viewComment (q suppressed), got %d", a.active)
	}
}

func TestApp_EscKey_FromTransition_DismissesViaChildModel(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.transition = transitionview.New("PROJ-1")
	app.transition.SetSize(120, 38)
	app.transition.SetTransitions([]jira.Transition{{ID: "1", Name: "Done"}})
	app.active = viewTransition
	app.transitionOrigin = viewIssue

	// Esc is handled by child's Update, which sets Dismissed() — parent polls it.
	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	a := model.(App)

	if a.active != viewIssue {
		t.Errorf("expected viewIssue after esc from transition, got %d", a.active)
	}
}

func TestApp_EscKey_FromComment_DismissesViaChildModel(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.comment = commentview.New("PROJ-1")
	app.comment.SetSize(120, 38)
	app.active = viewComment
	app.previousView = viewIssue

	// Esc is handled by child's Update, which sets Dismissed() — parent polls it.
	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	a := model.(App)

	if a.active != viewIssue {
		t.Errorf("expected viewIssue after esc from comment, got %d", a.active)
	}
}

// --- View rendering for new view states ---

func TestApp_View_Transition(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.transition = transitionview.New("PROJ-1")
	app.transition.SetSize(120, 38)
	app.active = viewTransition

	v := app.View()
	if v == "" {
		t.Error("expected non-empty transition view")
	}
	if !strings.Contains(v, "PROJ-1") {
		t.Error("expected issue key in transition view")
	}
}

func TestApp_View_Transition_WithTransitions(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.transition = transitionview.New("PROJ-1")
	app.transition.SetSize(120, 38)
	app.transition.SetTransitions([]jira.Transition{
		{ID: "1", Name: "In Progress"},
		{ID: "2", Name: "Done"},
	})
	app.active = viewTransition

	v := app.View()
	if !strings.Contains(v, "In Progress") {
		t.Error("expected 'In Progress' in transition view")
	}
}

func TestApp_View_Comment(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.comment = commentview.New("PROJ-2")
	app.comment.SetSize(120, 38)
	app.active = viewComment

	v := app.View()
	if v == "" {
		t.Error("expected non-empty comment view")
	}
	if !strings.Contains(v, "PROJ-2") {
		t.Error("expected issue key in comment view")
	}
}

func TestApp_View_Branch(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.branch = branchview.New(jira.Issue{Key: "PROJ-3", Summary: "Branch test"}, "", false, "local")
	app.branch.SetSize(120, 38)
	app.active = viewBranch

	v := app.View()
	if v == "" {
		t.Error("expected non-empty branch view")
	}
}

func TestApp_View_StatusMessage(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewSprint
	app.statusMsg = "Moved to In Progress"

	v := app.View()
	if !strings.Contains(v, "Moved to In Progress") {
		t.Error("expected status message in view output")
	}
}

func TestApp_View_Create(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.create = createview.New(c)
	app.create.SetSize(120, 40)
	app.active = viewCreate

	v := app.View()
	if v == "" {
		t.Error("expected non-empty create view")
	}
}

// --- inputActive tests for new views ---

func TestApp_InputActive_Transition(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.transition = transitionview.New("PROJ-1")
	app.transition.SetSize(120, 38)
	app.active = viewTransition

	if !app.inputActive() {
		t.Error("expected inputActive() to return true in viewTransition")
	}
}

func TestApp_InputActive_Comment(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.comment = commentview.New("PROJ-1")
	app.comment.SetSize(120, 38)
	app.active = viewComment

	if !app.inputActive() {
		t.Error("expected inputActive() to return true in viewComment")
	}
}

func TestApp_InputActive_FalseForSprint(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewSprint

	if app.inputActive() {
		t.Error("expected inputActive() to return false in viewSprint (not filtering)")
	}
}

func TestApp_InputActive_FalseForIssue(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewIssue

	if app.inputActive() {
		t.Error("expected inputActive() to return false in viewIssue")
	}
}

// --- Command execution tests for transitions/comments ---

func TestApp_FetchTransitions_Success(t *testing.T) {
	c := defaultStub()
	c.transitions = []jira.Transition{
		{ID: "1", Name: "Start Progress", ToStatus: "In Progress"},
		{ID: "2", Name: "Close Issue", ToStatus: "Done"},
	}
	app := NewApp(c, "", nil, nil, "")

	cmd := app.fetchTransitions("PROJ-1")
	msg := cmd()

	loaded, ok := msg.(TransitionsLoadedMsg)
	if !ok {
		t.Fatalf("expected TransitionsLoadedMsg, got %T", msg)
	}
	if loaded.Key != "PROJ-1" {
		t.Errorf("expected key 'PROJ-1', got %q", loaded.Key)
	}
	if len(loaded.Transitions) != 2 {
		t.Errorf("expected 2 transitions, got %d", len(loaded.Transitions))
	}
	if loaded.Transitions[0].ToStatus != "In Progress" {
		t.Errorf("Transitions[0].ToStatus = %q, want %q", loaded.Transitions[0].ToStatus, "In Progress")
	}
	if loaded.Transitions[1].ToStatus != "Done" {
		t.Errorf("Transitions[1].ToStatus = %q, want %q", loaded.Transitions[1].ToStatus, "Done")
	}
}

func TestApp_FetchTransitions_Error(t *testing.T) {
	c := defaultStub()
	c.transErr = errors.New("transitions failed")
	app := NewApp(c, "", nil, nil, "")

	cmd := app.fetchTransitions("PROJ-1")
	msg := cmd()

	if _, ok := msg.(ErrMsg); !ok {
		t.Fatalf("expected ErrMsg, got %T", msg)
	}
}

func TestApp_TransitionIssue_Success(t *testing.T) {
	c := defaultStub()
	app := NewApp(c, "", nil, nil, "")

	cmd := app.transitionIssue("PROJ-1", "2", "Done")
	msg := cmd()

	transitioned, ok := msg.(IssueTransitionedMsg)
	if !ok {
		t.Fatalf("expected IssueTransitionedMsg, got %T", msg)
	}
	if transitioned.Key != "PROJ-1" {
		t.Errorf("expected key 'PROJ-1', got %q", transitioned.Key)
	}
	if transitioned.NewStatus != "Done" {
		t.Errorf("expected 'Done', got %q", transitioned.NewStatus)
	}
	if transitioned.Err != nil {
		t.Errorf("expected nil error, got %v", transitioned.Err)
	}
}

func TestApp_TransitionIssue_Error(t *testing.T) {
	c := defaultStub()
	c.transIssErr = errors.New("transition failed")
	app := NewApp(c, "", nil, nil, "")

	cmd := app.transitionIssue("PROJ-1", "2", "Done")
	msg := cmd()

	transitioned, ok := msg.(IssueTransitionedMsg)
	if !ok {
		t.Fatalf("expected IssueTransitionedMsg, got %T", msg)
	}
	if transitioned.Err == nil {
		t.Error("expected error")
	}
}

func TestApp_AddComment_Success(t *testing.T) {
	c := defaultStub()
	app := NewApp(c, "", nil, nil, "")

	cmd := app.addComment("PROJ-1", "This is a comment")
	msg := cmd()

	added, ok := msg.(CommentAddedMsg)
	if !ok {
		t.Fatalf("expected CommentAddedMsg, got %T", msg)
	}
	if added.Key != "PROJ-1" {
		t.Errorf("expected key 'PROJ-1', got %q", added.Key)
	}
	if added.Err != nil {
		t.Errorf("expected nil error, got %v", added.Err)
	}
}

func TestApp_AddComment_Error(t *testing.T) {
	c := defaultStub()
	c.commentErr = errors.New("comment failed")
	app := NewApp(c, "", nil, nil, "")

	cmd := app.addComment("PROJ-1", "This is a comment")
	msg := cmd()

	added, ok := msg.(CommentAddedMsg)
	if !ok {
		t.Fatalf("expected CommentAddedMsg, got %T", msg)
	}
	if added.Err == nil {
		t.Error("expected error")
	}
}

// --- Refresh current view ---

func TestApp_RefreshCurrentView_WithBoardID(t *testing.T) {
	c := defaultStub()
	c.boardSprints = []jira.Sprint{{ID: 10, Name: "Sprint 10"}}
	app := newTestApp(c, "")
	app.boardID = 42

	cmd := app.refreshCurrentView()
	if cmd == nil {
		t.Error("expected non-nil cmd when boardID is set")
	}
}

func TestApp_RefreshCurrentView_NoBoardID(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.boardID = 0

	cmd := app.refreshCurrentView()
	if cmd != nil {
		t.Error("expected nil cmd when boardID is zero")
	}
}

// --- sanitiseError on BranchCreatedMsg ---

func TestApp_BranchCreatedMsg_ErrorWithURL_IsSanitised(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewBranch

	model, _ := app.Update(BranchCreatedMsg{Err: fmt.Errorf("failed: https://internal.server/api")})
	a := model.(App)

	if a.err == nil {
		t.Fatal("expected error to be set")
	}
	if strings.Contains(a.err.Error(), "https://") {
		t.Errorf("expected URL stripped from error, got %q", a.err.Error())
	}
	if !strings.Contains(a.err.Error(), "[url redacted]") {
		t.Errorf("expected [url redacted], got %q", a.err.Error())
	}
}

// --- ErrMsg sanitisation ---

func TestApp_ErrMsg_SanitisesURLs(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")

	model, _ := app.Update(ErrMsg{Err: fmt.Errorf("request to https://api.jira.com/rest/api/2 failed")})
	a := model.(App)

	if strings.Contains(a.err.Error(), "https://") {
		t.Errorf("expected URL sanitised in ErrMsg, got %q", a.err.Error())
	}
}

func TestApp_FilterDuplicatedMsg_SetsStatus(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewFilters

	dup := jira.SavedFilter{ID: "abc", Name: "My Filter (copy)", JQL: "status = Open"}
	model, _ := app.Update(FilterDuplicatedMsg{Filter: dup})
	a := model.(App)

	if a.statusMsg == "" {
		t.Error("expected status message after duplicate")
	}
	if !strings.Contains(a.statusMsg, "duplicated") {
		t.Errorf("expected 'duplicated' in status, got %q", a.statusMsg)
	}
}

func TestApp_FiltersKey_ClearsStaleState(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewSprint
	app.savedFilters = []jira.SavedFilter{{ID: "x", Name: "Test", JQL: "status = Open"}}

	// Put filter into a dirty edit state.
	app.filter.StartAdd("stale query")

	// Press 'f' to open filters — should call Reset().
	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	a := model.(App)

	if a.active != viewFilters {
		t.Fatalf("expected viewFilters, got %d", a.active)
	}
	if a.filter.InputActive() {
		t.Error("expected InputActive false after Reset — stale edit state should be cleared")
	}
}

func TestApp_TransitionView_UsesToStatusWhenPresent(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewTransition
	app.transitionOrigin = viewSprint
	app.transition = transitionview.New("PROJ-1")
	app.transition.SetSize(120, 38)
	app.transition.SetTransitions([]jira.Transition{
		{ID: "11", Name: "Start Progress", ToStatus: "In Progress"},
	})

	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_ = model
	if cmd == nil {
		t.Fatal("expected non-nil cmd after selecting transition")
	}

	msg := cmd()
	transitioned, ok := msg.(IssueTransitionedMsg)
	if !ok {
		t.Fatalf("expected IssueTransitionedMsg, got %T", msg)
	}
	if transitioned.NewStatus != "In Progress" {
		t.Errorf("NewStatus = %q, want %q (should use ToStatus)", transitioned.NewStatus, "In Progress")
	}
}

func TestApp_TransitionView_FallsBackToNameWhenToStatusEmpty(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewTransition
	app.transitionOrigin = viewSprint
	app.transition = transitionview.New("PROJ-1")
	app.transition.SetSize(120, 38)
	app.transition.SetTransitions([]jira.Transition{
		{ID: "11", Name: "Done"},
	})

	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_ = model
	if cmd == nil {
		t.Fatal("expected non-nil cmd after selecting transition")
	}

	msg := cmd()
	transitioned, ok := msg.(IssueTransitionedMsg)
	if !ok {
		t.Fatalf("expected IssueTransitionedMsg, got %T", msg)
	}
	if transitioned.NewStatus != "Done" {
		t.Errorf("NewStatus = %q, want %q (should fall back to Name)", transitioned.NewStatus, "Done")
	}
}

// --- Search board view tests ---

func TestApp_BoardToggle_FromSearchResults(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")

	// Set up search results.
	issues := []jira.Issue{
		{Key: "PROJ-1", Summary: "Task", Status: "To Do"},
	}
	model, _ := app.Update(SearchResultsMsg{Issues: issues, Query: "status = Open"})
	a := model.(App)

	if a.active != viewSearch {
		t.Fatalf("expected viewSearch, got %d", a.active)
	}

	// Press 'b' to switch to search board view.
	model, _ = a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	a = model.(App)

	if a.active != viewSearchBoard {
		t.Errorf("expected viewSearchBoard, got %d", a.active)
	}

	// Press 'b' again to switch back to search results.
	model, _ = a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	a = model.(App)

	if a.active != viewSearch {
		t.Errorf("expected viewSearch after toggle back, got %d", a.active)
	}
}

func TestApp_BackKey_FromSearchBoard_GoesToSearch(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewSearchBoard

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	a := model.(App)

	if a.active != viewSearch {
		t.Errorf("expected viewSearch on esc from search board, got %d", a.active)
	}
}

func TestApp_BackKey_FromIssue_ToSearchBoard(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")

	app.active = viewIssue
	app.previousView = viewSearchBoard
	app.searchIssues = []jira.Issue{{Key: "PROJ-1", Summary: "Task", Status: "To Do"}}
	app.searchBoardTitle = "status = Open"

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	a := model.(App)

	if a.active != viewSearchBoard {
		t.Errorf("expected back to viewSearchBoard, got %d", a.active)
	}
}

func TestApp_SearchResultsMsg_CachesIssues(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")

	issues := []jira.Issue{
		{Key: "PROJ-1", Summary: "One", Status: "To Do"},
		{Key: "PROJ-2", Summary: "Two", Status: "Done"},
	}
	model, _ := app.Update(SearchResultsMsg{Issues: issues, Query: "project = PROJ"})
	a := model.(App)

	if len(a.searchIssues) != 2 {
		t.Errorf("expected 2 cached search issues, got %d", len(a.searchIssues))
	}
	if a.searchBoardTitle != "project = PROJ" {
		t.Errorf("expected searchBoardTitle %q, got %q", "project = PROJ", a.searchBoardTitle)
	}
}

func TestApp_SearchBoardDisplayTitle_UsesFilterName(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")

	// Without a filter name, display title should be the raw JQL.
	app.searchBoardTitle = "status = Open"
	if got := app.searchBoardDisplayTitle(); got != "status = Open" {
		t.Errorf("expected raw JQL title, got %q", got)
	}

	// With a filter name set, display title should use it.
	app.search.SetFilterName("My Bugs")
	if got := app.searchBoardDisplayTitle(); got != "Filter: My Bugs" {
		t.Errorf("expected filter title, got %q", got)
	}

	// Clearing filter name should revert to raw JQL.
	app.search.SetFilterName("")
	if got := app.searchBoardDisplayTitle(); got != "status = Open" {
		t.Errorf("expected raw JQL title after clearing, got %q", got)
	}
}

func TestApp_IssuesPageMsg_SearchSource_AppendsToSearchCache(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewSearch
	app.search.Show()
	app.search.SetResults([]jira.Issue{{Key: "S-1"}}, "status = Open")
	app.searchIssues = []jira.Issue{{Key: "S-1"}}

	model, _ := app.Update(IssuesPageMsg{
		Issues:  []jira.Issue{{Key: "S-2"}, {Key: "S-1"}}, // S-1 is a duplicate.
		HasMore: false,
		Source:  "search",
		From:    2,
		Seq:     app.paginationSeq,
	})
	a := model.(App)

	if len(a.searchIssues) != 2 {
		t.Errorf("expected 2 cached search issues (deduped), got %d", len(a.searchIssues))
	}
}

func TestApp_TransitionKey_FromSearchBoard(t *testing.T) {
	c := defaultStub()
	c.transitions = []jira.Transition{{ID: "1", Name: "In Progress"}}
	app := newTestApp(c, "")

	// Set up search board with an issue.
	app.searchIssues = []jira.Issue{{Key: "PROJ-1", Summary: "Task", Status: "To Do"}}
	app.board.SetIssues(app.searchIssues, "status = Open")
	app.active = viewSearchBoard

	// Select the issue in the board for highlight.
	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("m")})
	a := model.(App)

	if a.active != viewTransition {
		t.Errorf("expected viewTransition, got %d", a.active)
	}
	if a.transitionOrigin != viewSearchBoard {
		t.Errorf("expected transitionOrigin viewSearchBoard, got %d", a.transitionOrigin)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd for fetching transitions")
	}
}

func TestApp_IssueTransitionedMsg_Success_FromSearchBoard(t *testing.T) {
	c := defaultStub()
	c.searchIssues = []jira.Issue{{Key: "PROJ-1"}}
	app := newTestApp(c, "")
	app.active = viewTransition
	app.transitionOrigin = viewSearchBoard
	app.searchIssues = []jira.Issue{{Key: "PROJ-1", Summary: "Task", Status: "To Do"}}
	app.searchBoardTitle = "status = Open"

	model, cmd := app.Update(IssueTransitionedMsg{Key: "PROJ-1", NewStatus: "Done"})
	a := model.(App)

	if a.active != viewSearchBoard {
		t.Errorf("expected viewSearchBoard after transition, got %d", a.active)
	}
	if a.statusMsg != "Moved to Done" {
		t.Errorf("expected 'Moved to Done', got %q", a.statusMsg)
	}
	// Search cache should be updated.
	if a.searchIssues[0].Status != "Done" {
		t.Errorf("expected search cache status 'Done', got %q", a.searchIssues[0].Status)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd (searchJQL refresh)")
	}
}

func TestApp_RefreshKey_FromSearchBoard(t *testing.T) {
	c := defaultStub()
	c.searchIssues = []jira.Issue{{Key: "S-1"}}
	app := newTestApp(c, "")
	app.active = viewSearchBoard
	app.searchBoardTitle = "status = Open"

	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	a := model.(App)

	if a.statusMsg != "Refreshing..." {
		t.Errorf("expected 'Refreshing...', got %q", a.statusMsg)
	}
	if cmd == nil {
		t.Error("expected non-nil cmd (searchJQL)")
	}
}

func TestApp_View_SearchBoard(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewSearchBoard

	v := app.View()
	if v == "" {
		t.Error("expected non-empty search board view")
	}
}

func TestFooterView_SearchBoard(t *testing.T) {
	v := footerView(viewSearchBoard, 120, "", false)
	if !strings.Contains(v, "list view") {
		t.Error("expected 'list view' in search board footer")
	}
	if !strings.Contains(v, "move") {
		t.Error("expected 'move' in search board footer")
	}
	if !strings.Contains(v, "columns") {
		t.Error("expected 'columns' in search board footer")
	}
	if !strings.Contains(v, "filters") {
		t.Error("expected 'filters' in search board footer")
	}
}

func TestApp_FiltersKey_FromSearchBoard(t *testing.T) {
	c := defaultStub()
	app := newTestApp(c, "")
	app.active = viewSearchBoard

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	a := model.(App)

	if a.active != viewFilters {
		t.Errorf("expected viewFilters, got %d", a.active)
	}
	if a.filterOrigin != viewSearchBoard {
		t.Errorf("expected filterOrigin viewSearchBoard, got %d", a.filterOrigin)
	}
}
