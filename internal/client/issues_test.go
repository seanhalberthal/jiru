package client

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/seanhalberthal/jiru/internal/config"
)

// --- buildCreatePayload tests ---

func TestBuildCreatePayload_BasicFields(t *testing.T) {
	c := &Client{config: &config.Config{AuthType: "basic"}}
	req := &CreateIssueRequest{
		Project:   "TEST",
		IssueType: "Story",
		Summary:   "New feature",
	}

	payload := c.buildCreatePayload(req)
	fields, ok := payload["fields"].(map[string]any)
	if !ok {
		t.Fatal("payload missing 'fields' key")
	}

	// Project should be a map with "key".
	proj, ok := fields["project"].(map[string]string)
	if !ok || proj["key"] != "TEST" {
		t.Errorf("project = %v, want {key: TEST}", fields["project"])
	}

	// Issue type should be a map with "name".
	issType, ok := fields["issuetype"].(map[string]string)
	if !ok || issType["name"] != "Story" {
		t.Errorf("issuetype = %v, want {name: Story}", fields["issuetype"])
	}

	if fields["summary"] != "New feature" {
		t.Errorf("summary = %v, want %q", fields["summary"], "New feature")
	}

	// Optional fields should be absent.
	for _, key := range []string{"description", "priority", "assignee", "labels", "components", "parent"} {
		if _, present := fields[key]; present {
			t.Errorf("optional field %q should be absent when not set", key)
		}
	}
}

func TestBuildCreatePayload_AllOptionalFields(t *testing.T) {
	c := &Client{config: &config.Config{AuthType: "basic"}}
	req := &CreateIssueRequest{
		Project:     "PROJ",
		IssueType:   "Bug",
		Summary:     "Something broke",
		Description: "Detailed description",
		Priority:    "High",
		Assignee:    "abc123",
		Labels:      []string{"backend", "urgent"},
		Components:  []string{"API", "Auth"},
		ParentKey:   "EPIC-1",
	}

	payload := c.buildCreatePayload(req)
	fields := payload["fields"].(map[string]any)

	if fields["description"] != "Detailed description" {
		t.Errorf("description = %v, want %q", fields["description"], "Detailed description")
	}

	prio, ok := fields["priority"].(map[string]string)
	if !ok || prio["name"] != "High" {
		t.Errorf("priority = %v, want {name: High}", fields["priority"])
	}

	// Basic auth uses accountId for assignee.
	assignee, ok := fields["assignee"].(map[string]string)
	if !ok || assignee["accountId"] != "abc123" {
		t.Errorf("assignee = %v, want {accountId: abc123}", fields["assignee"])
	}

	labels, ok := fields["labels"].([]string)
	if !ok || len(labels) != 2 || labels[0] != "backend" {
		t.Errorf("labels = %v, want [backend urgent]", fields["labels"])
	}

	components, ok := fields["components"].([]map[string]string)
	if !ok || len(components) != 2 {
		t.Fatalf("components = %v, want 2 component maps", fields["components"])
	}
	if components[0]["name"] != "API" || components[1]["name"] != "Auth" {
		t.Errorf("components = %v, want [{name:API} {name:Auth}]", components)
	}

	parent, ok := fields["parent"].(map[string]string)
	if !ok || parent["key"] != "EPIC-1" {
		t.Errorf("parent = %v, want {key: EPIC-1}", fields["parent"])
	}
}

func TestBuildCreatePayload_BearerAuthAssigneeFormat(t *testing.T) {
	// Bearer auth should use "name" instead of "accountId" for assignee.
	c := &Client{config: &config.Config{AuthType: "bearer"}}
	req := &CreateIssueRequest{
		Project:   "TEST",
		IssueType: "Task",
		Summary:   "Bearer test",
		Assignee:  "jsmith",
	}

	payload := c.buildCreatePayload(req)
	fields := payload["fields"].(map[string]any)

	assignee, ok := fields["assignee"].(map[string]string)
	if !ok {
		t.Fatal("assignee should be a map[string]string")
	}
	if _, hasAccountID := assignee["accountId"]; hasAccountID {
		t.Error("bearer auth should not use accountId")
	}
	if assignee["name"] != "jsmith" {
		t.Errorf("assignee name = %q, want %q", assignee["name"], "jsmith")
	}
}

