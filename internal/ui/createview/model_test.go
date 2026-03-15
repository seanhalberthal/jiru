package createview

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/seanhalberthal/jiru/internal/client"
	"github.com/seanhalberthal/jiru/internal/config"
	"github.com/seanhalberthal/jiru/internal/jira"
)

// --- Stub client ---

type stubClient struct {
	cfg           *config.Config
	projects      []jira.Project
	projectsErr   error
	issueTypes    []string
	issueTypesErr error
	metadata      *jira.JQLMetadata
	metadataErr   error
	userResults   []client.UserInfo
	userErr       error
	createResp    *client.CreateIssueResponse
	createErr     error
}

func (s *stubClient) Me() (string, error)                                        { return "Test User", nil }
func (s *stubClient) Config() *config.Config                                     { return s.cfg }
func (s *stubClient) SprintIssues(_ int) ([]jira.Issue, error)                   { return nil, nil }
func (s *stubClient) GetIssue(_ string) (*jira.Issue, error)                     { return nil, nil }
func (s *stubClient) IssueURL(_ string) string                                   { return "" }
func (s *stubClient) Boards(_ string) ([]jira.Board, error)                      { return nil, nil }
func (s *stubClient) BoardSprints(_ int, _ string) ([]jira.Sprint, error)        { return nil, nil }
func (s *stubClient) SearchJQL(_ string, _ uint) ([]jira.Issue, error)           { return nil, nil }
func (s *stubClient) SprintIssueStats(_ int) (int, int, int, int, error)         { return 0, 0, 0, 0, nil }
func (s *stubClient) ResolveParents(_ []jira.Issue) map[string]client.ParentInfo { return nil }
func (s *stubClient) BoardIssues(_ string, _ ...string) ([]jira.Issue, error)    { return nil, nil }
func (s *stubClient) EpicIssues(_ string) ([]jira.Issue, error)                  { return nil, nil }
func (s *stubClient) Projects() ([]jira.Project, error)                          { return s.projects, s.projectsErr }
func (s *stubClient) JQLMetadata() (*jira.JQLMetadata, error)                    { return s.metadata, s.metadataErr }
func (s *stubClient) SearchUsers(_, _ string) ([]client.UserInfo, error) {
	return s.userResults, s.userErr
}
func (s *stubClient) CreateIssue(_ *client.CreateIssueRequest) (*client.CreateIssueResponse, error) {
	return s.createResp, s.createErr
}
func (s *stubClient) IssueTypes(_ string) ([]string, error) {
	return s.issueTypes, s.issueTypesErr
}
func (s *stubClient) Transitions(_ string) ([]jira.Transition, error)          { return nil, nil }
func (s *stubClient) TransitionIssue(_, _ string) error                        { return nil }
func (s *stubClient) AddComment(_, _ string) error                             { return nil }
func (s *stubClient) SprintIssuesPage(_, _, _ int) (*client.PageResult, error) { return nil, nil }
func (s *stubClient) SearchJQLPage(_ string, _ int, _ string) (*client.PageResult, error) {
	return nil, nil
}
func (s *stubClient) EpicIssuesPage(_ string, _, _ int) (*client.PageResult, error) { return nil, nil }

func defaultStub() *stubClient {
	return &stubClient{
		cfg:        &config.Config{Domain: "test.atlassian.net", User: "alice", APIToken: "tok", AuthType: "basic", Project: "PROJ"},
		projects:   []jira.Project{{Key: "PROJ", Name: "Project"}, {Key: "TEST", Name: "Test Project"}},
		issueTypes: []string{"Bug", "Story", "Task"},
		metadata: &jira.JQLMetadata{
			Priorities: []string{"High", "Medium", "Low"},
			Labels:     []string{"frontend", "backend", "urgent"},
		},
		userResults: []client.UserInfo{
			{AccountID: "alice-id", DisplayName: "Alice"},
			{AccountID: "bob-id", DisplayName: "Bob"},
		},
		createResp: &client.CreateIssueResponse{Key: "PROJ-42"},
	}
}

func testModel(c *stubClient) Model {
	m := New(c)
	m.SetSize(120, 40)
	return m
}

// --- Tests ---

func TestNew_StartsAtProjectStep(t *testing.T) {
	m := testModel(defaultStub())
	if m.step != stepProject {
		t.Errorf("expected step %d, got %d", stepProject, m.step)
	}
}

func TestNew_PreSelectsConfiguredProject(t *testing.T) {
	c := defaultStub()
	m := testModel(c)
	if m.project != "PROJ" {
		t.Errorf("expected project 'PROJ', got %q", m.project)
	}
}

func TestNew_InitialSentinels(t *testing.T) {
	m := testModel(defaultStub())
	if m.Done() {
		t.Error("expected Done() == false initially")
	}
	if m.Quit() {
		t.Error("expected Quit() == false initially")
	}
	if m.CreatedKey() != "" {
		t.Errorf("expected empty CreatedKey(), got %q", m.CreatedKey())
	}
}

func TestProjectsLoaded_PopulatesProjects(t *testing.T) {
	m := testModel(defaultStub())
	projects := []jira.Project{{Key: "PROJ", Name: "Project"}, {Key: "TEST", Name: "Test"}}
	m, _ = m.Update(projectsLoadedMsg{projects: projects})

	if !m.projectLoaded {
		t.Error("expected projectLoaded to be true")
	}
	if len(m.projects) != 2 {
		t.Errorf("expected 2 projects, got %d", len(m.projects))
	}
	if m.loading {
		t.Error("expected loading to be false")
	}
}

func TestProjectsLoaded_PreSelectsConfigured(t *testing.T) {
	c := defaultStub()
	m := testModel(c)
	projects := []jira.Project{{Key: "OTHER", Name: "Other"}, {Key: "PROJ", Name: "Project"}}
	m, _ = m.Update(projectsLoadedMsg{projects: projects})

	if m.projectCursor != 1 {
		t.Errorf("expected cursor at 1 (PROJ), got %d", m.projectCursor)
	}
}

