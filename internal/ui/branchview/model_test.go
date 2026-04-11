package branchview

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/seanhalberthal/jiru/internal/jira"
)

func testIssue() jira.Issue {
	return jira.Issue{
		Key:     "PROJ-123",
		Summary: "Fix login bug",
		Status:  "To Do",
	}
}

func TestNew_InitialState(t *testing.T) {
	m := New(testIssue(), "", false, "local", false)
	if m.branchName.Value() != "proj-123-fix-login-bug" {
		t.Errorf("expected slugified branch name, got %q", m.branchName.Value())
	}
	if m.branchMode != "local" {
		t.Errorf("expected mode 'local', got %q", m.branchMode)
	}
	if m.baseBranch.Value() != "main" {
		t.Errorf("expected 'main', got %q", m.baseBranch.Value())
	}
	if !m.InputActive() {
		t.Error("expected input active immediately")
	}
}

func TestSubmittedBranch_Sentinel(t *testing.T) {
	m := New(testIssue(), "", false, "local", false)

	if m.SubmittedBranch() != nil {
		t.Error("expected nil initially")
	}

	// Simulate pressing enter.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	req := m.SubmittedBranch()
	if req == nil {
		t.Fatal("expected branch request after enter")
	}
	if req.Name != "proj-123-fix-login-bug" {
		t.Errorf("expected slugified name, got %q", req.Name)
	}
	if req.Base != "main" {
		t.Errorf("expected 'main', got %q", req.Base)
	}
	if req.RepoPath != "" {
		t.Errorf("expected empty repo path, got %q", req.RepoPath)
	}

	// Should reset after read.
	if m.SubmittedBranch() != nil {
		t.Error("expected sentinel to reset")
	}
}

func TestSubmittedBranch_WithRepoPath(t *testing.T) {
	m := New(testIssue(), "/tmp/test-repo", false, "local", false)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	req := m.SubmittedBranch()
	if req == nil {
		t.Fatal("expected branch request")
	}
	if req.RepoPath != "/tmp/test-repo" {
		t.Errorf("expected repo path, got %q", req.RepoPath)
	}
}

