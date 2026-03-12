package ui

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/seanhalberthal/jiratui/internal/client"
	"github.com/seanhalberthal/jiratui/internal/jira"
	"github.com/seanhalberthal/jiratui/internal/theme"
	"github.com/seanhalberthal/jiratui/internal/ui/homeview"
	"github.com/seanhalberthal/jiratui/internal/ui/issueview"
	"github.com/seanhalberthal/jiratui/internal/ui/searchview"
	"github.com/seanhalberthal/jiratui/internal/ui/sprintview"
)

// view represents which pane is currently active.
type view int

const (
	viewLoading view = iota
	viewHome
	viewSprint
	viewIssue
	viewSearch
)

// App is the root bubbletea model.
type App struct {
	client       *client.Client
	keys         KeyMap
	active       view
	previousView view
	home         homeview.Model
	sprint       sprintview.Model
	issue        issueview.Model
	search       searchview.Model
	spinner      spinner.Model
	width        int
	height       int
	showHelp     bool
	statusMsg    string
	err          error
	boardID      int
	directIssue  string
}

// NewApp creates a new root application model.
func NewApp(c *client.Client, directIssue string) App {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(theme.ColourPrimary)

	return App{
		client:      c,
		keys:        DefaultKeyMap(),
		active:      viewLoading,
		home:        homeview.New(),
		sprint:      sprintview.New(),
		issue:       issueview.New(),
		search:      searchview.New(),
		spinner:     s,
		directIssue: directIssue,
	}
}

func (a App) Init() tea.Cmd {
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
		return a, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, a.keys.Quit):
			return a, tea.Quit
		case key.Matches(msg, a.keys.ToggleHelp):
			a.showHelp = !a.showHelp
			return a, nil
		case key.Matches(msg, a.keys.Search) && !a.search.Visible() && a.active != viewLoading:
			a.search.Show()
			a.previousView = a.active
			a.active = viewSearch
			return a, textinput.Blink
		case key.Matches(msg, a.keys.Home) && a.active != viewHome && a.active != viewLoading:
			a.active = viewHome
			return a, nil
		case key.Matches(msg, a.keys.Back) && a.active == viewIssue:
			a.active = viewSprint
			return a, nil
		case key.Matches(msg, a.keys.Back) && a.active == viewSprint:
			if a.client.Config().BoardID == 0 {
				a.active = viewHome
			}
			return a, nil
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
			return a, a.fetchActiveSprint()
		}
		return a, a.fetchBoards()

	case SprintLoadedMsg:
		a.statusMsg = fmt.Sprintf("Sprint: %s", msg.Sprint.Name)
		return a, a.fetchSprintIssues(msg.Sprint.ID)

	case IssuesLoadedMsg:
		a.active = viewSprint
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
			a.issue = a.issue.SetIssue(iss)
			a.issue.SetIssueURL(a.client.IssueURL(iss.Key))
			// Fetch full details in background.
			return a, tea.Batch(cmd, a.fetchIssueDetail(iss.Key))
		}
	case viewIssue:
		a.issue, cmd = a.issue.Update(msg)
		if url, ok := a.issue.OpenURL(); ok {
			openBrowser(url)
		}
	case viewSearch:
		a.search, cmd = a.search.Update(msg)
		if q := a.search.SubmittedQuery(); q != "" {
			a.statusMsg = "Searching..."
			return a, tea.Batch(cmd, a.searchJQL(q))
		}
		if iss := a.search.SelectedIssue(); iss != nil {
			a.search.Hide()
			a.active = viewIssue
			return a, tea.Batch(cmd, a.fetchIssueDetail(iss.Key))
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
	case viewLoading:
		msg := a.statusMsg
		if msg == "" {
			msg = "Connecting to Jira..."
		}
		content = lipgloss.Place(
			a.width, a.height-2,
			lipgloss.Center, lipgloss.Center,
			fmt.Sprintf("%s %s", a.spinner.View(), msg),
		)
	case viewHome:
		content = a.home.View()
	case viewSprint:
		content = a.sprint.View()
	case viewIssue:
		content = a.issue.View()
	case viewSearch:
		content = a.search.View()
	}

	if a.err != nil {
		errView := theme.StyleError.Render(fmt.Sprintf("Error: %v", a.err))
		content = lipgloss.JoinVertical(lipgloss.Left, content, errView)
	}

	help := a.helpView()

	return lipgloss.JoinVertical(lipgloss.Left, content, help)
}

func (a App) helpView() string {
	if !a.showHelp {
		return theme.StyleHelpKey.Render("?") + theme.StyleHelpDesc.Render(" help")
	}

	return fmt.Sprintf(
		"%s %s  %s %s  %s %s  %s %s  %s %s  %s %s",
		theme.StyleHelpKey.Render("j/k"), theme.StyleHelpDesc.Render("navigate"),
		theme.StyleHelpKey.Render("enter/l"), theme.StyleHelpDesc.Render("open"),
		theme.StyleHelpKey.Render("esc/h"), theme.StyleHelpDesc.Render("back"),
		theme.StyleHelpKey.Render("o"), theme.StyleHelpDesc.Render("browser"),
		theme.StyleHelpKey.Render("r"), theme.StyleHelpDesc.Render("refresh"),
		theme.StyleHelpKey.Render("q"), theme.StyleHelpDesc.Render("quit"),
	)
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

func (a App) fetchSprintIssues(sprintID int) tea.Cmd {
	return func() tea.Msg {
		issues, err := a.client.SprintIssues(sprintID)
		if err != nil {
			return ErrMsg{Err: err}
		}
		return IssuesLoadedMsg{Issues: issues}
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

func (a App) searchJQL(jql string) tea.Cmd {
	return func() tea.Msg {
		issues, err := a.client.SearchJQL(jql, 50)
		if err != nil {
			return ErrMsg{Err: err}
		}
		return SearchResultsMsg{Issues: issues, Query: jql}
	}
}

func (a App) fetchActiveSprintForBoard(boardID int) tea.Cmd {
	return func() tea.Msg {
		sprints, err := a.client.BoardSprints(boardID, "active")
		if err != nil {
			return ErrMsg{Err: err}
		}
		if len(sprints) == 0 {
			return ErrMsg{Err: fmt.Errorf("no active sprint found for this board")}
		}
		return SprintLoadedMsg{Sprint: &sprints[0]}
	}
}

func openBrowser(url string) {
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
