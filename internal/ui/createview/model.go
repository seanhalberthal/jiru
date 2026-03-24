package createview

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/seanhalberthal/jiru/internal/client"
	"github.com/seanhalberthal/jiru/internal/jira"
	"github.com/seanhalberthal/jiru/internal/theme"
)

// Steps in the create wizard.
const (
	stepProject = iota
	stepIssueType
	stepSummary
	stepPriority
	stepAssignee
	stepLabels
	stepParent
	stepDescription
	stepConfirm
	totalSteps
)

// stepMeta describes a wizard step.
type stepMeta struct {
	title       string
	description string
	placeholder string
	required    bool
}

var steps = [totalSteps]stepMeta{
	stepProject: {
		title:       "Project",
		description: "Select the project for the new issue.\nUse ↑/↓ to navigate, enter to select.",
		required:    true,
	},
	stepIssueType: {
		title:       "Issue Type",
		description: "Select the issue type.\nUse ↑/↓ to navigate, enter to select.",
		required:    true,
	},
	stepSummary: {
		title:       "Summary",
		description: "Enter a brief summary for the issue.",
		placeholder: "Issue summary",
		required:    true,
	},
	stepPriority: {
		title:       "Priority",
		description: "Select the priority level.\nUse ↑/↓ to navigate, enter to select.",
	},
	stepAssignee: {
		title:       "Assignee",
		description: "Start typing to search for a user.\nResults update as you type. Press enter to confirm.",
		placeholder: "Search for a user...",
	},
	stepLabels: {
		title:       "Labels",
		description: "Enter labels separated by commas.\nTab to autocomplete from available labels.",
		placeholder: "label1, label2, ...",
	},
	stepParent: {
		title:       "Parent Issue",
		description: "Enter the parent issue key (e.g., PROJ-123).\nLeave blank for no parent.",
		placeholder: "PROJ-123",
	},
	stepDescription: {
		title:       "Description",
		description: "Enter a description for the issue.\nUse Jira wiki markup for formatting.",
		placeholder: "Issue description...",
	},
	stepConfirm: {
		title:       "Confirm",
		description: "Review the issue details below.\nPress enter to create, or ctrl+b to go back.",
	},
}

// Internal messages for async operations.
type projectsLoadedMsg struct {
	projects []jira.Project
	err      error
}

type issueTypesLoadedMsg struct {
	types []jira.IssueTypeInfo
	err   error
}

type customFieldsLoadedMsg struct {
	fields []jira.CustomFieldDef
	err    error
}

type prioritiesLoadedMsg struct {
	priorities []string
	err        error
}

type labelsLoadedMsg struct {
	labels []string
	err    error
}

type userSearchResultMsg struct {
	users []client.UserInfo
}

type issueCreatedMsg struct {
	key string
	err error
}

// Model is the create issue wizard.
type Model struct {
	step   int
	width  int
	height int

	// Text inputs for free-form fields.
	inputs [totalSteps]textinput.Model
	values [totalSteps]string

	// Picker data.
	projects      []jira.Project
	projectCursor int
	projectLoaded bool

	issueTypes            []jira.IssueTypeInfo
	issueTypeCursor       int
	issueTypeLoaded       bool
	issueTypeFetchProject string

	priorities     []string
	priorityCursor int
	priorityLoaded bool

	labels       []string // Available labels for autocomplete.
	labelsLoaded bool

	// Assignee search.
	userResults    []client.UserInfo
	userCursor     int
	userSearchTerm string

	// State.
	loading    bool
	spinner    spinner.Model
	errMsg     string
	done       bool
	quit       bool
	createdKey string // Key of the created issue.

	// Custom fields.
	customFields  []jira.CustomFieldDef
	customValues  map[string]string // field ID → current value
	customCursors map[string]int    // field ID → picker cursor for option fields
	customInput   textinput.Model   // reusable text input for string/number fields
	customLoaded  bool

	// Client for API calls.
	client  client.JiraClient
	project string // Pre-selected project from config.

	// Scroll offset for long picker lists.
	scrollOffset int
}

// New creates a new create issue wizard.
func New(c client.JiraClient) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(theme.ColourPrimary)

	m := Model{
		step:    stepProject,
		client:  c,
		spinner: s,
		project: c.Config().Project,
	}

	for i := range m.inputs {
		ti := textinput.New()
		ti.CharLimit = 1000
		ti.Width = 60
		if steps[i].placeholder != "" {
			ti.Placeholder = steps[i].placeholder
		}
		m.inputs[i] = ti
	}

	return m
}

