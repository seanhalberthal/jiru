package client

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// --- Me tests ---

func TestMe_DisplayName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/rest/api/2/myself") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"displayName":"Alice Smith","name":"asmith"}`))
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	name, err := c.Me()
	if err != nil {
		t.Fatal(err)
	}
	// DisplayName takes precedence over Name.
	if name != "Alice Smith" {
		t.Errorf("name = %q, want %q", name, "Alice Smith")
	}
}

func TestMe_FallsBackToName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"displayName":"","name":"asmith"}`))
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	name, err := c.Me()
	if err != nil {
		t.Fatal(err)
	}
	if name != "asmith" {
		t.Errorf("name = %q, want %q", name, "asmith")
	}
}

func TestMe_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"invalid credentials"}`))
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	_, err := c.Me()
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
	if !strings.Contains(err.Error(), "auth check failed") {
		t.Errorf("error = %q, expected 'auth check failed'", err.Error())
	}
}

// --- Projects tests ---

func TestProjects_ParsesResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/rest/api/2/project") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"key":"TEST","name":"Test Project","projectTypeKey":"software"},
			{"key":"OPS","name":"Operations","projectTypeKey":"business"}
		]`))
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	projects, err := c.Projects()
	if err != nil {
		t.Fatal(err)
	}

	if len(projects) != 2 {
		t.Fatalf("len = %d, want 2", len(projects))
	}
	if projects[0].Key != "TEST" || projects[0].Name != "Test Project" || projects[0].Type != "software" {
		t.Errorf("projects[0] = %+v", projects[0])
	}
	if projects[1].Key != "OPS" {
		t.Errorf("projects[1].Key = %q, want %q", projects[1].Key, "OPS")
	}
}

func TestProjects_EmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	projects, err := c.Projects()
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 0 {
		t.Errorf("len = %d, want 0", len(projects))
	}
}

func TestProjects_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`internal error`))
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	_, err := c.Projects()
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

// --- SearchUsers tests ---

func TestSearchUsers_ParsesResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/rest/api/3/user/assignable/search") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		// Check query parameters.
		project := r.URL.Query().Get("project")
		if project != "TEST" {
			t.Errorf("project = %q, want %q", project, "TEST")
		}
		query := r.URL.Query().Get("query")
		if query != "ali" {
			t.Errorf("query = %q, want %q", query, "ali")
		}
		maxResults := r.URL.Query().Get("maxResults")
		if maxResults != "10" {
			t.Errorf("maxResults = %q, want %q", maxResults, "10")
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"accountId":"abc-123","displayName":"Alice Smith","active":true},
			{"accountId":"def-456","displayName":"Ali Khan","active":true},
			{"accountId":"ghi-789","displayName":"","active":true}
		]`))
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	users, err := c.SearchUsers("TEST", "ali")
	if err != nil {
		t.Fatal(err)
	}

	// Users with empty displayName should be filtered out.
	if len(users) != 2 {
		t.Fatalf("len = %d, want 2 (empty displayName filtered)", len(users))
	}
	if users[0].AccountID != "abc-123" || users[0].DisplayName != "Alice Smith" {
		t.Errorf("users[0] = %+v", users[0])
	}
	if users[1].AccountID != "def-456" || users[1].DisplayName != "Ali Khan" {
		t.Errorf("users[1] = %+v", users[1])
	}
}

func TestSearchUsers_EmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	users, err := c.SearchUsers("TEST", "nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 0 {
		t.Errorf("len = %d, want 0", len(users))
	}
}

func TestSearchUsers_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"not authorised"}`))
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	_, err := c.SearchUsers("TEST", "ali")
	if err == nil {
		t.Fatal("expected error for 403 response")
	}
}

// --- Transitions tests (additional to those in issues_test.go) ---

func TestTransitions_EmptyList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"transitions":[]}`))
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	transitions, err := c.Transitions("TEST-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(transitions) != 0 {
		t.Errorf("len = %d, want 0", len(transitions))
	}
}

