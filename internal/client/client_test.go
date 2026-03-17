package client

import (
	"encoding/json"
	"testing"

	jiracli "github.com/ankitpokhrel/jira-cli/pkg/jira"
	"github.com/seanhalberthal/jiru/internal/jira"
)

func TestEnrichWithParents_PopulatesFields(t *testing.T) {
	issues := []jira.Issue{
		{Key: "A-1", ParentKey: "A-100"},
		{Key: "A-2", ParentKey: "A-200"},
		{Key: "A-3"}, // no parent
	}
	parents := map[string]ParentInfo{
		"A-100": {Key: "A-100", Summary: "My Epic", IssueType: "Epic"},
		"A-200": {Key: "A-200", Summary: "My Feature", IssueType: "Feature"},
	}

	result := EnrichWithParents(issues, parents)

	if result[0].ParentSummary != "My Epic" {
		t.Errorf("expected 'My Epic', got %q", result[0].ParentSummary)
	}
	if result[0].ParentType != "Epic" {
		t.Errorf("expected 'Epic', got %q", result[0].ParentType)
	}
	if result[1].ParentSummary != "My Feature" {
		t.Errorf("expected 'My Feature', got %q", result[1].ParentSummary)
	}
	// Issue without parent should be unchanged.
	if result[2].ParentSummary != "" {
		t.Errorf("expected empty ParentSummary, got %q", result[2].ParentSummary)
	}
}

func TestEnrichWithParents_NilMap(t *testing.T) {
	issues := []jira.Issue{{Key: "A-1", ParentKey: "A-100"}}
	result := EnrichWithParents(issues, nil)
	if result[0].ParentSummary != "" {
		t.Errorf("expected empty ParentSummary with nil map, got %q", result[0].ParentSummary)
	}
}

func TestEnrichWithParents_MissingParent(t *testing.T) {
	issues := []jira.Issue{{Key: "A-1", ParentKey: "A-999"}}
	parents := map[string]ParentInfo{
		"A-100": {Key: "A-100", Summary: "Not this one"},
	}
	result := EnrichWithParents(issues, parents)
	if result[0].ParentSummary != "" {
		t.Errorf("expected empty ParentSummary for missing parent, got %q", result[0].ParentSummary)
	}
}

func TestEnrichWithParents_EmptySlice(t *testing.T) {
	result := EnrichWithParents(nil, nil)
	if result != nil {
		t.Errorf("expected nil result for nil input, got %v", result)
	}
}

// --- convertIssue tests ---

// issueFromJSON is a test helper that creates a jiracli.Issue from JSON.
func issueFromJSON(t *testing.T, raw string) *jiracli.Issue {
	t.Helper()
	var iss jiracli.Issue
	if err := json.Unmarshal([]byte(raw), &iss); err != nil {
		t.Fatalf("failed to unmarshal test issue: %v", err)
	}
	return &iss
}

func TestConvertIssue_BasicFields(t *testing.T) {
	iss := issueFromJSON(t, `{
		"key": "TEST-1",
		"fields": {
			"summary": "Test summary",
			"description": "A plain string description",
			"status": {"name": "In Progress"},
			"priority": {"name": "High"},
			"assignee": {"displayName": "alice"},
			"reporter": {"displayName": "bob"},
			"issueType": {"name": "Story"},
			"labels": ["backend", "urgent"]
		}
	}`)
	result := convertIssue(iss)
	if result.Key != "TEST-1" {
		t.Errorf("Key = %q, want %q", result.Key, "TEST-1")
	}
	if result.Summary != "Test summary" {
		t.Errorf("Summary = %q, want %q", result.Summary, "Test summary")
	}
	if result.Status != "In Progress" {
		t.Errorf("Status = %q, want %q", result.Status, "In Progress")
	}
	if result.Description != "A plain string description" {
		t.Errorf("Description = %q, want %q", result.Description, "A plain string description")
	}
	if result.Priority != "High" {
		t.Errorf("Priority = %q, want %q", result.Priority, "High")
	}
	if result.Assignee != "alice" {
		t.Errorf("Assignee = %q, want %q", result.Assignee, "alice")
	}
	if result.Reporter != "bob" {
		t.Errorf("Reporter = %q, want %q", result.Reporter, "bob")
	}
	if result.IssueType != "Story" {
		t.Errorf("IssueType = %q, want %q", result.IssueType, "Story")
	}
	if len(result.Labels) != 2 {
		t.Errorf("Labels len = %d, want 2", len(result.Labels))
	}
}

func TestConvertIssue_NilParent(t *testing.T) {
	iss := issueFromJSON(t, `{
		"key": "TEST-2",
		"fields": {"summary": "No parent"}
	}`)
	result := convertIssue(iss)
	if result.ParentKey != "" {
		t.Errorf("expected empty ParentKey for nil parent, got %q", result.ParentKey)
	}
}

func TestConvertIssue_WithParent(t *testing.T) {
	iss := issueFromJSON(t, `{
		"key": "TEST-4",
		"fields": {
			"summary": "Has parent",
			"parent": {"key": "EPIC-1"}
		}
	}`)
	result := convertIssue(iss)
	if result.ParentKey != "EPIC-1" {
		t.Errorf("ParentKey = %q, want %q", result.ParentKey, "EPIC-1")
	}
}

