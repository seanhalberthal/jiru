package ui

import (
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"al.essio.dev/pkg/shellescape"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/seanhalberthal/jiru/internal/client"
	"github.com/seanhalberthal/jiru/internal/config"
	"github.com/seanhalberthal/jiru/internal/confluence"
	"github.com/seanhalberthal/jiru/internal/filters"
	"github.com/seanhalberthal/jiru/internal/jira"
	"github.com/seanhalberthal/jiru/internal/ui/assignpickview"
	"github.com/seanhalberthal/jiru/internal/ui/branchview"
	"github.com/seanhalberthal/jiru/internal/ui/deleteview"
	"github.com/seanhalberthal/jiru/internal/ui/linkpickview"
)

// Commands — tea.Cmd factories for async operations.

func (a App) verifyAuth() tea.Cmd {
	return func() tea.Msg {
		name, err := a.client.Me()
		if err != nil {
			return ErrMsg{Err: err}
		}
		return ClientReadyMsg{Client: a.client, DisplayName: name}
	}
}

func (a App) fetchSprintIssues(sprintID int, sprintName string) tea.Cmd {
	seq := a.paginationSeq
	return func() tea.Msg {
		// Use v3 JQL search instead of the Agile v1 sprint endpoint.
		// The Agile v1 API has an undocumented truncation limit on Jira Cloud
		// (~1000 issues) where it returns empty pages despite reporting more.
		// The v3 /search/jql endpoint uses cursor-based pagination and does
		// not suffer from this limitation.
		jql := fmt.Sprintf("sprint = %d ORDER BY updated DESC", sprintID)
		page, err := a.client.SearchJQLPage(jql, client.DefaultPageSize, 0, "")
		if err != nil {
			return ErrMsg{Err: err}
		}

		// Resolve parents for the first page.
		parents := a.client.ResolveParents(page.Issues)
		enriched := client.EnrichWithParents(page.Issues, parents)

		return IssuesLoadedMsg{
			Issues:     enriched,
			Title:      sprintName,
			HasMore:    page.HasMore,
			Source:     "sprint",
			From:       len(page.Issues),
			SprintID:   sprintID,
			SprintName: sprintName,
			JQL:        jql,
			NextToken:  page.NextToken,
			Seq:        seq,
		}
	}
}

// pagesPerBatch is how many API pages to fetch before updating the UI.
// Jira caps each page at 100, so 2 pages ≈ 200 issues per visible update.
const pagesPerBatch = 2

func (a App) fetchMoreIssues(msg IssuesPageMsg) tea.Cmd {
	return func() tea.Msg {
		var allIssues []jira.Issue
		from := msg.From
		nextToken := msg.NextToken
		hasMore := true
		source := msg.Source
		jql := msg.JQL

		for range pagesPerBatch {
			var page *client.PageResult
			var err error

			switch source {
			case "sprint", "epic", "board", "search":
				// All issue loading uses v3 JQL cursor-based search.
				// Sprint and epic previously used Agile v1 endpoints, but
				// those have an undocumented truncation limit on Jira Cloud
				// (~1000 issues). The v3 /search/jql endpoint does not.
				page, err = a.client.SearchJQLPage(jql, client.DefaultPageSize, from, nextToken)
			case "boardapi":
				page, err = a.client.BoardIssuesPage(msg.SprintID, from, client.DefaultPageSize)
			}

			if err != nil {
				return ErrMsg{Err: err}
			}

			allIssues = append(allIssues, page.Issues...)
			from += len(page.Issues)

			// Detect cursor loop — Jira Cloud has a known bug where
			// nextPageToken can repeat, returning the same page forever.
			if page.NextToken == nextToken && nextToken != "" {
				hasMore = false
				break
			}
			nextToken = page.NextToken
			hasMore = page.HasMore && from < client.MaxTotalIssues

			if !hasMore || len(page.Issues) == 0 {
				break
			}
		}

		// Resolve parents for the whole batch.
		parents := a.client.ResolveParents(allIssues)
		enriched := client.EnrichWithParents(allIssues, parents)

		return IssuesPageMsg{
			Issues:     enriched,
			HasMore:    hasMore,
			Source:     source,
			From:       from,
			SprintID:   msg.SprintID,
			SprintName: msg.SprintName,
			EpicKey:    msg.EpicKey,
			JQL:        jql,
			Project:    msg.Project,
			Seq:        msg.Seq,
			NextToken:  nextToken,
		}
	}
}

func (a App) fetchIssueDetail(key string) tea.Cmd {
	return func() tea.Msg {
		issue, err := a.client.GetIssue(key)
		if err != nil {
			return ErrMsg{Err: err}
		}
		return IssueDetailMsg{Issue: issue}
	}
}

