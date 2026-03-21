package issueview

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/seanhalberthal/jiru/internal/jira"
	"github.com/seanhalberthal/jiru/internal/markup"
	"github.com/seanhalberthal/jiru/internal/theme"
)

// IssueRef represents a reference to another issue found in the current issue's context.
type IssueRef struct {
	Key     string
	Display string // Shown instead of Key when set (e.g., page title for Confluence refs).
	Label   string // e.g., "parent", "child — Fix login bug", "in description"
	Group   string // e.g., "Parent", "To Do (3)", "In Progress (1)", "Done (2)", "Description", "Comments"
}

var issueKeyExtractRe = regexp.MustCompile(`[A-Z][A-Z0-9]*-[0-9]+`)

// Model is the issue detail view.
type Model struct {
	viewport   viewport.Model
	issue      *jira.Issue
	children   []jira.ChildIssue
	branches   []jira.BranchInfo
	issueURL   string
	width      int
	height     int
	openURL    bool // set when user presses 'o'.
	copyURL    bool // set when user presses 'x'.
	openKeys   key.Binding
	copyKeys   key.Binding
	topKeys    key.Binding
	bottomKeys key.Binding
}

// New creates a new issue view model.
func New() Model {
	vp := viewport.New(0, 0)
	return Model{
		viewport:   vp,
		openKeys:   key.NewBinding(key.WithKeys("o")),
		copyKeys:   key.NewBinding(key.WithKeys("x")),
		topKeys:    key.NewBinding(key.WithKeys("g")),
		bottomKeys: key.NewBinding(key.WithKeys("G")),
	}
}

// SetSize updates the dimensions.
func (m Model) SetSize(width, height int) Model {
	m.width = width
	m.height = height
	headerHeight := 3 // Title bar.
	m.viewport.Width = width
	m.viewport.Height = height - headerHeight
	if m.issue != nil {
		m.viewport.SetContent(m.renderContent())
	}
	return m
}

// SetIssue sets the issue to display and renders content.
func (m Model) SetIssue(iss jira.Issue) Model {
	m.issue = &iss
	m.children = nil // Reset children until they're fetched for the new issue.
	m.branches = nil // Reset branches until they're fetched for the new issue.
	m.viewport.SetContent(m.renderContent())
	m.viewport.GotoTop()
	return m
}

// UpdateIssue updates the issue data without resetting children or branches.
// Used when refreshing issue details (e.g., after IssueDetailMsg) where
// async-fetched children and branches should be preserved.
func (m Model) UpdateIssue(iss jira.Issue) Model {
	m.issue = &iss
	m.viewport.SetContent(m.renderContent())
	return m
}

// SetChildren sets the child issues and re-renders.
func (m Model) SetChildren(children []jira.ChildIssue) Model {
	m.children = children
	if m.issue != nil {
		m.viewport.SetContent(m.renderContent())
	}
	return m
}

// SetBranches sets the git branch info and re-renders.
func (m Model) SetBranches(branches []jira.BranchInfo) Model {
	m.branches = branches
	if m.issue != nil {
		m.viewport.SetContent(m.renderContent())
	}
	return m
}

// SetIssueURL sets the browser URL for the current issue.
func (m *Model) SetIssueURL(url string) {
	m.issueURL = url
}

// CurrentIssue returns the currently displayed issue, or nil.
func (m Model) CurrentIssue() *jira.Issue {
	return m.issue
}

// SetWatching updates the watch state for the current issue.
func (m *Model) SetWatching(watching bool) {
	if m.issue != nil {
		m.issue.IsWatching = watching
		m.renderContent()
	}
}

// OpenURL returns the URL to open (if requested) and resets the flag.
func (m *Model) OpenURL() (string, bool) {
	if !m.openURL || m.issueURL == "" {
		return "", false
	}
	m.openURL = false
	return m.issueURL, true
}

// CopyURL returns the URL to copy (if requested) and resets the flag.
func (m *Model) CopyURL() (string, bool) {
	if !m.copyURL || m.issueURL == "" {
		return "", false
	}
	m.copyURL = false
	return m.issueURL, true
}

// HasParent returns true if the current issue has a parent.
func (m Model) HasParent() bool {
	return m.issue != nil && m.issue.ParentKey != ""
}