// Init starts the wizard by fetching projects.
func (m Model) Init() tea.Cmd {
	m.loading = true
	return tea.Batch(m.spinner.Tick, m.fetchProjects())
}

// Done returns true (once) when the issue has been created.
func (m *Model) Done() bool {
	d := m.done
	m.done = false
	return d
}

// Quit returns true (once) when the user cancelled.
func (m *Model) Quit() bool {
	q := m.quit
	m.quit = false
	return q
}

// CreatedKey returns the key of the newly created issue (once).
func (m *Model) CreatedKey() string {
	k := m.createdKey
	m.createdKey = ""
	return k
}

// InputActive returns true when a text input is focused.
func (m Model) InputActive() bool {
	if isInputStep(m.step) {
		return true
	}
	if m.isCustomFieldStep(m.step) {
		idx := m.customFieldIndex(m.step)
		if idx >= 0 && idx < len(m.customFields) {
			ft := m.customFields[idx].FieldType
			return ft == "string" || ft == "number"
		}
	}
	return false
}

// SetSize updates terminal dimensions.
func (m *Model) SetSize(w, h int) {
	m.width = w
	m.height = h
	for i := range m.inputs {
		m.inputs[i].Width = min(w-10, 78) // cap at 78 so prompt (2 chars) fits within content area of 80
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case projectsLoadedMsg:
		m.loading = false
		m.projectLoaded = true
		if msg.err != nil {
			m.errMsg = fmt.Sprintf("Failed to fetch projects: %s", msg.err)
		} else {
			m.projects = msg.projects
			// Pre-select configured project.
			if m.project != "" {
				for i, p := range m.projects {
					if p.Key == m.project {
						m.projectCursor = i
						break
					}
				}
			}
		}
		return m, nil

	case issueTypesLoadedMsg:
		m.loading = false
		m.issueTypeLoaded = true
		if msg.err != nil {
			m.errMsg = fmt.Sprintf("Failed to fetch issue types: %s", msg.err)
		} else {
			m.issueTypes = msg.types
			m.issueTypeCursor = 0
		}
		return m, nil

	case customFieldsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			// Non-fatal — proceed without custom fields.
			m.customLoaded = true
			m.customFields = nil
		} else {
			m.SetCustomFields(msg.fields)
		}
		return m, nil

	case prioritiesLoadedMsg:
		m.loading = false
		m.priorityLoaded = true
		if msg.err != nil {
			// Non-fatal — skip priority selection.
			m.errMsg = ""
			m.priorities = nil
		} else {
			m.priorities = msg.priorities
			m.priorityCursor = 0
		}
		return m, nil

	case labelsLoadedMsg:
		m.labelsLoaded = true
		if msg.err == nil {
			m.labels = msg.labels
		}
		return m, nil

	case userSearchResultMsg:
		m.userResults = msg.users
		m.userCursor = 0
		return m, nil

	case issueCreatedMsg:
		m.loading = false
		if msg.err != nil {
			m.errMsg = fmt.Sprintf("Failed to create issue: %s", msg.err)
			return m, nil
		}
		m.createdKey = msg.key
		m.done = true
		return m, nil

	case tea.KeyMsg:
		if m.loading {
			if msg.String() == "ctrl+c" || msg.String() == "esc" {
				m.quit = true
			}
			return m, nil
		}

		if msg.String() == "ctrl+c" || msg.String() == "esc" {
			m.quit = true
			return m, nil
		}

		if msg.String() == "ctrl+b" {
			return m.goBack()
		}

		switch {
		case m.step == stepProject:
			return m.handleProjectPicker(msg)
		case m.step == stepIssueType:
			return m.handleIssueTypePicker(msg)
		case m.step == stepPriority:
			return m.handlePriorityPicker(msg)
		case m.step == stepAssignee:
			if handled, model, cmd := m.handleAssigneeInput(msg); handled {
				return model, cmd
			}
		case m.step == m.confirmStep():
			if msg.String() == "enter" {
				m.loading = true
				return m, tea.Batch(m.spinner.Tick, m.createIssue())
			}
		case m.isCustomFieldStep(m.step):
			return m.handleCustomFieldStep(msg)
		default:
			// Text input steps (summary, labels, parent, description).
			if msg.String() == "enter" {
				return m.advanceFromInput()
			}
		}
	}

	// Update current text input.
	if isInputStep(m.step) && !m.loading {
		var cmd tea.Cmd
		m.inputs[m.step], cmd = m.inputs[m.step].Update(msg)

		// Live user search for assignee field.
		if m.step == stepAssignee {
			val := m.inputs[stepAssignee].Value()
			if val != m.userSearchTerm && len(val) >= 2 {
				m.userSearchTerm = val
				return m, tea.Batch(cmd, m.searchUsers(val))
			}
			if len(val) < 2 {
				m.userResults = nil
				m.userSearchTerm = ""
			}
		}

		return m, cmd
	}

	// Update custom field text input.
	if m.isCustomFieldStep(m.step) && !m.loading {
		idx := m.customFieldIndex(m.step)
		if idx >= 0 && idx < len(m.customFields) {
			cf := m.customFields[idx]
			if cf.FieldType == "string" || cf.FieldType == "number" {
				var cmd tea.Cmd
				m.customInput, cmd = m.customInput.Update(msg)
				return m, cmd
			}
		}
	}

	return m, nil
}

