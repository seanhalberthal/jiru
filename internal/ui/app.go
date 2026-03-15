package ui

import (
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strings"

	"al.essio.dev/pkg/shellescape"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/seanhalberthal/jiru/internal/client"
	"github.com/seanhalberthal/jiru/internal/config"
	"github.com/seanhalberthal/jiru/internal/jira"
	"github.com/seanhalberthal/jiru/internal/theme"
	"github.com/seanhalberthal/jiru/internal/ui/boardview"
	"github.com/seanhalberthal/jiru/internal/ui/branchview"
	"github.com/seanhalberthal/jiru/internal/ui/commentview"
	"github.com/seanhalberthal/jiru/internal/ui/createview"
	"github.com/seanhalberthal/jiru/internal/ui/homeview"
	"github.com/seanhalberthal/jiru/internal/ui/issueview"
	"github.com/seanhalberthal/jiru/internal/ui/searchview"
	"github.com/seanhalberthal/jiru/internal/ui/setupview"
	"github.com/seanhalberthal/jiru/internal/ui/sprintview"
	"github.com/seanhalberthal/jiru/internal/ui/transitionview"
	"github.com/seanhalberthal/jiru/internal/validate"
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
	viewBoard
	viewBranch
	viewCreate
	viewTransition
	viewComment
)

// App is the root bubbletea model.
type App struct {
	client        client.JiraClient
	keys          KeyMap
	active        view
	previousView  view
	home          homeview.Model
	sprint        sprintview.Model
	issue         issueview.Model
	search        searchview.Model
	board         boardview.Model
	branch        branchview.Model
	create        createview.Model
	transition    transitionview.Model
	comment       commentview.Model
	setup         setupview.Model
	spinner       spinner.Model
	width         int
	height        int
	statusMsg     string
	err           error
	boardID       int
	directIssue   string
	needsSetup    bool
	currentIssues []jira.Issue // Cached for list↔board toggle.
	boardTitle    string       // Dynamic title: sprint name, board name, project key, etc.
	jqlMetaLoaded bool         // Prevents redundant metadata fetches.
	paginationSeq int          // Incremented each time a new fetch starts; stale pages are discarded.
	version       string
}