func TestDismissed_Sentinel(t *testing.T) {
	m := New(testIssue(), "", false, "local", false)
	if m.Dismissed() {
		t.Error("expected not dismissed initially")
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !m.Dismissed() {
		t.Error("expected dismissed on esc")
	}
	if m.Dismissed() {
		t.Error("expected sentinel to reset")
	}
}

func TestInputActive(t *testing.T) {
	m := New(testIssue(), "", false, "local", false)
	if !m.InputActive() {
		t.Error("expected input always active")
	}
}

func TestSlugify_Lowercase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"PROJ-123 Fix login bug!", "proj-123-fix-login-bug"},
		{"PROJ-1 Hello World", "proj-1-hello-world"},
		{"ABC-99   Spaces  Everywhere ", "abc-99-spaces-everywhere"},
		{"TEST-1 Special@#$chars", "test-1-special-chars"},
	}
	for _, tt := range tests {
		got := Slugify(tt.input, false)
		if got != tt.want {
			t.Errorf("Slugify(%q, false) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSlugify_Uppercase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"PROJ-123 Fix login bug!", "PROJ-123-Fix-Login-Bug"},
		{"proj-1 hello world", "PROJ-1-Hello-World"},
		{"ABC-99   Spaces  Everywhere ", "ABC-99-Spaces-Everywhere"},
		{"TEST-1 Special@#$chars", "TEST-1-Special-Chars"},
	}
	for _, tt := range tests {
		got := Slugify(tt.input, true)
		if got != tt.want {
			t.Errorf("Slugify(%q, true) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSlugify_Truncation(t *testing.T) {
	long := "PROJ-1 " + "this is a very long summary that should be truncated to a reasonable length for branch names"
	got := Slugify(long, false)
	if len(got) > 80 {
		t.Errorf("expected slug <= 80 chars, got %d: %q", len(got), got)
	}
}

func TestNew_Uppercase(t *testing.T) {
	m := New(testIssue(), "", true, "local", false)
	if m.branchName.Value() != "PROJ-123-Fix-Login-Bug" {
		t.Errorf("expected title-case branch name, got %q", m.branchName.Value())
	}
}

// initTestRepo creates a temporary git repo with a few branches for testing.
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	cmds := [][]string{
		{"git", "-C", dir, "init", "-b", "main"},
		{"git", "-C", dir, "config", "user.email", "test@test.com"},
		{"git", "-C", dir, "config", "user.name", "Test"},
		{"git", "-C", dir, "commit", "--allow-empty", "-m", "init"},
		{"git", "-C", dir, "checkout", "-b", "develop"},
		{"git", "-C", dir, "checkout", "-b", "feature/login"},
		{"git", "-C", dir, "checkout", "main"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("git command %v failed: %v", args, err)
		}
	}
	return dir
}

func TestListBranches(t *testing.T) {
	repo := initTestRepo(t)
	branches := listBranches(repo)

	want := map[string]bool{"main": false, "develop": false, "feature/login": false}
	for _, b := range branches {
		if _, ok := want[b]; ok {
			want[b] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("expected branch %q in list, got %v", name, branches)
		}
	}
}

func TestListBranches_InvalidPath(t *testing.T) {
	branches := listBranches("/nonexistent/path")
	if branches != nil {
		t.Errorf("expected nil for invalid path, got %v", branches)
	}
}

func TestListBranches_Sorted(t *testing.T) {
	repo := initTestRepo(t)
	branches := listBranches(repo)

	for i := 1; i < len(branches); i++ {
		if branches[i] < branches[i-1] {
			t.Errorf("branches not sorted: %v", branches)
			break
		}
	}
}

func TestCurrentBranch(t *testing.T) {
	repo := initTestRepo(t)
	got := currentBranch(repo)
	if got != "main" {
		t.Errorf("currentBranch = %q, want %q", got, "main")
	}
}

func TestCurrentBranch_InvalidPath(t *testing.T) {
	got := currentBranch("/nonexistent/path")
	if got != "" {
		t.Errorf("expected empty string for invalid path, got %q", got)
	}
}

func TestNew_WithRepoPath_SetsCurrentBranch(t *testing.T) {
	repo := initTestRepo(t)
	m := New(testIssue(), repo, false, "local", false)

	if m.baseBranch.Value() != "main" {
		t.Errorf("baseBranch = %q, want %q (current branch)", m.baseBranch.Value(), "main")
	}
	if len(m.branches) == 0 {
		t.Error("expected branches to be populated")
	}
}

func TestNew_WithRepoPath_DifferentCurrentBranch(t *testing.T) {
	repo := initTestRepo(t)
	// Switch to develop.
	cmd := exec.Command("git", "-C", repo, "checkout", "develop")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	m := New(testIssue(), repo, false, "local", false)
	if m.baseBranch.Value() != "develop" {
		t.Errorf("baseBranch = %q, want %q", m.baseBranch.Value(), "develop")
	}
}

func TestUpdateSuggestions_FiltersOnInput(t *testing.T) {
	m := New(testIssue(), "", false, "local", false)
	m.branches = []string{"main", "develop", "feature/login", "feature/signup", "hotfix/crash"}

	// Simulate typing in base branch field.
	m.activeInput = 1
	m.baseBranch.SetValue("feat")
	m.updateSuggestions()

	if !m.showSugg {
		t.Error("expected suggestions to be shown")
	}
	if len(m.suggestions) != 2 {
		t.Errorf("expected 2 suggestions matching 'feat', got %d: %v", len(m.suggestions), m.suggestions)
	}
}

func TestUpdateSuggestions_EmptyQuery(t *testing.T) {
	m := New(testIssue(), "", false, "local", false)
	m.branches = []string{"main", "develop"}
	m.activeInput = 1
	m.baseBranch.SetValue("")
	m.updateSuggestions()

	if m.showSugg {
		t.Error("expected no suggestions for empty query")
	}
}

func TestUpdateSuggestions_ExactMatch_HidesSuggestions(t *testing.T) {
	m := New(testIssue(), "", false, "local", false)
	m.branches = []string{"main", "develop"}
	m.activeInput = 1
	m.baseBranch.SetValue("main")
	m.updateSuggestions()

	if m.showSugg {
		t.Error("expected suggestions hidden when input exactly matches single result")
	}
}

func TestUpdateSuggestions_CaseInsensitive(t *testing.T) {
	m := New(testIssue(), "", false, "local", false)
	m.branches = []string{"main", "Develop", "HOTFIX"}
	m.activeInput = 1
	m.baseBranch.SetValue("dev")
	m.updateSuggestions()

	if len(m.suggestions) != 1 || m.suggestions[0] != "Develop" {
		t.Errorf("expected case-insensitive match, got %v", m.suggestions)
	}
}

func TestUpdateSuggestions_NotShownOnBranchNameInput(t *testing.T) {
	m := New(testIssue(), "", false, "local", false)
	m.branches = []string{"main", "develop"}
	m.activeInput = 0 // Branch name field, not base branch.
	m.baseBranch.SetValue("ma")
	m.updateSuggestions()

	if m.showSugg {
		t.Error("expected no suggestions when editing branch name field")
	}
}

func TestEsc_ClosesSuggestionsFirst(t *testing.T) {
	m := New(testIssue(), "", false, "local", false)
	m.branches = []string{"main", "develop"}
	m.activeInput = 1
	m.baseBranch.SetValue("ma")
	m.updateSuggestions()

	// Esc should close suggestions, not dismiss the view.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.showSugg {
		t.Error("expected suggestions to be closed")
	}
	if m.Dismissed() {
		t.Error("expected view not dismissed — esc should close suggestions first")
	}
}

func TestListBranches_DeduplicatesRemote(t *testing.T) {
	repo := initTestRepo(t)

	// Create a bare remote and push to it so we get origin/ refs.
	remote := filepath.Join(t.TempDir(), "remote.git")
	cmds := [][]string{
		{"git", "init", "--bare", remote},
		{"git", "-C", repo, "remote", "add", "origin", remote},
		{"git", "-C", repo, "push", "origin", "main"},
		{"git", "-C", repo, "push", "origin", "develop"},
		{"git", "-C", repo, "fetch"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("git command %v failed: %v", args, err)
		}
	}

	branches := listBranches(repo)
	// "main" should appear once, not twice (local + origin/main).
	count := 0
	for _, b := range branches {
		if b == "main" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 'main' once (deduplicated), got %d times in %v", count, branches)
	}
}

func TestSetSize(t *testing.T) {
	m := New(testIssue(), "", false, "local", false)
	m.SetSize(100, 40)

	if m.width != 100 {
		t.Errorf("expected width 100, got %d", m.width)
	}
	if m.height != 40 {
		t.Errorf("expected height 40, got %d", m.height)
	}
	if m.branchName.Width != 48 {
		t.Errorf("expected branchName width 48 (100/2-2), got %d", m.branchName.Width)
	}
	if m.baseBranch.Width != 48 {
		t.Errorf("expected baseBranch width 48 (100/2-2), got %d", m.baseBranch.Width)
	}
}

func TestView_RendersBranchName(t *testing.T) {
	m := New(testIssue(), "/tmp/repo", false, "local", false)
	m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, "proj-123-fix-login-bug") {
		t.Error("expected View() to contain the branch name")
	}
	if !strings.Contains(view, "Create Branch") {
		t.Error("expected View() to contain title 'Create Branch'")
	}
	if !strings.Contains(view, "Branch name") {
		t.Error("expected View() to contain 'Branch name' label")
	}
	if !strings.Contains(view, "Base branch") {
		t.Error("expected View() to contain 'Base branch' label")
	}
}

func TestView_ClipboardMode(t *testing.T) {
	m := New(testIssue(), "", false, "local", false)
	m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, "clipboard") {
		t.Error("expected View() to contain 'clipboard' when repoPath is empty")
	}
	if !strings.Contains(view, "copy") {
		t.Error("expected View() to contain 'copy' hint when repoPath is empty")
	}
}

