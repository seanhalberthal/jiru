package cli

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/undont/jiru/internal/config"
)

func TestCommentCmd_Success(t *testing.T) {
	stub := &stubClient{cfg: &config.Config{Project: "PROJ"}}
	cleanup := setStubClient(stub)
	defer cleanup()

	out := captureStdout(t, func() {
		cmd := CommentCmd()
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		cmd.SetArgs([]string{"PROJ-1", "This is a comment"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("CommentCmd returned error: %v", err)
		}
	})

	var result map[string]any
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, out)
	}
	if result["ok"] != true {
		t.Errorf("ok = %v, want true", result["ok"])
	}
	if result["key"] != "PROJ-1" {
		t.Errorf("key = %v, want PROJ-1", result["key"])
	}
}

func TestCommentCmd_InvalidKey(t *testing.T) {
	stub := &stubClient{cfg: &config.Config{Project: "PROJ"}}
	cleanup := setStubClient(stub)
	defer cleanup()

	cmd := CommentCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"bad-key", "comment"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid key")
	}
}

func TestCommentCmd_EmptyBody(t *testing.T) {
	stub := &stubClient{cfg: &config.Config{Project: "PROJ"}}
	cleanup := setStubClient(stub)
	defer cleanup()

	// Empty string literal resolves to empty body
	cmd := CommentCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	// Need 2 args: issue key + body. An empty body string still counts as an arg.
	cmd.SetArgs([]string{"PROJ-1", ""})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for empty body")
	}
}

func TestCommentCmd_APIError(t *testing.T) {
	stub := &stubClient{
		cfg:           &config.Config{Project: "PROJ"},
		addCommentErr: errors.New("permission denied"),
	}
	cleanup := setStubClient(stub)
	defer cleanup()

	cmd := CommentCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"PROJ-1", "comment"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error from API failure")
	}
}
