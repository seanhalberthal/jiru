package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/seanhalberthal/jiru/internal/adf"
	"github.com/seanhalberthal/jiru/internal/client"
	"github.com/seanhalberthal/jiru/internal/config"
	"github.com/seanhalberthal/jiru/internal/filters"
	"github.com/seanhalberthal/jiru/internal/jira"
	"github.com/seanhalberthal/jiru/internal/ui/assignpickview"
	"github.com/seanhalberthal/jiru/internal/ui/boardpickview"
	"github.com/seanhalberthal/jiru/internal/ui/branchview"
	"github.com/seanhalberthal/jiru/internal/ui/commentview"
	"github.com/seanhalberthal/jiru/internal/ui/createview"
	"github.com/seanhalberthal/jiru/internal/ui/deleteview"
	"github.com/seanhalberthal/jiru/internal/ui/editview"
	"github.com/seanhalberthal/jiru/internal/ui/helpview"
	"github.com/seanhalberthal/jiru/internal/ui/issuepickview"
	"github.com/seanhalberthal/jiru/internal/ui/issueview"
	"github.com/seanhalberthal/jiru/internal/ui/linkpickview"
	"github.com/seanhalberthal/jiru/internal/ui/profilepickview"
	"github.com/seanhalberthal/jiru/internal/ui/setupview"
	"github.com/seanhalberthal/jiru/internal/ui/transitionpickview"
	"github.com/seanhalberthal/jiru/internal/ui/wikiview"
)