func (a App) fetchChildIssues(key string) tea.Cmd {
	return func() tea.Msg {
		children, err := a.client.ChildIssues(key)
		if err != nil {
			// Non-fatal: just return empty children.
			return ChildIssuesMsg{ParentKey: key}
		}
		return ChildIssuesMsg{ParentKey: key, Children: children}
	}
}

func (a App) fetchBoardsForPicker() tea.Cmd {
	return func() tea.Msg {
		project := a.client.Config().Project
		boards, err := a.client.Boards(project)
		if err != nil {
			return ErrMsg{Err: err}
		}
		return BoardPickLoadedMsg{Boards: boards}
	}
}

func (a App) fetchJQLMetadata() tea.Cmd {
	return func() tea.Msg {
		meta, err := a.client.JQLMetadata()
		if err != nil {
			// Silently degrade — static completions still work.
			return JQLMetadataMsg{Meta: nil}
		}
		return JQLMetadataMsg{Meta: meta}
	}
}

func (a App) searchUsers(prefix string) tea.Cmd {
	return func() tea.Msg {
		users, err := a.client.SearchUsers(a.client.Config().Project, prefix)
		if err != nil {
			return UserSearchMsg{Prefix: prefix, Names: nil}
		}
		names := make([]string, len(users))
		for i, u := range users {
			names[i] = u.DisplayName
		}
		return UserSearchMsg{Prefix: prefix, Names: names}
	}
}

func (a App) searchJQL(jql string) tea.Cmd {
	seq := a.paginationSeq
	return func() tea.Msg {
		page, err := a.client.SearchJQLPage(jql, client.DefaultPageSize, 0, "")
		if err != nil {
			return ErrMsg{Err: err}
		}
		return SearchResultsMsg{
			Issues:    page.Issues,
			Query:     jql,
			HasMore:   page.HasMore,
			From:      len(page.Issues),
			Seq:       seq,
			NextToken: page.NextToken,
		}
	}
}

func (a App) fetchTransitions(key string) tea.Cmd {
	return func() tea.Msg {
		transitions, err := a.client.Transitions(key)
		if err != nil {
			return ErrMsg{Err: err}
		}
		return TransitionsLoadedMsg{Key: key, Transitions: transitions}
	}
}

func (a App) transitionIssue(key, transitionID, targetStatus string) tea.Cmd {
	return func() tea.Msg {
		err := a.client.TransitionIssue(key, transitionID)
		if err != nil {
			return IssueTransitionedMsg{Key: key, Err: err}
		}
		return IssueTransitionedMsg{Key: key, NewStatus: targetStatus}
	}
}

func (a App) addComment(key, body string) tea.Cmd {
	return func() tea.Msg {
		err := a.client.AddComment(key, body)
		if err != nil {
			return CommentAddedMsg{Key: key, Err: err}
		}
		return CommentAddedMsg{Key: key}
	}
}

func (a App) toggleWatch(key string, currentlyWatching bool) tea.Cmd {
	return func() tea.Msg {
		var err error
		if currentlyWatching {
			err = a.client.UnwatchIssue(key)
		} else {
			err = a.client.WatchIssue(key)
		}
		return IssueWatchToggledMsg{Key: key, IsWatching: !currentlyWatching, Err: err}
	}
}

func (a App) searchUsersForAssign(prefix string) tea.Cmd {
	return func() tea.Msg {
		users, err := a.client.SearchUsers(a.client.Config().Project, prefix)
		if err != nil {
			return AssignUserSearchMsg{Users: nil}
		}
		return AssignUserSearchMsg{Users: users}
	}
}

func (a App) assignIssue(key string, req *assignpickview.AssignRequest) tea.Cmd {
	return func() tea.Msg {
		err := a.client.AssignIssue(key, req.AccountID)
		if err != nil {
			return IssueAssignedMsg{Key: key, Err: err}
		}
		return IssueAssignedMsg{Key: key, Assignee: req.DisplayName}
	}
}

func (a App) editIssue(key string, req *client.EditIssueRequest) tea.Cmd {
	return func() tea.Msg {
		err := a.client.EditIssue(key, req)
		if err != nil {
			return IssueEditedMsg{Key: key, Err: err}
		}
		return IssueEditedMsg{Key: key}
	}
}

func (a App) fetchLinkTypes() tea.Cmd {
	return func() tea.Msg {
		types, err := a.client.GetIssueLinkTypes()
		if err != nil {
			return ErrMsg{Err: err}
		}
		return LinkTypesLoadedMsg{Types: types}
	}
}

