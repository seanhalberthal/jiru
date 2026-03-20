package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"testing"
)

func TestOutputJSON(t *testing.T) {
	// Capture stdout.
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	input := map[string]any{
		"key":     "PROJ-123",
		"summary": "Test issue",
		"count":   float64(42),
	}

	if err := OutputJSON(input); err != nil {
		os.Stdout = origStdout
		t.Fatalf("OutputJSON returned error: %v", err)
	}

	_ = w.Close()
	os.Stdout = origStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}

	output := buf.String()

	// Must be valid JSON.
	var decoded map[string]any
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("OutputJSON produced invalid JSON: %v\nOutput: %s", err, output)
	}

	// Must be indented (contains newlines and leading spaces).
	if !bytes.Contains([]byte(output), []byte("\n  ")) {
		t.Errorf("OutputJSON should produce indented JSON, got:\n%s", output)
	}

	// Verify values round-trip correctly.
	if decoded["key"] != "PROJ-123" {
		t.Errorf("decoded key = %v, want PROJ-123", decoded["key"])
	}
	if decoded["summary"] != "Test issue" {
		t.Errorf("decoded summary = %v, want Test issue", decoded["summary"])
	}
	if decoded["count"] != float64(42) {
		t.Errorf("decoded count = %v, want 42", decoded["count"])
	}
}

func TestGetCmd_ArgCount(t *testing.T) {
	cmd := GetCmd()

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"no args", []string{}, true},
		{"one arg", []string{"PROJ-123"}, false},
		{"two args", []string{"PROJ-123", "PROJ-456"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cmd.Args(cmd, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetCmd.Args(%v) err=%v, wantErr=%v", tt.args, err, tt.wantErr)
			}
		})
	}
}

func TestGetCmd_IssueKeyValidation(t *testing.T) {
	// Invalid keys should produce a validation error from GetCmd's RunE.
	// Valid keys are not tested via Execute() because Client() is nil,
	// which would panic. The Args check + invalid key tests are sufficient
	// to verify the command is wired correctly.
	invalidKeys := []struct {
		key  string
		desc string
	}{
		{"proj-123", "lowercase"},
		{"PROJ", "missing dash and number"},
		{"PROJ-", "missing number"},
		{"123-PROJ", "reversed"},
		{"PROJ 123", "space"},
		{"PROJ-1 OR 1=1", "injection attempt"},
	}

	for _, tt := range invalidKeys {
		t.Run(tt.desc, func(t *testing.T) {
			cmd := GetCmd()
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true
			cmd.SetArgs([]string{tt.key})
			err := cmd.Execute()

			if err == nil {
				t.Errorf("GetCmd(%q) expected validation error, got nil", tt.key)
			} else if !isValidationError(err) {
				t.Errorf("GetCmd(%q) expected validation error, got: %v", tt.key, err)
			}
		})
	}
}

func TestSearchCmd_ArgCount(t *testing.T) {
	cmd := SearchCmd()

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"no args", []string{}, true},
		{"one arg", []string{"status = Open"}, false},
		{"two args", []string{"status = Open", "extra"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cmd.Args(cmd, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("SearchCmd.Args(%v) err=%v, wantErr=%v", tt.args, err, tt.wantErr)
			}
		})
	}
}

func TestListCmd_ArgCount(t *testing.T) {
	cmd := ListCmd()

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"no args", []string{}, false},
		{"one arg", []string{"extra"}, true},
		{"two args", []string{"a", "b"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cmd.Args(cmd, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("ListCmd.Args(%v) err=%v, wantErr=%v", tt.args, err, tt.wantErr)
			}
		})
	}
}

func TestBoardsCmd_ArgCount(t *testing.T) {
	cmd := BoardsCmd()

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{"no args", []string{}, false},
		{"one arg", []string{"extra"}, true},
		{"two args", []string{"a", "b"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cmd.Args(cmd, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("BoardsCmd.Args(%v) err=%v, wantErr=%v", tt.args, err, tt.wantErr)
			}
		})
	}
}

// isValidationError checks whether the error is from issue key validation.
func isValidationError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return len(msg) > 0 && (contains(msg, "invalid issue key") || contains(msg, "must match"))
}

func contains(s, substr string) bool {
	return bytes.Contains([]byte(s), []byte(substr))
}