// handleKeyMsg processes keyboard input for navigation and global key bindings.
// Returns (updatedApp, cmd, handled). When handled is false, the key was not
// consumed by navigation and should be delegated to the active child view.
func (a App) handleKeyMsg(msg tea.KeyMsg) (App, tea.Cmd, bool) {
	// Clear status on esc (explicit dismiss).
	if msg.String() == "esc" {
		a.statusMsg = ""
		a.statusIsError = false
	}

	// ctrl+c always quits, regardless of input state.
	if msg.String() == "ctrl+c" {
		return a, tea.Quit, true
	}

	// Quit confirmation: confirm on esc/q/enter/y, dismiss on anything else.
	if a.confirmQuit {
		a.confirmQuit = false
		k := msg.String()
		if k == "esc" || k == "q" || k == "enter" || k == "y" {
			return a, tea.Quit, true
		}
		return a, nil, true
	}

	// Handle error overlay: esc/q dismiss, r retries or dismisses.
	if a.err != nil {
		if a.isBackKey(msg) {
			a.err = nil
			a.retryCmd = nil
			// If stuck at loading (nothing will re-trigger), navigate back.
			if a.active == viewLoading {
				updated, cmd := a.navigateBack()
				return updated, cmd, true
			}
			return a, nil, true
		}
		if msg.String() == "r" {
			if a.retryCmd != nil {
				retry := a.retryCmd
				a.err = nil
				a.retryCmd = nil
				return a, retry, true
			}
			// No retry command — dismiss error and fall through to normal
			// key handling so 'r' can reach the refresh handler.
			a.err = nil
		} else {
			// Swallow all other keys while error is showing.
			return a, nil, true
		}
	}

	// Setup wizard handles all its own keys (esc, enter, ctrl+b).
	if a.active == viewSetup {
		return a, nil, false
	}

	// When text input is active (search overlay or list filter),
	// let the child view handle everything else.
	if a.inputActive() {
		return a, nil, false
	}

	// esc, q, and h/H all navigate back one level (or quit at the top).
	if a.isBackKey(msg) {
		updated, cmd := a.navigateBack()
		return updated, cmd, true
	}

	switch {
	case key.Matches(msg, a.keys.HomeTab) && (a.active == viewSprint || a.active == viewBoard):
		a.tabOrigin = a.active
		a.active = viewSpaces
		if !a.spacesLoaded && a.client != nil {
			a.spacesLoaded = true
			return a, a.fetchConfluenceSpaces(), true
		}
		return a, nil, true

	case key.Matches(msg, a.keys.HomeTab) && a.active == viewSpaces:
		a.active = a.tabOrigin
		return a, nil, true

	case key.Matches(msg, a.keys.Help) && a.active != viewLoading && a.active != viewHelp:
		a.help = helpview.New()
		a.help.SetSize(a.width, a.height)
		a.helpOrigin = a.active
		a.active = viewHelp
		return a, nil, true

	case key.Matches(msg, a.keys.Home) && a.active != viewSprint && a.active != viewLoading && a.active != viewSetup:
		a.active = viewSprint
		a.issueStack = nil
		a.pageStack = nil
		return a, nil, true

	case key.Matches(msg, a.keys.Search) && !a.search.Visible() && a.active != viewLoading && a.active != viewSpaces && a.active != viewConfluence:
		a.search.Show()
		a.searchOrigin = a.active
		a.previousView = a.active
		a.active = viewSearch
		if !a.jqlMetaLoaded {
			return a, tea.Batch(textinput.Blink, a.fetchJQLMetadata()), true
		}
		return a, textinput.Blink, true

	case key.Matches(msg, a.keys.Setup) && (a.active == viewSprint || a.active == viewBoard):
		a.setup = setupview.New(a.currentConfig())
		a.setup.SetSize(a.width, a.maxContentHeight())
		a.setup.GoToConfirm()
		a.needsSetup = true
		a.previousView = a.active
		a.active = viewSetup
		return a, a.setup.Init(), true

	case key.Matches(msg, a.keys.Board) && a.active == viewSprint:
		a.board = a.board.SetIssues(a.currentIssues, a.boardTitle)
		a.active = viewBoard
		return a, nil, true

	case key.Matches(msg, a.keys.Board) && a.active == viewBoard:
		a.active = viewSprint
		return a, nil, true

	case key.Matches(msg, a.keys.Board) && a.active == viewSearch && a.search.ShowingResults():
		a.board = a.board.SetIssues(a.searchIssues, a.searchBoardDisplayTitle())
		a.active = viewSearchBoard
		return a, nil, true

	case key.Matches(msg, a.keys.Board) && a.active == viewSearchBoard:
		a.search.Reshow()
		a.active = viewSearch
		return a, nil, true

	case key.Matches(msg, a.keys.Branch) && a.active == viewIssue:
		if iss := a.issue.CurrentIssue(); iss != nil {
			repoPath := ""
			branchUpper := false
			branchMode := "local"
			branchCopyName := false
			if a.client != nil {
				repoPath = a.client.Config().RepoPath
				branchUpper = a.client.Config().BranchUppercase
				branchMode = a.client.Config().BranchMode
				branchCopyName = a.client.Config().BranchCopyName
			}
			a.branch = branchview.New(*iss, repoPath, branchUpper, branchMode, branchCopyName)
			a.branch.SetSize(a.width, a.height-2)
			a.active = viewBranch
			return a, nil, true
		}

	case key.Matches(msg, a.keys.Create) && a.client != nil &&
		(a.active == viewSprint || a.active == viewBoard):
		a.create = createview.New(a.client)
		a.create.SetSize(a.width, a.height)
		a.previousView = a.active
		a.active = viewCreate
		return a, a.create.Init(), true

	case key.Matches(msg, a.keys.Transition) && (a.active == viewIssue || a.active == viewBoard || a.active == viewSearchBoard || a.active == viewSprint || a.active == viewSearch):
		var issueKey string
		switch a.active {
		case viewIssue:
			if iss := a.issue.CurrentIssue(); iss != nil {
				issueKey = iss.Key
			}
		case viewBoard, viewSearchBoard:
			if iss, ok := a.board.HighlightedIssue(); ok {
				issueKey = iss.Key
			}
		case viewSprint:
			if iss, ok := a.issueList.HighlightedIssue(); ok {
				issueKey = iss.Key
			}
		case viewSearch:
			if iss, ok := a.search.HighlightedIssue(); ok {
				issueKey = iss.Key
			}
		}
		if issueKey != "" {
			a.transition = transitionpickview.New(issueKey)
			a.transition.SetSize(a.width, a.height-2)
			a.transitionOrigin = a.active
			a.active = viewTransition
			return a, a.fetchTransitions(issueKey), true
		}

	case key.Matches(msg, a.keys.Comment) && a.active == viewIssue:
		if iss := a.issue.CurrentIssue(); iss != nil {
			a.comment = commentview.New(iss.Key)
			a.comment.SetSize(a.width, a.height-2)
			a.active = viewComment
			return a, nil, true
		}

	case key.Matches(msg, a.keys.Assign) && a.active == viewIssue:
		if iss := a.issue.CurrentIssue(); iss != nil {
			a.assign = assignpickview.New(iss.Key, iss.Assignee)
			a.assign.SetSize(a.width, a.height-2)
			a.active = viewAssign
			return a, nil, true
		}

	case key.Matches(msg, a.keys.Edit) && a.active == viewIssue:
		if iss := a.issue.CurrentIssue(); iss != nil {
			a.edit = editview.New(iss.Key)
			var priorities []string
			if a.jqlMeta != nil {
				priorities = a.jqlMeta.Priorities
			}
			a.edit.SetIssue(*iss, priorities)
			a.edit.SetSize(a.width, a.height-2)
			a.active = viewEdit
			return a, nil, true
		}

	case key.Matches(msg, a.keys.Link) && (a.active == viewIssue || a.active == viewSprint || a.active == viewSearch || a.active == viewBoard || a.active == viewSearchBoard):
		var issueKey string
		switch a.active {
		case viewIssue:
			if iss := a.issue.CurrentIssue(); iss != nil {
				issueKey = iss.Key
			}
		case viewBoard, viewSearchBoard:
			if iss, ok := a.board.HighlightedIssue(); ok {
				issueKey = iss.Key
			}
		case viewSprint:
			if iss, ok := a.issueList.HighlightedIssue(); ok {
				issueKey = iss.Key
			}
		case viewSearch:
			if iss, ok := a.search.HighlightedIssue(); ok {
				issueKey = iss.Key
			}
		}
		if issueKey != "" {
			a.link = linkpickview.New(issueKey)
			a.link.SetSize(a.width, a.height-2)
			a.linkOrigin = a.active
			a.active = viewLink
			return a, a.fetchLinkTypes(), true
		}

	case msg.String() == "x" && a.client != nil && a.active == viewSpaces && !a.wikiList.Filtering():
		var pageID string
		if sel := a.wikiList.SelectedItem(); sel != nil {
			pageID = sel.PageID()
		}
		if pageID != "" {
			u := a.client.ConfluencePageURL(pageID)
			if err := copyToClipboard(u); err == nil {
				a.statusMsg = fmt.Sprintf("Copied: %s", u)
			} else {
				a.err = fmt.Errorf("clipboard: %w", err)
			}
			return a, nil, true
		}

	case msg.String() == "x" && a.client != nil && (a.active == viewSprint || a.active == viewSearch || a.active == viewBoard || a.active == viewSearchBoard):
		var issueKey string
		switch a.active {
		case viewBoard, viewSearchBoard:
			if iss, ok := a.board.HighlightedIssue(); ok {
				issueKey = iss.Key
			}
		case viewSprint:
			if iss, ok := a.issueList.HighlightedIssue(); ok {
				issueKey = iss.Key
			}
		case viewSearch:
			if iss, ok := a.search.HighlightedIssue(); ok {
				issueKey = iss.Key
			}
		}
		if issueKey != "" {
			url := a.client.IssueURL(issueKey)
			if err := copyToClipboard(url); err == nil {
				a.statusMsg = fmt.Sprintf("Copied: %s", url)
			} else {
				a.err = fmt.Errorf("clipboard: %w", err)
			}
			return a, nil, true
		}

	case key.Matches(msg, a.keys.Delete) && a.active == viewIssue:
		if iss := a.issue.CurrentIssue(); iss != nil {
			a.del = deleteview.New(iss.Key, iss.Summary)
			a.del.SetSize(a.width, a.height-2)
			a.active = viewDelete
			return a, nil, true
		}

	case key.Matches(msg, a.keys.Watch) && a.active == viewIssue:
		if iss := a.issue.CurrentIssue(); iss != nil {
			return a, a.toggleWatch(iss.Key, iss.IsWatching), true
		}

	case key.Matches(msg, a.keys.Parent) && a.active == viewIssue:
		if iss := a.issue.CurrentIssue(); iss != nil && a.issue.HasParent() {
			a.issueStack = append(a.issueStack, *iss)
			placeholder := jira.Issue{Key: iss.ParentKey, Summary: "Loading..."}
			a.issue = a.issue.SetIssue(placeholder)
			a.issue.SetIssueURL(a.client.IssueURL(iss.ParentKey))
			loadSeq := a.nextIssueLoadSeq()
			return a, a.fetchIssueBundle(iss.ParentKey, "", loadSeq), true
		}
		if a.issue.CurrentIssue() != nil && !a.issue.HasParent() {
			a.statusMsg = "No parent issue"
			return a, nil, true
		}

	case key.Matches(msg, a.keys.IssuePick) && a.active == viewIssue:
		refs := a.issue.IssueKeys()
		if len(refs) > 0 {
			a.issuePick = issuepickview.New(refs)
			// Set title based on whether Confluence pages are present.
			hasPages := false
			for _, r := range refs {
				if r.Group == "Confluence Pages" {
					hasPages = true
					break
				}
			}
			if hasPages {
				a.issuePick.SetTitle("Issues & Pages")
			}
			a.issuePick.SetSize(a.width, a.height-2)
			a.issuePickOrigin = viewIssue
			a.active = viewIssuePick
			return a, nil, true
		}

	case key.Matches(msg, a.keys.IssuePick) && a.active == viewConfluence:
		if page := a.wikiPage.CurrentPage(); page != nil {
			var refs []issueview.IssueRef
			// Extract Confluence page links.
			for _, p := range adf.ExtractPageRefs(page.BodyADF) {
				refs = append(refs, issueview.IssueRef{Key: p.ID, Display: p.Title, Group: "Linked Pages"})
			}
			// Also extract Jira issue keys.
			for _, k := range adf.ExtractIssueKeys(page.BodyADF) {
				refs = append(refs, issueview.IssueRef{Key: k, Group: "Jira Issues"})
			}
			if len(refs) > 0 {
				a.issuePick = issuepickview.New(refs)
				a.issuePick.SetTitle("Pages & Issues")
				a.issuePick.SetSize(a.width, a.height-2)
				a.issuePickOrigin = viewConfluence
				a.active = viewIssuePick
				return a, nil, true
			}
		}

	case key.Matches(msg, a.keys.Filters) &&
		(a.active == viewSprint || a.active == viewBoard || a.active == viewSearchBoard):
		a.filter.Reset()
		a.filter = a.filter.SetFilters(a.savedFilters)
		a.filterOrigin = a.active
		a.previousView = a.active
		a.active = viewFilters
		return a, nil, true

	case key.Matches(msg, a.keys.Profile) &&
		(a.active == viewSprint || a.active == viewBoard):
		profiles, err := config.ListProfileNames()
		if err != nil {
			return a, nil, true
		}
		if len(profiles) == 0 {
			// No profiles.json yet — show single "default" entry.
			profiles = []string{"default"}
		}
		a.profile = profilepickview.New(profiles, a.profileName)
		a.profile.SetSize(a.width, a.height-2)
		a.profileOrigin = a.active
		a.active = viewProfile
		return a, nil, true

	case key.Matches(msg, a.keys.BoardPick) && a.client != nil &&
		(a.active == viewSprint || a.active == viewBoard):
		a.boardPick = boardpickview.New()
		a.boardPick = a.boardPick.SetSize(a.width, a.height-2)
		a.boardPickOrigin = a.active
		a.active = viewBoardPick
		return a, a.fetchBoardsForPicker(), true

	case key.Matches(msg, a.keys.Refresh) && (a.active == viewSprint || a.active == viewBoard):
		a.previousView = a.active
		a.active = viewLoading
		a.loadingMsg = "Refreshing sprint issues..."
		a.paginationSeq++
		return a, tea.Batch(a.spinner.Tick, a.fetchActiveSprintForBoard(a.boardID)), true

	case key.Matches(msg, a.keys.Refresh) && a.active == viewSearchBoard:
		a.statusMsg = "Refreshing..."
		a.paginationSeq++
		return a, a.searchJQL(a.searchBoardTitle), true

	case key.Matches(msg, a.keys.Refresh) && a.active == viewConfluence:
		if page := a.wikiPage.CurrentPage(); page != nil {
			a.statusMsg = "Refreshing..."
			return a, a.fetchConfluencePage(page.ID), true
		}

	case key.Matches(msg, a.keys.Refresh) && a.active == viewIssue:
		if iss := a.issue.CurrentIssue(); iss != nil {
			a.statusMsg = "Refreshing..."
			loadSeq := a.nextIssueLoadSeq()
			return a, a.fetchIssueBundle(iss.Key, iss.IssueType, loadSeq), true
		}
	}

	// Unhandled key — let the active child view process it.
	return a, nil, false
}

