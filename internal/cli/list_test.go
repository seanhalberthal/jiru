package cli

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/seanhalberthal/jiru/internal/config"
	"github.com/seanhalberthal/jiru/internal/jira"
)

func TestListCmd_ActiveSprint(t *testing.T) {
	// When the board has an active sprint, ListCmd should fetch sprint issues.
	issues := []jira.Issue{
		{Key: "PROJ-1", Summary: "First issue", Status: "Open", IssueType: "Task"},
		{Key: "PROJ-2", Summary: "Second issue", Status: "In Progress", IssueType: "Bug"},
	}

	stub := &stubClient{
		cfg:          &config.Config{Project: "PROJ", BoardID: 42},
		boardSprints: []jira.Sprint{{ID: 100, Name: "Sprint 1", State: "active"}},
		sprintIssues: issues,
	}
	cleanup := setStubClient(stub)
	defer cleanup()

	out := captureStdout(t, func() {
		cmd := ListCmd()
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		if err := cmd.Execute(); err != nil {
			t.Fatalf("ListCmd returned error: %v", err)
		}
	})

	var decoded []jira.Issue
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, out)
	}

	if len(decoded) != 2 {
		t.Fatalf("issue count = %d, want 2", len(decoded))
	}
	if decoded[0].Key != "PROJ-1" {
		t.Errorf("first issue key = %q, want %q", decoded[0].Key, "PROJ-1")
	}
	if decoded[1].Key != "PROJ-2" {
		t.Errorf("second issue key = %q, want %q", decoded[1].Key, "PROJ-2")
	}
}

func TestListCmd_NoActiveSprint_FallsBackToBoardIssues(t *testing.T) {
	// When no active sprint exists, ListCmd falls back to BoardIssues.
	boardIssues := []jira.Issue{
		{Key: "PROJ-10", Summary: "Board issue", Status: "To Do", IssueType: "Story"},
	}

	stub := &stubClient{
		cfg:          &config.Config{Project: "PROJ", BoardID: 42},
		boardSprints: []jira.Sprint{}, // No active sprints.
		boardIssues:  boardIssues,
	}
	cleanup := setStubClient(stub)
	defer cleanup()

	out := captureStdout(t, func() {
		cmd := ListCmd()
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		if err := cmd.Execute(); err != nil {
			t.Fatalf("ListCmd returned error: %v", err)
		}
	})

	var decoded []jira.Issue
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, out)
	}

	if len(decoded) != 1 {
		t.Fatalf("issue count = %d, want 1", len(decoded))
	}
	if decoded[0].Key != "PROJ-10" {
		t.Errorf("issue key = %q, want %q", decoded[0].Key, "PROJ-10")
	}
}

func TestListCmd_NoBoardConfigured(t *testing.T) {
	// When BoardID is 0 (not set), ListCmd should return a clear error.
	stub := &stubClient{
		cfg: &config.Config{Project: "PROJ", BoardID: 0},
	}
	cleanup := setStubClient(stub)
	defer cleanup()

	cmd := ListCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	err := cmd.Execute()

	if err == nil {
		t.Fatal("expected error when no board is configured, got nil")
	}

	msg := err.Error()
	if !contains(msg, "no board configured") {
		t.Errorf("error = %q, want message containing 'no board configured'", msg)
	}
}

func TestListCmd_BoardSprintsError(t *testing.T) {
	stub := &stubClient{
		cfg:          &config.Config{Project: "PROJ", BoardID: 42},
		boardSprtErr: errors.New("API timeout"),
	}
	cleanup := setStubClient(stub)
	defer cleanup()

	cmd := ListCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	err := cmd.Execute()

	if err == nil {
		t.Fatal("expected error from BoardSprints failure, got nil")
	}
	if err.Error() != "API timeout" {
		t.Errorf("error = %q, want %q", err.Error(), "API timeout")
	}
}

func TestListCmd_SprintIssuesError(t *testing.T) {
	stub := &stubClient{
		cfg:          &config.Config{Project: "PROJ", BoardID: 42},
		boardSprints: []jira.Sprint{{ID: 100, Name: "Sprint 1", State: "active"}},
		sprintIssErr: errors.New("forbidden"),
	}
	cleanup := setStubClient(stub)
	defer cleanup()

	cmd := ListCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	err := cmd.Execute()

	if err == nil {
		t.Fatal("expected error from SprintIssues failure, got nil")
	}
	if err.Error() != "forbidden" {
		t.Errorf("error = %q, want %q", err.Error(), "forbidden")
	}
}

func TestListCmd_BoardIssuesFallbackError(t *testing.T) {
	// When no active sprint and BoardIssues also fails.
	stub := &stubClient{
		cfg:          &config.Config{Project: "PROJ", BoardID: 42},
		boardSprints: []jira.Sprint{},
		boardIssErr:  errors.New("board not found"),
	}
	cleanup := setStubClient(stub)
	defer cleanup()

	cmd := ListCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	err := cmd.Execute()

	if err == nil {
		t.Fatal("expected error from BoardIssues fallback failure, got nil")
	}
	if err.Error() != "board not found" {
		t.Errorf("error = %q, want %q", err.Error(), "board not found")
	}
}

func TestListCmd_EmptySprintIssues(t *testing.T) {
	// An active sprint with no issues should produce an empty JSON array.
	stub := &stubClient{
		cfg:          &config.Config{Project: "PROJ", BoardID: 42},
		boardSprints: []jira.Sprint{{ID: 100, Name: "Sprint 1", State: "active"}},
		sprintIssues: []jira.Issue{},
	}
	cleanup := setStubClient(stub)
	defer cleanup()

	out := captureStdout(t, func() {
		cmd := ListCmd()
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		if err := cmd.Execute(); err != nil {
			t.Fatalf("ListCmd returned error: %v", err)
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

func TestListCmd_MultipleSprints_UsesFirst(t *testing.T) {
	// When multiple sprints are returned, ListCmd should use the first one.
	// We verify this indirectly by checking that the issues come from the
	// configured sprint issues (our stub doesn't distinguish by sprint ID,
	// but the code always picks sprints[0]).
	stub := &stubClient{
		cfg: &config.Config{Project: "PROJ", BoardID: 42},
		boardSprints: []jira.Sprint{
			{ID: 100, Name: "Sprint 10", State: "active"},
			{ID: 101, Name: "Sprint 11", State: "active"},
		},
		sprintIssues: []jira.Issue{
			{Key: "PROJ-1", Summary: "Only issue", Status: "Open", IssueType: "Task"},
		},
	}
	cleanup := setStubClient(stub)
	defer cleanup()

	out := captureStdout(t, func() {
		cmd := ListCmd()
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		if err := cmd.Execute(); err != nil {
			t.Fatalf("ListCmd returned error: %v", err)
		}
	})

	var decoded []jira.Issue
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if len(decoded) != 1 {
		t.Fatalf("issue count = %d, want 1", len(decoded))
	}
}

func TestListCmd_RejectsArgs(t *testing.T) {
	// ListCmd takes no arguments; passing any should fail.
	cmd := ListCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"unexpected"})
	err := cmd.Execute()

	if err == nil {
		t.Fatal("expected error for unexpected argument, got nil")
	}
}
