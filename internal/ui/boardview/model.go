package boardview

import (
	"fmt"
	"sort"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/seanhalberthal/jiru/internal/jira"
	"github.com/seanhalberthal/jiru/internal/theme"
)

// ParentGroup represents a unique parent issue that can be used for filtering.
type ParentGroup struct {
	Key       string // e.g., "PROJ-42"
	Summary   string // e.g., "User Authentication"
	IssueType string // e.g., "Epic", "Feature", "Initiative"
}

// Model is the kanban board view.
type Model struct {
	columns      []column
	activeCol    int
	width        int
	height       int
	title        string
	parentFilter string        // If set, only show issues from this parent key.
	parentGroups []ParentGroup // Available parent groups derived from issue data.
	parentLabel  string        // Dynamic label for the parent type (e.g., "Epic", "Feature").
	selected     *jira.Issue
	allIssues    []jira.Issue // Unfiltered issue set.
}

// New creates a new board view model.
func New() Model {
	return Model{}
}

// SetSize updates the board dimensions.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.distributeColumnWidths()
}

// SetIssues populates the board with issues, grouping by status.
// Also extracts available parent groups for filtering.
func (m *Model) SetIssues(issues []jira.Issue, title string) {
	m.allIssues = issues
	m.title = title
	m.parentGroups = extractParentGroups(issues)
	m.parentLabel = deriveParentLabel(m.parentGroups)
	m.buildColumns(issues)
}

// AppendIssues adds more issues and rebuilds columns (for progressive pagination).
func (m *Model) AppendIssues(issues []jira.Issue) {
	m.allIssues = append(m.allIssues, issues...)
	m.buildColumns(m.allIssues)
	m.parentGroups = extractParentGroups(m.allIssues)
	m.parentLabel = deriveParentLabel(m.parentGroups)
}

// SetParentFilter filters the board to show only issues from the given parent.
// Pass "" to clear the filter.
func (m *Model) SetParentFilter(parentKey string) {
	m.parentFilter = parentKey
	if parentKey == "" {
		m.buildColumns(m.allIssues)
		return
	}
	filtered := make([]jira.Issue, 0)
	for _, iss := range m.allIssues {
		if iss.ParentKey == parentKey {
			filtered = append(filtered, iss)
		}
	}
	m.buildColumns(filtered)
}

// SelectedIssue returns the issue the user selected (if any) and resets.
func (m *Model) SelectedIssue() (jira.Issue, bool) {
	if m.selected == nil {
		return jira.Issue{}, false
	}
	iss := *m.selected
	m.selected = nil
	return iss, true
}

// HighlightedIssue returns the currently highlighted issue without consuming it.
func (m *Model) HighlightedIssue() (jira.Issue, bool) {
	if len(m.columns) == 0 || m.activeCol >= len(m.columns) {
		return jira.Issue{}, false
	}
	if iss := m.columns[m.activeCol].selectedIssue(); iss != nil {
		return *iss, true
	}
	return jira.Issue{}, false
}

// ParentFilter returns the current parent filter key.
func (m *Model) ParentFilter() string {
	return m.parentFilter
}

// ParentGroups returns the available parent groups for filtering.
func (m *Model) ParentGroups() []ParentGroup {
	return m.parentGroups
}

// ParentLabel returns the dynamic label for the parent type (e.g., "Epic", "Feature").
func (m *Model) ParentLabel() string {
	return m.parentLabel
}

// extractParentGroups collects unique parent issues from the issue set.
func extractParentGroups(issues []jira.Issue) []ParentGroup {
	seen := make(map[string]bool)
	var groups []ParentGroup
	for _, iss := range issues {
		if iss.ParentKey != "" && !seen[iss.ParentKey] {
			seen[iss.ParentKey] = true
			groups = append(groups, ParentGroup{
				Key:       iss.ParentKey,
				Summary:   iss.ParentSummary,
				IssueType: iss.ParentType,
			})
		}
	}
	return groups
}

// deriveParentLabel determines what to call the parent type based on actual data.
// If all parents share the same issue type, use that (e.g., "Feature").
// If mixed or unknown, fall back to "Parent".
func deriveParentLabel(groups []ParentGroup) string {
	if len(groups) == 0 {
		return "Parent"
	}
	label := groups[0].IssueType
	if label == "" {
		return "Parent"
	}
	for _, g := range groups[1:] {
		if g.IssueType != label {
			return "Parent" // Mixed types, use generic label.
		}
	}
	return label // All same type — use it (e.g., "Epic", "Feature", "Initiative").
}

