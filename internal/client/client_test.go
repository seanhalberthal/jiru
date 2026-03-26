package client

import (
	"encoding/json"
	"testing"

	"github.com/seanhalberthal/jiru/internal/api"
	"github.com/seanhalberthal/jiru/internal/config"
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

// issueFromJSON is a test helper that creates an api.Issue from JSON.
func issueFromJSON(t *testing.T, raw string) *api.Issue {
	t.Helper()
	var iss api.Issue
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
			"issuetype": {"name": "Story"},
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
	iss := issueFromJSON(t, `{
		"key": "TS-5",
		"fields": {
			"summary": "Colon offset",
			"created": "2024-01-15T10:30:45.123+00:00"
		}
	}`)
	result := convertIssue(iss)
	if result.Created.IsZero() {
		t.Error("Created should parse colon-format offset (+00:00)")
	}
	if result.Created.Year() != 2024 || result.Created.Month() != 1 || result.Created.Day() != 15 {
		t.Errorf("Created = %v, want 2024-01-15", result.Created)
	}
}

func TestParseJiraTime(t *testing.T) {
	tests := []struct {
		name  string
		input string
		ok    bool
	}{
		{"compact offset", "2024-01-15T10:30:00.000+0000", true},
		{"colon offset", "2024-01-15T10:30:00.000+00:00", true},
		{"negative offset", "2024-01-15T10:30:00.000-0500", true},
		{"UTC Z", "2024-01-15T10:30:00.000Z", true},
		{"empty", "", false},
		{"garbage", "not-a-date", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := parseJiraTime(tt.input)
			if ok != tt.ok {
				t.Errorf("parseJiraTime(%q) ok=%v, want %v", tt.input, ok, tt.ok)
			}
		})
	}
}

func TestConvertIssue_CommentTimestamps(t *testing.T) {
	iss := issueFromJSON(t, `{
		"key": "TS-6",
		"fields": {
			"summary": "Comment timestamps",
			"comment": {
				"comments": [
					{"author": {"displayName": "alice"}, "body": "first", "created": "2024-03-10T14:30:00.000+0000"},
					{"author": {"displayName": "bob"}, "body": "second"}
				],
				"total": 2
			}
		}
	}`)
	result := convertIssue(iss)
	if len(result.Comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(result.Comments))
	}
	if result.Comments[0].Created.IsZero() {
		t.Error("Comment[0].Created should be parsed")
	}
	if result.Comments[0].Created.Hour() != 14 {
		t.Errorf("Comment[0].Created hour = %d, want 14", result.Comments[0].Created.Hour())
	}
	if !result.Comments[1].Created.IsZero() {
		t.Errorf("Comment[1].Created should be zero for missing timestamp, got %v", result.Comments[1].Created)
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

// --- toPageResult tests ---

func TestToPageResult_HasMoreWhenTotalExceedsFrom(t *testing.T) {
	c := &Client{}
	resp := &api.SearchResult{
		Issues: []*api.Issue{
			{Key: "A-1", Fields: api.IssueFields{Summary: "one"}},
			{Key: "A-2", Fields: api.IssueFields{Summary: "two"}},
		},
		Total: 10,
	}
	result := c.toPageResult(resp, 0)

	if len(result.Issues) != 2 {
		t.Fatalf("Issues len = %d, want 2", len(result.Issues))
	}
	if !result.HasMore {
		t.Error("HasMore should be true when newFrom (2) < Total (10)")
	}
	if result.From != 0 {
		t.Errorf("From = %d, want 0", result.From)
	}
	if result.Total != 10 {
		t.Errorf("Total = %d, want 10", result.Total)
	}
}

func TestToPageResult_NoMoreWhenAllFetched(t *testing.T) {
	c := &Client{}
	resp := &api.SearchResult{
		Issues: []*api.Issue{
			{Key: "A-3", Fields: api.IssueFields{Summary: "three"}},
		},
		Total: 3,
	}
	// Starting from offset 2, fetching 1 issue means newFrom = 3, matching total.
	result := c.toPageResult(resp, 2)

	if result.HasMore {
		t.Error("HasMore should be false when newFrom (3) == Total (3)")
	}
}

func TestToPageResult_EmptyIssuesSlice(t *testing.T) {
	c := &Client{}
	resp := &api.SearchResult{
		Issues: []*api.Issue{},
		Total:  5,
	}
	result := c.toPageResult(resp, 0)

	if result.HasMore {
		t.Error("HasMore should be false for empty issues slice")
	}
	if len(result.Issues) != 0 {
		t.Errorf("Issues len = %d, want 0", len(result.Issues))
	}
}

func TestToPageResult_ZeroTotal(t *testing.T) {
	// When Total is 0 (unknown), HasMore is true if issues are non-empty
	// — the client relies on receiving an empty page to stop.
	c := &Client{}
	resp := &api.SearchResult{
		Issues: []*api.Issue{
			{Key: "A-1", Fields: api.IssueFields{Summary: "one"}},
		},
		Total: 0,
	}
	result := c.toPageResult(resp, 0)

	if !result.HasMore {
		t.Error("HasMore should be true when Total is 0 but issues are non-empty")
	}
}

func TestToPageResult_MaxTotalIssuesCap(t *testing.T) {
	// Even if the server reports more results, HasMore should be false
	// when we've reached MaxTotalIssues.
	c := &Client{}
	resp := &api.SearchResult{
		Issues: []*api.Issue{
			{Key: "A-1", Fields: api.IssueFields{Summary: "one"}},
		},
		Total: 5000,
	}
	// Starting from MaxTotalIssues-1, so newFrom = MaxTotalIssues.
	result := c.toPageResult(resp, MaxTotalIssues-1)

	if result.HasMore {
		t.Errorf("HasMore should be false when newFrom (%d) >= MaxTotalIssues (%d)",
			MaxTotalIssues, MaxTotalIssues)
	}
}

func TestToPageResult_ConvertsIssues(t *testing.T) {
	c := &Client{}
	resp := &api.SearchResult{
		Issues: []*api.Issue{
			{
				Key: "PROJ-42",
				Fields: api.IssueFields{
					Summary:  "Test issue",
					Status:   api.NameField{Name: "Done"},
					Priority: api.NameField{Name: "Critical"},
					Assignee: api.UserField{DisplayName: "alice"},
				},
			},
		},
		Total: 1,
	}
	result := c.toPageResult(resp, 0)

	if result.Issues[0].Key != "PROJ-42" {
		t.Errorf("Key = %q, want %q", result.Issues[0].Key, "PROJ-42")
	}
	if result.Issues[0].Status != "Done" {
		t.Errorf("Status = %q, want %q", result.Issues[0].Status, "Done")
	}
	if result.Issues[0].Priority != "Critical" {
		t.Errorf("Priority = %q, want %q", result.Issues[0].Priority, "Critical")
	}
	if result.Issues[0].Assignee != "alice" {
		t.Errorf("Assignee = %q, want %q", result.Issues[0].Assignee, "alice")
	}
}

// --- IssueURL tests ---

func TestIssueURL(t *testing.T) {
	c := &Client{
		config: &config.Config{Domain: "myteam.atlassian.net"},
	}
	got := c.IssueURL("TEST-123")
	want := "https://myteam.atlassian.net/browse/TEST-123"
	if got != want {
		t.Errorf("IssueURL = %q, want %q", got, want)
	}
}
