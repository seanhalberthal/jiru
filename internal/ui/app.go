package ui

import (
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"al.essio.dev/pkg/shellescape"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/seanhalberthal/jiru/internal/adf"
	"github.com/seanhalberthal/jiru/internal/client"
	"github.com/seanhalberthal/jiru/internal/config"
	"github.com/seanhalberthal/jiru/internal/confluence"
	"github.com/seanhalberthal/jiru/internal/filters"
	"github.com/seanhalberthal/jiru/internal/jira"
	"github.com/seanhalberthal/jiru/internal/jql"
	"github.com/seanhalberthal/jiru/internal/recents"
	"github.com/seanhalberthal/jiru/internal/theme"
	"github.com/seanhalberthal/jiru/internal/ui/assignview"
	"github.com/seanhalberthal/jiru/internal/ui/boardview"
	"github.com/seanhalberthal/jiru/internal/ui/branchview"
	"github.com/seanhalberthal/jiru/internal/ui/commentview"
	"github.com/seanhalberthal/jiru/internal/ui/confluenceview"
	"github.com/seanhalberthal/jiru/internal/ui/createview"
	"github.com/seanhalberthal/jiru/internal/ui/deleteview"
	"github.com/seanhalberthal/jiru/internal/ui/editview"
	"github.com/seanhalberthal/jiru/internal/ui/filterview"
	"github.com/seanhalberthal/jiru/internal/ui/homeview"
	"github.com/seanhalberthal/jiru/internal/ui/issuepickview"
	"github.com/seanhalberthal/jiru/internal/ui/issueview"
	"github.com/seanhalberthal/jiru/internal/ui/linkview"
	"github.com/seanhalberthal/jiru/internal/ui/profileview"
	"github.com/seanhalberthal/jiru/internal/ui/searchview"
	"github.com/seanhalberthal/jiru/internal/ui/setupview"
	"github.com/seanhalberthal/jiru/internal/ui/spacesview"
	"github.com/seanhalberthal/jiru/internal/ui/sprintview"
	"github.com/seanhalberthal/jiru/internal/ui/transitionview"
)

// view represents which pane is currently active.
type view int

const (
	viewSetup view = iota
	viewLoading
	viewHome
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
	viewSpaces     // Confluence space/page browser
	viewConfluence // Confluence page detail
)

// App is the root bubbletea model.
type App struct {
	client           client.JiraClient
	keys             KeyMap
	active           view
	previousView     view
	searchOrigin     view // View that was active before search was opened.
	filterOrigin     view // View that was active before filters was opened.
	transitionOrigin view // View that was active before transition was opened.
	home             homeview.Model
	sprint           sprintview.Model
	issue            issueview.Model
	search           searchview.Model
	board            boardview.Model
	branch           branchview.Model
	create           createview.Model
	transition       transitionview.Model
	comment          commentview.Model
	filter           filterview.Model
	assign           assignview.Model
	edit             editview.Model
	link             linkview.Model
	del              deleteview.Model
	issuePick        issuepickview.Model
	profile          profileview.Model
	spaces           spacesview.Model
	confPage         confluenceview.Model
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
	err              error
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
	savedFilters     []jira.SavedFilter // Cached filter list for filterview.
	version          string
	confirmQuit      bool // True when waiting for quit confirmation.
}

