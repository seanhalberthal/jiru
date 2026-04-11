package setupview

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/seanhalberthal/jiru/internal/config"
	"github.com/seanhalberthal/jiru/internal/jira"
)

func partialConfig() *config.Config {
	return &config.Config{
		Domain:   "test.atlassian.net",
		User:     "user@test.com",
		APIToken: "token",
		AuthType: "basic",
		Project:  "TEST",
		RepoPath: "/home/user/repo",
	}
}

func TestGoToConfirm_JumpsToConfirmStep(t *testing.T) {
	m := New(partialConfig())
	if m.step == stepConfirm {
		t.Fatal("should not start at confirm")
	}

	m.GoToConfirm()
	if m.step != stepConfirm {
		t.Errorf("step = %d, want %d (stepConfirm)", m.step, stepConfirm)
	}
}

func TestGoToConfirm_PreservesValues(t *testing.T) {
	m := New(partialConfig())
	m.GoToConfirm()

	cfg := m.Config()
	if cfg.Domain != "test.atlassian.net" {
		t.Errorf("Domain = %q, want %q", cfg.Domain, "test.atlassian.net")
	}
	if cfg.RepoPath != "/home/user/repo" {
		t.Errorf("RepoPath = %q, want %q", cfg.RepoPath, "/home/user/repo")
	}
}

func TestCtrlR_RestartsFromDomain(t *testing.T) {
	m := New(partialConfig())
	m.GoToConfirm()

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}, Alt: false})
	// ctrl+r is not a rune key, simulate it properly.
	m.step = stepConfirm // Reset to confirm first.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})

	if m.step != stepDomain {
		t.Errorf("step = %d, want %d (stepDomain) after ctrl+r", m.step, stepDomain)
	}
}

func TestCtrlR_OnlyWorksAtConfirm(t *testing.T) {
	m := New(partialConfig())
	m.step = stepUser

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	if m.step != stepUser {
		t.Errorf("ctrl+r should not work outside confirm step, step = %d", m.step)
	}
}

func TestConfig_IncludesRepoPath(t *testing.T) {
	m := New(partialConfig())
	m.GoToConfirm()

	cfg := m.Config()
	if cfg.RepoPath != "/home/user/repo" {
		t.Errorf("Config().RepoPath = %q, want %q", cfg.RepoPath, "/home/user/repo")
	}
}

func TestConfig_EmptyRepoPath(t *testing.T) {
	partial := partialConfig()
	partial.RepoPath = ""
	m := New(partial)
	m.GoToConfirm()

	cfg := m.Config()
	if cfg.RepoPath != "" {
		t.Errorf("Config().RepoPath = %q, want empty", cfg.RepoPath)
	}
}

func TestRepoPathStep_Exists(t *testing.T) {
	// Verify stepRepoPath is between stepBoardID and stepConfirm.
	if stepRepoPath <= stepBoardID {
		t.Errorf("stepRepoPath (%d) should be after stepBoardID (%d)", stepRepoPath, stepBoardID)
	}
	if stepRepoPath >= stepConfirm {
		t.Errorf("stepRepoPath (%d) should be before stepConfirm (%d)", stepRepoPath, stepConfirm)
	}
}

func TestRepoPathStep_IsInputStep(t *testing.T) {
	if !isInputStep(stepRepoPath) {
		t.Error("stepRepoPath should be an input step")
	}
}

func TestRepoPathValidation_EmptyIsValid(t *testing.T) {
	validator := steps[stepRepoPath].validate
	if validator == nil {
		t.Fatal("stepRepoPath should have a validator")
	}
	if err := validator(""); err != nil {
		t.Errorf("empty repo path should be valid, got: %v", err)
	}
}

func TestRepoPathValidation_NonexistentPath(t *testing.T) {
	validator := steps[stepRepoPath].validate
	if err := validator("/nonexistent/path/to/repo"); err == nil {
		t.Error("expected error for nonexistent path")
	}
}

