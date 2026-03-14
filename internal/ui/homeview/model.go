package homeview

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/seanhalberthal/jiru/internal/jira"
)

type Model struct {
	list     list.Model
	boards   []jira.BoardStats
	width    int
	height   int
	selected *jira.Board
	openKeys key.Binding
}

func New() Model {
	delegate := NewDelegate()
	l := list.New(nil, delegate, 0, 0)
	l.Title = "Boards"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)
	l.Styles.Title = l.Styles.Title.Bold(true)

	return Model{
		list:     l,
		openKeys: key.NewBinding(key.WithKeys("enter")),
	}
}

func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.list.SetSize(width, height)
}

func (m *Model) SetBoards(boards []jira.BoardStats) {
	m.boards = boards
	items := make([]list.Item, len(boards))
	for i, b := range boards {
		items[i] = boardItem{stats: b}
	}
	m.list.SetItems(items)
	m.list.Title = fmt.Sprintf("Boards (%d)", len(boards))
}

func (m *Model) SelectedBoard() *jira.Board {
	b := m.selected
	m.selected = nil
	return b
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		// Don't handle keys when filtering.
		if m.list.FilterState() != list.Filtering {
			if key.Matches(keyMsg, m.openKeys) {
				if item, ok := m.list.SelectedItem().(boardItem); ok {
					board := item.stats.Board
					m.selected = &board
					return m, nil
				}
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// Filtering returns true when the list filter input is active.
func (m Model) Filtering() bool {
	return m.list.FilterState() == list.Filtering
}

func (m Model) View() string {
	return m.list.View()
}