func TestTransitions_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errorMessages":["Issue not found"]}`))
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	_, err := c.Transitions("TEST-999")
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}

// --- CreateMetaFields tests ---

func TestCreateMetaFields_ParsesResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/rest/api/3/issue/createmeta/TEST/issuetypes/10001") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"values": [
				{
					"fieldId": "customfield_10001",
					"name": "Story Points",
					"required": false,
					"schema": {"type": "number"},
					"allowedValues": []
				},
				{
					"fieldId": "customfield_10002",
					"name": "Team",
					"required": true,
					"schema": {"type": "option"},
					"allowedValues": [
						{"value": "Alpha"},
						{"value": "Beta"}
					]
				},
				{
					"fieldId": "customfield_10003",
					"name": "Notes",
					"required": false,
					"schema": {"type": "string"},
					"allowedValues": []
				},
				{
					"fieldId": "summary",
					"name": "Summary",
					"required": true,
					"schema": {"type": "string"}
				},
				{
					"fieldId": "description",
					"name": "Description",
					"required": false,
					"schema": {"type": "string"}
				}
			]
		}`))
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	fields, err := c.CreateMetaFields("TEST", "10001")
	if err != nil {
		t.Fatal(err)
	}

	// Standard fields (summary, description) and non-customfield_ fields are skipped.
	if len(fields) != 3 {
		t.Fatalf("len = %d, want 3 (only customfield_ entries)", len(fields))
	}

	// Check number field.
	if fields[0].ID != "customfield_10001" || fields[0].FieldType != "number" {
		t.Errorf("fields[0] = %+v, want customfield_10001/number", fields[0])
	}

	// Check option field with allowed values.
	if fields[1].ID != "customfield_10002" || fields[1].FieldType != "option" {
		t.Errorf("fields[1] = %+v, want customfield_10002/option", fields[1])
	}
	if !fields[1].Required {
		t.Error("fields[1].Required should be true")
	}
	if len(fields[1].AllowedValues) != 2 || fields[1].AllowedValues[0] != "Alpha" {
		t.Errorf("fields[1].AllowedValues = %v, want [Alpha Beta]", fields[1].AllowedValues)
	}

	// Check string field.
	if fields[2].FieldType != "string" {
		t.Errorf("fields[2].FieldType = %q, want %q", fields[2].FieldType, "string")
	}
}

func TestCreateMetaFields_UnsupportedType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"values": [
				{
					"fieldId": "customfield_10099",
					"name": "Weird Field",
					"required": false,
					"schema": {"type": "array", "items": "string"},
					"allowedValues": []
				}
			]
		}`))
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	fields, err := c.CreateMetaFields("TEST", "10001")
	if err != nil {
		t.Fatal(err)
	}

	// Unsupported type should still be returned with "unsupported" fieldType.
	if len(fields) != 1 {
		t.Fatalf("len = %d, want 1", len(fields))
	}
	if fields[0].FieldType != "unsupported" {
		t.Errorf("FieldType = %q, want %q", fields[0].FieldType, "unsupported")
	}
}

func TestCreateMetaFields_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errorMessages":["not found"]}`))
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	_, err := c.CreateMetaFields("NOPE", "99999")
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}

// --- JQLMetadata tests ---

