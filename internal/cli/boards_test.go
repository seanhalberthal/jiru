package cli

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/seanhalberthal/jiru/internal/config"
	"github.com/seanhalberthal/jiru/internal/jira"
)

func TestBoardsCmd_SuccessfulList(t *testing.T) {
	boards := []jira.Board{
		{ID: 1, Name: "Team Alpha", Type: "scrum"},
		{ID: 2, Name: "Team Beta", Type: "kanban"},
		{ID: 3, Name: "Team Gamma", Type: "scrum"},
	}

	stub := &stubClient{
		cfg:    &config.Config{Project: "PROJ"},
		boards: boards,
	}
	cleanup := setStubClient(stub)
	defer cleanup()

	out := captureStdout(t, func() {
		cmd := BoardsCmd()
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		if err := cmd.Execute(); err != nil {
			t.Fatalf("BoardsCmd returned error: %v", err)
		}
	})

	var decoded []jira.Board
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, out)
	}

	if len(decoded) != 3 {
		t.Fatalf("board count = %d, want 3", len(decoded))
	}
	if decoded[0].Name != "Team Alpha" {
		t.Errorf("first board name = %q, want %q", decoded[0].Name, "Team Alpha")
	}
	if decoded[1].Type != "kanban" {
		t.Errorf("second board type = %q, want %q", decoded[1].Type, "kanban")
	}
	if decoded[2].ID != 3 {
		t.Errorf("third board id = %d, want 3", decoded[2].ID)
	}
}

func TestBoardsCmd_EmptyList(t *testing.T) {
	stub := &stubClient{
		cfg:    &config.Config{Project: "PROJ"},
		boards: []jira.Board{},
	}
	cleanup := setStubClient(stub)
	defer cleanup()

	out := captureStdout(t, func() {
		cmd := BoardsCmd()
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		if err := cmd.Execute(); err != nil {
			t.Fatalf("BoardsCmd returned error: %v", err)
		}
	})

	var decoded []jira.Board
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, out)
	}

	if len(decoded) != 0 {
		t.Errorf("expected empty array, got %d items", len(decoded))
	}
}

func TestBoardsCmd_APIError(t *testing.T) {
	stub := &stubClient{
		cfg:       &config.Config{Project: "PROJ"},
		boardsErr: errors.New("permission denied"),
	}
	cleanup := setStubClient(stub)
	defer cleanup()

	cmd := BoardsCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	err := cmd.Execute()

	if err == nil {
		t.Fatal("expected error from Boards failure, got nil")
	}
	if err.Error() != "permission denied" {
		t.Errorf("error = %q, want %q", err.Error(), "permission denied")
	}
}

func TestBoardsCmd_RejectsArgs(t *testing.T) {
	cmd := BoardsCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"unexpected"})
	err := cmd.Execute()

	if err == nil {
		t.Fatal("expected error for unexpected argument, got nil")
	}
}

func TestBoardsCmd_NilBoardsReturned(t *testing.T) {
	// When the client returns nil (not an empty slice), output should still
	// be valid JSON.
	stub := &stubClient{
		cfg:    &config.Config{Project: "PROJ"},
		boards: nil,
	}
	cleanup := setStubClient(stub)
	defer cleanup()

	out := captureStdout(t, func() {
		cmd := BoardsCmd()
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		if err := cmd.Execute(); err != nil {
			t.Fatalf("BoardsCmd returned error: %v", err)
		}
	})

	if !json.Valid(out) {
		t.Fatalf("output is not valid JSON: %s", out)
	}
}

func TestBoardsCmd_SingleBoard(t *testing.T) {
	// Verify a single board serialises correctly with all fields.
	stub := &stubClient{
		cfg: &config.Config{Project: "PROJ"},
		boards: []jira.Board{
			{ID: 99, Name: "Solo Board", Type: "kanban"},
		},
	}
	cleanup := setStubClient(stub)
	defer cleanup()

	out := captureStdout(t, func() {
		cmd := BoardsCmd()
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		if err := cmd.Execute(); err != nil {
			t.Fatalf("BoardsCmd returned error: %v", err)
		}
	})

	var decoded []jira.Board
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if len(decoded) != 1 {
		t.Fatalf("board count = %d, want 1", len(decoded))
	}

	board := decoded[0]
	if board.ID != 99 {
		t.Errorf("board id = %d, want 99", board.ID)
	}
	if board.Name != "Solo Board" {
		t.Errorf("board name = %q, want %q", board.Name, "Solo Board")
	}
	if board.Type != "kanban" {
		t.Errorf("board type = %q, want %q", board.Type, "kanban")
	}
}
