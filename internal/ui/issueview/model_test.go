package issueview

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/seanhalberthal/jiru/internal/jira"
)

func TestExtractConfluencePages_WikiMarkupLink(t *testing.T) {
	text := `See [Confluence Spec|https://mysite.atlassian.net/wiki/spaces/PD/pages/355467265] for details`
	refs := extractConfluencePages(text)
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref, got %d", len(refs))
	}
	if refs[0].Key != "355467265" {
		t.Errorf("Key = %q, want %q", refs[0].Key, "355467265")
	}
	if refs[0].Display != "Confluence Spec" {
		t.Errorf("Display = %q, want %q", refs[0].Display, "Confluence Spec")
	}
	if refs[0].Group != "Confluence Pages" {
		t.Errorf("Group = %q, want %q", refs[0].Group, "Confluence Pages")
	}
}

func TestExtractConfluencePages_BareURL(t *testing.T) {
	text := `Check https://mysite.atlassian.net/wiki/spaces/ENG/pages/12345 for info`
	refs := extractConfluencePages(text)
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref, got %d", len(refs))
	}
	if refs[0].Key != "12345" {
		t.Errorf("Key = %q, want %q", refs[0].Key, "12345")
	}
	if refs[0].Display != "Page 12345" {
		t.Errorf("Display = %q, want fallback %q", refs[0].Display, "Page 12345")
	}
}

func TestExtractConfluencePages_Multiple(t *testing.T) {
	text := `[Page A|https://x.atlassian.net/wiki/spaces/A/pages/111] and [Page B|https://x.atlassian.net/wiki/spaces/B/pages/222]`
	refs := extractConfluencePages(text)
	if len(refs) != 2 {
		t.Fatalf("expected 2 refs, got %d", len(refs))
	}
	if refs[0].Key != "111" || refs[1].Key != "222" {
		t.Errorf("keys = [%q, %q], want [111, 222]", refs[0].Key, refs[1].Key)
	}
}

func TestExtractConfluencePages_Empty(t *testing.T) {
	if refs := extractConfluencePages(""); len(refs) != 0 {
		t.Errorf("expected no refs for empty text, got %d", len(refs))
	}
}

func TestExtractConfluencePages_NoMatch(t *testing.T) {
	if refs := extractConfluencePages("Just a normal description with PROJ-123"); len(refs) != 0 {
		t.Errorf("expected no confluence refs, got %d", len(refs))
	}
}

func TestIssueKeys_IncludesConfluencePages(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)

	iss := jira.Issue{
		Key:         "PROJ-1",
		Summary:     "Test",
		Status:      "To Do",
		Description: "See [Design Doc|https://x.atlassian.net/wiki/spaces/ENG/pages/99999] for specs",
	}
	m = m.SetIssue(iss)

	refs := m.IssueKeys()
	var found bool
	for _, r := range refs {
		if r.Key == "99999" && r.Group == "Confluence Pages" && r.Display == "Design Doc" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Confluence page ref with key=99999, group='Confluence Pages', display='Design Doc'")
	}
}

func TestIssueKeys_ConfluencePagesDeduped(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)

	iss := jira.Issue{
		Key:         "PROJ-1",
		Summary:     "Test",
		Status:      "To Do",
		Description: "[Doc|https://x.atlassian.net/wiki/spaces/A/pages/111]",
		Comments: []jira.Comment{
			{Author: "Alice", Body: "[Doc|https://x.atlassian.net/wiki/spaces/A/pages/111]"},
		},
	}
	m = m.SetIssue(iss)

	refs := m.IssueKeys()
	count := 0
	for _, r := range refs {
		if r.Key == "111" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected page 111 once (deduped), got %d", count)
	}
}

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

func TestSetIssue_ShowsStatusInMetadata(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)

	iss := jira.Issue{
		Key:     "PROJ-1",
		Summary: "Test",
		Status:  "In Progress",
	}
	m = m.SetIssue(iss)

	// Status is shown in the header, not in the body metadata.
	view := m.View()
	if !strings.Contains(view, "In Progress") {
		t.Error("expected status in header")
	}
	content := m.renderContent()
	if strings.Contains(content, "Status:") {
		t.Error("Status field should not appear in body metadata (it's in the header)")
	}
}

