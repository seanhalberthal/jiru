package setupview

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/seanhalberthal/jiru/internal/client"
	"github.com/seanhalberthal/jiru/internal/config"
	"github.com/seanhalberthal/jiru/internal/jira"
	"github.com/seanhalberthal/jiru/internal/theme"
	"github.com/seanhalberthal/jiru/internal/validate"
)

const (
	stepWelcome = iota
	stepDomain
	stepUser
	stepAPIToken
	stepAuthType
	stepProject
	stepBoardID
	stepRepoPath
	stepBranchCase
	stepBranchMode
	stepBranchCopy
	stepConfirm
	totalSteps = stepConfirm + 1
)

// stepMeta describes a wizard step.
type stepMeta struct {
	title       string
	description string
	placeholder string
	required    bool
	mask        bool // true for API token — masks input
	validate    func(string) error
	prefill     string // default value to pre-fill
}

var steps = [totalSteps]stepMeta{
	stepWelcome: {
		title:       "Welcome to jiru",
		description: "It looks like your Jira credentials aren't configured yet.\nThis wizard will walk you through setting them up.\n\nPress enter to continue, or esc to quit.",
	},
	stepDomain: {
		title:       "Jira Domain",
		description: "Enter your Jira instance domain.\nIf you paste a full URL, the protocol will be stripped automatically.",
		placeholder: "mycompany.atlassian.net",
		required:    true,
		validate: func(s string) error {
			s = cleanDomain(s)
			return validate.Domain(s)
		},
	},
	stepUser: {
		title:       "Jira User (Email)",
		description: "Enter the email address you use to log in to Jira.",
		placeholder: "you@company.com",
		required:    true,
		validate:    validate.Email,
	},
	stepAPIToken: {
		title:       "API Token",
		description: "Enter your Jira API token.\nGenerate one at: https://id.atlassian.com/manage-profile/security/api-tokens",
		placeholder: "Paste your API token here",
		required:    true,
		mask:        true,
		validate: func(s string) error {
			if len(s) == 0 {
				return fmt.Errorf("API token cannot be empty")
			}
			return nil
		},
	},
	stepAuthType: {
		title:       "Auth Type",
		description: "Authentication method. Use 'basic' for Jira Cloud (most common)\nor 'bearer' for Jira Server/Data Centre with PAT.",
		placeholder: "basic",
		prefill:     "basic",
		validate:    validate.AuthType,
	},
	stepProject: {
		title:       "Default Project (optional)",
		description: "Select a default project to filter boards.\nUse \u2191/\u2193 to navigate, enter to select.",
	},
	stepBoardID: {
		title:       "Default Board (optional)",
		description: "Select a default board to open on startup.\nUse \u2191/\u2193 to navigate, enter to select.",
	},
	stepRepoPath: {
		title:       "Git Repository Path (optional)",
		description: "Path to your local git repository for branch creation.\nWhen set, pressing 'n' on an issue will create a branch directly.\nLeave blank to copy the git command to clipboard instead.\n~ is expanded to your home directory.",
		placeholder: "~/path/to/your/repo",
		validate: func(s string) error {
			if s == "" {
				return nil
			}
			s = expandTilde(s)
			info, err := os.Stat(s)
			if err != nil {
				return fmt.Errorf("path does not exist: %s", s)
			}
			if !info.IsDir() {
				return fmt.Errorf("path is not a directory: %s", s)
			}
			// Check for .git directory.
			gitDir := filepath.Join(s, ".git")
			if _, err := os.Stat(gitDir); err != nil {
				return fmt.Errorf("not a git repository (no .git directory): %s", s)
			}
			// Canonicalise to prevent path traversal issues.
			resolved, err := filepath.EvalSymlinks(s)
			if err != nil {
				return fmt.Errorf("cannot resolve path: %w", err)
			}
			_ = resolved // Canonicalisation applied at input storage time.
			return nil
		},
	},
	stepBranchCase: {
		title:       "Branch Name Case (optional)",
		description: "Use title case branch names when creating branches from issues?\n\nTitle case: PROJ-123-Fix-Login-Bug\nLowercase:  proj-123-fix-login-bug",
	},
	stepBranchMode: {
		title:       "Branch Creation Mode (optional)",
		description: "Where should branches be created?\n\nLocal:  checkout a new branch in your local repo\nRemote: push the branch to origin (no local checkout)\nBoth:   checkout locally and push to origin",
	},
	stepBranchCopy: {
		title:       "Copy Branch Name on Create (optional)",
		description: "After a branch is created, copy the branch name to the clipboard?\n\nHandy for pasting into worktree creation commands, commit messages or PR titles",
	},
	stepConfirm: {
		title:       "Confirm",
		description: "Review your settings below.\nPress enter to save, ctrl+b to go back, or ctrl+r to restart.",
	},
}

// validationOKMsg is sent when async API validation succeeds.
type validationOKMsg struct {
	step int
}

// validationFailMsg is sent when async API validation fails.
type validationFailMsg struct {
	step int
	err  error
}

