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
	viewport       viewport.Model
	page           *confluence.Page
	ancestors      []confluence.PageAncestor
	links          []jira.RemoteLink
	comments       []confluence.Comment
	commentLine    int   // viewport line where the footer comments section starts (-1 = none).
	inlineLines    []int // viewport lines where inline comments are rendered.
	inlineIdx      int   // current position in inline comment cycle (-1 = none).
	spaceKey       string
	width          int
	height         int
	openURL        string // sentinel: open in browser
	copyURL        string // sentinel: copy URL to clipboard
	selIssue       string // sentinel: navigate to linked Jira issue
	selAnc         string // sentinel: navigate to ancestor page
	dismissed      bool
	openKeys       key.Binding
	urlKeys        key.Binding
	copyKeys       key.Binding
	backKeys       key.Binding
	topKeys        key.Binding
	bottomKeys     key.Binding
	commentKeys    key.Binding
	nextInlineKeys key.Binding
	prevInlineKeys key.Binding
}

func New() Model {
	vp := viewport.New(0, 0)
	vp.KeyMap.HalfPageDown.SetKeys("d", "ctrl+d")
	vp.KeyMap.HalfPageUp.SetKeys("u", "ctrl+u")

	return Model{
		viewport:       vp,
		commentLine:    -1,
		inlineIdx:      -1,
		openKeys:       key.NewBinding(key.WithKeys("enter")),
		urlKeys:        key.NewBinding(key.WithKeys("o")),
		copyKeys:       key.NewBinding(key.WithKeys("x")),
		backKeys:       key.NewBinding(key.WithKeys("esc")),
		topKeys:        key.NewBinding(key.WithKeys("g")),
		bottomKeys:     key.NewBinding(key.WithKeys("G")),
		commentKeys:    key.NewBinding(key.WithKeys("c")),
		nextInlineKeys: key.NewBinding(key.WithKeys("]")),
		prevInlineKeys: key.NewBinding(key.WithKeys("[")),
	}
}

func (m Model) SetSize(width, height int) Model {
	m.width = width
	m.height = height
	m.viewport.Width = width
	m.viewport.Height = height - m.headerHeight()
	if m.page != nil {
		m.renderContent()
	}
	return m
}

// headerHeight returns the actual rendered header height, or a default
// estimate when no page is loaded yet.
func (m Model) headerHeight() int {
	if m.page == nil {
		return 4 // Estimate: title + meta + border + padding.
	}
	header := m.renderHeader()
	h := max(lipgloss.Height(header), 2)
	return h
}

// SetPage sets the page to display and renders it.
// Comments are cleared — they arrive asynchronously via SetComments.
func (m *Model) SetPage(page *confluence.Page) {
	m.page = page
	m.comments = nil
	m.commentLine = -1
	m.inlineLines = nil
	m.inlineIdx = -1
	m.recalcViewport()
	m.renderContent()
}

// SetAncestors sets the breadcrumb ancestor chain.
func (m *Model) SetAncestors(ancestors []confluence.PageAncestor) {
	m.ancestors = ancestors
	m.recalcViewport()
	if m.page != nil {
		m.renderContent()
	}
}

