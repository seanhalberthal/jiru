package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
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

func TestResetConfig_RemovesProfilesAndLegacyConfig(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	cfgDir := filepath.Join(dir, ".config", "jiru")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}

	// Create profiles.yaml.
	store := &ProfileStore{
		Active: "default",
		Profiles: map[string]Config{
			"default": {Domain: "test.atlassian.net", User: "user@test.com"},
			"staging": {Domain: "staging.atlassian.net", User: "staging@test.com"},
		},
	}
	writeTestProfiles(t, dir, store)

	// Set keyring tokens for both profiles.
	_ = keyring.Set(keyringService, keyringUserForProfile("default"), "default-token")
	_ = keyring.Set(keyringService, keyringUserForProfile("staging"), "staging-token")

	// Also create a legacy config.env.
	if err := os.WriteFile(filepath.Join(cfgDir, "config.env"), []byte("export JIRA_DOMAIN=\"test\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := ResetConfig(); err != nil {
		t.Fatalf("ResetConfig failed: %v", err)
	}

	// profiles.yaml should be removed.
	if _, err := os.Stat(filepath.Join(cfgDir, "profiles.yaml")); !os.IsNotExist(err) {
		t.Error("profiles.yaml should be removed after reset")
	}
	// Legacy config.env should also be removed.
	if _, err := os.Stat(filepath.Join(cfgDir, "config.env")); !os.IsNotExist(err) {
		t.Error("config.env should be removed after reset")
	}
	// Keyring entries should be gone.
	if _, err := keyring.Get(keyringService, keyringUserForProfile("default")); err == nil {
		t.Error("expected default profile keyring entry to be deleted")
	}
	if _, err := keyring.Get(keyringService, keyringUserForProfile("staging")); err == nil {
		t.Error("expected staging profile keyring entry to be deleted")
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

func TestWriteConfigProfile_CreatesProfile(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	cfg := &Config{
		Domain:   "test.atlassian.net",
		User:     "user@test.com",
		APIToken: "secret-token",
		AuthType: "basic",
		Project:  "TEST",
		BoardID:  42,
	}

	if err := WriteConfigProfile("default", cfg); err != nil {
		t.Fatalf("WriteConfigProfile failed: %v", err)
	}

	// Verify profile was created in profiles.yaml.
	store, err := LoadProfiles()
	if err != nil {
		t.Fatalf("LoadProfiles failed: %v", err)
	}
	if store == nil {
		t.Fatal("expected non-nil store")
	}
	p, ok := store.Profiles["default"]
	if !ok {
		t.Fatal("default profile not found")
	}
	if p.Domain != "test.atlassian.net" {
		t.Errorf("Domain = %q, want %q", p.Domain, "test.atlassian.net")
	}
	if p.User != "user@test.com" {
		t.Errorf("User = %q, want %q", p.User, "user@test.com")
	}
	if p.Project != "TEST" {
		t.Errorf("Project = %q, want %q", p.Project, "TEST")
	}
	if p.BoardID != 42 {
		t.Errorf("BoardID = %d, want 42", p.BoardID)
	}
	if p.APIToken != "" {
		t.Error("profile should not contain API token (should be in keyring only)")
	}

	// Token should be in keyring.
	token, err := keyring.Get(keyringService, keyringUser)
	if err != nil {
		t.Fatalf("keyring.Get failed: %v", err)
	}
	if token != "secret-token" {
		t.Errorf("keyring token = %q, want %q", token, "secret-token")
	}

	// config.env should NOT be created.
	configEnvPath := filepath.Join(dir, ".config", "jiru", "config.env")
	if _, err := os.Stat(configEnvPath); !os.IsNotExist(err) {
		t.Error("config.env should not be created")
	}
}

func TestWriteConfigProfile_FilePermissions(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	cfg := &Config{
		Domain:   "test.atlassian.net",
		User:     "user@test.com",
		APIToken: "token",
		AuthType: "basic",
	}

	_ = WriteConfigProfile("default", cfg)

	path := filepath.Join(dir, ".config", "jiru", "profiles.yaml")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("profiles.yaml permissions = %o, want 0600", perm)
	}
}

func TestWriteConfigProfile_DoesNotSetTokenInEnv(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	// Clear any existing token env var.
	t.Setenv("JIRA_API_TOKEN", "")

	cfg := &Config{
		Domain:   "test.atlassian.net",
		User:     "user@test.com",
		APIToken: "secret-token",
		AuthType: "basic",
	}

	_ = WriteConfigProfile("default", cfg)

	// The token should NOT be in the environment.
	if os.Getenv("JIRA_API_TOKEN") == "secret-token" {
		t.Error("WriteConfigProfile should not set JIRA_API_TOKEN in the process environment")
	}
}

func TestWriteConfigProfile_SetsNonSecretEnvVars(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	cfg := &Config{
		Domain:   "test.atlassian.net",
		User:     "user@test.com",
		APIToken: "token",
		AuthType: "bearer",
	}

	_ = WriteConfigProfile("default", cfg)

	if os.Getenv("JIRA_DOMAIN") != "test.atlassian.net" {
		t.Error("expected JIRA_DOMAIN to be set in env")
	}
	if os.Getenv("JIRA_AUTH_TYPE") != "bearer" {
		t.Error("expected JIRA_AUTH_TYPE to be set in env")
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

func TestLoad_DefaultAuthType(t *testing.T) {
	t.Setenv("JIRA_DOMAIN", "test.atlassian.net")
	t.Setenv("JIRA_USER", "user@test.com")
	t.Setenv("JIRA_API_TOKEN", "token")
	t.Setenv("JIRA_AUTH_TYPE", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AuthType != "basic" {
		t.Errorf("AuthType = %q, want %q (default)", cfg.AuthType, "basic")
	}
}

func TestLoad_DefaultBranchMode(t *testing.T) {
	t.Setenv("JIRA_DOMAIN", "test.atlassian.net")
	t.Setenv("JIRA_USER", "user@test.com")
	t.Setenv("JIRA_API_TOKEN", "token")
	t.Setenv("JIRA_BRANCH_MODE", "")
	t.Setenv("HOME", t.TempDir())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.BranchMode != "local" {
		t.Errorf("BranchMode = %q, want %q (default)", cfg.BranchMode, "local")
	}
}

func TestLoad_InvalidBranchMode(t *testing.T) {
	t.Setenv("JIRA_DOMAIN", "test.atlassian.net")
	t.Setenv("JIRA_USER", "user@test.com")
	t.Setenv("JIRA_API_TOKEN", "token")
	t.Setenv("JIRA_BRANCH_MODE", "invalid")

	_, err := Load()
	if err == nil {
		t.Error("expected error for invalid JIRA_BRANCH_MODE")
	}
	if err != nil && !strings.Contains(err.Error(), "JIRA_BRANCH_MODE") {
		t.Errorf("error should mention JIRA_BRANCH_MODE, got: %v", err)
	}
}

func TestLoad_BranchUppercase(t *testing.T) {
	t.Setenv("JIRA_DOMAIN", "test.atlassian.net")
	t.Setenv("JIRA_USER", "user@test.com")
	t.Setenv("JIRA_API_TOKEN", "token")
	t.Setenv("JIRA_BRANCH_UPPERCASE", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.BranchUppercase {
		t.Error("BranchUppercase should be true when JIRA_BRANCH_UPPERCASE=true")
	}
}

func TestPartialLoad_MissingFields(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("JIRA_DOMAIN", "test.atlassian.net")
	t.Setenv("JIRA_USER", "")
	t.Setenv("JIRA_USERNAME", "")
	t.Setenv("JIRA_API_TOKEN", "")
	t.Setenv("JIRA_URL", "")

	cfg, missing := PartialLoad()
	if cfg.Domain != "test.atlassian.net" {
		t.Errorf("Domain = %q, want %q", cfg.Domain, "test.atlassian.net")
	}

	hasUser := false
	hasToken := false
	for _, f := range missing {
		if f == "user" {
			hasUser = true
		}
		if f == "api_token" {
			hasToken = true
		}
	}
	if !hasUser {
		t.Error("expected 'user' in missing fields")
	}
	if !hasToken {
		t.Error("expected 'api_token' in missing fields")
	}
}

func TestPartialLoad_InvalidAuthTypeFallsBack(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("JIRA_DOMAIN", "test.atlassian.net")
	t.Setenv("JIRA_USER", "user@test.com")
	t.Setenv("JIRA_API_TOKEN", "token")
	t.Setenv("JIRA_AUTH_TYPE", "oauth")

	cfg, _ := PartialLoad()
	if cfg.AuthType != "basic" {
		t.Errorf("AuthType = %q, want %q (fallback for invalid)", cfg.AuthType, "basic")
	}
}

func TestPartialLoad_InvalidBranchModeFallsBack(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("JIRA_DOMAIN", "test.atlassian.net")
	t.Setenv("JIRA_USER", "user@test.com")
	t.Setenv("JIRA_API_TOKEN", "token")
	t.Setenv("JIRA_BRANCH_MODE", "invalid")

	cfg, _ := PartialLoad()
	if cfg.BranchMode != "local" {
		t.Errorf("BranchMode = %q, want %q (fallback for invalid)", cfg.BranchMode, "local")
	}
}

func TestStripProtocol_EdgeCases(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://foo.com", "foo.com"},
		{"http://bar.com", "bar.com"},
		{"foo.com", "foo.com"},
		{"", ""},
		{"https://", "https://"},                   // Prefix-only, nothing after it.
		{"http://", "http://"},                     // Prefix-only, nothing after it.
		{"ftp://example.com", "ftp://example.com"}, // Unsupported protocol.
	}
	for _, tt := range tests {
		got := stripProtocol(tt.input)
		if got != tt.want {
			t.Errorf("stripProtocol(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestWriteConfigProfile_StoresBranchMode(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	cfg := &Config{
		Domain:     "test.atlassian.net",
		User:       "user@test.com",
		APIToken:   "token",
		AuthType:   "basic",
		BranchMode: "remote",
	}

	if err := WriteConfigProfile("default", cfg); err != nil {
		t.Fatalf("WriteConfigProfile failed: %v", err)
	}

	store, err := LoadProfiles()
	if err != nil {
		t.Fatalf("LoadProfiles failed: %v", err)
	}
	p := store.Profiles["default"]
	if p.BranchMode != "remote" {
		t.Errorf("BranchMode = %q, want %q", p.BranchMode, "remote")
	}
}

func TestWriteConfigProfile_StoresBranchUppercase(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	cfg := &Config{
		Domain:          "test.atlassian.net",
		User:            "user@test.com",
		APIToken:        "token",
		AuthType:        "basic",
		BranchUppercase: true,
	}

	if err := WriteConfigProfile("default", cfg); err != nil {
		t.Fatalf("WriteConfigProfile failed: %v", err)
	}

	store, err := LoadProfiles()
	if err != nil {
		t.Fatalf("LoadProfiles failed: %v", err)
	}
	p := store.Profiles["default"]
	if !p.BranchUppercase {
		t.Error("BranchUppercase should be true")
	}
}

func TestWriteConfigProfile_NamedProfile(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	cfg := &Config{
		Domain:   "staging.atlassian.net",
		User:     "staging@test.com",
		APIToken: "staging-token",
		AuthType: "bearer",
		Project:  "STG",
	}

	if err := WriteConfigProfile("staging", cfg); err != nil {
		t.Fatalf("WriteConfigProfile failed: %v", err)
	}

	store, err := LoadProfiles()
	if err != nil {
		t.Fatalf("LoadProfiles failed: %v", err)
	}
	p, ok := store.Profiles["staging"]
	if !ok {
		t.Fatal("staging profile not found")
	}
	if p.Domain != "staging.atlassian.net" {
		t.Errorf("Domain = %q, want %q", p.Domain, "staging.atlassian.net")
	}
	if p.AuthType != "bearer" {
		t.Errorf("AuthType = %q, want %q", p.AuthType, "bearer")
	}

	// Token should be in profile-specific keyring key.
	token, err := keyring.Get(keyringService, keyringUserForProfile("staging"))
	if err != nil {
		t.Fatalf("keyring.Get failed: %v", err)
	}
	if token != "staging-token" {
		t.Errorf("keyring token = %q, want %q", token, "staging-token")
	}
}

func TestWriteConfigProfile_EmptyNameDefaultsToDefault(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	cfg := &Config{
		Domain:   "test.atlassian.net",
		User:     "user@test.com",
		APIToken: "token",
		AuthType: "basic",
	}

	if err := WriteConfigProfile("", cfg); err != nil {
		t.Fatalf("WriteConfigProfile failed: %v", err)
	}

	store, err := LoadProfiles()
	if err != nil {
		t.Fatalf("LoadProfiles failed: %v", err)
	}
	if _, ok := store.Profiles["default"]; !ok {
		t.Error("empty profile name should save as 'default'")
	}
}

func TestLoadJiraCliConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	// Create the jira-cli config directory and file.
	jiraDir := filepath.Join(dir, ".config", ".jira")
	if err := os.MkdirAll(jiraDir, 0o700); err != nil {
		t.Fatal(err)
	}
	configContent := `server: https://jira-cli.atlassian.net
login: jira-user@example.com
board:
  id: 99
`
	if err := os.WriteFile(filepath.Join(jiraDir, ".config.yml"), []byte(configContent), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{}
	if err := cfg.loadJiraCliConfig(); err != nil {
		t.Fatalf("loadJiraCliConfig failed: %v", err)
	}

	if cfg.Domain != "jira-cli.atlassian.net" {
		t.Errorf("Domain = %q, want %q", cfg.Domain, "jira-cli.atlassian.net")
	}
	if cfg.User != "jira-user@example.com" {
		t.Errorf("User = %q, want %q", cfg.User, "jira-user@example.com")
	}
	if cfg.BoardID != 99 {
		t.Errorf("BoardID = %d, want 99", cfg.BoardID)
	}
}

func TestLoadJiraCliConfig_DoesNotOverrideExisting(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	// Create the jira-cli config directory and file.
	jiraDir := filepath.Join(dir, ".config", ".jira")
	if err := os.MkdirAll(jiraDir, 0o700); err != nil {
		t.Fatal(err)
	}
	configContent := `server: https://override.atlassian.net
login: override@example.com
board:
  id: 77
`
	if err := os.WriteFile(filepath.Join(jiraDir, ".config.yml"), []byte(configContent), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{
		Domain:  "existing.atlassian.net",
		User:    "existing@test.com",
		BoardID: 42,
	}
	if err := cfg.loadJiraCliConfig(); err != nil {
		t.Fatalf("loadJiraCliConfig failed: %v", err)
	}

	// Existing values should not be overridden.
	if cfg.Domain != "existing.atlassian.net" {
		t.Errorf("Domain = %q, want %q (should not be overridden)", cfg.Domain, "existing.atlassian.net")
	}
	if cfg.User != "existing@test.com" {
		t.Errorf("User = %q, want %q (should not be overridden)", cfg.User, "existing@test.com")
	}
	if cfg.BoardID != 42 {
		t.Errorf("BoardID = %d, want 42 (should not be overridden)", cfg.BoardID)
	}
}

func TestLoadJiraCliConfig_MissingFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	cfg := &Config{}
	err := cfg.loadJiraCliConfig()
	if err == nil {
		t.Error("expected error when jira-cli config file does not exist")
	}
}

func TestLoadJiraCliConfig_NoBoardField(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	jiraDir := filepath.Join(dir, ".config", ".jira")
	if err := os.MkdirAll(jiraDir, 0o700); err != nil {
		t.Fatal(err)
	}
	configContent := `server: https://nobboard.atlassian.net
login: user@example.com
`
	if err := os.WriteFile(filepath.Join(jiraDir, ".config.yml"), []byte(configContent), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{}
	if err := cfg.loadJiraCliConfig(); err != nil {
		t.Fatalf("loadJiraCliConfig failed: %v", err)
	}

	if cfg.Domain != "nobboard.atlassian.net" {
		t.Errorf("Domain = %q, want %q", cfg.Domain, "nobboard.atlassian.net")
	}
	if cfg.BoardID != 0 {
		t.Errorf("BoardID = %d, want 0 when board field missing", cfg.BoardID)
	}
}
