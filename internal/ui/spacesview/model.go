package spacesview

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/seanhalberthal/jiru/internal/confluence"
	"github.com/seanhalberthal/jiru/internal/recents"
	"github.com/seanhalberthal/jiru/internal/theme"
)

type state int

const (
	stateSpaces state = iota
	statePages
)

// Model is the space/page browser view.
type Model struct {
	list         list.Model
	state        state
	width        int
	height       int
	spaces       []confluence.Space
	pages        []confluence.Page
	recents      []recents.Entry
	selected     *selectedPage // sentinel: page selected for navigation
	spaceID      string        // ID of the space being browsed
	spaceKey     string
	fetchSpaceID string // sentinel: space ID needing page fetch (one-shot)
	dismissed    bool
	openKeys     key.Binding
	backKeys     key.Binding
}

type selectedPage struct {
	ID      string
	Title   string
	SpaceID string
}

func New() Model {
	l := list.New(nil, spaceDelegate{}, 0, 0)
	l.Title = "Confluence Spaces"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)
	l.Styles.Title = theme.StyleTitle
	l.Styles.StatusBar = l.Styles.StatusBar.Foreground(theme.ColourSubtle)
	l.Styles.StatusBarFilterCount = l.Styles.StatusBarFilterCount.Foreground(theme.ColourSubtle)

	// Override default pagination keys to remove f/d/b/u which conflict
	// with the app's global keybindings.
	l.KeyMap.NextPage.SetKeys("right", "l", "pgdown")
	l.KeyMap.PrevPage.SetKeys("left", "h", "pgup")

	return Model{
		list:     l,
		state:    stateSpaces,
		openKeys: key.NewBinding(key.WithKeys("enter")),
		backKeys: key.NewBinding(key.WithKeys("esc", "backspace")),
	}
}

func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.list.SetSize(width, height)
}

// SetSpaces populates the spaces list.
func (m *Model) SetSpaces(spaces []confluence.Space) {
	m.spaces = spaces
	m.rebuildSpacesList()
}

// SetRecents sets the recently viewed pages shown above spaces.
func (m *Model) SetRecents(entries []recents.Entry) {
	m.recents = entries
	if m.state == stateSpaces {
		m.rebuildSpacesList()
	}
}

// SetPages populates the pages list for the selected space.
func (m *Model) SetPages(pages []confluence.Page) {
	m.pages = pages
	items := make([]list.Item, len(pages))
	for i, p := range pages {
		items[i] = pageItem{page: p}
	}
	m.list.SetItems(items)
	m.list.Title = fmt.Sprintf("Pages in %s (%d)", m.spaceKey, len(pages))
}

// SelectedPage returns the page selected for navigation (sentinel, resets after read).
func (m *Model) SelectedPage() *selectedPage {
	p := m.selected
	m.selected = nil
	return p
}

// NeedsSpacePages returns the space ID that needs pages fetched, or empty string.
func (m *Model) NeedsSpacePages() string {
	return ""
}

// SpaceSelected returns the selected space ID if a space was drilled into.
func (m *Model) SpaceSelected() (spaceID string, ok bool) {
	return "", false
}

// Dismissed returns true if the user pressed back from the top level.
func (m *Model) Dismissed() bool {
	d := m.dismissed
	m.dismissed = false
	return d
}

// Filtering returns true when the list filter input is active.
func (m Model) Filtering() bool {
	return m.list.FilterState() == list.Filtering
}

// Filtered returns true when a filter has been applied (but input is not active).
func (m Model) Filtered() bool {
	return m.list.FilterState() == list.FilterApplied
}

// ResetFilter clears the applied filter.
func (m *Model) ResetFilter() {
	m.list.ResetFilter()
}

// GoToSpaces transitions from the pages view back to the spaces list.
func (m *Model) GoToSpaces() {
	m.state = stateSpaces
	m.rebuildSpacesList()
}

