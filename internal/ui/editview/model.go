package editview

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/seanhalberthal/jiru/internal/client"
	"github.com/seanhalberthal/jiru/internal/jira"
	"github.com/seanhalberthal/jiru/internal/theme"
)

const (
	fieldSummary     = 0
	fieldPriority    = 1
	fieldLabels      = 2
	fieldDescription = 3
	numFields        = 4
)

// Model is the field editor overlay.
type Model struct {
	issueKey        string
	summary         textinput.Model
	labels          textinput.Model
	priorities      []string
	priorityCursor  int
	activeField     int
	submitted       *client.EditIssueRequest
	dismissed       bool
	width           int
	height          int
	description     textarea.Model
	origDescription string
	// Original values for diff computation.
	origSummary  string
	origPriority string
	origLabels   []string
}

// New creates a new field editor for the given issue key.
func New(issueKey string) Model {
	summary := textinput.New()
	summary.Placeholder = "Summary"
	summary.CharLimit = 255
	summary.Width = 80
	summary.Focus()

	labels := textinput.New()
	labels.Placeholder = "Labels (comma-separated)"
	labels.CharLimit = 500
	labels.Width = 80

	desc := textarea.New()
	desc.Placeholder = "Description (wiki markup)"
	desc.CharLimit = 0
	desc.SetHeight(8)
	desc.SetWidth(80)

	return Model{
		issueKey:    issueKey,
		summary:     summary,
		labels:      labels,
		description: desc,
	}
}

// SetIssue pre-populates the editor from the current issue.
func (m *Model) SetIssue(iss jira.Issue, priorities []string) {
	m.summary.SetValue(iss.Summary)
	m.origSummary = iss.Summary
	m.origPriority = iss.Priority
	m.origLabels = iss.Labels
	m.labels.SetValue(strings.Join(iss.Labels, ", "))
	m.priorities = priorities

	m.description.SetValue(iss.Description)
	// Move cursor to the very beginning (row 0, col 0) so it's visible.
	for m.description.Line() > 0 {
		m.description.CursorUp()
	}
	m.description.CursorStart()
	m.origDescription = iss.Description

	// Pre-select current priority.
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
	// Use most of the viewport width: up to 80% of terminal width, capped at 120.
	inputWidth := min(120, width*4/5)
	if inputWidth > 0 {
		m.summary.Width = inputWidth
		m.labels.Width = inputWidth
		if m.issueKey != "" {
			m.description.SetWidth(inputWidth)
			// Scale description height based on available space.
			descHeight := max(6, (height-20)/2)
			m.description.SetHeight(descHeight)
		}
	}
}

func (m *Model) focusField() {
	m.summary.Blur()
	m.labels.Blur()
	m.description.Blur()
	switch m.activeField {
	case fieldSummary:
		m.summary.Focus()
	case fieldLabels:
		m.labels.Focus()
	case fieldDescription:
		m.description.Focus()
	}
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
			m.dismissed = true
			return m, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+s"))):
			m.submitted = m.buildRequest()
			return m, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("tab"))):
			m.activeField = (m.activeField + 1) % numFields
			m.focusField()
			return m, nil
		case key.Matches(msg, key.NewBinding(key.WithKeys("shift+tab"))):
			m.activeField = (m.activeField + numFields - 1) % numFields
			m.focusField()
			return m, nil
		}

		// Priority field: use j/k or arrows to cycle.
		if m.activeField == fieldPriority {
			switch {
			case key.Matches(msg, key.NewBinding(key.WithKeys("j", "down", "right"))):
				if m.priorityCursor < len(m.priorities)-1 {
					m.priorityCursor++
				}
				return m, nil
			case key.Matches(msg, key.NewBinding(key.WithKeys("k", "up", "left"))):
				if m.priorityCursor > 0 {
					m.priorityCursor--
				}
				return m, nil
			}
		}
	}

	// Update the active text input.
	var cmd tea.Cmd
	switch m.activeField {
	case fieldSummary:
		m.summary, cmd = m.summary.Update(msg)
	case fieldLabels:
		m.labels, cmd = m.labels.Update(msg)
	case fieldDescription:
		m.description, cmd = m.description.Update(msg)
	}

	return m, cmd
}

// buildRequest computes the diff between original and edited values.
func (m Model) buildRequest() *client.EditIssueRequest {
	req := &client.EditIssueRequest{}

	// Summary: only send if changed.
	if newSummary := m.summary.Value(); newSummary != m.origSummary {
		req.Summary = newSummary
	}

	// Priority: only send if changed.
	if len(m.priorities) > 0 {
		newPriority := m.priorities[m.priorityCursor]
		if newPriority != m.origPriority {
			req.Priority = newPriority
		}
	}

	// Labels: compute diff.
	newLabelsRaw := m.labels.Value()
	if newLabelsRaw != strings.Join(m.origLabels, ", ") {
		req.Labels = computeLabelsDiff(m.origLabels, parseLabels(newLabelsRaw))
	}

	// Description: only send if changed.
	if newDesc := m.description.Value(); newDesc != m.origDescription {
		req.Description = newDesc
	}

	return req
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

// View renders the field editor overlay.
func (m Model) View() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.ColourPrimary).
		MarginBottom(1)

	title := titleStyle.Render(fmt.Sprintf("Edit %s", m.issueKey))

	labelStyle := lipgloss.NewStyle().Bold(true).Width(10)
	activeLabel := labelStyle.Foreground(theme.ColourPrimary)
	inactiveLabel := labelStyle.Foreground(theme.ColourSubtle)

	// Summary field.
	summaryLabel := inactiveLabel
	if m.activeField == fieldSummary {
		summaryLabel = activeLabel
	}
	summaryLine := lipgloss.JoinHorizontal(lipgloss.Top,
		summaryLabel.Render("Summary"),
		m.summary.View(),
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

	// Labels field.
	labelsLabel := inactiveLabel
	if m.activeField == fieldLabels {
		labelsLabel = activeLabel
	}
	labelsLine := lipgloss.JoinHorizontal(lipgloss.Top,
		labelsLabel.Render("Labels"),
		m.labels.View(),
	)

	// Description field.
	descLabel := inactiveLabel
	if m.activeField == fieldDescription {
		descLabel = activeLabel
	}
	descLine := lipgloss.JoinVertical(lipgloss.Left,
		descLabel.Render("Desc"),
		m.description.View(),
	)

	help := theme.StyleHelpKey.Render("tab") + " " + theme.StyleHelpDesc.Render("next field") + "  " +
		theme.StyleHelpKey.Render("ctrl+s") + " " + theme.StyleHelpDesc.Render("save") + "  " +
		theme.StyleHelpKey.Render("esc") + " " + theme.StyleHelpDesc.Render("cancel")

	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		summaryLine,
		"",
		priorityLine,
		"",
		labelsLine,
		"",
		descLine,
		"",
		help,
	)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColourPrimary).
		Padding(1, 3).
		Width(min(m.width-4, 130))

	box := boxStyle.Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