// projectsLoadedMsg is sent when projects are fetched for the project picker.
type projectsLoadedMsg struct {
	projects []jira.Project
	err      error
}

// boardsLoadedMsg is sent when boards are fetched for the board picker.
type boardsLoadedMsg struct {
	boards []jira.Board
	err    error
}

// Model is the setup wizard Bubble Tea model.
type Model struct {
	step       int
	inputs     [totalSteps]textinput.Model
	values     [totalSteps]string // Confirmed values per step.
	errMsg     string
	width      int
	height     int
	done       bool           // Wizard completed.
	quit       bool           // User chose to quit.
	config     *config.Config // Pre-loaded partial config.
	validating bool           // True while an async API check is in flight.
	valSpinner spinner.Model

	// Project picker state.
	projects       []jira.Project // Available projects for selection.
	projectCursor  int            // 0 = "None", 1+ = project index.
	projectsLoaded bool           // True once projects have been fetched.

	// Branch case toggle state.
	branchCaseCursor int // 0 = lowercase, 1 = UPPERCASE

	// Branch mode toggle state.
	branchModeCursor int // 0 = local, 1 = remote, 2 = both

	// Branch copy-key toggle state.
	branchCopyCursor int // 0 = off, 1 = on

	// Auth rate limiting.
	lastAuthAttempt time.Time

	// Board picker state.
	boards             []jira.Board // Available boards for selection.
	boardCursor        int          // 0 = "None", 1+ = board index.
	boardsLoaded       bool         // True once boards have been fetched.
	boardsFetchProject string       // Project key used for the last board fetch.

	// Profile creation state.
	forNewProfile    bool // True when creating a new profile (always prompts for name).
	profileNameInput textinput.Model
	askingProfile    bool   // True when prompting for a profile name.
	profileName      string // Non-empty when saving as a named profile.
}

// New creates a new setup wizard, pre-filled with any values from partial config.
func New(partial *config.Config) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(theme.ColourPrimary)

	m := Model{
		step:       stepWelcome,
		config:     partial,
		valSpinner: s,
	}

	for i := range m.inputs {
		ti := textinput.New()
		ti.CharLimit = 500
		ti.Width = 60
		if steps[i].placeholder != "" {
			ti.Placeholder = steps[i].placeholder
		}
		if steps[i].mask {
			ti.EchoMode = textinput.EchoPassword
		}
		m.inputs[i] = ti
	}

	// Pre-fill from partial config.
	if partial != nil {
		if partial.Domain != "" {
			m.inputs[stepDomain].SetValue(partial.Domain)
			m.values[stepDomain] = partial.Domain
		}
		if partial.User != "" {
			m.inputs[stepUser].SetValue(partial.User)
			m.values[stepUser] = partial.User
		}
		if partial.APIToken != "" {
			m.inputs[stepAPIToken].SetValue(partial.APIToken)
			m.values[stepAPIToken] = partial.APIToken
		}
		if partial.AuthType != "" {
			m.inputs[stepAuthType].SetValue(partial.AuthType)
			m.values[stepAuthType] = partial.AuthType
		} else {
			m.inputs[stepAuthType].SetValue("basic")
			m.values[stepAuthType] = "basic"
		}
		if partial.Project != "" {
			m.values[stepProject] = partial.Project
		}
		if partial.BoardID != 0 {
			bid := strconv.Itoa(partial.BoardID)
			m.values[stepBoardID] = bid
		}
		if partial.RepoPath != "" {
			m.inputs[stepRepoPath].SetValue(partial.RepoPath)
			m.values[stepRepoPath] = partial.RepoPath
		}
		if partial.BranchUppercase {
			m.branchCaseCursor = 1
			m.values[stepBranchCase] = "true"
		}
		switch partial.BranchMode {
		case "remote":
			m.branchModeCursor = 1
			m.values[stepBranchMode] = "remote"
		case "both":
			m.branchModeCursor = 2
			m.values[stepBranchMode] = "both"
		default:
			m.values[stepBranchMode] = "local"
		}
		if partial.BranchCopyName {
			m.branchCopyCursor = 1
			m.values[stepBranchCopy] = "true"
		}
	} else {
		// Default auth type.
		m.inputs[stepAuthType].SetValue("basic")
		m.values[stepAuthType] = "basic"
	}

	// Also apply prefill for steps that have it and no partial value.
	for i, s := range steps {
		if s.prefill != "" && m.inputs[i].Value() == "" {
			m.inputs[i].SetValue(s.prefill)
		}
	}

	return m
}

// SetSize sets the terminal dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	for i := range m.inputs {
		m.inputs[i].Width = min(width-10, 78) // cap at 78 so prompt (2 chars) fits within content area of 80
	}
}

// GoToConfirm jumps the wizard directly to the confirm/preview step.
func (m *Model) GoToConfirm() {
	m.step = stepConfirm
}

// Done returns true (once) when the wizard has completed successfully.
func (m *Model) Done() bool {
	d := m.done
	m.done = false
	return d
}