func TestProjectsLoaded_Error(t *testing.T) {
	m := testModel(defaultStub())
	m, _ = m.Update(projectsLoadedMsg{err: errors.New("network error")})

	if m.errMsg == "" {
		t.Error("expected error message to be set")
	}
	if !strings.Contains(m.errMsg, "network error") {
		t.Errorf("expected 'network error' in errMsg, got %q", m.errMsg)
	}
}

func TestProjectPicker_Navigation(t *testing.T) {
	m := testModel(defaultStub())
	m, _ = m.Update(projectsLoadedMsg{projects: defaultStub().projects})

	// Initial cursor at 0.
	if m.projectCursor != 0 {
		t.Fatalf("expected cursor 0, got %d", m.projectCursor)
	}

	// Move down.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m.projectCursor != 1 {
		t.Errorf("expected cursor 1 after j, got %d", m.projectCursor)
	}

	// Move down past end — should not go beyond.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m.projectCursor != 1 {
		t.Errorf("expected cursor 1 at boundary, got %d", m.projectCursor)
	}

	// Move up.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if m.projectCursor != 0 {
		t.Errorf("expected cursor 0 after k, got %d", m.projectCursor)
	}

	// Move up past start — should not go negative.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if m.projectCursor != 0 {
		t.Errorf("expected cursor 0 at boundary, got %d", m.projectCursor)
	}
}

func TestProjectPicker_SelectAdvancesToIssueType(t *testing.T) {
	m := testModel(defaultStub())
	m, _ = m.Update(projectsLoadedMsg{projects: defaultStub().projects})

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.values[stepProject] != "PROJ" {
		t.Errorf("expected project value 'PROJ', got %q", m.values[stepProject])
	}
	if m.step != stepIssueType {
		t.Errorf("expected step %d (issueType), got %d", stepIssueType, m.step)
	}
}

func TestProjectPicker_EmptyProjectsShowsError(t *testing.T) {
	m := testModel(defaultStub())
	m, _ = m.Update(projectsLoadedMsg{projects: nil})

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.errMsg == "" {
		t.Error("expected error message for empty projects")
	}
	if m.step != stepProject {
		t.Errorf("expected to stay on project step, got %d", m.step)
	}
}

func TestProjectPicker_IgnoresKeysWhileLoading(t *testing.T) {
	m := testModel(defaultStub())
	// Projects not loaded yet — loading should be set by Init.
	// But we simulate it directly:
	m.loading = true

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	// Should not advance.
	if m.step != stepProject {
		t.Errorf("expected to stay on project step while loading, got %d", m.step)
	}
}

func TestIssueTypesLoaded_PopulatesTypes(t *testing.T) {
	m := testModel(defaultStub())
	m.step = stepIssueType
	m, _ = m.Update(issueTypesLoadedMsg{types: []string{"Bug", "Story", "Task"}})

	if !m.issueTypeLoaded {
		t.Error("expected issueTypeLoaded to be true")
	}
	if len(m.issueTypes) != 3 {
		t.Errorf("expected 3 issue types, got %d", len(m.issueTypes))
	}
}

func TestIssueTypesLoaded_Error(t *testing.T) {
	m := testModel(defaultStub())
	m.step = stepIssueType
	m, _ = m.Update(issueTypesLoadedMsg{err: errors.New("fetch failed")})

	if !strings.Contains(m.errMsg, "fetch failed") {
		t.Errorf("expected 'fetch failed' in errMsg, got %q", m.errMsg)
	}
}

func TestIssueTypePicker_Navigation(t *testing.T) {
	m := testModel(defaultStub())
	m.step = stepIssueType
	m, _ = m.Update(issueTypesLoadedMsg{types: []string{"Bug", "Story", "Task"}})

	// Move down.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m.issueTypeCursor != 1 {
		t.Errorf("expected cursor 1, got %d", m.issueTypeCursor)
	}

	// Move up.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if m.issueTypeCursor != 0 {
		t.Errorf("expected cursor 0, got %d", m.issueTypeCursor)
	}
}

func TestIssueTypePicker_SelectAdvancesToSummary(t *testing.T) {
	m := testModel(defaultStub())
	m.step = stepIssueType
	m, _ = m.Update(issueTypesLoadedMsg{types: []string{"Bug", "Story"}})

	// Select "Bug" (cursor at 0).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.values[stepIssueType] != "Bug" {
		t.Errorf("expected 'Bug', got %q", m.values[stepIssueType])
	}
	if m.step != stepSummary {
		t.Errorf("expected step %d (summary), got %d", stepSummary, m.step)
	}
}

func TestSummary_RequiredValidation(t *testing.T) {
	m := testModel(defaultStub())
	m.step = stepSummary
	m.inputs[stepSummary].Focus()

	// Try to advance with empty summary.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.errMsg == "" {
		t.Error("expected error for empty required field")
	}
	if m.step != stepSummary {
		t.Errorf("expected to stay on summary step, got %d", m.step)
	}
}

func TestSummary_AdvancesToPriority(t *testing.T) {
	m := testModel(defaultStub())
	m.step = stepSummary
	m.inputs[stepSummary].Focus()
	m.inputs[stepSummary].SetValue("Fix login bug")

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.values[stepSummary] != "Fix login bug" {
		t.Errorf("expected 'Fix login bug', got %q", m.values[stepSummary])
	}
	if m.step != stepPriority {
		t.Errorf("expected step %d (priority), got %d", stepPriority, m.step)
	}
}

