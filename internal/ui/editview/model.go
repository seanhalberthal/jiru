package editview

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/seanhalberthal/jiru/internal/client"
	"github.com/seanhalberthal/jiru/internal/jira"
	"github.com/seanhalberthal/jiru/internal/markup"
	"github.com/seanhalberthal/jiru/internal/theme"
	"github.com/seanhalberthal/jiru/internal/validate"
)

const (
	fieldSummary     = 0
	fieldIssueType   = 1
	fieldParent      = 2
	fieldPriority    = 3
	fieldStoryPoints = 4
	fieldLabels      = 5
	fieldFixVersions = 6
	fieldDescription = 7
	numFields        = 8
)

const (
	boxMaxWidth       = 180
	boxChromeWidth    = 8
	labelWidth        = 14
	promptWidth       = 2
	overlayChromeRows = 25
)

// Model is the field editor overlay.
type Model struct {
	issueKey        string
	summary         textinput.Model
	issueTypes      []string
	issueTypeCursor int
	priorities      []string
	priorityCursor  int
	storyPoints     textinput.Model
	labels          textinput.Model
	fixVersions     textinput.Model
	parent          textinput.Model
	activeField     int
	editing         bool
	submitted       *client.EditIssueRequest
	validationErr   string
	dismissed       bool
	width           int
	height          int
	description     textarea.Model
	descViewport    viewport.Model
	descWidth       int
	descHeight      int
	inputWidth      int
	origDescription string
	// Original values for diff computation.
	origSummary     string
	origIssueType   string
	origPriority    string
	origStoryPoints *float64
	origLabels      []string
	origFixVersions []string
	origParent      string
}

// formatStoryPoints renders a story-point value, stripping the decimal when the
// value is whole (e.g., 3 → "3", 1.5 → "1.5").
func formatStoryPoints(sp float64) string {
	if sp == float64(int64(sp)) {
		return fmt.Sprintf("%d", int64(sp))
	}
	return fmt.Sprintf("%g", sp)
}

// New creates a new field editor for the given issue key.
func New(issueKey string) Model {
	summary := textinput.New()
	summary.Placeholder = "Summary"
	summary.CharLimit = 255
	summary.Width = 80

	storyPoints := textinput.New()
	storyPoints.Placeholder = "Story Points"
	storyPoints.CharLimit = 10
	storyPoints.Width = 80

	labels := textinput.New()
	labels.Placeholder = "Labels (comma-separated)"
	labels.CharLimit = 500
	labels.Width = 80

	fixVersions := textinput.New()
	fixVersions.Placeholder = "Fix Versions (comma-separated)"
	fixVersions.CharLimit = 500
	fixVersions.Width = 80

	parent := textinput.New()
	parent.Placeholder = "Parent issue key (e.g. PROJ-123) — clear to remove"
	parent.CharLimit = 255
	parent.Width = 80

	desc := textarea.New()
	desc.Placeholder = "Description (wiki markup)"
	desc.CharLimit = 0
	desc.SetHeight(8)
	desc.SetWidth(80)

	descVP := viewport.New(80, 8)

	return Model{
		issueKey:     issueKey,
		summary:      summary,
		storyPoints:  storyPoints,
		labels:       labels,
		fixVersions:  fixVersions,
		parent:       parent,
		description:  desc,
		descViewport: descVP,
	}
}

