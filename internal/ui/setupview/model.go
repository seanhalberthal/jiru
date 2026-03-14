package setupview

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/seanhalberthal/jiratui/internal/client"
	"github.com/seanhalberthal/jiratui/internal/config"
	"github.com/seanhalberthal/jiratui/internal/jira"
	"github.com/seanhalberthal/jiratui/internal/theme"
	"github.com/seanhalberthal/jiratui/internal/validate"
)

const (
	stepWelcome = iota
	stepDomain
	stepUser
	stepAPIToken
	stepAuthType
	stepProject
	stepBoardID
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
		title:       "Welcome to jiratui",
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
	stepConfirm: {
		title:       "Confirm",
		description: "Review your settings below.\nPress enter to save, or ctrl+b to go back and edit.",
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

	// Board picker state.
	boards             []jira.Board // Available boards for selection.
	boardCursor        int          // 0 = "None", 1+ = board index.
	boardsLoaded       bool         // True once boards have been fetched.
	boardsFetchProject string       // Project key used for the last board fetch.
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
		m.inputs[i].Width = min(width-10, 80)
	}
}

// Done returns true when the wizard has completed successfully.
func (m Model) Done() bool { return m.done }

// Quit returns true when the user chose to exit.
func (m Model) Quit() bool { return m.quit }

// InputActive returns true when a text input is focused.
func (m Model) InputActive() bool {
	return isInputStep(m.step)
}

// Config returns the completed config from wizard values.
func (m Model) Config() *config.Config {
	cfg := &config.Config{
		Domain:   cleanDomain(m.values[stepDomain]),
		User:     m.values[stepUser],
		APIToken: m.values[stepAPIToken],
		AuthType: m.values[stepAuthType],
		Project:  m.values[stepProject],
	}
	if bid := m.values[stepBoardID]; bid != "" {
		if id, err := strconv.Atoi(bid); err == nil {
			cfg.BoardID = id
		}
	}
	return cfg
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

		// ctrl+c and esc always quit the wizard.
		if msg.String() == "ctrl+c" || msg.String() == "esc" {
			m.quit = true
			return m, nil
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
				m.done = true
				return m, nil
			}

		case stepProject:
			return m.handleProjectPicker(msg)

		case stepBoardID:
			return m.handleBoardPicker(msg)

		default:
			// Input steps.
			if msg.String() == "enter" {
				value := strings.TrimSpace(m.inputs[m.step].Value())
				// Clean domain input.
				if m.step == stepDomain {
					value = cleanDomain(value)
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
	case "enter":
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
	case "enter":
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
	cfg := &config.Config{
		Domain:   cleanDomain(m.values[stepDomain]),
		User:     m.values[stepUser],
		APIToken: m.values[stepAPIToken],
		AuthType: m.values[stepAuthType],
		Project:  m.values[stepProject],
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

	s := steps[m.step]
	contentWidth := min(m.width-8, 80)

	titleStyle := theme.StyleTitle.MarginBottom(1)
	descStyle := theme.StyleSubtle
	errStyle := theme.StyleError
	stepIndicator := theme.StyleSubtle.Render(fmt.Sprintf("Step %d of %d", m.step, totalSteps-1))

	var sections []string

	// Title + step indicator.
	if m.step == stepWelcome {
		sections = append(sections, titleStyle.Render(s.title))
	} else {
		sections = append(sections, lipgloss.JoinHorizontal(lipgloss.Top,
			titleStyle.Render(s.title),
			"  ",
			stepIndicator,
		))
	}

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

	// Footer — consistent across all steps.
	var footerParts []string

	// enter action varies by step.
	switch m.step {
	case stepWelcome:
		footerParts = append(footerParts, fmt.Sprintf("%s %s",
			theme.StyleHelpKey.Render("enter"), theme.StyleHelpDesc.Render("start")))
	case stepConfirm:
		footerParts = append(footerParts, fmt.Sprintf("%s %s",
			theme.StyleHelpKey.Render("enter"), theme.StyleHelpDesc.Render("save")))
	case stepProject, stepBoardID:
		footerParts = append(footerParts, fmt.Sprintf("%s %s",
			theme.StyleHelpKey.Render("↑/↓"), theme.StyleHelpDesc.Render("navigate")))
		footerParts = append(footerParts, fmt.Sprintf("%s %s",
			theme.StyleHelpKey.Render("enter"), theme.StyleHelpDesc.Render("select")))
	default:
		footerParts = append(footerParts, fmt.Sprintf("%s %s",
			theme.StyleHelpKey.Render("enter"), theme.StyleHelpDesc.Render("next")))
	}

	// ctrl+b back (shown on all steps except welcome).
	if m.step > stepWelcome {
		footerParts = append(footerParts, fmt.Sprintf("%s %s",
			theme.StyleHelpKey.Render("ctrl+b"), theme.StyleHelpDesc.Render("back")))
	}

	// esc always quits.
	footerParts = append(footerParts, fmt.Sprintf("%s %s",
		theme.StyleHelpKey.Render("esc"), theme.StyleHelpDesc.Render("quit")))

	footer := strings.Join(footerParts, "  ")

	// Centre the content on screen.
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColourPrimary).
		Padding(1, 3).
		Width(contentWidth).
		Render(content)

	placed := lipgloss.Place(m.width, m.height-2, lipgloss.Center, lipgloss.Center, box)
	return lipgloss.JoinVertical(lipgloss.Left, placed, footer)
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
		"\nAPI token: OS keychain • Other settings: ~/.config/jiratui/config.env")

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
	return step > stepWelcome && step < stepConfirm && step != stepProject && step != stepBoardID
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
