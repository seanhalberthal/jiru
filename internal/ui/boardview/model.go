package boardview

import (
	"fmt"
	"sort"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/seanhalberthal/jiru/internal/jira"
	"github.com/seanhalberthal/jiru/internal/theme"
)

// Model is the kanban board view.
type Model struct {
	columns       []column
	activeCol     int
	width         int
	height        int
	title         string
	selected      *jira.Issue
	allIssues     []jira.Issue // Full issue set for rebuilds (e.g. when known statuses arrive).
	knownStatuses []string     // All statuses from the Jira instance (from JQL metadata).
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

// SetKnownStatuses sets the full list of statuses from the Jira instance.
// When set, the board creates columns for all statuses that have issues,
// using the complete status list for proper ordering.
func (m *Model) SetKnownStatuses(statuses []string) {
	// Deduplicate — the /status endpoint can return the same name
	// across different workflows/projects.
	seen := make(map[string]bool, len(statuses))
	deduped := make([]string, 0, len(statuses))
	for _, s := range statuses {
		if !seen[s] {
			seen[s] = true
			deduped = append(deduped, s)
		}
	}
	m.knownStatuses = deduped
	// Rebuild if we already have issues.
	if len(m.allIssues) > 0 {
		m.buildColumns(m.allIssues)
	}
}

// SetIssues populates the board with issues, grouping by status.
func (m *Model) SetIssues(issues []jira.Issue, title string) {
	m.allIssues = issues
	m.title = title
	m.buildColumns(issues)
}

// AppendIssues adds more issues and rebuilds columns (for progressive pagination).
// Deduplicates by issue key to handle overlapping pages.
func (m *Model) AppendIssues(issues []jira.Issue) {
	seen := make(map[string]bool, len(m.allIssues))
	for _, iss := range m.allIssues {
		seen[iss.Key] = true
	}
	for _, iss := range issues {
		if !seen[iss.Key] {
			m.allIssues = append(m.allIssues, iss)
			seen[iss.Key] = true
		}
	}
	m.buildColumns(m.allIssues)
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

func (m *Model) buildColumns(issues []jira.Issue) {
	// Save state from existing columns so we can restore after rebuild —
	// prevents cursor jumping during progressive pagination.
	type colState struct {
		cursor  int
		offset  int
		visited bool
	}
	savedState := make(map[string]colState, len(m.columns))
	for _, col := range m.columns {
		savedState[col.name] = colState{cursor: col.cursor, offset: col.offset, visited: col.visited}
	}

	// Group issues by status.
	statusMap := make(map[string][]jira.Issue)
	for _, iss := range issues {
		statusMap[iss.Status] = append(statusMap[iss.Status], iss)
	}

	var statusOrder []string

	if len(m.knownStatuses) > 0 {
		// Use known statuses, but only include those with issues.
		seen := make(map[string]bool)
		for _, s := range m.knownStatuses {
			if len(statusMap[s]) > 0 {
				statusOrder = append(statusOrder, s)
			}
			seen[s] = true
		}
		// Append any statuses from the data that aren't in the known list.
		for status := range statusMap {
			if !seen[status] {
				statusOrder = append(statusOrder, status)
			}
		}
	} else {
		// Fallback: columns from issue data only.
		for status := range statusMap {
			statusOrder = append(statusOrder, status)
		}
	}

	// Sort by status category: todo (0) → in progress (1) → done (2) → cancelled (3).
	// Within the same category, sort by workflow sub-priority (dev → review → QA).
	// SliceStable preserves original order for statuses with equal category and sub-priority.
	sort.SliceStable(statusOrder, func(i, j int) bool {
		ci, cj := theme.StatusCategory(statusOrder[i]), theme.StatusCategory(statusOrder[j])
		if ci != cj {
			return ci < cj
		}
		return theme.StatusSubPriority(statusOrder[i]) < theme.StatusSubPriority(statusOrder[j])
	})

	m.columns = make([]column, 0, len(statusOrder))
	for _, status := range statusOrder {
		col := newColumn(status, statusMap[status])
		// Restore saved state if this column existed before.
		if st, ok := savedState[status]; ok {
			col.cursor = st.cursor
			col.offset = st.offset
			col.visited = st.visited
		}
		m.columns = append(m.columns, col)
	}

	// Clamp active column.
	if m.activeCol >= len(m.columns) {
		m.activeCol = 0
	}
	for i := range m.columns {
		m.columns[i].clampCursor()
	}
	// The active column is always considered visited.
	if len(m.columns) > 0 {
		m.columns[m.activeCol].visited = true
	}

	m.distributeColumnWidths()
}

// maxVisibleColumns is the hard cap on columns shown at once.
// Navigate with h/l to scroll through remaining columns.
const maxVisibleColumns = 4

// absMinColumnWidth is the absolute floor — columns never go narrower than this.
const absMinColumnWidth = 20

func (m *Model) distributeColumnWidths() {
	n := len(m.columns)
	if n == 0 || m.width == 0 {
		return
	}

	// Show up to maxVisibleColumns, only reducing if the terminal is too
	// narrow to fit them at absMinColumnWidth each.
	maxVisible := min(maxVisibleColumns, n)
	for maxVisible > 1 && m.width/maxVisible < absMinColumnWidth {
		maxVisible--
	}

	// Distribute the full width across the visible columns only.
	available := m.width - (maxVisible - 1) // subtract separators
	colWidth := available / maxVisible

	// Reserve 2 lines for the board title bar.
	contentHeight := max(m.height-2, 7)
	for i := range m.columns {
		m.columns[i].setSize(colWidth, contentHeight)
	}
}

// visibleColumnRange returns the start and end indices of columns to render,
// windowed around the active column.
func (m *Model) visibleColumnRange() (int, int) {
	n := len(m.columns)
	maxVisible := min(maxVisibleColumns, n)
	for maxVisible > 1 && m.width/maxVisible < absMinColumnWidth {
		maxVisible--
	}
	if maxVisible >= n {
		return 0, n
	}

	// Centre the window on the active column.
	half := maxVisible / 2
	start := max(m.activeCol-half, 0)
	end := start + maxVisible
	if end > n {
		end = n
		start = end - maxVisible
	}
	return start, end
}

func (m *Model) nextColumn() {
	if m.activeCol < len(m.columns)-1 {
		prev := m.columns[m.activeCol].cursor
		m.activeCol++
		m.enterColumn(prev)
	}
}

func (m *Model) prevColumn() {
	if m.activeCol > 0 {
		prev := m.columns[m.activeCol].cursor
		m.activeCol--
		m.enterColumn(prev)
	}
}

// enterColumn handles cursor positioning when navigating to a column.
// First visit: carry the source column's cursor position for visual continuity.
// Return visit: restore the column's saved position so the user doesn't lose their place.
func (m *Model) enterColumn(sourceCursor int) {
	col := &m.columns[m.activeCol]
	if col.visited {
		return
	}
	col.visited = true
	col.setCursor(sourceCursor)
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
