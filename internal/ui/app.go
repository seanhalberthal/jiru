package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/seanhalberthal/jiru/internal/client"
	"github.com/seanhalberthal/jiru/internal/config"
	"github.com/seanhalberthal/jiru/internal/confluence"
	"github.com/seanhalberthal/jiru/internal/filters"
	"github.com/seanhalberthal/jiru/internal/jira"
	"github.com/seanhalberthal/jiru/internal/jql"
	"github.com/seanhalberthal/jiru/internal/recents"
	"github.com/seanhalberthal/jiru/internal/theme"
	"github.com/seanhalberthal/jiru/internal/ui/assignpickview"
	"github.com/seanhalberthal/jiru/internal/ui/boardpickview"
	"github.com/seanhalberthal/jiru/internal/ui/boardview"
	"github.com/seanhalberthal/jiru/internal/ui/branchview"
	"github.com/seanhalberthal/jiru/internal/ui/commentview"
	"github.com/seanhalberthal/jiru/internal/ui/createview"
	"github.com/seanhalberthal/jiru/internal/ui/deleteview"
	"github.com/seanhalberthal/jiru/internal/ui/editview"
	"github.com/seanhalberthal/jiru/internal/ui/filterpickview"
	"github.com/seanhalberthal/jiru/internal/ui/helpview"
	"github.com/seanhalberthal/jiru/internal/ui/issuelistview"
	"github.com/seanhalberthal/jiru/internal/ui/issuepickview"
	"github.com/seanhalberthal/jiru/internal/ui/issueview"
	"github.com/seanhalberthal/jiru/internal/ui/linkpickview"
	"github.com/seanhalberthal/jiru/internal/ui/profilepickview"
	"github.com/seanhalberthal/jiru/internal/ui/searchview"
	"github.com/seanhalberthal/jiru/internal/ui/setupview"
	"github.com/seanhalberthal/jiru/internal/ui/transitionpickview"
	"github.com/seanhalberthal/jiru/internal/ui/wikilistview"
	"github.com/seanhalberthal/jiru/internal/ui/wikiview"
)

// view represents which pane is currently active.
type view int

const (
	viewSetup view = iota
	viewLoading
	viewSprint
	viewIssue
	viewSearch
	viewSearchBoard // Board view for search/filter results.
	viewBoard
	viewBranch
	viewCreate
	viewTransition
	viewComment
	viewFilters
	viewAssign
	viewEdit
	viewLink
	viewDelete
	viewIssuePick
	viewProfile
	viewBoardPick  // Board switcher overlay
	viewSpaces     // Confluence space/page browser
	viewConfluence // Confluence page detail
	viewHelp       // Help overlay
)

// App is the root bubbletea model.
type App struct {
	client           client.JiraClient
	keys             KeyMap
	active           view
	previousView     view
	searchOrigin     view // View that was active before search was opened.
	filterOrigin     view // View that was active before filters was opened.
	filterSaveReturn view // Return target when filters was opened from a save-from-search flow.
	transitionOrigin view // View that was active before transition was opened.
	linkOrigin       view // View that was active before link was opened.
	issueList        issuelistview.Model
	issue            issueview.Model
	search           searchview.Model
	board            boardview.Model
	branch           branchview.Model
	create           createview.Model
	transition       transitionpickview.Model
	comment          commentview.Model
	filter           filterpickview.Model
	assign           assignpickview.Model
	edit             editview.Model
	link             linkpickview.Model
	del              deleteview.Model
	issuePick        issuepickview.Model
	profile          profilepickview.Model
	boardPick        boardpickview.Model
	boardPickOrigin  view
	wikiList         wikilistview.Model
	wikiPage         wikiview.Model
	help             helpview.Model
	helpOrigin       view
	profileOrigin    view
	profileName      string             // Current active profile name.
	spacesLoaded     bool               // Prevents redundant space fetches.
	cachedSpaces     []confluence.Space // Cached for space key resolution.
	tabOrigin        view               // View that was active before tab to confluence.
	issuePickOrigin  view               // View that was active before issue picker.
	setup            setupview.Model
	spinner          spinner.Model
	width            int
	height           int
	statusMsg        string
	statusIsError    bool      // True when the current status message is an error.
	statusMsgTime    time.Time // When the status message was set.
	loadingMsg       string    // Contextual loading message for the loading view.
	err              error
	retryCmd         tea.Cmd // Command to retry on 'r' from the error dialog.
	boardID          int
	directIssue      string
	needsSetup       bool
	issueStack       []jira.Issue       // Stack of issues for parent/pick navigation.
	pageStack        []confluence.Page  // Stack of pages for back-navigation in confluence.
	currentIssues    []jira.Issue       // Cached for list↔board toggle.
	searchIssues     []jira.Issue       // Cached search results for list↔board toggle.
	searchBoardTitle string             // Title for the search board view.
	boardTitle       string             // Dynamic title: sprint name, board name, project key, etc.
	jqlMetaLoaded    bool               // Prevents redundant metadata fetches.
	jqlMeta          *jira.JQLMetadata  // Cached metadata for edit view priorities etc.
	paginationSeq    int                // Incremented each time a new fetch starts; stale pages are discarded.
	savedFilters     []jira.SavedFilter // Cached filter list for filterpickview.
	version          string
	confirmQuit      bool // True when waiting for quit confirmation.
}