func TestView_RepoModes(t *testing.T) {
	tests := []struct {
		mode string
		want string
	}{
		{"local", "creates branch locally"},
		{"remote", "pushes branch to origin"},
		{"both", "pushes to origin"},
	}
	for _, tt := range tests {
		m := New(testIssue(), "/tmp/repo", false, tt.mode, false)
		m.SetSize(120, 40)
		view := m.View()
		if !strings.Contains(view, tt.want) {
			t.Errorf("mode %q: expected View() to contain %q", tt.mode, tt.want)
		}
	}
}

func TestView_ShowsSuggestions(t *testing.T) {
	m := New(testIssue(), "", false, "local", false)
	m.branches = []string{"main", "develop", "feature/login"}
	m.activeInput = 1
	m.baseBranch.SetValue("dev")
	m.updateSuggestions()
	m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, "develop") {
		t.Error("expected View() to contain suggestion 'develop'")
	}
}

func TestView_ShowsErrorMessage(t *testing.T) {
	m := New(testIssue(), "", false, "local", false)
	m.errMsg = "branch name must not start with '-'"
	m.SetSize(80, 24)

	view := m.View()
	if !strings.Contains(view, "branch name must not start with '-'") {
		t.Error("expected View() to contain the error message")
	}
}

func TestUpdate_TabSwitchesInput(t *testing.T) {
	m := New(testIssue(), "", false, "local", false)
	if m.activeInput != 0 {
		t.Fatal("expected activeInput to start at 0")
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.activeInput != 1 {
		t.Errorf("expected activeInput 1 after tab, got %d", m.activeInput)
	}
}

func TestUpdate_TabBack(t *testing.T) {
	m := New(testIssue(), "", false, "local", false)
	// Move to base branch field first.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.activeInput != 1 {
		t.Fatal("expected activeInput 1 after tab")
	}

	// shift+tab should go back to branch name.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.activeInput != 0 {
		t.Errorf("expected activeInput 0 after shift+tab, got %d", m.activeInput)
	}
}