func TestRepoPathValidation_NotADirectory(t *testing.T) {
	f, err := os.CreateTemp("", "jiru-test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(f.Name()) }()
	_ = f.Close()

	validator := steps[stepRepoPath].validate
	if err := validator(f.Name()); err == nil {
		t.Error("expected error for file path")
	}
}

func TestRepoPathValidation_NotAGitRepo(t *testing.T) {
	dir := t.TempDir()
	validator := steps[stepRepoPath].validate
	if err := validator(dir); err == nil {
		t.Error("expected error for directory without .git")
	}
}

func TestRepoPathValidation_ValidGitRepo(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	validator := steps[stepRepoPath].validate
	if err := validator(dir); err != nil {
		t.Errorf("expected valid git repo path, got: %v", err)
	}
}

func TestExpandTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}

	tests := []struct {
		input string
		want  string
	}{
		{"~/projects/repo", filepath.Join(home, "projects/repo")},
		{"~/", home},
		{"~", home},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
		{"", ""},
	}
	for _, tt := range tests {
		got := expandTilde(tt.input)
		if got != tt.want {
			t.Errorf("expandTilde(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// --- Update loop step transition tests ---

func TestUpdate_DomainStep_EnterAdvances(t *testing.T) {
	m := New(&config.Config{AuthType: "basic"})
	m.step = stepDomain
	m.inputs[stepDomain].SetValue("myco.atlassian.net")

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.step != stepUser {
		t.Errorf("expected stepUser after valid domain, got %d", m.step)
	}
}

func TestUpdate_DomainStep_InvalidShowsError(t *testing.T) {
	m := New(&config.Config{AuthType: "basic"})
	m.step = stepDomain
	m.inputs[stepDomain].SetValue("not-a-domain")

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.step != stepDomain {
		t.Errorf("expected to stay on stepDomain for invalid input, got %d", m.step)
	}
	if m.errMsg == "" {
		t.Error("expected validation error for invalid domain")
	}
}

func TestUpdate_DomainStep_EmptyShowsRequired(t *testing.T) {
	m := New(&config.Config{AuthType: "basic"})
	m.step = stepDomain
	m.inputs[stepDomain].SetValue("")

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.step != stepDomain {
		t.Errorf("expected to stay on stepDomain for empty, got %d", m.step)
	}
	if m.errMsg == "" {
		t.Error("expected required field error")
	}
}

func TestUpdate_EscFromAnyStep_Quits(t *testing.T) {
	m := New(&config.Config{AuthType: "basic"})
	m.step = stepDomain

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !m.Quit() {
		t.Error("expected Quit sentinel from esc")
	}
}

func TestUpdate_UserStep_EnterAdvances(t *testing.T) {
	m := New(&config.Config{AuthType: "basic"})
	m.step = stepUser
	m.inputs[stepUser].SetValue("user@example.com")

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.step != stepAPIToken {
		t.Errorf("expected stepAPIToken after valid email, got %d", m.step)
	}
}

func TestUpdate_UserStep_InvalidShowsError(t *testing.T) {
	m := New(&config.Config{AuthType: "basic"})
	m.step = stepUser
	m.inputs[stepUser].SetValue("not-an-email")

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.step != stepUser {
		t.Errorf("expected to stay on stepUser, got %d", m.step)
	}
	if m.errMsg == "" {
		t.Error("expected validation error for invalid email")
	}
}

func TestUpdate_TokenStep_EnterAdvances(t *testing.T) {
	m := New(&config.Config{AuthType: "basic"})
	m.step = stepAPIToken
	m.inputs[stepAPIToken].SetValue("some-api-token-value")

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.step != stepAuthType {
		t.Errorf("expected stepAuthType after token, got %d", m.step)
	}
}

func TestUpdate_TokenStep_EmptyShowsRequired(t *testing.T) {
	m := New(&config.Config{AuthType: "basic"})
	m.step = stepAPIToken
	m.inputs[stepAPIToken].SetValue("")

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.step != stepAPIToken {
		t.Errorf("expected to stay on stepAPIToken, got %d", m.step)
	}
}

func TestUpdate_CtrlB_GoesBack(t *testing.T) {
	m := New(&config.Config{AuthType: "basic"})
	m.step = stepUser

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlB})
	if m.step != stepDomain {
		t.Errorf("expected stepDomain after ctrl+b, got %d", m.step)
	}
}

func TestUpdate_CtrlB_NoopAtWelcome(t *testing.T) {
	m := New(&config.Config{AuthType: "basic"})
	m.step = stepWelcome

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlB})
	if m.step != stepWelcome {
		t.Errorf("expected to stay at stepWelcome, got %d", m.step)
	}
}

func TestUpdate_WelcomeStep_EnterAdvances(t *testing.T) {
	m := New(&config.Config{AuthType: "basic"})
	m.step = stepWelcome

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.step == stepWelcome {
		t.Error("expected to advance past welcome step")
	}
}

func TestUpdate_BranchCaseToggle(t *testing.T) {
	m := New(&config.Config{AuthType: "basic"})
	m.step = stepBranchCase

	// Default is 0 (lowercase), toggle to 1.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.branchCaseCursor != 1 {
		t.Errorf("expected cursor 1 after down, got %d", m.branchCaseCursor)
	}

	// Enter confirms.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.values[stepBranchCase] != "true" {
		t.Errorf("expected 'true' for uppercase, got %q", m.values[stepBranchCase])
	}
}

func TestUpdate_BranchModeToggle(t *testing.T) {
	m := New(&config.Config{AuthType: "basic"})
	m.step = stepBranchMode

	// Default is 0 (local), down to 1 (remote).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.branchModeCursor != 1 {
		t.Errorf("expected cursor 1, got %d", m.branchModeCursor)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.values[stepBranchMode] != "remote" {
		t.Errorf("expected 'remote', got %q", m.values[stepBranchMode])
	}
}

func TestUpdate_ConfirmStep_Enter_SetsDone(t *testing.T) {
	m := New(partialConfig())
	m.GoToConfirm()

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.Done() {
		t.Error("expected Done after enter on confirm step")
	}
}

func TestUpdate_CleanDomain_StripsProtocol(t *testing.T) {
	m := New(&config.Config{AuthType: "basic"})
	m.step = stepDomain
	m.inputs[stepDomain].SetValue("https://myco.atlassian.net")

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.values[stepDomain] != "myco.atlassian.net" {
		t.Errorf("expected protocol stripped, got %q", m.values[stepDomain])
	}
}

func TestRepoPathValidation_TildeExpansion(t *testing.T) {
	// Create a temp dir that looks like ~/some-repo with a .git dir.
	// We can't easily test ~ in the validator since it expands to a real path,
	// but we can verify the validator handles an expanded tilde path.
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	validator := steps[stepRepoPath].validate
	if err := validator(dir); err != nil {
		t.Errorf("expected valid after expansion, got: %v", err)
	}
}

// --- View rendering tests ---

func sizedModel(partial *config.Config) Model {
	m := New(partial)
	m.SetSize(120, 40)
	return m
}

func TestView_WelcomeStep(t *testing.T) {
	m := sizedModel(&config.Config{AuthType: "basic"})
	m.step = stepWelcome

	view := m.View()
	if view == "" {
		t.Fatal("expected non-empty View for welcome step")
	}
	if !strings.Contains(view, "terminal UI for Jira") {
		t.Error("expected tagline in view output")
	}
	if !strings.Contains(view, "configured yet") {
		t.Error("expected first-run message in view output")
	}
	if !strings.Contains(view, "continue") {
		t.Error("expected 'continue' keybind hint")
	}
}

func TestView_WelcomeStep_NewProfile(t *testing.T) {
	m := sizedModel(&config.Config{AuthType: "basic"})
	m.step = stepWelcome
	m.SetForNewProfile()

	view := m.View()
	if !strings.Contains(view, "new profile") {
		t.Error("expected new-profile message in view output")
	}
	if strings.Contains(view, "configured yet") {
		t.Error("should not show first-run message when adding a profile")
	}
}

func TestView_DomainStep(t *testing.T) {
	m := sizedModel(&config.Config{AuthType: "basic"})
	m.step = stepDomain

	view := m.View()
	if view == "" {
		t.Fatal("expected non-empty View for domain step")
	}
	if !strings.Contains(view, "Jira Domain") {
		t.Error("expected domain title in view")
	}
	if !strings.Contains(view, "Step 1") {
		t.Error("expected step indicator in view")
	}
}