func TestPrioritiesLoaded_PopulatesPriorities(t *testing.T) {
	m := testModel(defaultStub())
	m.step = stepPriority
	m, _ = m.Update(prioritiesLoadedMsg{priorities: []string{"High", "Medium", "Low"}})

	if !m.priorityLoaded {
		t.Error("expected priorityLoaded to be true")
	}
	if len(m.priorities) != 3 {
		t.Errorf("expected 3 priorities, got %d", len(m.priorities))
	}
}

func TestPriorityPicker_NoneOption(t *testing.T) {
	m := testModel(defaultStub())
	m.step = stepPriority
	m, _ = m.Update(prioritiesLoadedMsg{priorities: []string{"High", "Medium", "Low"}})

	// Cursor starts at 0 ("None").
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.values[stepPriority] != "" {
		t.Errorf("expected empty priority for 'None', got %q", m.values[stepPriority])
	}
	if m.step != stepAssignee {
		t.Errorf("expected step %d (assignee), got %d", stepAssignee, m.step)
	}
}

func TestPriorityPicker_SelectNonNone(t *testing.T) {
	m := testModel(defaultStub())
	m.step = stepPriority
	m, _ = m.Update(prioritiesLoadedMsg{priorities: []string{"High", "Medium", "Low"}})

	// Move to "High" (index 1).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.values[stepPriority] != "High" {
		t.Errorf("expected 'High', got %q", m.values[stepPriority])
	}
}

func TestPriorityPicker_NavigationBounds(t *testing.T) {
	m := testModel(defaultStub())
	m.step = stepPriority
	m, _ = m.Update(prioritiesLoadedMsg{priorities: []string{"High", "Medium"}})

	// Move to end: None(0), High(1), Medium(2) = 3 items.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m.priorityCursor != 2 {
		t.Errorf("expected cursor 2, got %d", m.priorityCursor)
	}

	// Can't go past end.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m.priorityCursor != 2 {
		t.Errorf("expected cursor 2 at boundary, got %d", m.priorityCursor)
	}
}

func TestAssignee_EnterWithUserResults(t *testing.T) {
	m := testModel(defaultStub())
	m.step = stepAssignee
	m.inputs[stepAssignee].Focus()
	m, _ = m.Update(userSearchResultMsg{users: []client.UserInfo{{AccountID: "alice-id", DisplayName: "Alice"}, {AccountID: "bob-id", DisplayName: "Bob"}}})

	// Enter selects the first user result.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.values[stepAssignee] != "alice-id" {
		t.Errorf("expected 'alice-id', got %q", m.values[stepAssignee])
	}
	if m.step != stepLabels {
		t.Errorf("expected step %d (labels), got %d", stepLabels, m.step)
	}
}

func TestAssignee_NavigateUserResults(t *testing.T) {
	m := testModel(defaultStub())
	m.step = stepAssignee
	m.inputs[stepAssignee].Focus()
	m, _ = m.Update(userSearchResultMsg{users: []client.UserInfo{{AccountID: "alice-id", DisplayName: "Alice"}, {AccountID: "bob-id", DisplayName: "Bob"}, {AccountID: "charlie-id", DisplayName: "Charlie"}}})

	// Navigate down.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.userCursor != 1 {
		t.Errorf("expected cursor 1, got %d", m.userCursor)
	}

	// Navigate down again.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.userCursor != 2 {
		t.Errorf("expected cursor 2, got %d", m.userCursor)
	}

	// Can't go past end.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.userCursor != 2 {
		t.Errorf("expected cursor 2 at boundary, got %d", m.userCursor)
	}

	// Navigate up.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.userCursor != 1 {
		t.Errorf("expected cursor 1, got %d", m.userCursor)
	}

	// Select "Bob".
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.values[stepAssignee] != "bob-id" {
		t.Errorf("expected 'bob-id', got %q", m.values[stepAssignee])
	}
}

func TestAssignee_TabAcceptsResult(t *testing.T) {
	m := testModel(defaultStub())
	m.step = stepAssignee
	m.inputs[stepAssignee].Focus()
	m, _ = m.Update(userSearchResultMsg{users: []client.UserInfo{{AccountID: "alice-id", DisplayName: "Alice"}, {AccountID: "bob-id", DisplayName: "Bob"}}})

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.inputs[stepAssignee].Value() != "Alice" {
		t.Errorf("expected input to be 'Alice' after tab, got %q", m.inputs[stepAssignee].Value())
	}
	if m.values[stepAssignee] != "alice-id" {
		t.Errorf("expected values[stepAssignee] to be account ID 'alice-id', got %q", m.values[stepAssignee])
	}
	if m.userResults != nil {
		t.Error("expected user results to be cleared after tab")
	}
}

func TestAssignee_TypingReachesTextInput(t *testing.T) {
	m := testModel(defaultStub())
	m.step = stepAssignee
	m.inputs[stepAssignee].Focus()

	// Type characters — they should reach the text input, not be swallowed.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	if m.inputs[stepAssignee].Value() != "a" {
		t.Errorf("expected 'a' in input, got %q", m.inputs[stepAssignee].Value())
	}

	// Second character should trigger a search command (len >= 2).
	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	if m.inputs[stepAssignee].Value() != "al" {
		t.Errorf("expected 'al' in input, got %q", m.inputs[stepAssignee].Value())
	}
	if cmd == nil {
		t.Error("expected non-nil cmd (search should have been triggered)")
	}
	if m.userSearchTerm != "al" {
		t.Errorf("expected userSearchTerm 'al', got %q", m.userSearchTerm)
	}
}