func (m Model) goBack() (Model, tea.Cmd) {
	if m.step <= stepProject {
		m.quit = true
		return m, nil
	}
	m.step--
	// If going back from confirm and there are no custom fields, go to description.
	if m.step > stepDescription && m.customLoaded && len(m.customFields) == 0 {
		m.step = stepDescription
	}
	m.errMsg = ""
	m.scrollOffset = 0
	return m, m.onStepEnter()
}

func (m Model) advanceFromInput() (Model, tea.Cmd) {
	value := strings.TrimSpace(m.inputs[m.step].Value())
	if steps[m.step].required && value == "" {
		m.errMsg = "This field is required"
		return m, nil
	}
	m.values[m.step] = value
	m.errMsg = ""
	m.inputs[m.step].Blur()
	m.step++
	m.scrollOffset = 0

	// After description, if custom fields haven't been loaded yet, they'll be
	// fetched in onStepEnter. If already loaded and there are none, skip to confirm.
	if m.step > stepDescription && m.customLoaded && len(m.customFields) == 0 {
		m.step = m.confirmStep()
	}

	return m, m.onStepEnter()
}

func (m Model) handleProjectPicker(msg tea.KeyMsg) (Model, tea.Cmd) {
	if !m.projectLoaded {
		return m, nil
	}
	switch msg.String() {
	case "enter":
		if len(m.projects) == 0 {
			m.errMsg = "No projects available"
			return m, nil
		}
		p := m.projects[m.projectCursor]
		m.values[stepProject] = p.Key
		m.errMsg = ""
		m.step++
		m.scrollOffset = 0
		return m, m.onStepEnter()
	case "up", "k":
		if m.projectCursor > 0 {
			m.projectCursor--
			m.adjustScroll(m.projectCursor, len(m.projects))
		}
	case "down", "j":
		if m.projectCursor < len(m.projects)-1 {
			m.projectCursor++
			m.adjustScroll(m.projectCursor, len(m.projects))
		}
	}
	return m, nil
}

func (m Model) handleIssueTypePicker(msg tea.KeyMsg) (Model, tea.Cmd) {
	if !m.issueTypeLoaded {
		return m, nil
	}
	switch msg.String() {
	case "enter":
		if len(m.issueTypes) == 0 {
			m.errMsg = "No issue types available"
			return m, nil
		}
		it := m.issueTypes[m.issueTypeCursor]
		m.values[stepIssueType] = it.Name
		// Reset custom fields when issue type changes.
		m.customLoaded = false
		m.customFields = nil
		m.customValues = nil
		m.customCursors = nil
		m.errMsg = ""
		m.step++
		m.scrollOffset = 0
		return m, m.onStepEnter()
	case "up", "k":
		if m.issueTypeCursor > 0 {
			m.issueTypeCursor--
			m.adjustScroll(m.issueTypeCursor, len(m.issueTypes))
		}
	case "down", "j":
		if m.issueTypeCursor < len(m.issueTypes)-1 {
			m.issueTypeCursor++
			m.adjustScroll(m.issueTypeCursor, len(m.issueTypes))
		}
	}
	return m, nil
}

func (m Model) handlePriorityPicker(msg tea.KeyMsg) (Model, tea.Cmd) {
	if !m.priorityLoaded {
		return m, nil
	}
	n := len(m.priorities) + 1 // +1 for "None" option.
	switch msg.String() {
	case "enter":
		if m.priorityCursor == 0 {
			m.values[stepPriority] = ""
		} else {
			m.values[stepPriority] = m.priorities[m.priorityCursor-1]
		}
		m.errMsg = ""
		m.step++
		m.scrollOffset = 0
		return m, m.onStepEnter()
	case "up", "k":
		if m.priorityCursor > 0 {
			m.priorityCursor--
			m.adjustScroll(m.priorityCursor, n)
		}
	case "down", "j":
		if m.priorityCursor < n-1 {
			m.priorityCursor++
			m.adjustScroll(m.priorityCursor, n)
		}
	}
	return m, nil
}