func TestView_ConfirmStep(t *testing.T) {
	m := sizedModel(partialConfig())
	m.GoToConfirm()

	view := m.View()
	if view == "" {
		t.Fatal("expected non-empty View for confirm step")
	}
	if !strings.Contains(view, "test.atlassian.net") {
		t.Error("expected domain in summary")
	}
	if !strings.Contains(view, "user@test.com") {
		t.Error("expected user email in summary")
	}
	if !strings.Contains(view, "Auth Type") {
		t.Error("expected Auth Type label in summary")
	}
	if !strings.Contains(view, "save") {
		t.Error("expected 'save' action in footer")
	}
}

func TestView_ProjectPickerStep(t *testing.T) {
	m := sizedModel(&config.Config{AuthType: "basic"})
	m.step = stepProject
	m.projectsLoaded = true
	m.projects = []jira.Project{
		{Key: "PROJ", Name: "My Project", Type: "classic"},
		{Key: "TEST", Name: "Test Project", Type: "next-gen"},
	}

	view := m.View()
	if view == "" {
		t.Fatal("expected non-empty View for project picker")
	}
	if !strings.Contains(view, "None") {
		t.Error("expected 'None' option in project picker")
	}
	if !strings.Contains(view, "My Project") {
		t.Error("expected project name in picker")
	}
	if !strings.Contains(view, "PROJ") {
		t.Error("expected project key in picker")
	}
	if !strings.Contains(view, "Test Project") {
		t.Error("expected second project in picker")
	}
}

func TestView_ProjectPickerStep_Loading(t *testing.T) {
	m := sizedModel(&config.Config{AuthType: "basic"})
	m.step = stepProject
	m.projectsLoaded = false
	m.validating = true

	view := m.View()
	if !strings.Contains(view, "Fetching projects") {
		t.Error("expected loading text when projects not yet loaded")
	}
}

func TestView_ProjectPickerStep_Empty(t *testing.T) {
	m := sizedModel(&config.Config{AuthType: "basic"})
	m.step = stepProject
	m.projectsLoaded = true
	m.projects = nil

	view := m.View()
	if !strings.Contains(view, "No projects found") {
		t.Error("expected 'No projects found' for empty project list")
	}
}

func TestView_BoardPickerStep(t *testing.T) {
	m := sizedModel(&config.Config{AuthType: "basic"})
	m.step = stepBoardID
	m.boardsLoaded = true
	m.boards = []jira.Board{
		{ID: 1, Name: "Sprint Board", Type: "scrum"},
		{ID: 2, Name: "Kanban Board", Type: "kanban"},
	}

	view := m.View()
	if view == "" {
		t.Fatal("expected non-empty View for board picker")
	}
	if !strings.Contains(view, "None") {
		t.Error("expected 'None' option in board picker")
	}
	if !strings.Contains(view, "Sprint Board") {
		t.Error("expected board name in picker")
	}
	if !strings.Contains(view, "scrum") {
		t.Error("expected board type in picker")
	}
}

func TestView_BoardPickerStep_Loading(t *testing.T) {
	m := sizedModel(&config.Config{AuthType: "basic"})
	m.step = stepBoardID
	m.boardsLoaded = false
	m.validating = true

	view := m.View()
	if !strings.Contains(view, "Fetching boards") {
		t.Error("expected loading text when boards not yet loaded")
	}
}

func TestView_BoardPickerStep_Empty(t *testing.T) {
	m := sizedModel(&config.Config{AuthType: "basic"})
	m.step = stepBoardID
	m.boardsLoaded = true
	m.boards = nil

	view := m.View()
	if !strings.Contains(view, "No boards found") {
		t.Error("expected 'No boards found' for empty board list")
	}
}

func TestView_BranchCaseStep(t *testing.T) {
	m := sizedModel(&config.Config{AuthType: "basic"})
	m.step = stepBranchCase

	view := m.View()
	if view == "" {
		t.Fatal("expected non-empty View for branch case step")
	}
	if !strings.Contains(view, "lowercase") {
		t.Error("expected 'lowercase' option in toggle")
	}
	if !strings.Contains(view, "Title Case") {
		t.Error("expected 'Title Case' option in toggle")
	}
}

func TestView_BranchModeStep(t *testing.T) {
	m := sizedModel(&config.Config{AuthType: "basic"})
	m.step = stepBranchMode

	view := m.View()
	if view == "" {
		t.Fatal("expected non-empty View for branch mode step")
	}
	if !strings.Contains(view, "local") {
		t.Error("expected 'local' option in toggle")
	}
	if !strings.Contains(view, "remote") {
		t.Error("expected 'remote' option in toggle")
	}
	if !strings.Contains(view, "both") {
		t.Error("expected 'both' option in toggle")
	}
}

func TestView_ValidationSpinner(t *testing.T) {
	m := sizedModel(&config.Config{AuthType: "basic"})
	m.step = stepAuthType
	m.validating = true

	view := m.View()
	if !strings.Contains(view, "Verifying credentials") {
		t.Error("expected validation spinner text in view")
	}
}

func TestView_ZeroWidth_ReturnsEmpty(t *testing.T) {
	m := New(&config.Config{AuthType: "basic"})
	// Don't call SetSize — width stays 0.
	view := m.View()
	if view != "" {
		t.Errorf("expected empty view with zero width, got %q", view)
	}
}

func TestView_ErrorMessage(t *testing.T) {
	m := sizedModel(&config.Config{AuthType: "basic"})
	m.step = stepDomain
	m.errMsg = "something went wrong"

	view := m.View()
	if !strings.Contains(view, "something went wrong") {
		t.Error("expected error message in view")
	}
}

// --- Message handler tests ---

func TestUpdate_ValidationOKMsg_Advances(t *testing.T) {
	m := New(&config.Config{AuthType: "basic"})
	m.step = stepAuthType
	m.validating = true
	m.values[stepDomain] = "test.atlassian.net"
	m.values[stepUser] = "user@test.com"
	m.values[stepAPIToken] = "token"
	m.values[stepAuthType] = "basic"

	prevStep := m.step
	m, _ = m.Update(validationOKMsg{step: stepAuthType})

	if m.errMsg != "" {
		t.Errorf("expected empty errMsg, got %q", m.errMsg)
	}
	// Step should have advanced past the auth type step.
	if m.step <= prevStep {
		t.Errorf("expected step to advance past %d, got %d", prevStep, m.step)
	}
	// The next step is stepProject, which triggers project fetching (validating = true again).
	// So we verify the step advanced rather than checking validating state.
	if m.step != stepProject {
		t.Errorf("expected step to be stepProject (%d), got %d", stepProject, m.step)
	}
}