func TestSetIssue_ShowsParent(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)

	iss := jira.Issue{
		Key:           "PROJ-2",
		Summary:       "Child",
		Status:        "To Do",
		ParentKey:     "PROJ-1",
		ParentSummary: "Parent Issue",
		ParentType:    "Epic",
	}
	m = m.SetIssue(iss)

	content := m.renderContent()
	if !strings.Contains(content, "Parent:") {
		t.Error("expected Parent field in metadata")
	}
	if !strings.Contains(content, "PROJ-1") {
		t.Error("expected parent key in metadata")
	}
	if !strings.Contains(content, "Parent Issue") {
		t.Error("expected parent summary in metadata")
	}
	if !strings.Contains(content, "Epic") {
		t.Error("expected parent type in metadata")
	}
}

func TestSetIssue_NoParentWhenEmpty(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)

	iss := jira.Issue{
		Key:     "PROJ-1",
		Summary: "No parent",
		Status:  "To Do",
	}
	m = m.SetIssue(iss)

	content := m.renderContent()
	if strings.Contains(content, "Parent:") {
		t.Error("expected no Parent field when ParentKey is empty")
	}
}

func TestSetChildren_ShowsChildren(t *testing.T) {
	m := New()
	m = m.SetSize(80, 30)

	iss := jira.Issue{
		Key:     "PROJ-1",
		Summary: "Parent",
		Status:  "In Progress",
	}
	m = m.SetIssue(iss)
	m = m.SetChildren([]jira.ChildIssue{
		{Key: "PROJ-2", Summary: "Child One", Status: "To Do", IssueType: "Sub-task"},
		{Key: "PROJ-3", Summary: "Child Two", Status: "Done", IssueType: "Sub-task"},
		{Key: "PROJ-4", Summary: "Child Three", Status: "In Progress", IssueType: "Sub-task"},
	})

	content := m.renderContent()
	if !strings.Contains(content, "Child Issues (3)") {
		t.Error("expected 'Child Issues (3)' section header")
	}
	if !strings.Contains(content, "PROJ-2") {
		t.Error("expected first child key")
	}
	if !strings.Contains(content, "Child One") {
		t.Error("expected first child summary")
	}
	if !strings.Contains(content, "PROJ-3") {
		t.Error("expected second child key")
	}
	// Check category sub-headers.
	if !strings.Contains(content, "To Do (1)") {
		t.Error("expected 'To Do (1)' category header")
	}
	if !strings.Contains(content, "In Progress (1)") {
		t.Error("expected 'In Progress (1)' category header")
	}
	if !strings.Contains(content, "Done (1)") {
		t.Error("expected 'Done (1)' category header")
	}
	// Check progress line.
	if !strings.Contains(content, "1/3 done") {
		t.Error("expected progress summary '1/3 done'")
	}
}

func TestSetChildren_GroupsCorrectly(t *testing.T) {
	m := New()
	m = m.SetSize(80, 30)

	iss := jira.Issue{Key: "PROJ-1", Summary: "Parent", Status: "In Progress"}
	m = m.SetIssue(iss)
	m = m.SetChildren([]jira.ChildIssue{
		{Key: "PROJ-2", Summary: "A", Status: "Done"},
		{Key: "PROJ-3", Summary: "B", Status: "Done"},
		{Key: "PROJ-4", Summary: "C", Status: "To Do"},
	})

	content := m.renderContent()
	if !strings.Contains(content, "Done (2)") {
		t.Error("expected 'Done (2)' category header")
	}
	if !strings.Contains(content, "To Do (1)") {
		t.Error("expected 'To Do (1)' category header")
	}
	// In Progress should not appear when empty.
	if strings.Contains(content, "In Progress (0)") {
		t.Error("expected no 'In Progress' category when empty")
	}
	if !strings.Contains(content, "2/3 done") {
		t.Error("expected progress summary '2/3 done'")
	}
}

func TestSetChildren_EmptyNoSection(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)

	iss := jira.Issue{
		Key:     "PROJ-1",
		Summary: "No children",
		Status:  "To Do",
	}
	m = m.SetIssue(iss)
	m = m.SetChildren(nil)

	content := m.renderContent()
	if strings.Contains(content, "Child Issues") {
		t.Error("expected no Child Issues section when children are nil")
	}
}

func TestSetIssue_ResetsChildren(t *testing.T) {
	m := New()
	m = m.SetSize(80, 30)

	iss := jira.Issue{Key: "PROJ-1", Summary: "First", Status: "To Do"}
	m = m.SetIssue(iss)
	m = m.SetChildren([]jira.ChildIssue{
		{Key: "PROJ-2", Summary: "Child", Status: "To Do"},
	})

	// Setting a new issue should clear children.
	iss2 := jira.Issue{Key: "PROJ-99", Summary: "Second", Status: "To Do"}
	m = m.SetIssue(iss2)

	content := m.renderContent()
	if strings.Contains(content, "Child Issues") {
		t.Error("expected children to be cleared after SetIssue")
	}
}