func TestBuildCreatePayload_CustomFields(t *testing.T) {
	c := &Client{config: &config.Config{AuthType: "basic"}}
	req := &CreateIssueRequest{
		Project:   "TEST",
		IssueType: "Story",
		Summary:   "Custom fields test",
		CustomFields: map[string]any{
			"customfield_10001": "plain string",
			"customfield_10002": float64(42),
			"customfield_10003": map[string]string{"value": "Option A"},
		},
	}

	payload := c.buildCreatePayload(req)
	fields := payload["fields"].(map[string]any)

	// String custom field.
	if fields["customfield_10001"] != "plain string" {
		t.Errorf("customfield_10001 = %v, want %q", fields["customfield_10001"], "plain string")
	}

	// Numeric custom field.
	if fields["customfield_10002"] != float64(42) {
		t.Errorf("customfield_10002 = %v, want 42", fields["customfield_10002"])
	}

	// Option custom field — should be wrapped as {value: "..."}.
	optField, ok := fields["customfield_10003"].(map[string]string)
	if !ok || optField["value"] != "Option A" {
		t.Errorf("customfield_10003 = %v, want {value: Option A}", fields["customfield_10003"])
	}
}

func TestBuildCreatePayload_IgnoresUnknownCustomFieldTypes(t *testing.T) {
	c := &Client{config: &config.Config{AuthType: "basic"}}
	req := &CreateIssueRequest{
		Project:   "TEST",
		IssueType: "Story",
		Summary:   "Unknown custom",
		CustomFields: map[string]any{
			"customfield_10099": []string{"not", "handled"},
		},
	}

	payload := c.buildCreatePayload(req)
	fields := payload["fields"].(map[string]any)

	// Unsupported types (like slices) should not appear in the payload.
	if _, present := fields["customfield_10099"]; present {
		t.Error("unsupported custom field types should be silently ignored")
	}
}

// --- HTTP integration tests for issue operations ---

func TestGetIssue_ParsesResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/rest/api/2/issue/TEST-42") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"key": "TEST-42",
			"fields": {
				"summary": "Fix the thing",
				"description": "It is broken",
				"status": {"name": "In Progress"},
				"priority": {"name": "High"},
				"assignee": {"displayName": "alice"},
				"reporter": {"displayName": "bob"},
				"issuetype": {"name": "Bug"},
				"labels": ["backend"],
				"created": "2024-01-15T10:30:45.123+0000",
				"parent": {"key": "EPIC-5"},
				"comment": {
					"comments": [
						{"author": {"displayName": "charlie"}, "body": "On it"}
					],
					"total": 1
				}
			}
		}`))
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	issue, err := c.GetIssue("TEST-42")
	if err != nil {
		t.Fatal(err)
	}

	if issue.Key != "TEST-42" {
		t.Errorf("Key = %q, want %q", issue.Key, "TEST-42")
	}
	if issue.Summary != "Fix the thing" {
		t.Errorf("Summary = %q, want %q", issue.Summary, "Fix the thing")
	}
	if issue.Description != "It is broken" {
		t.Errorf("Description = %q, want %q", issue.Description, "It is broken")
	}
	if issue.Status != "In Progress" {
		t.Errorf("Status = %q, want %q", issue.Status, "In Progress")
	}
	if issue.ParentKey != "EPIC-5" {
		t.Errorf("ParentKey = %q, want %q", issue.ParentKey, "EPIC-5")
	}
	if len(issue.Comments) != 1 || issue.Comments[0].Author != "charlie" {
		t.Errorf("Comments = %v, want 1 comment by charlie", issue.Comments)
	}
}

func TestGetIssue_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errorMessages":["Issue does not exist"]}`))
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	_, err := c.GetIssue("NOPE-999")
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error should mention status code, got: %v", err)
	}
}

func TestCreateIssue_SendsCorrectRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/rest/api/2/issue") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		fields, ok := payload["fields"].(map[string]any)
		if !ok {
			t.Fatal("request body missing 'fields'")
		}
		proj := fields["project"].(map[string]any)
		if proj["key"] != "TEST" {
			t.Errorf("project key = %v, want TEST", proj["key"])
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"10001","key":"TEST-99"}`))
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	resp, err := c.CreateIssue(&CreateIssueRequest{
		Project:   "TEST",
		IssueType: "Task",
		Summary:   "New task",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Key != "TEST-99" {
		t.Errorf("Key = %q, want %q", resp.Key, "TEST-99")
	}
}

func TestCreateIssue_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"errors":{"summary":"required"}}`))
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	_, err := c.CreateIssue(&CreateIssueRequest{
		Project:   "TEST",
		IssueType: "Bug",
		Summary:   "",
	})
	if err == nil {
		t.Fatal("expected error for 400 response")
	}
}