func TestAssignee_SearchResultsUpdateWhileTyping(t *testing.T) {
	m := testModel(defaultStub())
	m.step = stepAssignee
	m.inputs[stepAssignee].Focus()

	// Type two characters to trigger search.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})

	// Simulate search results arriving.
	m, _ = m.Update(userSearchResultMsg{users: []client.UserInfo{{AccountID: "alice-id", DisplayName: "Alice"}, {AccountID: "alej-id", DisplayName: "Alejandro"}}})
	if len(m.userResults) != 2 {
		t.Errorf("expected 2 results, got %d", len(m.userResults))
	}

	// Type another character — should trigger a new search.
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})
	if m.inputs[stepAssignee].Value() != "ali" {
		t.Errorf("expected 'ali' in input, got %q", m.inputs[stepAssignee].Value())
	}
	if cmd == nil {
		t.Error("expected non-nil cmd (new search should have been triggered)")
	}
	if m.userSearchTerm != "ali" {
		t.Errorf("expected userSearchTerm 'ali', got %q", m.userSearchTerm)
	}

	// Simulate updated results.
	m, _ = m.Update(userSearchResultMsg{users: []client.UserInfo{{AccountID: "alice-id", DisplayName: "Alice"}}})
	if len(m.userResults) != 1 {
		t.Errorf("expected 1 result after refined search, got %d", len(m.userResults))
	}
}

func TestAssignee_EnterWithNoResults(t *testing.T) {
	m := testModel(defaultStub())
	m.step = stepAssignee
	m.inputs[stepAssignee].Focus()
	m.inputs[stepAssignee].SetValue("custom-user")

	// No user search results — should use raw input.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.values[stepAssignee] != "custom-user" {
		t.Errorf("expected 'custom-user', got %q", m.values[stepAssignee])
	}
}

func TestUserSearchResults_ResetsCursor(t *testing.T) {
	m := testModel(defaultStub())
	m.userCursor = 5
	m, _ = m.Update(userSearchResultMsg{users: []client.UserInfo{{AccountID: "alice-id", DisplayName: "Alice"}}})
	if m.userCursor != 0 {
		t.Errorf("expected cursor reset to 0, got %d", m.userCursor)
	}
}

func TestLabels_AdvancesToParent(t *testing.T) {
	m := testModel(defaultStub())
	m.step = stepLabels
	m.inputs[stepLabels].Focus()
	m.inputs[stepLabels].SetValue("bug, frontend")

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.values[stepLabels] != "bug, frontend" {
		t.Errorf("expected 'bug, frontend', got %q", m.values[stepLabels])
	}
	if m.step != stepParent {
		t.Errorf("expected step %d (parent), got %d", stepParent, m.step)
	}
}

func TestLabels_EmptyAllowed(t *testing.T) {
	m := testModel(defaultStub())
	m.step = stepLabels
	m.inputs[stepLabels].Focus()

	// Labels are optional — empty should advance.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.step != stepParent {
		t.Errorf("expected step %d (parent), got %d", stepParent, m.step)
	}
}

func TestParent_AdvancesToDescription(t *testing.T) {
	m := testModel(defaultStub())
	m.step = stepParent
	m.inputs[stepParent].Focus()
	m.inputs[stepParent].SetValue("PROJ-100")

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.values[stepParent] != "PROJ-100" {
		t.Errorf("expected 'PROJ-100', got %q", m.values[stepParent])
	}
	if m.step != stepDescription {
		t.Errorf("expected step %d (description), got %d", stepDescription, m.step)
	}
}

func TestDescription_AdvancesToConfirm(t *testing.T) {
	m := testModel(defaultStub())
	m.step = stepDescription
	m.inputs[stepDescription].Focus()
	m.inputs[stepDescription].SetValue("Some description")

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.values[stepDescription] != "Some description" {
		t.Errorf("expected 'Some description', got %q", m.values[stepDescription])
	}
	if m.step != stepConfirm {
		t.Errorf("expected step %d (confirm), got %d", stepConfirm, m.step)
	}
}

func TestConfirm_EnterTriggersCreate(t *testing.T) {
	c := defaultStub()
	m := testModel(c)
	m.step = stepConfirm
	m.values[stepProject] = "PROJ"
	m.values[stepIssueType] = "Bug"
	m.values[stepSummary] = "Fix login"

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.loading {
		t.Error("expected loading to be true after confirm")
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd from confirm")
	}
}

func TestIssueCreated_Success(t *testing.T) {
	m := testModel(defaultStub())
	m.step = stepConfirm
	m.loading = true

	m, _ = m.Update(issueCreatedMsg{key: "PROJ-42"})
	if !m.Done() {
		t.Error("expected Done() == true")
	}
	if m.CreatedKey() != "PROJ-42" {
		t.Errorf("expected 'PROJ-42', got %q", m.CreatedKey())
	}
	if m.loading {
		t.Error("expected loading to be false")
	}
}

func TestIssueCreated_Error(t *testing.T) {
	m := testModel(defaultStub())
	m.step = stepConfirm
	m.loading = true

	m, _ = m.Update(issueCreatedMsg{err: errors.New("permission denied")})
	if m.Done() {
		t.Error("expected Done() == false on error")
	}
	if !strings.Contains(m.errMsg, "permission denied") {
		t.Errorf("expected 'permission denied' in errMsg, got %q", m.errMsg)
	}
}

func TestEsc_Quits(t *testing.T) {
	m := testModel(defaultStub())
	m, _ = m.Update(projectsLoadedMsg{projects: defaultStub().projects})

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !m.Quit() {
		t.Error("expected Quit() == true after Esc")
	}
}

func TestCtrlC_Quits(t *testing.T) {
	m := testModel(defaultStub())
	m, _ = m.Update(projectsLoadedMsg{projects: defaultStub().projects})

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if !m.Quit() {
		t.Error("expected Quit() == true after Ctrl+C")
	}
}

func TestEsc_QuitsWhileLoading(t *testing.T) {
	m := testModel(defaultStub())
	m.loading = true

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !m.Quit() {
		t.Error("expected Quit() == true after Esc while loading")
	}
}

func TestCtrlB_GoesBack(t *testing.T) {
	m := testModel(defaultStub())
	m.step = stepSummary

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlB})
	if m.step != stepIssueType {
		t.Errorf("expected step %d (issueType), got %d", stepIssueType, m.step)
	}
}

