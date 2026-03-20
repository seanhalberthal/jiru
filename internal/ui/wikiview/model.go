package wikiview

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/seanhalberthal/jiru/internal/adf"
	"github.com/seanhalberthal/jiru/internal/confluence"
	"github.com/seanhalberthal/jiru/internal/jira"
	"github.com/seanhalberthal/jiru/internal/theme"
)

// Model is the Confluence page detail view.
type Model struct {
	viewport   viewport.Model
	page       *confluence.Page
	ancestors  []confluence.PageAncestor
	links      []jira.RemoteLink
	spaceKey   string
	width      int
	height     int
	openURL    string // sentinel: open in browser
	selIssue   string // sentinel: navigate to linked Jira issue
	selAnc     string // sentinel: navigate to ancestor page
	dismissed  bool
	openKeys   key.Binding
	urlKeys    key.Binding
	backKeys   key.Binding
	topKeys    key.Binding
	bottomKeys key.Binding
}

func New() Model {
	vp := viewport.New(0, 0)
	vp.KeyMap.HalfPageDown.SetKeys("d", "ctrl+d")
	vp.KeyMap.HalfPageUp.SetKeys("u", "ctrl+u")

	return Model{
		viewport:   vp,
		openKeys:   key.NewBinding(key.WithKeys("enter")),
		urlKeys:    key.NewBinding(key.WithKeys("o")),
		backKeys:   key.NewBinding(key.WithKeys("esc")),
		topKeys:    key.NewBinding(key.WithKeys("g")),
		bottomKeys: key.NewBinding(key.WithKeys("G")),
	}
}

func (m Model) SetSize(width, height int) Model {
	m.width = width
	m.height = height
	m.viewport.Width = width
	m.viewport.Height = height - 4 // Reserve for fixed header (title + meta + border + padding).
	if m.page != nil {
		m.renderContent()
	}
	return m
}

// SetPage sets the page to display and renders it.
func (m *Model) SetPage(page *confluence.Page) {
	m.page = page
	m.renderContent()
}

// SetAncestors sets the breadcrumb ancestor chain.
func (m *Model) SetAncestors(ancestors []confluence.PageAncestor) {
	m.ancestors = ancestors
	if m.page != nil {
		m.renderContent()
	}
}

// SetSpaceKey sets the space key for breadcrumb display.
func (m *Model) SetSpaceKey(spaceKey string) {
	m.spaceKey = spaceKey
}

// SetLinks sets the linked Jira issues.
func (m *Model) SetLinks(links []jira.RemoteLink) {
	m.links = links
	if m.page != nil {
		m.renderContent()
	}
}

// OpenURL returns the URL to open in browser (sentinel, resets after read).
func (m *Model) OpenURL() (string, bool) {
	u := m.openURL
	m.openURL = ""
	return u, u != ""
}

// SelectedIssue returns the Jira issue key to navigate to (sentinel).
func (m *Model) SelectedIssue() (string, bool) {
	k := m.selIssue
	m.selIssue = ""
	return k, k != ""
}

// SelectedAncestor returns the ancestor page ID to navigate to (sentinel).
func (m *Model) SelectedAncestor() (string, bool) {
	a := m.selAnc
	m.selAnc = ""
	return a, a != ""
}

// Dismissed returns true if the user pressed back.
func (m *Model) Dismissed() bool {
	d := m.dismissed
	m.dismissed = false
	return d
}