func TestHasParent(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)

	// No issue → no parent.
	if m.HasParent() {
		t.Error("expected no parent when no issue set")
	}

	// Issue without parent.
	m = m.SetIssue(jira.Issue{Key: "PROJ-1", Summary: "No parent", Status: "To Do"})
	if m.HasParent() {
		t.Error("expected no parent when ParentKey is empty")
	}

	// Issue with parent.
	m = m.SetIssue(jira.Issue{Key: "PROJ-2", Summary: "Child", Status: "To Do", ParentKey: "PROJ-1"})
	if !m.HasParent() {
		t.Error("expected parent when ParentKey is set")
	}
}

func TestIssueKeys_ParentChildrenDescriptionComments(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)

	iss := jira.Issue{
		Key:           "PROJ-5",
		Summary:       "Main issue",
		Status:        "To Do",
		ParentKey:     "PROJ-1",
		ParentSummary: "Parent Issue",
		ParentType:    "Epic",
		Description:   "See PROJ-10 and PROJ-11 for details",
		Comments: []jira.Comment{
			{Author: "Alice", Body: "Related to PROJ-20"},
		},
	}
	m = m.SetIssue(iss)
	m = m.SetChildren([]jira.ChildIssue{
		{Key: "PROJ-6", Summary: "Child One", Status: "To Do"},
		{Key: "PROJ-7", Summary: "Child Two", Status: "Done"},
	})

	refs := m.IssueKeys()

	// Should have: parent (PROJ-1), children (PROJ-6, PROJ-7), description (PROJ-10, PROJ-11), comment (PROJ-20).
	keys := make(map[string]string)
	for _, r := range refs {
		keys[r.Key] = r.Label
	}

	if _, ok := keys["PROJ-1"]; !ok {
		t.Error("expected parent key PROJ-1")
	}
	if _, ok := keys["PROJ-6"]; !ok {
		t.Error("expected child key PROJ-6")
	}
	if _, ok := keys["PROJ-7"]; !ok {
		t.Error("expected child key PROJ-7")
	}
	if _, ok := keys["PROJ-10"]; !ok {
		t.Error("expected description key PROJ-10")
	}
	if _, ok := keys["PROJ-11"]; !ok {
		t.Error("expected description key PROJ-11")
	}
	if _, ok := keys["PROJ-20"]; !ok {
		t.Error("expected comment key PROJ-20")
	}
	if _, ok := keys["PROJ-5"]; ok {
		t.Error("expected current issue key PROJ-5 to be excluded")
	}
	if len(refs) != 6 {
		t.Errorf("expected 6 refs, got %d", len(refs))
	}
}

func TestIssueKeys_Deduplication(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)

	// PROJ-6 appears as both a child and in description — should only appear once.
	iss := jira.Issue{
		Key:         "PROJ-5",
		Summary:     "Main",
		Status:      "To Do",
		Description: "See PROJ-6 for details",
	}
	m = m.SetIssue(iss)
	m = m.SetChildren([]jira.ChildIssue{
		{Key: "PROJ-6", Summary: "Child", Status: "To Do"},
	})

	refs := m.IssueKeys()
	count := 0
	for _, r := range refs {
		if r.Key == "PROJ-6" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected PROJ-6 once (deduped), got %d times", count)
	}
}

func TestIssueKeys_NoIssue(t *testing.T) {
	m := New()
	refs := m.IssueKeys()
	if len(refs) != 0 {
		t.Errorf("expected no refs when no issue set, got %d", len(refs))
	}
}

func TestIssueKeys_Labels(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)

	iss := jira.Issue{
		Key:           "PROJ-5",
		Summary:       "Main",
		Status:        "To Do",
		ParentKey:     "PROJ-1",
		ParentSummary: "Epic",
		ParentType:    "Epic",
	}
	m = m.SetIssue(iss)

	refs := m.IssueKeys()
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref, got %d", len(refs))
	}
	if !strings.Contains(refs[0].Label, "parent") {
		t.Errorf("expected parent label, got %q", refs[0].Label)
	}
	if !strings.Contains(refs[0].Label, "Epic") {
		t.Errorf("expected parent summary in label, got %q", refs[0].Label)
	}
}

