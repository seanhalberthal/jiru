package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/undont/jiru/internal/config"
	"github.com/undont/jiru/internal/jira"
)

func TestResolveInput_Empty(t *testing.T) {
	got, err := resolveInput("")
	if err != nil {
		t.Fatalf("resolveInput empty: %v", err)
	}
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestResolveInput_Literal(t *testing.T) {
	got, err := resolveInput("hello world")
	if err != nil {
		t.Fatalf("resolveInput literal: %v", err)
	}
	if got != "hello world" {
		t.Errorf("got %q, want %q", got, "hello world")
	}
}

func TestResolveInput_File(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "input.md")
	if err := os.WriteFile(path, []byte("file content"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := resolveInput("@" + path)
	if err != nil {
		t.Fatalf("resolveInput file: %v", err)
	}
	if got != "file content" {
		t.Errorf("got %q, want %q", got, "file content")
	}
}

func TestResolveInput_FileMissing(t *testing.T) {
	_, err := resolveInput("@/nonexistent/file.md")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestEditCmd_Summary(t *testing.T) {
	issue := &jira.Issue{
		Key:     "PROJ-1",
		Summary: "Updated summary",
		Status:  "Open",
	}
	stub := &stubClient{
		cfg:   &config.Config{Project: "PROJ"},
		issue: issue,
	}
	cleanup := setStubClient(stub)
	defer cleanup()

	out := captureStdout(t, func() {
		cmd := EditCmd()
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		cmd.SetArgs([]string{"PROJ-1", "--summary", "Updated summary"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("EditCmd returned error: %v", err)
		}
	})

	var decoded jira.Issue
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, out)
	}
	if decoded.Key != "PROJ-1" {
		t.Errorf("key = %q, want %q", decoded.Key, "PROJ-1")
	}
}

func TestEditCmd_NoFlags(t *testing.T) {
	stub := &stubClient{cfg: &config.Config{Project: "PROJ"}}
	cleanup := setStubClient(stub)
	defer cleanup()

	cmd := EditCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"PROJ-1"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for no flags")
	}
}

func TestEditCmd_InvalidKey(t *testing.T) {
	stub := &stubClient{cfg: &config.Config{Project: "PROJ"}}
	cleanup := setStubClient(stub)
	defer cleanup()

	cmd := EditCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"bad-key", "--summary", "test"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid key")
	}
}
