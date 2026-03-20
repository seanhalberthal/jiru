package config

import (
	"fmt"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestWriteConfigProfile_StoresTokenInKeyring(t *testing.T) {
	keyring.MockInit()

	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", dir)

	cfg := &Config{
		Domain:   "test.atlassian.net",
		User:     "user@test.com",
		APIToken: "secret-token",
		AuthType: "basic",
	}
	if err := WriteConfigProfile("default", cfg); err != nil {
		t.Fatalf("WriteConfigProfile failed: %v", err)
	}

	// Token should be in the keyring.
	got, err := keyring.Get(keyringService, keyringUser)
	if err != nil {
		t.Fatalf("keyring.Get failed: %v", err)
	}
	if got != "secret-token" {
		t.Errorf("keyring token = %q, want %q", got, "secret-token")
	}

	// Token should NOT be in profiles.yml.
	store, err := LoadProfiles()
	if err != nil {
		t.Fatalf("LoadProfiles failed: %v", err)
	}
	if store.Profiles["default"].APIToken != "" {
		t.Error("profiles.yml should not contain API token")
	}
}

func TestWriteConfigProfile_ErrorsWhenKeyringUnavailable(t *testing.T) {
	keyring.MockInitWithError(fmt.Errorf("keyring unavailable"))

	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", dir)

	cfg := &Config{
		Domain:   "test.atlassian.net",
		User:     "user@test.com",
		APIToken: "fallback-token",
		AuthType: "basic",
	}
	err := WriteConfigProfile("default", cfg)
	if err == nil {
		t.Fatal("WriteConfigProfile should fail when keyring is unavailable")
	}
	if got := err.Error(); got != "failed to store API token in keychain: keyring unavailable" {
		t.Errorf("error = %q, want mention of keychain", got)
	}
}

func TestResetConfig_ClearsKeyringTokens(t *testing.T) {
	keyring.MockInit()

	// Set tokens for multiple profiles.
	_ = keyring.Set(keyringService, keyringUser, "default-secret")
	_ = keyring.Set(keyringService, keyringUserForProfile("staging"), "staging-secret")

	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", dir)

	store := &ProfileStore{
		Active: "default",
		Profiles: map[string]Config{
			"default": {Domain: "default.atlassian.net"},
			"staging": {Domain: "staging.atlassian.net"},
		},
	}
	writeTestProfiles(t, dir, store)

	if err := ResetConfig(); err != nil {
		t.Fatalf("ResetConfig failed: %v", err)
	}

	if _, err := keyring.Get(keyringService, keyringUser); err == nil {
		t.Error("expected default keyring token to be deleted after reset")
	}
	if _, err := keyring.Get(keyringService, keyringUserForProfile("staging")); err == nil {
		t.Error("expected staging keyring token to be deleted after reset")
	}
}

func TestWriteConfigProfile_StoresRepoPath(t *testing.T) {
	keyring.MockInit()

	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", dir)

	cfg := &Config{
		Domain:   "test.atlassian.net",
		User:     "user@test.com",
		APIToken: "token",
		AuthType: "basic",
		RepoPath: "/home/user/repo",
	}
	if err := WriteConfigProfile("default", cfg); err != nil {
		t.Fatalf("WriteConfigProfile failed: %v", err)
	}

	store, err := LoadProfiles()
	if err != nil {
		t.Fatalf("LoadProfiles failed: %v", err)
	}
	if store.Profiles["default"].RepoPath != "/home/user/repo" {
		t.Errorf("RepoPath = %q, want %q", store.Profiles["default"].RepoPath, "/home/user/repo")
	}
}

func TestWriteConfigProfile_OmitsEmptyRepoPath(t *testing.T) {
	keyring.MockInit()

	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", dir)

	cfg := &Config{
		Domain:   "test.atlassian.net",
		User:     "user@test.com",
		APIToken: "token",
		AuthType: "basic",
	}
	if err := WriteConfigProfile("default", cfg); err != nil {
		t.Fatalf("WriteConfigProfile failed: %v", err)
	}

	store, err := LoadProfiles()
	if err != nil {
		t.Fatalf("LoadProfiles failed: %v", err)
	}
	if store.Profiles["default"].RepoPath != "" {
		t.Errorf("RepoPath = %q, want empty", store.Profiles["default"].RepoPath)
	}
}

func TestLoadProfile_UsesProfiles(t *testing.T) {
	keyring.MockInit()

	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", dir)
	t.Setenv("JIRA_DOMAIN", "")
	t.Setenv("JIRA_USER", "")
	t.Setenv("JIRA_API_TOKEN", "")
	t.Setenv("JIRA_URL", "")
	t.Setenv("JIRA_USERNAME", "")

	// Create a profile.
	store := &ProfileStore{
		Active: "default",
		Profiles: map[string]Config{
			"default": {
				Domain:   "profile.atlassian.net",
				User:     "profile@test.com",
				AuthType: "basic",
			},
		},
	}
	writeTestProfiles(t, dir, store)
	_ = keyring.Set(keyringService, keyringUser, "profile-token")

	cfg, err := LoadProfile("")
	if err != nil {
		t.Fatalf("LoadProfile failed: %v", err)
	}

	if cfg.Domain != "profile.atlassian.net" {
		t.Errorf("Domain = %q, want %q", cfg.Domain, "profile.atlassian.net")
	}
	if cfg.User != "profile@test.com" {
		t.Errorf("User = %q, want %q", cfg.User, "profile@test.com")
	}
}
