package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_AllEnvVars(t *testing.T) {
	t.Setenv("JIRA_DOMAIN", "test.atlassian.net")
	t.Setenv("JIRA_USER", "user@test.com")
	t.Setenv("JIRA_API_TOKEN", "test-token")
	t.Setenv("JIRA_AUTH_TYPE", "bearer")
	t.Setenv("JIRA_BOARD_ID", "42")
	t.Setenv("JIRA_PROJECT", "TEST")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Domain != "test.atlassian.net" {
		t.Errorf("Domain = %q, want %q", cfg.Domain, "test.atlassian.net")
	}
	if cfg.User != "user@test.com" {
		t.Errorf("User = %q, want %q", cfg.User, "user@test.com")
	}
	if cfg.APIToken != "test-token" {
		t.Errorf("APIToken = %q, want %q", cfg.APIToken, "test-token")
	}
	if cfg.AuthType != "bearer" {
		t.Errorf("AuthType = %q, want %q", cfg.AuthType, "bearer")
	}
	if cfg.BoardID != 42 {
		t.Errorf("BoardID = %d, want 42", cfg.BoardID)
	}
	if cfg.Project != "TEST" {
		t.Errorf("Project = %q, want %q", cfg.Project, "TEST")
	}
}

func TestLoad_JiraURLAlias(t *testing.T) {
	t.Setenv("JIRA_URL", "https://alias.atlassian.net")
	t.Setenv("JIRA_USER", "user@test.com")
	t.Setenv("JIRA_API_TOKEN", "token")
	t.Setenv("JIRA_DOMAIN", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Domain != "alias.atlassian.net" {
		t.Errorf("Domain = %q, want %q", cfg.Domain, "alias.atlassian.net")
	}
}

func TestLoad_JiraUsernameAlias(t *testing.T) {
	t.Setenv("JIRA_DOMAIN", "test.atlassian.net")
	t.Setenv("JIRA_USERNAME", "altuser@test.com")
	t.Setenv("JIRA_API_TOKEN", "token")
	t.Setenv("JIRA_USER", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.User != "altuser@test.com" {
		t.Errorf("User = %q, want %q", cfg.User, "altuser@test.com")
	}
}

func TestLoad_InvalidBoardID(t *testing.T) {
	t.Setenv("JIRA_DOMAIN", "test.atlassian.net")
	t.Setenv("JIRA_USER", "user@test.com")
	t.Setenv("JIRA_API_TOKEN", "token")
	t.Setenv("JIRA_BOARD_ID", "not-a-number")

	_, err := Load()
	if err == nil {
		t.Error("expected error for non-numeric JIRA_BOARD_ID")
	}
}

func TestLoad_InvalidAuthType(t *testing.T) {
	t.Setenv("JIRA_DOMAIN", "test.atlassian.net")
	t.Setenv("JIRA_USER", "user@test.com")
	t.Setenv("JIRA_API_TOKEN", "token")
	t.Setenv("JIRA_AUTH_TYPE", "oauth")

	_, err := Load()
	if err == nil {
		t.Error("expected error for invalid JIRA_AUTH_TYPE")
	}
}

func TestLoad_MissingDomain(t *testing.T) {
	t.Setenv("JIRA_DOMAIN", "")
	t.Setenv("JIRA_URL", "")
	t.Setenv("JIRA_USER", "user@test.com")
	t.Setenv("JIRA_API_TOKEN", "token")
	t.Setenv("HOME", t.TempDir())

	_, err := Load()
	if err == nil {
		t.Error("expected error for missing domain")
	}
}

func TestLoad_MissingUser(t *testing.T) {
	t.Setenv("JIRA_DOMAIN", "test.atlassian.net")
	t.Setenv("JIRA_USER", "")
	t.Setenv("JIRA_USERNAME", "")
	t.Setenv("JIRA_API_TOKEN", "token")
	t.Setenv("HOME", t.TempDir())

	_, err := Load()
	if err == nil {
		t.Error("expected error for missing user")
	}
}

func TestLoad_MissingToken(t *testing.T) {
	t.Setenv("JIRA_DOMAIN", "test.atlassian.net")
	t.Setenv("JIRA_USER", "user@test.com")
	t.Setenv("JIRA_API_TOKEN", "")
	t.Setenv("HOME", t.TempDir())

	_, err := Load()
	if err == nil {
		t.Error("expected error for missing API token")
	}
}

func TestLoad_ServerURL(t *testing.T) {
	t.Setenv("JIRA_DOMAIN", "test.atlassian.net")
	t.Setenv("JIRA_USER", "user@test.com")
	t.Setenv("JIRA_API_TOKEN", "token")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ServerURL() != "https://test.atlassian.net" {
		t.Errorf("ServerURL() = %q, want %q", cfg.ServerURL(), "https://test.atlassian.net")
	}
}

func TestLoad_RepoPath(t *testing.T) {
	t.Setenv("JIRA_DOMAIN", "test.atlassian.net")
	t.Setenv("JIRA_USER", "user@test.com")
	t.Setenv("JIRA_API_TOKEN", "token")
	t.Setenv("JIRA_REPO_PATH", "/home/user/myrepo")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RepoPath != "/home/user/myrepo" {
		t.Errorf("RepoPath = %q, want %q", cfg.RepoPath, "/home/user/myrepo")
	}
}

func TestLoad_RepoPathEmpty(t *testing.T) {
	t.Setenv("JIRA_DOMAIN", "test.atlassian.net")
	t.Setenv("JIRA_USER", "user@test.com")
	t.Setenv("JIRA_API_TOKEN", "token")
	t.Setenv("JIRA_REPO_PATH", "")
	t.Setenv("HOME", t.TempDir())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RepoPath != "" {
		t.Errorf("RepoPath = %q, want empty", cfg.RepoPath)
	}
}

func TestPartialLoad_RepoPath(t *testing.T) {
	t.Setenv("JIRA_DOMAIN", "test.atlassian.net")
	t.Setenv("JIRA_USER", "user@test.com")
	t.Setenv("JIRA_API_TOKEN", "token")
	t.Setenv("JIRA_REPO_PATH", "/repos/project")
	t.Setenv("HOME", t.TempDir())

	cfg, missing := PartialLoad()
	if len(missing) != 0 {
		t.Fatalf("unexpected missing fields: %v", missing)
	}
	if cfg.RepoPath != "/repos/project" {
		t.Errorf("RepoPath = %q, want %q", cfg.RepoPath, "/repos/project")
	}
}

func TestResetConfig_ClearsEnvVars(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	// Set all env vars that ResetConfig should clear.
	for _, k := range []string{
		"JIRA_DOMAIN", "JIRA_URL", "JIRA_USER", "JIRA_USERNAME",
		"JIRA_API_TOKEN", "JIRA_AUTH_TYPE", "JIRA_BOARD_ID",
		"JIRA_PROJECT", "JIRA_REPO_PATH",
	} {
		t.Setenv(k, "some-value")
	}

	if err := ResetConfig(); err != nil {
		t.Fatalf("ResetConfig failed: %v", err)
	}

	for _, k := range []string{
		"JIRA_DOMAIN", "JIRA_URL", "JIRA_USER", "JIRA_USERNAME",
		"JIRA_API_TOKEN", "JIRA_AUTH_TYPE", "JIRA_BOARD_ID",
		"JIRA_PROJECT", "JIRA_REPO_PATH",
	} {
		if v := os.Getenv(k); v != "" {
			t.Errorf("env %s = %q, want empty after reset", k, v)
		}
	}
}

func TestResetConfig_RemovesConfigFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	cfgDir := filepath.Join(dir, ".config", "jiru")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(cfgDir, "config.env")
	if err := os.WriteFile(cfgPath, []byte("export JIRA_DOMAIN=\"test\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := ResetConfig(); err != nil {
		t.Fatalf("ResetConfig failed: %v", err)
	}

	if _, err := os.Stat(cfgPath); !os.IsNotExist(err) {
		t.Error("config.env should be removed after reset")
	}
}

func TestLoad_RepoPathExpandsTilde(t *testing.T) {
	t.Setenv("JIRA_DOMAIN", "test.atlassian.net")
	t.Setenv("JIRA_USER", "user@test.com")
	t.Setenv("JIRA_API_TOKEN", "token")
	t.Setenv("JIRA_REPO_PATH", "~/projects/myrepo")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	home, _ := os.UserHomeDir()
	want := filepath.Join(home, "projects/myrepo")
	if cfg.RepoPath != want {
		t.Errorf("RepoPath = %q, want %q (tilde expanded)", cfg.RepoPath, want)
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
		{"~/projects", filepath.Join(home, "projects")},
		{"~", home},
		{"/absolute", "/absolute"},
		{"relative", "relative"},
		{"", ""},
	}
	for _, tt := range tests {
		got := expandTilde(tt.input)
		if got != tt.want {
			t.Errorf("expandTilde(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestResetConfig_NoConfigFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	// No config file exists — should not error.
	if err := ResetConfig(); err != nil {
		t.Fatalf("ResetConfig should not error when config file doesn't exist: %v", err)
	}
}