// NewApp creates a new root application model.
// If missing is non-empty, the setup wizard is shown instead of normal loading.
func NewApp(c client.JiraClient, directIssue string, partial *config.Config, missing []string, version string) App {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(theme.ColourPrimary)

	fv := filterpickview.New()

	app := App{
		client:          c,
		keys:            DefaultKeyMap(),
		active:          viewLoading,
		issueList:       issuelistview.New(),
		issue:           issueview.New(),
		search:          searchview.New(),
		board:           boardview.New(),
		filter:          fv,
		wikiList:        wikilistview.New(),
		wikiPage:        wikiview.New(),
		spinner:         s,
		directIssue:     directIssue,
		version:         version,
		tabOrigin:       viewSprint,
		issuePickOrigin: viewIssue,
	}

	// Load saved filters — non-fatal if unavailable.
	if fs, err := filters.Load(); err == nil {
		app.savedFilters = filters.Sorted(fs)
	}
	app.filter = app.filter.SetFilters(app.savedFilters)

	if len(missing) > 0 {
		app.needsSetup = true
		app.setup = setupview.New(partial)
		app.active = viewSetup
	}

	return app
}

// SetProfileName sets the active profile name for display/switching.
func (a *App) SetProfileName(name string) {
	a.profileName = name
}

// resetViews re-initialises all child view models and applies current dimensions.
func (a App) resetViews() App {
	contentHeight := a.maxContentHeight()

	a.issueList = issuelistview.New().SetSize(a.width, contentHeight)
	a.issue = issueview.New().SetSize(a.width, contentHeight)
	a.search = searchview.New().SetSize(a.width, contentHeight)
	a.board = boardview.New().SetSize(a.width, contentHeight)
	a.wikiList = wikilistview.New().SetSize(a.width, contentHeight)
	a.wikiPage = wikiview.New().SetSize(a.width, contentHeight)
	a.filter = filterpickview.New().SetSize(a.width, contentHeight)
	a.branch = branchview.Model{}
	a.transition = transitionpickview.Model{}
	a.comment = commentview.Model{}
	a.assign = assignpickview.Model{}
	a.edit = editview.Model{}
	a.link = linkpickview.Model{}
	a.del = deleteview.Model{}
	a.issuePick = issuepickview.Model{}
	a.help = helpview.Model{}
	a.create = createview.Model{}

	return a
}

func (a App) Init() tea.Cmd {
	if a.needsSetup {
		return a.setup.Init()
	}
	return tea.Batch(
		a.spinner.Tick,
		a.verifyAuth(),
	)
}

