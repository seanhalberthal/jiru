package demo

import "testing"

func TestDeleteIssueLink_RemovesSeededLink(t *testing.T) {
	c := New()
	featured := ProjectKey + "-101"

	before, err := c.GetIssue(featured)
	if err != nil {
		t.Fatal(err)
	}
	if len(before.LinkedIssues) != 2 {
		t.Fatalf("seeded links = %d, want 2", len(before.LinkedIssues))
	}

	if err := c.DeleteIssueLink("20001"); err != nil {
		t.Fatalf("DeleteIssueLink: %v", err)
	}

	after, err := c.GetIssue(featured)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.LinkedIssues) != 1 {
		t.Fatalf("links after delete = %d, want 1", len(after.LinkedIssues))
	}
	if after.LinkedIssues[0].LinkID != "20002" {
		t.Errorf("remaining link = %q, want 20002", after.LinkedIssues[0].LinkID)
	}
}

func TestDeleteIssueLink_UnknownIDErrors(t *testing.T) {
	c := New()
	if err := c.DeleteIssueLink("does-not-exist"); err == nil {
		t.Error("expected an error deleting a non-existent link")
	}
}

func TestLinkIssue_ThenDeleteRoundTrip(t *testing.T) {
	c := New()
	source := ProjectKey + "-102"
	target := ProjectKey + "-103"

	if err := c.LinkIssue(target, source, "Blocks"); err != nil {
		t.Fatalf("LinkIssue: %v", err)
	}

	src, err := c.GetIssue(source)
	if err != nil {
		t.Fatal(err)
	}
	var linkID string
	for _, l := range src.LinkedIssues {
		if l.Key == target {
			linkID = l.LinkID
		}
	}
	if linkID == "" {
		t.Fatalf("created link not found on source %s", source)
	}

	// The reciprocal entry should appear on the target with the same ID.
	tgt, err := c.GetIssue(target)
	if err != nil {
		t.Fatal(err)
	}
	reciprocal := false
	for _, l := range tgt.LinkedIssues {
		if l.LinkID == linkID && l.Key == source {
			reciprocal = true
		}
	}
	if !reciprocal {
		t.Errorf("reciprocal link not found on target %s", target)
	}

	// Deleting by ID clears both sides.
	if err := c.DeleteIssueLink(linkID); err != nil {
		t.Fatalf("DeleteIssueLink: %v", err)
	}
	src, _ = c.GetIssue(source)
	for _, l := range src.LinkedIssues {
		if l.LinkID == linkID {
			t.Error("link still present on source after delete")
		}
	}
	tgt, _ = c.GetIssue(target)
	for _, l := range tgt.LinkedIssues {
		if l.LinkID == linkID {
			t.Error("link still present on target after delete")
		}
	}
}