// NeedsFetch returns the space ID that needs pages fetched (one-shot sentinel).
func (m *Model) NeedsFetch() string {
	id := m.fetchSpaceID
	m.fetchSpaceID = ""
	return id
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if m.list.FilterState() != list.Filtering {
			if key.Matches(keyMsg, m.openKeys) {
				return m.handleOpen()
			}
			if key.Matches(keyMsg, m.backKeys) {
				return m.handleBack()
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) handleOpen() (Model, tea.Cmd) {
	sel := m.list.SelectedItem()
	if sel == nil {
		return m, nil
	}

	switch item := sel.(type) {
	case recentItem:
		m.selected = &selectedPage{
			ID:    item.entry.PageID,
			Title: item.entry.Title,
		}
	case spaceItem:
		m.list.ResetFilter()
		m.state = statePages
		m.spaceID = item.space.ID
		m.spaceKey = item.space.Key
		m.fetchSpaceID = item.space.ID
		m.list.SetItems(nil)
		m.list.Title = fmt.Sprintf("Pages in %s (loading...)", displayKey(item.space))
		// Parent will check NeedsFetch() and fetch pages.
	case pageItem:
		m.selected = &selectedPage{
			ID:      item.page.ID,
			Title:   item.page.Title,
			SpaceID: item.page.SpaceID,
		}
	}
	return m, nil
}

func (m Model) handleBack() (Model, tea.Cmd) {
	if m.state == statePages {
		m.state = stateSpaces
		m.rebuildSpacesList()
		return m, nil
	}
	m.dismissed = true
	return m, nil
}

// InPagesState returns true when showing pages for a space.
func (m Model) InPagesState() bool {
	return m.state == statePages
}

// CurrentSpaceID returns the space ID currently being browsed.
func (m Model) CurrentSpaceID() string {
	return m.spaceID
}

func (m *Model) rebuildSpacesList() {
	var items []list.Item

	// Add recent pages first.
	for _, e := range m.recents {
		items = append(items, recentItem{entry: e})
	}

	// Add spaces — global first, personal last.
	for _, s := range m.spaces {
		if s.Type != "personal" {
			items = append(items, spaceItem{space: s})
		}
	}
	for _, s := range m.spaces {
		if s.Type == "personal" {
			items = append(items, spaceItem{space: s})
		}
	}

	m.list.SetItems(items)
	total := len(m.spaces)
	if len(m.recents) > 0 {
		m.list.Title = fmt.Sprintf("Confluence (%d spaces, %d recent)", total, len(m.recents))
	} else {
		m.list.Title = fmt.Sprintf("Confluence Spaces (%d)", total)
	}
}

func (m Model) View() string {
	return m.list.View()
}

// --- Custom delegate ---

// spaceDelegate renders space/page items with the same cursor style as the Jira views.
type spaceDelegate struct{}

func (d spaceDelegate) Height() int                             { return 2 }
func (d spaceDelegate) Spacing() int                            { return 0 }
func (d spaceDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d spaceDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	isSelected := index == m.Index()
	width := m.Width()

	cursor := "  "
	if isSelected {
		cursor = lipgloss.NewStyle().
			Foreground(theme.ColourPrimary).
			Bold(true).
			Render("▸ ")
	}

	var line1, line2 string

	switch i := item.(type) {
	case recentItem:
		title := i.entry.Title
		if isSelected {
			title = theme.StyleKey.Underline(true).Render(title)
		}
		line1 = cursor + title
		desc := "Recently viewed"
		if i.entry.SpaceKey != "" && !strings.HasPrefix(i.entry.SpaceKey, "~") {
			desc += " · " + i.entry.SpaceKey
		}
		line2 = "  " + theme.StyleSubtle.Render(desc)

	case spaceItem:
		name := i.space.Name
		if isSelected {
			name = theme.StyleKey.Underline(true).Render(name)
		}
		// Show the space key as a badge, but hide ugly personal space hashes.
		keyBadge := ""
		if !strings.HasPrefix(i.space.Key, "~") {
			keyBadge = " " + theme.StyleSubtle.Render("["+i.space.Key+"]")
		}
		line1 = cursor + name + keyBadge
		desc := i.space.Type
		if i.space.Description != "" {
			desc += " · " + truncate(i.space.Description, 60)
		}
		line2 = "  " + theme.StyleSubtle.Render(desc)

	case pageItem:
		title := i.page.Title
		if isSelected {
			title = theme.StyleKey.Underline(true).Render(title)
		}
		line1 = cursor + title
		line2 = "  " + theme.StyleSubtle.Render(fmt.Sprintf("v%d", i.page.Version))
	}

	// Truncate first line to terminal width.
	if lipgloss.Width(line1) > width && width > 6 {
		line1 = line1[:width-3] + "..."
	}

	_, _ = fmt.Fprintf(w, "%s\n%s", line1, line2)
}

// displayKey returns a human-readable key for a space,
// falling back to the name for personal spaces with hash keys.
func displayKey(s confluence.Space) string {
	if strings.HasPrefix(s.Key, "~") {
		return s.Name
	}
	return s.Key
}

// --- List items ---

type recentItem struct {
	entry recents.Entry
}

func (i recentItem) Title() string       { return i.entry.Title }
func (i recentItem) Description() string { return "" }
func (i recentItem) FilterValue() string { return i.entry.Title + " " + i.entry.SpaceKey }

type spaceItem struct {
	space confluence.Space
}

func (i spaceItem) Title() string       { return i.space.Name }
func (i spaceItem) Description() string { return "" }
func (i spaceItem) FilterValue() string { return i.space.Key + " " + i.space.Name }

type pageItem struct {
	page confluence.Page
}

func (i pageItem) Title() string       { return i.page.Title }
func (i pageItem) Description() string { return "" }
func (i pageItem) FilterValue() string { return i.page.Title }

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