// SetIssue pre-populates the editor from the current issue.
func (m *Model) SetIssue(iss jira.Issue, priorities []string, issueTypes []string) {
	m.activeField = fieldSummary
	m.editing = false
	m.validationErr = ""
	m.blurFields()

	m.summary.SetValue(iss.Summary)
	m.summary.CursorStart()
	m.origSummary = iss.Summary
	m.origPriority = iss.Priority
	m.origLabels = iss.Labels
	m.labels.SetValue(strings.Join(iss.Labels, ", "))
	m.labels.CursorStart()
	m.priorities = priorities

	m.origIssueType = iss.IssueType
	m.issueTypes = issueTypes
	m.issueTypeCursor = 0
	for i, t := range issueTypes {
		if t == iss.IssueType {
			m.issueTypeCursor = i
			break
		}
	}

	m.origStoryPoints = iss.StoryPoints
	if iss.StoryPoints != nil {
		m.storyPoints.SetValue(formatStoryPoints(*iss.StoryPoints))
	} else {
		m.storyPoints.SetValue("")
	}
	m.storyPoints.CursorStart()

	m.origFixVersions = iss.FixVersions
	m.fixVersions.SetValue(strings.Join(iss.FixVersions, ", "))
	m.fixVersions.CursorStart()

	m.origParent = iss.ParentKey
	m.parent.SetValue(iss.ParentKey)
	m.parent.CursorStart()

	m.description.SetValue(iss.Description)
	// Move cursor to the very beginning (row 0, col 0) so it's visible.
	for m.description.Line() > 0 {
		m.description.CursorUp()
	}
	m.description.CursorStart()
	m.origDescription = iss.Description
	m.updateDescPreview()

	// Pre-select current priority.
	m.priorityCursor = 0
	for i, p := range priorities {
		if p == iss.Priority {
			m.priorityCursor = i
			break
		}
	}
}

// SubmittedEdit returns the edit request (once) and clears the sentinel.
func (m *Model) SubmittedEdit() *client.EditIssueRequest {
	s := m.submitted
	m.submitted = nil
	return s
}

// Dismissed returns true (once) if the user cancelled.
func (m *Model) Dismissed() bool {
	d := m.dismissed
	m.dismissed = false
	return d
}

// InputActive returns true (always suppresses global keys).
func (m Model) InputActive() bool {
	return true
}

// SetSize updates the overlay dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	// Box: rounded border (1 each side) + Padding(1, 3) (3 cols each side) = 8 cols of chrome.
	// Use most of the viewport, capped so it doesn't span absurdly wide terminals.
	boxWidth := min(width-4, boxMaxWidth)
	if boxWidth <= boxChromeWidth+labelWidth {
		return
	}
	contentWidth := boxWidth - boxChromeWidth
	// textinput renders a "> " prompt in addition to the editable region — its
	// .Width property only controls the editable part, so reserve promptWidth
	// chars or the row will overflow the box edge.
	inputWidth := contentWidth - labelWidth - promptWidth
	m.inputWidth = inputWidth
	m.summary.Width = inputWidth
	m.storyPoints.Width = inputWidth
	m.labels.Width = inputWidth
	m.fixVersions.Width = inputWidth
	m.parent.Width = inputWidth
	if m.issueKey != "" {
		// Description label is stacked above the textarea, so the textarea
		// itself can use the full content width.
		m.description.SetWidth(contentWidth)
		// Rows consumed by the rest of the overlay: title(2) + 7 single-line
		// fields with one-row gaps(14) + desc label(1) + bottom gap+help(2) +
		// padding(2) + border(2) ≈ 23. Leave a little slack.
		descHeight := max(8, height-overlayChromeRows)
		m.description.SetHeight(descHeight)
		m.descWidth = contentWidth
		m.descHeight = descHeight
		m.descViewport.Width = contentWidth
		m.descViewport.Height = descHeight
		m.updateDescPreview()
	}
}

func (m *Model) blurFields() {
	m.summary.Blur()
	m.storyPoints.Blur()
	m.labels.Blur()
	m.fixVersions.Blur()
	m.parent.Blur()
	m.description.Blur()
}

func (m *Model) enterEditMode() {
	if !isTextField(m.activeField) {
		return
	}
	m.editing = true
	m.blurFields()
	// Place the cursor at the end of the existing value so editing appends
	// rather than starting in front of the text.
	switch m.activeField {
	case fieldSummary:
		m.summary.Focus()
		m.summary.CursorEnd()
	case fieldStoryPoints:
		m.storyPoints.Focus()
		m.storyPoints.CursorEnd()
	case fieldLabels:
		m.labels.Focus()
		m.labels.CursorEnd()
	case fieldFixVersions:
		m.fixVersions.Focus()
		m.fixVersions.CursorEnd()
	case fieldParent:
		m.parent.Focus()
		m.parent.CursorEnd()
	case fieldDescription:
		m.description.Focus()
		// Walk to the last line, then to the end of it.
		for {
			before := m.description.Line()
			m.description.CursorDown()
			if m.description.Line() == before {
				break
			}
		}
		m.description.CursorEnd()
	}
}