func TestConvertIssue_NonStringDescription(t *testing.T) {
	// V3 API returns an ADF object for description — should be treated as empty string.
	iss := issueFromJSON(t, `{
		"key": "TEST-3",
		"fields": {
			"summary": "ADF desc",
			"description": {"type": "doc", "content": []}
		}
	}`)
	result := convertIssue(iss)
	if result.Description != "" {
		t.Errorf("expected empty description for non-string, got %q", result.Description)
	}
}

func TestConvertIssue_Comments(t *testing.T) {
	iss := issueFromJSON(t, `{
		"key": "TEST-5",
		"fields": {
			"summary": "With comments",
			"comment": {
				"comments": [
					{"author": {"displayName": "alice"}, "body": "looks good"},
					{"author": {"displayName": "bob"}, "body": {"type": "doc"}}
				],
				"total": 2
			}
		}
	}`)
	result := convertIssue(iss)
	if len(result.Comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(result.Comments))
	}
	if result.Comments[0].Author != "alice" {
		t.Errorf("Comment[0].Author = %q, want %q", result.Comments[0].Author, "alice")
	}
	if result.Comments[0].Body != "looks good" {
		t.Errorf("Comment[0].Body = %q, want %q", result.Comments[0].Body, "looks good")
	}
	if result.Comments[1].Body != "" {
		t.Errorf("Comment[1].Body = %q, want empty (non-string body)", result.Comments[1].Body)
	}
}

func TestConvertIssue_EmptyFields(t *testing.T) {
	iss := issueFromJSON(t, `{"key": "TEST-6", "fields": {}}`)
	result := convertIssue(iss)
	if result.Key != "TEST-6" {
		t.Errorf("Key = %q, want %q", result.Key, "TEST-6")
	}
	if result.Summary != "" {
		t.Errorf("Summary = %q, want empty", result.Summary)
	}
}

// --- convertIssue timestamp tests ---

func TestConvertIssue_ValidTimestamps(t *testing.T) {
	iss := issueFromJSON(t, `{
		"key": "TS-1",
		"fields": {
			"summary": "Timestamp test",
			"created": "2024-01-15T10:30:45.123+0000",
			"updated": "2024-06-20T14:22:33.456+1000"
		}
	}`)
	result := convertIssue(iss)

	if result.Created.IsZero() {
		t.Error("Created should be parsed, got zero time")
	}
	if result.Created.Year() != 2024 || result.Created.Month() != 1 || result.Created.Day() != 15 {
		t.Errorf("Created = %v, want 2024-01-15", result.Created)
	}
	if result.Created.Hour() != 10 || result.Created.Minute() != 30 {
		t.Errorf("Created time = %v, want 10:30", result.Created)
	}

	if result.Updated.IsZero() {
		t.Error("Updated should be parsed, got zero time")
	}
	if result.Updated.Year() != 2024 || result.Updated.Month() != 6 {
		t.Errorf("Updated = %v, want 2024-06", result.Updated)
	}
}

func TestConvertIssue_EmptyTimestamps(t *testing.T) {
	iss := issueFromJSON(t, `{
		"key": "TS-2",
		"fields": {
			"summary": "No timestamps"
		}
	}`)
	result := convertIssue(iss)
	if !result.Created.IsZero() {
		t.Errorf("Created should be zero for missing timestamp, got %v", result.Created)
	}
	if !result.Updated.IsZero() {
		t.Errorf("Updated should be zero for missing timestamp, got %v", result.Updated)
	}
}

func TestConvertIssue_MalformedTimestamp(t *testing.T) {
	iss := issueFromJSON(t, `{
		"key": "TS-3",
		"fields": {
			"summary": "Bad timestamps",
			"created": "not-a-date",
			"updated": "2024/01/15"
		}
	}`)
	result := convertIssue(iss)

	// Malformed timestamps should silently produce zero time.
	if !result.Created.IsZero() {
		t.Errorf("Created should be zero for malformed timestamp, got %v", result.Created)
	}
	if !result.Updated.IsZero() {
		t.Errorf("Updated should be zero for malformed timestamp, got %v", result.Updated)
	}
}

func TestConvertIssue_AlternateTimezoneOffset(t *testing.T) {
	iss := issueFromJSON(t, `{
		"key": "TS-4",
		"fields": {
			"summary": "Negative offset",
			"created": "2024-03-10T08:15:00.000-0500"
		}
	}`)
	result := convertIssue(iss)
	if result.Created.IsZero() {
		t.Error("Created should be parsed for negative timezone offset")
	}
	if result.Created.Hour() != 8 {
		t.Errorf("Created hour = %d, want 8 (local time in -0500)", result.Created.Hour())
	}
}

func TestConvertIssue_ISO8601ColonOffset(t *testing.T) {
	// Some Jira instances return "+00:00" instead of "+0000".
	// The current parser layout "2006-01-02T15:04:05.000-0700" won't match this.
	iss := issueFromJSON(t, `{
		"key": "TS-5",
		"fields": {
			"summary": "Colon offset",
			"created": "2024-01-15T10:30:45.123+00:00"
		}
	}`)
	result := convertIssue(iss)
	// This format won't parse with the current layout — documents the blind spot.
	if !result.Created.IsZero() {
		t.Log("Created parsed successfully for colon offset — parser may have been updated")
	}
}

// --- JQLEscape tests ---

func TestJqlEscape(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"Done", "Done"},
		{"O'Brien", `O\'Brien`},
		{"It's a test", `It\'s a test`},
		{"no'quotes'here", `no\'quotes\'here`},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := JQLEscape(tt.input)
			if got != tt.want {
				t.Errorf("JQLEscape(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
