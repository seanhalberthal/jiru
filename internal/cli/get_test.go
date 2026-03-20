package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/seanhalberthal/jiru/internal/config"
	"github.com/seanhalberthal/jiru/internal/jira"
)

func TestGetCmd_SuccessfulFetch(t *testing.T) {
	// Verify that a valid issue key produces JSON output containing the
	// expected fields.
	issue := &jira.Issue{
		Key:       "PROJ-42",
		Summary:   "Fix the widget",
		Status:    "In Progress",
		Priority:  "High",
		Assignee:  "alice",
		Reporter:  "bob",
		IssueType: "Bug",
		Created:   time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC),
		Updated:   time.Date(2025, 6, 2, 14, 30, 0, 0, time.UTC),
	}

	stub := &stubClient{
		cfg:   &config.Config{Project: "PROJ"},
		issue: issue,
	}
	cleanup := setStubClient(stub)
	defer cleanup()

	// Capture stdout.
	out := captureStdout(t, func() {
		cmd := GetCmd()
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		cmd.SetArgs([]string{"PROJ-42"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("GetCmd returned error: %v", err)
		}
	})

	var decoded jira.Issue
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, out)
	}

	if decoded.Key != "PROJ-42" {
		t.Errorf("key = %q, want %q", decoded.Key, "PROJ-42")
	}
	if decoded.Summary != "Fix the widget" {
		t.Errorf("summary = %q, want %q", decoded.Summary, "Fix the widget")
	}
	if decoded.Status != "In Progress" {
		t.Errorf("status = %q, want %q", decoded.Status, "In Progress")
	}
	if decoded.Priority != "High" {
		t.Errorf("priority = %q, want %q", decoded.Priority, "High")
	}
}

func TestGetCmd_IssueNotFound(t *testing.T) {
	stub := &stubClient{
		cfg:      &config.Config{Project: "PROJ"},
		issueErr: errors.New("issue not found"),
	}
	cleanup := setStubClient(stub)
	defer cleanup()

	cmd := GetCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"PROJ-999"})
	err := cmd.Execute()

	if err == nil {
		t.Fatal("expected error for missing issue, got nil")
	}
	if err.Error() != "issue not found" {
		t.Errorf("error = %q, want %q", err.Error(), "issue not found")
	}
}

func TestGetCmd_InvalidIssueKey(t *testing.T) {
	// Even with a valid client, invalid keys should be rejected before
	// the API call is made.
	stub := &stubClient{
		cfg: &config.Config{Project: "PROJ"},
	}
	cleanup := setStubClient(stub)
	defer cleanup()

	invalidKeys := []string{
		"lowercase-123",
		"PROJ",
		"123",
		"",
	}

	for _, key := range invalidKeys {
		t.Run(key, func(t *testing.T) {
			cmd := GetCmd()
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true
			args := []string{}
			if key != "" {
				args = append(args, key)
			}
			cmd.SetArgs(args)
			err := cmd.Execute()

			if err == nil {
				t.Errorf("expected error for key %q, got nil", key)
			}
		})
	}
}

func TestGetCmd_APIError(t *testing.T) {
	stub := &stubClient{
		cfg:      &config.Config{Project: "PROJ"},
		issueErr: errors.New("502 Bad Gateway"),
	}
	cleanup := setStubClient(stub)
	defer cleanup()

	cmd := GetCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"PROJ-1"})
	err := cmd.Execute()

	if err == nil {
		t.Fatal("expected error from API failure, got nil")
	}
	if err.Error() != "502 Bad Gateway" {
		t.Errorf("error = %q, want %q", err.Error(), "502 Bad Gateway")
	}
}

func TestGetCmd_IssueWithComments(t *testing.T) {
	// Verify that comments are included in the JSON output.
	issue := &jira.Issue{
		Key:       "PROJ-10",
		Summary:   "Issue with comments",
		Status:    "Open",
		IssueType: "Task",
		Comments: []jira.Comment{
			{Author: "alice", Body: "First comment", Created: time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)},
			{Author: "bob", Body: "Second comment", Created: time.Date(2025, 1, 2, 11, 0, 0, 0, time.UTC)},
		},
	}

	stub := &stubClient{
		cfg:   &config.Config{Project: "PROJ"},
		issue: issue,
	}
	cleanup := setStubClient(stub)
	defer cleanup()

	out := captureStdout(t, func() {
		cmd := GetCmd()
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		cmd.SetArgs([]string{"PROJ-10"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("GetCmd returned error: %v", err)
		}
	})

	var decoded jira.Issue
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, out)
	}

	if len(decoded.Comments) != 2 {
		t.Fatalf("comments count = %d, want 2", len(decoded.Comments))
	}
	if decoded.Comments[0].Author != "alice" {
		t.Errorf("first comment author = %q, want %q", decoded.Comments[0].Author, "alice")
	}
	if decoded.Comments[1].Body != "Second comment" {
		t.Errorf("second comment body = %q, want %q", decoded.Comments[1].Body, "Second comment")
	}
}

func TestGetCmd_IssueWithLabelsAndParent(t *testing.T) {
	// Verify that labels and parent fields round-trip through JSON.
	issue := &jira.Issue{
		Key:           "PROJ-5",
		Summary:       "Child task",
		Status:        "To Do",
		IssueType:     "Sub-task",
		Labels:        []string{"backend", "urgent"},
		ParentKey:     "PROJ-1",
		ParentType:    "Epic",
		ParentSummary: "Parent epic",
	}

	stub := &stubClient{
		cfg:   &config.Config{Project: "PROJ"},
		issue: issue,
	}
	cleanup := setStubClient(stub)
	defer cleanup()

	out := captureStdout(t, func() {
		cmd := GetCmd()
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		cmd.SetArgs([]string{"PROJ-5"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("GetCmd returned error: %v", err)
		}
	})

	var decoded jira.Issue
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if len(decoded.Labels) != 2 {
		t.Fatalf("labels count = %d, want 2", len(decoded.Labels))
	}
	if decoded.Labels[0] != "backend" || decoded.Labels[1] != "urgent" {
		t.Errorf("labels = %v, want [backend urgent]", decoded.Labels)
	}
	if decoded.ParentKey != "PROJ-1" {
		t.Errorf("parent_key = %q, want %q", decoded.ParentKey, "PROJ-1")
	}
	if decoded.ParentType != "Epic" {
		t.Errorf("parent_type = %q, want %q", decoded.ParentType, "Epic")
	}
}

// captureStdout redirects os.Stdout to a pipe, runs fn, and returns the
// captured output as bytes.
func captureStdout(t *testing.T, fn func()) []byte {
	t.Helper()
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	fn()

	_ = w.Close()
	os.Stdout = origStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
