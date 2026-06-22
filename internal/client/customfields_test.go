package client

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/undont/jiru/internal/api"
)

// --- IssueFields custom-field decoding ---

func TestIssueFields_UnmarshalJSON_CapturesAllCustomFields(t *testing.T) {
	iss := issueFromJSON(t, `{
		"key": "X-1",
		"fields": {
			"summary": "captured",
			"customfield_10016": 5,
			"customfield_99999": "hello",
			"labels": ["a"]
		}
	}`)
	if got := iss.Fields.Custom["customfield_10016"]; string(got) != "5" {
		t.Errorf("customfield_10016 = %q, want 5", string(got))
	}
	if got, ok := iss.Fields.Custom["customfield_99999"]; !ok || string(got) != `"hello"` {
		t.Errorf("customfield_99999 raw = %q (ok=%v)", string(got), ok)
	}
	// Non-customfield_* keys must NOT leak into the map.
	if _, ok := iss.Fields.Custom["summary"]; ok {
		t.Error("Custom should only hold customfield_* keys")
	}
	if _, ok := iss.Fields.Custom["labels"]; ok {
		t.Error("Custom should only hold customfield_* keys")
	}
}

func TestIssueFields_StoryPoints_PrefersDiscoveredID(t *testing.T) {
	iss := issueFromJSON(t, `{
		"key": "X-2",
		"fields": {
			"customfield_10016": 8,
			"customfield_99999": 13
		}
	}`)
	// Discovered ID takes precedence over the well-known fallback.
	got := iss.Fields.StoryPoints("customfield_99999")
	if got == nil || *got != 13 {
		t.Errorf("StoryPoints with preferred = %v, want 13", got)
	}
}

func TestIssueFields_StoryPoints_FallsBackToWellKnown(t *testing.T) {
	iss := issueFromJSON(t, `{
		"key": "X-3",
		"fields": {
			"customfield_10026": 21
		}
	}`)
	got := iss.Fields.StoryPoints()
	if got == nil || *got != 21 {
		t.Errorf("StoryPoints with no preferred = %v, want 21 (from customfield_10026)", got)
	}
}

func TestIssueFields_StoryPoints_NullAndEmpty(t *testing.T) {
	iss := issueFromJSON(t, `{
		"key": "X-4",
		"fields": {
			"customfield_10016": null
		}
	}`)
	if got := iss.Fields.StoryPoints(); got != nil {
		t.Errorf("StoryPoints with null = %v, want nil", got)
	}

	iss2 := issueFromJSON(t, `{
		"key": "X-5",
		"fields": {}
	}`)
	if got := iss2.Fields.StoryPoints("customfield_99999"); got != nil {
		t.Errorf("StoryPoints with no fields = %v, want nil", got)
	}
}

func TestIssueFields_StoryPoints_DedupsPreferred(t *testing.T) {
	iss := issueFromJSON(t, `{
		"key": "X-6",
		"fields": {"customfield_99999": 3}
	}`)
	// Passing the same ID twice (and an empty) shouldn't crash.
	got := iss.Fields.StoryPoints("customfield_99999", "customfield_99999", "")
	if got == nil || *got != 3 {
		t.Errorf("StoryPoints with dedup = %v, want 3", got)
	}
}

// --- convertIssue: FixVersions + StoryPoints integration ---

func TestConvertIssue_FixVersions(t *testing.T) {
	iss := issueFromJSON(t, `{
		"key": "FV-1",
		"fields": {
			"summary": "with versions",
			"fixVersions": [
				{"name": "v1.2.0"},
				{"name": "v1.3.0"},
				{"name": ""}
			]
		}
	}`)
	out := convertIssue(iss)
	want := []string{"v1.2.0", "v1.3.0"}
	if len(out.FixVersions) != len(want) {
		t.Fatalf("FixVersions = %v, want %v", out.FixVersions, want)
	}
	for i, v := range want {
		if out.FixVersions[i] != v {
			t.Errorf("FixVersions[%d] = %q, want %q", i, out.FixVersions[i], v)
		}
	}
}

func TestConvertIssue_FixVersions_EmptyOmitted(t *testing.T) {
	iss := issueFromJSON(t, `{
		"key": "FV-2",
		"fields": {"summary": "no versions"}
	}`)
	out := convertIssue(iss)
	if out.FixVersions != nil {
		t.Errorf("FixVersions should be nil for missing field, got %v", out.FixVersions)
	}
}

func TestConvertIssue_StoryPoints_UsesPreferredID(t *testing.T) {
	iss := issueFromJSON(t, `{
		"key": "SP-1",
		"fields": {
			"summary": "sp",
			"customfield_77777": 8.5,
			"customfield_10016": 1
		}
	}`)
	out := convertIssue(iss, "customfield_77777")
	if out.StoryPoints == nil || *out.StoryPoints != 8.5 {
		t.Errorf("StoryPoints = %v, want 8.5", out.StoryPoints)
	}
}