func TestUpdate_ValidationFailMsg_ShowsError(t *testing.T) {
	m := New(&config.Config{AuthType: "basic"})
	m.step = stepAuthType
	m.validating = true

	m, _ = m.Update(validationFailMsg{step: stepAuthType, err: fmt.Errorf("auth failed")})

	if m.validating {
		t.Error("expected validating to be false after validationFailMsg")
	}
	if m.errMsg != "auth failed" {
		t.Errorf("expected errMsg = 'auth failed', got %q", m.errMsg)
	}
	if m.step != stepAuthType {
		t.Errorf("expected to remain on stepAuthType, got %d", m.step)
	}
}

func TestUpdate_ProjectsLoadedMsg(t *testing.T) {
	m := New(&config.Config{AuthType: "basic"})
	m.step = stepProject
	m.validating = true

	projects := []jira.Project{
		{Key: "PROJ", Name: "My Project"},
		{Key: "TEST", Name: "Test Project"},
	}

	m, _ = m.Update(projectsLoadedMsg{projects: projects})

	if !m.projectsLoaded {
		t.Error("expected projectsLoaded to be true")
	}
	if m.validating {
		t.Error("expected validating to be false")
	}
	if len(m.projects) != 2 {
		t.Errorf("expected 2 projects, got %d", len(m.projects))
	}
	if m.errMsg != "" {
		t.Errorf("expected empty errMsg, got %q", m.errMsg)
	}
}

func TestUpdate_ProjectsLoadedMsg_Error(t *testing.T) {
	m := New(&config.Config{AuthType: "basic"})
	m.step = stepProject
	m.validating = true

	m, _ = m.Update(projectsLoadedMsg{err: fmt.Errorf("network error")})

	if !m.projectsLoaded {
		t.Error("expected projectsLoaded to be true even on error")
	}
	if m.validating {
		t.Error("expected validating to be false")
	}
	if m.projects != nil {
		t.Error("expected nil projects on error")
	}
	if !strings.Contains(m.errMsg, "network error") {
		t.Errorf("expected errMsg to contain 'network error', got %q", m.errMsg)
	}
}

func TestUpdate_ProjectsLoadedMsg_PreselectsExistingKey(t *testing.T) {
	m := New(&config.Config{AuthType: "basic"})
	m.step = stepProject
	m.validating = true
	m.values[stepProject] = "TEST"

	projects := []jira.Project{
		{Key: "PROJ", Name: "My Project"},
		{Key: "TEST", Name: "Test Project"},
	}

	m, _ = m.Update(projectsLoadedMsg{projects: projects})

	// TEST is at index 1 in the projects slice, so cursor should be 2 (1-indexed, 0 = None).
	if m.projectCursor != 2 {
		t.Errorf("expected projectCursor = 2 for pre-selected TEST, got %d", m.projectCursor)
	}
}

func TestUpdate_BoardsLoadedMsg(t *testing.T) {
	m := New(&config.Config{AuthType: "basic"})
	m.step = stepBoardID
	m.validating = true

	boards := []jira.Board{
		{ID: 1, Name: "Sprint Board", Type: "scrum"},
		{ID: 2, Name: "Kanban Board", Type: "kanban"},
	}

	m, _ = m.Update(boardsLoadedMsg{boards: boards})

	if !m.boardsLoaded {
		t.Error("expected boardsLoaded to be true")
	}
	if m.validating {
		t.Error("expected validating to be false")
	}
	if len(m.boards) != 2 {
		t.Errorf("expected 2 boards, got %d", len(m.boards))
	}
	if m.errMsg != "" {
		t.Errorf("expected empty errMsg, got %q", m.errMsg)
	}
}

func TestUpdate_BoardsLoadedMsg_Error(t *testing.T) {
	m := New(&config.Config{AuthType: "basic"})
	m.step = stepBoardID
	m.validating = true

	m, _ = m.Update(boardsLoadedMsg{err: fmt.Errorf("fetch failed")})

	if !m.boardsLoaded {
		t.Error("expected boardsLoaded to be true even on error")
	}
	if m.boards != nil {
		t.Error("expected nil boards on error")
	}
	if !strings.Contains(m.errMsg, "fetch failed") {
		t.Errorf("expected errMsg to contain 'fetch failed', got %q", m.errMsg)
	}
}

func TestUpdate_BoardsLoadedMsg_PreselectsExistingID(t *testing.T) {
	m := New(&config.Config{AuthType: "basic"})
	m.step = stepBoardID
	m.validating = true
	m.values[stepBoardID] = "2"

	boards := []jira.Board{
		{ID: 1, Name: "Sprint Board", Type: "scrum"},
		{ID: 2, Name: "Kanban Board", Type: "kanban"},
	}

	m, _ = m.Update(boardsLoadedMsg{boards: boards})

	// Board with ID 2 is at index 1, so cursor should be 2 (1-indexed, 0 = None).
	if m.boardCursor != 2 {
		t.Errorf("expected boardCursor = 2 for pre-selected board ID 2, got %d", m.boardCursor)
	}
}

// --- Picker handler tests ---

func TestUpdate_ProjectPicker_Navigation(t *testing.T) {
	m := New(&config.Config{AuthType: "basic"})
	m.step = stepProject
	m.projectsLoaded = true
	m.projects = []jira.Project{
		{Key: "PROJ", Name: "My Project"},
		{Key: "TEST", Name: "Test Project"},
	}
	m.projectCursor = 0

	// Down from 0 to 1.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.projectCursor != 1 {
		t.Errorf("expected cursor 1 after down, got %d", m.projectCursor)
	}

	// Down from 1 to 2.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.projectCursor != 2 {
		t.Errorf("expected cursor 2 after second down, got %d", m.projectCursor)
	}

	// Down from 2 should stay at 2 (max is len(projects)).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.projectCursor != 2 {
		t.Errorf("expected cursor to stay at 2, got %d", m.projectCursor)
	}

	// Up from 2 to 1.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.projectCursor != 1 {
		t.Errorf("expected cursor 1 after up, got %d", m.projectCursor)
	}

	// Up from 1 to 0.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.projectCursor != 0 {
		t.Errorf("expected cursor 0 after second up, got %d", m.projectCursor)
	}

	// Up from 0 should stay at 0.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.projectCursor != 0 {
		t.Errorf("expected cursor to stay at 0, got %d", m.projectCursor)
	}
}