func (m Model) handleAssigneeInput(msg tea.KeyMsg) (bool, Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		// If user results are showing and one is selected, use the account ID.
		if len(m.userResults) > 0 && m.userCursor < len(m.userResults) {
			selected := m.userResults[m.userCursor]
			m.values[stepAssignee] = selected.AccountID
			m.inputs[stepAssignee].SetValue(selected.DisplayName)
		} else {
			m.values[stepAssignee] = strings.TrimSpace(m.inputs[stepAssignee].Value())
		}
		m.errMsg = ""
		m.inputs[stepAssignee].Blur()
		m.userResults = nil
		m.step++
		m.scrollOffset = 0
		return true, m, m.onStepEnter()
	case "tab":
		if len(m.userResults) > 0 && m.userCursor < len(m.userResults) {
			selected := m.userResults[m.userCursor]
			m.inputs[stepAssignee].SetValue(selected.DisplayName)
			m.values[stepAssignee] = selected.AccountID
			m.userResults = nil
		}
		return true, m, nil
	case "up":
		if len(m.userResults) > 0 && m.userCursor > 0 {
			m.userCursor--
		}
		return true, m, nil
	case "down":
		if len(m.userResults) > 0 && m.userCursor < len(m.userResults)-1 {
			m.userCursor++
		}
		return true, m, nil
	}
	// Not handled — let the text input process the key.
	return false, m, nil
}

func (m Model) handleCustomFieldStep(msg tea.KeyMsg) (Model, tea.Cmd) {
	idx := m.customFieldIndex(m.step)
	if idx < 0 || idx >= len(m.customFields) {
		return m, nil
	}
	cf := m.customFields[idx]

	switch cf.FieldType {
	case "option":
		switch msg.String() {
		case "enter":
			if len(cf.AllowedValues) > 0 {
				cursor := m.customCursors[cf.ID]
				m.customValues[cf.ID] = cf.AllowedValues[cursor]
			}
			m.errMsg = ""
			m.step++
			m.scrollOffset = 0
			return m, m.onStepEnter()
		case "up", "k":
			cursor := m.customCursors[cf.ID]
			if cursor > 0 {
				m.customCursors[cf.ID] = cursor - 1
				m.adjustScroll(cursor-1, len(cf.AllowedValues))
			}
		case "down", "j":
			cursor := m.customCursors[cf.ID]
			if cursor < len(cf.AllowedValues)-1 {
				m.customCursors[cf.ID] = cursor + 1
				m.adjustScroll(cursor+1, len(cf.AllowedValues))
			}
		}
	case "string", "number":
		if msg.String() == "enter" {
			val := strings.TrimSpace(m.customInput.Value())
			if cf.Required && val == "" {
				m.errMsg = "This field is required"
				return m, nil
			}
			m.customValues[cf.ID] = val
			m.customInput.Blur()
			m.errMsg = ""
			m.step++
			m.scrollOffset = 0
			return m, m.onStepEnter()
		}
	case "unsupported":
		if msg.String() == "enter" {
			m.step++
			m.scrollOffset = 0
			return m, m.onStepEnter()
		}
	}
	return m, nil
}

