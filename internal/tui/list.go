package tui

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/mattn/go-runewidth"

	"github.com/toba/musup/internal/db"
)

type artistItem struct {
	name        string
	albumCount  int    // local
	newestAlbum string // kept for sort
	trackCount  int    // local
	totalAlbums int    // catalog (0 = not synced)
	totalTracks int    // catalog (0 = not synced)
	latestDate  string // ISO date of most recent catalog release
	synced      bool
	followed    bool
	hasNew      bool
}

func (i artistItem) FilterValue() string { return i.name }

// colWidths holds dynamic column widths shared between the list model and delegate.
type colWidths struct {
	trackNum int // max width of track number strings (e.g. "433/1010")
	albumNum int // max width of album number strings (e.g. "2/105")
}

// syncState tracks which artists are currently syncing and provides a spinner.
// Shared by pointer between listModel and artistDelegate so both see updates.
type syncState struct {
	artists map[string]bool
	spinner spinner.Model
}

type artistDelegate struct {
	widths *colWidths
	sync   *syncState
}

func (d artistDelegate) Height() int                             { return 1 }
func (d artistDelegate) Spacing() int                            { return 0 }
func (d artistDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

// fixedWidth returns the total width of non-name columns.
// cursor(2) + indicator(2) + gap(1) + trackNum + " tracks"(7) + gap(2) + albumNum + " albums"(7)
func (d artistDelegate) fixedWidth() int {
	return 2 + 2 + 1 + d.widths.trackNum + 7 + 2 + d.widths.albumNum + 7
}

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

	// Status indicator: 2 chars — spinner if syncing, green if followed, dim otherwise
	var statusInd string
	if d.sync != nil && d.sync.artists[ai.name] {
		statusInd = d.sync.spinner.View() + " "
	} else if ai.followed {
		statusInd = localStyle.Render("• ")
	} else {
		statusInd = subtleStyle.Render("• ")
	}

	// Track column: right-align number, left-align noun
	var trackNum string
	if ai.synced {
		trackNum = fmt.Sprintf("%d/%d", ai.trackCount, ai.totalTracks)
	} else {
		trackNum = strconv.Itoa(ai.trackCount)
	}
	trackNoun := "tracks"
	if !ai.synced {
		trackNoun = pluralize(ai.trackCount, "track", "tracks")
	}
	trackStr := fmt.Sprintf("%*s %-6s", d.widths.trackNum, trackNum, trackNoun)

	// Album column: right-align number, left-align noun
	var albumNum string
	if ai.synced {
		albumNum = fmt.Sprintf("%d/%d", ai.albumCount, ai.totalAlbums)
	} else {
		albumNum = strconv.Itoa(ai.albumCount)
	}
	albumNoun := "albums"
	if !ai.synced {
		albumNoun = pluralize(ai.albumCount, "album", "albums")
	}
	albumStr := fmt.Sprintf("%*s %-6s", d.widths.albumNum, albumNum, albumNoun)

	// Dynamic name column: fill remaining space after fixed-width columns
	nameCol := max(10, m.Width()-d.fixedWidth())

	// Truncate or pad artist name to dynamic column width (rune-aware)
	name := ai.name
	nameWidth := runewidth.StringWidth(name)
	if nameWidth > nameCol {
		name = runewidth.Truncate(name, nameCol-3, "...")
		nameWidth = runewidth.StringWidth(name)
	}
	name += strings.Repeat(" ", max(0, nameCol-nameWidth))

	colStyle := mutedStyle
	if isSelected {
		colStyle = accentStyle
	}

	line := cursor + statusInd + nameStyle.Render(name) + " " + colStyle.Render(trackStr) + "  " + colStyle.Render(albumStr)

	_, _ = fmt.Fprint(w, line)
}

