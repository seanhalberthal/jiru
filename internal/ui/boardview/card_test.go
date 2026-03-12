package boardview

import (
	"strings"
	"testing"

	"github.com/seanhalberthal/jiratui/internal/jira"
)

func TestRenderCardContainsKey(t *testing.T) {
	issue := jira.Issue{
		Key:       "PROJ-42",
		Summary:   "Fix the login bug",
		IssueType: "Bug",
		Assignee:  "Alice",
	}
	card := renderCard(issue, 40, false)
	if !strings.Contains(card, "PROJ-42") {
		t.Errorf("card should contain issue key, got: %s", card)
	}
}

func TestRenderCardContainsAssignee(t *testing.T) {
	issue := jira.Issue{
		Key:       "PROJ-1",
		Summary:   "Some task",
		IssueType: "Story",
		Assignee:  "Bob",
	}
	card := renderCard(issue, 40, false)
	if !strings.Contains(card, "Bob") {
		t.Errorf("card should contain assignee, got: %s", card)
	}
}

func TestRenderCardUnassigned(t *testing.T) {
	issue := jira.Issue{
		Key:       "PROJ-1",
		Summary:   "Orphan task",
		IssueType: "Task",
	}
	card := renderCard(issue, 40, false)
	if !strings.Contains(card, "Unassigned") {
		t.Errorf("card should show 'Unassigned' for empty assignee, got: %s", card)
	}
}

func TestRenderCardTruncatesLongSummary(t *testing.T) {
	issue := jira.Issue{
		Key:       "PROJ-1",
		Summary:   "This is a very long summary that should be truncated when it exceeds the card width",
		IssueType: "Story",
		Assignee:  "Alice",
	}
	card := renderCard(issue, 30, false)
	if !strings.Contains(card, "...") {
		t.Errorf("expected truncated summary with '...', got: %s", card)
	}
}

func TestRenderCardIssueType(t *testing.T) {
	issue := jira.Issue{
		Key:       "PROJ-1",
		Summary:   "Test",
		IssueType: "Bug",
		Assignee:  "Alice",
	}
	card := renderCard(issue, 40, false)
	if !strings.Contains(card, "Bug") {
		t.Errorf("card should contain issue type, got: %s", card)
	}
}