// NewApp creates a new root application model.
// If missing is non-empty, the setup wizard is shown instead of normal loading.
func NewApp(c client.JiraClient, directIssue string, partial *config.Config, missing []string, version string) App {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(theme.ColourPrimary)

	app := App{
		client:      c,
		keys:        DefaultKeyMap(),
		active:      viewLoading,
		home:        homeview.New(),
		sprint:      sprintview.New(),
		issue:       issueview.New(),
		search:      searchview.New(),
		board:       boardview.New(),
		spinner:     s,
		directIssue: directIssue,
		version:     version,
	}

	if len(missing) > 0 {
		app.needsSetup = true
		app.setup = setupview.New(partial)
		app.active = viewSetup
	}

	return app
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
		contentHeight := msg.Height - 2 // Reserve for help bar.
		a.sprint = a.sprint.SetSize(msg.Width, contentHeight)
		a.issue = a.issue.SetSize(msg.Width, contentHeight)
		a.home.SetSize(msg.Width, contentHeight)
		a.search.SetSize(msg.Width, contentHeight)
		a.board.SetSize(msg.Width, contentHeight)
		a.branch.SetSize(msg.Width, contentHeight)
		a.transition.SetSize(msg.Width, contentHeight)
		a.comment.SetSize(msg.Width, contentHeight)
		a.setup.SetSize(msg.Width, msg.Height)
		a.create.SetSize(msg.Width, msg.Height)
		return a, nil

	case tea.KeyMsg:
		// Clear status message on any keypress.
		a.statusMsg = ""

		// ctrl+c always quits, regardless of input state.
		if msg.String() == "ctrl+c" {
			return a, tea.Quit
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
		case key.Matches(msg, a.keys.Search) && !a.search.Visible() && a.active != viewLoading:
			a.search.Show()
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
		case key.Matches(msg, a.keys.Home) && a.active != viewHome && a.active != viewLoading:
			a.active = viewHome
			return a, nil
		case key.Matches(msg, a.keys.Board) && a.active == viewSprint:
			a.board.SetIssues(a.currentIssues, a.boardTitle)
			a.active = viewBoard
			return a, nil
		case key.Matches(msg, a.keys.Board) && a.active == viewBoard:
			a.active = viewSprint
			return a, nil
		case msg.String() == "e" && a.active == viewBoard:
			groups := a.board.ParentGroups()
			current := a.board.ParentFilter()
			next := cycleParentFilter(groups, current)
			a.board.SetParentFilter(next)
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
		case key.Matches(msg, a.keys.Transition) && (a.active == viewIssue || a.active == viewBoard):
			var issueKey string
			switch a.active {
			case viewIssue:
				if iss := a.issue.CurrentIssue(); iss != nil {
					issueKey = iss.Key
				}
			case viewBoard:
				if iss, ok := a.board.HighlightedIssue(); ok {
					issueKey = iss.Key
				}
			}
			if issueKey != "" {
				a.transition = transitionview.New(issueKey)
				a.transition.SetSize(a.width, a.height-2)
				a.previousView = a.active
				a.active = viewTransition
				return a, a.fetchTransitions(issueKey)
			}
		case key.Matches(msg, a.keys.Comment) && a.active == viewIssue:
			if iss := a.issue.CurrentIssue(); iss != nil {
				a.comment = commentview.New(iss.Key)
				a.comment.SetSize(a.width, a.height-2)
				a.previousView = a.active
				a.active = viewComment
				return a, nil
			}
		case key.Matches(msg, a.keys.Refresh) && a.active == viewSprint:
			a.previousView = viewSprint
			a.active = viewLoading
			a.statusMsg = "Refreshing..."
			a.paginationSeq++
			return a, tea.Batch(a.spinner.Tick, a.fetchActiveSprintForBoard(a.boardID))
		}

	case ClientReadyMsg:
		a.err = nil
		a.statusMsg = fmt.Sprintf("Authenticated as %s", msg.DisplayName)
		if a.directIssue != "" {
			return a, a.fetchIssueDetail(a.directIssue)
		}
		if a.client.Config().BoardID != 0 {
			a.boardID = a.client.Config().BoardID
			a.paginationSeq++
			return a, a.fetchActiveSprintForBoard(a.boardID)
		}
		return a, a.fetchBoards()

	case SprintLoadedMsg:
		a.err = nil
		a.statusMsg = msg.Sprint.Name
		a.paginationSeq++
		return a, a.fetchSprintIssues(msg.Sprint.ID, msg.Sprint.Name)

	case IssuesLoadedMsg:
		a.err = nil
		a.active = viewSprint
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
				NextToken:  msg.NextToken,
				Seq:        msg.Seq,
			})
		}
		return a, nil

	case IssuesPageMsg:
		if msg.Seq != a.paginationSeq {
			return a, nil // Stale page from a previous fetch — discard.
		}
		switch msg.Source {
		case "search":
			a.search.AppendResults(msg.Issues)
		default:
			a.currentIssues = append(a.currentIssues, msg.Issues...)
			a.sprint = a.sprint.AppendIssues(msg.Issues)
			a.board.AppendIssues(msg.Issues)
		}
		if msg.HasMore {
			return a, a.fetchMoreIssues(msg)
		}
		a.sprint = a.sprint.SetLoading(false)
		return a, nil

	case IssueSelectedMsg:
		a.active = viewIssue
		a.issue = a.issue.SetIssue(msg.Issue)
		return a, nil

	case IssueDetailMsg:
		// Update the issue view with full details if we're still viewing it.
		if a.active == viewIssue && msg.Issue != nil {
			a.issue = a.issue.SetIssue(*msg.Issue)
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
		a.search.SetResults(msg.Issues, msg.Query)
		a.active = viewSearch
		a.statusMsg = ""
		if msg.HasMore {
			return a, a.fetchMoreIssues(IssuesPageMsg{
				Source:    "search",
				From:      msg.From,
				JQL:       msg.Query,
				NextToken: msg.NextToken,
				Seq:       msg.Seq,
			})
		}
		return a, nil

	case JQLMetadataMsg:
		a.search.SetMetadata(msg.Meta)
		a.jqlMetaLoaded = true
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
			a.active = a.previousView
			if msg.Err != nil {
				a.err = msg.Err
			} else {
				a.statusMsg = fmt.Sprintf("Moved to %s", msg.NewStatus)
				var cmds []tea.Cmd
				if a.previousView == viewIssue {
					// Re-fetch issue details to reflect the new status.
					cmds = append(cmds, a.fetchIssueDetail(msg.Key))
				}
				if a.previousView == viewBoard || a.previousView == viewSprint {
					// Refresh the board/sprint to reflect the status change.
					cmds = append(cmds, a.refreshCurrentView())
				}
				if len(cmds) > 0 {
					return a, tea.Batch(cmds...)
				}
			}
		}
		return a, nil

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
			if err := config.WriteConfig(cfg); err != nil {
				a.err = sanitiseError(fmt.Errorf("failed to save config: %w", err))
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
			// Fetch full details in background.
			return a, tea.Batch(cmd, a.fetchIssueDetail(iss.Key))
		}
	case viewBoard:
		a.board, cmd = a.board.Update(msg)
		if iss, ok := a.board.SelectedIssue(); ok {
			a.active = viewIssue
			a.previousView = viewBoard
			a.issue = a.issue.SetIssue(iss)
			a.issue.SetIssueURL(a.client.IssueURL(iss.Key))
			return a, tea.Batch(cmd, a.fetchIssueDetail(iss.Key))
		}
	case viewIssue:
		a.issue, cmd = a.issue.Update(msg)
		if url, ok := a.issue.OpenURL(); ok {
			openBrowser(url)
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
			return a, a.fetchIssueDetail(key)
		}
	case viewTransition:
		a.transition, cmd = a.transition.Update(msg)
		if t := a.transition.Selected(); t != nil {
			return a, a.transitionIssue(a.transition.IssueKey(), t.ID, t.Name)
		}
		if a.transition.Dismissed() {
			a.active = a.previousView
		}
	case viewComment:
		a.comment, cmd = a.comment.Update(msg)
		if body := a.comment.SubmittedComment(); body != "" {
			return a, a.addComment(a.comment.IssueKey(), body)
		}
		if a.comment.Dismissed() {
			a.active = viewIssue
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
		if iss := a.search.SelectedIssue(); iss != nil {
			a.search.Hide()
			a.active = viewIssue
			return a, tea.Batch(cmd, a.fetchIssueDetail(iss.Key))
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
	case viewBoard:
		content = a.board.View()
	case viewBranch:
		content = a.branch.View()
	case viewTransition:
		content = a.transition.View()
	case viewComment:
		content = a.comment.View()
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

	// Build footer with optional board-view extras.
	var extra []footerBinding
	if a.active == viewBoard {
		extra = append(extra, footerBinding{"e", "filter " + a.board.ParentLabel()})
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
	case viewTransition:
		a.active = a.previousView
		return a, nil
	case viewComment:
		a.active = viewIssue
		return a, nil
	case viewBranch:
		a.active = viewIssue
		return a, nil
	case viewIssue:
		if a.previousView == viewBoard {
			a.active = viewBoard
		} else {
			a.active = viewSprint
		}
		return a, nil
	case viewBoard:
		a.active = viewSprint
		return a, nil
	case viewSprint:
		if a.client.Config().BoardID == 0 {
			a.active = viewHome
			return a, nil
		}
		return a, tea.Quit
	case viewCreate:
		a.active = a.previousView
		return a, nil
	case viewSearch:
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
		return a, tea.Quit
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
		page, err := a.client.SprintIssuesPage(sprintID, 0, client.DefaultPageSize)
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

		for range pagesPerBatch {
			var page *client.PageResult
			var err error

			switch msg.Source {
			case "sprint":
				page, err = a.client.SprintIssuesPage(msg.SprintID, from, client.DefaultPageSize)
			case "board", "search":
				page, err = a.client.SearchJQLPage(msg.JQL, client.DefaultPageSize, nextToken)
			case "epic":
				page, err = a.client.EpicIssuesPage(msg.EpicKey, from, client.DefaultPageSize)
			}

			if err != nil {
				return ErrMsg{Err: err}
			}

			allIssues = append(allIssues, page.Issues...)
			from += len(page.Issues)
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
			Source:     msg.Source,
			From:       from,
			SprintID:   msg.SprintID,
			SprintName: msg.SprintName,
			EpicKey:    msg.EpicKey,
			JQL:        msg.JQL,
			Project:    msg.Project,
			NextToken:  nextToken,
			Seq:        msg.Seq,
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

func (a App) fetchBoards() tea.Cmd {
	return func() tea.Msg {
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
		page, err := a.client.SearchJQLPage(jql, client.DefaultPageSize, "")
		if err != nil {
			return ErrMsg{Err: err}
		}
		return SearchResultsMsg{
			Issues:    page.Issues,
			Query:     jql,
			HasMore:   page.HasMore,
			From:      len(page.Issues),
			NextToken: page.NextToken,
			Seq:       seq,
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

func (a App) transitionIssue(key, transitionID, transitionName string) tea.Cmd {
	return func() tea.Msg {
		err := a.client.TransitionIssue(key, transitionID)
		if err != nil {
			return IssueTransitionedMsg{Key: key, Err: err}
		}
		return IssueTransitionedMsg{Key: key, NewStatus: transitionName}
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

		// No active sprint — fetch board issues via JQL with progressive pagination.
		project := a.client.Config().Project
		if err := validate.ProjectKey(project); err != nil {
			return ErrMsg{Err: fmt.Errorf("no active iteration and invalid project key: %w", err)}
		}
		escapedProject := jqlEscape(project)
		jql := fmt.Sprintf(
			"project = '%s' AND statusCategory != Done ORDER BY status ASC, updated DESC",
			escapedProject,
		)

		page, fetchErr := a.client.SearchJQLPage(jql, client.DefaultPageSize, "")
		if fetchErr != nil {
			return ErrMsg{Err: fmt.Errorf("no active iteration and board issue fetch failed: %w", fetchErr)}
		}

		parents := a.client.ResolveParents(page.Issues)
		enriched := client.EnrichWithParents(page.Issues, parents)

		return IssuesLoadedMsg{
			Issues:    enriched,
			Title:     "Board",
			HasMore:   page.HasMore,
			Source:    "board",
			From:      len(page.Issues),
			JQL:       jql,
			NextToken: page.NextToken,
			Project:   project,
			Seq:       seq,
		}
	}
}

// cycleParentFilter returns the next parent key in the list, or "" to clear.
func cycleParentFilter(groups []boardview.ParentGroup, current string) string {
	if current == "" && len(groups) > 0 {
		return groups[0].Key
	}
	for i, g := range groups {
		if g.Key == current {
			if i+1 < len(groups) {
				return groups[i+1].Key
			}
			return "" // Wrap around to clear filter.
		}
	}
	return ""
}

// jqlEscape escapes single quotes in JQL string literals.
func jqlEscape(s string) string {
	return strings.ReplaceAll(s, `'`, `\'`)
}

var urlPattern = regexp.MustCompile(`https?://\S+`)

// sanitiseError strips URL-like content from error messages to prevent
// leaking API endpoints, tokens, or internal server details to the terminal.
func sanitiseError(err error) error {
	msg := err.Error()
	clean := urlPattern.ReplaceAllString(msg, "[url redacted]")
	return fmt.Errorf("%s", clean)
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