// statusDismissDelay is how long a status message stays visible before auto-dismissing.
const statusDismissDelay = 5 * time.Second

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	prevStatus := a.statusMsg

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		contentHeight := a.maxContentHeight()
		a.issueList = a.issueList.SetSize(msg.Width, contentHeight)
		a.issue = a.issue.SetSize(msg.Width, contentHeight)
		a.search = a.search.SetSize(msg.Width, contentHeight)
		a.board = a.board.SetSize(msg.Width, contentHeight)
		a.branch.SetSize(msg.Width, contentHeight)
		a.transition.SetSize(msg.Width, contentHeight)
		a.comment.SetSize(msg.Width, contentHeight)
		a.assign.SetSize(msg.Width, contentHeight)
		a.edit.SetSize(msg.Width, contentHeight)
		a.link.SetSize(msg.Width, contentHeight)
		a.del.SetSize(msg.Width, contentHeight)
		a.issuePick.SetSize(msg.Width, contentHeight)
		a.filter = a.filter.SetSize(msg.Width, contentHeight)
		a.help.SetSize(msg.Width, msg.Height)
		a.setup.SetSize(msg.Width, contentHeight)
		a.create.SetSize(msg.Width, msg.Height)
		a.wikiList = a.wikiList.SetSize(msg.Width, contentHeight)
		a.wikiPage = a.wikiPage.SetSize(msg.Width, contentHeight)
		return a, nil

	case statusDismissMsg:
		// Auto-dismiss status message after timeout, if it hasn't been replaced.
		if a.statusMsg != "" && msg.setAt.Equal(a.statusMsgTime) {
			a.statusMsg = ""
			a.statusIsError = false
		}
		return a, nil

	case tea.KeyMsg:
		updated, keyCmd, handled := a.handleKeyMsg(msg)
		if handled {
			// Schedule auto-dismiss if a new status message was set.
			if updated.statusMsg != "" && updated.statusMsg != prevStatus {
				updated.statusMsgTime = time.Now()
				t := updated.statusMsgTime
				keyCmd = tea.Batch(keyCmd, tea.Tick(statusDismissDelay, func(_ time.Time) tea.Msg {
					return statusDismissMsg{setAt: t}
				}))
			}
			return updated, keyCmd
		}
		a = updated // Preserve side effects (e.g. status cleared on esc).

	// --- Async message handlers ---

	case ClientReadyMsg:
		a.err = nil
		a.statusMsg = fmt.Sprintf("Authenticated as %s", msg.DisplayName)
		a.loadingMsg = "Fetching boards..."
		// Fetch JQL metadata (statuses, types, etc.) eagerly — used by
		// both search autocomplete and the board view column layout.
		var metaCmd tea.Cmd
		if !a.jqlMetaLoaded {
			metaCmd = a.fetchJQLMetadata()
		}
		if a.directIssue != "" {
			return a, tea.Batch(a.fetchIssueBundle(a.directIssue), metaCmd)
		}
		if a.client.Config().BoardID != 0 {
			a.boardID = a.client.Config().BoardID
			a.paginationSeq++
			return a, tea.Batch(a.fetchActiveSprintForBoard(a.boardID), metaCmd)
		}
		// No board configured — redirect to setup wizard.
		a.setup = setupview.New(a.currentConfig())
		a.setup.SetSize(a.width, a.maxContentHeight())
		a.setup.GoToConfirm()
		a.needsSetup = true
		a.previousView = viewSprint
		a.active = viewSetup
		return a, tea.Batch(a.setup.Init(), metaCmd)

	case SprintLoadedMsg:
		a.err = nil
		a.loadingMsg = fmt.Sprintf("Loading %s...", msg.Sprint.Name)
		a.paginationSeq++
		return a, a.fetchSprintIssues(msg.Sprint.ID, msg.Sprint.Name)

	case IssuesLoadedMsg:
		a.err = nil
		if a.previousView == viewBoard {
			a.active = viewBoard
		} else {
			a.active = viewSprint
		}
		a.currentIssues = msg.Issues
		a.boardTitle = msg.Title
		a.issueList = a.issueList.SetIssues(msg.Issues)
		if msg.HasMore {
			a.issueList = a.issueList.SetLoading(true)
			return a, a.fetchMoreIssues(IssuesPageMsg{
				Source:     msg.Source,
				From:       msg.From,
				SprintID:   msg.SprintID,
				SprintName: msg.SprintName,
				EpicKey:    msg.EpicKey,
				JQL:        msg.JQL,
				Project:    msg.Project,
				Seq:        msg.Seq,
				NextToken:  msg.NextToken,
			})
		}
		// Single page — populate board immediately.
		a.board = a.board.SetIssues(msg.Issues, msg.Title)
		return a, nil

	case IssuesPageMsg:
		if msg.Seq != a.paginationSeq {
			return a, nil // Stale page from a previous fetch — discard.
		}
		switch msg.Source {
		case "search":
			a.search.AppendResults(msg.Issues)
			// Keep search cache in sync for board toggle.
			seen := make(map[string]bool, len(a.searchIssues))
			for _, iss := range a.searchIssues {
				seen[iss.Key] = true
			}
			for _, iss := range msg.Issues {
				if !seen[iss.Key] {
					a.searchIssues = append(a.searchIssues, iss)
				}
			}
		default:
			// Dedup — Jira's offset-based pagination can return overlapping results.
			seen := make(map[string]bool, len(a.currentIssues))
			for _, iss := range a.currentIssues {
				seen[iss.Key] = true
			}
			for _, iss := range msg.Issues {
				if !seen[iss.Key] {
					a.currentIssues = append(a.currentIssues, iss)
				}
			}
			a.issueList = a.issueList.AppendIssues(msg.Issues)
		}
		if msg.HasMore {
			return a, a.fetchMoreIssues(msg)
		}
		// All pages loaded — populate the board once with the full dataset.
		if msg.Source == "search" && a.active == viewSearchBoard {
			a.board = a.board.SetIssues(a.searchIssues, a.searchBoardDisplayTitle())
		} else if msg.Source != "search" {
			a.board = a.board.SetIssues(a.currentIssues, a.boardTitle)
		}
		a.issueList = a.issueList.SetLoading(false)
		return a, nil

	case IssueSelectedMsg:
		a.issueStack = nil
		a.active = viewIssue
		a.issue = a.issue.SetIssue(msg.Issue)
		a.issue.SetIssueURL(a.client.IssueURL(msg.Issue.Key))
		return a, a.fetchIssueBundle(msg.Issue.Key)

	case IssueDetailMsg:
		// Update the issue view with full details if we're still viewing it.
		// Use UpdateIssue (not SetIssue) to preserve async-fetched children and branches.
		if a.active == viewIssue && msg.Issue != nil {
			// Preserve enriched parent data if the detail fetch didn't provide it.
			if prev := a.issue.CurrentIssue(); prev != nil && msg.Issue.ParentKey != "" {
				if msg.Issue.ParentType == "" && prev.ParentType != "" {
					msg.Issue.ParentType = prev.ParentType
				}
				if msg.Issue.ParentSummary == "" && prev.ParentSummary != "" {
					msg.Issue.ParentSummary = prev.ParentSummary
				}
			}
			a.issue = a.issue.UpdateIssue(*msg.Issue)
			a.issue.SetIssueURL(a.client.IssueURL(msg.Issue.Key))
		}
		return a, nil

	case ChildIssuesMsg:
		if a.active == viewIssue {
			if curr := a.issue.CurrentIssue(); curr != nil && curr.Key == msg.ParentKey {
				a.issue = a.issue.SetChildren(msg.Children)
			}
		}
		return a, nil

	case BranchInfoMsg:
		if a.active == viewIssue {
			if curr := a.issue.CurrentIssue(); curr != nil && curr.Key == msg.IssueKey {
				a.issue = a.issue.SetBranches(msg.Branches)
			}
		}
		return a, nil

	case SearchResultsMsg:
		a.err = nil
		if !a.search.Visible() {
			// Came from filter apply — make search visible in results mode.
			a.search.Reshow()
			a.previousView = a.searchOrigin
		}
		a.search.SetResults(msg.Issues, msg.Query)
		// Cache search results for board toggle.
		a.searchIssues = msg.Issues
		a.searchBoardTitle = msg.Query
		// Stay on search board when refreshing from there.
		if a.active == viewSearchBoard {
			a.board = a.board.SetIssues(msg.Issues, a.searchBoardDisplayTitle())
		} else {
			a.active = viewSearch
		}
		a.statusMsg = ""
		if msg.HasMore {
			return a, a.fetchMoreIssues(IssuesPageMsg{
				Source:    "search",
				From:      msg.From,
				JQL:       msg.Query,
				Seq:       msg.Seq,
				NextToken: msg.NextToken,
			})
		}
		return a, nil

	case JQLMetadataMsg:
		a.search = a.search.SetMetadata(msg.Meta)
		a.jqlMetaLoaded = true
		a.jqlMeta = msg.Meta
		if msg.Meta != nil {
			if len(msg.Meta.Statuses) > 0 {
				a.board = a.board.SetKnownStatuses(msg.Meta.Statuses)
			}
			if len(msg.Meta.StatusCategories) > 0 {
				theme.SetStatusCategoryMap(msg.Meta.StatusCategories)
			}
			a.filter.SetValues(&jql.ValueProvider{
				Statuses:    msg.Meta.Statuses,
				IssueTypes:  msg.Meta.IssueTypes,
				Priorities:  msg.Meta.Priorities,
				Resolutions: msg.Meta.Resolutions,
				Projects:    msg.Meta.Projects,
				Labels:      msg.Meta.Labels,
				Components:  msg.Meta.Components,
				Versions:    msg.Meta.Versions,
				Sprints:     msg.Meta.Sprints,
			})
		}
		return a, nil

	case UserSearchMsg:
		a.search = a.search.SetUserResults(msg.Names)
		return a, nil

	case TransitionsLoadedMsg:
		if a.active == viewTransition {
			a.transition.SetTransitions(msg.Transitions)
		}
		return a, nil

	case IssueTransitionedMsg:
		if a.active == viewTransition {
			a.active = a.transitionOrigin
			if msg.Err != nil {
				a.err = sanitiseError(msg.Err)
			} else {
				a.statusMsg = fmt.Sprintf("Moved to %s", msg.NewStatus)
				// Update cached issue lists so stale status isn't shown on back-navigation.
				for i, iss := range a.currentIssues {
					if iss.Key == msg.Key {
						a.currentIssues[i].Status = msg.NewStatus
					}
				}
				a.search.UpdateIssueStatus(msg.Key, msg.NewStatus)
				a.issueList = a.issueList.UpdateIssueStatus(msg.Key, msg.NewStatus)
				// Update search issue cache.
				for i, iss := range a.searchIssues {
					if iss.Key == msg.Key {
						a.searchIssues[i].Status = msg.NewStatus
					}
				}
				// Update board view in-place so the cursor follows the card.
				if a.transitionOrigin == viewBoard || a.transitionOrigin == viewSearchBoard {
					a.board = a.board.UpdateIssueStatus(msg.Key, msg.NewStatus)
				}
				if a.transitionOrigin == viewSearchBoard {
					// Re-run the filter query so issues that no longer match are removed.
					a.paginationSeq++
					return a, a.searchJQL(a.searchBoardTitle)
				}
				if a.transitionOrigin == viewIssue {
					// Re-fetch issue details to reflect the new status.
					return a, a.fetchIssueDetail(msg.Key)
				}
			}
		}
		return a, nil

	case IssueAssignedMsg:
		if a.active == viewAssign {
			a.active = viewIssue
			if msg.Err != nil {
				a.err = sanitiseError(msg.Err)
			} else {
				a.statusMsg = fmt.Sprintf("Assigned to %s", msg.Assignee)
				return a, a.fetchIssueDetail(msg.Key)
			}
		}
		return a, nil

	case AssignUserSearchMsg:
		if a.active == viewAssign {
			a.assign.SetUsers(msg.Users)
		}
		return a, nil

	case IssueEditedMsg:
		if a.active == viewEdit {
			a.active = viewIssue
			if msg.Err != nil {
				a.err = sanitiseError(msg.Err)
			} else {
				a.statusMsg = fmt.Sprintf("Updated %s", msg.Key)
				return a, a.fetchIssueDetail(msg.Key)
			}
		}
		return a, nil

	case LinkTypesLoadedMsg:
		if a.active == viewLink {
			a.link.SetLinkTypes(msg.Types)
		}
		return a, nil

	case IssueLinkCreatedMsg:
		if a.active == viewLink {
			a.active = a.linkOrigin
			if msg.Err != nil {
				a.err = sanitiseError(msg.Err)
			} else {
				a.statusMsg = fmt.Sprintf("Linked %s → %s", msg.SourceKey, msg.TargetKey)
				if a.linkOrigin == viewIssue {
					return a, a.fetchIssueDetail(msg.SourceKey)
				}
			}
		}
		return a, nil

	case IssueDeletedMsg:
		if a.active == viewDelete {
			if msg.Err != nil {
				a.active = viewIssue
				a.err = sanitiseError(msg.Err)
			} else {
				a.statusMsg = fmt.Sprintf("Deleted %s", msg.Key)
				// Navigate to previous list view, not back to the deleted issue.
				switch a.previousView {
				case viewBoard:
					a.active = viewBoard
				case viewSearchBoard:
					a.board = a.board.SetIssues(a.searchIssues, a.searchBoardDisplayTitle())
					a.active = viewSearchBoard
				default:
					a.active = viewSprint
				}
			}
		}
		return a, nil

	case FilterSavedMsg:
		reloadSavedFilters(&a)
		a.statusMsg = fmt.Sprintf("Filter %q saved", msg.Filter.Name)
		return a, nil

	case FilterDeletedMsg:
		reloadSavedFilters(&a)
		a.statusMsg = "Filter deleted"
		return a, nil

	case FilterDuplicatedMsg:
		reloadSavedFilters(&a)
		a.statusMsg = fmt.Sprintf("Filter %q duplicated", msg.Filter.Name)
		return a, nil

	case BoardPickLoadedMsg:
		a.boardPick = a.boardPick.SetBoards(msg.Boards)
		return a, nil

	case ProfileSwitchedMsg:
		a.client = msg.Client
		a.profileName = msg.Name
		a = a.resetViews()
		a.spacesLoaded = false
		a.currentIssues = nil
		a.boardTitle = ""
		a.jqlMeta = nil
		a.jqlMetaLoaded = false
		a.paginationSeq++
		a.issueStack = nil
		a.directIssue = ""
		a.boardID = msg.Config.BoardID

		// Reload filters for the new profile.
		filters.SetProfile(msg.Name)
		if fs, err := filters.Load(); err == nil {
			a.savedFilters = filters.Sorted(fs)
		}
		a.filter = a.filter.SetFilters(a.savedFilters)

		a.statusMsg = fmt.Sprintf("Switched to profile: %s", msg.Name)
		a.loadingMsg = "Verifying credentials..."
		a.active = viewLoading
		return a, tea.Batch(a.spinner.Tick, a.verifyAuth())

	case CommentAddedMsg:
		if a.active == viewComment {
			a.active = viewIssue
			if msg.Err != nil {
				a.err = msg.Err
			} else {
				a.statusMsg = "Comment added"
				return a, a.fetchIssueDetail(msg.Key)
			}
		}
		return a, nil

	case IssueWatchToggledMsg:
		if msg.Err != nil {
			a.err = sanitiseError(msg.Err)
		} else {
			if msg.IsWatching {
				a.statusMsg = fmt.Sprintf("Watching %s", msg.Key)
			} else {
				a.statusMsg = fmt.Sprintf("Unwatched %s", msg.Key)
			}
			// Update the cached issue.
			if iss := a.issue.CurrentIssue(); iss != nil && iss.Key == msg.Key {
				a.issue.SetWatching(msg.IsWatching)
			}
		}
		return a, nil

	case BranchCreatedMsg:
		if a.active == viewBranch {
			a.active = viewIssue
			if msg.Err != nil {
				a.err = sanitiseError(msg.Err)
			} else if msg.Copied {
				a.statusMsg = fmt.Sprintf("Copied to clipboard: %s", msg.Name)
			} else {
				switch msg.Mode {
				case "remote":
					a.statusMsg = fmt.Sprintf("Pushed branch '%s' to origin", msg.Name)
				case "both":
					a.statusMsg = fmt.Sprintf("Created and pushed branch '%s'", msg.Name)
				default:
					a.statusMsg = fmt.Sprintf("Switched to new branch '%s'", msg.Name)
				}
				if msg.NameCopied {
					a.statusMsg += " (copied to clipboard)"
				}
			}
		}
		return a, nil

	case OpenURLMsg:
		openBrowser(msg.URL)
		return a, nil

	case ErrMsg:
		// JQL search errors are shown inline so the user can fix the query
		// without leaving the search view (avoids trapping keys behind the
		// error overlay).
		if a.active == viewSearch {
			a.statusMsg = sanitiseError(msg.Err).Error()
			a.statusIsError = true
			return a, nil
		}
		a.err = sanitiseError(msg.Err)
		a.retryCmd = a.refreshCurrentView()
		// Clear loading state — pagination errors should not leave the UI stuck.
		a.issueList = a.issueList.SetLoading(false)
		return a, nil

	case SpacesLoadedMsg:
		a.wikiList = a.wikiList.SetSpaces(msg.Spaces)
		a.cachedSpaces = msg.Spaces
		// Load recents.
		if entries, err := recents.Load(); err == nil && len(entries) > 0 {
			a.wikiList = a.wikiList.SetRecents(recents.Sorted(entries))
		}
		return a, nil

	case SpacePagesLoadedMsg:
		a.wikiList = a.wikiList.SetPages(msg.Pages)
		return a, nil

	case ConfluencePageLoadedMsg:
		a.wikiPage.SetPage(msg.Page)
		a.wikiPage.SetAncestors(msg.Ancestors)
		if msg.SpaceKey != "" {
			a.wikiPage.SetSpaceKey(msg.SpaceKey)
		}
		a.active = viewConfluence
		// Record in recents.
		if msg.Page != nil {
			_ = recents.Add(msg.Page.ID, msg.Page.Title, msg.SpaceKey)
		}
		// Fetch comments asynchronously.
		var cmds []tea.Cmd
		cmds = append(cmds, tea.ClearScreen)
		if msg.Page != nil {
			cmds = append(cmds, a.fetchConfluenceComments(msg.Page.ID))
		}
		return a, tea.Batch(cmds...)

	case ConfluenceCommentsLoadedMsg:
		if a.wikiPage.CurrentPage() != nil && a.wikiPage.CurrentPage().ID == msg.PageID {
			a.wikiPage.SetComments(msg.Comments)
		}
		return a, nil

	case RemoteLinksLoadedMsg:
		if a.active == viewIssue {
			if curr := a.issue.CurrentIssue(); curr != nil && curr.Key == msg.IssueKey {
				a.issue = a.issue.SetRemoteLinks(msg.Links)
			}
		}
		return a, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		a.spinner, cmd = a.spinner.Update(msg)
		return a, cmd
	}

	// Delegate to the active child view.
	var cmd tea.Cmd
	a, cmd = a.updateActiveView(msg)

	// Schedule auto-dismiss tick when a new status message appears.
	if a.statusMsg != "" && a.statusMsg != prevStatus {
		a.statusMsgTime = time.Now()
		t := a.statusMsgTime
		cmd = tea.Batch(cmd, tea.Tick(statusDismissDelay, func(_ time.Time) tea.Msg {
			return statusDismissMsg{setAt: t}
		}))
	}

	return a, cmd
}