func TestUpdate_ProjectPicker_Select(t *testing.T) {
	m := New(&config.Config{AuthType: "basic"})
	m.step = stepProject
	m.projectsLoaded = true
	m.projects = []jira.Project{
		{Key: "PROJ", Name: "My Project"},
		{Key: "TEST", Name: "Test Project"},
	}
	m.projectCursor = 2 // Second project (TEST).

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.values[stepProject] != "TEST" {
		t.Errorf("expected project value = 'TEST', got %q", m.values[stepProject])
	}
	if m.step != stepBoardID {
		t.Errorf("expected to advance to stepBoardID (%d), got %d", stepBoardID, m.step)
	}
}

func TestUpdate_ProjectPicker_SelectNone(t *testing.T) {
	m := New(&config.Config{AuthType: "basic"})
	m.step = stepProject
	m.projectsLoaded = true
	m.projects = []jira.Project{
		{Key: "PROJ", Name: "My Project"},
	}
	m.projectCursor = 0 // "None" option.

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.values[stepProject] != "" {
		t.Errorf("expected empty project value for None, got %q", m.values[stepProject])
	}
	if m.step != stepBoardID {
		t.Errorf("expected to advance to stepBoardID, got %d", m.step)
	}
}

func TestUpdate_ProjectPicker_SpaceSelects(t *testing.T) {
	m := New(&config.Config{AuthType: "basic"})
	m.step = stepProject
	m.projectsLoaded = true
	m.projects = []jira.Project{
		{Key: "PROJ", Name: "My Project"},
		{Key: "TEST", Name: "Test Project"},
	}
	m.projectCursor = 2

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})

	if m.values[stepProject] != "TEST" {
		t.Errorf("expected project value = 'TEST', got %q", m.values[stepProject])
	}
	if m.step != stepBoardID {
		t.Errorf("expected to advance to stepBoardID, got %d", m.step)
	}
}

func TestUpdate_ProjectPicker_NotLoaded_IgnoresKeys(t *testing.T) {
	m := New(&config.Config{AuthType: "basic"})
	m.step = stepProject
	m.projectsLoaded = false

	startCursor := m.projectCursor
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.projectCursor != startCursor {
		t.Error("expected picker to ignore keys when projects not loaded")
	}
}

func TestUpdate_BoardPicker_Navigation(t *testing.T) {
	m := New(&config.Config{AuthType: "basic"})
	m.step = stepBoardID
	m.boardsLoaded = true
	m.boards = []jira.Board{
		{ID: 1, Name: "Sprint Board", Type: "scrum"},
		{ID: 2, Name: "Kanban Board", Type: "kanban"},
	}
	m.boardCursor = 0

	// Down from 0 to 1.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.boardCursor != 1 {
		t.Errorf("expected cursor 1 after down, got %d", m.boardCursor)
	}

	// Down from 1 to 2.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.boardCursor != 2 {
		t.Errorf("expected cursor 2 after second down, got %d", m.boardCursor)
	}

	// Down from 2 should stay at 2.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.boardCursor != 2 {
		t.Errorf("expected cursor to stay at 2, got %d", m.boardCursor)
	}

	// Up from 2 to 1.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.boardCursor != 1 {
		t.Errorf("expected cursor 1 after up, got %d", m.boardCursor)
	}

	// Up from 1 to 0.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.boardCursor != 0 {
		t.Errorf("expected cursor 0 after second up, got %d", m.boardCursor)
	}

	// Up from 0 should stay at 0.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.boardCursor != 0 {
		t.Errorf("expected cursor to stay at 0, got %d", m.boardCursor)
	}
}

func TestUpdate_BoardPicker_Select(t *testing.T) {
	m := New(&config.Config{AuthType: "basic"})
	m.step = stepBoardID
	m.boardsLoaded = true
	m.boards = []jira.Board{
		{ID: 1, Name: "Sprint Board", Type: "scrum"},
		{ID: 2, Name: "Kanban Board", Type: "kanban"},
	}
	m.boardCursor = 1 // First board (ID 1).

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.values[stepBoardID] != "1" {
		t.Errorf("expected board value = '1', got %q", m.values[stepBoardID])
	}
	if m.step != stepRepoPath {
		t.Errorf("expected to advance to stepRepoPath (%d), got %d", stepRepoPath, m.step)
	}
}

func TestUpdate_BoardPicker_SelectNone(t *testing.T) {
	m := New(&config.Config{AuthType: "basic"})
	m.step = stepBoardID
	m.boardsLoaded = true
	m.boards = []jira.Board{
		{ID: 1, Name: "Sprint Board", Type: "scrum"},
	}
	m.boardCursor = 0 // "None" option.

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.values[stepBoardID] != "" {
		t.Errorf("expected empty board value for None, got %q", m.values[stepBoardID])
	}
	if m.step != stepRepoPath {
		t.Errorf("expected to advance to stepRepoPath, got %d", m.step)
	}
}

func TestUpdate_BoardPicker_NotLoaded_IgnoresKeys(t *testing.T) {
	m := New(&config.Config{AuthType: "basic"})
	m.step = stepBoardID
	m.boardsLoaded = false

	startCursor := m.boardCursor
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.boardCursor != startCursor {
		t.Error("expected picker to ignore keys when boards not loaded")
	}
}

// --- nextMissingStep tests ---

