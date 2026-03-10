package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/toba/musup/internal/state"
)

type artistItem struct {
	name        string
	albumCount  int
	newestAlbum string
}

func (i artistItem) FilterValue() string { return i.name }

type artistDelegate struct{}

func (d artistDelegate) Height() int                             { return 1 }
func (d artistDelegate) Spacing() int                            { return 0 }
func (d artistDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d artistDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	ai, ok := item.(artistItem)
	if !ok {
		return
	}

	isSelected := index == m.Index()

	cursor := "  "
	nameStyle := lipgloss.NewStyle()
	if isSelected {
		cursor = cursorStyle.Render("> ")
		nameStyle = nameStyle.Foreground(colorAccent).Bold(true)
	}

	noun := "albums"
	if ai.albumCount == 1 {
		noun = "album"
	}
	meta := mutedStyle.Render(fmt.Sprintf(" %d %s", ai.albumCount, noun))
	newest := ""
	if ai.newestAlbum != "" && m.Width() > 60 {
		newest = subtleStyle.Render(" · " + ai.newestAlbum)
	}

	line := cursor + nameStyle.Render(ai.name) + meta + newest

	_, _ = fmt.Fprint(w, line)
}

type listModel struct {
	list     list.Model
	db       *state.DB
	allItems []artistItem
	sortMode sortMode
}

func newListModel(db *state.DB, summaries []state.ArtistSummary, width, height int) listModel {
	items := make([]artistItem, len(summaries))
	for i, s := range summaries {
		items[i] = artistItem{
			name:        s.Name,
			albumCount:  s.AlbumCount,
			newestAlbum: s.NewestAlbum,
		}
	}

	listItems := make([]list.Item, len(items))
	for i := range items {
		listItems[i] = items[i]
	}

	l := list.New(listItems, artistDelegate{}, width, height-2)
	l.Title = fmt.Sprintf("musup — %d artists", len(items))
	l.SetShowHelp(false)
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle
	l.Styles.FilterPrompt = lipgloss.NewStyle().Foreground(colorAccent)
	l.Styles.FilterCursor = lipgloss.NewStyle().Foreground(colorAccent)

	return listModel{
		list:     l,
		db:       db,
		allItems: items,
	}
}

func (m listModel) Update(msg tea.Msg) (listModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height-2)
	case tea.KeyMsg:
		// Don't intercept keys when filtering
		if m.list.FilterState() == list.Filtering {
			break
		}
		switch msg.String() {
		case "enter":
			if item, ok := m.list.SelectedItem().(artistItem); ok {
				return m, func() tea.Msg { return showDetailMsg{artist: item.name} }
			}
		case "o":
			return m, func() tea.Msg { return showSortMsg{} }
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	case sortChosenMsg:
		m.sortMode = msg.mode
		m.applySort()
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *listModel) applySort() {
	sorted := make([]artistItem, len(m.allItems))
	copy(sorted, m.allItems)
	sortArtists(sorted, m.sortMode)

	listItems := make([]list.Item, len(sorted))
	for i := range sorted {
		listItems[i] = sorted[i]
	}
	m.list.SetItems(listItems)
}

func (m listModel) View() string {
	var b strings.Builder
	b.WriteString(m.list.View())
	b.WriteString("\n" + subtleStyle.Render(" /: filter · o: sort · enter: detail · q: quit"))
	return b.String()
}

type showDetailMsg struct{ artist string }
type showSortMsg struct{}