// Quit returns true (once) when the user chose to exit.
func (m *Model) Quit() bool {
	q := m.quit
	m.quit = false
	return q
}

// InputActive returns true when a text input is focused.
func (m Model) InputActive() bool {
	return isInputStep(m.step) || m.askingProfile
}

// Config returns the completed config from wizard values.
func (m Model) Config() *config.Config {
	branchMode := m.values[stepBranchMode]
	if branchMode == "" {
		branchMode = "local"
	}
	cfg := &config.Config{
		Domain:          cleanDomain(m.values[stepDomain]),
		User:            m.values[stepUser],
		APIToken:        m.values[stepAPIToken],
		AuthType:        m.values[stepAuthType],
		Project:         m.values[stepProject],
		RepoPath:        m.values[stepRepoPath],
		BranchUppercase: m.values[stepBranchCase] == "true",
		BranchMode:      branchMode,
		BranchCopyName:  m.values[stepBranchCopy] == "true",
	}
	if bid := m.values[stepBoardID]; bid != "" {
		if id, err := strconv.Atoi(bid); err == nil {
			cfg.BoardID = id
		}
	}
	return cfg
}

// ProfileName returns the profile name if the user chose "save as new profile".
// Empty string means save to the default profile.
func (m Model) ProfileName() string {
	return m.profileName
}

// SetForNewProfile marks this wizard as creating a new profile.
// The confirm step will prompt for a profile name before saving.
func (m *Model) SetForNewProfile() {
	m.forNewProfile = true
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case validationOKMsg:
		m.validating = false
		m.errMsg = ""
		m.step++
		return m, m.onStepEnter()

	case validationFailMsg:
		m.validating = false
		m.errMsg = msg.err.Error()
		// Re-focus the input so the user can correct the value.
		if isInputStep(m.step) {
			m.inputs[m.step].Focus()
		}
		return m, textinput.Blink

	case projectsLoadedMsg:
		m.projectsLoaded = true
		m.validating = false
		if msg.err != nil {
			m.errMsg = fmt.Sprintf("Could not fetch projects: %s", msg.err.Error())
			m.projects = nil
		} else {
			m.projects = msg.projects
			m.errMsg = ""
		}
		// Pre-select existing project key if any.
		if key := m.values[stepProject]; key != "" {
			for i, p := range m.projects {
				if p.Key == key {
					m.projectCursor = i + 1
					break
				}
			}
		}
		return m, nil

	case boardsLoadedMsg:
		m.boardsLoaded = true
		m.validating = false
		if msg.err != nil {
			m.errMsg = fmt.Sprintf("Could not fetch boards: %s", msg.err.Error())
			m.boards = nil
		} else {
			m.boards = msg.boards
			m.errMsg = ""
		}
		// Pre-select existing board ID if any.
		if bid := m.values[stepBoardID]; bid != "" {
			if id, err := strconv.Atoi(bid); err == nil {
				for i, b := range m.boards {
					if b.ID == id {
						m.boardCursor = i + 1 // +1 because 0 is "None"
						break
					}
				}
			}
		}
		return m, nil

	case spinner.TickMsg:
		if m.validating {
			var cmd tea.Cmd
			m.valSpinner, cmd = m.valSpinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case tea.KeyMsg:
		// Ignore all keys while validating.
		if m.validating {
			return m, nil
		}

		// ctrl+c and esc always quit the wizard (or cancel profile name prompt).
		if msg.String() == "ctrl+c" || msg.String() == "esc" {
			if m.askingProfile {
				m.askingProfile = false
				m.errMsg = ""
				return m, nil
			}
			m.quit = true
			return m, nil
		}

		// Handle profile name prompt.
		if m.askingProfile {
			if msg.String() == "enter" {
				name := strings.TrimSpace(m.profileNameInput.Value())
				if name == "" {
					m.errMsg = "Profile name cannot be empty"
					return m, nil
				}
				m.profileName = name
				m.askingProfile = false
				m.done = true
				return m, nil
			}
			var cmd tea.Cmd
			m.profileNameInput, cmd = m.profileNameInput.Update(msg)
			return m, cmd
		}

		// ctrl+b: always go back one step (no-op at welcome).
		if msg.String() == "ctrl+b" {
			if m.step > stepWelcome {
				m.step--
				m.errMsg = ""
				return m, m.onStepEnter()
			}
			return m, nil
		}

		switch m.step {
		case stepWelcome:
			if msg.String() == "enter" {
				// Skip steps that are already filled from partial config.
				m.step = m.nextMissingStep(stepDomain)
				return m, m.onStepEnter()
			}

		case stepConfirm:
			if msg.String() == "enter" {
				if m.forNewProfile && !m.askingProfile {
					// Need a profile name before saving.
					m.profileNameInput = textinput.New()
					m.profileNameInput.Placeholder = "Profile name (e.g. work, staging)"
					m.profileNameInput.CharLimit = 30
					m.profileNameInput.Width = 40
					m.profileNameInput.Focus()
					m.askingProfile = true
					m.errMsg = ""
					return m, textinput.Blink
				}
				m.done = true
				return m, nil
			}
			if msg.String() == "ctrl+r" {
				// Restart wizard from the first input step.
				m.step = stepDomain
				m.errMsg = ""
				return m, m.onStepEnter()
			}

		case stepProject:
			return m.handleProjectPicker(msg)

		case stepBoardID:
			return m.handleBoardPicker(msg)

		case stepBranchCase:
			return m.handleBranchCaseToggle(msg)

		case stepBranchMode:
			return m.handleBranchModeToggle(msg)

		case stepBranchCopy:
			return m.handleBranchCopyToggle(msg)

		default:
			// Input steps.
			if msg.String() == "enter" {
				value := strings.TrimSpace(m.inputs[m.step].Value())
				// Clean domain input.
				if m.step == stepDomain {
					value = cleanDomain(value)
					m.inputs[m.step].SetValue(value)
				}
				// Expand ~ and canonicalise repo path.
				if m.step == stepRepoPath && value != "" {
					value = expandTilde(value)
					if resolved, err := filepath.EvalSymlinks(value); err == nil {
						value = filepath.Clean(resolved)
					}
					m.inputs[m.step].SetValue(value)
				}
				// Required check.
				if steps[m.step].required && value == "" {
					m.errMsg = "This field is required"
					return m, nil
				}
				// Validate.
				if steps[m.step].validate != nil && value != "" {
					if err := steps[m.step].validate(value); err != nil {
						m.errMsg = err.Error()
						return m, nil
					}
				}
				m.values[m.step] = value
				m.errMsg = ""
				m.inputs[m.step].Blur()

				// Trigger async API validation at checkpoint steps.
				if cmd := m.apiValidation(); cmd != nil {
					if time.Since(m.lastAuthAttempt) < 2*time.Second {
						m.errMsg = "Please wait before retrying"
						return m, nil
					}
					m.lastAuthAttempt = time.Now()
					m.validating = true
					return m, tea.Batch(m.valSpinner.Tick, cmd)
				}

				m.step++
				return m, m.onStepEnter()
			}
		}
	}

	// Update the current text input.
	if isInputStep(m.step) && !m.validating {
		var cmd tea.Cmd
		m.inputs[m.step], cmd = m.inputs[m.step].Update(msg)
		return m, cmd
	}

	return m, nil
}

