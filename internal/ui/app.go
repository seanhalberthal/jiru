package ui

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/seanhalberthal/jiratui/internal/client"
	"github.com/seanhalberthal/jiratui/internal/theme"
	"github.com/seanhalberthal/jiratui/internal/ui/issueview"
	"github.com/seanhalberthal/jiratui/internal/ui/sprintview"
)

// view represents which pane is currently active.
type view int

const (
	viewLoading view = iota
	viewSprint
	viewIssue
)

// App is the root bubbletea model.
type App struct {
	client    *client.Client
	keys      KeyMap
	active    view
	sprint    sprintview.Model
	issue     issueview.Model
	spinner   spinner.Model
	width     int
	height    int
	showHelp  bool
	statusMsg string
	err       error
}

// NewApp creates a new root application model.
func NewApp(c *client.Client) App {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(theme.ColourPrimary)

	return App{
		client:  c,
		keys:    DefaultKeyMap(),
		active:  viewLoading,
		sprint:  sprintview.New(),
		issue:   issueview.New(),
		spinner: s,
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
		return a, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, a.keys.Quit):
			return a, tea.Quit
		case key.Matches(msg, a.keys.ToggleHelp):
			a.showHelp = !a.showHelp
			return a, nil
		case key.Matches(msg, a.keys.Back) && a.active == viewIssue:
			a.active = viewSprint
			return a, nil
		case key.Matches(msg, a.keys.Refresh) && a.active == viewSprint:
			a.active = viewLoading
			a.statusMsg = "Refreshing..."
			return a, tea.Batch(a.spinner.Tick, a.fetchActiveSprint())
		}

	case ClientReadyMsg:
		a.statusMsg = fmt.Sprintf("Logged in as %s", msg.DisplayName)
		return a, a.fetchActiveSprint()

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
	case viewSprint:
		content = a.sprint.View()
	case viewIssue:
		content = a.issue.View()
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