func (m *Model) leaveEditMode() {
	wasEditingDesc := m.editing && m.activeField == fieldDescription
	m.editing = false
	m.blurFields()
	if wasEditingDesc {
		m.updateDescPreview()
	}
}

func isTextField(field int) bool {
	switch field {
	case fieldSummary, fieldStoryPoints, fieldLabels, fieldFixVersions, fieldParent, fieldDescription:
		return true
	default:
		return false
	}
}

func (m *Model) moveIssueType(delta int) {
	if len(m.issueTypes) == 0 {
		return
	}
	next := m.issueTypeCursor + delta
	if next < 0 || next >= len(m.issueTypes) {
		return
	}
	m.issueTypeCursor = next
}

func (m *Model) movePriority(delta int) {
	if len(m.priorities) == 0 {
		return
	}
	next := m.priorityCursor + delta
	if next < 0 || next >= len(m.priorities) {
		return
	}
	m.priorityCursor = next
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case editorReturnedMsg:
		if msg.ok {
			m.description.SetValue(msg.content)
			for m.description.Line() > 0 {
				m.description.CursorUp()
			}
			m.description.CursorStart()
			m.updateDescPreview()
		}
		return m, nil
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
			if m.editing {
				m.leaveEditMode()
				return m, nil
			}
			m.dismissed = true
			return m, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+a"))):
			m.clearActiveField()
			return m, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+s"))):
			req, err := m.buildRequest()
			if err != nil {
				m.validationErr = err.Error()
				return m, nil
			}
			m.validationErr = ""
			m.submitted = req
			return m, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+g"))):
			m.leaveEditMode()
			return m, openInEditor(m.description.Value())
		case !m.editing && key.Matches(msg, key.NewBinding(key.WithKeys("j", "down"))):
			m.activeField = (m.activeField + 1) % numFields
			return m, nil
		case !m.editing && key.Matches(msg, key.NewBinding(key.WithKeys("k", "up"))):
			m.activeField = (m.activeField + numFields - 1) % numFields
			return m, nil
		case !m.editing && key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
			if isTextField(m.activeField) {
				m.enterEditMode()
			}
			return m, nil
		case !m.editing && m.activeField == fieldDescription &&
			key.Matches(msg, key.NewBinding(key.WithKeys("d", "ctrl+d", "pgdown"))):
			m.descViewport.HalfPageDown()
			return m, nil
		case !m.editing && m.activeField == fieldDescription &&
			key.Matches(msg, key.NewBinding(key.WithKeys("u", "ctrl+u", "pgup"))):
			m.descViewport.HalfPageUp()
			return m, nil
		case !m.editing && m.activeField == fieldDescription &&
			key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+e"))):
			m.descViewport.ScrollDown(1)
			return m, nil
		case !m.editing && m.activeField == fieldDescription &&
			key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+y"))):
			m.descViewport.ScrollUp(1)
			return m, nil
		case !m.editing && m.activeField == fieldDescription &&
			key.Matches(msg, key.NewBinding(key.WithKeys("g", "home"))):
			m.descViewport.GotoTop()
			return m, nil
		case !m.editing && m.activeField == fieldDescription &&
			key.Matches(msg, key.NewBinding(key.WithKeys("G", "end"))):
			m.descViewport.GotoBottom()
			return m, nil
		}

		// IssueType and Priority fields use h/l or arrows to cycle.
		if !m.editing && m.activeField == fieldIssueType {
			switch {
			case key.Matches(msg, key.NewBinding(key.WithKeys("l", "right"))):
				m.moveIssueType(1)
				return m, nil
			case key.Matches(msg, key.NewBinding(key.WithKeys("h", "left"))):
				m.moveIssueType(-1)
				return m, nil
			}
		}

		if !m.editing && m.activeField == fieldPriority {
			switch {
			case key.Matches(msg, key.NewBinding(key.WithKeys("l", "right"))):
				m.movePriority(1)
				return m, nil
			case key.Matches(msg, key.NewBinding(key.WithKeys("h", "left"))):
				m.movePriority(-1)
				return m, nil
			}
		}
	}

	if !m.editing {
		return m, nil
	}

	// Update the active text input.
	var cmd tea.Cmd
	switch m.activeField {
	case fieldSummary:
		m.summary, cmd = m.summary.Update(msg)
	case fieldStoryPoints:
		m.storyPoints, cmd = m.storyPoints.Update(msg)
	case fieldLabels:
		m.labels, cmd = m.labels.Update(msg)
	case fieldFixVersions:
		m.fixVersions, cmd = m.fixVersions.Update(msg)
	case fieldParent:
		m.parent, cmd = m.parent.Update(msg)
	case fieldDescription:
		m.description, cmd = m.description.Update(msg)
	}

	return m, cmd
}