// onStepEnter handles transitions to a new step.
func (m *Model) onStepEnter() tea.Cmd {
	switch m.step {
	case stepProject:
		if m.projectLoaded {
			return nil
		}
		m.loading = true
		return tea.Batch(m.spinner.Tick, m.fetchProjects())

	case stepIssueType:
		proj := m.values[stepProject]
		if m.issueTypeLoaded && m.issueTypeFetchProject == proj {
			return nil
		}
		m.loading = true
		m.issueTypeLoaded = false
		m.issueTypeFetchProject = proj
		return tea.Batch(m.spinner.Tick, m.fetchIssueTypes(proj))

	case stepPriority:
		if m.priorityLoaded {
			return nil
		}
		m.loading = true
		// Also fetch labels in parallel for later use.
		cmds := []tea.Cmd{m.spinner.Tick, m.fetchPriorities()}
		if !m.labelsLoaded {
			cmds = append(cmds, m.fetchLabels())
		}
		return tea.Batch(cmds...)

	case stepAssignee:
		m.inputs[stepAssignee].Focus()
		m.userResults = nil
		m.userSearchTerm = ""
		return textinput.Blink
	}

	if isInputStep(m.step) {
		m.inputs[m.step].Focus()
		return textinput.Blink
	}

	// Custom field steps.
	if m.isCustomFieldStep(m.step) {
		// If custom fields haven't been loaded yet, fetch them.
		if !m.customLoaded {
			it := m.issueTypes[m.issueTypeCursor]
			if it.ID != "" {
				m.loading = true
				return tea.Batch(m.spinner.Tick, m.fetchCustomFields(m.values[stepProject], it.ID))
			}
			// No issue type ID — skip custom fields.
			m.customLoaded = true
			m.customFields = nil
			m.step = m.confirmStep()
			return nil
		}

		idx := m.customFieldIndex(m.step)
		if idx >= 0 && idx < len(m.customFields) {
			cf := m.customFields[idx]
			if cf.FieldType == "string" || cf.FieldType == "number" {
				m.customInput = textinput.New()
				m.customInput.CharLimit = 1000
				m.customInput.Width = min(m.width-10, 80)
				if cf.FieldType == "number" {
					m.customInput.Placeholder = "Enter a number"
				} else {
					m.customInput.Placeholder = cf.Name
				}
				// Restore previous value if any.
				if prev, ok := m.customValues[cf.ID]; ok {
					m.customInput.SetValue(prev)
				}
				m.customInput.Focus()
				return textinput.Blink
			}
		}
		return nil
	}

	return nil
}

// adjustScroll adjusts the scroll offset so the cursor is visible.
func (m *Model) adjustScroll(cursor, total int) {
	maxVisible := m.maxPickerVisible()
	if maxVisible <= 0 || total <= maxVisible {
		m.scrollOffset = 0
		return
	}
	if cursor < m.scrollOffset {
		m.scrollOffset = cursor
	}
	if cursor >= m.scrollOffset+maxVisible {
		m.scrollOffset = cursor - maxVisible + 1
	}
}

func (m Model) maxPickerVisible() int {
	// Reserve space for title, description, border, footer.
	available := m.height - 12
	if available < 5 {
		return 5
	}
	return available
}

// Commands.

func (m Model) fetchProjects() tea.Cmd {
	c := m.client
	return func() tea.Msg {
		projects, err := c.Projects()
		return projectsLoadedMsg{projects: projects, err: err}
	}
}

func (m Model) fetchIssueTypes(project string) tea.Cmd {
	c := m.client
	return func() tea.Msg {
		types, err := c.IssueTypesWithID(project)
		return issueTypesLoadedMsg{types: types, err: err}
	}
}

func (m Model) fetchCustomFields(project, issueTypeID string) tea.Cmd {
	c := m.client
	return func() tea.Msg {
		fields, err := c.CreateMetaFields(project, issueTypeID)
		return customFieldsLoadedMsg{fields: fields, err: err}
	}
}

func (m Model) fetchPriorities() tea.Cmd {
	c := m.client
	return func() tea.Msg {
		meta, err := c.JQLMetadata()
		if err != nil {
			return prioritiesLoadedMsg{err: err}
		}
		return prioritiesLoadedMsg{priorities: meta.Priorities}
	}
}

func (m Model) fetchLabels() tea.Cmd {
	c := m.client
	return func() tea.Msg {
		meta, err := c.JQLMetadata()
		if err != nil {
			return labelsLoadedMsg{err: err}
		}
		return labelsLoadedMsg{labels: meta.Labels}
	}
}

func (m Model) searchUsers(prefix string) tea.Cmd {
	c := m.client
	project := m.values[stepProject]
	return func() tea.Msg {
		users, err := c.SearchUsers(project, prefix)
		if err != nil {
			return userSearchResultMsg{users: nil}
		}
		return userSearchResultMsg{users: users}
	}
}

