package client

import (
	"testing"

	"github.com/undont/jiru/internal/api"
)

func TestConvertConfluencePage_NilVersion(t *testing.T) {
	p := &api.ConfluencePage{
		ID:    "123",
		Title: "Test",
	}
	page := convertConfluencePage(p)
	if page.Version != 0 {
		t.Errorf("version = %d, want 0", page.Version)
	}
	if page.Author != "" {
		t.Errorf("author = %q, want empty", page.Author)
	}
}

func TestConvertConfluencePage_NilBody(t *testing.T) {
	p := &api.ConfluencePage{
		ID:    "123",
		Title: "Test",
	}
	page := convertConfluencePage(p)
	if page.BodyADF != "" {
		t.Errorf("bodyADF = %q, want empty", page.BodyADF)
	}
	if page.BodyStore != "" {
		t.Errorf("bodyStore = %q, want empty", page.BodyStore)
	}
}

func TestConvertConfluencePage_WithBody(t *testing.T) {
	p := &api.ConfluencePage{
		ID:    "123",
		Title: "Test",
		Body: &struct {
			Storage *struct {
				Value string `json:"value"`
			} `json:"storage"`
			AtlasDocFormat *struct {
				Value string `json:"value"`
			} `json:"atlas_doc_format"`
		}{
			AtlasDocFormat: &struct {
				Value string `json:"value"`
			}{Value: `{"type":"doc"}`},
			Storage: &struct {
				Value string `json:"value"`
			}{Value: "<p>hello</p>"},
		},
	}
	page := convertConfluencePage(p)
	if page.BodyADF != `{"type":"doc"}` {
		t.Errorf("bodyADF = %q", page.BodyADF)
	}
	if page.BodyStore != "<p>hello</p>" {
		t.Errorf("bodyStore = %q", page.BodyStore)
	}
}

func TestConvertConfluencePage_WithVersion(t *testing.T) {
	p := &api.ConfluencePage{
		ID:    "123",
		Title: "Test",
		Version: &struct {
			Number    int    `json:"number"`
			Message   string `json:"message"`
			CreatedAt string `json:"createdAt"`
			AuthorID  string `json:"authorId"`
		}{
			Number:    5,
			AuthorID:  "user-abc",
			CreatedAt: "2025-06-01T12:00:00Z",
		},
		CreatedAt: "2025-05-01T10:00:00Z",
	}
	page := convertConfluencePage(p)
	if page.Version != 5 {
		t.Errorf("version = %d, want 5", page.Version)
	}
	if page.Author != "user-abc" {
		t.Errorf("author = %q, want user-abc", page.Author)
	}
	if page.Updated.IsZero() {
		t.Error("updated should not be zero")
	}
	if page.Created.IsZero() {
		t.Error("created should not be zero")
	}
}

func TestExtractPath_FullURL(t *testing.T) {
	got := extractPath("https://example.atlassian.net/wiki/api/v2/spaces?limit=25&cursor=abc")
	want := "/wiki/api/v2/spaces?limit=25&cursor=abc"
	if got != want {
		t.Errorf("extractPath = %q, want %q", got, want)
	}
}

func TestExtractPath_NoQuery(t *testing.T) {
	got := extractPath("https://example.atlassian.net/wiki/api/v2/spaces")
	want := "/wiki/api/v2/spaces"
	if got != want {
		t.Errorf("extractPath = %q, want %q", got, want)
	}
}

func TestExtractPath_Empty(t *testing.T) {
	got := extractPath("")
	if got != "" {
		t.Errorf("extractPath empty = %q, want empty", got)
	}
}