func (m *Model) clearActiveField() {
	switch m.activeField {
	case fieldSummary:
		m.summary.SetValue("")
	case fieldStoryPoints:
		m.storyPoints.SetValue("")
	case fieldLabels:
		m.labels.SetValue("")
	case fieldFixVersions:
		m.fixVersions.SetValue("")
	case fieldParent:
		m.parent.SetValue("")
	case fieldDescription:
		m.description.SetValue("")
	}
}

// buildRequest computes the diff between original and edited values.
// Returns an error if a field fails validation (e.g. a malformed parent key).
func (m Model) buildRequest() (*client.EditIssueRequest, error) {
	req := &client.EditIssueRequest{}

	// Summary: only send if changed.
	if newSummary := m.summary.Value(); newSummary != m.origSummary {
		req.Summary = newSummary
	}

	// IssueType: only send if changed.
	if len(m.issueTypes) > 0 {
		newType := m.issueTypes[m.issueTypeCursor]
		if newType != m.origIssueType {
			req.IssueType = newType
		}
	}

	// Priority: only send if changed.
	if len(m.priorities) > 0 {
		newPriority := m.priorities[m.priorityCursor]
		if newPriority != m.origPriority {
			req.Priority = newPriority
		}
	}

	// Story Points: only send if changed.
	newSPStr := strings.TrimSpace(m.storyPoints.Value())
	if newSPStr == "" {
		if m.origStoryPoints != nil {
			var val *float64 = nil
			req.StoryPoints = &val
		}
	} else {
		if val, err := strconv.ParseFloat(newSPStr, 64); err == nil {
			if m.origStoryPoints == nil || *m.origStoryPoints != val {
				ptr := &val
				req.StoryPoints = &ptr
			}
		}
	}

	// Labels: compute diff.
	newLabelsRaw := m.labels.Value()
	if newLabelsRaw != strings.Join(m.origLabels, ", ") {
		req.Labels = computeLabelsDiff(m.origLabels, parseLabels(newLabelsRaw))
	}

	// Fix Versions: compute diff.
	newFixVersionsRaw := m.fixVersions.Value()
	if newFixVersionsRaw != strings.Join(m.origFixVersions, ", ") {
		req.FixVersions = computeLabelsDiff(m.origFixVersions, parseLabels(newFixVersionsRaw))
	}

	// Description: only send if changed.
	if newDesc := m.description.Value(); newDesc != m.origDescription {
		req.Description = newDesc
	}

	// Parent: set to a new key or remove. Jira keys are uppercase, so normalise
	// before diffing to avoid spurious changes from case differences. An empty
	// value clears the parent; a non-empty value must be a valid issue key.
	newParent := strings.ToUpper(strings.TrimSpace(m.parent.Value()))
	if newParent != m.origParent {
		if newParent == "" {
			empty := ""
			req.Parent = &empty
		} else {
			if err := validate.IssueKey(newParent); err != nil {
				return nil, err
			}
			p := newParent
			req.Parent = &p
		}
	}

	return req, nil
}