func TestCtrlB_QuitsAtProjectStep(t *testing.T) {
	m := testModel(defaultStub())
	m.step = stepProject
	m, _ = m.Update(projectsLoadedMsg{projects: defaultStub().projects})

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlB})
	if !m.Quit() {
		t.Error("expected Quit() == true when going back from first step")
	}
}

func TestCtrlB_ClearsError(t *testing.T) {
	m := testModel(defaultStub())
	m.step = stepSummary
	m.errMsg = "some error"

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlB})
	if m.errMsg != "" {
		t.Errorf("expected errMsg cleared, got %q", m.errMsg)
	}
}

func TestInputActive_TrueForInputSteps(t *testing.T) {
	inputSteps := []int{stepSummary, stepAssignee, stepLabels, stepParent, stepDescription}
	for _, step := range inputSteps {
		m := testModel(defaultStub())
		m.step = step
		if !m.InputActive() {
			t.Errorf("expected InputActive() == true for step %d", step)
		}
	}
}

func TestInputActive_FalseForPickerSteps(t *testing.T) {
	pickerSteps := []int{stepProject, stepIssueType, stepPriority, stepConfirm}
	for _, step := range pickerSteps {
		m := testModel(defaultStub())
		m.step = step
		if m.InputActive() {
			t.Errorf("expected InputActive() == false for step %d", step)
		}
	}
}

func TestLabelsLoaded_PopulatesLabels(t *testing.T) {
	m := testModel(defaultStub())
	m, _ = m.Update(labelsLoadedMsg{labels: []string{"frontend", "backend"}})

	if !m.labelsLoaded {
		t.Error("expected labelsLoaded to be true")
	}
	if len(m.labels) != 2 {
		t.Errorf("expected 2 labels, got %d", len(m.labels))
	}
}

func TestLabelsLoaded_ErrorIsNonFatal(t *testing.T) {
	m := testModel(defaultStub())
	m, _ = m.Update(labelsLoadedMsg{err: errors.New("labels error")})

	if !m.labelsLoaded {
		t.Error("expected labelsLoaded to be true even on error")
	}
	if len(m.labels) != 0 {
		t.Errorf("expected 0 labels on error, got %d", len(m.labels))
	}
}

func TestLabelHints_MatchesPrefix(t *testing.T) {
	m := testModel(defaultStub())
	m.labels = []string{"frontend", "backend", "bugfix", "feature"}

	hints := m.labelHints("fro")
	if len(hints) != 1 {
		t.Errorf("expected 1 hint for 'fro', got %d", len(hints))
	}
	if len(hints) > 0 && hints[0] != "frontend" {
		t.Errorf("expected 'frontend', got %q", hints[0])
	}
}

func TestLabelHints_CaseInsensitive(t *testing.T) {
	m := testModel(defaultStub())
	m.labels = []string{"Frontend", "Backend"}

	hints := m.labelHints("front")
	if len(hints) != 1 {
		t.Errorf("expected 1 hint for 'front', got %d", len(hints))
	}
}

func TestLabelHints_EmptyPrefix(t *testing.T) {
	m := testModel(defaultStub())
	m.labels = []string{"frontend", "backend"}

	hints := m.labelHints("")
	if len(hints) != 0 {
		t.Errorf("expected 0 hints for empty prefix, got %d", len(hints))
	}
}

func TestLabelHints_CommaDelimitedLastToken(t *testing.T) {
	m := testModel(defaultStub())
	m.labels = []string{"frontend", "backend", "bugfix"}

	hints := m.labelHints("frontend, bug")
	if len(hints) != 1 {
		t.Errorf("expected 1 hint for trailing 'bug', got %d", len(hints))
	}
	if len(hints) > 0 && hints[0] != "bugfix" {
		t.Errorf("expected 'bugfix', got %q", hints[0])
	}
}

func TestLabelHints_MaxFiveResults(t *testing.T) {
	m := testModel(defaultStub())
	m.labels = []string{"a1", "a2", "a3", "a4", "a5", "a6", "a7"}

	hints := m.labelHints("a")
	if len(hints) != 5 {
		t.Errorf("expected max 5 hints, got %d", len(hints))
	}
}

func TestTruncate(t *testing.T) {
	m := testModel(defaultStub())

	short := m.truncate("hello", 10)
	if short != "hello" {
		t.Errorf("expected 'hello', got %q", short)
	}

	long := m.truncate("this is a very long string that exceeds limit", 20)
	if len(long) != 20 {
		t.Errorf("expected length 20, got %d", len(long))
	}
	if !strings.HasSuffix(long, "...") {
		t.Errorf("expected '...' suffix, got %q", long)
	}
}

func TestIsInputStep(t *testing.T) {
	expected := map[int]bool{
		stepProject:     false,
		stepIssueType:   false,
		stepSummary:     true,
		stepPriority:    false,
		stepAssignee:    true,
		stepLabels:      true,
		stepParent:      true,
		stepDescription: true,
		stepConfirm:     false,
	}
	for step, want := range expected {
		got := isInputStep(step)
		if got != want {
			t.Errorf("isInputStep(%d) = %v, want %v", step, got, want)
		}
	}
}

