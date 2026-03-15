package ui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

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
	"github.com/seanhalberthal/jiru/internal/ui/homeview"
	"github.com/seanhalberthal/jiru/internal/ui/issueview"
	"github.com/seanhalberthal/jiru/internal/ui/searchview"
	"github.com/seanhalberthal/jiru/internal/ui/setupview"
	"github.com/seanhalberthal/jiru/internal/ui/sprintview"
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
}

// NewApp creates a new root application model.
// If missing is non-empty, the setup wizard is shown instead of normal loading.
func NewApp(c client.JiraClient, directIssue string, partial *config.Config, missing []string) App {
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
		a.setup.SetSize(msg.Width, msg.Height)
		return a, nil

	case tea.KeyMsg:
		// Clear status message on any keypress.
		a.statusMsg = ""

		// ctrl+c always quits, regardless of input state.
		if msg.String() == "ctrl+c" {
			return a, tea.Quit
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
		case key.Matches(msg, a.keys.Refresh) && a.active == viewSprint:
			a.active = viewLoading
			a.statusMsg = "Refreshing..."
			return a, tea.Batch(a.spinner.Tick, a.fetchActiveSprint())
		}

	case ClientReadyMsg:
		a.statusMsg = fmt.Sprintf("Authenticated as %s", msg.DisplayName)
		if a.directIssue != "" {
			return a, a.fetchIssueDetail(a.directIssue)
		}
		if a.client.Config().BoardID != 0 {
			a.boardID = a.client.Config().BoardID
			return a, a.fetchActiveSprintForBoard(a.boardID)
		}
		return a, a.fetchBoards()

	case SprintLoadedMsg:
		a.statusMsg = msg.Sprint.Name
		return a, a.fetchSprintIssues(msg.Sprint.ID, msg.Sprint.Name)

	case IssuesLoadedMsg:
		a.active = viewSprint
		a.currentIssues = msg.Issues
		a.boardTitle = msg.Title
		a.sprint = a.sprint.SetIssues(msg.Issues)
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
		a.home.SetBoards(msg.Boards)
		a.active = viewHome
		a.statusMsg = ""
		return a, nil

	case SearchResultsMsg:
		a.search.SetResults(msg.Issues, msg.Query)
		a.active = viewSearch
		a.statusMsg = ""
		return a, nil

	case JQLMetadataMsg:
		a.search.SetMetadata(msg.Meta)
		a.jqlMetaLoaded = true
		return a, nil

	case UserSearchMsg:
		a.search.SetUserResults(msg.Names)
		return a, nil

	case BranchCreatedMsg:
		if a.active == viewBranch {
			a.active = viewIssue
			if msg.Err != nil {
				a.err = msg.Err
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
		a.err = msg.Err
		a.active = viewSprint
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
				a.err = fmt.Errorf("failed to save config: %w", err)
				a.active = viewLoading
				return a, nil
			}
			// Create client and proceed to normal loading.
			a.client = client.New(cfg)
			a.needsSetup = false
			a.active = viewLoading
			return a, tea.Batch(a.spinner.Tick, a.verifyAuth())
		}
		return a, cmd
	case viewHome:
		a.home, cmd = a.home.Update(msg)
		if b := a.home.SelectedBoard(); b != nil {
			a.boardID = b.ID
			a.statusMsg = fmt.Sprintf("Loading %s...", b.Name)
			a.active = viewLoading
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
	case viewSearch:
		a.search, cmd = a.search.Update(msg)
		if prefix := a.search.NeedsUserSearch(); prefix != "" {
			cmd = tea.Batch(cmd, a.searchUsers(prefix))
		}
		if q := a.search.SubmittedQuery(); q != "" {
			a.statusMsg = "Searching..."
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
	case viewBoard:
		content = a.board.View()
	case viewBranch:
		content = a.branch.View()
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
	help := footerView(a.active, a.width, extra...)

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
	if a.active == viewSprint && a.sprint.Filtering() {
		return true
	}
	if a.active == viewHome && a.home.Filtering() {
		return true
	}
	if a.active == viewBranch && a.branch.InputActive() {
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
	case viewSearch:
		a.search.BackToInput()
		return a, nil
	case viewHome, viewLoading:
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

func (a App) fetchActiveSprint() tea.Cmd {
	return func() tea.Msg {
		sprint, err := a.client.ActiveSprint()
		if err != nil {
			return ErrMsg{Err: err}
		}
		return SprintLoadedMsg{Sprint: sprint}
	}
}

func (a App) fetchSprintIssues(sprintID int, sprintName string) tea.Cmd {
	return func() tea.Msg {
		issues, err := a.client.SprintIssues(sprintID)
		if err != nil {
			return ErrMsg{Err: err}
		}

		// Resolve parent metadata (single JQL call).
		parents := a.client.ResolveParents(issues)
		issues = client.EnrichWithParents(issues, parents)

		return IssuesLoadedMsg{Issues: issues, Title: sprintName}
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
		names, err := a.client.SearchUsers(a.client.Config().Project, prefix)
		if err != nil {
			return UserSearchMsg{Prefix: prefix, Names: nil}
		}
		return UserSearchMsg{Prefix: prefix, Names: names}
	}
}

func (a App) searchJQL(jql string) tea.Cmd {
	return func() tea.Msg {
		issues, err := a.client.SearchJQL(jql, 50)
		if err != nil {
			return ErrMsg{Err: err}
		}
		return SearchResultsMsg{Issues: issues, Query: jql}
	}
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
			// Push to origin without local checkout.
			out, err := exec.Command("git", "-C", req.RepoPath,
				"push", "origin", req.Base+":refs/heads/"+req.Name).CombinedOutput()
			if err != nil {
				return BranchCreatedMsg{Err: fmt.Errorf("%s", strings.TrimSpace(string(out)))}
			}
			return BranchCreatedMsg{Name: req.Name, Mode: "remote"}

		case "both":
			// Create local branch.
			out, err := exec.Command("git", "-C", req.RepoPath,
				"checkout", "-b", req.Name, req.Base).CombinedOutput()
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
			out, err := exec.Command("git", "-C", req.RepoPath,
				"checkout", "-b", req.Name, req.Base).CombinedOutput()
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
		text = fmt.Sprintf("git push origin %s:refs/heads/%s", req.Base, req.Name)
	case "both":
		text = fmt.Sprintf("git checkout -b %s %s && git push -u origin %s", req.Name, req.Base, req.Name)
	default:
		text = fmt.Sprintf("git checkout -b %s %s", req.Name, req.Base)
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
	return func() tea.Msg {
		sprints, err := a.client.BoardSprints(boardID, "active")
		if err == nil && len(sprints) > 0 {
			// Scrum board with active sprint — existing path.
			return SprintLoadedMsg{Sprint: &sprints[0]}
		}

		// No active sprint — fetch board issues via JQL instead.
		// This handles kanban boards and scrum boards between sprints.
		project := a.client.Config().Project
		issues, err := a.client.BoardIssues(project)
		if err != nil {
			return ErrMsg{Err: fmt.Errorf("no active iteration and board issue fetch failed: %w", err)}
		}

		// Resolve parent metadata in same command (single JQL call).
		parents := a.client.ResolveParents(issues)
		issues = client.EnrichWithParents(issues, parents)

		return IssuesLoadedMsg{Issues: issues, Title: "Board"}
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