// navigateBack moves to the parent view, or initiates quit confirmation at the top level.
func (a App) navigateBack() (App, tea.Cmd) {
	switch a.active {
	case viewFilters:
		if a.filterSaveReturn != 0 {
			a.active = a.filterSaveReturn
			a.filterSaveReturn = 0
			return a, nil
		}
		a.active = a.filterOrigin
		return a, nil
	case viewTransition:
		a.active = a.transitionOrigin
		return a, nil
	case viewComment:
		a.active = viewIssue
		return a, nil
	case viewAssign:
		a.active = viewIssue
		return a, nil
	case viewEdit:
		a.active = viewIssue
		return a, nil
	case viewLink:
		a.active = a.linkOrigin
		return a, nil
	case viewDelete:
		a.active = viewIssue
		return a, nil
	case viewBranch:
		a.active = viewIssue
		return a, nil
	case viewIssuePick:
		a.active = a.issuePickOrigin
		return a, nil
	case viewProfile:
		a.active = a.profileOrigin
		return a, nil
	case viewBoardPick:
		a.active = a.boardPickOrigin
		return a, nil
	case viewHelp:
		a.active = a.helpOrigin
		return a, nil
	case viewIssue:
		if len(a.issueStack) > 0 {
			prev := a.issueStack[len(a.issueStack)-1]
			a.issueStack = a.issueStack[:len(a.issueStack)-1]
			a.issue = a.issue.SetIssue(prev)
			a.issue.SetIssueURL(a.client.IssueURL(prev.Key))
			loadSeq := a.nextIssueLoadSeq()
			return a, a.fetchIssueBundle(prev.Key, prev.IssueType, loadSeq)
		}
		switch a.previousView {
		case viewSearch:
			a.search.Reshow()
			a.active = viewSearch
			a.previousView = a.searchOrigin
		case viewSearchBoard:
			a.board = a.board.SetIssues(a.searchIssues, a.searchBoardDisplayTitle())
			a.active = viewSearchBoard
		case viewBoard:
			a.active = viewBoard
		case viewConfluence:
			a.active = viewConfluence
		default:
			a.active = viewSprint
		}
		return a, nil
	case viewBoard:
		a.active = viewSprint
		return a, nil
	case viewSearchBoard:
		a.search.Reshow()
		a.active = viewSearch
		return a, nil
	case viewSprint:
		if a.issueList.Filtered() {
			a.issueList = a.issueList.ResetFilter()
			return a, nil
		}
		a.confirmQuit = true
		return a, nil
	case viewCreate:
		a.active = a.previousView
		return a, nil
	case viewSearch:
		// If the results list filter is applied, clear it first.
		if a.search.ResultsFiltered() {
			a.search.ResetResultsFilter()
			return a, nil
		}
		// If showing results and we came from filters, go back to filters
		// instead of dropping into the JQL input.
		if a.search.ShowingResults() && a.previousView == viewFilters {
			a.search.Hide()
			a.active = viewFilters
			return a, nil
		}
		a.search.BackToInput()
		return a, nil
	case viewLoading:
		// If we came from a content view, go back to it.
		switch a.previousView {
		case viewSprint, viewBoard:
			a.active = a.previousView
			return a, nil
		}
		// Initial load or no meaningful previous view — quit.
		return a, tea.Quit
	case viewSpaces:
		// Navigate within confluence — esc/q stays in wiki mode.
		if a.wikiList.Filtered() {
			a.wikiList.ResetFilter()
			return a, nil
		}
		if a.wikiList.InPagesState() {
			a.wikiList.GoToSpaces()
			return a, nil
		}
		// At the top level — quit confirmation (matches Jira home behaviour).
		a.confirmQuit = true
		return a, nil
	case viewConfluence:
		// Pop from page stack if available (page → page navigation).
		if len(a.pageStack) > 0 {
			prev := a.pageStack[len(a.pageStack)-1]
			a.pageStack = a.pageStack[:len(a.pageStack)-1]
			a.wikiPage.SetPage(&prev)
			return a, a.fetchConfluencePage(prev.ID)
		}
		// Cross-type back: return to the view we came from (e.g., issue view).
		switch a.previousView {
		case viewIssue:
			a.active = viewIssue
		default:
			a.active = viewSpaces
		}
		return a, nil
	}
	return a, nil
}