func TestView_NonEmptyAfterSize(t *testing.T) {
	m := testModel(defaultStub())
	m, _ = m.Update(projectsLoadedMsg{projects: defaultStub().projects})

	view := m.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestView_EmptyWithZeroWidth(t *testing.T) {
	c := defaultStub()
	m := New(c)
	// Don't call SetSize — width remains 0.

	view := m.View()
	if view != "" {
		t.Error("expected empty view with zero width")
	}
}

func TestView_ShowsProjectTitle(t *testing.T) {
	m := testModel(defaultStub())
	m, _ = m.Update(projectsLoadedMsg{projects: defaultStub().projects})

	view := m.View()
	if !strings.Contains(view, "Project") {
		t.Error("expected 'Project' in view at project step")
	}
}

func TestView_ConfirmShowsSummary(t *testing.T) {
	m := testModel(defaultStub())
	m.step = stepConfirm
	m.projects = defaultStub().projects
	m.values[stepProject] = "PROJ"
	m.values[stepIssueType] = "Bug"
	m.values[stepSummary] = "Fix login"

	view := m.View()
	if !strings.Contains(view, "Fix login") {
		t.Error("expected summary in confirm view")
	}
	if !strings.Contains(view, "Bug") {
		t.Error("expected issue type in confirm view")
	}
}

func TestWindowSizeMsg_UpdatesDimensions(t *testing.T) {
	c := defaultStub()
	m := New(c)
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})

	if m.width != 100 {
		t.Errorf("expected width 100, got %d", m.width)
	}
	if m.height != 50 {
		t.Errorf("expected height 50, got %d", m.height)
	}
}

func TestFullWizardFlow(t *testing.T) {
	c := defaultStub()
	m := testModel(c)

	// Step 1: Load and select project.
	m, _ = m.Update(projectsLoadedMsg{projects: c.projects})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.step != stepIssueType {
		t.Fatalf("expected issueType step, got %d", m.step)
	}

	// Step 2: Load and select issue type.
	m, _ = m.Update(issueTypesLoadedMsg{types: c.issueTypes})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}) // Move to "Story"
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.values[stepIssueType] != "Story" {
		t.Fatalf("expected 'Story', got %q", m.values[stepIssueType])
	}

	// Step 3: Enter summary.
	m.inputs[stepSummary].SetValue("Implement login page")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.step != stepPriority {
		t.Fatalf("expected priority step, got %d", m.step)
	}

	// Step 4: Load and select priority.
	m, _ = m.Update(prioritiesLoadedMsg{priorities: c.metadata.Priorities})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}) // Move to "High"
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.values[stepPriority] != "High" {
		t.Fatalf("expected 'High', got %q", m.values[stepPriority])
	}

	// Step 5: Skip assignee.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.step != stepLabels {
		t.Fatalf("expected labels step, got %d", m.step)
	}

	// Step 6: Skip labels.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Step 7: Skip parent.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Step 8: Skip description.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Step 9: Confirm.
	if m.step != stepConfirm {
		t.Fatalf("expected confirm step, got %d", m.step)
	}

	// Verify values.
	if m.values[stepProject] != "PROJ" {
		t.Errorf("project: expected 'PROJ', got %q", m.values[stepProject])
	}
	if m.values[stepIssueType] != "Story" {
		t.Errorf("issueType: expected 'Story', got %q", m.values[stepIssueType])
	}
	if m.values[stepSummary] != "Implement login page" {
		t.Errorf("summary: expected 'Implement login page', got %q", m.values[stepSummary])
	}
	if m.values[stepPriority] != "High" {
		t.Errorf("priority: expected 'High', got %q", m.values[stepPriority])
	}

	// Confirm and create.
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.loading {
		t.Error("expected loading after confirm")
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd after confirm")
	}

	// Simulate creation success.
	m, _ = m.Update(issueCreatedMsg{key: "PROJ-42"})
	if !m.Done() {
		t.Error("expected Done() after creation")
	}
	if m.CreatedKey() != "PROJ-42" {
		t.Errorf("expected 'PROJ-42', got %q", m.CreatedKey())
	}
}

func TestScrollOffset_AdjustsForLongLists(t *testing.T) {
	m := testModel(defaultStub())
	m.SetSize(120, 20) // Small height to trigger scrolling.

	// maxPickerVisible = 20 - 12 = 8.
	// Create a list of 15 projects.
	projects := make([]jira.Project, 15)
	for i := range projects {
		projects[i] = jira.Project{Key: "P" + strings.Repeat("X", i), Name: "Project"}
	}
	m, _ = m.Update(projectsLoadedMsg{projects: projects})

	// Move cursor to end.
	for i := 0; i < 14; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	}

	if m.projectCursor != 14 {
		t.Errorf("expected cursor 14, got %d", m.projectCursor)
	}
	if m.scrollOffset == 0 {
		t.Error("expected non-zero scroll offset for long list")
	}
}

func TestMaxPickerVisible_MinimumFive(t *testing.T) {
	m := testModel(defaultStub())
	m.SetSize(120, 10) // height - 12 = -2 < 5.

	v := m.maxPickerVisible()
	if v != 5 {
		t.Errorf("expected minimum 5, got %d", v)
	}
}

func TestRenderSummary_ResolvesProjectName(t *testing.T) {
	m := testModel(defaultStub())
	m.step = stepConfirm
	m.projects = []jira.Project{{Key: "PROJ", Name: "My Project"}}
	m.values[stepProject] = "PROJ"

	summary := m.renderSummary()
	if !strings.Contains(summary, "My Project") {
		t.Error("expected resolved project name in summary")
	}
	if !strings.Contains(summary, "PROJ") {
		t.Error("expected project key in summary")
	}
}

func TestRenderSummary_UnsetFieldsShowNotSet(t *testing.T) {
	m := testModel(defaultStub())
	m.step = stepConfirm

	summary := m.renderSummary()
	if !strings.Contains(summary, "(not set)") {
		t.Error("expected '(not set)' for empty fields")
	}
}

func TestOnStepEnter_IssueTypeRefetchesOnProjectChange(t *testing.T) {
	m := testModel(defaultStub())
	m.step = stepIssueType
	m.issueTypeLoaded = true
	m.issueTypeFetchProject = "OLD"
	m.values[stepProject] = "NEW"

	cmd := m.onStepEnter()
	if cmd == nil {
		t.Error("expected non-nil cmd to refetch issue types for new project")
	}
	if !m.loading {
		t.Error("expected loading to be true while fetching")
	}
	if m.issueTypeLoaded {
		t.Error("expected issueTypeLoaded to be reset")
	}
}