// CurrentPage returns the currently displayed page.
func (m *Model) CurrentPage() *confluence.Page {
	return m.page
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if key.Matches(keyMsg, m.backKeys) {
			m.dismissed = true
			return m, nil
		}
		if key.Matches(keyMsg, m.urlKeys) && m.page != nil {
			m.openURL = fmt.Sprintf("page/%s", m.page.ID) // parent constructs full URL
			return m, nil
		}
		if key.Matches(keyMsg, m.topKeys) {
			m.viewport.GotoTop()
			return m, nil
		}
		if key.Matches(keyMsg, m.bottomKeys) {
			m.viewport.GotoBottom()
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// renderHeader returns a fixed header matching the issueview style.
func (m Model) renderHeader() string {
	if m.page == nil {
		return ""
	}

	// Line 1: breadcrumb + title + scroll %.
	title := theme.StyleKey.Render(m.page.Title)
	bc := m.renderBreadcrumb()
	if bc != "" {
		title = bc + "  " + title
	}

	scrollPct := fmt.Sprintf("%.0f%%", m.viewport.ScrollPercent()*100)
	info := theme.StyleSubtle.Render(scrollPct)
	gap := strings.Repeat(" ", max(0, m.width-lipgloss.Width(title)-lipgloss.Width(info)))
	line1 := title + gap + info

	// Line 2: author · version · updated.
	var meta []string
	if m.page.Author != "" {
		meta = append(meta, theme.UserStyle(m.page.Author).Render(m.page.Author))
	}
	if m.page.Version > 0 {
		meta = append(meta, theme.StyleSubtle.Render(fmt.Sprintf("v%d", m.page.Version)))
	}
	if !m.page.Updated.IsZero() {
		meta = append(meta, theme.StyleSubtle.Render(m.page.Updated.Format("2 Jan 2006 15:04")))
	}
	line2 := strings.Join(meta, theme.StyleSubtle.Render(" · "))

	headerStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(theme.ColourSubtle)

	return headerStyle.Render(line1 + "\n" + line2)
}

func (m *Model) renderContent() {
	if m.page == nil {
		return
	}

	var b strings.Builder

	// Body — render ADF
	if m.page.BodyADF != "" {
		rendered := adf.Render(m.page.BodyADF, m.width-2)
		b.WriteString(rendered)
	} else {
		b.WriteString(theme.StyleSubtle.Render("(no content)"))
	}

	// Linked Jira issues
	if len(m.links) > 0 {
		b.WriteString("\n\n")
		b.WriteString(theme.StyleSubtle.Render("── Linked Jira Issues ──"))
		b.WriteString("\n")
		for _, link := range m.links {
			fmt.Fprintf(&b, "  %s  %s\n",
				theme.StyleKey.Render(link.Title),
				theme.StyleSubtle.Render(link.URL),
			)
		}
	}

	m.viewport.SetContent(b.String())
}

func (m *Model) renderBreadcrumb() string {
	// Filter ancestors with empty titles.
	var validAncestors []confluence.PageAncestor
	for _, a := range m.ancestors {
		if strings.TrimSpace(a.Title) != "" {
			validAncestors = append(validAncestors, a)
		}
	}

	if len(validAncestors) == 0 && m.spaceKey == "" {
		return ""
	}

	sep := lipgloss.NewStyle().Foreground(theme.ColourSubtle).Render(" › ")
	var parts []string

	if m.spaceKey != "" && !strings.HasPrefix(m.spaceKey, "~") {
		parts = append(parts, theme.StyleSubtle.Render(m.spaceKey))
	}

	for _, a := range validAncestors {
		// Skip the ancestor if it has the same title as the current page.
		if m.page != nil && a.Title == m.page.Title {
			continue
		}
		parts = append(parts, theme.StyleSubtle.Render(a.Title))
	}

	if len(parts) == 0 {
		return ""
	}

	bc := strings.Join(parts, sep)

	// Truncate from the left if too wide (keep last 2 parts).
	if m.width > 0 && lipgloss.Width(bc) > m.width/2 {
		ellipsis := theme.StyleSubtle.Render("…")
		if len(parts) > 2 {
			parts = parts[len(parts)-2:]
			bc = ellipsis + sep + strings.Join(parts, sep)
		}
	}

	return bc
}

func (m Model) View() string {
	header := m.renderHeader()
	return lipgloss.JoinVertical(lipgloss.Left, header, m.viewport.View())
}