// NewApp creates a new root application model.
// If missing is non-empty, the setup wizard is shown instead of normal loading.
func NewApp(c client.JiraClient, directIssue string, partial *config.Config, missing []string, version string) App {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(theme.ColourPrimary)

	fv := filterview.New()

	app := App{
		client:          c,
		keys:            DefaultKeyMap(),
		active:          viewLoading,
		home:            homeview.New(),
		sprint:          sprintview.New(),
		issue:           issueview.New(),
		search:          searchview.New(),
		board:           boardview.New(),
		filter:          fv,
		spaces:          spacesview.New(),
		confPage:        confluenceview.New(),
		spinner:         s,
		directIssue:     directIssue,
		version:         version,
		tabOrigin:       viewHome,
		issuePickOrigin: viewIssue,
	}

	// Load saved filters — non-fatal if unavailable.
	if fs, err := filters.Load(); err == nil {
		app.savedFilters = filters.Sorted(fs)
	}
	app.filter.SetFilters(app.savedFilters)

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

func (a App) Init() tea.Cmd {
	if a.needsSetup {
		return a.setup.Init()
	}
	return tea.Batch(
		a.spinner.Tick,
		a.verifyAuth(),
	)
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		contentHeight := msg.Height - 4 // Reserve for help bar (may wrap to 2 rows) + status line.
		a.sprint = a.sprint.SetSize(msg.Width, contentHeight)
		a.issue = a.issue.SetSize(msg.Width, contentHeight)
		a.home.SetSize(msg.Width, contentHeight)
		a.search.SetSize(msg.Width, contentHeight)
		a.board.SetSize(msg.Width, contentHeight)
		a.branch.SetSize(msg.Width, contentHeight)
		a.transition.SetSize(msg.Width, contentHeight)
		a.comment.SetSize(msg.Width, contentHeight)
		a.assign.SetSize(msg.Width, contentHeight)
		a.edit.SetSize(msg.Width, contentHeight)
		a.link.SetSize(msg.Width, contentHeight)
		a.del.SetSize(msg.Width, contentHeight)
		a.issuePick.SetSize(msg.Width, contentHeight)
		a.filter.SetSize(msg.Width, contentHeight)
		a.setup.SetSize(msg.Width, msg.Height)
		a.create.SetSize(msg.Width, msg.Height)
		a.spaces.SetSize(msg.Width, contentHeight)
		a.confPage = a.confPage.SetSize(msg.Width, contentHeight)
		return a, nil

	case tea.KeyMsg:
		// Clear status message on any keypress.
		a.statusMsg = ""

		// ctrl+c always quits, regardless of input state.
		if msg.String() == "ctrl+c" {
			return a, tea.Quit
		}

		// Quit confirmation: confirm on esc/q/enter/y, dismiss on anything else.
		if a.confirmQuit {
			a.confirmQuit = false
			k := msg.String()
			if k == "esc" || k == "q" || k == "enter" || k == "y" {
				return a, tea.Quit
			}
			return a, nil
		}

		// Dismiss error overlay on esc/q.
		if a.err != nil {
			if a.isBackKey(msg) {
				a.err = nil
				// If stuck at loading (nothing will re-trigger), navigate back.
				if a.active == viewLoading {
					return a.navigateBack()
				}
				return a, nil
			}
			// Swallow all other keys while error is showing.
			return a, nil
		}

		// Setup wizard handles all its own keys (esc, enter, ctrl+b).
		if a.active == viewSetup {
			break
		}

		// When text input is active (search overlay or list filter),
		// let the child view handle everything else.
		if a.inputActive() {
			break
		}

		// esc, q, and h/H all navigate back one level (or quit at the top).
		if a.isBackKey(msg) {
			return a.navigateBack()
		}

		switch {
		case key.Matches(msg, a.keys.HomeTab) && (a.active == viewHome || a.active == viewSprint || a.active == viewBoard):
			a.tabOrigin = a.active
			a.active = viewSpaces
			if !a.spacesLoaded && a.client != nil {
				a.spacesLoaded = true
				return a, a.fetchConfluenceSpaces()
			}
			return a, nil
		case key.Matches(msg, a.keys.HomeTab) && a.active == viewSpaces:
			a.active = a.tabOrigin
			return a, nil
		case key.Matches(msg, a.keys.Search) && !a.search.Visible() && a.active != viewLoading:
			a.search.Show()
			a.searchOrigin = a.active
			a.previousView = a.active
			a.active = viewSearch
			if !a.jqlMetaLoaded {
				return a, tea.Batch(textinput.Blink, a.fetchJQLMetadata())
			}
			return a, textinput.Blink
		case key.Matches(msg, a.keys.Setup) && (a.active == viewHome || a.active == viewSprint || a.active == viewBoard):
			a.setup = setupview.New(a.currentConfig())
			a.setup.SetSize(a.width, a.height)
			a.setup.GoToConfirm()
			a.needsSetup = true
			a.previousView = a.active
			a.active = viewSetup
			return a, a.setup.Init()
		case key.Matches(msg, a.keys.Board) && a.active == viewSprint:
			a.board.SetIssues(a.currentIssues, a.boardTitle)
			a.active = viewBoard
			return a, nil
		case key.Matches(msg, a.keys.Board) && a.active == viewBoard:
			a.active = viewSprint
			return a, nil
		case key.Matches(msg, a.keys.Board) && a.active == viewSearch && a.search.ShowingResults():
			a.board.SetIssues(a.searchIssues, a.searchBoardDisplayTitle())
			a.active = viewSearchBoard
			return a, nil
		case key.Matches(msg, a.keys.Board) && a.active == viewSearchBoard:
			a.search.Reshow()
			a.active = viewSearch
			return a, nil
		case key.Matches(msg, a.keys.Branch) && a.active == viewIssue:
			if iss := a.issue.CurrentIssue(); iss != nil {
				repoPath := ""
				branchUpper := false
				branchMode := "local"
				if a.client != nil {
					repoPath = a.client.Config().RepoPath
					branchUpper = a.client.Config().BranchUppercase
					branchMode = a.client.Config().BranchMode
				}
				a.branch = branchview.New(*iss, repoPath, branchUpper, branchMode)
				a.branch.SetSize(a.width, a.height-2)
				a.active = viewBranch
				return a, nil
			}
		case key.Matches(msg, a.keys.Create) && a.client != nil &&
			(a.active == viewHome || a.active == viewSprint || a.active == viewBoard):
			a.create = createview.New(a.client)
			a.create.SetSize(a.width, a.height)
			a.previousView = a.active
			a.active = viewCreate
			return a, a.create.Init()
		case key.Matches(msg, a.keys.Transition) && (a.active == viewIssue || a.active == viewBoard || a.active == viewSearchBoard):
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
			}
			if issueKey != "" {
				a.transition = transitionview.New(issueKey)
				a.transition.SetSize(a.width, a.height-2)
				a.transitionOrigin = a.active
				a.active = viewTransition
				return a, a.fetchTransitions(issueKey)
			}
		case key.Matches(msg, a.keys.Comment) && a.active == viewIssue:
			if iss := a.issue.CurrentIssue(); iss != nil {
				a.comment = commentview.New(iss.Key)
				a.comment.SetSize(a.width, a.height-2)
				a.active = viewComment
				return a, nil
			}
		case key.Matches(msg, a.keys.Assign) && a.active == viewIssue:
			if iss := a.issue.CurrentIssue(); iss != nil {
				a.assign = assignview.New(iss.Key, iss.Assignee)
				a.assign.SetSize(a.width, a.height-2)
				a.active = viewAssign
				return a, nil
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
				return a, nil
			}
		case key.Matches(msg, a.keys.Link) && a.active == viewIssue:
			if iss := a.issue.CurrentIssue(); iss != nil {
				a.link = linkview.New(iss.Key)
				a.link.SetSize(a.width, a.height-2)
				a.active = viewLink
				return a, a.fetchLinkTypes()
			}
		case key.Matches(msg, a.keys.Delete) && a.active == viewIssue:
			if iss := a.issue.CurrentIssue(); iss != nil {
				a.del = deleteview.New(iss.Key, iss.Summary)
				a.del.SetSize(a.width, a.height-2)
				a.active = viewDelete
				return a, nil
			}
		case key.Matches(msg, a.keys.Parent) && a.active == viewIssue:
			if iss := a.issue.CurrentIssue(); iss != nil && a.issue.HasParent() {
				a.issueStack = append(a.issueStack, *iss)
				placeholder := jira.Issue{Key: iss.ParentKey, Summary: "Loading..."}
				a.issue = a.issue.SetIssue(placeholder)
				a.issue.SetIssueURL(a.client.IssueURL(iss.ParentKey))
				return a, a.fetchIssueBundle(iss.ParentKey)
			}
		case key.Matches(msg, a.keys.IssuePick) && a.active == viewIssue:
			refs := a.issue.IssueKeys()
			if len(refs) > 0 {
				a.issuePick = issuepickview.New(refs)
				a.issuePick.SetSize(a.width, a.height-2)
				a.issuePickOrigin = viewIssue
				a.active = viewIssuePick
				return a, nil
			}
		case key.Matches(msg, a.keys.Pages) && a.active == viewConfluence:
			if page := a.confPage.CurrentPage(); page != nil {
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
					return a, nil
				}
			}
		case key.Matches(msg, a.keys.Filters) &&
			(a.active == viewHome || a.active == viewSprint || a.active == viewBoard || a.active == viewSearchBoard):
			a.filter.Reset()
			a.filter.SetFilters(a.savedFilters)
			a.filterOrigin = a.active
			a.previousView = a.active
			a.active = viewFilters
			return a, nil
		case key.Matches(msg, a.keys.Profile) &&
			(a.active == viewHome || a.active == viewSprint || a.active == viewBoard):
			profiles, err := config.ListProfileNames()
			if err != nil {
				return a, nil
			}
			if len(profiles) == 0 {
				// No profiles.yml yet — show single "default" entry.
				profiles = []string{"default"}
			}
			a.profile = profileview.New(profiles, a.profileName)
			a.profile.SetSize(a.width, a.height-2)
			a.profileOrigin = a.active
			a.active = viewProfile
			return a, nil
		case key.Matches(msg, a.keys.Refresh) && (a.active == viewSprint || a.active == viewBoard):
			a.previousView = a.active
			a.active = viewLoading
			a.statusMsg = "Refreshing..."
			a.paginationSeq++
			return a, tea.Batch(a.spinner.Tick, a.fetchActiveSprintForBoard(a.boardID))
		case key.Matches(msg, a.keys.Refresh) && a.active == viewSearchBoard:
			a.statusMsg = "Refreshing..."
			a.paginationSeq++
			return a, a.searchJQL(a.searchBoardTitle)
		case key.Matches(msg, a.keys.Refresh) && a.active == viewHome:
			a.statusMsg = "Refreshing..."
			a.active = viewLoading
			return a, tea.Batch(a.spinner.Tick, a.fetchBoards())
		case key.Matches(msg, a.keys.Refresh) && a.active == viewConfluence:
			if page := a.confPage.CurrentPage(); page != nil {
				a.statusMsg = "Refreshing..."
				return a, a.fetchConfluencePage(page.ID)
			}
		case key.Matches(msg, a.keys.Refresh) && a.active == viewIssue:
			if iss := a.issue.CurrentIssue(); iss != nil {
				a.statusMsg = "Refreshing..."
				return a, a.fetchIssueBundle(iss.Key)
			}
		}

	case ClientReadyMsg:
		a.err = nil
		a.statusMsg = fmt.Sprintf("Authenticated as %s", msg.DisplayName)
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
		return a, tea.Batch(a.fetchBoards(), metaCmd)

	case SprintLoadedMsg:
		a.err = nil
		a.statusMsg = msg.Sprint.Name
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
		a.sprint = a.sprint.SetIssues(msg.Issues)
		if msg.HasMore {
			a.sprint = a.sprint.SetLoading(true)
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
		a.board.SetIssues(msg.Issues, msg.Title)
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
			a.sprint = a.sprint.AppendIssues(msg.Issues)
		}
		if msg.HasMore {
			return a, a.fetchMoreIssues(msg)
		}
		// All pages loaded — populate the board once with the full dataset.
		a.board.SetIssues(a.currentIssues, a.boardTitle)
		a.sprint = a.sprint.SetLoading(false)
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

	case BoardsLoadedMsg:
		a.err = nil
		a.home.SetBoards(msg.Boards)
		a.active = viewHome
		a.statusMsg = ""
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
		a.active = viewSearch
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
		a.search.SetMetadata(msg.Meta)
		a.jqlMetaLoaded = true
		a.jqlMeta = msg.Meta
		if msg.Meta != nil {
			if len(msg.Meta.Statuses) > 0 {
				a.board.SetKnownStatuses(msg.Meta.Statuses)
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
		a.search.SetUserResults(msg.Names)
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
				a.sprint = a.sprint.UpdateIssueStatus(msg.Key, msg.NewStatus)
				// Update search issue cache.
				for i, iss := range a.searchIssues {
					if iss.Key == msg.Key {
						a.searchIssues[i].Status = msg.NewStatus
					}
				}
				// Update board view in-place so the cursor follows the card.
				if a.transitionOrigin == viewBoard || a.transitionOrigin == viewSearchBoard {
					a.board.UpdateIssueStatus(msg.Key, msg.NewStatus)
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
			a.active = viewIssue
			if msg.Err != nil {
				a.err = sanitiseError(msg.Err)
			} else {
				a.statusMsg = fmt.Sprintf("Linked %s → %s", msg.SourceKey, msg.TargetKey)
				return a, a.fetchIssueDetail(msg.SourceKey)
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
					a.board.SetIssues(a.searchIssues, a.searchBoardDisplayTitle())
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

	case ProfileSwitchedMsg:
		a.client = msg.Client
		a.profileName = msg.Name
		a.home = homeview.New()
		a.sprint = sprintview.New()
		a.issue = issueview.New()
		a.search = searchview.New()
		a.board = boardview.New()
		a.spaces = spacesview.New()
		a.confPage = confluenceview.New()
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
		a.filter = filterview.New()
		a.filter.SetFilters(a.savedFilters)

		// Re-apply sizes.
		contentHeight := a.height - 3
		a.home.SetSize(a.width, contentHeight)
		a.sprint = a.sprint.SetSize(a.width, contentHeight)
		a.issue = a.issue.SetSize(a.width, contentHeight)
		a.search.SetSize(a.width, contentHeight)
		a.board.SetSize(a.width, contentHeight)
		a.filter.SetSize(a.width, contentHeight)
		a.spaces.SetSize(a.width, contentHeight)
		a.confPage = a.confPage.SetSize(a.width, contentHeight)

		a.statusMsg = fmt.Sprintf("Switched to profile: %s", msg.Name)
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
			}
		}
		return a, nil

	case OpenURLMsg:
		openBrowser(msg.URL)
		return a, nil

	case ErrMsg:
		a.err = sanitiseError(msg.Err)
		// Clear loading state — pagination errors should not leave the UI stuck.
		a.sprint = a.sprint.SetLoading(false)
		return a, nil

	case SpacesLoadedMsg:
		a.spaces.SetSpaces(msg.Spaces)
		a.cachedSpaces = msg.Spaces
		// Load recents.
		if entries, err := recents.Load(); err == nil && len(entries) > 0 {
			a.spaces.SetRecents(recents.Sorted(entries))
		}
		return a, nil

	case SpacePagesLoadedMsg:
		a.spaces.SetPages(msg.Pages)
		return a, nil

	case ConfluencePageLoadedMsg:
		a.confPage.SetPage(msg.Page)
		a.confPage.SetAncestors(msg.Ancestors)
		if msg.SpaceKey != "" {
			a.confPage.SetSpaceKey(msg.SpaceKey)
		}
		a.active = viewConfluence
		// Record in recents.
		if msg.Page != nil {
			_ = recents.Add(msg.Page.ID, msg.Page.Title, msg.SpaceKey)
		}
		return a, nil

	case RemoteLinksLoadedMsg:
		// TODO: display linked Confluence pages in the issue view.
		return a, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		a.spinner, cmd = a.spinner.Update(msg)
		return a, cmd
	}

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
			a.active = viewLoading
			return a, tea.Batch(a.spinner.Tick, a.verifyAuth())
		}
		return a, cmd
	case viewHome:
		a.home, cmd = a.home.Update(msg)
		if b := a.home.SelectedBoard(); b != nil {
			a.boardID = b.ID
			a.statusMsg = fmt.Sprintf("Loading %s...", b.Name)
			a.previousView = viewHome
			a.active = viewLoading
			a.paginationSeq++
			return a, tea.Batch(cmd, a.fetchActiveSprintForBoard(b.ID))
		}
	case viewSprint:
		a.sprint, cmd = a.sprint.Update(msg)

		// Check if sprint view wants to open an issue.
		if iss, ok := a.sprint.SelectedIssue(); ok {
			a.active = viewIssue
			a.previousView = viewSprint
			a.issue = a.issue.SetIssue(iss)
			a.issue.SetIssueURL(a.client.IssueURL(iss.Key))
			// Fetch full details and children in background.
			return a, tea.Batch(cmd, a.fetchIssueBundle(iss.Key))
		}
	case viewBoard, viewSearchBoard:
		a.board, cmd = a.board.Update(msg)
		if iss, ok := a.board.SelectedIssue(); ok {
			a.previousView = a.active // Preserves viewBoard or viewSearchBoard.
			a.active = viewIssue
			a.issue = a.issue.SetIssue(iss)
			a.issue.SetIssueURL(a.client.IssueURL(iss.Key))
			return a, tea.Batch(cmd, a.fetchIssueBundle(iss.Key))
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
			return a, a.fetchIssueBundle(key)
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
			a.active = viewIssue
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
			// Check if this is a Confluence page ID (all digits) from the page picker.
			if a.issuePickOrigin == viewConfluence && isPageID(ref.Key) {
				// Push current page onto stack for back-navigation.
				if page := a.confPage.CurrentPage(); page != nil {
					a.pageStack = append(a.pageStack, *page)
				}
				a.confPage = confluenceview.New()
				a.confPage = a.confPage.SetSize(a.width, a.height-4)
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
			return a, a.fetchIssueBundle(ref.Key)
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
			a.setup.SetSize(a.width, a.height)
			a.needsSetup = true
			a.previousView = a.profileOrigin
			a.active = viewSetup
			return a, a.setup.Init()
		}
		if a.profile.Dismissed() {
			a.active = a.profileOrigin
		}
	case viewFilters:
		a.filter, cmd = a.filter.Update(msg)

		// Apply a filter → run JQL search.
		// Stay on viewFilters while loading — SearchResultsMsg transitions to viewSearch.
		if f := a.filter.Applied(); f != nil {
			a.searchOrigin = viewFilters
			a.search.SetFilterName(f.Name)
			a.statusMsg = "Searching..."
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
					a.filter.SetFilters(a.savedFilters)
				}
			}
		}

		// Dismissed — go back.
		if a.filter.Dismissed() {
			a.active = a.filterOrigin
			return a, cmd
		}
		return a, cmd
	case viewSpaces:
		a.spaces, cmd = a.spaces.Update(msg)
		if p := a.spaces.SelectedPage(); p != nil {
			a.confPage = confluenceview.New()
			a.confPage = a.confPage.SetSize(a.width, a.height-4)
			return a, a.fetchConfluencePage(p.ID)
		}
		if fetchID := a.spaces.NeedsFetch(); fetchID != "" {
			return a, a.fetchSpacePages(fetchID)
		}
		if a.spaces.Dismissed() {
			a.confirmQuit = true
		}
	case viewConfluence:
		a.confPage, cmd = a.confPage.Update(msg)
		if url, ok := a.confPage.OpenURL(); ok {
			if a.client != nil {
				openBrowser(a.client.ConfluencePageURL(strings.TrimPrefix(url, "page/")))
			}
		}
		if a.confPage.Dismissed() {
			a.active = viewSpaces
		}
	case viewSearch:
		a.search, cmd = a.search.Update(msg)
		if prefix := a.search.NeedsUserSearch(); prefix != "" {
			cmd = tea.Batch(cmd, a.searchUsers(prefix))
		}
		if q := a.search.SubmittedQuery(); q != "" {
			a.statusMsg = "Searching..."
			a.paginationSeq++
			return a, tea.Batch(cmd, a.searchJQL(q))
		}
		if jql := a.search.SaveFilter(); jql != "" {
			a.filter.Reset()
			a.filter.SetFilters(a.savedFilters)
			a.filter.StartAdd(jql)
			a.filterOrigin = a.active
			a.previousView = a.active
			a.active = viewFilters
			return a, cmd
		}
		if iss := a.search.SelectedIssue(); iss != nil {
			a.issueStack = nil
			a.search.Hide()
			a.previousView = viewSearch
			a.active = viewIssue
			return a, tea.Batch(cmd, a.fetchIssueBundle(iss.Key))
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

func (a App) View() string {
	if a.width == 0 {
		return "Loading..."
	}

	var content string

	switch a.active {
	case viewSetup:
		content = a.setup.View()
	case viewLoading:
		msg := a.statusMsg
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
	case viewHome:
		content = a.home.View()
	case viewSprint:
		content = a.sprint.View()
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
	case viewSpaces:
		content = a.spaces.View()
	case viewConfluence:
		content = a.confPage.View()
	}

	if a.err != nil {
		errBox := theme.StyleErrorDialog.Width(a.width / 2).Render(
			lipgloss.JoinVertical(lipgloss.Center,
				theme.StyleError.Render("Error"),
				"",
				theme.StyleSubtle.Render(a.err.Error()),
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
	case viewSearch:
		if a.search.ShowingResults() {
			extra = append(extra, footerBinding{"enter", "open"})
			if a.searchOrigin != viewFilters {
				extra = append(extra, footerBinding{"s", "save filter"})
			}
			extra = append(extra,
				footerBinding{"b", "board view"},
				footerBinding{"r", "refresh"},
				footerBinding{"/", "filter"},
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
	if a.statusMsg != "" && a.active != viewLoading {
		style := lipgloss.NewStyle().Foreground(theme.ColourSuccess)
		if a.err != nil {
			style = lipgloss.NewStyle().Foreground(theme.ColourError)
		}
		status := style.Render(a.statusMsg)
		return lipgloss.JoinVertical(lipgloss.Left, content, status, help)
	}

	return lipgloss.JoinVertical(lipgloss.Left, content, help)
}

// inputActive reports whether a text input is focused (search overlay, list filter, or setup wizard).
func (a App) inputActive() bool {
	if a.active == viewSetup && a.setup.InputActive() {
		return true
	}
	if a.active == viewSearch && a.search.InputActive() {
		return true
	}
	if a.active == viewCreate && a.create.InputActive() {
		return true
	}
	if a.active == viewSprint && a.sprint.Filtering() {
		return true
	}
	if a.active == viewHome && a.home.Filtering() {
		return true
	}
	if a.active == viewBranch && a.branch.InputActive() {
		return true
	}
	if a.active == viewTransition && a.transition.InputActive() {
		return true
	}
	if a.active == viewComment && a.comment.InputActive() {
		return true
	}
	if a.active == viewFilters && a.filter.InputActive() {
		return true
	}
	if a.active == viewAssign && a.assign.InputActive() {
		return true
	}
	if a.active == viewEdit && a.edit.InputActive() {
		return true
	}
	if a.active == viewLink && a.link.InputActive() {
		return true
	}
	if a.active == viewDelete && a.del.InputActive() {
		return true
	}
	if a.active == viewIssuePick && a.issuePick.InputActive() {
		return true
	}
	if a.active == viewProfile && a.profile.InputActive() {
		return true
	}
	if a.active == viewSpaces && a.spaces.Filtering() {
		return true
	}
	return false
}

// currentConfig returns the current config for pre-filling the setup wizard.
func (a App) currentConfig() *config.Config {
	if a.client != nil {
		return a.client.Config()
	}
	return nil
}

// isBackKey returns true if the key should trigger back-navigation.
func (a App) isBackKey(msg tea.KeyMsg) bool {
	k := msg.String()
	return k == "esc" || k == "q"
}

// navigateBack moves to the parent view, or quits if already at the top level.
func (a App) navigateBack() (tea.Model, tea.Cmd) {
	switch a.active {
	case viewFilters:
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
		a.active = viewIssue
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
	case viewIssue:
		if len(a.issueStack) > 0 {
			prev := a.issueStack[len(a.issueStack)-1]
			a.issueStack = a.issueStack[:len(a.issueStack)-1]
			a.issue = a.issue.SetIssue(prev)
			a.issue.SetIssueURL(a.client.IssueURL(prev.Key))
			return a, a.fetchIssueBundle(prev.Key)
		}
		switch a.previousView {
		case viewSearch:
			a.search.Reshow()
			a.active = viewSearch
			a.previousView = a.searchOrigin
		case viewSearchBoard:
			a.board.SetIssues(a.searchIssues, a.searchBoardDisplayTitle())
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
		if a.sprint.Filtered() {
			a.sprint = a.sprint.ResetFilter()
			return a, nil
		}
		if a.client.Config().BoardID == 0 {
			a.active = viewHome
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
		case viewHome, viewSprint, viewBoard:
			a.active = a.previousView
			return a, nil
		}
		// Initial load or no meaningful previous view — quit.
		return a, tea.Quit
	case viewHome:
		if a.home.Filtered() {
			a.home.ResetFilter()
			return a, nil
		}
		a.confirmQuit = true
		return a, nil
	case viewSpaces:
		// Navigate within confluence — esc/q stays in wiki mode.
		if a.spaces.Filtered() {
			a.spaces.ResetFilter()
			return a, nil
		}
		if a.spaces.InPagesState() {
			a.spaces.GoToSpaces()
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
			a.confPage.SetPage(&prev)
			return a, a.fetchConfluencePage(prev.ID)
		}
		a.active = viewSpaces
		return a, nil
	}
	return a, nil
}

// Commands.

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

func (a App) fetchBoards() tea.Cmd {
	return func() tea.Msg {
		// Load status category metadata first so issue counts are accurate.
		// Without this, custom statuses (e.g. "Code Review") all count as "open".
		if meta, err := a.client.JQLMetadata(); err == nil && meta != nil {
			theme.SetStatusCategoryMap(meta.StatusCategories)
		}

		project := a.client.Config().Project
		boards, err := a.client.Boards(project)
		if err != nil {
			return ErrMsg{Err: err}
		}
		stats := make([]jira.BoardStats, len(boards))
		for i, b := range boards {
			stats[i] = jira.BoardStats{Board: b}
			sprints, err := a.client.BoardSprints(b.ID, "active")
			if err != nil || len(sprints) == 0 {
				continue
			}
			stats[i].ActiveSprint = sprints[0].Name
			open, inProg, done, total, err := a.client.SprintIssueStats(sprints[0].ID)
			if err != nil {
				continue
			}
			stats[i].OpenIssues = open
			stats[i].InProgress = inProg
			stats[i].DoneIssues = done
			stats[i].TotalIssues = total
		}
		return BoardsLoadedMsg{Boards: stats}
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

func (a App) searchUsersForAssign(prefix string) tea.Cmd {
	return func() tea.Msg {
		users, err := a.client.SearchUsers(a.client.Config().Project, prefix)
		if err != nil {
			return AssignUserSearchMsg{Users: nil}
		}
		return AssignUserSearchMsg{Users: users}
	}
}

func (a App) assignIssue(key string, req *assignview.AssignRequest) tea.Cmd {
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

func (a App) linkIssue(req *linkview.LinkRequest) tea.Cmd {
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
// detail, children, and branch info (if a repo path is configured).
func (a App) fetchIssueBundle(key string) tea.Cmd {
	cmds := []tea.Cmd{a.fetchIssueDetail(key), a.fetchChildIssues(key)}
	if branchCmd := a.fetchBranchInfo(key); branchCmd != nil {
		cmds = append(cmds, branchCmd)
	}
	return tea.Batch(cmds...)
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
				"rev-list", "--count", "HEAD.."+name).CombinedOutput()
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

// reloadSavedFilters refreshes the in-memory filter list from disk.
func reloadSavedFilters(a *App) {
	if fs, err := filters.Load(); err == nil {
		a.savedFilters = filters.Sorted(fs)
	}
	a.filter.SetFilters(a.savedFilters)
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

// searchBoardDisplayTitle returns the display title for the search board view.
// Uses the saved filter name when available, otherwise falls back to the raw JQL.
func (a *App) searchBoardDisplayTitle() string {
	if name := a.search.FilterName(); name != "" {
		return "Filter: " + name
	}
	return a.searchBoardTitle
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

func resolveSpaceKey(spaceID string, spaces []confluence.Space) string {
	for _, s := range spaces {
		if s.ID == spaceID {
			return s.Key
		}
	}
	return ""
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