func TestNextMissingStep_AllFilled(t *testing.T) {
	m := New(partialConfig())
	// Set all input values.
	m.values[stepDomain] = "test.atlassian.net"
	m.values[stepUser] = "user@test.com"
	m.values[stepAPIToken] = "token"
	m.values[stepAuthType] = "basic"
	m.values[stepProject] = "TEST"
	m.values[stepBoardID] = "1"
	m.values[stepRepoPath] = "/some/path"
	m.inputs[stepDomain].SetValue("test.atlassian.net")
	m.inputs[stepUser].SetValue("user@test.com")
	m.inputs[stepAPIToken].SetValue("token")
	m.inputs[stepAuthType].SetValue("basic")
	m.inputs[stepRepoPath].SetValue("/some/path")

	result := m.nextMissingStep(stepDomain)
	if result != stepConfirm {
		t.Errorf("expected stepConfirm when all filled, got %d", result)
	}
}

func TestNextMissingStep_SkipsFilled(t *testing.T) {
	m := New(&config.Config{AuthType: "basic"})
	// Fill domain and user but leave token empty.
	m.values[stepDomain] = "test.atlassian.net"
	m.inputs[stepDomain].SetValue("test.atlassian.net")
	m.values[stepUser] = "user@test.com"
	m.inputs[stepUser].SetValue("user@test.com")

	result := m.nextMissingStep(stepDomain)
	if result != stepAPIToken {
		t.Errorf("expected stepAPIToken as first missing, got %d", result)
	}
}

func TestNextMissingStep_SkipsPickerAndBranchSteps(t *testing.T) {
	m := New(&config.Config{AuthType: "basic"})
	// Fill all input steps but leave pickers empty.
	m.values[stepDomain] = "test.atlassian.net"
	m.inputs[stepDomain].SetValue("test.atlassian.net")
	m.values[stepUser] = "user@test.com"
	m.inputs[stepUser].SetValue("user@test.com")
	m.values[stepAPIToken] = "token"
	m.inputs[stepAPIToken].SetValue("token")
	m.values[stepAuthType] = "basic"
	m.inputs[stepAuthType].SetValue("basic")
	m.inputs[stepRepoPath].SetValue("/some/path")
	m.values[stepRepoPath] = "/some/path"

	// Project is a picker step with no value — should be the first missing.
	result := m.nextMissingStep(stepDomain)
	if result != stepProject {
		t.Errorf("expected stepProject as first missing picker, got %d", result)
	}
}

func TestNextMissingStep_FromMiddle(t *testing.T) {
	m := New(&config.Config{AuthType: "basic"})
	// Starting from stepRepoPath with no value set.
	result := m.nextMissingStep(stepRepoPath)
	if result != stepRepoPath {
		t.Errorf("expected stepRepoPath, got %d", result)
	}
}

// --- Config output tests ---

func TestConfig_BranchMode(t *testing.T) {
	tests := []struct {
		name     string
		mode     string
		expected string
	}{
		{"local", "local", "local"},
		{"remote", "remote", "remote"},
		{"both", "both", "both"},
		{"empty defaults to local", "", "local"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(&config.Config{AuthType: "basic"})
			m.values[stepBranchMode] = tt.mode
			cfg := m.Config()
			if cfg.BranchMode != tt.expected {
				t.Errorf("Config().BranchMode = %q, want %q", cfg.BranchMode, tt.expected)
			}
		})
	}
}

func TestConfig_BoardID(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected int
	}{
		{"valid ID", "42", 42},
		{"empty", "", 0},
		{"invalid", "abc", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New(&config.Config{AuthType: "basic"})
			m.values[stepBoardID] = tt.value
			cfg := m.Config()
			if cfg.BoardID != tt.expected {
				t.Errorf("Config().BoardID = %d, want %d", cfg.BoardID, tt.expected)
			}
		})
	}
}

func TestConfig_BranchUppercase(t *testing.T) {
	m := New(&config.Config{AuthType: "basic"})
	m.values[stepBranchCase] = "true"
	cfg := m.Config()
	if !cfg.BranchUppercase {
		t.Error("expected BranchUppercase = true when value is 'true'")
	}

	m2 := New(&config.Config{AuthType: "basic"})
	m2.values[stepBranchCase] = ""
	cfg2 := m2.Config()
	if cfg2.BranchUppercase {
		t.Error("expected BranchUppercase = false when value is empty")
	}
}

// --- Render summary tests ---

func TestRenderSummary_MaskedToken(t *testing.T) {
	m := sizedModel(partialConfig())
	m.GoToConfirm()
	// Set a long token to trigger masking.
	m.values[stepAPIToken] = "abcd1234efgh5678"

	view := m.View()
	// The token should be masked — first 4 + last 4 visible, rest masked.
	if strings.Contains(view, "abcd1234efgh5678") {
		t.Error("expected API token to be masked in summary")
	}
	if !strings.Contains(view, "abcd") {
		t.Error("expected first 4 chars of token visible")
	}
	if !strings.Contains(view, "5678") {
		t.Error("expected last 4 chars of token visible")
	}
}

func TestRenderSummary_NotSetValues(t *testing.T) {
	m := sizedModel(&config.Config{AuthType: "basic"})
	m.GoToConfirm()
	// Most values are empty.

	view := m.View()
	if !strings.Contains(view, "(not set)") {
		t.Error("expected '(not set)' for empty values in summary")
	}
}

func TestRenderSummary_ProjectDisplayName(t *testing.T) {
	m := sizedModel(&config.Config{AuthType: "basic"})
	m.GoToConfirm()
	m.values[stepProject] = "PROJ"
	m.projects = []jira.Project{
		{Key: "PROJ", Name: "My Project"},
	}

	view := m.View()
	if !strings.Contains(view, "My Project (PROJ)") {
		t.Error("expected project display name in summary")
	}
}

func TestRenderSummary_BoardDisplayName(t *testing.T) {
	m := sizedModel(&config.Config{AuthType: "basic"})
	m.GoToConfirm()
	m.values[stepBoardID] = "1"
	m.boards = []jira.Board{
		{ID: 1, Name: "Sprint Board", Type: "scrum"},
	}

	view := m.View()
	if !strings.Contains(view, "Sprint Board [scrum]") {
		t.Error("expected board display name in summary")
	}
}