func TestJQLMetadata_AggregatesResults(t *testing.T) {
	// This test verifies that JQLMetadata makes parallel calls and aggregates results.
	// We serve a single handler that routes by path.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		path := r.URL.Path
		switch {
		case strings.HasSuffix(path, "/status"):
			_, _ = w.Write([]byte(`[
				{"name":"Open","statusCategory":{"key":"new"}},
				{"name":"In Progress","statusCategory":{"key":"indeterminate"}},
				{"name":"Done","statusCategory":{"key":"done"}}
			]`))
		case strings.HasSuffix(path, "/issuetype"):
			_, _ = w.Write([]byte(`[{"name":"Story"},{"name":"Bug"},{"name":"Task"}]`))
		case strings.HasSuffix(path, "/priority"):
			_, _ = w.Write([]byte(`[{"name":"High"},{"name":"Medium"},{"name":"Low"}]`))
		case strings.HasSuffix(path, "/resolution"):
			_, _ = w.Write([]byte(`[{"name":"Fixed"},{"name":"Won't Fix"}]`))
		case strings.HasSuffix(path, "/project"):
			_, _ = w.Write([]byte(`[{"key":"TEST","name":"Test","projectTypeKey":"software"}]`))
		case strings.HasSuffix(path, "/label"):
			_, _ = w.Write([]byte(`{"values":["backend","frontend","urgent"]}`))
		case strings.Contains(path, "/project/TEST/components"):
			_, _ = w.Write([]byte(`[{"name":"API"},{"name":"UI"}]`))
		case strings.Contains(path, "/project/TEST/version"):
			_, _ = w.Write([]byte(`[{"name":"1.0","released":false,"archived":false},{"name":"0.9","released":true,"archived":false}]`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errorMessages":["not found: ` + path + `"]}`))
		}
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	meta, err := c.JQLMetadata()
	if err != nil {
		t.Fatal(err)
	}

	// Statuses.
	if len(meta.Statuses) != 3 {
		t.Errorf("Statuses len = %d, want 3", len(meta.Statuses))
	}

	// Status categories should be populated.
	if meta.StatusCategories == nil {
		t.Fatal("StatusCategories should not be nil")
	}
	if meta.StatusCategories["Open"] != 0 {
		t.Errorf("StatusCategories[Open] = %d, want 0 (todo)", meta.StatusCategories["Open"])
	}
	if meta.StatusCategories["In Progress"] != 1 {
		t.Errorf("StatusCategories[In Progress] = %d, want 1", meta.StatusCategories["In Progress"])
	}
	if meta.StatusCategories["Done"] != 2 {
		t.Errorf("StatusCategories[Done] = %d, want 2", meta.StatusCategories["Done"])
	}

	// Issue types.
	if len(meta.IssueTypes) != 3 {
		t.Errorf("IssueTypes len = %d, want 3", len(meta.IssueTypes))
	}

	// Priorities.
	if len(meta.Priorities) != 3 {
		t.Errorf("Priorities len = %d, want 3", len(meta.Priorities))
	}

	// Resolutions.
	if len(meta.Resolutions) != 2 {
		t.Errorf("Resolutions len = %d, want 2", len(meta.Resolutions))
	}

	// Projects.
	if len(meta.Projects) != 1 || meta.Projects[0] != "TEST" {
		t.Errorf("Projects = %v, want [TEST]", meta.Projects)
	}

	// Labels.
	if len(meta.Labels) != 3 {
		t.Errorf("Labels len = %d, want 3", len(meta.Labels))
	}

	// Components (project-scoped).
	if len(meta.Components) != 2 {
		t.Errorf("Components len = %d, want 2", len(meta.Components))
	}

	// Versions — only unreleased, non-archived versions should be returned.
	if len(meta.Versions) != 1 || meta.Versions[0] != "1.0" {
		t.Errorf("Versions = %v, want [1.0]", meta.Versions)
	}
}

func TestJQLMetadata_NoProject(t *testing.T) {
	// When project is empty, components and versions should be nil/empty.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path

		switch {
		case strings.HasSuffix(path, "/status"):
			_, _ = w.Write([]byte(`[{"name":"Open","statusCategory":{"key":"new"}}]`))
		case strings.HasSuffix(path, "/issuetype"):
			_, _ = w.Write([]byte(`[{"name":"Story"}]`))
		case strings.HasSuffix(path, "/priority"):
			_, _ = w.Write([]byte(`[{"name":"High"}]`))
		case strings.HasSuffix(path, "/resolution"):
			_, _ = w.Write([]byte(`[{"name":"Fixed"}]`))
		case strings.HasSuffix(path, "/project"):
			_, _ = w.Write([]byte(`[{"key":"TEST","name":"Test","projectTypeKey":"software"}]`))
		case strings.HasSuffix(path, "/label"):
			_, _ = w.Write([]byte(`{"values":["backend"]}`))
		default:
			t.Errorf("unexpected request to %s — components/versions should not be fetched without project", path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	// Override config to have empty project.
	c.config.Project = ""

	meta, err := c.JQLMetadata()
	if err != nil {
		t.Fatal(err)
	}

	if len(meta.Components) != 0 {
		t.Errorf("Components len = %d, want 0 (no project)", len(meta.Components))
	}
	if len(meta.Versions) != 0 {
		t.Errorf("Versions len = %d, want 0 (no project)", len(meta.Versions))
	}
}

func TestJQLMetadata_SilentlyIgnoresFailures(t *testing.T) {
	// Individual endpoint failures should not cause the whole call to fail.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path

		switch {
		case strings.HasSuffix(path, "/status"):
			_, _ = w.Write([]byte(`[{"name":"Open","statusCategory":{"key":"new"}}]`))
		case strings.HasSuffix(path, "/issuetype"):
			// Simulate a failure.
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`error`))
		case strings.HasSuffix(path, "/priority"):
			_, _ = w.Write([]byte(`[{"name":"High"}]`))
		case strings.HasSuffix(path, "/resolution"):
			_, _ = w.Write([]byte(`[{"name":"Fixed"}]`))
		case strings.HasSuffix(path, "/project"):
			_, _ = w.Write([]byte(`[{"key":"TEST","name":"Test","projectTypeKey":"software"}]`))
		case strings.HasSuffix(path, "/label"):
			_, _ = w.Write([]byte(`{"values":["backend"]}`))
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))
		}
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	c.config.Project = ""

	meta, err := c.JQLMetadata()
	if err != nil {
		t.Fatal(err)
	}

	// Statuses should still work.
	if len(meta.Statuses) != 1 {
		t.Errorf("Statuses len = %d, want 1", len(meta.Statuses))
	}
	// Issue types should be nil due to the error.
	if meta.IssueTypes != nil {
		t.Errorf("IssueTypes = %v, want nil (endpoint failed)", meta.IssueTypes)
	}
	// Other fields should still be populated.
	if len(meta.Priorities) != 1 {
		t.Errorf("Priorities len = %d, want 1", len(meta.Priorities))
	}
}

