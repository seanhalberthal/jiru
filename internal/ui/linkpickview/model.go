package linkpickview

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/seanhalberthal/jiru/internal/jira"
	"github.com/seanhalberthal/jiru/internal/theme"
	"github.com/seanhalberthal/jiru/internal/validate"
)

const (
	stepPickType = iota
	stepEnterKey
)

// linkTypeEntry represents a selectable direction for a link type.
type linkTypeEntry struct {
	typeName  string
	label     string // e.g., "blocks →" or "is blocked by ←"
	isOutward bool
}

// LinkRequest holds the result of the link wizard.
type LinkRequest struct {
	InwardKey  string
	OutwardKey string
	LinkType   string
}

// Model is the two-step link wizard overlay.
type Model struct {
	issueKey  string
	entries   []linkTypeEntry
	cursor    int
	step      int
	input     textinput.Model
	loading   bool
	submitted *LinkRequest
	dismissed bool
	errMsg    string
	width     int
	height    int
}

// New creates a new link wizard for the given issue.
func New(issueKey string) Model {
	ti := textinput.New()
	ti.Placeholder = "e.g. PROJ-123"
	ti.CharLimit = 30
	ti.Width = 30

	return Model{
		issueKey: issueKey,
		loading:  true,
		input:    ti,
	}
}

// SetLinkTypes populates the picker with available link types.
func (m *Model) SetLinkTypes(types []jira.IssueLinkType) {
	m.entries = make([]linkTypeEntry, 0, len(types)*2)
	for _, t := range types {
		m.entries = append(m.entries, linkTypeEntry{
			typeName:  t.Name,
			label:     t.Outward + " →",
			isOutward: true,
		})
		m.entries = append(m.entries, linkTypeEntry{
			typeName:  t.Name,
			label:     t.Inward + " ←",
			isOutward: false,
		})
	}
	m.loading = false
	m.cursor = 0
}

// SubmittedLink returns the link request (once) and clears the sentinel.
func (m *Model) SubmittedLink() *LinkRequest {
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

// InputActive returns true when text input is active (step 2).
func (m Model) InputActive() bool {
	return m.step == stepEnterKey
}

// SetSize updates the overlay dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if m.loading {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.step {
		case stepPickType:
			switch {
			case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
				m.dismissed = true
			case key.Matches(msg, key.NewBinding(key.WithKeys("k", "up"))):
				if m.cursor > 0 {
					m.cursor--
				}
			case key.Matches(msg, key.NewBinding(key.WithKeys("j", "down"))):
				if m.cursor < len(m.entries)-1 {
					m.cursor++
				}
			case key.Matches(msg, key.NewBinding(key.WithKeys("enter", " "))):
				if len(m.entries) > 0 {
					m.step = stepEnterKey
					m.input.Focus()
					m.errMsg = ""
					return m, textinput.Blink
				}
			}
			return m, nil

		case stepEnterKey:
			switch {
			case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
				m.step = stepPickType
				m.input.SetValue("")
				m.errMsg = ""
				return m, nil
			case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
				targetKey := strings.TrimSpace(m.input.Value())
				if err := validate.IssueKey(targetKey); err != nil {
					m.errMsg = "Invalid issue key"
					return m, nil
				}
				entry := m.entries[m.cursor]
				if entry.isOutward {
					// This issue is the source (outward), target is inward.
					m.submitted = &LinkRequest{
						InwardKey:  targetKey,
						OutwardKey: m.issueKey,
						LinkType:   entry.typeName,
					}
				} else {
					// This issue is the target (inward), other is outward.
					m.submitted = &LinkRequest{
						InwardKey:  m.issueKey,
						OutwardKey: targetKey,
						LinkType:   entry.typeName,
					}
				}
				return m, nil
			}
		}
	}

	if m.step == stepEnterKey {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	return m, nil
}

// View renders the link wizard overlay.
func (m Model) View() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.ColourPrimary).
		MarginBottom(1)

	title := titleStyle.Render(fmt.Sprintf("Link %s", m.issueKey))

	if m.loading {
		content := lipgloss.JoinVertical(lipgloss.Left,
			title,
			theme.StyleSubtle.Render("Loading link types..."),
		)
		return m.centreBox(content)
	}

	if len(m.entries) == 0 {
		content := lipgloss.JoinVertical(lipgloss.Left,
			title,
			theme.StyleSubtle.Render("No link types available."),
			"",
			theme.StyleHelpKey.Render("esc")+" "+theme.StyleHelpDesc.Render("close"),
		)
		return m.centreBox(content)
	}

	switch m.step {
	case stepPickType:
		var items string
		for i, e := range m.entries {
			cursor := "  "
			style := lipgloss.NewStyle()
			if i == m.cursor {
				cursor = theme.StyleKey.Render("> ")
				style = style.Bold(true)
			}
			items += cursor + style.Render(e.label) + "\n"
		}

		help := theme.StyleHelpKey.Render("j/k") + " " + theme.StyleHelpDesc.Render("navigate") + "  " +
			theme.StyleHelpKey.Render("enter/space") + " " + theme.StyleHelpDesc.Render("select") + "  " +
			theme.StyleHelpKey.Render("esc") + " " + theme.StyleHelpDesc.Render("cancel")

		subtitle := theme.StyleSubtle.Render("Select link type:")

		content := lipgloss.JoinVertical(lipgloss.Left,
			title,
			subtitle,
			"",
			items,
			help,
		)
		return m.centreBox(content)

	case stepEnterKey:
		entry := m.entries[m.cursor]
		subtitle := theme.StyleSubtle.Render(fmt.Sprintf("Link type: %s", entry.label))

		var errLine string
		if m.errMsg != "" {
			errLine = "\n" + lipgloss.NewStyle().Foreground(theme.ColourError).Render(m.errMsg)
		}

		help := theme.StyleHelpKey.Render("enter") + " " + theme.StyleHelpDesc.Render("link") + "  " +
			theme.StyleHelpKey.Render("esc") + " " + theme.StyleHelpDesc.Render("back")

		content := lipgloss.JoinVertical(lipgloss.Left,
			title,
			subtitle,
			"",
			"Target issue key:",
			m.input.View(),
			errLine,
			"",
			help,
		)
		return m.centreBox(content)
	}

	return ""
}

func (m Model) centreBox(content string) string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColourPrimary).
		Padding(1, 2)

	box := boxStyle.Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