// recalcViewport adjusts the viewport height based on the current header.
func (m *Model) recalcViewport() {
	if m.height == 0 {
		return
	}
	hh := m.headerHeight()
	vpH := max(m.height-hh, 1)
	m.viewport.Height = vpH
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

// SetComments sets the page comments and re-renders.
func (m *Model) SetComments(comments []confluence.Comment) {
	m.comments = comments
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

// CopyURL returns the page ID to copy as a URL (sentinel, resets after read).
func (m *Model) CopyURL() (string, bool) {
	u := m.copyURL
	m.copyURL = ""
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
		if key.Matches(keyMsg, m.copyKeys) && m.page != nil {
			m.copyURL = m.page.ID
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
		if key.Matches(keyMsg, m.commentKeys) && m.commentLine >= 0 {
			m.viewport.SetYOffset(m.commentLine)
			return m, nil
		}
		if key.Matches(keyMsg, m.nextInlineKeys) && len(m.inlineLines) > 0 {
			m.inlineIdx++
			if m.inlineIdx >= len(m.inlineLines) {
				m.inlineIdx = 0
			}
			m.viewport.SetYOffset(m.inlineLines[m.inlineIdx])
			return m, nil
		}
		if key.Matches(keyMsg, m.prevInlineKeys) && len(m.inlineLines) > 0 {
			m.inlineIdx--
			if m.inlineIdx < 0 {
				m.inlineIdx = len(m.inlineLines) - 1
			}
			m.viewport.SetYOffset(m.inlineLines[m.inlineIdx])
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
	if stats := m.commentStats(); stats != "" {
		meta = append(meta, stats)
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
	var lineCount int

	// Body — render ADF (with inline comments placed at their annotation points).
	var placement adf.CommentPlacement
	if m.page.BodyADF != "" {
		commentMap := m.buildInlineCommentMap()
		var rendered string
		rendered, placement = adf.RenderWithComments(m.page.BodyADF, m.width-2, commentMap)
		b.WriteString(rendered)
		lineCount = strings.Count(rendered, "\n")
	} else {
		placement = adf.CommentPlacement{Placed: make(map[string]bool)}
		b.WriteString(theme.StyleSubtle.Render("(no content)"))
	}
	m.inlineLines = placement.Lines
	m.inlineIdx = -1

	// Linked Jira issues
	if len(m.links) > 0 {
		b.WriteString("\n\n")
		b.WriteString(theme.StyleSubtle.Render("── Linked Jira Issues ──"))
		b.WriteString("\n")
		lineCount += 3
		for _, link := range m.links {
			fmt.Fprintf(&b, "  %s  %s\n",
				theme.StyleKey.Render(link.Title),
				theme.StyleSubtle.Render(link.URL),
			)
			lineCount++
		}
	}

	// Comments — footer, plus any inline comments not placed in the body.
	m.commentLine = -1
	for _, c := range m.comments {
		if !c.Inline {
			// Record the line where footer comments start (for "c" jump).
			m.commentLine = lineCount + 1
			break
		}
	}
	m.renderComments(&b, placement.Placed)

	m.viewport.SetContent(b.String())
}

// renderComments appends the comments section to the content builder.
// placedInline contains IDs of inline comments already rendered within the ADF body.
func (m *Model) renderComments(b *strings.Builder, placedInline map[string]bool) {
	if len(m.comments) == 0 {
		return
	}

	// Split into footer and unplaced inline.
	var footer, inline []confluence.Comment
	for _, c := range m.comments {
		if c.Inline {
			if !placedInline[c.MarkerRef] {
				inline = append(inline, c)
			}
		} else {
			footer = append(footer, c)
		}
	}

	bodyWidth := m.width - 4

	if len(footer) > 0 {
		b.WriteString("\n\n")
		b.WriteString(theme.StyleSubtle.Render(fmt.Sprintf("── Comments (%d) ──", len(footer))))
		b.WriteString("\n")

		for _, c := range footer {
			b.WriteString("\n")
			m.renderComment(b, &c, bodyWidth)
		}
	}

	if len(inline) > 0 {
		b.WriteString("\n\n")
		b.WriteString(theme.StyleSubtle.Render(fmt.Sprintf("── Inline Comments (%d) ──", len(inline))))
		b.WriteString("\n")

		for _, c := range inline {
			b.WriteString("\n")
			// Show the highlighted text the comment is anchored to.
			if c.HighlightedText != "" {
				quote := lipgloss.NewStyle().
					Foreground(theme.ColourSubtle).
					Italic(true).
					Render(fmt.Sprintf("\u201c%s\u201d", truncate(c.HighlightedText, 80)))
				b.WriteString("  ")
				b.WriteString(quote)
				if c.ResolutionStatus != "" {
					b.WriteString("  ")
					b.WriteString(inlineStatusBadge(c.ResolutionStatus))
				}
				b.WriteString("\n")
			} else if c.ResolutionStatus != "" {
				b.WriteString("  ")
				b.WriteString(inlineStatusBadge(c.ResolutionStatus))
				b.WriteString("\n")
			}
			m.renderComment(b, &c, bodyWidth)
		}
	}
}

// renderComment renders a single comment: author + timestamp, then ADF body.
func (m *Model) renderComment(b *strings.Builder, c *confluence.Comment, bodyWidth int) {
	// Author + timestamp line.
	var meta []string
	if c.Author != "" {
		meta = append(meta, theme.UserStyle(c.Author).Bold(true).Render(c.Author))
	}
	if !c.Created.IsZero() {
		meta = append(meta, theme.StyleSubtle.Render(c.Created.Format("2 Jan 2006 15:04")))
	}
	if len(meta) > 0 {
		b.WriteString("  ")
		b.WriteString(strings.Join(meta, theme.StyleSubtle.Render(" · ")))
		b.WriteString("\n")
	}

	// Body — render ADF.
	if c.BodyADF != "" {
		rendered := adf.Render(c.BodyADF, bodyWidth)
		// Indent comment body by 2 spaces.
		for line := range strings.SplitSeq(rendered, "\n") {
			b.WriteString("  ")
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
}

// inlineStatusBadge returns a styled badge for inline comment resolution status.
func inlineStatusBadge(status string) string {
	switch strings.ToLower(status) {
	case "resolved":
		return lipgloss.NewStyle().
			Foreground(theme.ColourSuccess).
			Render("✓ resolved")
	case "open", "reopened":
		return lipgloss.NewStyle().
			Foreground(theme.ColourWarning).
			Render("○ " + strings.ToLower(status))
	default:
		return theme.StyleSubtle.Render(status)
	}
}

// buildInlineCommentMap builds a map of marker ref → ADF render data.
// The ADF annotation marks use markerRef (a UUID) as their ID, not the comment ID.
func (m *Model) buildInlineCommentMap() map[string]adf.InlineComment {
	cm := make(map[string]adf.InlineComment)
	for _, c := range m.comments {
		if c.Inline && c.MarkerRef != "" {
			cm[c.MarkerRef] = adf.InlineComment{
				Author:  c.Author,
				BodyADF: c.BodyADF,
				Status:  c.ResolutionStatus,
			}
		}
	}
	return cm
}

// commentStats returns a styled summary of comment counts for the header,
// or empty string if there are no comments.
func (m Model) commentStats() string {
	var footer, unresolved int
	for _, c := range m.comments {
		if c.Inline {
			if !strings.EqualFold(c.ResolutionStatus, "resolved") {
				unresolved++
			}
		} else {
			footer++
		}
	}
	if footer == 0 && unresolved == 0 {
		return ""
	}

	var parts []string
	if footer > 0 {
		label := fmt.Sprintf("%d comment", footer)
		if footer != 1 {
			label += "s"
		}
		parts = append(parts, lipgloss.NewStyle().Foreground(theme.ColourPrimary).Render(label))
	}
	if unresolved > 0 {
		label := fmt.Sprintf("%d unresolved", unresolved)
		parts = append(parts, lipgloss.NewStyle().Foreground(theme.ColourWarning).Render(label))
	}
	return strings.Join(parts, theme.StyleSubtle.Render(", "))
}

// truncate shortens a string to maxLen runes, adding an ellipsis if truncated.
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-1]) + "…"
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
	out := lipgloss.JoinVertical(lipgloss.Left, header, m.viewport.View())

	// Ensure output never exceeds allocated height.
	if m.height > 0 {
		lines := strings.Split(out, "\n")
		if len(lines) > m.height {
			lines = lines[:m.height]
			out = strings.Join(lines, "\n")
		}
	}
	return out
}
