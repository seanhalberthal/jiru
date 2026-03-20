package client

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// --- SearchJQLPage tests ---

func TestSearchJQLPage_RequestPathAndParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if !strings.HasPrefix(r.URL.Path, "/rest/api/3/search/jql") {
			t.Errorf("expected v3 search path, got %s", r.URL.Path)
		}

		// Verify JQL parameter is present.
		jql := r.URL.Query().Get("jql")
		if jql != "project = TEST" {
			t.Errorf("jql = %q, want %q", jql, "project = TEST")
		}

		// Verify maxResults.
		maxResults := r.URL.Query().Get("maxResults")
		if maxResults != "50" {
			t.Errorf("maxResults = %q, want %q", maxResults, "50")
		}

		// Verify fields parameter.
		fields := r.URL.Query().Get("fields")
		if fields != searchFields {
			t.Errorf("fields = %q, want %q", fields, searchFields)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"issues": [
				{"key": "TEST-1", "fields": {"summary": "First"}},
				{"key": "TEST-2", "fields": {"summary": "Second"}}
			],
			"total": 5,
			"nextPageToken": "token-abc"
		}`))
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	result, err := c.SearchJQLPage("project = TEST", 50, 0, "")
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Issues) != 2 {
		t.Fatalf("Issues len = %d, want 2", len(result.Issues))
	}
	if result.Issues[0].Key != "TEST-1" {
		t.Errorf("Issues[0].Key = %q, want %q", result.Issues[0].Key, "TEST-1")
	}
	if result.NextToken != "token-abc" {
		t.Errorf("NextToken = %q, want %q", result.NextToken, "token-abc")
	}
	if !result.HasMore {
		t.Error("HasMore should be true when nextPageToken is present")
	}
	if result.From != 0 {
		t.Errorf("From = %d, want 0", result.From)
	}
}

func TestSearchJQLPage_WithNextToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("nextPageToken")
		if token != "page-2-token" {
			t.Errorf("nextPageToken = %q, want %q", token, "page-2-token")
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"issues": [{"key": "TEST-3", "fields": {"summary": "Third"}}],
			"total": 3
		}`))
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	result, err := c.SearchJQLPage("project = TEST", 50, 2, "page-2-token")
	if err != nil {
		t.Fatal(err)
	}

	if result.HasMore {
		t.Error("HasMore should be false when no nextPageToken in response")
	}
	if result.NextToken != "" {
		t.Errorf("NextToken = %q, want empty", result.NextToken)
	}
}

func TestSearchJQLPage_EmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"issues": [], "total": 0}`))
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	result, err := c.SearchJQLPage("project = EMPTY", 50, 0, "")
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Issues) != 0 {
		t.Errorf("Issues len = %d, want 0", len(result.Issues))
	}
	if result.HasMore {
		t.Error("HasMore should be false for empty results")
	}
}

func TestSearchJQLPage_MaxTotalCap(t *testing.T) {
	// When from + len(issues) >= MaxTotalIssues, HasMore should be false
	// even if nextPageToken is present.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"issues": [{"key": "TEST-1", "fields": {"summary": "Final"}}],
			"total": 9999,
			"nextPageToken": "more-data"
		}`))
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	result, err := c.SearchJQLPage("project = TEST", 50, MaxTotalIssues-1, "prev-token")
	if err != nil {
		t.Fatal(err)
	}

	if result.HasMore {
		t.Error("HasMore should be false at MaxTotalIssues cap")
	}
}

func TestSearchJQLPage_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"errorMessages":["invalid JQL"]}`))
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	_, err := c.SearchJQLPage("bad jql !!!", 50, 0, "")
	if err == nil {
		t.Fatal("expected error for 400 response")
	}
}

// --- SearchJQL (all pages) tests ---

func TestSearchJQL_SinglePage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"issues": [
				{"key": "TEST-1", "fields": {"summary": "One"}},
				{"key": "TEST-2", "fields": {"summary": "Two"}}
			],
			"total": 2
		}`))
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	issues, err := c.SearchJQL("project = TEST", 100)
	if err != nil {
		t.Fatal(err)
	}

	if len(issues) != 2 {
		t.Fatalf("len = %d, want 2", len(issues))
	}
}

func TestSearchJQL_MultiplePages(t *testing.T) {
	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := callCount.Add(1)
		w.Header().Set("Content-Type", "application/json")

		switch page {
		case 1:
			_, _ = w.Write([]byte(`{
				"issues": [{"key": "TEST-1", "fields": {"summary": "One"}}],
				"total": 2,
				"nextPageToken": "page2"
			}`))
		case 2:
			// Verify the token was passed.
			token := r.URL.Query().Get("nextPageToken")
			if token != "page2" {
				t.Errorf("page 2 token = %q, want %q", token, "page2")
			}
			_, _ = w.Write([]byte(`{
				"issues": [{"key": "TEST-2", "fields": {"summary": "Two"}}],
				"total": 2
			}`))
		default:
			t.Error("unexpected additional request")
			_, _ = w.Write([]byte(`{"issues": [], "total": 2}`))
		}
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	issues, err := c.SearchJQL("project = TEST", 100)
	if err != nil {
		t.Fatal(err)
	}

	if len(issues) != 2 {
		t.Fatalf("len = %d, want 2", len(issues))
	}
	if issues[0].Key != "TEST-1" || issues[1].Key != "TEST-2" {
		t.Errorf("issues = [%s, %s], want [TEST-1, TEST-2]", issues[0].Key, issues[1].Key)
	}
}

