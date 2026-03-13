package client

import (
	"testing"

	"github.com/seanhalberthal/jiratui/internal/jira"
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