func TestUpdate_SuggestionNavigation(t *testing.T) {
	m := New(testIssue(), "", false, "local", false)
	m.branches = []string{"develop", "feature/login", "feature/signup"}
	m.activeInput = 1
	m.baseBranch.SetValue("e")
	m.updateSuggestions()

	if !m.showSugg {
		t.Fatal("expected suggestions to be shown")
	}
	if m.suggIdx != 0 {
		t.Fatalf("expected suggIdx 0, got %d", m.suggIdx)
	}

	// Navigate down.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.suggIdx != 1 {
		t.Errorf("expected suggIdx 1 after down, got %d", m.suggIdx)
	}

	// Navigate down again.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.suggIdx != 2 {
		t.Errorf("expected suggIdx 2 after second down, got %d", m.suggIdx)
	}

	// Should not go past the end.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.suggIdx != 2 {
		t.Errorf("expected suggIdx to stay at 2, got %d", m.suggIdx)
	}

	// Navigate up.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.suggIdx != 1 {
		t.Errorf("expected suggIdx 1 after up, got %d", m.suggIdx)
	}

	// Navigate up to 0.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.suggIdx != 0 {
		t.Errorf("expected suggIdx 0, got %d", m.suggIdx)
	}

	// Should not go below 0.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.suggIdx != 0 {
		t.Errorf("expected suggIdx to stay at 0, got %d", m.suggIdx)
	}
}

func TestUpdate_SuggestionAccept(t *testing.T) {
	m := New(testIssue(), "", false, "local", false)
	m.branches = []string{"develop", "feature/login", "feature/signup"}
	m.activeInput = 1
	m.baseBranch.SetValue("feat")
	m.updateSuggestions()

	if !m.showSugg {
		t.Fatal("expected suggestions to be shown")
	}

	// Accept the first suggestion (feature/login) with enter.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.baseBranch.Value() != "feature/login" {
		t.Errorf("expected baseBranch to be set to suggestion, got %q", m.baseBranch.Value())
	}
	if m.showSugg {
		t.Error("expected suggestions to be closed after accepting")
	}
}