// handleProjectPicker handles key events for the project picker step.
func (m Model) handleProjectPicker(msg tea.KeyMsg) (Model, tea.Cmd) {
	if !m.projectsLoaded {
		return m, nil
	}

	switch msg.String() {
	case "enter", " ":
		if len(m.projects) == 0 || m.projectCursor == 0 {
			m.values[stepProject] = ""
		} else {
			m.values[stepProject] = m.projects[m.projectCursor-1].Key
		}
		m.errMsg = ""
		m.step++
		return m, m.onStepEnter()

	case "up", "k":
		if m.projectCursor > 0 {
			m.projectCursor--
		}

	case "down", "j":
		if m.projectCursor < len(m.projects) {
			m.projectCursor++
		}
	}

	return m, nil
}

// handleBoardPicker handles key events for the board picker step.
func (m Model) handleBoardPicker(msg tea.KeyMsg) (Model, tea.Cmd) {
	if !m.boardsLoaded {
		return m, nil
	}

	switch msg.String() {
	case "enter", " ":
		if len(m.boards) == 0 || m.boardCursor == 0 {
			m.values[stepBoardID] = ""
		} else {
			board := m.boards[m.boardCursor-1]
			m.values[stepBoardID] = strconv.Itoa(board.ID)
		}
		m.errMsg = ""
		m.step++
		return m, m.onStepEnter()

	case "up", "k":
		if m.boardCursor > 0 {
			m.boardCursor--
		}

	case "down", "j":
		if m.boardCursor < len(m.boards) {
			m.boardCursor++
		}
	}

	return m, nil
}

// handleBranchCaseToggle handles key events for the branch case toggle step.
func (m Model) handleBranchCaseToggle(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "enter", " ":
		if m.branchCaseCursor == 1 {
			m.values[stepBranchCase] = "true"
		} else {
			m.values[stepBranchCase] = ""
		}
		m.step++
		return m, m.onStepEnter()
	case "up", "k", "down", "j", "tab":
		// Toggle between the two options.
		m.branchCaseCursor = 1 - m.branchCaseCursor
	}
	return m, nil
}

// handleBranchModeToggle handles key events for the branch mode toggle step.
func (m Model) handleBranchModeToggle(msg tea.KeyMsg) (Model, tea.Cmd) {
	modes := []string{"local", "remote", "both"}
	switch msg.String() {
	case "enter", " ":
		m.values[stepBranchMode] = modes[m.branchModeCursor]
		m.step++
		return m, m.onStepEnter()
	case "up", "k":
		if m.branchModeCursor > 0 {
			m.branchModeCursor--
		}
	case "down", "j":
		if m.branchModeCursor < len(modes)-1 {
			m.branchModeCursor++
		}
	case "tab":
		m.branchModeCursor = (m.branchModeCursor + 1) % len(modes)
	}
	return m, nil
}

