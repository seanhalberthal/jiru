package api

import (
	"encoding/json"
	"testing"
)

func TestIssue_JSONRoundTrip(t *testing.T) {
	raw := `{
		"key": "TEST-1",
		"fields": {
			"summary": "Test issue",
			"description": "A description",
			"status": {"name": "In Progress"},
			"priority": {"name": "High"},
			"assignee": {"name": "auser", "displayName": "Alice User", "accountId": "abc123"},
			"reporter": {"name": "buser", "displayName": "Bob User", "accountId": "def456"},
			"issuetype": {"id": "10001", "name": "Story", "subtask": false},
			"parent": {"key": "EPIC-1"},
			"labels": ["backend", "urgent"],
			"created": "2024-01-15T10:30:45.123+0000",
			"updated": "2024-06-20T14:22:33.456+1000",
			"comment": {
				"comments": [
					{"author": {"displayName": "alice"}, "body": "looks good"},
					{"author": {"displayName": "bob"}, "body": "needs work"}
				],
				"total": 2
			}
		}
	}`

	var iss Issue
	if err := json.Unmarshal([]byte(raw), &iss); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if iss.Key != "TEST-1" {
		t.Errorf("Key = %q", iss.Key)
	}
	if iss.Fields.Summary != "Test issue" {
		t.Errorf("Summary = %q", iss.Fields.Summary)
	}
	if iss.Fields.Status.Name != "In Progress" {
		t.Errorf("Status = %q", iss.Fields.Status.Name)
	}
	if iss.Fields.Assignee.DisplayName != "Alice User" {
		t.Errorf("Assignee.DisplayName = %q", iss.Fields.Assignee.DisplayName)
	}
	if iss.Fields.Assignee.Name != "auser" {
		t.Errorf("Assignee.Name = %q", iss.Fields.Assignee.Name)
	}
	if iss.Fields.Assignee.AccountID != "abc123" {
		t.Errorf("Assignee.AccountID = %q", iss.Fields.Assignee.AccountID)
	}
	if iss.Fields.Parent == nil || iss.Fields.Parent.Key != "EPIC-1" {
		t.Errorf("Parent = %v", iss.Fields.Parent)
	}
	if len(iss.Fields.Labels) != 2 {
		t.Errorf("Labels = %v", iss.Fields.Labels)
	}
	if iss.Fields.IssueType.Name != "Story" {
		t.Errorf("IssueType.Name = %q", iss.Fields.IssueType.Name)
	}
	if len(iss.Fields.Comment.Comments) != 2 {
		t.Errorf("Comments = %d", len(iss.Fields.Comment.Comments))
	}
}

func TestIssue_NilParent(t *testing.T) {
	raw := `{"key": "TEST-2", "fields": {"summary": "No parent"}}`
	var iss Issue
	if err := json.Unmarshal([]byte(raw), &iss); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if iss.Fields.Parent != nil {
		t.Errorf("expected nil parent, got %v", iss.Fields.Parent)
	}
}

func TestIssue_ADFDescription(t *testing.T) {
	raw := `{"key": "TEST-3", "fields": {"summary": "ADF", "description": {"type": "doc", "content": []}}}`
	var iss Issue
	if err := json.Unmarshal([]byte(raw), &iss); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// Description should be a map, not a string.
	if _, ok := iss.Fields.Description.(string); ok {
		t.Error("expected non-string description for ADF")
	}
}

func TestSearchResult_JSON(t *testing.T) {
	raw := `{
		"total": 42,
		"maxResults": 100,
		"startAt": 0,
		"isLast": false,
		"nextPageToken": "abc123",
		"issues": [
			{"key": "TEST-1", "fields": {"summary": "First"}},
			{"key": "TEST-2", "fields": {"summary": "Second"}}
		]
	}`

	var sr SearchResult
	if err := json.Unmarshal([]byte(raw), &sr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if sr.Total != 42 {
		t.Errorf("Total = %d", sr.Total)
	}
	if sr.NextPageToken != "abc123" {
		t.Errorf("NextPageToken = %q", sr.NextPageToken)
	}
	if len(sr.Issues) != 2 {
		t.Errorf("Issues = %d", len(sr.Issues))
	}
}

func TestBoardResult_JSON(t *testing.T) {
	raw := `{"maxResults": 50, "total": 3, "isLast": true, "values": [
		{"id": 1, "name": "Board One", "type": "scrum", "location": {"projectKey": "ONE"}},
		{"id": 2, "name": "Board Two", "type": "kanban"}
	]}`

	var br BoardResult
	if err := json.Unmarshal([]byte(raw), &br); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(br.Boards) != 2 {
		t.Errorf("Boards = %d", len(br.Boards))
	}
	if br.Boards[0].Name != "Board One" {
		t.Errorf("Board[0].Name = %q", br.Boards[0].Name)
	}
	if br.Boards[0].Location.ProjectKey != "ONE" {
		t.Errorf("Board[0].Location.ProjectKey = %q, want %q", br.Boards[0].Location.ProjectKey, "ONE")
	}
}

func TestTransitionResponse_JSON(t *testing.T) {
	raw := `{"transitions": [
		{"id": "11", "name": "Start Progress", "to": {"name": "In Progress"}},
		{"id": "21", "name": "Close Issue", "to": {"name": "Done"}}
	]}`

	var tr TransitionResponse
	if err := json.Unmarshal([]byte(raw), &tr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(tr.Transitions) != 2 {
		t.Fatalf("Transitions = %d", len(tr.Transitions))
	}
	if tr.Transitions[0].To.Name != "In Progress" {
		t.Errorf("To.Name = %q", tr.Transitions[0].To.Name)
	}
	if tr.Transitions[1].ID != "21" {
		t.Errorf("ID = %q", tr.Transitions[1].ID)
	}
}