func TestSearchJQL_RespectsLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"issues": [
				{"key": "TEST-1", "fields": {"summary": "One"}},
				{"key": "TEST-2", "fields": {"summary": "Two"}},
				{"key": "TEST-3", "fields": {"summary": "Three"}}
			],
			"total": 100
		}`))
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	// Limit to 2 results.
	issues, err := c.SearchJQL("project = TEST", 2)
	if err != nil {
		t.Fatal(err)
	}

	if len(issues) > 2 {
		t.Errorf("len = %d, want <= 2 (respecting limit)", len(issues))
	}
}

func TestSearchJQL_CursorLoopDetection(t *testing.T) {
	// Simulates the Jira Cloud bug where nextPageToken repeats.
	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		// Always return the same token — simulating the loop bug.
		_, _ = w.Write([]byte(`{
			"issues": [{"key": "TEST-1", "fields": {"summary": "Same"}}],
			"total": 100,
			"nextPageToken": "stuck-token"
		}`))
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	issues, err := c.SearchJQL("project = TEST", 0)
	if err != nil {
		t.Fatal(err)
	}

	// Should have stopped after detecting the loop (2 calls: initial + one repeat).
	count := callCount.Load()
	if count > 3 {
		t.Errorf("made %d requests, expected <= 3 (loop detection should stop pagination)", count)
	}
	// Should have collected at least the first page.
	if len(issues) == 0 {
		t.Error("expected at least one issue")
	}
}

// --- SprintIssuesPage tests ---

func TestSprintIssuesPage_RequestPathAndResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/rest/agile/1.0/sprint/42/issue") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		// Check startAt and maxResults.
		if r.URL.Query().Get("startAt") != "0" {
			t.Errorf("startAt = %q, want %q", r.URL.Query().Get("startAt"), "0")
		}
		if r.URL.Query().Get("maxResults") != "50" {
			t.Errorf("maxResults = %q, want %q", r.URL.Query().Get("maxResults"), "50")
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"issues": [
				{"key": "SP-1", "fields": {"summary": "Sprint issue", "status": {"name": "To Do"}}}
			],
			"total": 1
		}`))
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	result, err := c.SprintIssuesPage(42, 0, 50)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Issues) != 1 {
		t.Fatalf("Issues len = %d, want 1", len(result.Issues))
	}
	if result.Issues[0].Key != "SP-1" {
		t.Errorf("Key = %q, want %q", result.Issues[0].Key, "SP-1")
	}
}

func TestSprintIssuesPage_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errorMessages":["Sprint not found"]}`))
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	_, err := c.SprintIssuesPage(999, 0, 50)
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}

// --- BoardIssuesPage tests ---

func TestBoardIssuesPage_RequestPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/rest/agile/1.0/board/7/issue") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"issues": [{"key": "BD-1", "fields": {"summary": "Board issue"}}],
			"total": 1
		}`))
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	result, err := c.BoardIssuesPage(7, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Issues) != 1 || result.Issues[0].Key != "BD-1" {
		t.Errorf("unexpected issues: %v", result.Issues)
	}
}

// --- EpicIssuesPage tests ---

func TestEpicIssuesPage_RequestPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/rest/agile/1.0/epic/EPIC-1/issue") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("startAt") != "10" {
			t.Errorf("startAt = %q, want %q", r.URL.Query().Get("startAt"), "10")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"issues": [{"key": "EP-5", "fields": {"summary": "Epic child"}}],
			"total": 11
		}`))
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	result, err := c.EpicIssuesPage("EPIC-1", 10, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Issues) != 1 || result.Issues[0].Key != "EP-5" {
		t.Errorf("unexpected issues: %v", result.Issues)
	}
}

// --- Boards tests ---

func TestBoards_ParsesResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/rest/agile/1.0/board") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"maxResults": 100,
			"total": 2,
			"isLast": true,
			"values": [
				{"id": 1, "name": "My Board", "type": "scrum"},
				{"id": 2, "name": "Kanban Board", "type": "kanban"}
			]
		}`))
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	boards, err := c.Boards("TEST")
	if err != nil {
		t.Fatal(err)
	}

	if len(boards) != 2 {
		t.Fatalf("len = %d, want 2", len(boards))
	}
	if boards[0].ID != 1 || boards[0].Name != "My Board" || boards[0].Type != "scrum" {
		t.Errorf("boards[0] = %+v, want {1 My Board scrum}", boards[0])
	}
	if boards[1].ID != 2 || boards[1].Name != "Kanban Board" {
		t.Errorf("boards[1] = %+v", boards[1])
	}
}

func TestBoards_WithProjectFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		project := r.URL.Query().Get("projectKeyOrId")
		if project != "MYPROJ" {
			t.Errorf("projectKeyOrId = %q, want %q", project, "MYPROJ")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"maxResults":100,"total":0,"isLast":true,"values":[]}`))
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	boards, err := c.Boards("MYPROJ")
	if err != nil {
		t.Fatal(err)
	}
	if len(boards) != 0 {
		t.Errorf("len = %d, want 0", len(boards))
	}
}

