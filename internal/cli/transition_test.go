package cli

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/undont/jiru/internal/config"
	"github.com/undont/jiru/internal/jira"
)

func TestTransitionCmd_ListMode(t *testing.T) {
	stub := &stubClient{
		cfg: &config.Config{Project: "PROJ"},
		transitions: []jira.Transition{
			{ID: "1", Name: "Start Progress", ToStatus: "In Progress"},
			{ID: "2", Name: "Done", ToStatus: "Done"},
		},
	}
	cleanup := setStubClient(stub)
	defer cleanup()

	out := captureStdout(t, func() {
		cmd := TransitionCmd()
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		cmd.SetArgs([]string{"PROJ-1"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("TransitionCmd returned error: %v", err)
		}
	})

	var decoded []jira.Transition
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, out)
	}
	if len(decoded) != 2 {
		t.Fatalf("got %d transitions, want 2", len(decoded))
	}
}

func TestTransitionCmd_ExecuteMode(t *testing.T) {
	stub := &stubClient{
		cfg:   &config.Config{Project: "PROJ"},
		issue: &jira.Issue{Key: "PROJ-1", Status: "In Progress"},
	}
	cleanup := setStubClient(stub)
	defer cleanup()

	out := captureStdout(t, func() {
		cmd := TransitionCmd()
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		cmd.SetArgs([]string{"PROJ-1", "1"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("TransitionCmd returned error: %v", err)
		}
	})

	var result map[string]any
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, out)
	}
	if result["ok"] != true {
		t.Errorf("ok = %v, want true", result["ok"])
	}
}

func TestTransitionCmd_InvalidKey(t *testing.T) {
	stub := &stubClient{cfg: &config.Config{Project: "PROJ"}}
	cleanup := setStubClient(stub)
	defer cleanup()

	cmd := TransitionCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"bad-key"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid key")
	}
}

func TestTransitionCmd_ListAPIError(t *testing.T) {
	stub := &stubClient{
		cfg:            &config.Config{Project: "PROJ"},
		transitionsErr: errors.New("not found"),
	}
	cleanup := setStubClient(stub)
	defer cleanup()

	cmd := TransitionCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"PROJ-1"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error from API failure")
	}
}