func (a App) linkIssue(req *linkpickview.LinkRequest) tea.Cmd {
	return func() tea.Msg {
		err := a.client.LinkIssue(req.InwardKey, req.OutwardKey, req.LinkType)
		if err != nil {
			return IssueLinkCreatedMsg{SourceKey: req.OutwardKey, TargetKey: req.InwardKey, Err: err}
		}
		return IssueLinkCreatedMsg{SourceKey: req.OutwardKey, TargetKey: req.InwardKey}
	}
}

func (a App) deleteIssue(req *deleteview.DeleteRequest) tea.Cmd {
	return func() tea.Msg {
		err := a.client.DeleteIssue(req.Key, req.Cascade)
		if err != nil {
			return IssueDeletedMsg{Key: req.Key, Err: err}
		}
		return IssueDeletedMsg{Key: req.Key}
	}
}

// fetchIssueBundle returns a batch of commands to fully load an issue:
// detail, children, branch info (if a repo path is configured), and remote links.
func (a App) fetchIssueBundle(key string) tea.Cmd {
	cmds := []tea.Cmd{a.fetchIssueDetail(key), a.fetchChildIssues(key), a.fetchRemoteLinks(key)}
	if branchCmd := a.fetchBranchInfo(key); branchCmd != nil {
		cmds = append(cmds, branchCmd)
	}
	return tea.Batch(cmds...)
}

func (a App) fetchRemoteLinks(key string) tea.Cmd {
	return func() tea.Msg {
		links, err := a.client.RemoteLinks(key)
		if err != nil {
			return RemoteLinksLoadedMsg{IssueKey: key}
		}
		return RemoteLinksLoadedMsg{Links: links, IssueKey: key}
	}
}

func (a App) fetchBranchInfo(issueKey string) tea.Cmd {
	repoPath := ""
	if a.client != nil {
		repoPath = a.client.Config().RepoPath
	}
	if repoPath == "" {
		return nil
	}
	return func() tea.Msg {
		// Find all remote branches containing the issue key (case-insensitive).
		out, err := exec.Command("git", "-C", repoPath, "branch", "-r", "--list",
			"*"+strings.ToLower(issueKey)+"*", "*"+strings.ToUpper(issueKey)+"*").CombinedOutput()
		if err != nil {
			return BranchInfoMsg{IssueKey: issueKey}
		}

		// Also check with original case.
		out2, err2 := exec.Command("git", "-C", repoPath, "branch", "-r", "--list",
			"*"+issueKey+"*").CombinedOutput()
		if err2 == nil {
			out = append(out, out2...)
		}

		// Deduplicate branch names.
		seen := make(map[string]bool)
		var branches []jira.BranchInfo
		for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
			name := strings.TrimSpace(line)
			if name == "" || seen[name] {
				continue
			}
			// Skip HEAD pointer refs (e.g., "origin/HEAD -> origin/main").
			if strings.Contains(name, "->") {
				continue
			}
			seen[name] = true

			// Count commits on this remote branch relative to the default branch.
			// Use rev-list to count commits that are on the branch but not on HEAD.
			countOut, countErr := exec.Command("git", "-C", repoPath,
				"rev-list", "--count", "--", "HEAD.."+name).CombinedOutput()
			commits := 0
			if countErr == nil {
				if n, parseErr := strconv.Atoi(strings.TrimSpace(string(countOut))); parseErr == nil {
					commits = n
				}
			}

			branches = append(branches, jira.BranchInfo{
				Name:         name,
				RemoteCommit: commits,
			})
		}

		return BranchInfoMsg{IssueKey: issueKey, Branches: branches}
	}
}

// refreshCurrentView returns a command to re-fetch the current sprint or board issues.
func (a App) refreshCurrentView() tea.Cmd {
	if a.boardID != 0 {
		return a.fetchActiveSprintForBoard(a.boardID)
	}
	return nil
}