func TestIssueKeys_ParentKeyOnly_NoSummary(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)
	m = m.SetIssue(jira.Issue{Key: "PROJ-2", Summary: "Child", Status: "To Do", ParentKey: "PROJ-1"})

	refs := m.IssueKeys()
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref, got %d", len(refs))
	}
	if refs[0].Label != "parent" {
		t.Errorf("expected plain 'parent' label, got %q", refs[0].Label)
	}
}

func TestUpdate_OpenKeySetsSentinel(t *testing.T) {
	m := New()
	m.SetIssueURL("https://example.com/browse/PROJ-1")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	_, ok := m.OpenURL()
	if !ok {
		t.Error("expected OpenURL sentinel after 'o' key")
	}
}

func TestUpdate_CopyKeySetsSentinel(t *testing.T) {
	m := New()
	m.SetIssueURL("https://example.com/browse/PROJ-1")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	_, ok := m.CopyURL()
	if !ok {
		t.Error("expected CopyURL sentinel after 'x' key")
	}
}

func TestUpdate_OpenKeyIgnoredWhenNoURL(t *testing.T) {
	m := New()
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	_, ok := m.OpenURL()
	if ok {
		t.Error("expected no sentinel when issueURL is empty")
	}
}

func TestUpdate_GotoTop(t *testing.T) {
	m := New()
	m = m.SetSize(80, 10)

	// Create an issue with enough content to scroll.
	iss := jira.Issue{
		Key:         "PROJ-1",
		Summary:     "Test",
		Status:      "To Do",
		Description: strings.Repeat("line\n", 50),
	}
	m = m.SetIssue(iss)

	// Scroll down first.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	if m.viewport.ScrollPercent() < 0.9 {
		t.Fatalf("expected near bottom after G, got %.0f%%", m.viewport.ScrollPercent()*100)
	}

	// Press g to go to top.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	if m.viewport.ScrollPercent() != 0 {
		t.Errorf("expected top after g, got %.0f%%", m.viewport.ScrollPercent()*100)
	}
}

func TestUpdate_GotoBottom(t *testing.T) {
	m := New()
	m = m.SetSize(80, 10)

	iss := jira.Issue{
		Key:         "PROJ-1",
		Summary:     "Test",
		Status:      "To Do",
		Description: strings.Repeat("line\n", 50),
	}
	m = m.SetIssue(iss)

	// Should start at top.
	if m.viewport.ScrollPercent() != 0 {
		t.Fatalf("expected at top initially, got %.0f%%", m.viewport.ScrollPercent()*100)
	}

	// Press G to go to bottom.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	if m.viewport.ScrollPercent() < 0.9 {
		t.Errorf("expected near bottom after G, got %.0f%%", m.viewport.ScrollPercent()*100)
	}
}

func TestCopyURL_SentinelReset(t *testing.T) {
	m := New()
	m.SetIssueURL("https://jira.example.com/browse/PROJ-1")

	// No copy requested yet.
	_, ok := m.CopyURL()
	if ok {
		t.Error("expected no URL before request")
	}

	// Simulate 'x' key press.
	m.copyURL = true
	url, ok := m.CopyURL()
	if !ok {
		t.Fatal("expected URL after request")
	}
	if url != "https://jira.example.com/browse/PROJ-1" {
		t.Errorf("expected URL, got %q", url)
	}

	// Should reset.
	_, ok = m.CopyURL()
	if ok {
		t.Error("expected reset after read")
	}
}

func TestCopyURL_EmptyURL(t *testing.T) {
	m := New()
	m.copyURL = true

	_, ok := m.CopyURL()
	if ok {
		t.Error("expected no URL when issueURL is empty")
	}
}

func TestSetBranches_ShowsInContent(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)

	iss := jira.Issue{Key: "PROJ-1", Summary: "Test", Status: "To Do"}
	m = m.SetIssue(iss)
	m = m.SetBranches([]jira.BranchInfo{
		{Name: "origin/feature/PROJ-1-fix-login", RemoteCommit: 3},
		{Name: "origin/PROJ-1-hotfix", RemoteCommit: 0},
	})

	content := m.renderContent()
	if !strings.Contains(content, "Branch:") {
		t.Error("expected Branch field in metadata")
	}
	if !strings.Contains(content, "origin/feature/PROJ-1-fix-login") {
		t.Error("expected first branch name")
	}
	if !strings.Contains(content, "3 commits on remote") {
		t.Error("expected commit count for first branch")
	}
	if !strings.Contains(content, "no commits on remote") {
		t.Error("expected 'no commits on remote' for second branch")
	}
}

