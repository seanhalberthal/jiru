package issueview

import (
	"strings"
	"testing"

	"github.com/seanhalberthal/jiru/internal/jira"
)

func TestWrapText(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		width int
		check func(t *testing.T, result string)
	}{
		{
			name:  "zero width passthrough",
			text:  "some text here",
			width: 0,
			check: func(t *testing.T, result string) {
				if result != "some text here" {
					t.Errorf("expected passthrough, got %q", result)
				}
			},
		},
		{
			name:  "negative width passthrough",
			text:  "some text",
			width: -5,
			check: func(t *testing.T, result string) {
				if result != "some text" {
					t.Errorf("expected passthrough, got %q", result)
				}
			},
		},
		{
			name:  "empty string",
			text:  "",
			width: 40,
			check: func(t *testing.T, result string) {
				if result != "" {
					t.Errorf("expected empty, got %q", result)
				}
			},
		},
		{
			name:  "short line no wrap",
			text:  "hello world",
			width: 40,
			check: func(t *testing.T, result string) {
				if result != "hello world" {
					t.Errorf("expected no wrap, got %q", result)
				}
			},
		},
		{
			name:  "long line wraps",
			text:  "the quick brown fox jumps over the lazy dog",
			width: 20,
			check: func(t *testing.T, result string) {
				lines := strings.Split(result, "\n")
				if len(lines) < 2 {
					t.Errorf("expected multiple lines, got %d", len(lines))
				}
			},
		},
		{
			name:  "preserves newlines",
			text:  "line one\nline two",
			width: 40,
			check: func(t *testing.T, result string) {
				if !strings.Contains(result, "line one") || !strings.Contains(result, "line two") {
					t.Errorf("expected both lines preserved, got %q", result)
				}
			},
		},
		{
			name:  "single long word",
			text:  "superlongwordthatwontfit",
			width: 10,
			check: func(t *testing.T, result string) {
				if !strings.Contains(result, "superlongwordthatwontfit") {
					t.Errorf("expected long word preserved, got %q", result)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wrapText(tt.text, tt.width)
			tt.check(t, result)
		})
	}
}

func TestOpenURL_SentinelReset(t *testing.T) {
	m := New()
	m.SetIssueURL("https://jira.example.com/browse/PROJ-1")

	// No URL requested yet.
	_, ok := m.OpenURL()
	if ok {
		t.Error("expected no URL before request")
	}

	// Simulate 'o' key press.
	m.openURL = true
	url, ok := m.OpenURL()
	if !ok {
		t.Fatal("expected URL after request")
	}
	if url != "https://jira.example.com/browse/PROJ-1" {
		t.Errorf("expected URL, got %q", url)
	}

	// Should reset.
	_, ok = m.OpenURL()
	if ok {
		t.Error("expected reset after read")
	}
}

func TestOpenURL_EmptyURL(t *testing.T) {
	m := New()
	m.openURL = true

	_, ok := m.OpenURL()
	if ok {
		t.Error("expected no URL when issueURL is empty")
	}
}

func TestSetIssue_RendersContent(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)

	iss := jira.Issue{
		Key:       "PROJ-1",
		Summary:   "Test Issue",
		Status:    "In Progress",
		IssueType: "Story",
		Priority:  "High",
		Assignee:  "Alice",
		Reporter:  "Bob",
	}
	m = m.SetIssue(iss)

	view := m.View()
	if !strings.Contains(view, "PROJ-1") {
		t.Error("expected issue key in view")
	}
	if !strings.Contains(view, "Test Issue") {
		t.Error("expected summary in view")
	}
}

func TestSetIssue_EmptyDescription(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)

	iss := jira.Issue{
		Key:     "PROJ-1",
		Summary: "No desc",
		Status:  "To Do",
	}
	m = m.SetIssue(iss)

	view := m.View()
	if !strings.Contains(view, "No description") {
		t.Error("expected 'No description' fallback text")
	}
}

func TestSetIssue_CommentsTruncatedToLast10(t *testing.T) {
	comments := make([]jira.Comment, 15)
	for i := range comments {
		comments[i] = jira.Comment{
			Author: "User",
			Body:   "Comment body",
		}
	}

	m := New()
	m = m.SetSize(80, 40)

	iss := jira.Issue{
		Key:      "PROJ-1",
		Summary:  "Many comments",
		Status:   "To Do",
		Comments: comments,
	}
	m = m.SetIssue(iss)

	view := m.View()
	if !strings.Contains(view, "15") {
		t.Error("expected comment count to show 15")
	}
}

func TestNoIssueView(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)
	view := m.View()
	if !strings.Contains(view, "No issue selected") {
		t.Errorf("expected 'No issue selected', got %q", view)
	}
}