// handleBranchCopyToggle handles key events for the copy-issue-key toggle step.
func (m Model) handleBranchCopyToggle(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "enter", " ":
		if m.branchCopyCursor == 1 {
			m.values[stepBranchCopy] = "true"
		} else {
			m.values[stepBranchCopy] = ""
		}
		m.step++
		return m, m.onStepEnter()
	case "up", "k", "down", "j", "tab":
		m.branchCopyCursor = 1 - m.branchCopyCursor
	}
	return m, nil
}

// onStepEnter returns the appropriate tea.Cmd when entering the current step.
// Must be called after setting m.step to the new value.
func (m *Model) onStepEnter() tea.Cmd {
	switch m.step {
	case stepProject:
		if m.projectsLoaded {
			return nil
		}
		m.validating = true
		m.projectsLoaded = false
		m.projectCursor = 0
		return tea.Batch(m.valSpinner.Tick, m.fetchProjectsCmd())

	case stepBoardID:
		currentProject := m.values[stepProject]
		if m.boardsLoaded && m.boardsFetchProject == currentProject {
			return nil // Already have boards for this project.
		}
		m.validating = true
		m.boardsLoaded = false
		m.boardCursor = 0
		m.boardsFetchProject = currentProject
		return tea.Batch(m.valSpinner.Tick, m.fetchBoardsCmd())
	}

	if isInputStep(m.step) {
		m.inputs[m.step].Focus()
		return textinput.Blink
	}
	return nil
}

// fetchProjectsCmd returns a command that fetches projects from Jira.
func (m Model) fetchProjectsCmd() tea.Cmd {
	cfg := m.buildPartialConfig()
	return func() tea.Msg {
		c := client.New(cfg)
		projects, err := c.Projects()
		return projectsLoadedMsg{projects: projects, err: err}
	}
}

// fetchBoardsCmd returns a command that fetches boards from Jira.
func (m Model) fetchBoardsCmd() tea.Cmd {
	cfg := m.buildPartialConfig()
	project := m.values[stepProject]
	return func() tea.Msg {
		c := client.New(cfg)
		boards, err := c.Boards(project)
		return boardsLoadedMsg{boards: boards, err: err}
	}
}

// apiValidation returns a tea.Cmd for async API validation at checkpoint steps,
// or nil if the current step doesn't need API validation.
func (m Model) apiValidation() tea.Cmd {
	switch m.step {
	case stepAuthType:
		// All credentials are now collected — verify auth.
		cfg := m.buildPartialConfig()
		return func() tea.Msg {
			c := client.New(cfg)
			_, err := c.Me()
			if err != nil {
				return validationFailMsg{step: stepAuthType, err: fmt.Errorf("authentication failed — check your domain, email, API token, and auth type")}
			}
			return validationOKMsg{step: stepAuthType}
		}
	}
	return nil
}

// buildPartialConfig constructs a config from the current wizard values.
func (m Model) buildPartialConfig() *config.Config {
	branchMode := m.values[stepBranchMode]
	if branchMode == "" {
		branchMode = "local"
	}
	cfg := &config.Config{
		Domain:          cleanDomain(m.values[stepDomain]),
		User:            m.values[stepUser],
		APIToken:        m.values[stepAPIToken],
		AuthType:        m.values[stepAuthType],
		Project:         m.values[stepProject],
		RepoPath:        m.values[stepRepoPath],
		BranchUppercase: m.values[stepBranchCase] == "true",
		BranchMode:      branchMode,
		BranchCopyName:  m.values[stepBranchCopy] == "true",
	}
	if cfg.AuthType == "" {
		cfg.AuthType = "basic"
	}
	if bid := m.values[stepBoardID]; bid != "" {
		if id, err := strconv.Atoi(bid); err == nil {
			cfg.BoardID = id
		}
	}
	return cfg
}