func TestSetBranches_SingleCommitGrammar(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)

	m = m.SetIssue(jira.Issue{Key: "PROJ-1", Summary: "Test", Status: "To Do"})
	m = m.SetBranches([]jira.BranchInfo{
		{Name: "origin/PROJ-1-fix", RemoteCommit: 1},
	})

	content := m.renderContent()
	if !strings.Contains(content, "1 commit on remote") {
		t.Error("expected singular 'commit'")
	}
	if strings.Contains(content, "1 commits") {
		t.Error("expected singular, not plural")
	}
}

func TestSetIssue_ResetsBranches(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)

	m = m.SetIssue(jira.Issue{Key: "PROJ-1", Summary: "First", Status: "To Do"})
	m = m.SetBranches([]jira.BranchInfo{
		{Name: "origin/PROJ-1-branch", RemoteCommit: 2},
	})

	// Setting a new issue should clear branches.
	m = m.SetIssue(jira.Issue{Key: "PROJ-2", Summary: "Second", Status: "To Do"})

	content := m.renderContent()
	if strings.Contains(content, "Branch:") {
		t.Error("expected branches to be cleared after SetIssue")
	}
}

func TestUpdateIssue_PreservesBranchesAndChildren(t *testing.T) {
	m := New()
	m = m.SetSize(80, 30)

	m = m.SetIssue(jira.Issue{Key: "PROJ-1", Summary: "Original", Status: "To Do"})
	m = m.SetChildren([]jira.ChildIssue{
		{Key: "PROJ-2", Summary: "Child", Status: "To Do"},
	})
	m = m.SetBranches([]jira.BranchInfo{
		{Name: "origin/PROJ-1-branch", RemoteCommit: 2},
	})

	// UpdateIssue should preserve children and branches.
	m = m.UpdateIssue(jira.Issue{Key: "PROJ-1", Summary: "Updated", Status: "In Progress"})

	// Summary and status are both in the header.
	view := m.View()
	if !strings.Contains(view, "Updated") {
		t.Error("expected updated summary in header")
	}
	if !strings.Contains(view, "In Progress") {
		t.Error("expected updated status in header")
	}
	content := m.renderContent()
	if !strings.Contains(content, "Child Issues") {
		t.Error("expected children preserved after UpdateIssue")
	}
	if !strings.Contains(content, "origin/PROJ-1-branch") {
		t.Error("expected branches preserved after UpdateIssue")
	}
}

func TestSetBranches_NoBranches_NoBranchField(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)

	m = m.SetIssue(jira.Issue{Key: "PROJ-1", Summary: "Test", Status: "To Do"})
	m = m.SetBranches(nil)

	content := m.renderContent()
	if strings.Contains(content, "Branch:") {
		t.Error("expected no Branch field when branches is nil")
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

func TestIssueKeys_GroupField(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)
	m = m.SetIssue(jira.Issue{
		Key: "PROJ-5", Summary: "Main", Status: "To Do",
		ParentKey: "PROJ-1", ParentSummary: "Parent Issue",
	})
	m = m.SetChildren([]jira.ChildIssue{
		{Key: "PROJ-10", Summary: "Todo child", Status: "To Do"},
		{Key: "PROJ-11", Summary: "IP child", Status: "In Progress"},
		{Key: "PROJ-12", Summary: "Done child", Status: "Done"},
	})

	refs := m.IssueKeys()
	groups := map[string]string{}
	for _, r := range refs {
		groups[r.Key] = r.Group
	}

	if groups["PROJ-1"] != "Parent" {
		t.Errorf("expected Group 'Parent' for parent, got %q", groups["PROJ-1"])
	}
	if groups["PROJ-10"] != "To Do (1)" {
		t.Errorf("expected Group 'To Do (1)', got %q", groups["PROJ-10"])
	}
	if groups["PROJ-11"] != "In Progress (1)" {
		t.Errorf("expected Group 'In Progress (1)', got %q", groups["PROJ-11"])
	}
	if groups["PROJ-12"] != "Done (1)" {
		t.Errorf("expected Group 'Done (1)', got %q", groups["PROJ-12"])
	}
}

