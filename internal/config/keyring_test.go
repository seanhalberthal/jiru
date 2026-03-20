package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestWriteConfigProfile_StoresTokenInKeyring(t *testing.T) {
	keyring.MockInit()

	dir := t.TempDir()
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

	// Token should NOT be in profiles.yaml.
	store, err := LoadProfiles()
	if err != nil {
		t.Fatalf("LoadProfiles failed: %v", err)
	}
	if store.Profiles["default"].APIToken != "" {
		t.Error("profiles.yaml should not contain API token")
	}
}

func TestWriteConfigProfile_ErrorsWhenKeyringUnavailable(t *testing.T) {
	keyring.MockInitWithError(fmt.Errorf("keyring unavailable"))

	dir := t.TempDir()
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

func TestMigrateToProfiles_MigratesConfigEnv(t *testing.T) {
	keyring.MockInit()

	dir := t.TempDir()
	t.Setenv("HOME", dir)

	// Create a legacy config.env file.
	cfgDir := filepath.Join(dir, ".config", "jiru")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	content := "export JIRA_DOMAIN=\"legacy.atlassian.net\"\nexport JIRA_USER=\"legacy@test.com\"\nexport JIRA_AUTH_TYPE=\"basic\"\nexport JIRA_PROJECT=\"LEG\"\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "config.env"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	// Store a token in the legacy keyring key.
	if err := keyring.Set(keyringService, keyringUser, "legacy-token"); err != nil {
		t.Fatal(err)
	}

	if err := MigrateToProfiles(); err != nil {
		t.Fatalf("MigrateToProfiles failed: %v", err)
	}

	// profiles.yaml should now exist with the migrated data.
	store, err := LoadProfiles()
	if err != nil {
		t.Fatalf("LoadProfiles failed: %v", err)
	}
	if store == nil {
		t.Fatal("expected non-nil store after migration")
	}
	if store.Active != "default" {
		t.Errorf("Active = %q, want %q", store.Active, "default")
	}
	p, ok := store.Profiles["default"]
	if !ok {
		t.Fatal("default profile not found after migration")
	}
	if p.Domain != "legacy.atlassian.net" {
		t.Errorf("Domain = %q, want %q", p.Domain, "legacy.atlassian.net")
	}
	if p.User != "legacy@test.com" {
		t.Errorf("User = %q, want %q", p.User, "legacy@test.com")
	}
	if p.Project != "LEG" {
		t.Errorf("Project = %q, want %q", p.Project, "LEG")
	}

	// Token should be in the profile keyring.
	token, err := keyring.Get(keyringService, keyringUser)
	if err != nil {
		t.Fatalf("keyring.Get failed: %v", err)
	}
	if token != "legacy-token" {
		t.Errorf("keyring token = %q, want %q", token, "legacy-token")
	}

	// Legacy config.env should be cleaned up.
	if _, err := os.Stat(filepath.Join(cfgDir, "config.env")); !os.IsNotExist(err) {
		t.Error("config.env should be deleted after migration")
	}
}

func TestMigrateToProfiles_IdempotentAfterMigration(t *testing.T) {
	keyring.MockInit()

	dir := t.TempDir()
	t.Setenv("HOME", dir)

	// Create profiles.yaml directly (already migrated).
	store := &ProfileStore{
		Active: "default",
		Profiles: map[string]Config{
			"default": {Domain: "existing.atlassian.net"},
		},
	}
	writeTestProfiles(t, dir, store)

	// Running migration again should be a no-op.
	if err := MigrateToProfiles(); err != nil {
		t.Fatalf("MigrateToProfiles failed: %v", err)
	}

	reloaded, err := LoadProfiles()
	if err != nil {
		t.Fatalf("LoadProfiles failed: %v", err)
	}
	if reloaded.Profiles["default"].Domain != "existing.atlassian.net" {
		t.Error("migration should not overwrite existing profiles")
	}
}

func TestMigrateToProfiles_NoConfigEnvIsNoOp(t *testing.T) {
	keyring.MockInit()

	dir := t.TempDir()
	t.Setenv("HOME", dir)

	// No config.env, no profiles.yaml — should be a no-op.
	if err := MigrateToProfiles(); err != nil {
		t.Fatalf("MigrateToProfiles failed: %v", err)
	}

	store, err := LoadProfiles()
	if err != nil {
		t.Fatalf("LoadProfiles failed: %v", err)
	}
	if store != nil {
		t.Error("expected nil store when no config exists to migrate")
	}
}

func TestLoadProfile_UsesProfilesNotConfigEnv(t *testing.T) {
	keyring.MockInit()

	dir := t.TempDir()
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

	// Also create a legacy config.env with different values (should be ignored).
	cfgDir := filepath.Join(dir, ".config", "jiru")
	content := "export JIRA_DOMAIN=\"legacy.atlassian.net\"\nexport JIRA_USER=\"legacy@test.com\"\n"
	_ = os.WriteFile(filepath.Join(cfgDir, "config.env"), []byte(content), 0o600)

	cfg, err := LoadProfile("")
	if err != nil {
		t.Fatalf("LoadProfile failed: %v", err)
	}

	// Should load from profile, not config.env.
	if cfg.Domain != "profile.atlassian.net" {
		t.Errorf("Domain = %q, want %q (from profile, not config.env)", cfg.Domain, "profile.atlassian.net")
	}
	if cfg.User != "profile@test.com" {
		t.Errorf("User = %q, want %q (from profile, not config.env)", cfg.User, "profile@test.com")
	}
}