func (m *Model) buildColumns(issues []jira.Issue) {
	// Group issues by status.
	statusMap := make(map[string][]jira.Issue)
	statusOrder := make([]string, 0)

	for _, iss := range issues {
		if _, exists := statusMap[iss.Status]; !exists {
			statusOrder = append(statusOrder, iss.Status)
		}
		statusMap[iss.Status] = append(statusMap[iss.Status], iss)
	}

	// Sort columns by status category: To Do → In Progress → Done.
	sort.SliceStable(statusOrder, func(i, j int) bool {
		return theme.StatusCategory(statusOrder[i]) < theme.StatusCategory(statusOrder[j])
	})

	m.columns = make([]column, len(statusOrder))
	for i, status := range statusOrder {
		m.columns[i] = newColumn(status, statusMap[status])
	}

	// Clamp active column.
	if m.activeCol >= len(m.columns) {
		m.activeCol = 0
	}
	for i := range m.columns {
		m.columns[i].clampCursor()
	}

	m.distributeColumnWidths()
}

// minColumnWidth is the narrowest a column should be rendered.
const minColumnWidth = 30

func (m *Model) distributeColumnWidths() {
	n := len(m.columns)
	if n == 0 || m.width == 0 {
		return
	}

	// Determine how many columns fit at a readable width.
	maxVisible := m.width / minColumnWidth
	if maxVisible < 1 {
		maxVisible = 1
	}
	if maxVisible > n {
		maxVisible = n
	}

	// Distribute the full width across the visible columns only.
	available := m.width - (maxVisible - 1) // subtract separators
	colWidth := available / maxVisible

	// Reserve 2 lines for the board title bar.
	contentHeight := m.height - 2
	if contentHeight < 7 {
		contentHeight = 7
	}
	for i := range m.columns {
		m.columns[i].setSize(colWidth, contentHeight)
	}
}

// visibleColumnRange returns the start and end indices of columns to render,
// windowed around the active column.
func (m *Model) visibleColumnRange() (int, int) {
	n := len(m.columns)
	maxVisible := m.width / minColumnWidth
	if maxVisible < 1 {
		maxVisible = 1
	}
	if maxVisible >= n {
		return 0, n
	}

	// Centre the window on the active column.
	half := maxVisible / 2
	start := m.activeCol - half
	if start < 0 {
		start = 0
	}
	end := start + maxVisible
	if end > n {
		end = n
		start = end - maxVisible
	}
	return start, end
}

func (m *Model) nextColumn() {
	if m.activeCol < len(m.columns)-1 {
		m.activeCol++
	}
}

func (m *Model) prevColumn() {
	if m.activeCol > 0 {
		m.activeCol--
	}
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if len(m.columns) == 0 {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			m.columns[m.activeCol].moveDown()
		case "k", "up":
			m.columns[m.activeCol].moveUp()
		case "h", "left", "shift+tab":
			m.prevColumn()
		case "l", "right", "tab":
			m.nextColumn()
		case "d":
			m.columns[m.activeCol].moveHalfPageDown()
		case "u":
			m.columns[m.activeCol].moveHalfPageUp()
		case "enter":
			if iss := m.columns[m.activeCol].selectedIssue(); iss != nil {
				m.selected = iss
			}
		case "g":
			m.columns[m.activeCol].cursor = 0
			m.columns[m.activeCol].offset = 0
		case "G":
			col := &m.columns[m.activeCol]
			if len(col.issues) > 0 {
				col.cursor = len(col.issues) - 1
				col.ensureVisible()
			}
		}
	}

	return m, nil
}

// View renders the kanban board.
func (m Model) View() string {
	if len(m.columns) == 0 {
		return theme.StyleSubtle.Render("No issues to display")
	}

	// Title bar — dynamic, reflects the data source (sprint name, board name, etc.).
	titleText := m.title
	if m.parentFilter != "" {
		filterLabel := m.parentLabel
		for _, g := range m.parentGroups {
			if g.Key == m.parentFilter {
				if g.Summary != "" {
					filterLabel = fmt.Sprintf("%s: %s %s", m.parentLabel, g.Key, g.Summary)
				} else {
					filterLabel = fmt.Sprintf("%s: %s", m.parentLabel, g.Key)
				}
				break
			}
		}
		titleText += " — " + filterLabel
	}
	// Show column position if not all columns are visible.
	start, end := m.visibleColumnRange()
	if end-start < len(m.columns) {
		titleText += fmt.Sprintf(" [%d/%d]", m.activeCol+1, len(m.columns))
	}
	title := theme.StyleTitle.Render(titleText)

	// Render only the visible column window.
	var colViews []string
	for i := start; i < end; i++ {
		active := i == m.activeCol
		rendered := m.columns[i].view(active)

		// Apply column border (separator between columns).
		if i < end-1 {
			rendered = theme.StyleColumnBorder.Render(rendered)
		}

		colViews = append(colViews, rendered)
	}

	board := lipgloss.JoinHorizontal(lipgloss.Top, colViews...)

	result := lipgloss.JoinVertical(lipgloss.Left, title, board)

	// Constrain output to available height so the board never pushes the
	// title or footer off-screen at small terminal sizes.
	if m.height > 0 {
		result = lipgloss.NewStyle().MaxHeight(m.height).Render(result)
	}

	return result
}