func (m Model) View() string {
	if m.width == 0 {
		return ""
	}

	if m.askingProfile {
		return m.renderProfileNamePrompt()
	}

	s := steps[m.step]
	contentWidth := min(m.width-8, 80)

	titleStyle := theme.StyleTitle.MarginBottom(1)
	descStyle := theme.StyleSubtle
	errStyle := theme.StyleError
	stepIndicator := theme.StyleSubtle.Render(fmt.Sprintf("Step %d of %d", m.step, totalSteps-1))

	var sections []string

	// Welcome step gets special treatment — centred logo with integrated message.
	if m.step == stepWelcome {
		return m.renderWelcome(contentWidth)
	}

	// Title + step indicator.
	sections = append(sections, lipgloss.JoinHorizontal(lipgloss.Top,
		titleStyle.Render(s.title),
		"  ",
		stepIndicator,
	))

	// Description.
	sections = append(sections, descStyle.Render(s.description))
	sections = append(sections, "")

	// Input or summary.
	switch m.step {
	case stepWelcome:
		// No input — just the description.

	case stepConfirm:
		sections = append(sections, m.renderSummary())

	case stepProject:
		sections = append(sections, m.renderProjectPicker(errStyle)...)

	case stepBoardID:
		sections = append(sections, m.renderBoardPicker(errStyle)...)

	case stepBranchCase:
		sections = append(sections, m.renderBranchCaseToggle()...)

	case stepBranchMode:
		sections = append(sections, m.renderBranchModeToggle()...)

	case stepBranchCopy:
		sections = append(sections, m.renderBranchCopyToggle()...)

	default:
		sections = append(sections, m.inputs[m.step].View())
		if m.validating {
			sections = append(sections, fmt.Sprintf("%s %s", m.valSpinner.View(), theme.StyleSubtle.Render(m.validationLabel())))
		} else if m.errMsg != "" {
			sections = append(sections, errStyle.Render(m.errMsg))
		}
		if steps[m.step].required && !m.validating {
			sections = append(sections, theme.StyleSubtle.Render("* required"))
		}
	}

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	// Centre the content on screen. Footer is rendered by the parent via
	// footerView() + FooterHints(), so we only produce the box here.
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColourPrimary).
		Padding(1, 3).
		Width(contentWidth).
		Render(content)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// FooterHint is a key + description pair used by the parent's global footer.
type FooterHint struct {
	Key  string
	Desc string
}

// FooterHints returns the step-specific keybind hints for the global footer.
// The parent view passes these as extras to footerView().
func (m Model) FooterHints() []FooterHint {
	var hints []FooterHint

	switch m.step {
	case stepWelcome:
		hints = append(hints, FooterHint{"enter", "start"})
	case stepConfirm:
		hints = append(hints, FooterHint{"enter", "save"}, FooterHint{"ctrl+r", "restart"})
	case stepProject, stepBoardID, stepBranchCase, stepBranchMode, stepBranchCopy:
		hints = append(hints, FooterHint{"↑/↓", "toggle"}, FooterHint{"enter", "select"})
	default:
		hints = append(hints, FooterHint{"enter", "next"})
	}

	if m.step > stepWelcome {
		hints = append(hints, FooterHint{"ctrl+b", "back"})
	}
	hints = append(hints, FooterHint{"esc", "quit"})
	return hints
}

// renderProjectPicker returns view sections for the project picker step.
func (m Model) renderProjectPicker(errStyle lipgloss.Style) []string {
	if m.validating {
		return []string{
			fmt.Sprintf("%s %s", m.valSpinner.View(), theme.StyleSubtle.Render("Fetching projects...")),
		}
	}

	if m.errMsg != "" {
		return []string{
			errStyle.Render(m.errMsg),
			theme.StyleSubtle.Render("Press enter to continue without a default project."),
		}
	}

	if len(m.projects) == 0 {
		return []string{
			theme.StyleSubtle.Render("No projects found. Press enter to continue."),
		}
	}

	selectedStyle := lipgloss.NewStyle().Foreground(theme.ColourPrimary).Bold(true)
	normalStyle := lipgloss.NewStyle()

	var items []string

	// "None" option.
	if m.projectCursor == 0 {
		items = append(items, selectedStyle.Render("▸ None — show all boards"))
	} else {
		items = append(items, normalStyle.Render("  None — show all boards"))
	}

	// Project items.
	for i, p := range m.projects {
		label := fmt.Sprintf("%s (%s)", p.Name, p.Key)
		if m.projectCursor == i+1 {
			items = append(items, selectedStyle.Render("▸ "+label))
		} else {
			items = append(items, normalStyle.Render("  "+label))
		}
	}

	return []string{strings.Join(items, "\n")}
}

// renderBoardPicker returns view sections for the board picker step.
func (m Model) renderBoardPicker(errStyle lipgloss.Style) []string {
	if m.validating {
		return []string{
			fmt.Sprintf("%s %s", m.valSpinner.View(), theme.StyleSubtle.Render("Fetching boards...")),
		}
	}

	if m.errMsg != "" {
		return []string{
			errStyle.Render(m.errMsg),
			theme.StyleSubtle.Render("Press enter to continue without a default board."),
		}
	}

	if len(m.boards) == 0 {
		return []string{
			theme.StyleSubtle.Render("No boards found. Press enter to continue."),
		}
	}

	selectedStyle := lipgloss.NewStyle().Foreground(theme.ColourPrimary).Bold(true)
	normalStyle := lipgloss.NewStyle()

	var items []string

	// "None" option.
	if m.boardCursor == 0 {
		items = append(items, selectedStyle.Render("▸ None — show all boards on startup"))
	} else {
		items = append(items, normalStyle.Render("  None — show all boards on startup"))
	}

	// Board items.
	for i, b := range m.boards {
		label := fmt.Sprintf("%s [%s]", b.Name, b.Type)
		if m.boardCursor == i+1 {
			items = append(items, selectedStyle.Render("▸ "+label))
		} else {
			items = append(items, normalStyle.Render("  "+label))
		}
	}

	return []string{strings.Join(items, "\n")}
}