func TestUpdate_EscClosesSuggestions(t *testing.T) {
	m := New(testIssue(), "", false, "local", false)
	m.branches = []string{"develop", "feature/login"}
	m.activeInput = 1
	m.baseBranch.SetValue("dev")
	m.updateSuggestions()

	if !m.showSugg {
		t.Fatal("expected suggestions to be visible")
	}

	// Esc should close suggestions, not dismiss the view.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.showSugg {
		t.Error("expected suggestions to be closed")
	}
	if m.dismissed {
		t.Error("expected view not to be dismissed")
	}
}

func TestUpdate_EscDismisses(t *testing.T) {
	m := New(testIssue(), "", false, "local", false)
	// No suggestions visible.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !m.dismissed {
		t.Error("expected view to be dismissed when esc pressed with no suggestions")
	}
}

func TestUpdate_ValidationError(t *testing.T) {
	m := New(testIssue(), "", false, "local", false)
	// Set an invalid branch name (starts with '-').
	m.branchName.SetValue("-invalid-name")

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.errMsg == "" {
		t.Error("expected validation error for branch name starting with '-'")
	}
	if !strings.Contains(m.errMsg, "start with '-'") {
		t.Errorf("expected error about starting with '-', got %q", m.errMsg)
	}
	if m.submitted != nil {
		t.Error("expected no submission on validation error")
	}
}

func TestUpdate_ValidationErrorBase(t *testing.T) {
	m := New(testIssue(), "", false, "local", false)
	// Branch name is valid, but base branch is invalid.
	m.baseBranch.SetValue("-bad-base")

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.errMsg == "" {
		t.Error("expected validation error for base branch")
	}
	if !strings.Contains(m.errMsg, "base branch") {
		t.Errorf("expected error to mention 'base branch', got %q", m.errMsg)
	}
	if m.submitted != nil {
		t.Error("expected no submission on validation error")
	}
}

func TestUpdate_SuccessfulSubmit(t *testing.T) {
	m := New(testIssue(), "/tmp/repo", false, "both", false)
	m.SetSize(80, 24)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	req := m.SubmittedBranch()
	if req == nil {
		t.Fatal("expected branch request after successful submit")
	}
	if req.Name != "proj-123-fix-login-bug" {
		t.Errorf("expected name 'proj-123-fix-login-bug', got %q", req.Name)
	}
	if req.Base != "main" {
		t.Errorf("expected base 'main', got %q", req.Base)
	}
	if req.RepoPath != "/tmp/repo" {
		t.Errorf("expected repoPath '/tmp/repo', got %q", req.RepoPath)
	}
	if req.Mode != "both" {
		t.Errorf("expected mode 'both', got %q", req.Mode)
	}
	if m.errMsg != "" {
		t.Errorf("expected no error on valid submit, got %q", m.errMsg)
	}
}

func TestUpdate_NonKeyMsg(t *testing.T) {
	// Ensure non-key messages are forwarded to the active text input.
	m := New(testIssue(), "", false, "local", false)

	// Send a non-key message (blink msg) to exercise the non-key branch.
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// No crash, model still functional.
	if m.branchName.Value() != "proj-123-fix-login-bug" {
		t.Error("expected branch name to be unchanged after window size msg")
	}

	// Same for activeInput == 1.
	m.activeInput = 1
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if m.baseBranch.Value() != "main" {
		t.Error("expected base branch to be unchanged after window size msg")
	}
}

func TestUpdate_TypingInBaseBranch(t *testing.T) {
	m := New(testIssue(), "", false, "local", false)
	m.branches = []string{"main", "develop", "feature/test"}

	// Switch to base branch input.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.activeInput != 1 {
		t.Fatal("expected activeInput 1 after tab")
	}

	// Type a character — triggers suggestion update through Update path.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})

	// The input should have changed (appended 'f' to existing value).
	// Suggestions should be updated based on the new value.
	// We cannot easily predict the exact value since the textinput may handle
	// cursor position, but the model should not have crashed.
	if m.activeInput != 1 {
		t.Error("expected to remain on base branch input")
	}
}