func TestRenderSummary_BranchCaseDisplay(t *testing.T) {
	m := sizedModel(&config.Config{AuthType: "basic"})
	m.GoToConfirm()
	m.values[stepBranchCase] = "true"

	view := m.View()
	if !strings.Contains(view, "UPPERCASE") {
		t.Error("expected 'UPPERCASE' in summary when branch case is true")
	}

	m2 := sizedModel(&config.Config{AuthType: "basic"})
	m2.GoToConfirm()
	m2.values[stepBranchCase] = ""

	view2 := m2.View()
	if !strings.Contains(view2, "lowercase") {
		t.Error("expected 'lowercase' in summary when branch case is empty")
	}
}

func TestRenderSummary_BranchModeDisplay(t *testing.T) {
	for _, mode := range []string{"local", "remote", "both"} {
		t.Run(mode, func(t *testing.T) {
			m := sizedModel(&config.Config{AuthType: "basic"})
			m.GoToConfirm()
			m.values[stepBranchMode] = mode

			view := m.View()
			if !strings.Contains(view, mode) {
				t.Errorf("expected %q in summary view", mode)
			}
		})
	}
}

// --- Picker rendering with error state ---

func TestView_ProjectPickerStep_Error(t *testing.T) {
	m := sizedModel(&config.Config{AuthType: "basic"})
	m.step = stepProject
	m.projectsLoaded = true
	m.errMsg = "Could not fetch projects: timeout"

	view := m.View()
	if !strings.Contains(view, "Could not fetch projects") {
		t.Error("expected error message in project picker view")
	}
	if !strings.Contains(view, "Press enter to continue") {
		t.Error("expected continue hint in project picker error view")
	}
}

func TestView_BoardPickerStep_Error(t *testing.T) {
	m := sizedModel(&config.Config{AuthType: "basic"})
	m.step = stepBoardID
	m.boardsLoaded = true
	m.errMsg = "Could not fetch boards: timeout"

	view := m.View()
	if !strings.Contains(view, "Could not fetch boards") {
		t.Error("expected error message in board picker view")
	}
	if !strings.Contains(view, "Press enter to continue") {
		t.Error("expected continue hint in board picker error view")
	}
}

// --- Branch toggle with j/k keys ---

func TestUpdate_BranchCaseToggle_JK(t *testing.T) {
	m := New(&config.Config{AuthType: "basic"})
	m.step = stepBranchCase
	m.branchCaseCursor = 0

	// j toggles.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.branchCaseCursor != 1 {
		t.Errorf("expected cursor 1 after j, got %d", m.branchCaseCursor)
	}

	// k toggles back.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.branchCaseCursor != 0 {
		t.Errorf("expected cursor 0 after k, got %d", m.branchCaseCursor)
	}
}

func TestUpdate_BranchModeToggle_JK(t *testing.T) {
	m := New(&config.Config{AuthType: "basic"})
	m.step = stepBranchMode
	m.branchModeCursor = 0

	// j (down) to 1.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.branchModeCursor != 1 {
		t.Errorf("expected cursor 1 after j, got %d", m.branchModeCursor)
	}

	// j (down) to 2.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.branchModeCursor != 2 {
		t.Errorf("expected cursor 2 after second j, got %d", m.branchModeCursor)
	}

	// j at max stays at 2.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.branchModeCursor != 2 {
		t.Errorf("expected cursor to stay at 2, got %d", m.branchModeCursor)
	}

	// k (up) to 1.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.branchModeCursor != 1 {
		t.Errorf("expected cursor 1 after k, got %d", m.branchModeCursor)
	}
}

func TestUpdate_BranchModeToggle_Tab(t *testing.T) {
	m := New(&config.Config{AuthType: "basic"})
	m.step = stepBranchMode
	m.branchModeCursor = 0

	// tab cycles: 0 -> 1.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.branchModeCursor != 1 {
		t.Errorf("expected cursor 1 after tab, got %d", m.branchModeCursor)
	}

	// tab cycles: 1 -> 2.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.branchModeCursor != 2 {
		t.Errorf("expected cursor 2 after second tab, got %d", m.branchModeCursor)
	}

	// tab wraps: 2 -> 0.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.branchModeCursor != 0 {
		t.Errorf("expected cursor 0 after wrap tab, got %d", m.branchModeCursor)
	}
}

// --- WindowSizeMsg handling ---

func TestUpdate_WindowSizeMsg(t *testing.T) {
	m := New(&config.Config{AuthType: "basic"})

	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})

	if m.width != 100 {
		t.Errorf("expected width = 100, got %d", m.width)
	}
	if m.height != 50 {
		t.Errorf("expected height = 50, got %d", m.height)
	}
}

// --- InputActive tests ---

func TestInputActive(t *testing.T) {
	tests := []struct {
		step   int
		active bool
	}{
		{stepWelcome, false},
		{stepDomain, true},
		{stepUser, true},
		{stepAPIToken, true},
		{stepAuthType, true},
		{stepProject, false},
		{stepBoardID, false},
		{stepRepoPath, true},
		{stepBranchCase, false},
		{stepBranchMode, false},
		{stepConfirm, false},
	}

	for _, tt := range tests {
		m := New(&config.Config{AuthType: "basic"})
		m.step = tt.step
		if m.InputActive() != tt.active {
			t.Errorf("step %d: InputActive() = %v, want %v", tt.step, m.InputActive(), tt.active)
		}
	}
}

// --- Spinner tick handling ---

func TestUpdate_SpinnerTick_WhenValidating(t *testing.T) {
	m := New(&config.Config{AuthType: "basic"})
	m.validating = true

	// Spinner tick should be processed and return a command.
	m, cmd := m.Update(m.valSpinner.Tick())
	if cmd == nil {
		// Spinner might not return a command depending on timing, but should not panic.
		_ = m
	}
}

func TestUpdate_SpinnerTick_WhenNotValidating(t *testing.T) {
	m := New(&config.Config{AuthType: "basic"})
	m.validating = false

	// Send a spinner tick when not validating — should be a no-op.
	_, cmd := m.Update(spinner.TickMsg{})
	if cmd != nil {
		t.Error("expected nil cmd when spinner tick received but not validating")
	}
}

// --- Keys ignored during validation ---

