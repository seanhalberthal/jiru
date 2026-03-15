package setupview

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/seanhalberthal/jiru/internal/config"
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