// renderBranchCaseToggle returns view sections for the branch case toggle step.
func (m Model) renderBranchCaseToggle() []string {
	selectedStyle := lipgloss.NewStyle().Foreground(theme.ColourPrimary).Bold(true)
	normalStyle := lipgloss.NewStyle()

	options := []string{"lowercase    (proj-123-fix-login-bug)", "Title Case   (PROJ-123-Fix-Login-Bug)"}
	var items []string
	for i, opt := range options {
		if m.branchCaseCursor == i {
			items = append(items, selectedStyle.Render("▸ "+opt))
		} else {
			items = append(items, normalStyle.Render("  "+opt))
		}
	}
	return []string{strings.Join(items, "\n")}
}

// renderBranchModeToggle returns view sections for the branch mode toggle step.
func (m Model) renderBranchModeToggle() []string {
	selectedStyle := lipgloss.NewStyle().Foreground(theme.ColourPrimary).Bold(true)
	normalStyle := lipgloss.NewStyle()

	options := []string{
		"local   (checkout new branch in local repo)",
		"remote  (push branch to origin)",
		"both    (checkout locally + push to origin)",
	}
	var items []string
	for i, opt := range options {
		if m.branchModeCursor == i {
			items = append(items, selectedStyle.Render("▸ "+opt))
		} else {
			items = append(items, normalStyle.Render("  "+opt))
		}
	}
	return []string{strings.Join(items, "\n")}
}

// renderBranchCopyToggle returns view sections for the copy-issue-key toggle step.
func (m Model) renderBranchCopyToggle() []string {
	selectedStyle := lipgloss.NewStyle().Foreground(theme.ColourPrimary).Bold(true)
	normalStyle := lipgloss.NewStyle()

	options := []string{
		"off  (don't touch the clipboard after create)",
		"on   (copy the branch name, e.g. proj-123-fix-login-bug)",
	}
	var items []string
	for i, opt := range options {
		if m.branchCopyCursor == i {
			items = append(items, selectedStyle.Render("▸ "+opt))
		} else {
			items = append(items, normalStyle.Render("  "+opt))
		}
	}
	return []string{strings.Join(items, "\n")}
}

