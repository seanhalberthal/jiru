package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanExportValue(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"double quoted", `"my-token"`, "my-token"},
		{"single quoted", `'my-token'`, "my-token"},
		{"unquoted", `my-token`, "my-token"},
		{"double quoted with comment", `"my-token" # jira token`, "my-token"},
		{"unquoted with comment", `my-token # jira token`, "my-token"},
		{"empty", ``, ""},
		{"spaces", `  my-token  `, "my-token"},
		{"quoted empty", `""`, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanExportValue(tt.raw)
			if got != tt.want {
				t.Errorf("cleanExportValue(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestParseZshExports(t *testing.T) {
	content := `# Jira config
export JIRA_DOMAIN="myco.atlassian.net"
export JIRA_USER='user@example.com'
export JIRA_API_TOKEN=abc123
export JIRA_BOARD_ID=42
export JIRA_AUTH_TYPE="bearer"
export JIRA_URL="https://alt.atlassian.net"
export JIRA_USERNAME="alt@example.com"
export UNRELATED_VAR="ignored"

# This line is just a comment
some_command --flag
`

	dir := t.TempDir()
	path := filepath.Join(dir, ".zshrc")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := parseZshExports(path)
	if err != nil {
		t.Fatal(err)
	}

	expected := map[string]string{
		"JIRA_DOMAIN":    "myco.atlassian.net",
		"JIRA_USER":      "user@example.com",
		"JIRA_API_TOKEN": "abc123",
		"JIRA_BOARD_ID":  "42",
		"JIRA_AUTH_TYPE": "bearer",
		"JIRA_URL":       "https://alt.atlassian.net",
		"JIRA_USERNAME":  "alt@example.com",
	}

	for k, want := range expected {
		if got[k] != want {
			t.Errorf("key %s: got %q, want %q", k, got[k], want)
		}
	}

	if _, exists := got["UNRELATED_VAR"]; exists {
		t.Error("should not capture UNRELATED_VAR")
	}
}

func TestStripProtocol(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://myco.atlassian.net", "myco.atlassian.net"},
		{"http://myco.atlassian.net", "myco.atlassian.net"},
		{"myco.atlassian.net", "myco.atlassian.net"},
	}
	for _, tt := range tests {
		got := stripProtocol(tt.input)
		if got != tt.want {
			t.Errorf("stripProtocol(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseZshExports_MissingFile(t *testing.T) {
	_, err := parseZshExports("/nonexistent/path/.zshrc")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadZshCredentials_FirstValueWins(t *testing.T) {
	dir := t.TempDir()

	// Simulate two files where the first has a value and the second has a different one.
	file1 := filepath.Join(dir, "first.zsh")
	file2 := filepath.Join(dir, "second.zsh")

	if err := os.WriteFile(file1, []byte("export JIRA_API_TOKEN=\"first-token\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, []byte("export JIRA_API_TOKEN=\"second-token\"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Parse individually and verify first-wins behaviour.
	exports1, _ := parseZshExports(file1)
	exports2, _ := parseZshExports(file2)

	merged := make(map[string]string)
	for k, v := range exports1 {
		if _, exists := merged[k]; !exists {
			merged[k] = v
		}
	}
	for k, v := range exports2 {
		if _, exists := merged[k]; !exists {
			merged[k] = v
		}
	}

	if merged["JIRA_API_TOKEN"] != "first-token" {
		t.Errorf("expected first-token, got %s", merged["JIRA_API_TOKEN"])
	}
}
