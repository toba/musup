package tui

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/toba/musup/internal/state"
)

type artistItem struct {
	name        string
	albumCount  int    // local
	newestAlbum string // kept for sort
	trackCount  int    // local
	totalAlbums int    // catalog (0 = not synced)
	totalTracks int    // catalog (0 = not synced)
	synced      bool
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

	// Sync indicator: 2 chars
	syncInd := "  "
	if ai.synced {
		syncInd = localStyle.Render("· ")
	}

	const (
		nameCol  = 30
		numWidth = 7 // fits "xxx/yyy"
	)

	// Track column: right-align number part within fixed width
	var trackNum string
	if ai.synced {
		trackNum = fmt.Sprintf("%d/%d", ai.trackCount, ai.totalTracks)
	} else {
		trackNum = strconv.Itoa(ai.trackCount)
	}
	trackNoun := "tracks"
	if !ai.synced && ai.trackCount == 1 {
		trackNoun = "track"
	}
	trackStr := fmt.Sprintf("%*s %s", numWidth, trackNum, trackNoun)

	// Album column: right-align number part within fixed width
	var albumNum string
	if ai.synced {
		albumNum = fmt.Sprintf("in %d/%d", ai.albumCount, ai.totalAlbums)
	} else {
		albumNum = strconv.Itoa(ai.albumCount)
	}
	albumNoun := "albums"
	if !ai.synced && ai.albumCount == 1 {
		albumNoun = "album"
	}
	albumStr := fmt.Sprintf("%*s %s", numWidth, albumNum, albumNoun)

	// Truncate or pad artist name to fixed column width
	name := ai.name
	if len(name) > nameCol {
		name = name[:nameCol-3] + "..."
	}
	name += strings.Repeat(" ", max(0, nameCol-len(name)))

	line := cursor + syncInd + nameStyle.Render(name) + " " + mutedStyle.Render(trackStr) + "  " + mutedStyle.Render(albumStr)

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
			trackCount:  s.TrackCount,
			totalAlbums: s.TotalAlbums,
			totalTracks: s.TotalTracks,
			synced:      s.Synced,
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
		case "u":
			if item, ok := m.list.SelectedItem().(artistItem); ok {
				return m, func() tea.Msg { return startSyncMsg{artist: item.name} }
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

func (m *listModel) refreshItems() {
	summaries, err := m.db.ArtistSummaries()
	if err != nil {
		return
	}
	items := make([]artistItem, len(summaries))
	for i, s := range summaries {
		items[i] = artistItem{
			name:        s.Name,
			albumCount:  s.AlbumCount,
			newestAlbum: s.NewestAlbum,
			trackCount:  s.TrackCount,
			totalAlbums: s.TotalAlbums,
			totalTracks: s.TotalTracks,
			synced:      s.Synced,
		}
	}
	m.allItems = items
	m.applySort()
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
	b.WriteString("\n" + subtleStyle.Render(" /: filter · o: sort · u: sync · enter: detail · q: quit"))
	return b.String()
}

type showDetailMsg struct{ artist string }
type showSortMsg struct{}