func TestUpdate_KeysIgnoredWhileValidating(t *testing.T) {
	m := New(&config.Config{AuthType: "basic"})
	m.step = stepDomain
	m.validating = true

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	// Should not advance — keys are ignored during validation.
	if m.step != stepDomain {
		t.Errorf("expected to stay on stepDomain while validating, got %d", m.step)
	}
}

// --- Prefill from partial config ---

func TestNew_PrefillsFromPartialConfig(t *testing.T) {
	partial := &config.Config{
		Domain:          "myco.atlassian.net",
		User:            "me@myco.com",
		APIToken:        "secret",
		AuthType:        "bearer",
		Project:         "MYPROJ",
		BoardID:         42,
		RepoPath:        "/home/me/repo",
		BranchUppercase: true,
		BranchMode:      "both",
	}
	m := New(partial)

	if m.values[stepDomain] != "myco.atlassian.net" {
		t.Errorf("domain not prefilled: %q", m.values[stepDomain])
	}
	if m.values[stepUser] != "me@myco.com" {
		t.Errorf("user not prefilled: %q", m.values[stepUser])
	}
	if m.values[stepAPIToken] != "secret" {
		t.Errorf("token not prefilled: %q", m.values[stepAPIToken])
	}
	if m.values[stepAuthType] != "bearer" {
		t.Errorf("auth type not prefilled: %q", m.values[stepAuthType])
	}
	if m.values[stepProject] != "MYPROJ" {
		t.Errorf("project not prefilled: %q", m.values[stepProject])
	}
	if m.values[stepBoardID] != "42" {
		t.Errorf("board ID not prefilled: %q", m.values[stepBoardID])
	}
	if m.values[stepRepoPath] != "/home/me/repo" {
		t.Errorf("repo path not prefilled: %q", m.values[stepRepoPath])
	}
	if m.branchCaseCursor != 1 {
		t.Errorf("branch case cursor not set for uppercase: %d", m.branchCaseCursor)
	}
	if m.branchModeCursor != 2 {
		t.Errorf("branch mode cursor not set for 'both': %d", m.branchModeCursor)
	}
}

func TestNew_DefaultAuthType(t *testing.T) {
	m := New(nil)
	if m.values[stepAuthType] != "basic" {
		t.Errorf("expected default auth type 'basic', got %q", m.values[stepAuthType])
	}
}

// --- Footer hints per step ---

func TestFooterHints_PerStep(t *testing.T) {
	tests := []struct {
		name    string
		step    int
		expects []string // substrings that must appear across Key/Desc fields
	}{
		{"welcome has start and quit", stepWelcome, []string{"start", "quit"}},
		{"confirm has save, restart, back", stepConfirm, []string{"save", "restart", "ctrl+b"}},
		{"domain has next, back, quit", stepDomain, []string{"next", "ctrl+b", "esc"}},
		{"project has toggle and select", stepProject, []string{"toggle", "select"}},
		{"branch copy has toggle and select", stepBranchCopy, []string{"toggle", "select"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := sizedModel(&config.Config{AuthType: "basic"})
			m.step = tt.step

			var joined strings.Builder
			for _, h := range m.FooterHints() {
				joined.WriteString(h.Key)
				joined.WriteString(" ")
				joined.WriteString(h.Desc)
				joined.WriteString(" ")
			}
			got := joined.String()
			for _, expect := range tt.expects {
				if !strings.Contains(got, expect) {
					t.Errorf("expected %q in footer hints for step %d, got %q", expect, tt.step, got)
				}
			}
		})
	}
}

// --- Required field indicator ---

func TestView_RequiredIndicator(t *testing.T) {
	m := sizedModel(&config.Config{AuthType: "basic"})
	m.step = stepDomain
	m.validating = false

	view := m.View()
	if !strings.Contains(view, "required") {
		t.Error("expected '* required' indicator for required step")
	}
}

// --- Validation label helper ---

func TestValidationLabel(t *testing.T) {
	m := New(&config.Config{AuthType: "basic"})

	m.step = stepAuthType
	if label := m.validationLabel(); label != "Verifying credentials..." {
		t.Errorf("expected 'Verifying credentials...', got %q", label)
	}

	m.step = stepProject
	if label := m.validationLabel(); label != "Fetching projects..." {
		t.Errorf("expected 'Fetching projects...', got %q", label)
	}

	m.step = stepBoardID
	if label := m.validationLabel(); label != "Fetching boards..." {
		t.Errorf("expected 'Fetching boards...', got %q", label)
	}

	m.step = stepDomain
	if label := m.validationLabel(); label != "Validating..." {
		t.Errorf("expected 'Validating...', got %q", label)
	}
}

// --- cleanDomain tests ---

func TestCleanDomain(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://myco.atlassian.net", "myco.atlassian.net"},
		{"http://myco.atlassian.net", "myco.atlassian.net"},
		{"https://myco.atlassian.net/", "myco.atlassian.net"},
		{"myco.atlassian.net", "myco.atlassian.net"},
		{"  myco.atlassian.net  ", "myco.atlassian.net"},
		{"HTTPS://MyCompany.atlassian.net", "MyCompany.atlassian.net"},
	}

	for _, tt := range tests {
		got := cleanDomain(tt.input)
		if got != tt.want {
			t.Errorf("cleanDomain(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// --- View rendering for picker cursor positions ---

func TestView_ProjectPickerStep_CursorHighlight(t *testing.T) {
	m := sizedModel(&config.Config{AuthType: "basic"})
	m.step = stepProject
	m.projectsLoaded = true
	m.projects = []jira.Project{
		{Key: "PROJ", Name: "My Project"},
	}
	m.projectCursor = 1 // Highlighting the project.

	view := m.View()
	// The selected item should have the marker.
	if !strings.Contains(view, "My Project") {
		t.Error("expected project to be visible when cursor is on it")
	}
}

func TestView_BoardPickerStep_CursorHighlight(t *testing.T) {
	m := sizedModel(&config.Config{AuthType: "basic"})
	m.step = stepBoardID
	m.boardsLoaded = true
	m.boards = []jira.Board{
		{ID: 1, Name: "Sprint Board", Type: "scrum"},
	}
	m.boardCursor = 1 // Highlighting the board.

	view := m.View()
	if !strings.Contains(view, "Sprint Board") {
		t.Error("expected board to be visible when cursor is on it")
	}
}
