package issuedelegate_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"

	"github.com/undont/jiru/internal/jira"
	"github.com/undont/jiru/internal/ui/issuedelegate"
)

func TestToItems_PreservesLength(t *testing.T) {
	issues := []jira.Issue{
		{Key: "A-1", Summary: "First"},
		{Key: "A-2", Summary: "Second"},
		{Key: "A-3", Summary: "Third"},
	}
	items := issuedelegate.ToItems(issues)
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
}

func TestToItems_Empty(t *testing.T) {
	items := issuedelegate.ToItems(nil)
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
}

func TestToItems_PreservesOrder(t *testing.T) {
	issues := []jira.Issue{
		{Key: "Z-1", Summary: "Zulu"},
		{Key: "A-1", Summary: "Alpha"},
	}
	items := issuedelegate.ToItems(issues)
	first := items[0].(issuedelegate.Item)
	if first.Issue.Key != "Z-1" {
		t.Errorf("expected first item key Z-1, got %q", first.Issue.Key)
	}
}

func TestItem_FilterValue(t *testing.T) {
	item := issuedelegate.Item{Issue: jira.Issue{Key: "PROJ-42", Summary: "Fix login"}}
	got := item.FilterValue()
	if !strings.Contains(got, "PROJ-42") || !strings.Contains(got, "Fix login") {
		t.Errorf("FilterValue() = %q, expected to contain key and summary", got)
	}
}

func TestItem_FilterValue_Empty(t *testing.T) {
	item := issuedelegate.Item{Issue: jira.Issue{}}
	got := item.FilterValue()
	if got != " " {
		t.Errorf("FilterValue() = %q, expected ' ' for empty issue", got)
	}
}

func TestDelegate_Height(t *testing.T) {
	d := issuedelegate.Delegate{}
	if d.Height() != 2 {
		t.Errorf("Height() = %d, want 2", d.Height())
	}
}

func TestDelegate_Spacing(t *testing.T) {
	d := issuedelegate.Delegate{}
	if d.Spacing() != 0 {
		t.Errorf("Spacing() = %d, want 0", d.Spacing())
	}
}

func TestDelegate_Render_NoPanic(t *testing.T) {
	d := issuedelegate.Delegate{}
	item := issuedelegate.Item{
		Issue: jira.Issue{
			Key:       "PROJ-123",
			Summary:   "A very long summary that exceeds available width for testing",
			Status:    "Done",
			IssueType: "Story",
			Assignee:  "alice",
		},
	}

	items := []list.Item{item}
	// Test various widths, especially narrow ones.
	for _, width := range []int{10, 20, 40, 80, 120} {
		l := list.New(items, d, width, 10)
		var buf bytes.Buffer
		d.Render(&buf, l, 0, item)
		if buf.Len() == 0 {
			t.Errorf("Render at width %d produced no output", width)
		}
	}
}

func TestDelegate_Render_Selected(t *testing.T) {
	d := issuedelegate.Delegate{}
	item := issuedelegate.Item{
		Issue: jira.Issue{
			Key:     "PROJ-1",
			Summary: "Test",
			Status:  "To Do",
		},
	}

	items := []list.Item{item}
	l := list.New(items, d, 80, 10)

	var buf bytes.Buffer
	// Index 0 is selected by default.
	d.Render(&buf, l, 0, item)
	output := buf.String()
	if !strings.Contains(output, "PROJ-1") {
		t.Error("expected issue key in render output")
	}
}

func TestDelegate_Render_Unassigned(t *testing.T) {
	d := issuedelegate.Delegate{}
	item := issuedelegate.Item{
		Issue: jira.Issue{
			Key:     "PROJ-1",
			Summary: "Test",
			Status:  "Open",
		},
	}

	items := []list.Item{item}
	l := list.New(items, d, 80, 10)

	var buf bytes.Buffer
	d.Render(&buf, l, 0, item)
	output := buf.String()
	if !strings.Contains(output, "Unassigned") {
		t.Error("expected 'Unassigned' for empty assignee")
	}
}