func TestEditIssue_SendsPutRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/rest/api/2/issue/TEST-1") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to decode body: %v", err)
		}

		fields := payload["fields"].(map[string]any)
		if fields["summary"] != "Updated summary" {
			t.Errorf("summary = %v, want %q", fields["summary"], "Updated summary")
		}
		if fields["description"] != "Updated desc" {
			t.Errorf("description = %v, want %q", fields["description"], "Updated desc")
		}

		// Priority should be wrapped in a name map.
		prio := fields["priority"].(map[string]any)
		if prio["name"] != "Low" {
			t.Errorf("priority name = %v, want %q", prio["name"], "Low")
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	err := c.EditIssue("TEST-1", &EditIssueRequest{
		Summary:     "Updated summary",
		Description: "Updated desc",
		Priority:    "Low",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestEditIssue_OmitsEmptyFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to decode body: %v", err)
		}
		fields := payload["fields"].(map[string]any)

		// Only summary should be present.
		if _, has := fields["description"]; has {
			t.Error("empty description should not be sent")
		}
		if _, has := fields["priority"]; has {
			t.Error("empty priority should not be sent")
		}
		if _, has := fields["labels"]; has {
			t.Error("nil labels should not be sent")
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	err := c.EditIssue("TEST-1", &EditIssueRequest{
		Summary: "Only summary changed",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestEditIssue_SendsLabels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to decode body: %v", err)
		}
		fields := payload["fields"].(map[string]any)

		labels, ok := fields["labels"].([]any)
		if !ok {
			t.Fatalf("labels = %T, want []any", fields["labels"])
		}
		if len(labels) != 2 {
			t.Errorf("labels len = %d, want 2", len(labels))
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	err := c.EditIssue("TEST-1", &EditIssueRequest{
		Labels: []string{"new-label", "another"},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestDeleteIssue_SendsDeleteRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/rest/api/2/issue/TEST-1") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		// Check cascade parameter.
		cascade := r.URL.Query().Get("deleteSubtasks")
		if cascade != "true" {
			t.Errorf("deleteSubtasks = %q, want %q", cascade, "true")
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	err := c.DeleteIssue("TEST-1", true)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDeleteIssue_CascadeFalse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cascade := r.URL.Query().Get("deleteSubtasks")
		if cascade != "false" {
			t.Errorf("deleteSubtasks = %q, want %q", cascade, "false")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	err := c.DeleteIssue("TEST-1", false)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDeleteIssue_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"errorMessages":["permission denied"]}`))
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	err := c.DeleteIssue("TEST-1", false)
	if err == nil {
		t.Fatal("expected error for 403 response")
	}
}

func TestAssignIssue_SendsPutRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/issue/TEST-1/assignee") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		body, _ := io.ReadAll(r.Body)
		var payload map[string]string
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to decode body: %v", err)
		}
		if payload["accountId"] != "user-123" {
			t.Errorf("accountId = %q, want %q", payload["accountId"], "user-123")
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	err := c.AssignIssue("TEST-1", "user-123")
	if err != nil {
		t.Fatal(err)
	}
}

func TestAssignIssue_DefaultUsesCurrentAccountID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to decode body: %v", err)
		}
		if payload["accountId"] != "me-acc-123" {
			t.Errorf("accountId = %v, want %q", payload["accountId"], "me-acc-123")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	c.accountID = "me-acc-123"
	if err := c.AssignIssue("TEST-1", "default"); err != nil {
		t.Fatal(err)
	}
}

func TestAssignIssue_NoneSendsNullAccountID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to decode body: %v", err)
		}
		if _, ok := payload["accountId"]; !ok {
			t.Error("accountId key missing from payload")
		}
		if payload["accountId"] != nil {
			t.Errorf("accountId = %v, want nil", payload["accountId"])
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	if err := c.AssignIssue("TEST-1", "none"); err != nil {
		t.Fatal(err)
	}
}

func TestAssignIssue_BearerUsesNameField(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to decode body: %v", err)
		}
		if _, ok := payload["accountId"]; ok {
			t.Error("bearer auth should not send accountId")
		}
		if payload["name"] != "jsmith" {
			t.Errorf("name = %v, want %q", payload["name"], "jsmith")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := newTestClient(srv, "bearer")
	c.userName = "jsmith"
	if err := c.AssignIssue("TEST-1", "default"); err != nil {
		t.Fatal(err)
	}
}

func TestTransitionIssue_SendsPostRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/issue/TEST-1/transitions") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to decode body: %v", err)
		}

		transition := payload["transition"].(map[string]any)
		if transition["id"] != "31" {
			t.Errorf("transition id = %v, want %q", transition["id"], "31")
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	err := c.TransitionIssue("TEST-1", "31")
	if err != nil {
		t.Fatal(err)
	}
}

func TestAddComment_SendsPostRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/issue/TEST-1/comment") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		body, _ := io.ReadAll(r.Body)
		var payload map[string]string
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to decode body: %v", err)
		}
		if payload["body"] != "Looks good to me" {
			t.Errorf("body = %q, want %q", payload["body"], "Looks good to me")
		}

		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"10001"}`))
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	err := c.AddComment("TEST-1", "Looks good to me")
	if err != nil {
		t.Fatal(err)
	}
}

func TestAddComment_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"errorMessages":["no permission"]}`))
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	err := c.AddComment("TEST-1", "comment")
	if err == nil {
		t.Fatal("expected error for 403 response")
	}
}

func TestTransitions_ParsesResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/issue/TEST-1/transitions") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"transitions": [
				{"id": "11", "name": "Start Progress", "to": {"name": "In Progress"}},
				{"id": "21", "name": "Done", "to": {"name": "Done"}},
				{"id": "31", "name": "Reopen", "to": {"name": "Open"}}
			]
		}`))
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	transitions, err := c.Transitions("TEST-1")
	if err != nil {
		t.Fatal(err)
	}

	if len(transitions) != 3 {
		t.Fatalf("len = %d, want 3", len(transitions))
	}
	if transitions[0].ID != "11" {
		t.Errorf("transitions[0].ID = %q, want %q", transitions[0].ID, "11")
	}
	if transitions[0].Name != "Start Progress" {
		t.Errorf("transitions[0].Name = %q, want %q", transitions[0].Name, "Start Progress")
	}
	if transitions[0].ToStatus != "In Progress" {
		t.Errorf("transitions[0].ToStatus = %q, want %q", transitions[0].ToStatus, "In Progress")
	}
}

func TestTransitions_InvalidKey(t *testing.T) {
	// Transitions validates the issue key before making the API call.
	c := &Client{config: &config.Config{}}
	_, err := c.Transitions("bad-key")
	if err == nil {
		t.Fatal("expected error for invalid issue key")
	}
}

func TestChildIssues_ConstructsJQL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// ChildIssues uses SearchJQL which calls SearchJQLPage on v3 path.
		jql := r.URL.Query().Get("jql")
		if !strings.Contains(jql, "parent = 'EPIC-1'") {
			t.Errorf("JQL should contain parent clause, got: %s", jql)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"issues": [
				{"key": "TEST-10", "fields": {"summary": "Child one", "status": {"name": "Open"}, "issuetype": {"name": "Story"}}},
				{"key": "TEST-11", "fields": {"summary": "Child two", "status": {"name": "Done"}, "issuetype": {"name": "Bug"}}}
			],
			"total": 2
		}`))
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	children, err := c.ChildIssues("EPIC-1")
	if err != nil {
		t.Fatal(err)
	}

	if len(children) != 2 {
		t.Fatalf("len = %d, want 2", len(children))
	}
	if children[0].Key != "TEST-10" {
		t.Errorf("children[0].Key = %q, want %q", children[0].Key, "TEST-10")
	}
	if children[0].Status != "Open" {
		t.Errorf("children[0].Status = %q, want %q", children[0].Status, "Open")
	}
	if children[1].IssueType != "Bug" {
		t.Errorf("children[1].IssueType = %q, want %q", children[1].IssueType, "Bug")
	}
}

func TestChildIssues_InvalidKey(t *testing.T) {
	c := &Client{config: &config.Config{}}
	_, err := c.ChildIssues("not-valid")
	if err == nil {
		t.Fatal("expected error for invalid issue key")
	}
}

func TestLinkIssue_SendsCorrectBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/rest/api/2/issueLink") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("failed to decode body: %v", err)
		}

		linkType := payload["type"].(map[string]any)
		if linkType["name"] != "Blocks" {
			t.Errorf("type name = %v, want %q", linkType["name"], "Blocks")
		}
		inward := payload["inwardIssue"].(map[string]any)
		if inward["key"] != "TEST-1" {
			t.Errorf("inward key = %v, want %q", inward["key"], "TEST-1")
		}
		outward := payload["outwardIssue"].(map[string]any)
		if outward["key"] != "TEST-2" {
			t.Errorf("outward key = %v, want %q", outward["key"], "TEST-2")
		}

		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	err := c.LinkIssue("TEST-1", "TEST-2", "Blocks")
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetIssueLinkTypes_ParsesResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/rest/api/2/issueLinkType") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"issueLinkTypes": [
				{"id": "1", "name": "Blocks", "inward": "is blocked by", "outward": "blocks"},
				{"id": "2", "name": "Relates", "inward": "relates to", "outward": "relates to"}
			]
		}`))
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	types, err := c.GetIssueLinkTypes()
	if err != nil {
		t.Fatal(err)
	}

	if len(types) != 2 {
		t.Fatalf("len = %d, want 2", len(types))
	}
	if types[0].Name != "Blocks" {
		t.Errorf("types[0].Name = %q, want %q", types[0].Name, "Blocks")
	}
	if types[0].Inward != "is blocked by" {
		t.Errorf("types[0].Inward = %q, want %q", types[0].Inward, "is blocked by")
	}
	if types[1].Outward != "relates to" {
		t.Errorf("types[1].Outward = %q, want %q", types[1].Outward, "relates to")
	}
}