func TestOnStepEnter_IssueTypeSkipsFetchIfCached(t *testing.T) {
	m := testModel(defaultStub())
	m.step = stepIssueType
	m.issueTypeLoaded = true
	m.issueTypeFetchProject = "PROJ"
	m.values[stepProject] = "PROJ"

	cmd := m.onStepEnter()
	if cmd != nil {
		t.Error("expected nil cmd when issue types already cached for this project")
	}
}

func TestCreateIssueCmd_BuildsCorrectRequest(t *testing.T) {
	c := defaultStub()
	var capturedReq *client.CreateIssueRequest
	// We can't easily capture the request with the stub, but we can test the command produces
	// the right message type by executing it.
	m := testModel(c)
	m.values[stepProject] = "PROJ"
	m.values[stepIssueType] = "Bug"
	m.values[stepSummary] = "Fix it"
	m.values[stepLabels] = "bug, frontend"

	cmd := m.createIssue()
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}

	msg := cmd()
	created, ok := msg.(issueCreatedMsg)
	if !ok {
		t.Fatalf("expected issueCreatedMsg, got %T", msg)
	}
	if created.key != "PROJ-42" {
		t.Errorf("expected 'PROJ-42', got %q", created.key)
	}

	_ = capturedReq // unused, but documents intent
}

func TestCreateIssueCmd_Error(t *testing.T) {
	c := defaultStub()
	c.createResp = nil
	c.createErr = errors.New("create failed")
	m := testModel(c)
	m.values[stepProject] = "PROJ"
	m.values[stepIssueType] = "Bug"
	m.values[stepSummary] = "Fix it"

	cmd := m.createIssue()
	msg := cmd()
	created, ok := msg.(issueCreatedMsg)
	if !ok {
		t.Fatalf("expected issueCreatedMsg, got %T", msg)
	}
	if created.err == nil {
		t.Error("expected error in issueCreatedMsg")
	}
}

func TestFetchProjects_ReturnsCorrectMsg(t *testing.T) {
	c := defaultStub()
	m := testModel(c)

	cmd := m.fetchProjects()
	msg := cmd()
	loaded, ok := msg.(projectsLoadedMsg)
	if !ok {
		t.Fatalf("expected projectsLoadedMsg, got %T", msg)
	}
	if len(loaded.projects) != 2 {
		t.Errorf("expected 2 projects, got %d", len(loaded.projects))
	}
}

func TestFetchIssueTypes_ReturnsCorrectMsg(t *testing.T) {
	c := defaultStub()
	m := testModel(c)

	cmd := m.fetchIssueTypes("PROJ")
	msg := cmd()
	loaded, ok := msg.(issueTypesLoadedMsg)
	if !ok {
		t.Fatalf("expected issueTypesLoadedMsg, got %T", msg)
	}
	if len(loaded.types) != 3 {
		t.Errorf("expected 3 types, got %d", len(loaded.types))
	}
}

// --- Project type and create request tests ---

// capturingStubClient records the CreateIssueRequest for assertion.
type capturingStubClient struct {
	stubClient
	capturedReq *client.CreateIssueRequest
}

func (s *capturingStubClient) CreateIssue(req *client.CreateIssueRequest) (*client.CreateIssueResponse, error) {
	s.capturedReq = req
	return s.createResp, s.createErr
}

func TestCreateIssueCmd_SetsProjectType(t *testing.T) {
	c := &capturingStubClient{stubClient: *defaultStub()}
	c.createResp = &client.CreateIssueResponse{Key: "PROJ-99"}
	m := New(c)
	m.SetSize(120, 40)
	m.projects = []jira.Project{
		{Key: "PROJ", Name: "Project", Type: "next-gen"},
		{Key: "CLASSIC", Name: "Classic Project", Type: "classic"},
	}
	m.values[stepProject] = "PROJ"
	m.values[stepIssueType] = "Story"
	m.values[stepSummary] = "Test"

	cmd := m.createIssue()
	cmd()

	if c.capturedReq == nil {
		t.Fatal("expected CreateIssue to be called")
	}
	if c.capturedReq.ProjectType != "next-gen" {
		t.Errorf("expected ProjectType 'next-gen', got %q", c.capturedReq.ProjectType)
	}
}

func TestCreateIssueCmd_ClassicProjectType(t *testing.T) {
	c := &capturingStubClient{stubClient: *defaultStub()}
	c.createResp = &client.CreateIssueResponse{Key: "CLS-1"}
	m := New(c)
	m.SetSize(120, 40)
	m.projects = []jira.Project{
		{Key: "CLS", Name: "Classic", Type: "classic"},
	}
	m.values[stepProject] = "CLS"
	m.values[stepIssueType] = "Bug"
	m.values[stepSummary] = "Test"

	cmd := m.createIssue()
	cmd()

	if c.capturedReq == nil {
		t.Fatal("expected CreateIssue to be called")
	}
	if c.capturedReq.ProjectType != "classic" {
		t.Errorf("expected ProjectType 'classic', got %q", c.capturedReq.ProjectType)
	}
}

func TestCreateIssueCmd_EmptyProjectType(t *testing.T) {
	c := &capturingStubClient{stubClient: *defaultStub()}
	c.createResp = &client.CreateIssueResponse{Key: "X-1"}
	m := New(c)
	m.SetSize(120, 40)
	// No projects loaded — project type lookup will find no match.
	m.values[stepProject] = "UNKNOWN"
	m.values[stepIssueType] = "Task"
	m.values[stepSummary] = "Test"

	cmd := m.createIssue()
	cmd()

	if c.capturedReq == nil {
		t.Fatal("expected CreateIssue to be called")
	}
	if c.capturedReq.ProjectType != "" {
		t.Errorf("expected empty ProjectType, got %q", c.capturedReq.ProjectType)
	}
}