func (m Model) createIssue() tea.Cmd {
	c := m.client

	// Look up the project type for the selected project.
	var projectType string
	for _, p := range m.projects {
		if p.Key == m.values[stepProject] {
			projectType = p.Type
			break
		}
	}

	req := &client.CreateIssueRequest{
		Project:     m.values[stepProject],
		ProjectType: projectType,
		IssueType:   m.values[stepIssueType],
		Summary:     m.values[stepSummary],
		Description: m.values[stepDescription],
		Priority:    m.values[stepPriority],
		Assignee:    m.values[stepAssignee],
		ParentKey:   m.values[stepParent],
	}
	if labels := m.values[stepLabels]; labels != "" {
		for _, l := range strings.Split(labels, ",") {
			l = strings.TrimSpace(l)
			if l != "" {
				req.Labels = append(req.Labels, l)
			}
		}
	}

	// Custom fields.
	if len(m.customValues) > 0 {
		req.CustomFields = make(map[string]any)
		for _, cf := range m.customFields {
			val, ok := m.customValues[cf.ID]
			if !ok || val == "" {
				continue
			}
			switch cf.FieldType {
			case "string":
				req.CustomFields[cf.ID] = val
			case "number":
				if f, err := strconv.ParseFloat(val, 64); err == nil {
					req.CustomFields[cf.ID] = f
				} else {
					req.CustomFields[cf.ID] = val
				}
			case "option":
				req.CustomFields[cf.ID] = map[string]string{"value": val}
			}
		}
	}

	return func() tea.Msg {
		resp, err := c.CreateIssue(req)
		if err != nil {
			return issueCreatedMsg{err: err}
		}
		return issueCreatedMsg{key: resp.Key}
	}
}

func (m Model) View() string {
	if m.width == 0 {
		return ""
	}

	contentWidth := min(m.width-8, 80)

	titleStyle := theme.StyleTitle.MarginBottom(1)
	descStyle := theme.StyleSubtle
	errStyle := theme.StyleError

	// Build step title and description.
	var stepTitle, stepDesc string
	var stepRequired bool

	switch {
	case m.step < totalSteps:
		s := steps[m.step]
		stepTitle = s.title
		stepDesc = s.description
		stepRequired = s.required
	case m.isCustomFieldStep(m.step):
		idx := m.customFieldIndex(m.step)
		if idx >= 0 && idx < len(m.customFields) {
			cf := m.customFields[idx]
			stepTitle = cf.Name
			stepRequired = cf.Required
			switch cf.FieldType {
			case "option":
				stepDesc = "Select a value.\nUse ↑/↓ to navigate, enter to select."
			case "number":
				stepDesc = "Enter a number value."
			case "unsupported":
				stepDesc = "This field type is not supported in the TUI. Press enter to skip."
			default:
				stepDesc = "Enter a value."
			}
		}
	case m.step == m.confirmStep():
		stepTitle = "Confirm"
		stepDesc = "Review the issue details below.\nPress enter to create, or ctrl+b to go back."
	}

	stepIndicator := theme.StyleSubtle.Render(
		fmt.Sprintf("Step %d of %d", m.step+1, m.totalSteps()))

	var sections []string
	sections = append(sections, lipgloss.JoinHorizontal(lipgloss.Top,
		titleStyle.Render("Create Issue — "+stepTitle),
		"  ",
		stepIndicator,
	))
	sections = append(sections, descStyle.Render(stepDesc))
	sections = append(sections, "")

	switch {
	case m.step == stepProject:
		sections = append(sections, m.renderProjectPicker()...)
	case m.step == stepIssueType:
		sections = append(sections, m.renderIssueTypePicker()...)
	case m.step == stepPriority:
		sections = append(sections, m.renderPriorityPicker()...)
	case m.step == stepAssignee:
		sections = append(sections, m.renderAssigneeInput()...)
	case m.step == m.confirmStep():
		sections = append(sections, m.renderSummary())
	case m.isCustomFieldStep(m.step):
		sections = append(sections, m.renderCustomField()...)
	default:
		sections = append(sections, m.inputs[m.step].View())
		if m.step == stepLabels && m.labelsLoaded && len(m.labels) > 0 {
			// Show label hints.
			val := m.inputs[stepLabels].Value()
			hints := m.labelHints(val)
			if len(hints) > 0 {
				sections = append(sections, theme.StyleSubtle.Render("Available: "+strings.Join(hints, ", ")))
			}
		}
	}

	if m.loading {
		sections = append(sections, fmt.Sprintf("%s %s",
			m.spinner.View(), theme.StyleSubtle.Render("Loading...")))
	} else if m.errMsg != "" {
		sections = append(sections, errStyle.Render(m.errMsg))
	}

	if stepRequired && !m.loading {
		sections = append(sections, theme.StyleSubtle.Render("* required"))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	// Footer.
	footer := m.renderFooter()

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColourPrimary).
		Padding(1, 3).
		Width(contentWidth).
		Render(content)

	placed := lipgloss.Place(m.width, m.height-2, lipgloss.Center, lipgloss.Center, box)
	return lipgloss.JoinVertical(lipgloss.Left, placed, footer)
}

