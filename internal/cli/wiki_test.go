package cli

import (
	"encoding/json"
	"testing"

	"github.com/undont/jiru/internal/config"
	"github.com/undont/jiru/internal/confluence"
)

func TestWikiSpacesCmd_Success(t *testing.T) {
	stub := &stubClient{
		cfg: &config.Config{Project: "PROJ"},
		confluenceSpaces: []confluence.Space{
			{ID: "1", Key: "ENG", Name: "Engineering"},
		},
	}
	cleanup := setStubClient(stub)
	defer cleanup()

	out := captureStdout(t, func() {
		cmd := WikiCmd()
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		cmd.SetArgs([]string{"spaces"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("wiki spaces returned error: %v", err)
		}
	})

	var decoded []confluence.Space
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, out)
	}
	if len(decoded) != 1 || decoded[0].Key != "ENG" {
		t.Errorf("unexpected spaces: %+v", decoded)
	}
}

func TestWikiPagesCmd_InvalidID(t *testing.T) {
	stub := &stubClient{cfg: &config.Config{Project: "PROJ"}}
	cleanup := setStubClient(stub)
	defer cleanup()

	cmd := WikiCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"pages", "not-numeric"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid space ID")
	}
}

func TestWikiPageCmd_JSON(t *testing.T) {
	stub := &stubClient{
		cfg:            &config.Config{Project: "PROJ"},
		confluencePage: &confluence.Page{ID: "123", Title: "Test Page", BodyADF: `{"type":"doc","version":1,"content":[]}`},
	}
	cleanup := setStubClient(stub)
	defer cleanup()

	out := captureStdout(t, func() {
		cmd := WikiCmd()
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		cmd.SetArgs([]string{"page", "123"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("wiki page returned error: %v", err)
		}
	})

	var decoded confluence.Page
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, out)
	}
	if decoded.ID != "123" {
		t.Errorf("id = %q, want %q", decoded.ID, "123")
	}
}

func TestWikiPageCmd_InvalidID(t *testing.T) {
	stub := &stubClient{cfg: &config.Config{Project: "PROJ"}}
	cleanup := setStubClient(stub)
	defer cleanup()

	cmd := WikiCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"page", "abc/injection"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid page ID")
	}
}

func TestWikiEditCmd_NoFlags(t *testing.T) {
	stub := &stubClient{cfg: &config.Config{Project: "PROJ"}}
	cleanup := setStubClient(stub)
	defer cleanup()

	cmd := WikiCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"edit", "123"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no flags provided")
	}
}

func TestWikiEditCmd_TitleOnly(t *testing.T) {
	stub := &stubClient{
		cfg:            &config.Config{Project: "PROJ"},
		confluencePage: &confluence.Page{ID: "123", Title: "Old Title", Version: 1, BodyADF: `{"type":"doc"}`},
	}
	cleanup := setStubClient(stub)
	defer cleanup()

	out := captureStdout(t, func() {
		cmd := WikiCmd()
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		cmd.SetArgs([]string{"edit", "123", "--title", "New Title"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("wiki edit returned error: %v", err)
		}
	})

	var decoded confluence.Page
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, out)
	}
}

func TestWikiEditCmd_InvalidID(t *testing.T) {
	stub := &stubClient{cfg: &config.Config{Project: "PROJ"}}
	cleanup := setStubClient(stub)
	defer cleanup()

	cmd := WikiCmd()
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"edit", "../../etc", "--title", "bad"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid page ID")
	}
}