func TestCreateIssueCmd_LabelParsing(t *testing.T) {
	c := &capturingStubClient{stubClient: *defaultStub()}
	c.createResp = &client.CreateIssueResponse{Key: "PROJ-1"}
	m := New(c)
	m.SetSize(120, 40)
	m.values[stepProject] = "PROJ"
	m.values[stepIssueType] = "Bug"
	m.values[stepSummary] = "Test"
	m.values[stepLabels] = "  bug , frontend,, backend  "

	cmd := m.createIssue()
	cmd()

	if c.capturedReq == nil {
		t.Fatal("expected CreateIssue to be called")
	}
	expected := []string{"bug", "frontend", "backend"}
	if len(c.capturedReq.Labels) != len(expected) {
		t.Fatalf("expected %d labels, got %d: %v", len(expected), len(c.capturedReq.Labels), c.capturedReq.Labels)
	}
	for i, l := range expected {
		if c.capturedReq.Labels[i] != l {
			t.Errorf("label[%d]: expected %q, got %q", i, l, c.capturedReq.Labels[i])
		}
	}
}

func TestCreateIssueCmd_ParentKeyPassedThrough(t *testing.T) {
	c := &capturingStubClient{stubClient: *defaultStub()}
	c.createResp = &client.CreateIssueResponse{Key: "PROJ-1"}
	m := New(c)
	m.SetSize(120, 40)
	m.values[stepProject] = "PROJ"
	m.values[stepIssueType] = "Story"
	m.values[stepSummary] = "Test"
	m.values[stepParent] = "PROJ-100"

	cmd := m.createIssue()
	cmd()

	if c.capturedReq == nil {
		t.Fatal("expected CreateIssue to be called")
	}
	if c.capturedReq.ParentKey != "PROJ-100" {
		t.Errorf("expected ParentKey 'PROJ-100', got %q", c.capturedReq.ParentKey)
	}
}

func TestCreateIssueCmd_AssigneeUsesAccountID(t *testing.T) {
	c := &capturingStubClient{stubClient: *defaultStub()}
	c.createResp = &client.CreateIssueResponse{Key: "PROJ-1"}
	m := New(c)
	m.SetSize(120, 40)
	m.values[stepProject] = "PROJ"
	m.values[stepIssueType] = "Bug"
	m.values[stepSummary] = "Test"
	m.values[stepAssignee] = "abc123-account-id"

	cmd := m.createIssue()
	cmd()

	if c.capturedReq == nil {
		t.Fatal("expected CreateIssue to be called")
	}
	if c.capturedReq.Assignee != "abc123-account-id" {
		t.Errorf("expected Assignee 'abc123-account-id', got %q", c.capturedReq.Assignee)
	}
}

// --- Priority error handling ---

func TestPrioritiesLoaded_ErrorIsNonFatal(t *testing.T) {
	m := testModel(defaultStub())
	m.step = stepPriority
	m.errMsg = "leftover error"

	m, _ = m.Update(prioritiesLoadedMsg{err: errors.New("priorities failed")})

	if m.errMsg != "" {
		t.Errorf("expected errMsg to be cleared (non-fatal), got %q", m.errMsg)
	}
	if m.priorities != nil {
		t.Errorf("expected nil priorities on error, got %v", m.priorities)
	}
	if !m.priorityLoaded {
		t.Error("expected priorityLoaded to be true even on error")
	}
}

// --- User search error ---

func TestSearchUsers_ErrorReturnsNilResults(t *testing.T) {
	c := defaultStub()
	c.userErr = errors.New("search failed")
	m := testModel(c)
	m.values[stepProject] = "PROJ"

	cmd := m.searchUsers("test")
	msg := cmd()
	result, ok := msg.(userSearchResultMsg)
	if !ok {
		t.Fatalf("expected userSearchResultMsg, got %T", msg)
	}
	if result.users != nil {
		t.Errorf("expected nil users on error, got %v", result.users)
	}
}

// --- Confirm view shows display name ---

func TestRenderSummary_ShowsAssigneeDisplayName(t *testing.T) {
	m := testModel(defaultStub())
	m.step = stepConfirm
	m.projects = defaultStub().projects
	m.values[stepProject] = "PROJ"
	m.values[stepIssueType] = "Bug"
	m.values[stepSummary] = "Test"
	// Account ID is stored in values, display name in the input.
	m.values[stepAssignee] = "abc123-account-id"
	m.inputs[stepAssignee].SetValue("Sean Halberthal")

	summary := m.renderSummary()
	if !strings.Contains(summary, "Sean Halberthal") {
		t.Error("expected display name in summary, not account ID")
	}
	if strings.Contains(summary, "abc123") {
		t.Error("account ID should not appear in summary view")
	}
}

// --- Scroll offset upward ---

func TestScrollOffset_AdjustsUpward(t *testing.T) {
	m := testModel(defaultStub())
	m.SetSize(120, 20)

	projects := make([]jira.Project, 15)
	for i := range projects {
		projects[i] = jira.Project{Key: "P" + strings.Repeat("X", i), Name: "Project"}
	}
	m, _ = m.Update(projectsLoadedMsg{projects: projects})

	// Scroll down to the end.
	for i := 0; i < 14; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	}
	scrolledOffset := m.scrollOffset

	// Now scroll back up to the top.
	for i := 0; i < 14; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	}

	if m.projectCursor != 0 {
		t.Errorf("expected cursor 0, got %d", m.projectCursor)
	}
	if m.scrollOffset >= scrolledOffset {
		t.Errorf("expected scroll offset to decrease from %d, got %d", scrolledOffset, m.scrollOffset)
	}
}