func createBranch(req *branchview.BranchRequest) tea.Cmd {
	return func() tea.Msg {
		if req.RepoPath == "" {
			return clipboardBranch(req)
		}

		mode := req.Mode
		if mode == "" {
			mode = "local"
		}

		switch mode {
		case "remote":
			// Validate refspec components don't contain ':'.
			if strings.Contains(req.Name, ":") || strings.Contains(req.Base, ":") {
				return BranchCreatedMsg{Err: fmt.Errorf("branch name and base must not contain ':'")}
			}
			// Push to origin without local checkout.
			out, err := exec.Command("git", "-C", req.RepoPath,
				"push", "origin", req.Base+":refs/heads/"+req.Name).CombinedOutput()
			if err != nil {
				return BranchCreatedMsg{Err: fmt.Errorf("%s", strings.TrimSpace(string(out)))}
			}
			return BranchCreatedMsg{Name: req.Name, Mode: "remote"}

		case "both":
			// Create local branch. '--' prevents branch names from being interpreted as flags.
			out, err := exec.Command("git", "-C", req.RepoPath,
				"checkout", "-b", "--", req.Name, req.Base).CombinedOutput()
			if err != nil {
				return BranchCreatedMsg{Err: fmt.Errorf("%s", strings.TrimSpace(string(out)))}
			}
			// Push to origin with tracking.
			out, err = exec.Command("git", "-C", req.RepoPath,
				"push", "-u", "origin", req.Name).CombinedOutput()
			if err != nil {
				return BranchCreatedMsg{Err: fmt.Errorf("branch created locally but push failed: %s", strings.TrimSpace(string(out)))}
			}
			return BranchCreatedMsg{Name: req.Name, Mode: "both"}

		default: // "local"
			// '--' prevents branch names from being interpreted as flags.
			out, err := exec.Command("git", "-C", req.RepoPath,
				"checkout", "-b", "--", req.Name, req.Base).CombinedOutput()
			if err != nil {
				return BranchCreatedMsg{Err: fmt.Errorf("%s", strings.TrimSpace(string(out)))}
			}
			return BranchCreatedMsg{Name: req.Name, Mode: "local"}
		}
	}
}

func clipboardBranch(req *branchview.BranchRequest) BranchCreatedMsg {
	var text string
	switch req.Mode {
	case "remote":
		text = fmt.Sprintf("git push origin %s:refs/heads/%s",
			shellescape.Quote(req.Base), shellescape.Quote(req.Name))
	case "both":
		text = fmt.Sprintf("git checkout -b %s %s && git push -u origin %s",
			shellescape.Quote(req.Name), shellescape.Quote(req.Base), shellescape.Quote(req.Name))
	default:
		text = fmt.Sprintf("git checkout -b %s %s",
			shellescape.Quote(req.Name), shellescape.Quote(req.Base))
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		cmd = exec.Command("xclip", "-selection", "clipboard")
	default:
		return BranchCreatedMsg{Err: fmt.Errorf("clipboard not supported on %s", runtime.GOOS)}
	}
	cmd.Stdin = strings.NewReader(text)
	if err := cmd.Run(); err != nil {
		return BranchCreatedMsg{Err: fmt.Errorf("clipboard copy failed: %w", err)}
	}
	return BranchCreatedMsg{Name: req.Name, Copied: true}
}

func (a App) fetchActiveSprintForBoard(boardID int) tea.Cmd {
	seq := a.paginationSeq
	return func() tea.Msg {
		sprints, err := a.client.BoardSprints(boardID, "active")
		if err == nil && len(sprints) > 0 {
			// Scrum board with active sprint — existing path.
			return SprintLoadedMsg{Sprint: &sprints[0]}
		}

		// No active sprint — fetch board issues via v3 JQL search using
		// the board's filter. The Agile v1 /board/{id}/issue endpoint has
		// an undocumented truncation limit on Jira Cloud (~1000 issues)
		// where it returns empty pages despite reporting more results.
		jql, filterErr := a.client.BoardFilterJQL(boardID)
		if filterErr == nil && jql != "" {
			// Replace any existing ORDER BY with updated DESC so the most
			// recently edited issues load first during progressive pagination.
			if idx := strings.Index(strings.ToUpper(jql), "ORDER BY"); idx >= 0 {
				jql = strings.TrimSpace(jql[:idx])
			}
			jql += " ORDER BY updated DESC"

			page, searchErr := a.client.SearchJQLPage(jql, client.DefaultPageSize, 0, "")
			if searchErr != nil {
				return ErrMsg{Err: searchErr}
			}

			parents := a.client.ResolveParents(page.Issues)
			enriched := client.EnrichWithParents(page.Issues, parents)

			return IssuesLoadedMsg{
				Issues:    enriched,
				Title:     "Board",
				HasMore:   page.HasMore,
				Source:    "board",
				From:      len(page.Issues),
				SprintID:  boardID,
				JQL:       jql,
				NextToken: page.NextToken,
				Seq:       seq,
			}
		}

		// Fallback: board filter unavailable — use Agile v1 directly.
		page, fetchErr := a.client.BoardIssuesPage(boardID, 0, client.DefaultPageSize)
		if fetchErr != nil {
			return ErrMsg{Err: fmt.Errorf("no active iteration and board issue fetch failed: %w", fetchErr)}
		}

		parents := a.client.ResolveParents(page.Issues)
		enriched := client.EnrichWithParents(page.Issues, parents)

		return IssuesLoadedMsg{
			Issues:   enriched,
			Title:    "Board",
			HasMore:  page.HasMore,
			Source:   "boardapi",
			From:     len(page.Issues),
			SprintID: boardID,
			Seq:      seq,
		}
	}
}

