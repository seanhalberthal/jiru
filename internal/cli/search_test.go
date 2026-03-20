package cli

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/seanhalberthal/jiru/internal/config"
	"github.com/seanhalberthal/jiru/internal/jira"
)

func TestSearchCmd_SuccessfulSearch(t *testing.T) {
	issues := []jira.Issue{
		{Key: "PROJ-1", Summary: "Match one", Status: "Open", IssueType: "Task"},
		{Key: "PROJ-2", Summary: "Match two", Status: "Done", IssueType: "Story"},
		{Key: "PROJ-3", Summary: "Match three", Status: "In Progress", IssueType: "Bug"},
	}

	stub := &stubClient{
		cfg:          &config.Config{Project: "PROJ"},
		searchIssues: issues,
	}
	cleanup := setStubClient(stub)
	defer cleanup()

	out := captureStdout(t, func() {
		cmd := SearchCmd()
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		cmd.SetArgs([]string{"status = Open"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("SearchCmd returned error: %v", err)
		}
	})

	var decoded []jira.Issue
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, out)
	}

	if len(decoded) != 3 {
		t.Fatalf("issue count = %d, want 3", len(decoded))
	}
	if decoded[0].Key != "PROJ-1" {
		t.Errorf("first issue key = %q, want %q", decoded[0].Key, "PROJ-1")
	}
	if decoded[2].Status != "In Progress" {
		t.Errorf("third issue status = %q, want %q", decoded[2].Status, "In Progress")
	}
}

func TestSearchCmd_EmptyResults(t *testing.T) {
	stub := &stubClient{
		cfg:          &config.Config{Project: "PROJ"},
		searchIssues: []jira.Issue{},
	}
	cleanup := setStubClient(stub)
	defer cleanup()

	out := captureStdout(t, func() {
		cmd := SearchCmd()
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		cmd.SetArgs([]string{"project = NONEXISTENT"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("SearchCmd returned error: %v", err)
		}
	})

	var decoded []jira.Issue
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, out)
	}

	if len(decoded) != 0 {
		t.Errorf("expected empty array, got %d items", len(decoded))
	}
}

func TestSearchCmd_APIError(t *testing.T) {
	stub := &stubClient{
		cfg:       &config.Config{Project: "PROJ"},
		searchErr: errors.New("JQL syntax error"),
	}
	cleanup := setStubClient(stub)
	defer cleanup()

	cmd := SearchCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"invalid jql syntax !!"})
	err := cmd.Execute()

	if err == nil {
		t.Fatal("expected error from SearchJQL failure, got nil")
	}
	if err.Error() != "JQL syntax error" {
		t.Errorf("error = %q, want %q", err.Error(), "JQL syntax error")
	}
}

func TestSearchCmd_NoArgs(t *testing.T) {
	cmd := SearchCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{})
	err := cmd.Execute()

	if err == nil {
		t.Fatal("expected error for missing JQL argument, got nil")
	}
}

func TestSearchCmd_TooManyArgs(t *testing.T) {
	cmd := SearchCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"status = Open", "extra arg"})
	err := cmd.Execute()

	if err == nil {
		t.Fatal("expected error for too many arguments, got nil")
	}
}

func TestSearchCmd_NilIssuesReturned(t *testing.T) {
	// When the client returns nil (not an empty slice), the JSON output
	// should still be valid.
	stub := &stubClient{
		cfg:          &config.Config{Project: "PROJ"},
		searchIssues: nil,
	}
	cleanup := setStubClient(stub)
	defer cleanup()

	out := captureStdout(t, func() {
		cmd := SearchCmd()
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		cmd.SetArgs([]string{"status = Open"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("SearchCmd returned error: %v", err)
		}
	})

	// nil slice encodes as "null" in JSON, which is valid.
	if !json.Valid(out) {
		t.Fatalf("output is not valid JSON: %s", out)
	}
}

func TestSearchCmd_ComplexJQL(t *testing.T) {
	// Verify that complex JQL strings pass through correctly.
	issues := []jira.Issue{
		{Key: "PROJ-42", Summary: "Complex query result", Status: "Open", IssueType: "Task"},
	}

	stub := &stubClient{
		cfg:          &config.Config{Project: "PROJ"},
		searchIssues: issues,
	}
	cleanup := setStubClient(stub)
	defer cleanup()

	out := captureStdout(t, func() {
		cmd := SearchCmd()
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		cmd.SetArgs([]string{`project = PROJ AND status IN ("Open", "In Progress") ORDER BY updated DESC`})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("SearchCmd returned error: %v", err)
		}
	})

	var decoded []jira.Issue
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if len(decoded) != 1 {
		t.Fatalf("issue count = %d, want 1", len(decoded))
	}
	if decoded[0].Key != "PROJ-42" {
		t.Errorf("issue key = %q, want %q", decoded[0].Key, "PROJ-42")
	}
}