// computeColumnWidths returns the max track and album number string widths for the given items.
func computeColumnWidths(items []artistItem) (trackNumWidth, albumNumWidth int) {
	for _, ai := range items {
		var tw, aw int
		if ai.synced {
			tw = len(fmt.Sprintf("%d/%d", ai.trackCount, ai.totalTracks))
			aw = len(fmt.Sprintf("%d/%d", ai.albumCount, ai.totalAlbums))
		} else {
			tw = len(strconv.Itoa(ai.trackCount))
			aw = len(strconv.Itoa(ai.albumCount))
		}
		trackNumWidth = max(trackNumWidth, tw)
		albumNumWidth = max(albumNumWidth, aw)
	}
	return trackNumWidth, albumNumWidth
}

type listModel struct {
	list        list.Model
	db          *db.DB
	allItems    []artistItem
	colW        *colWidths
	sync        *syncState
	sortMode    sortMode
	showNew     bool // filter to artists with newer catalog albums
	recentYears int  // 0 = off, 1-9 = filter to last N years
}

func newListModel(d *db.DB, summaries []db.ArtistSummariesRow, width, height int) listModel {
	items := summariesToItems(summaries)
	sortArtists(items, sortByName)

	listItems := make([]list.Item, len(items))
	for i := range items {
		listItems[i] = items[i]
	}

	tw, aw := computeColumnWidths(items)
	cw := &colWidths{trackNum: tw, albumNum: aw}
	syncSpinner := newSpinner()
	syncSpinner.Style = lipgloss.NewStyle().Foreground(colorTeal)
	ss := &syncState{
		artists: make(map[string]bool),
		spinner: syncSpinner,
	}
	l := list.New(listItems, artistDelegate{widths: cw, sync: ss}, width, height-2)
	l.Title = fmt.Sprintf("musup — %d artists", len(items))
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle

	return listModel{
		list:     l,
		db:       d,
		allItems: items,
		colW:     cw,
		sync:     ss,
	}
}

func (m listModel) Update(msg tea.Msg) (listModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height-2)
	case tea.KeyPressMsg:
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
		case "U":
			var artists []string
			for _, item := range m.allItems {
				if item.followed {
					artists = append(artists, item.name)
				}
			}
			if len(artists) > 0 {
				return m, func() tea.Msg { return startBulkSyncMsg{artists: artists} }
			}
			m.list.NewStatusMessage(subtleStyle.Render("No followed artists — use f to toggle"))
			return m, nil
		case "f":
			if item, ok := m.list.SelectedItem().(artistItem); ok {
				newFollowed := !item.followed
				if err := m.db.SetFollowed(item.name, newFollowed); err != nil {
					m.list.NewStatusMessage(errorStyle.Render(err.Error()))
					return m, nil
				}
				if newFollowed {
					artist := item.name
					return m, func() tea.Msg { return startBgSyncMsg{artist: artist} }
				}
				m.list.NewStatusMessage(subtleStyle.Render(item.name + " unfollowed"))
				return m, m.refreshCmd()
			}
		case "n":
			m.showNew = !m.showNew
			m.applySort()
			if m.showNew {
				m.list.NewStatusMessage(localStyle.Render("Showing artists with new releases"))
			} else {
				m.list.NewStatusMessage(subtleStyle.Render("Showing all artists"))
			}
			return m, nil
		case "r":
			if item, ok := m.list.SelectedItem().(artistItem); ok {
				if err := m.db.MarkReviewed(item.name); err != nil {
					m.list.NewStatusMessage(errorStyle.Render(err.Error()))
					return m, nil
				}
				m.list.NewStatusMessage(subtleStyle.Render(item.name + " marked as reviewed"))
				return m, m.refreshCmd()
			}
		case "o":
			return m, func() tea.Msg { return showSortMsg{} }
		case "p":
			var unfollowed int
			for _, item := range m.allItems {
				if !item.followed {
					unfollowed++
				}
			}
			if unfollowed == 0 {
				m.list.NewStatusMessage(subtleStyle.Render("No unfollowed artists to prune"))
				return m, nil
			}
			return m, m.startPruneCmd()
		case "0":
			if m.recentYears > 0 {
				m.recentYears = 0
				m.applySort()
				m.list.NewStatusMessage(subtleStyle.Render("Year filter cleared"))
			}
			return m, nil
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			n, _ := strconv.Atoi(msg.String())
			if n == m.recentYears {
				m.recentYears = 0
				m.applySort()
				m.list.NewStatusMessage(subtleStyle.Render("Year filter cleared"))
			} else {
				m.recentYears = n
				m.applySort()
				label := fmt.Sprintf("last %d %s", n, pluralize(n, "year", "years"))
				m.list.NewStatusMessage(localStyle.Render("Showing artists with releases in the " + label))
			}
			return m, nil
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