// IssueKeys collects all referenced issue keys with context labels.
// The current issue's own key is excluded and duplicates are removed.
func (m Model) IssueKeys() []IssueRef {
	if m.issue == nil {
		return nil
	}

	seen := map[string]bool{m.issue.Key: true}
	var refs []IssueRef

	// Parent.
	if m.issue.ParentKey != "" {
		label := "parent"
		if m.issue.ParentSummary != "" {
			label = "parent — " + m.issue.ParentSummary
			if m.issue.ParentType != "" {
				label += " (" + m.issue.ParentType + ")"
			}
		}
		seen[m.issue.ParentKey] = true
		refs = append(refs, IssueRef{Key: m.issue.ParentKey, Label: label, Group: "Parent"})
	}

	// Children — grouped by status category (To Do → In Progress → Done).
	if len(m.children) > 0 {
		var todo, inProg, done []jira.ChildIssue
		for _, child := range m.children {
			switch theme.StatusCategory(child.Status) {
			case 2, 3:
				done = append(done, child)
			case 1:
				inProg = append(inProg, child)
			default:
				todo = append(todo, child)
			}
		}
		addGroup := func(group string, children []jira.ChildIssue) {
			for _, child := range children {
				if key := child.Key; key != "" && !seen[key] {
					seen[key] = true
					refs = append(refs, IssueRef{
						Key:   key,
						Label: child.Summary + "  " + child.Status,
						Group: group,
					})
				}
			}
		}
		addGroup(fmt.Sprintf("To Do (%d)", len(todo)), todo)
		addGroup(fmt.Sprintf("In Progress (%d)", len(inProg)), inProg)
		addGroup(fmt.Sprintf("Done (%d)", len(done)), done)
	}

	// Keys in description.
	descGroup := "Description"
	if m.issue.Description != "" {
		for _, key := range issueKeyExtractRe.FindAllString(m.issue.Description, -1) {
			if key != "" && !seen[key] {
				seen[key] = true
				refs = append(refs, IssueRef{Key: key, Label: "in description", Group: descGroup})
			}
		}
	}

	// Keys in comments.
	commentGroup := "Comments"
	for _, c := range m.issue.Comments {
		for _, key := range issueKeyExtractRe.FindAllString(c.Body, -1) {
			if key != "" && !seen[key] {
				seen[key] = true
				refs = append(refs, IssueRef{Key: key, Label: "in comments", Group: commentGroup})
			}
		}
	}

	return refs
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, m.openKeys) && m.issueURL != "" {
			m.openURL = true
			return m, nil
		}
		if key.Matches(msg, m.copyKeys) && m.issueURL != "" {
			m.copyURL = true
			return m, nil
		}
		if key.Matches(msg, m.topKeys) {
			m.viewport.GotoTop()
			return m, nil
		}
		if key.Matches(msg, m.bottomKeys) {
			m.viewport.GotoBottom()
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// View renders the issue view.
func (m Model) View() string {
	if m.issue == nil {
		return theme.StyleSubtle.Render("No issue selected.")
	}

	header := m.renderHeader()
	return lipgloss.JoinVertical(lipgloss.Left, header, m.viewport.View())
}

func (m Model) renderHeader() string {
	if m.issue == nil {
		return ""
	}

	iss := m.issue
	keyStyle := theme.StyleKey
	statusStyle := theme.StatusStyle(iss.Status)

	title := fmt.Sprintf("%s  %s  %s",
		keyStyle.Render(iss.Key),
		iss.Summary,
		statusStyle.Render(fmt.Sprintf("[%s]", iss.Status)),
	)

	scrollPct := fmt.Sprintf("%.0f%%", m.viewport.ScrollPercent()*100)
	info := theme.StyleSubtle.Render(scrollPct)

	gap := strings.Repeat(" ", max(0, m.width-lipgloss.Width(title)-lipgloss.Width(info)))

	headerStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(theme.ColourSubtle)

	return headerStyle.Render(title + gap + info)
}

func (m Model) renderContent() string {
	if m.issue == nil {
		return ""
	}

	iss := m.issue
	var b strings.Builder

	// Metadata.
	labelStyle := theme.StyleSubtle
	writeField := func(label, value string) {
		if value == "" {
			value = "—"
		}
		fmt.Fprintf(&b, "%s %s\n", labelStyle.Render(label+":"), value)
	}

	writeField("Type", theme.TypeStyle(iss.IssueType).Render(iss.IssueType))
	writeField("Priority", theme.PriorityStyle(iss.Priority).Render(iss.Priority))
	writeField("Assignee", theme.UserStyle(iss.Assignee).Render(iss.Assignee))
	writeField("Reporter", theme.UserStyle(iss.Reporter).Render(iss.Reporter))
	if iss.IsWatching {
		writeField("Watching", lipgloss.NewStyle().Foreground(theme.ColourSuccess).Render("yes"))
	}
	if !iss.Created.IsZero() {
		writeField("Created", iss.Created.Local().Format("2 Jan 2006 15:04"))
	}
	if !iss.Updated.IsZero() {
		writeField("Updated", iss.Updated.Local().Format("2 Jan 2006 15:04"))
	}

	if len(iss.Labels) > 0 {
		writeField("Labels", strings.Join(iss.Labels, ", "))
	}

	// Git branches.
	if len(m.branches) > 0 {
		for _, br := range m.branches {
			val := theme.StyleKey.Render(br.Name)
			if br.RemoteCommit > 0 {
				commitStr := fmt.Sprintf("%d commit", br.RemoteCommit)
				if br.RemoteCommit > 1 {
					commitStr += "s"
				}
				val += "  " + theme.StyleStatusInProgress.Render(commitStr+" on remote")
			} else {
				val += "  " + theme.StyleSubtle.Render("no commits on remote")
			}
			writeField("Branch", val)
		}
	}

	// Parent issue.
	if iss.ParentKey != "" {
		parentVal := theme.StyleKey.Render(iss.ParentKey)
		if iss.ParentSummary != "" {
			parentVal += "  " + iss.ParentSummary
		}
		if iss.ParentType != "" {
			parentVal += "  " + theme.StyleSubtle.Render("("+iss.ParentType+")")
		}
		writeField("Parent", parentVal)
	}

	// Child issues grouped by status category.
	if len(m.children) > 0 {
		b.WriteString("\n")
		b.WriteString(theme.StyleTitle.Render(fmt.Sprintf("Child Issues (%d)", len(m.children))))
		b.WriteString("\n")

		// Bucket children by status category.
		var todo, inProgress, done []jira.ChildIssue
		for _, child := range m.children {
			switch theme.StatusCategory(child.Status) {
			case 2, 3:
				done = append(done, child)
			case 1:
				inProgress = append(inProgress, child)
			default:
				todo = append(todo, child)
			}
		}

		// Progress bar — three segments: done (green), in progress (amber), to do (grey).
		total := len(m.children)
		doneCount := len(done)
		progCount := len(inProgress)
		barWidth := 20
		var doneBars, progBars, todoBars int
		if total > 0 {
			doneBars = (doneCount * barWidth) / total
			progBars = (progCount * barWidth) / total
			// Ensure at least 1 bar for non-empty segments that rounded to 0.
			if progCount > 0 && progBars == 0 {
				progBars = 1
			}
			if doneCount > 0 && doneBars == 0 {
				doneBars = 1
			}
			todoBars = max(barWidth-doneBars-progBars, 0)
		} else {
			todoBars = barWidth
		}
		bar := theme.StyleStatusDone.Render(strings.Repeat("█", doneBars)) +
			theme.StyleStatusInProgress.Render(strings.Repeat("█", progBars)) +
			theme.StyleSubtle.Render(strings.Repeat("░", todoBars))
		progressLine := fmt.Sprintf("%d/%d done  %s", doneCount, total, bar)
		b.WriteString(progressLine)
		b.WriteString("\n")

		// Render each non-empty category.
		type group struct {
			label    string
			style    lipgloss.Style
			children []jira.ChildIssue
		}
		groups := []group{
			{"To Do", theme.StyleStatusOpen, todo},
			{"In Progress", theme.StyleStatusInProgress, inProgress},
			{"Done", theme.StyleStatusDone, done},
		}
		for _, g := range groups {
			if len(g.children) == 0 {
				continue
			}
			b.WriteString("\n")
			fmt.Fprintf(&b, "  %s\n", g.style.Render(fmt.Sprintf("%s (%d)", g.label, len(g.children))))
			for _, child := range g.children {
				childStatus := theme.StatusStyle(child.Status).Render(fmt.Sprintf("[%s]", child.Status))
				fmt.Fprintf(&b, "    %s  %s  %s\n",
					theme.StyleKey.Render(child.Key),
					child.Summary,
					childStatus,
				)
			}
		}
	}

	// Description.
	b.WriteString("\n")
	b.WriteString(theme.StyleTitle.Render("Description"))
	b.WriteString("\n\n")

	desc := iss.Description
	if desc == "" {
		desc = theme.StyleSubtle.Render("No description.")
	} else {
		desc = markup.Render(desc, m.width-4)
	}
	b.WriteString(desc)

	// Comments.
	if len(iss.Comments) > 0 {
		b.WriteString("\n\n")
		b.WriteString(theme.StyleTitle.Render(fmt.Sprintf("Comments (%d)", len(iss.Comments))))
		b.WriteString("\n")

		// Show most recent comments (last 10).
		start := 0
		if len(iss.Comments) > 10 {
			start = len(iss.Comments) - 10
		}
		for _, c := range iss.Comments[start:] {
			b.WriteString("\n")
			author := theme.UserStyle(c.Author).Bold(true).Render(c.Author)
			fmt.Fprintf(&b, "%s\n", author)
			body := markup.Render(c.Body, m.width-4)
			b.WriteString(body)
			b.WriteString("\n")
		}
	}

	return b.String()
}

// wrapText wraps text at the given width.
func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}

	var result strings.Builder
	for line := range strings.SplitSeq(text, "\n") {
		if lipgloss.Width(line) <= width {
			result.WriteString(line)
			result.WriteString("\n")
			continue
		}

		words := strings.Fields(line)
		current := ""
		for _, word := range words {
			if current == "" {
				current = word
			} else if lipgloss.Width(current+" "+word) <= width {
				current += " " + word
			} else {
				result.WriteString(current)
				result.WriteString("\n")
				current = word
			}
		}
		if current != "" {
			result.WriteString(current)
			result.WriteString("\n")
		}
	}

	return strings.TrimRight(result.String(), "\n")
}