func TestProgressBar_SmallSegmentGetsMinimumBar(t *testing.T) {
	m := New()
	m = m.SetSize(80, 30)
	m = m.SetIssue(jira.Issue{Key: "PROJ-1", Summary: "Parent", Status: "Open"})

	// 1 in-progress out of 20 — would round to 0 bars without the guard.
	children := make([]jira.ChildIssue, 20)
	children[0] = jira.ChildIssue{Key: "PROJ-2", Summary: "IP", Status: "In Progress"}
	for i := 1; i < 20; i++ {
		children[i] = jira.ChildIssue{
			Key:     fmt.Sprintf("PROJ-%d", i+2),
			Summary: "done",
			Status:  "Done",
		}
	}
	m = m.SetChildren(children)

	content := m.View()
	if !strings.Contains(content, "19/20 done") {
		t.Error("expected '19/20 done' in progress bar")
	}
}

// --- Remote links tests ---

func TestSetRemoteLinks_RendersLinkedPages(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)
	m = m.SetIssue(jira.Issue{Key: "PROJ-1", Summary: "Test", Status: "Open"})
	m = m.SetRemoteLinks([]jira.RemoteLink{
		{ID: 1, Title: "Design Spec", URL: "https://x.atlassian.net/wiki/spaces/ENG/pages/111"},
		{ID: 2, Title: "API Docs", URL: "https://x.atlassian.net/wiki/spaces/ENG/pages/222"},
	})

	content := m.renderContent()
	if !strings.Contains(content, "Linked Pages (2)") {
		t.Error("expected 'Linked Pages (2)' section header")
	}
	if !strings.Contains(content, "Design Spec") {
		t.Error("expected 'Design Spec' link title")
	}
	if !strings.Contains(content, "API Docs") {
		t.Error("expected 'API Docs' link title")
	}
}

func TestSetRemoteLinks_EmptyNoSection(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)
	m = m.SetIssue(jira.Issue{Key: "PROJ-1", Summary: "Test", Status: "Open"})
	m = m.SetRemoteLinks(nil)

	content := m.renderContent()
	if strings.Contains(content, "Linked Pages") {
		t.Error("expected no 'Linked Pages' section when links are empty")
	}
}

func TestSetRemoteLinks_FallbackToURL(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)
	m = m.SetIssue(jira.Issue{Key: "PROJ-1", Summary: "Test", Status: "Open"})
	m = m.SetRemoteLinks([]jira.RemoteLink{
		{ID: 1, Title: "", URL: "https://x.atlassian.net/some/page"},
	})

	content := m.renderContent()
	if !strings.Contains(content, "https://x.atlassian.net/some/page") {
		t.Error("expected URL as fallback when title is empty")
	}
}

func TestIssueKeys_IncludesRemoteLinks(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)
	m = m.SetIssue(jira.Issue{Key: "PROJ-1", Summary: "Test", Status: "Open"})
	m = m.SetRemoteLinks([]jira.RemoteLink{
		{ID: 1, Title: "Design Spec", URL: "https://x.atlassian.net/wiki/spaces/ENG/pages/555"},
	})

	refs := m.IssueKeys()
	var found bool
	for _, r := range refs {
		if r.Key == "555" && r.Group == "Linked Pages" && r.Display == "Design Spec" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected remote link Confluence page ref with key=555 in IssueKeys")
	}
}

func TestIssueKeys_RemoteLinksDeduped(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)
	// Description contains the same page URL as the remote link.
	m = m.SetIssue(jira.Issue{
		Key:         "PROJ-1",
		Summary:     "Test",
		Status:      "Open",
		Description: "See https://x.atlassian.net/wiki/spaces/ENG/pages/555 for details",
	})
	m = m.SetRemoteLinks([]jira.RemoteLink{
		{ID: 1, Title: "Design Spec", URL: "https://x.atlassian.net/wiki/spaces/ENG/pages/555"},
	})

	refs := m.IssueKeys()
	count := 0
	for _, r := range refs {
		if r.Key == "555" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected page 555 once (deduped across description and remote links), got %d", count)
	}
}

func TestIssueKeys_RemoteLinksNonConfluenceIgnored(t *testing.T) {
	m := New()
	m = m.SetSize(80, 24)
	m = m.SetIssue(jira.Issue{Key: "PROJ-1", Summary: "Test", Status: "Open"})
	m = m.SetRemoteLinks([]jira.RemoteLink{
		{ID: 1, Title: "External Site", URL: "https://example.com/docs"},
	})

	refs := m.IssueKeys()
	for _, r := range refs {
		if r.Group == "Linked Pages" {
			t.Error("expected non-Confluence remote links to be excluded from IssueKeys")
		}
	}
}
