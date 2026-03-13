package config

import "testing"

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
