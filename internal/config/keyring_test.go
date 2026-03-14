package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestWriteConfig_StoresTokenInKeyring(t *testing.T) {
	keyring.MockInit()

	dir := t.TempDir()
	t.Setenv("HOME", dir)
	cfgDir := filepath.Join(dir, ".config", "jiratui")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{
		Domain:   "test.atlassian.net",
		User:     "user@test.com",
		APIToken: "secret-token",
		AuthType: "basic",
	}
	if err := WriteConfig(cfg); err != nil {
		t.Fatalf("WriteConfig failed: %v", err)
	}

	// Token should be in the keyring.
	got, err := keyring.Get(keyringService, keyringUser)
	if err != nil {
		t.Fatalf("keyring.Get failed: %v", err)
	}
	if got != "secret-token" {
		t.Errorf("keyring token = %q, want %q", got, "secret-token")
	}

	// Token should NOT be in the config file.
	data, err := os.ReadFile(filepath.Join(cfgDir, "config.env"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "JIRA_API_TOKEN") {
		t.Error("config.env should not contain JIRA_API_TOKEN when keyring is available")
	}
}

func TestWriteConfig_ErrorsWhenKeyringUnavailable(t *testing.T) {
	keyring.MockInitWithError(fmt.Errorf("keyring unavailable"))

	dir := t.TempDir()
	t.Setenv("HOME", dir)
	cfgDir := filepath.Join(dir, ".config", "jiratui")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{
		Domain:   "test.atlassian.net",
		User:     "user@test.com",
		APIToken: "fallback-token",
		AuthType: "basic",
	}
	err := WriteConfig(cfg)
	if err == nil {
		t.Fatal("WriteConfig should fail when keyring is unavailable")
	}
	if !strings.Contains(err.Error(), "keychain") {
		t.Errorf("error should mention keychain, got: %v", err)
	}
}

func TestApplyConfigFile_ReadsTokenFromKeyring(t *testing.T) {
	keyring.MockInit()
	if err := keyring.Set(keyringService, keyringUser, "keyring-token"); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	t.Setenv("HOME", dir)
	// Write a config file without the token.
	cfgDir := filepath.Join(dir, ".config", "jiratui")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	content := "export JIRA_DOMAIN=\"test.atlassian.net\"\nexport JIRA_USER=\"user@test.com\"\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "config.env"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{AuthType: "basic"}
	cfg.applyConfigFile()

	if cfg.APIToken != "keyring-token" {
		t.Errorf("APIToken = %q, want %q", cfg.APIToken, "keyring-token")
	}
}

func TestApplyConfigFile_MigratesFileTokenToKeyring(t *testing.T) {
	keyring.MockInit()

	dir := t.TempDir()
	t.Setenv("HOME", dir)
	cfgDir := filepath.Join(dir, ".config", "jiratui")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	content := "export JIRA_DOMAIN=\"test.atlassian.net\"\nexport JIRA_API_TOKEN=\"file-token\"\nexport JIRA_AUTH_TYPE=\"basic\"\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "config.env"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{AuthType: "basic"}
	cfg.applyConfigFile()

	// Token should be loaded.
	if cfg.APIToken != "file-token" {
		t.Errorf("APIToken = %q, want %q", cfg.APIToken, "file-token")
	}

	// Token should now be in the keyring.
	got, err := keyring.Get(keyringService, keyringUser)
	if err != nil {
		t.Fatalf("keyring.Get failed: %v", err)
	}
	if got != "file-token" {
		t.Errorf("keyring token = %q, want %q", got, "file-token")
	}

	// Token should be removed from the config file.
	data, err := os.ReadFile(filepath.Join(cfgDir, "config.env"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "JIRA_API_TOKEN") {
		t.Error("config.env should no longer contain JIRA_API_TOKEN after migration")
	}
	// Other config values should be preserved.
	if !strings.Contains(string(data), "JIRA_DOMAIN") {
		t.Error("config.env should still contain JIRA_DOMAIN after migration")
	}
}

func TestApplyConfigFile_KeyringWithNoConfigFile(t *testing.T) {
	keyring.MockInit()
	if err := keyring.Set(keyringService, keyringUser, "keyring-only-token"); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	t.Setenv("HOME", dir)
	// No config file exists.

	cfg := &Config{AuthType: "basic"}
	cfg.applyConfigFile()

	if cfg.APIToken != "keyring-only-token" {
		t.Errorf("APIToken = %q, want %q", cfg.APIToken, "keyring-only-token")
	}
}
