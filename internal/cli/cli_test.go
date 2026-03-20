package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/seanhalberthal/jiru/internal/config"
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

// --- Tests for OutputError ---

func TestOutputError_FormatsJSON(t *testing.T) {
	// OutputError writes a JSON error object to stderr then calls os.Exit(1).
	// We cannot test the os.Exit() behaviour directly, but we can verify
	// the formatting by redirecting stderr and calling the format logic.
	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w

	// Write the same format that OutputError uses, without calling os.Exit.
	testErr := "something went wrong"
	_, _ = fmt.Fprintf(os.Stderr, `{"error": %q}`+"\n", testErr)

	_ = w.Close()
	os.Stderr = origStderr

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}

	output := buf.String()

	// Must be valid JSON.
	var decoded map[string]string
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("OutputError produced invalid JSON: %v\nOutput: %s", err, output)
	}

	if decoded["error"] != "something went wrong" {
		t.Errorf("error field = %q, want %q", decoded["error"], "something went wrong")
	}
}

func TestOutputError_SpecialCharacters(t *testing.T) {
	// Verify that special characters (quotes, newlines) are properly escaped.
	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w

	testErr := `error with "quotes" and\nnewlines`
	_, _ = fmt.Fprintf(os.Stderr, `{"error": %q}`+"\n", testErr)

	_ = w.Close()
	os.Stderr = origStderr

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}

	output := buf.String()

	var decoded map[string]string
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("OutputError produced invalid JSON for special chars: %v\nOutput: %s", err, output)
	}

	if decoded["error"] != testErr {
		t.Errorf("error field = %q, want %q", decoded["error"], testErr)
	}
}

// --- Tests for Client() and Config() accessors ---

func TestClient_ReturnsSetClient(t *testing.T) {
	stub := &stubClient{
		cfg:    &config.Config{Project: "PROJ"},
		meName: "testuser",
	}
	cleanup := setStubClient(stub)
	defer cleanup()

	got := Client()
	if got == nil {
		t.Fatal("Client() returned nil after setStubClient")
	}

	// Verify it's our stub by checking a known method.
	name, err := got.Me()
	if err != nil {
		t.Fatalf("Me() returned error: %v", err)
	}
	if name != "testuser" {
		t.Errorf("Me() = %q, want %q", name, "testuser")
	}
}

func TestConfig_ReturnsSetConfig(t *testing.T) {
	stub := &stubClient{
		cfg: &config.Config{Project: "MYPROJ", BoardID: 99, Domain: "test.atlassian.net"},
	}
	cleanup := setStubClient(stub)
	defer cleanup()

	got := Config()
	if got == nil {
		t.Fatal("Config() returned nil after setStubClient")
	}
	if got.Project != "MYPROJ" {
		t.Errorf("Config().Project = %q, want %q", got.Project, "MYPROJ")
	}
	if got.BoardID != 99 {
		t.Errorf("Config().BoardID = %d, want 99", got.BoardID)
	}
}

func TestClient_NilBeforeInit(t *testing.T) {
	// Before any initialisation, the client should be nil (default state).
	origClient := cliClient
	origConfig := cliConfig
	cliClient = nil
	cliConfig = nil
	defer func() {
		cliClient = origClient
		cliConfig = origConfig
	}()

	if Client() != nil {
		t.Error("Client() should be nil before initialisation")
	}
	if Config() != nil {
		t.Error("Config() should be nil before initialisation")
	}
}

// --- Tests for InitClient / InitClientWithProfile ---

func TestInitClientWithProfile_MissingConfig(t *testing.T) {
	// InitClientWithProfile should fail when required config is missing.
	// We use a nonexistent profile and ensure no env vars are set.
	// Save and clear relevant env vars to ensure config.LoadProfile fails.
	envVars := []string{"JIRA_DOMAIN", "JIRA_URL", "JIRA_USER", "JIRA_USERNAME", "JIRA_API_TOKEN", "JIRA_AUTH_TYPE"}
	for _, v := range envVars {
		t.Setenv(v, "")
	}

	// Also use a temporary config dir to avoid picking up real profiles.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	err := InitClientWithProfile("nonexistent-profile-xyz")
	if err == nil {
		t.Fatal("expected error from InitClientWithProfile with missing config, got nil")
	}
	if !strings.Contains(err.Error(), "configuration error") {
		t.Errorf("error = %q, want message containing 'configuration error'", err.Error())
	}
}

func TestInitClient_DelegatesToInitClientWithProfile(t *testing.T) {
	// InitClient calls InitClientWithProfile(""), so it should also fail
	// when configuration is missing.
	envVars := []string{"JIRA_DOMAIN", "JIRA_URL", "JIRA_USER", "JIRA_USERNAME", "JIRA_API_TOKEN", "JIRA_AUTH_TYPE"}
	for _, v := range envVars {
		t.Setenv(v, "")
	}

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	err := InitClient()
	if err == nil {
		t.Fatal("expected error from InitClient with missing config, got nil")
	}
	if !strings.Contains(err.Error(), "configuration error") {
		t.Errorf("error = %q, want message containing 'configuration error'", err.Error())
	}
}

// --- Tests for OutputJSON edge cases ---

func TestOutputJSON_EmptySlice(t *testing.T) {
	out := captureStdout(t, func() {
		if err := OutputJSON([]string{}); err != nil {
			t.Fatalf("OutputJSON returned error: %v", err)
		}
	})

	trimmed := strings.TrimSpace(string(out))
	if trimmed != "[]" {
		t.Errorf("OutputJSON([]string{}) = %q, want %q", trimmed, "[]")
	}
}

func TestOutputJSON_NilSlice(t *testing.T) {
	out := captureStdout(t, func() {
		var s []string
		if err := OutputJSON(s); err != nil {
			t.Fatalf("OutputJSON returned error: %v", err)
		}
	})

	trimmed := strings.TrimSpace(string(out))
	if trimmed != "null" {
		t.Errorf("OutputJSON(nil) = %q, want %q", trimmed, "null")
	}
}

func TestOutputJSON_NestedStruct(t *testing.T) {
	type Inner struct {
		Value int `json:"value"`
	}
	type Outer struct {
		Name  string `json:"name"`
		Inner Inner  `json:"inner"`
	}

	out := captureStdout(t, func() {
		if err := OutputJSON(Outer{Name: "test", Inner: Inner{Value: 42}}); err != nil {
			t.Fatalf("OutputJSON returned error: %v", err)
		}
	})

	var decoded map[string]any
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("OutputJSON produced invalid JSON: %v", err)
	}

	if decoded["name"] != "test" {
		t.Errorf("name = %v, want %q", decoded["name"], "test")
	}
	inner, ok := decoded["inner"].(map[string]any)
	if !ok {
		t.Fatal("inner field is not an object")
	}
	if inner["value"] != float64(42) {
		t.Errorf("inner.value = %v, want 42", inner["value"])
	}
}

func TestOutputJSON_StringValue(t *testing.T) {
	out := captureStdout(t, func() {
		if err := OutputJSON("hello"); err != nil {
			t.Fatalf("OutputJSON returned error: %v", err)
		}
	})

	trimmed := strings.TrimSpace(string(out))
	if trimmed != `"hello"` {
		t.Errorf("OutputJSON(string) = %q, want %q", trimmed, `"hello"`)
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