// parseLabels splits a comma-separated label string into a trimmed slice.
func parseLabels(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// computeLabelsDiff returns the label operations needed to go from old to new.
// New labels are added as-is; removed labels are prefixed with "-".
func computeLabelsDiff(old, new []string) []string {
	oldSet := make(map[string]bool, len(old))
	for _, l := range old {
		oldSet[l] = true
	}
	newSet := make(map[string]bool, len(new))
	for _, l := range new {
		newSet[l] = true
	}

	var ops []string
	// Removals.
	for _, l := range old {
		if !newSet[l] {
			ops = append(ops, "-"+l)
		}
	}
	// Additions.
	for _, l := range new {
		if !oldSet[l] {
			ops = append(ops, l)
		}
	}
	return ops
}

// currentPriority returns the currently selected priority name.
func (m Model) currentPriority() string {
	if len(m.priorities) == 0 {
		return ""
	}
	return m.priorities[m.priorityCursor]
}

// currentIssueType returns the currently selected issue type name.
func (m Model) currentIssueType() string {
	if len(m.issueTypes) == 0 {
		return ""
	}
	return m.issueTypes[m.issueTypeCursor]
}

// summaryPreview renders the summary as wrapped text — Jira summaries can run
// well past the textinput's visible window, and a textinput scrolls rather
// than wraps, so the only way to actually see the full value without entering
// edit mode is to wrap it ourselves. The leading "> " keeps visual alignment
// with the textinput's prompt while editing.
func (m Model) summaryPreview() string {
	width := m.inputWidth
	if width <= 0 {
		width = 60
	}
	val := m.summary.Value()
	if val == "" {
		return "> " + lipgloss.NewStyle().Foreground(theme.ColourSubtle).Render("(no summary)")
	}
	wrapped := lipgloss.NewStyle().Width(width).Render(val)
	return "> " + wrapped
}

// updateDescPreview re-renders the description into the scrollable viewport.
// Called whenever the description value or render width changes (SetIssue,
// SetSize, leaveEditMode while on the description field, and after the
// external editor returns) so the viewport always has fresh content to scroll
// through. Scroll position is reset to the top on each refresh — feedback
// after editing usually wants to start from the beginning anyway.
func (m *Model) updateDescPreview() {
	width := m.descWidth
	if width <= 0 {
		width = 60
	}
	raw := m.description.Value()
	var rendered string
	if strings.TrimSpace(raw) == "" {
		rendered = lipgloss.NewStyle().Foreground(theme.ColourSubtle).
			Render("(no description — press enter to edit)")
	} else {
		rendered = markup.Render(raw, width)
	}
	m.descViewport.SetContent(rendered)
	m.descViewport.GotoTop()
}

// View renders the field editor overlay.
func (m Model) View() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.ColourPrimary).
		MarginBottom(1)

	title := titleStyle.Render(fmt.Sprintf("Edit %s", m.issueKey))

	labelStyle := lipgloss.NewStyle().Bold(true).Width(labelWidth)
	activeLabel := labelStyle.Foreground(theme.ColourPrimary)
	inactiveLabel := labelStyle.Foreground(theme.ColourSubtle)

	// Summary field. Use the raw textinput while editing; otherwise show a
	// wrapped preview so long summaries are fully visible instead of being
	// clipped at the textinput's scroll window.
	summaryLabel := inactiveLabel
	if m.activeField == fieldSummary {
		summaryLabel = activeLabel
	}
	var summaryBody string
	if m.activeField == fieldSummary && m.editing {
		summaryBody = m.summary.View()
	} else {
		summaryBody = m.summaryPreview()
	}
	summaryLine := lipgloss.JoinHorizontal(lipgloss.Top,
		summaryLabel.Render("Summary"),
		summaryBody,
	)

	// IssueType field.
	issueTypeLabel := inactiveLabel
	if m.activeField == fieldIssueType {
		issueTypeLabel = activeLabel
	}
	issueTypeValue := m.currentIssueType()
	issueTypeStyle := lipgloss.NewStyle()
	if m.activeField == fieldIssueType {
		issueTypeStyle = issueTypeStyle.Bold(true)
		issueTypeValue = "◀ " + issueTypeValue + " ▶"
	}
	issueTypeLine := lipgloss.JoinHorizontal(lipgloss.Top,
		issueTypeLabel.Render("Issue Type"),
		issueTypeStyle.Render(issueTypeValue),
	)

	// Parent field. A free-text issue key; clearing it removes the parent.
	parentLabel := inactiveLabel
	if m.activeField == fieldParent {
		parentLabel = activeLabel
	}
	parentLine := lipgloss.JoinHorizontal(lipgloss.Top,
		parentLabel.Render("Parent"),
		m.parent.View(),
	)

	// Priority field.
	priorityLabel := inactiveLabel
	if m.activeField == fieldPriority {
		priorityLabel = activeLabel
	}
	priorityValue := m.currentPriority()
	priorityStyle := lipgloss.NewStyle()
	if m.activeField == fieldPriority {
		priorityStyle = priorityStyle.Bold(true)
		priorityValue = "◀ " + priorityValue + " ▶"
	}
	priorityLine := lipgloss.JoinHorizontal(lipgloss.Top,
		priorityLabel.Render("Priority"),
		priorityStyle.Render(priorityValue),
	)

	// Story Points field.
	spLabel := inactiveLabel
	if m.activeField == fieldStoryPoints {
		spLabel = activeLabel
	}
	spLine := lipgloss.JoinHorizontal(lipgloss.Top,
		spLabel.Render("Story Points"),
		m.storyPoints.View(),
	)

	// Labels field.
	labelsLabel := inactiveLabel
	if m.activeField == fieldLabels {
		labelsLabel = activeLabel
	}
	labelsLine := lipgloss.JoinHorizontal(lipgloss.Top,
		labelsLabel.Render("Labels"),
		m.labels.View(),
	)

	// Fix Versions field.
	fvLabel := inactiveLabel
	if m.activeField == fieldFixVersions {
		fvLabel = activeLabel
	}
	fvLine := lipgloss.JoinHorizontal(lipgloss.Top,
		fvLabel.Render("Fix Versions"),
		m.fixVersions.View(),
	)

	// Description field. Show rendered wiki markup by default; switch to the
	// raw textarea only while the description field is being edited.
	descLabel := inactiveLabel
	if m.activeField == fieldDescription {
		descLabel = activeLabel
	}
	var descBody string
	if m.activeField == fieldDescription && m.editing {
		descBody = m.description.View()
	} else {
		descBody = m.descViewport.View()
	}
	descLine := lipgloss.JoinVertical(lipgloss.Left,
		descLabel.Render("Desc"),
		descBody,
	)

	help := theme.StyleHelpKey.Render("j/k") + " " + theme.StyleHelpDesc.Render("navigate fields") + "  " +
		theme.StyleHelpKey.Render("h/l") + " " + theme.StyleHelpDesc.Render("change option") + "  " +
		theme.StyleHelpKey.Render("enter") + " " + theme.StyleHelpDesc.Render("edit text") + "  " +
		theme.StyleHelpKey.Render("ctrl+g") + " " + theme.StyleHelpDesc.Render("edit desc in $EDITOR") + "  " +
		theme.StyleHelpKey.Render("ctrl+s") + " " + theme.StyleHelpDesc.Render("save") + "  " +
		theme.StyleHelpKey.Render("esc") + " " + theme.StyleHelpDesc.Render("cancel")
	if m.activeField == fieldDescription && !m.editing {
		help += "  " + theme.StyleHelpKey.Render("d/u") + " " + theme.StyleHelpDesc.Render("scroll desc") + "  " +
			theme.StyleHelpKey.Render("g/G") + " " + theme.StyleHelpDesc.Render("top/bottom")
	}

	if m.validationErr != "" {
		help = lipgloss.NewStyle().Foreground(theme.ColourError).Render(m.validationErr) + "\n" + help
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		summaryLine,
		"",
		issueTypeLine,
		"",
		parentLine,
		"",
		priorityLine,
		"",
		spLine,
		"",
		labelsLine,
		"",
		fvLine,
		"",
		descLine,
		"",
		help,
	)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColourPrimary).
		Padding(1, 3).
		Width(min(m.width-4, boxMaxWidth))

	box := boxStyle.Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