func (m *listModel) startSyncing(artist string) tea.Cmd {
	m.sync.artists[artist] = true
	if len(m.sync.artists) == 1 {
		return m.sync.spinner.Tick
	}
	return nil
}

func (m *listModel) stopSyncing(artist string) {
	delete(m.sync.artists, artist)
}

type refreshMsg struct {
	summaries []db.ArtistSummariesRow
}

// refreshCmd returns a tea.Cmd that fetches summaries asynchronously.
func (m *listModel) refreshCmd() tea.Cmd {
	return func() tea.Msg {
		summaries, err := m.db.Q.ArtistSummaries(context.Background())
		if err != nil {
			return refreshMsg{}
		}
		return refreshMsg{summaries: summaries}
	}
}

// refreshFrom updates the list with the given summaries (no DB call).
func (m *listModel) refreshFrom(summaries []db.ArtistSummariesRow) {
	m.allItems = summariesToItems(summaries)
	m.colW.trackNum, m.colW.albumNum = computeColumnWidths(m.allItems)
	m.applySort()
}

func (m *listModel) applySort() {
	var minYear int
	if m.recentYears > 0 {
		minYear = time.Now().Year() - m.recentYears + 1
	}

	filtered := make([]artistItem, 0, len(m.allItems))
	for _, item := range m.allItems {
		if m.showNew && (!item.followed || !item.hasNew) {
			continue
		}
		if minYear > 0 {
			if len(item.latestDate) < 4 {
				continue
			}
			y, err := strconv.Atoi(item.latestDate[:4])
			if err != nil || y < minYear {
				continue
			}
		}
		filtered = append(filtered, item)
	}
	sortArtists(filtered, m.sortMode)

	listItems := make([]list.Item, len(filtered))
	for i := range filtered {
		listItems[i] = filtered[i]
	}
	m.list.SetItems(listItems)
	m.updateTitle(len(filtered))
}

func (m *listModel) updateTitle(count int) {
	title := fmt.Sprintf("musup — %d artists", count)
	var suffixes []string
	if m.showNew {
		suffixes = append(suffixes, "new releases")
	}
	if m.recentYears > 0 {
		suffixes = append(suffixes, fmt.Sprintf("last %d %s", m.recentYears, pluralize(m.recentYears, "year", "years")))
	}
	if len(suffixes) > 0 {
		title += " (" + strings.Join(suffixes, ", ") + ")"
	}
	m.list.Title = title
}

func (m listModel) View() string {
	var b strings.Builder
	b.WriteString(m.list.View())
	b.WriteString("\n" + subtleStyle.Render(" /: filter · n: new · 1-9: years · r: reviewed · f: follow · o: sort · q: quit · ?: help"))
	return b.String()
}

type startPruneMsg struct {
	artists []string
}

func (m *listModel) startPruneCmd() tea.Cmd {
	return func() tea.Msg {
		names, err := m.db.Q.ListUnfollowedArtistNames(context.Background())
		if err != nil || len(names) == 0 {
			return nil
		}
		return startPruneMsg{artists: names}
	}
}

type showDetailMsg struct{ artist string }
type showSortMsg struct{}