func (a App) switchProfile(name string) tea.Cmd {
	return func() tea.Msg {
		if err := config.SwitchProfile(name); err != nil {
			return ErrMsg{Err: err}
		}
		cfg, err := config.LoadProfile(name)
		if err != nil {
			return ErrMsg{Err: err}
		}
		c := client.New(cfg)
		return ProfileSwitchedMsg{Client: c, Config: cfg, Name: name}
	}
}

// --- Confluence commands ---

func (a App) fetchConfluenceSpaces() tea.Cmd {
	return func() tea.Msg {
		spaces, err := a.client.ConfluenceSpaces()
		if err != nil {
			return ErrMsg{Err: err}
		}
		return SpacesLoadedMsg{Spaces: spaces}
	}
}

func (a App) fetchSpacePages(spaceID string) tea.Cmd {
	return func() tea.Msg {
		pages, err := a.client.ConfluenceSpacePages(spaceID, 50)
		if err != nil {
			return ErrMsg{Err: err}
		}
		return SpacePagesLoadedMsg{Pages: pages, SpaceID: spaceID}
	}
}

func (a App) fetchConfluencePage(pageID string) tea.Cmd {
	spaces := a.cachedSpaces // Capture for closure.
	return func() tea.Msg {
		page, err := a.client.ConfluencePage(pageID)
		if err != nil {
			return ErrMsg{Err: err}
		}
		ancestors, _ := a.client.ConfluencePageAncestors(pageID)
		// Resolve space key from cached spaces.
		spaceKey := resolveSpaceKey(page.SpaceID, spaces)
		// Resolve author account ID to display name.
		if page.Author != "" {
			page.Author = a.client.GetUserDisplayName(page.Author)
		}
		return ConfluencePageLoadedMsg{
			Page:      page,
			Ancestors: ancestors,
			SpaceKey:  spaceKey,
		}
	}
}

func (a App) fetchConfluenceComments(pageID string) tea.Cmd {
	return func() tea.Msg {
		comments, err := a.client.ConfluencePageComments(pageID)
		if err != nil {
			// Non-fatal — page still displays without comments.
			return ConfluenceCommentsLoadedMsg{PageID: pageID}
		}
		// Resolve author account IDs to display names.
		for i := range comments {
			if comments[i].Author != "" {
				comments[i].Author = a.client.GetUserDisplayName(comments[i].Author)
			}
		}
		return ConfluenceCommentsLoadedMsg{PageID: pageID, Comments: comments}
	}
}

func resolveSpaceKey(spaceID string, spaces []confluence.Space) string {
	for _, s := range spaces {
		if s.ID == spaceID {
			return s.Key
		}
	}
	return ""
}

// --- Utility functions ---

var urlPattern = regexp.MustCompile(`https?://\S+`)

// sanitiseError strips URL-like content from error messages to prevent
// leaking API endpoints, tokens, or internal server details to the terminal.
func sanitiseError(err error) error {
	msg := err.Error()
	clean := urlPattern.ReplaceAllString(msg, "[url redacted]")
	return fmt.Errorf("%s", clean)
}

// isPageID returns true if s looks like a Confluence page ID (all digits).
func isPageID(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func isHTTPS(url string) bool {
	return strings.HasPrefix(url, "https://")
}

func openBrowser(url string) {
	if !isHTTPS(url) {
		return
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return
	}
	_ = cmd.Start()
}

func copyToClipboard(text string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		cmd = exec.Command("xclip", "-selection", "clipboard")
	default:
		return fmt.Errorf("clipboard not supported on %s", runtime.GOOS)
	}
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

// reloadSavedFilters refreshes the in-memory filter list from disk.
func reloadSavedFilters(a *App) {
	if fs, err := filters.Load(); err == nil {
		a.savedFilters = filters.Sorted(fs)
	}
	a.filter = a.filter.SetFilters(a.savedFilters)
}

// searchBoardDisplayTitle returns the display title for the search board view.
// Uses the saved filter name when available, otherwise falls back to the raw JQL.
func (a *App) searchBoardDisplayTitle() string {
	if name := a.search.FilterName(); name != "" {
		return "Filter: " + name
	}
	return a.searchBoardTitle
}