// --- IssueTypesWithID tests ---

func TestIssueTypesWithID_ProjectScoped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if strings.Contains(r.URL.Path, "/createmeta") {
			_, _ = w.Write([]byte(`{
				"projects": [{
					"key": "TEST",
					"issuetypes": [
						{"id": "10001", "name": "Story"},
						{"id": "10002", "name": "Bug"}
					]
				}]
			}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	types, err := c.IssueTypesWithID("TEST")
	if err != nil {
		t.Fatal(err)
	}

	if len(types) != 2 {
		t.Fatalf("len = %d, want 2", len(types))
	}
	if types[0].ID != "10001" || types[0].Name != "Story" {
		t.Errorf("types[0] = %+v, want {10001 Story}", types[0])
	}
}

func TestIssueTypesWithID_FallsBackToGlobal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if strings.Contains(r.URL.Path, "/createmeta") {
			// Simulate createmeta failure (deprecated on some instances).
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errorMessages":["not found"]}`))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/issuetype") {
			_, _ = w.Write([]byte(`[{"name":"Task"},{"name":"Epic"}]`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := newTestClient(srv, "basic")
	types, err := c.IssueTypesWithID("TEST")
	if err != nil {
		t.Fatal(err)
	}

	// Fallback should return types without IDs.
	if len(types) != 2 {
		t.Fatalf("len = %d, want 2", len(types))
	}
	if types[0].ID != "" {
		t.Errorf("types[0].ID = %q, want empty (fallback has no IDs)", types[0].ID)
	}
	if types[0].Name != "Task" {
		t.Errorf("types[0].Name = %q, want %q", types[0].Name, "Task")
	}
}
