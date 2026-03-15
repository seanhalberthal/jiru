package branchview

import (
	"os"
	"os/exec"
	"path/filepath"
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
	m := New(testIssue(), "", false, "local")
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
	m := New(testIssue(), "", false, "local")

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
	m := New(testIssue(), "/tmp/test-repo", false, "local")
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
	m := New(testIssue(), "", false, "local")
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
	m := New(testIssue(), "", false, "local")
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
	m := New(testIssue(), "", true, "local")
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
	m := New(testIssue(), repo, false, "local")

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

	m := New(testIssue(), repo, false, "local")
	if m.baseBranch.Value() != "develop" {
		t.Errorf("baseBranch = %q, want %q", m.baseBranch.Value(), "develop")
	}
}

func TestUpdateSuggestions_FiltersOnInput(t *testing.T) {
	m := New(testIssue(), "", false, "local")
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
	m := New(testIssue(), "", false, "local")
	m.branches = []string{"main", "develop"}
	m.activeInput = 1
	m.baseBranch.SetValue("")
	m.updateSuggestions()

	if m.showSugg {
		t.Error("expected no suggestions for empty query")
	}
}

func TestUpdateSuggestions_ExactMatch_HidesSuggestions(t *testing.T) {
	m := New(testIssue(), "", false, "local")
	m.branches = []string{"main", "develop"}
	m.activeInput = 1
	m.baseBranch.SetValue("main")
	m.updateSuggestions()

	if m.showSugg {
		t.Error("expected suggestions hidden when input exactly matches single result")
	}
}

func TestUpdateSuggestions_CaseInsensitive(t *testing.T) {
	m := New(testIssue(), "", false, "local")
	m.branches = []string{"main", "Develop", "HOTFIX"}
	m.activeInput = 1
	m.baseBranch.SetValue("dev")
	m.updateSuggestions()

	if len(m.suggestions) != 1 || m.suggestions[0] != "Develop" {
		t.Errorf("expected case-insensitive match, got %v", m.suggestions)
	}
}

func TestUpdateSuggestions_NotShownOnBranchNameInput(t *testing.T) {
	m := New(testIssue(), "", false, "local")
	m.branches = []string{"main", "develop"}
	m.activeInput = 0 // Branch name field, not base branch.
	m.baseBranch.SetValue("ma")
	m.updateSuggestions()

	if m.showSugg {
		t.Error("expected no suggestions when editing branch name field")
	}
}

func TestEsc_ClosesSuggestionsFirst(t *testing.T) {
	m := New(testIssue(), "", false, "local")
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