func TestSlugify_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		uppercase bool
		want      string
	}{
		{
			name:      "empty input",
			input:     "",
			uppercase: false,
			want:      "",
		},
		{
			name:      "empty input uppercase",
			input:     "",
			uppercase: true,
			want:      "",
		},
		{
			name:      "only special characters",
			input:     "!@#$%^&*()",
			uppercase: false,
			want:      "",
		},
		{
			name:      "only special characters uppercase",
			input:     "!@#$%^&*()",
			uppercase: true,
			want:      "",
		},
		{
			name:      "very long input lowercase",
			input:     "PROJ-1 " + strings.Repeat("word ", 30),
			uppercase: false,
		},
		{
			name:      "very long input uppercase",
			input:     "PROJ-1 " + strings.Repeat("word ", 30),
			uppercase: true,
		},
		{
			name:      "single character",
			input:     "a",
			uppercase: false,
			want:      "a",
		},
		{
			name:      "hyphens only",
			input:     "---",
			uppercase: false,
			want:      "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Slugify(tt.input, tt.uppercase)

			// For long inputs, just check truncation.
			if strings.Contains(tt.name, "very long") {
				if len(got) > 80 {
					t.Errorf("Slugify(%q, %v) = %q (len %d), want <= 80 chars",
						tt.input, tt.uppercase, got, len(got))
				}
				if got == "" {
					t.Errorf("Slugify(%q, %v) = empty, want non-empty for valid input",
						tt.input, tt.uppercase)
				}
				return
			}

			if tt.want != "" || tt.input == "" || tt.input == "!@#$%^&*()" || tt.input == "---" {
				if got != tt.want {
					t.Errorf("Slugify(%q, %v) = %q, want %q",
						tt.input, tt.uppercase, got, tt.want)
				}
			}
		})
	}
}

func TestView_SuggestionsOverflowIndicator(t *testing.T) {
	m := New(testIssue(), "", false, "local", false)
	// Create more than 8 branches to trigger the "... N more" indicator.
	m.branches = make([]string, 15)
	for i := range m.branches {
		m.branches[i] = strings.Repeat("a", i+1) + "-branch"
	}
	m.activeInput = 1
	m.baseBranch.SetValue("a")
	m.updateSuggestions()
	m.SetSize(80, 24)

	if len(m.suggestions) <= 8 {
		t.Fatalf("expected more than 8 suggestions, got %d", len(m.suggestions))
	}

	view := m.View()
	if !strings.Contains(view, "more") {
		t.Error("expected View() to contain overflow indicator with 'more'")
	}
}

func TestUpdate_SuggestionAcceptWithTab(t *testing.T) {
	m := New(testIssue(), "", false, "local", false)
	m.branches = []string{"develop", "feature/login"}
	m.activeInput = 1
	m.baseBranch.SetValue("dev")
	m.updateSuggestions()

	if !m.showSugg {
		t.Fatal("expected suggestions to be shown")
	}

	// Accept suggestion with tab (alternative to enter).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.baseBranch.Value() != "develop" {
		t.Errorf("expected baseBranch to be 'develop' after tab accept, got %q", m.baseBranch.Value())
	}
	if m.showSugg {
		t.Error("expected suggestions to be closed after accepting with tab")
	}
}

func TestUpdate_CtrlNCtrlPNavigation(t *testing.T) {
	m := New(testIssue(), "", false, "local", false)
	m.branches = []string{"develop", "feature/login", "feature/signup"}
	m.activeInput = 1
	m.baseBranch.SetValue("e")
	m.updateSuggestions()

	if m.suggIdx != 0 {
		t.Fatalf("expected suggIdx 0, got %d", m.suggIdx)
	}

	// ctrl+n should navigate down.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	if m.suggIdx != 1 {
		t.Errorf("expected suggIdx 1 after ctrl+n, got %d", m.suggIdx)
	}

	// ctrl+p should navigate up.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	if m.suggIdx != 0 {
		t.Errorf("expected suggIdx 0 after ctrl+p, got %d", m.suggIdx)
	}
}