func (m Model) renderProjectPicker() []string {
	if m.loading {
		return []string{fmt.Sprintf("%s %s", m.spinner.View(), theme.StyleSubtle.Render("Fetching projects..."))}
	}
	if len(m.projects) == 0 {
		return []string{theme.StyleSubtle.Render("No projects found.")}
	}
	return m.renderPickerList(m.projectCursor, len(m.projects), func(i int) string {
		p := m.projects[i]
		return fmt.Sprintf("%s (%s)", p.Name, p.Key)
	})
}

func (m Model) renderIssueTypePicker() []string {
	if m.loading {
		return []string{fmt.Sprintf("%s %s", m.spinner.View(), theme.StyleSubtle.Render("Fetching issue types..."))}
	}
	if len(m.issueTypes) == 0 {
		return []string{theme.StyleSubtle.Render("No issue types found.")}
	}
	return m.renderPickerList(m.issueTypeCursor, len(m.issueTypes), func(i int) string {
		return m.issueTypes[i].Name
	})
}

func (m Model) renderPriorityPicker() []string {
	if m.loading {
		return []string{fmt.Sprintf("%s %s", m.spinner.View(), theme.StyleSubtle.Render("Fetching priorities..."))}
	}

	n := len(m.priorities) + 1
	return m.renderPickerList(m.priorityCursor, n, func(i int) string {
		if i == 0 {
			return "None — use project default"
		}
		return m.priorities[i-1]
	})
}

func (m Model) renderAssigneeInput() []string {
	var parts []string
	parts = append(parts, m.inputs[stepAssignee].View())

	if len(m.userResults) > 0 {
		selectedStyle := lipgloss.NewStyle().Foreground(theme.ColourPrimary).Bold(true)
		normalStyle := lipgloss.NewStyle()

		var items []string
		for i, u := range m.userResults {
			if i == m.userCursor {
				items = append(items, selectedStyle.Render("▸ "+u.DisplayName))
			} else {
				items = append(items, normalStyle.Render("  "+u.DisplayName))
			}
		}
		parts = append(parts, strings.Join(items, "\n"))
		parts = append(parts, theme.StyleSubtle.Render("↑/↓ navigate  tab accept  enter confirm"))
	}

	return parts
}

// renderPickerList renders a scrollable picker list.
func (m Model) renderPickerList(cursor, total int, label func(int) string) []string {
	selectedStyle := lipgloss.NewStyle().Foreground(theme.ColourPrimary).Bold(true)
	normalStyle := lipgloss.NewStyle()

	maxVisible := m.maxPickerVisible()
	start := m.scrollOffset
	end := start + maxVisible
	if end > total {
		end = total
	}

	var items []string

	if start > 0 {
		items = append(items, theme.StyleSubtle.Render("  ↑ more"))
	}

	for i := start; i < end; i++ {
		l := label(i)
		if i == cursor {
			items = append(items, selectedStyle.Render("▸ "+l))
		} else {
			items = append(items, normalStyle.Render("  "+l))
		}
	}

	if end < total {
		items = append(items, theme.StyleSubtle.Render("  ↓ more"))
	}

	return []string{strings.Join(items, "\n")}
}

func (m Model) renderCustomField() []string {
	idx := m.customFieldIndex(m.step)
	if idx < 0 || idx >= len(m.customFields) {
		return nil
	}
	cf := m.customFields[idx]

	switch cf.FieldType {
	case "option":
		if len(cf.AllowedValues) == 0 {
			return []string{theme.StyleSubtle.Render("No options available.")}
		}
		cursor := m.customCursors[cf.ID]
		return m.renderPickerList(cursor, len(cf.AllowedValues), func(i int) string {
			return cf.AllowedValues[i]
		})
	case "string", "number":
		return []string{m.customInput.View()}
	case "unsupported":
		return []string{theme.StyleSubtle.Render("(not supported in TUI — press enter to skip)")}
	}
	return nil
}