func TestBoards_NoProjectFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		project := r.URL.Query().Get("projectKeyOrId")
		if project != "" {
			t.Errorf("projectKeyOrId should be absent, got %q", project)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"maxResults":100,"total":0,"isLast":true,"values":[]}`))
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	_, err := c.Boards("")
	if err != nil {
		t.Fatal(err)
	}
}

// --- BoardSprints tests ---

func TestBoardSprints_ParsesResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/rest/agile/1.0/board/5/sprint") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		state := r.URL.Query().Get("state")
		if state != "active" {
			t.Errorf("state = %q, want %q", state, "active")
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"maxResults": 50,
			"isLast": true,
			"values": [
				{"id": 10, "name": "Sprint 42", "state": "active"},
				{"id": 11, "name": "Sprint 43", "state": "future"}
			]
		}`))
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	sprints, err := c.BoardSprints(5, "active")
	if err != nil {
		t.Fatal(err)
	}

	if len(sprints) != 2 {
		t.Fatalf("len = %d, want 2", len(sprints))
	}
	if sprints[0].ID != 10 || sprints[0].Name != "Sprint 42" || sprints[0].State != "active" {
		t.Errorf("sprints[0] = %+v", sprints[0])
	}
}

func TestBoardSprints_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errorMessages":["Board not found"]}`))
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	_, err := c.BoardSprints(999, "active")
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}

// --- BoardFilterJQL tests ---

func TestBoardFilterJQL_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case strings.Contains(r.URL.Path, "/board/5/configuration"):
			_, _ = w.Write([]byte(`{"filter":{"id":"12345"}}`))
		case strings.Contains(r.URL.Path, "/filter/12345"):
			_, _ = w.Write([]byte(`{"jql":"project = TEST AND type = Bug"}`))
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	jql, err := c.BoardFilterJQL(5)
	if err != nil {
		t.Fatal(err)
	}
	if jql != "project = TEST AND type = Bug" {
		t.Errorf("JQL = %q, want %q", jql, "project = TEST AND type = Bug")
	}
}

func TestBoardFilterJQL_NoFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"filter":{"id":""}}`))
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	_, err := c.BoardFilterJQL(5)
	if err == nil {
		t.Fatal("expected error when board has no filter")
	}
	if !strings.Contains(err.Error(), "no filter") {
		t.Errorf("error = %q, expected 'no filter' message", err.Error())
	}
}

// --- Malformed JSON response tests ---

func TestSearchJQLPage_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{not valid json`))
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	_, err := c.SearchJQLPage("project = TEST", 50, 0, "")
	if err == nil {
		t.Fatal("expected error for malformed JSON response")
	}
}

func TestBoards_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{broken`))
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	_, err := c.Boards("")
	if err == nil {
		t.Fatal("expected error for malformed JSON response")
	}
}

// --- SprintIssues (all pages) tests ---

func TestSprintIssues_FetchesAllPages(t *testing.T) {
	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := callCount.Add(1)
		w.Header().Set("Content-Type", "application/json")

		switch page {
		case 1:
			_, _ = w.Write([]byte(`{
				"issues": [{"key": "SP-1", "fields": {"summary": "One"}}],
				"total": 2
			}`))
		case 2:
			_, _ = w.Write([]byte(`{
				"issues": [{"key": "SP-2", "fields": {"summary": "Two"}}],
				"total": 2
			}`))
		default:
			_, _ = w.Write([]byte(`{"issues": [], "total": 2}`))
		}
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	issues, err := c.SprintIssues(42)
	if err != nil {
		t.Fatal(err)
	}

	if len(issues) != 2 {
		t.Fatalf("len = %d, want 2", len(issues))
	}
}

// --- EpicIssues (all pages) tests ---

func TestEpicIssues_FetchesAllPages(t *testing.T) {
	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := callCount.Add(1)
		w.Header().Set("Content-Type", "application/json")

		switch page {
		case 1:
			_, _ = w.Write([]byte(`{
				"issues": [{"key": "EP-1", "fields": {"summary": "One"}}],
				"total": 2
			}`))
		case 2:
			_, _ = w.Write([]byte(`{
				"issues": [{"key": "EP-2", "fields": {"summary": "Two"}}],
				"total": 2
			}`))
		default:
			_, _ = w.Write([]byte(`{"issues": [], "total": 2}`))
		}
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	issues, err := c.EpicIssues("EPIC-1")
	if err != nil {
		t.Fatal(err)
	}

	if len(issues) != 2 {
		t.Fatalf("len = %d, want 2", len(issues))
	}
}