// updateActiveView delegates the message to the currently active child view
// and handles sentinel-based navigation from child view responses.
func (a App) updateActiveView(msg tea.Msg) (App, tea.Cmd) {
	var cmd tea.Cmd
	switch a.active {
	case viewSetup:
		a.setup, cmd = a.setup.Update(msg)
		if a.setup.Quit() {
			if a.client != nil {
				// Re-invoked from a view — go back to where we were.
				a.needsSetup = false
				a.active = a.previousView
				return a, nil
			}
			return a, tea.Quit
		}
		if a.setup.Done() {
			cfg := a.setup.Config()
			profileName := a.setup.ProfileName()
			if profileName == "" {
				profileName = "default"
			}
			saveErr := config.WriteConfigProfile(profileName, cfg)
			if saveErr == nil {
				a.profileName = profileName
				filters.SetProfile(profileName)
			}
			if saveErr != nil {
				a.err = sanitiseError(fmt.Errorf("failed to save config: %w", saveErr))
				a.active = viewLoading
				return a, nil
			}
			// Create client and proceed to normal loading.
			a.client = client.New(cfg)
			a.needsSetup = false
			a.previousView = viewSetup
			a.loadingMsg = "Verifying credentials..."
			a.active = viewLoading
			return a, tea.Batch(a.spinner.Tick, a.verifyAuth())
		}
		return a, cmd
	case viewSprint:
		a.issueList, cmd = a.issueList.Update(msg)

		// Check if sprint view wants to open an issue.
		if iss, ok := a.issueList.SelectedIssue(); ok {
			a.active = viewIssue
			a.previousView = viewSprint
			a.issue = a.issue.SetIssue(iss)
			a.issue.SetIssueURL(a.client.IssueURL(iss.Key))
			// Fetch full details and children in background.
			loadSeq := a.nextIssueLoadSeq()
			return a, tea.Batch(cmd, a.fetchIssueBundle(iss.Key, iss.IssueType, loadSeq))
		}
	case viewBoard, viewSearchBoard:
		a.board, cmd = a.board.Update(msg)
		if iss, ok := a.board.SelectedIssue(); ok {
			a.previousView = a.active // Preserves viewBoard or viewSearchBoard.
			a.active = viewIssue
			a.issue = a.issue.SetIssue(iss)
			a.issue.SetIssueURL(a.client.IssueURL(iss.Key))
			loadSeq := a.nextIssueLoadSeq()
			return a, tea.Batch(cmd, a.fetchIssueBundle(iss.Key, iss.IssueType, loadSeq))
		}
	case viewIssue:
		a.issue, cmd = a.issue.Update(msg)
		if url, ok := a.issue.OpenURL(); ok {
			openBrowser(url)
		}
		if url, ok := a.issue.CopyURL(); ok {
			if err := copyToClipboard(url); err == nil {
				a.statusMsg = fmt.Sprintf("Copied: %s", url)
			} else {
				a.err = fmt.Errorf("clipboard: %w", err)
			}
		}
	case viewBranch:
		a.branch, cmd = a.branch.Update(msg)
		if req := a.branch.SubmittedBranch(); req != nil {
			return a, createBranch(req)
		}
		if a.branch.Dismissed() {
			a.active = viewIssue
		}
	case viewCreate:
		a.create, cmd = a.create.Update(msg)
		if a.create.Quit() {
			a.active = a.previousView
			return a, nil
		}
		if a.create.Done() {
			key := a.create.CreatedKey()
			a.statusMsg = fmt.Sprintf("Created %s", key)
			a.active = viewIssue
			loadSeq := a.nextIssueLoadSeq()
			return a, a.fetchIssueBundle(key, "", loadSeq)
		}
	case viewHelp:
		a.help, cmd = a.help.Update(msg)
		if a.help.Dismissed() {
			a.active = a.helpOrigin
		}
	case viewTransition:
		a.transition, cmd = a.transition.Update(msg)
		if t := a.transition.Selected(); t != nil {
			toStatus := t.ToStatus
			if toStatus == "" {
				toStatus = t.Name // Fallback if API didn't provide target status.
			}
			return a, a.transitionIssue(a.transition.IssueKey(), t.ID, toStatus)
		}
		if a.transition.Dismissed() {
			a.active = a.transitionOrigin
		}
	case viewComment:
		a.comment, cmd = a.comment.Update(msg)
		if body := a.comment.SubmittedComment(); body != "" {
			return a, a.addComment(a.comment.IssueKey(), body)
		}
		if a.comment.Dismissed() {
			a.active = viewIssue
		}
	case viewAssign:
		a.assign, cmd = a.assign.Update(msg)
		if prefix := a.assign.NeedsUserSearch(); prefix != "" {
			cmd = tea.Batch(cmd, a.searchUsersForAssign(prefix))
		}
		if req := a.assign.SelectedAssignee(); req != nil {
			return a, a.assignIssue(a.issue.CurrentIssue().Key, req)
		}
		if a.assign.Dismissed() {
			a.active = viewIssue
		}
	case viewEdit:
		a.edit, cmd = a.edit.Update(msg)
		if req := a.edit.SubmittedEdit(); req != nil {
			return a, a.editIssue(a.issue.CurrentIssue().Key, req)
		}
		if a.edit.Dismissed() {
			a.active = viewIssue
		}
	case viewLink:
		a.link, cmd = a.link.Update(msg)
		if req := a.link.SubmittedLink(); req != nil {
			return a, a.linkIssue(req)
		}
		if a.link.Dismissed() {
			a.active = a.linkOrigin
		}
	case viewDelete:
		a.del, cmd = a.del.Update(msg)
		if req := a.del.Confirmed(); req != nil {
			return a, a.deleteIssue(req)
		}
		if a.del.Dismissed() {
			a.active = viewIssue
		}
	case viewIssuePick:
		a.issuePick, cmd = a.issuePick.Update(msg)
		if ref := a.issuePick.Selected(); ref != nil {
			// Check if this is a Confluence page ID (all digits).
			if isPageID(ref.Key) {
				if a.issuePickOrigin == viewConfluence {
					// Page-to-page: push onto stack, preserve previousView
					// so the original cross-type origin is retained.
					if page := a.wikiPage.CurrentPage(); page != nil {
						a.pageStack = append(a.pageStack, *page)
					}
				} else {
					// Cross-type (e.g., issue → confluence): record origin.
					a.previousView = a.issuePickOrigin
				}
				a.wikiPage = wikiview.New()
				a.wikiPage = a.wikiPage.SetSize(a.width, a.maxContentHeight())
				a.active = viewConfluence
				return a, a.fetchConfluencePage(ref.Key)
			}
			if a.issuePickOrigin == viewIssue {
				if iss := a.issue.CurrentIssue(); iss != nil {
					a.issueStack = append(a.issueStack, *iss)
				}
			}
			placeholder := jira.Issue{Key: ref.Key, Summary: "Loading..."}
			a.issue = a.issue.SetIssue(placeholder)
			a.issue.SetIssueURL(a.client.IssueURL(ref.Key))
			a.previousView = a.issuePickOrigin
			a.active = viewIssue
			loadSeq := a.nextIssueLoadSeq()
			return a, a.fetchIssueBundle(ref.Key, "", loadSeq)
		}
		if a.issuePick.Dismissed() {
			a.active = a.issuePickOrigin
		}
	case viewProfile:
		a.profile, cmd = a.profile.Update(msg)
		if name := a.profile.Selected(); name != "" {
			if name == a.profileName {
				// Already on this profile.
				a.active = a.profileOrigin
			} else {
				a.active = a.profileOrigin
				return a, a.switchProfile(name)
			}
		}
		if a.profile.NewProfile() {
			// Launch setup wizard for a new profile.
			empty := &config.Config{AuthType: "basic"}
			a.setup = setupview.New(empty)
			a.setup.SetForNewProfile()
			a.setup.SetSize(a.width, a.maxContentHeight())
			a.needsSetup = true
			a.previousView = a.profileOrigin
			a.active = viewSetup
			return a, a.setup.Init()
		}
		if a.profile.Dismissed() {
			a.active = a.profileOrigin
		}
	case viewBoardPick:
		a.boardPick, cmd = a.boardPick.Update(msg)
		if b := a.boardPick.Selected(); b != nil {
			a.boardID = b.ID
			a.loadingMsg = fmt.Sprintf("Loading %s...", b.Name)
			a.previousView = a.boardPickOrigin
			a.active = viewLoading
			a.paginationSeq++
			return a, tea.Batch(cmd, a.spinner.Tick, a.fetchActiveSprintForBoard(b.ID))
		}
		if a.boardPick.Dismissed() {
			a.active = a.boardPickOrigin
		}
	case viewFilters:
		a.filter, cmd = a.filter.Update(msg)

		// Apply a filter → run JQL search.
		// Stay on viewFilters while loading — SearchResultsMsg transitions to viewSearch.
		if f := a.filter.Applied(); f != nil {
			a.searchOrigin = viewFilters
			a.search.SetFilterName(f.Name)
			a.statusMsg = "Searching..."
			a.statusIsError = false
			a.paginationSeq++
			return a, tea.Batch(cmd, a.searchJQL(f.JQL))
		}

		// Save / update a filter.
		if id, name, jql, ok := a.filter.SaveRequested(); ok {
			var err error
			var saved jira.SavedFilter
			if id == "" {
				saved, err = filters.Add(name, jql)
			} else {
				err = filters.Update(id, name, jql)
				saved = jira.SavedFilter{ID: id, Name: name, JQL: jql}
			}
			if err != nil {
				a.err = err
			} else {
				return a, func() tea.Msg { return FilterSavedMsg{Filter: saved} }
			}
		}

		// Delete a filter.
		if id := a.filter.DeleteRequested(); id != "" {
			if err := filters.Delete(id); err != nil {
				a.err = err
			} else {
				return a, func() tea.Msg { return FilterDeletedMsg{ID: id} }
			}
		}

		// Copy JQL to clipboard.
		if jql := a.filter.CopyJQLRequested(); jql != "" {
			if err := copyToClipboard(jql); err != nil {
				a.err = err
			} else {
				a.statusMsg = "JQL copied to clipboard"
			}
		}

		// Duplicate a filter.
		if id := a.filter.DuplicateRequested(); id != "" {
			dup, err := filters.Duplicate(id)
			if err != nil {
				a.err = err
			} else if dup.ID != "" {
				return a, func() tea.Msg { return FilterDuplicatedMsg{Filter: dup} }
			}
		}

		// Toggle favourite.
		if id := a.filter.FavouriteRequested(); id != "" {
			if err := filters.ToggleFavourite(id); err != nil {
				a.err = err
			} else {
				if fs, err := filters.Load(); err == nil {
					a.savedFilters = filters.Sorted(fs)
					a.filter = a.filter.SetFilters(a.savedFilters)
				}
			}
		}

		// Dismissed — go back.
		if a.filter.Dismissed() {
			if a.filterSaveReturn != 0 {
				a.active = a.filterSaveReturn
				a.filterSaveReturn = 0
			} else {
				a.active = a.filterOrigin
			}
			return a, cmd
		}
		return a, cmd
	case viewSpaces:
		a.wikiList, cmd = a.wikiList.Update(msg)
		if p := a.wikiList.SelectedPage(); p != nil {
			a.wikiPage = wikiview.New()
			a.wikiPage = a.wikiPage.SetSize(a.width, a.maxContentHeight())
			return a, a.fetchConfluencePage(p.ID)
		}
		if fetchID := a.wikiList.NeedsFetch(); fetchID != "" {
			return a, a.fetchSpacePages(fetchID)
		}
		if a.wikiList.Dismissed() {
			a.confirmQuit = true
		}
	case viewConfluence:
		a.wikiPage, cmd = a.wikiPage.Update(msg)
		if url, ok := a.wikiPage.OpenURL(); ok {
			if a.client != nil {
				openBrowser(a.client.ConfluencePageURL(strings.TrimPrefix(url, "page/")))
			}
		}
		if pageID, ok := a.wikiPage.CopyURL(); ok {
			if a.client != nil {
				u := a.client.ConfluencePageURL(pageID)
				if err := copyToClipboard(u); err == nil {
					a.statusMsg = fmt.Sprintf("Copied: %s", u)
				} else {
					a.err = fmt.Errorf("clipboard: %w", err)
				}
			}
		}
		if a.wikiPage.Dismissed() {
			a.active = viewSpaces
		}
	case viewSearch:
		a.search, cmd = a.search.Update(msg)
		if prefix := a.search.NeedsUserSearch(); prefix != "" {
			cmd = tea.Batch(cmd, a.searchUsers(prefix))
		}
		if q := a.search.SubmittedQuery(); q != "" {
			a.statusMsg = "Searching..."
			a.statusIsError = false
			a.paginationSeq++
			return a, tea.Batch(cmd, a.searchJQL(q))
		}
		if jql := a.search.SaveFilter(); jql != "" {
			a.filter.Reset()
			a.filter = a.filter.SetFilters(a.savedFilters)
			a.filter.StartAdd(jql)
			a.filterSaveReturn = a.active // Return to search after save; don't overwrite filterOrigin.
			a.active = viewFilters
			return a, cmd
		}
		if iss := a.search.SelectedIssue(); iss != nil {
			a.issueStack = nil
			a.search.Hide()
			a.previousView = viewSearch
			a.active = viewIssue
			a.issue = a.issue.SetIssue(*iss)
			a.issue.SetIssueURL(a.client.IssueURL(iss.Key))
			loadSeq := a.nextIssueLoadSeq()
			return a, tea.Batch(cmd, a.fetchIssueBundle(iss.Key, iss.IssueType, loadSeq))
		}
		// User closed search without entering a query — return to previous view.
		if a.search.Dismissed() {
			a.active = a.previousView
			return a, cmd
		}
		// Safety net: search became hidden but no sentinel fired.
		if !a.search.Visible() {
			a.active = a.previousView
			return a, cmd
		}
	}

	return a, cmd
}

// inputActive reports whether a text input is focused (search overlay, list filter, or setup wizard).
func (a App) inputActive() bool {
	switch a.active {
	case viewSetup:
		return a.setup.InputActive()
	case viewSearch:
		return a.search.InputActive()
	case viewCreate:
		return a.create.InputActive()
	case viewSprint:
		return a.issueList.Filtering()
	case viewBranch:
		return a.branch.InputActive()
	case viewTransition:
		return a.transition.InputActive()
	case viewComment:
		return a.comment.InputActive()
	case viewFilters:
		return a.filter.InputActive()
	case viewAssign:
		return a.assign.InputActive()
	case viewEdit:
		return a.edit.InputActive()
	case viewLink:
		return a.link.InputActive()
	case viewDelete:
		return a.del.InputActive()
	case viewIssuePick:
		return a.issuePick.InputActive()
	case viewProfile:
		return a.profile.InputActive()
	case viewBoardPick:
		return a.boardPick.InputActive()
	case viewSpaces:
		return a.wikiList.Filtering()
	default:
		return false
	}
}

// isBackKey returns true if the key should trigger back-navigation.
func (a App) isBackKey(msg tea.KeyMsg) bool {
	k := msg.String()
	return k == "esc" || k == "q"
}