func TestExtractFixVersions_NilAndAllEmpty(t *testing.T) {
	if got := extractFixVersions(nil); got != nil {
		t.Errorf("extractFixVersions(nil) = %v, want nil", got)
	}
	if got := extractFixVersions([]api.NameField{{Name: ""}, {Name: ""}}); got != nil {
		t.Errorf("extractFixVersions(all empty) = %v, want nil", got)
	}
}

// --- discoverStoryPointsField via /rest/api/2/field ---

func TestStoryPointsFieldID_DiscoversByName(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/field") {
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"id":"summary","name":"Summary","custom":false},
			{"id":"customfield_12345","name":"Story Points","custom":true},
			{"id":"customfield_99999","name":"Sprint","custom":true}
		]`))
	}))
	defer srv.Close()

	c := newDiscoveryTestClient(srv)
	if got := c.StoryPointsFieldID(); got != "customfield_12345" {
		t.Errorf("StoryPointsFieldID = %q, want customfield_12345", got)
	}
	// Calling again must not re-hit the endpoint (sync.Once).
	_ = c.StoryPointsFieldID()
	_ = c.StoryPointsFieldID()
	if h := atomic.LoadInt32(&hits); h != 1 {
		t.Errorf("/field hit count = %d, want 1 (sync.Once)", h)
	}
}

func TestStoryPointsFieldID_PrefersStoryPointsOverEstimate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"id":"customfield_AAA","name":"Story point estimate","custom":true},
			{"id":"customfield_BBB","name":"Story Points","custom":true}
		]`))
	}))
	defer srv.Close()

	c := newDiscoveryTestClient(srv)
	if got := c.StoryPointsFieldID(); got != "customfield_BBB" {
		t.Errorf("StoryPointsFieldID = %q, want customfield_BBB (prefer 'Story Points')", got)
	}
}

func TestStoryPointsFieldID_FallbackContainsMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"id":"customfield_777","name":"Custom Story point thing","custom":true}
		]`))
	}))
	defer srv.Close()

	c := newDiscoveryTestClient(srv)
	if got := c.StoryPointsFieldID(); got != "customfield_777" {
		t.Errorf("StoryPointsFieldID = %q, want customfield_777 (lenient match)", got)
	}
}

func TestStoryPointsFieldID_NoMatchReturnsEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"id":"customfield_111","name":"Sprint","custom":true},
			{"id":"customfield_222","name":"Epic Link","custom":true}
		]`))
	}))
	defer srv.Close()

	c := newDiscoveryTestClient(srv)
	if got := c.StoryPointsFieldID(); got != "" {
		t.Errorf("StoryPointsFieldID = %q, want empty (no SP field)", got)
	}
}

func TestStoryPointsFieldID_HTTPErrorReturnsEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newDiscoveryTestClient(srv)
	if got := c.StoryPointsFieldID(); got != "" {
		t.Errorf("StoryPointsFieldID on 500 = %q, want empty", got)
	}
}

// --- searchFields() composition ---

func TestSearchFields_IncludesDiscoveredAndKnownIDs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"customfield_42","name":"Story Points","custom":true}]`))
	}))
	defer srv.Close()

	c := newDiscoveryTestClient(srv)
	fields := c.searchFields()

	// Discovered ID must appear first among the SP IDs.
	for _, want := range []string{"customfield_42", "customfield_10016", "customfield_10026", "customfield_10002", "customfield_10004", "fixVersions", "summary"} {
		if !strings.Contains(fields, want) {
			t.Errorf("searchFields missing %q: %s", want, fields)
		}
	}

	// Discovered ID should not be duplicated.
	if strings.Count(fields, "customfield_42") != 1 {
		t.Errorf("customfield_42 should appear once, got %s", fields)
	}
}

func TestSearchFields_NoDiscoveredID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := newDiscoveryTestClient(srv)
	fields := c.searchFields()
	for _, want := range api.StoryPointFieldIDs {
		if !strings.Contains(fields, want) {
			t.Errorf("searchFields missing fallback %q: %s", want, fields)
		}
	}
}

// newDiscoveryTestClient returns a Client that routes /field calls to srv but
// otherwise mirrors newTestClient. Unlike newTestClient, it does NOT prime
// spFieldOnce — the whole point is to exercise discovery.
func newDiscoveryTestClient(srv *httptest.Server) *Client {
	c := newTestClient(srv, "basic")
	c.spFieldOnce = sync.Once{}
	c.spFieldID = ""
	return c
}