func (m Model) renderWelcome(contentWidth int) string {
	var sections []string

	// Centred logo.
	if logo := theme.RenderLogo(contentWidth); logo != "" {
		centredLogo := lipgloss.NewStyle().Width(contentWidth - 8).Align(lipgloss.Center).Render(logo)
		sections = append(sections, centredLogo)
		sections = append(sections, "")
	}

	// Tagline beneath the logo.
	tagline := lipgloss.NewStyle().
		Foreground(theme.ColourPrimary).
		Bold(true).
		Width(contentWidth - 8).
		Align(lipgloss.Center).
		Render("A terminal UI for Jira")
	sections = append(sections, tagline)
	sections = append(sections, "")

	// Setup prompt — context-aware message.
	msgStyle := theme.StyleSubtle.Width(contentWidth - 8).Align(lipgloss.Center)
	var msgText string
	if m.forNewProfile {
		msgText = "Add a new profile.\nThis wizard will collect the credentials for the new connection."
	} else {
		msgText = "Your credentials aren't configured yet.\nThis wizard will walk you through setting them up."
	}
	msg := msgStyle.Render(msgText)
	sections = append(sections, msg)
	sections = append(sections, "")

	// Keybind hints.
	hint := theme.StyleHelpKey.Render("enter") + " " + theme.StyleHelpDesc.Render("continue") + "    " +
		theme.StyleHelpKey.Render("esc") + " " + theme.StyleHelpDesc.Render("quit")
	centredHint := lipgloss.NewStyle().Width(contentWidth - 8).Align(lipgloss.Center).Render(hint)
	sections = append(sections, centredHint)

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColourPrimary).
		Padding(2, 4).
		Width(contentWidth).
		Render(content)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m Model) renderProfileNamePrompt() string {
	contentWidth := min(m.width-8, 80)
	titleStyle := theme.StyleTitle.MarginBottom(1)

	var sections []string
	sections = append(sections, titleStyle.Render("New Profile"))
	sections = append(sections, theme.StyleSubtle.Render("Enter a name for this profile."))
	sections = append(sections, "")
	sections = append(sections, m.profileNameInput.View())

	if m.errMsg != "" {
		sections = append(sections, theme.StyleError.Render(m.errMsg))
	}

	sections = append(sections, "")
	sections = append(sections, fmt.Sprintf("%s %s  %s %s",
		theme.StyleHelpKey.Render("enter"), theme.StyleHelpDesc.Render("save"),
		theme.StyleHelpKey.Render("esc"), theme.StyleHelpDesc.Render("cancel")))

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColourPrimary).
		Padding(1, 3).
		Width(contentWidth).
		Render(content)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m Model) renderSummary() string {
	labelStyle := lipgloss.NewStyle().Bold(true).Width(14)
	valueStyle := lipgloss.NewStyle().Foreground(theme.ColourPrimary)
	maskedStyle := lipgloss.NewStyle().Foreground(theme.ColourSubtle)

	// Resolve project display name.
	projectDisplay := ""
	if key := m.values[stepProject]; key != "" {
		for _, p := range m.projects {
			if p.Key == key {
				projectDisplay = fmt.Sprintf("%s (%s)", p.Name, p.Key)
				break
			}
		}
		if projectDisplay == "" {
			projectDisplay = key // Fallback to raw key.
		}
	}

	// Resolve board display name.
	boardDisplay := ""
	if bid := m.values[stepBoardID]; bid != "" {
		if id, err := strconv.Atoi(bid); err == nil {
			for _, b := range m.boards {
				if b.ID == id {
					boardDisplay = fmt.Sprintf("%s [%s]", b.Name, b.Type)
					break
				}
			}
			if boardDisplay == "" {
				boardDisplay = bid // Fallback to raw ID.
			}
		}
	}

	rows := []struct {
		label string
		value string
		mask  bool
	}{
		{"Domain", m.values[stepDomain], false},
		{"User", m.values[stepUser], false},
		{"API Token", m.values[stepAPIToken], true},
		{"Auth Type", m.values[stepAuthType], false},
		{"Project", projectDisplay, false},
		{"Board", boardDisplay, false},
		{"Repo Path", m.values[stepRepoPath], false},
		{"Branch Case", func() string {
			if m.values[stepBranchCase] == "true" {
				return "UPPERCASE"
			}
			return "lowercase"
		}(), false},
		{"Branch Mode", func() string {
			switch m.values[stepBranchMode] {
			case "remote":
				return "remote"
			case "both":
				return "both"
			default:
				return "local"
			}
		}(), false},
		{"Copy Key", func() string {
			if m.values[stepBranchCopy] == "true" {
				return "on"
			}
			return "off"
		}(), false},
	}

	var lines []string
	for _, r := range rows {
		display := r.value
		if display == "" {
			display = "(not set)"
		} else if r.mask {
			// Show first 4 and last 4 chars, mask the rest.
			if len(display) > 10 {
				display = display[:4] + strings.Repeat("*", len(display)-8) + display[len(display)-4:]
			} else {
				display = strings.Repeat("*", len(display))
			}
		}
		style := valueStyle
		if r.mask {
			style = maskedStyle
		}
		if display == "(not set)" {
			style = maskedStyle
		}
		lines = append(lines, labelStyle.Render(r.label)+style.Render(display))
	}

	summary := strings.Join(lines, "\n")
	saveNote := theme.StyleSubtle.Render(
		"\nAPI token: OS keychain\nOther settings: $XDG_CONFIG_HOME/jiru/profiles.json")

	return summary + saveNote
}

// nextMissingStep returns the next step at or after `from` that has no value yet.
// If all are filled, returns stepConfirm.
func (m Model) nextMissingStep(from int) int {
	for i := from; i < stepConfirm; i++ {
		if i == stepWelcome {
			continue
		}
		// Picker steps: check values directly (no text input).
		if i == stepProject || i == stepBoardID {
			if m.values[i] != "" {
				continue
			}
			return i
		}
		// Branch case, mode, and copy-key all have valid defaults, so skip them.
		if i == stepBranchCase || i == stepBranchMode || i == stepBranchCopy {
			continue
		}
		if !isInputStep(i) {
			continue
		}
		if m.values[i] == "" && m.inputs[i].Value() == "" {
			return i
		}
	}
	return stepConfirm
}

func isInputStep(step int) bool {
	return step > stepWelcome && step < stepConfirm && step != stepProject && step != stepBoardID && step != stepBranchCase && step != stepBranchMode && step != stepBranchCopy
}

// validationLabel returns a user-facing label for the current validation step.
func (m Model) validationLabel() string {
	switch m.step {
	case stepAuthType:
		return "Verifying credentials..."
	case stepProject:
		return "Fetching projects..."
	case stepBoardID:
		return "Fetching boards..."
	default:
		return "Validating..."
	}
}

// expandTilde replaces a leading ~ with the user's home directory.
func expandTilde(s string) string {
	if !strings.HasPrefix(s, "~") {
		return s
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return s
	}
	return filepath.Join(home, s[1:])
}

// cleanDomain strips protocol prefix and trailing slashes from a domain string.
func cleanDomain(s string) string {
	s = strings.TrimSpace(s)
	for _, prefix := range []string{"https://", "http://"} {
		if len(s) > len(prefix) && strings.HasPrefix(strings.ToLower(s), prefix) {
			s = s[len(prefix):]
		}
	}
	s = strings.TrimRight(s, "/")
	return s
}
