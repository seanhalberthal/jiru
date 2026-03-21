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
	colOffset     int // Index of the first visible column (scroll-on-edge).
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
	m.ensureColumnVisible()
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

// UpdateIssueStatus moves an issue to a new status column in-place.
// The active column and cursor follow the moved card so the user can
// continue transitioning without re-selecting it.
func (m *Model) UpdateIssueStatus(issueKey, newStatus string) {
	// Update the canonical issue list.
	found := false
	for i, iss := range m.allIssues {
		if iss.Key == issueKey {
			m.allIssues[i].Status = newStatus
			found = true
			break
		}
	}
	if !found {
		return
	}

	// Find and remove the issue from its current column.
	var movedIssue jira.Issue
	srcCol := -1
	for ci := range m.columns {
		for ii, iss := range m.columns[ci].issues {
			if iss.Key == issueKey {
				movedIssue = iss
				movedIssue.Status = newStatus
				srcCol = ci
				m.columns[ci].issues = append(m.columns[ci].issues[:ii], m.columns[ci].issues[ii+1:]...)
				m.columns[ci].clampCursor()
				break
			}
		}
		if srcCol >= 0 {
			break
		}
	}
	if srcCol < 0 {
		return
	}

	// Find or create the destination column.
	dstCol := -1
	for ci, col := range m.columns {
		if col.name == newStatus {
			dstCol = ci
			break
		}
	}
	if dstCol < 0 {
		// Status column doesn't exist yet — rebuild columns to get proper ordering.
		m.buildColumns(m.allIssues)
		// Find the card's new column and position the cursor on it.
		for ci, col := range m.columns {
			for ii, iss := range col.issues {
				if iss.Key == issueKey {
					m.activeCol = ci
					m.columns[ci].cursor = ii
					m.columns[ci].visited = true
					m.columns[ci].ensureVisible()
					return
				}
			}
		}
		return
	}

	// Insert the issue at the top of the destination column.
	m.columns[dstCol].issues = append([]jira.Issue{movedIssue}, m.columns[dstCol].issues...)

	// Remove the source column if it's now empty.
	if len(m.columns[srcCol].issues) == 0 {
		m.columns = append(m.columns[:srcCol], m.columns[srcCol+1:]...)
		// Adjust destination index if the removed column was before it.
		if srcCol < dstCol {
			dstCol--
		}
	}

	// Move the active column and cursor to the moved card.
	m.activeCol = dstCol
	m.columns[dstCol].cursor = 0
	m.columns[dstCol].visited = true
	m.columns[dstCol].ensureVisible()

	m.distributeColumnWidths()
	m.ensureColumnVisible()
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
	m.ensureColumnVisible()
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

// maxVisibleCols returns the number of columns that fit on screen.
func (m *Model) maxVisibleCols() int {
	n := len(m.columns)
	mv := min(maxVisibleColumns, n)
	for mv > 1 && m.width/mv < absMinColumnWidth {
		mv--
	}
	return mv
}

// ensureColumnVisible adjusts colOffset so the active column is within the
// visible window. The window only scrolls when the cursor reaches an edge —
// it never centres or pre-scrolls.
func (m *Model) ensureColumnVisible() {
	n := len(m.columns)
	mv := m.maxVisibleCols()
	if mv >= n {
		m.colOffset = 0
		return
	}
	if m.activeCol < m.colOffset {
		m.colOffset = m.activeCol
	} else if m.activeCol >= m.colOffset+mv {
		m.colOffset = m.activeCol - mv + 1
	}
	// Clamp bounds.
	if m.colOffset < 0 {
		m.colOffset = 0
	}
	if m.colOffset+mv > n {
		m.colOffset = n - mv
	}
}

// visibleColumnRange returns the start and end indices of columns to render.
func (m Model) visibleColumnRange() (int, int) {
	n := len(m.columns)
	mv := min(maxVisibleColumns, n)
	for mv > 1 && m.width/mv < absMinColumnWidth {
		mv--
	}
	if mv >= n {
		return 0, n
	}
	start := m.colOffset
	end := start + mv
	if end > n {
		end = n
		start = end - mv
	}
	return start, end
}

func (m *Model) nextColumn() {
	if m.activeCol < len(m.columns)-1 {
		prev := m.columns[m.activeCol].cursor
		m.activeCol++
		m.enterColumn(prev)
		m.ensureColumnVisible()
	}
}

func (m *Model) prevColumn() {
	if m.activeCol > 0 {
		prev := m.columns[m.activeCol].cursor
		m.activeCol--
		m.enterColumn(prev)
		m.ensureColumnVisible()
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