func (a App) View() string {
	if a.width == 0 {
		return "Loading..."
	}

	var content string

	switch a.active {
	case viewSetup:
		content = a.setup.View()
	case viewLoading:
		msg := a.loadingMsg
		if msg == "" {
			msg = "Connecting to Jira..."
		}
		spinnerLine := fmt.Sprintf("%s %s", a.spinner.View(), msg)
		var loadingContent string
		if logo := theme.RenderLogo(a.width); logo != "" {
			centredSpinner := lipgloss.NewStyle().Width(lipgloss.Width(logo)).Align(lipgloss.Center).Render(spinnerLine)
			loadingContent = lipgloss.JoinVertical(lipgloss.Left, logo, "", centredSpinner)
		} else {
			loadingContent = spinnerLine
		}
		content = lipgloss.Place(
			a.width, a.height-2,
			lipgloss.Center, lipgloss.Center,
			loadingContent,
		)
	case viewSprint:
		content = a.issueList.View()
	case viewIssue:
		content = a.issue.View()
	case viewSearch:
		content = a.search.View()
	case viewCreate:
		content = a.create.View()
	case viewBoard, viewSearchBoard:
		content = a.board.View()
	case viewBranch:
		content = a.branch.View()
	case viewHelp:
		content = a.help.View()
	case viewTransition:
		content = a.transition.View()
	case viewComment:
		content = a.comment.View()
	case viewFilters:
		content = a.filter.View()
	case viewAssign:
		content = a.assign.View()
	case viewEdit:
		content = a.edit.View()
	case viewLink:
		content = a.link.View()
	case viewDelete:
		content = a.del.View()
	case viewIssuePick:
		content = a.issuePick.View()
	case viewProfile:
		content = a.profile.View()
	case viewBoardPick:
		content = a.boardPick.View()
	case viewSpaces:
		content = a.wikiList.View()
	case viewConfluence:
		content = a.wikiPage.View()
	}

	if a.err != nil {
		var hint string
		if a.retryCmd != nil {
			hint = theme.StyleHelpKey.Render("r") + " " + theme.StyleHelpDesc.Render("retry") + "  " +
				theme.StyleHelpKey.Render("esc") + " " + theme.StyleHelpDesc.Render("dismiss")
		} else {
			hint = theme.StyleHelpKey.Render("esc") + " " + theme.StyleHelpDesc.Render("dismiss")
		}
		errBox := theme.StyleErrorDialog.Width(a.width / 2).Render(
			lipgloss.JoinVertical(lipgloss.Center,
				theme.StyleError.Render("Error"),
				"",
				theme.StyleSubtle.Render(a.err.Error()),
				"",
				hint,
			),
		)
		content = lipgloss.Place(a.width, a.height-2, lipgloss.Center, lipgloss.Center, errBox)
	}

	if a.confirmQuit {
		quitBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.ColourPrimary).
			Padding(1, 3).
			Align(lipgloss.Center).
			Render(lipgloss.JoinVertical(lipgloss.Center,
				lipgloss.NewStyle().Bold(true).Foreground(theme.ColourPrimary).Render("Quit jiru?"),
				"",
				theme.StyleSubtle.Render("q / esc / enter  to quit"),
				theme.StyleSubtle.Render("any other key    to cancel"),
			))
		content = lipgloss.Place(a.width, a.height-2, lipgloss.Center, lipgloss.Center, quitBox)
	}

	// Build footer with optional view-specific extras.
	var extra []footerBinding
	switch a.active {
	case viewSetup:
		for _, h := range a.setup.FooterHints() {
			extra = append(extra, footerBinding{h.Key, h.Desc})
		}
	case viewSearch:
		if a.search.ShowingResults() {
			extra = append(extra, footerBinding{"enter", "open"})
			if a.search.FilterName() == "" {
				extra = append(extra, footerBinding{"s", "save filter"})
			}
			extra = append(extra,
				footerBinding{"m", "move"},
				footerBinding{"L", "link"},
				footerBinding{"x", "copy url"},
				footerBinding{"b", "board view"},
				footerBinding{"r", "refresh"},
				footerBinding{"/", "filter"},
				footerBinding{"H", "home"},
				footerBinding{"esc", "back"},
			)
		} else {
			extra = append(extra,
				footerBinding{"enter", "search"},
				footerBinding{"↑↓", "browse"},
				footerBinding{"tab", "accept"},
				footerBinding{"esc", "close"},
			)
		}
	case viewFilters:
		switch {
		case a.filter.EditingName():
			extra = append(extra,
				footerBinding{"enter", "next"},
				footerBinding{"esc", "back"},
			)
		case a.filter.EditingQuery():
			extra = append(extra,
				footerBinding{"enter", "save"},
				footerBinding{"↑↓", "browse"},
				footerBinding{"tab", "accept"},
				footerBinding{"esc", "back"},
			)
		case a.filter.ConfirmingDelete():
			extra = append(extra,
				footerBinding{"y/enter", "confirm"},
				footerBinding{"n/esc", "cancel"},
			)
		default:
			extra = append(extra,
				footerBinding{"j/k", "navigate"},
				footerBinding{"enter", "apply"},
				footerBinding{"n", "new"},
				footerBinding{"e", "edit"},
				footerBinding{"d", "duplicate"},
				footerBinding{"f", "favourite"},
				footerBinding{"D", "delete"},
				footerBinding{"x", "copy JQL"},
				footerBinding{"esc", "back"},
			)
		}
	}
	help := footerView(a.active, a.width, a.version, a.err != nil, extra...)

	// Show status message above the footer when set.
	// Hide the footer entirely when the quit confirmation dialog is showing.
	var footer string
	if a.confirmQuit {
		footer = ""
	} else if a.statusMsg != "" && a.active != viewLoading {
		style := lipgloss.NewStyle().Foreground(theme.ColourSuccess)
		if a.statusIsError || a.err != nil {
			style = lipgloss.NewStyle().Foreground(theme.ColourError)
		}
		footer = lipgloss.JoinVertical(lipgloss.Left, style.Render(a.statusMsg), help)
	} else {
		footer = help
	}

	// Build the final output with exactly a.height lines. Manual line
	// construction avoids lipgloss Height/MaxHeight bugs with styled
	// content that caused double-footer rendering.
	footerLines := strings.Split(footer, "\n")
	contentTarget := max(a.height-len(footerLines), 0)

	contentLines := strings.Split(content, "\n")
	switch {
	case len(contentLines) < contentTarget:
		// Pad with blank lines.
		for len(contentLines) < contentTarget {
			contentLines = append(contentLines, "")
		}
	case len(contentLines) > contentTarget:
		// Truncate excess lines.
		contentLines = contentLines[:contentTarget]
	}

	return strings.Join(append(contentLines, footerLines...), "\n")
}

// maxContentHeight returns the available height for content views, reserving
// space for the tallest possible footer (issue view has the most bindings).
func (a App) maxContentHeight() int {
	footer := footerView(viewIssue, a.width, a.version, false)
	return a.height - lipgloss.Height(footer)
}

// currentConfig returns the current config for pre-filling the setup wizard.
func (a App) currentConfig() *config.Config {
	if a.client != nil {
		return a.client.Config()
	}
	return nil
}