func (m Model) renderSummary() string {
	labelStyle := lipgloss.NewStyle().Bold(true).Width(14)
	valueStyle := lipgloss.NewStyle().Foreground(theme.ColourPrimary)
	subtleStyle := lipgloss.NewStyle().Foreground(theme.ColourSubtle)

	// Resolve project display name.
	projectDisplay := m.values[stepProject]
	for _, p := range m.projects {
		if p.Key == m.values[stepProject] {
			projectDisplay = fmt.Sprintf("%s (%s)", p.Name, p.Key)
			break
		}
	}

	rows := []struct {
		label string
		value string
	}{
		{"Project", projectDisplay},
		{"Issue Type", m.values[stepIssueType]},
		{"Summary", m.values[stepSummary]},
		{"Priority", m.values[stepPriority]},
		{"Assignee", m.inputs[stepAssignee].Value()},
		{"Labels", m.values[stepLabels]},
		{"Parent", m.values[stepParent]},
		{"Description", m.truncate(m.values[stepDescription], 60)},
	}

	// Add custom field rows.
	for _, cf := range m.customFields {
		if cf.FieldType == "unsupported" {
			continue
		}
		rows = append(rows, struct {
			label string
			value string
		}{cf.Name, m.customValues[cf.ID]})
	}

	var lines []string
	for _, r := range rows {
		display := r.value
		style := valueStyle
		if display == "" {
			display = "(not set)"
			style = subtleStyle
		}
		lines = append(lines, labelStyle.Render(r.label)+style.Render(display))
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderFooter() string {
	var parts []string

	switch {
	case m.step == stepProject || m.step == stepIssueType || m.step == stepPriority:
		parts = append(parts, fmt.Sprintf("%s %s",
			theme.StyleHelpKey.Render("↑/↓"), theme.StyleHelpDesc.Render("navigate")))
		parts = append(parts, fmt.Sprintf("%s %s",
			theme.StyleHelpKey.Render("enter"), theme.StyleHelpDesc.Render("select")))
	case m.step == m.confirmStep():
		parts = append(parts, fmt.Sprintf("%s %s",
			theme.StyleHelpKey.Render("enter"), theme.StyleHelpDesc.Render("create")))
	case m.isCustomFieldStep(m.step):
		idx := m.customFieldIndex(m.step)
		if idx >= 0 && idx < len(m.customFields) && m.customFields[idx].FieldType == "option" {
			parts = append(parts, fmt.Sprintf("%s %s",
				theme.StyleHelpKey.Render("↑/↓"), theme.StyleHelpDesc.Render("navigate")))
			parts = append(parts, fmt.Sprintf("%s %s",
				theme.StyleHelpKey.Render("enter"), theme.StyleHelpDesc.Render("select")))
		} else {
			parts = append(parts, fmt.Sprintf("%s %s",
				theme.StyleHelpKey.Render("enter"), theme.StyleHelpDesc.Render("next")))
		}
	default:
		parts = append(parts, fmt.Sprintf("%s %s",
			theme.StyleHelpKey.Render("enter"), theme.StyleHelpDesc.Render("next")))
	}

	if m.step > stepProject {
		parts = append(parts, fmt.Sprintf("%s %s",
			theme.StyleHelpKey.Render("ctrl+b"), theme.StyleHelpDesc.Render("back")))
	}

	parts = append(parts, fmt.Sprintf("%s %s",
		theme.StyleHelpKey.Render("esc"), theme.StyleHelpDesc.Render("cancel")))

	return strings.Join(parts, "  ")
}

func (m Model) labelHints(current string) []string {
	// Get the last token being typed.
	parts := strings.Split(current, ",")
	prefix := strings.TrimSpace(parts[len(parts)-1])
	if prefix == "" {
		return nil
	}
	prefix = strings.ToLower(prefix)

	var hints []string
	for _, l := range m.labels {
		if strings.HasPrefix(strings.ToLower(l), prefix) {
			hints = append(hints, l)
			if len(hints) >= 5 {
				break
			}
		}
	}
	return hints
}

func (m Model) truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func isInputStep(step int) bool {
	switch step {
	case stepSummary, stepAssignee, stepLabels, stepParent, stepDescription:
		return true
	}
	return false
}

func (m Model) confirmStep() int {
	return stepDescription + 1 + len(m.customFields)
}

func (m Model) totalSteps() int {
	return m.confirmStep() + 1
}

func (m Model) isCustomFieldStep(step int) bool {
	return step > stepDescription && step < m.confirmStep()
}

func (m Model) customFieldIndex(step int) int {
	return step - stepDescription - 1
}

// SetCustomFields sets the available custom fields after they're fetched.
func (m *Model) SetCustomFields(fields []jira.CustomFieldDef) {
	m.customFields = fields
	m.customLoaded = true
	m.customValues = make(map[string]string)
	m.customCursors = make(map[string]int)
}

// NeedsCustomFields returns true if the wizard is waiting for custom field metadata.
func (m Model) NeedsCustomFields() (project, issueTypeID string, needs bool) {
	if m.step == stepDescription+1 && !m.customLoaded && len(m.issueTypes) > 0 {
		it := m.issueTypes[m.issueTypeCursor]
		if it.ID != "" {
			return m.values[stepProject], it.ID, true
		}
	}
	return "", "", false
}
