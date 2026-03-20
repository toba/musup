package tui

import (
	"context"
	"fmt"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/toba/musup/internal/integration/musicbrainz"
	"github.com/toba/musup/internal/scan"
	"github.com/toba/musup/internal/state"
)

type viewState int

const (
	viewScanning viewState = iota
	viewList
	viewDetail
	viewAlbumDetail
	viewSortPicker
	viewSyncing
	viewBulkSyncing
	viewPruning
	viewHelp
)

type Model struct {
	db     *state.DB
	mb     *musicbrainz.Client
	root   string
	state  viewState
	width  int
	height int

	scanStatus string
	scanning   bool // background scan in progress

	list          listModel
	detail        detailModel
	albumDetail   albumDetailModel
	sort          sortModel
	sync          syncModel
	bulkSync      bulkSyncModel
	prune         pruneModel
	prevState     viewState // view behind the sync modal
	prevHelpState viewState // view behind the help modal
}

func New(db *state.DB, root, version string) Model {
	mb := musicbrainz.New("musup", version, "https://github.com/toba/musup")
	return Model{
		db:         db,
		mb:         mb,
		root:       root,
		state:      viewScanning,
		scanStatus: "Loading artists...",
	}
}

func (m Model) Init() tea.Cmd {
	return m.loadCached()
}

type cachedMsg struct {
	summaries []state.ArtistSummary
}

func (m Model) loadCached() tea.Cmd {
	return func() tea.Msg {
		summaries, err := m.db.ArtistSummaries()
		if err != nil || len(summaries) == 0 {
			return cachedMsg{}
		}
		return cachedMsg{summaries: summaries}
	}
}

func (m Model) startScan() tea.Cmd {
	return func() tea.Msg {
		err := scan.Scan(context.Background(), m.db, m.root)
		if err != nil {
			return scanDoneMsg{err: err}
		}
		summaries, err := m.db.ArtistSummaries()
		return scanDoneMsg{summaries: summaries, err: err}
	}
}

type scanDoneMsg struct {
	summaries []state.ArtistSummary
	err       error
}

type startBgSyncMsg struct {
	artist string
}

type bgSyncDoneMsg struct {
	artist string
	err    error
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case cachedMsg:
		if len(msg.summaries) > 0 {
			m.list = newListModel(m.db, msg.summaries, m.width, m.height)
			m.state = viewList
			m.scanning = true
			return m, m.startScan()
		}
		m.scanStatus = "Scanning music files..."
		return m, m.startScan()
	case scanDoneMsg:
		m.scanning = false
		if msg.err != nil {
			if m.state == viewScanning {
				m.scanStatus = fmt.Sprintf("Error: %v", msg.err)
			}
			return m, nil
		}
		if len(msg.summaries) == 0 {
			if m.state == viewScanning {
				m.scanStatus = "No supported music files found in this directory."
			}
			return m, nil
		}
		if m.state == viewScanning {
			// First launch with no cached data.
			m.list = newListModel(m.db, msg.summaries, m.width, m.height)
			m.state = viewList
		} else {
			// Background scan finished — refresh list in place.
			m.list.refreshFrom(msg.summaries)
		}
		return m, nil
	case spinner.TickMsg:
		// Route spinner ticks to the list regardless of current view,
		// so the inline sync spinner keeps animating.
		if m.list.sync != nil && len(m.list.sync.artists) > 0 {
			var cmd tea.Cmd
			m.list.sync.spinner, cmd = m.list.sync.spinner.Update(msg)
			return m, cmd
		}
		return m, nil
	case refreshMsg:
		if len(msg.summaries) > 0 {
			m.list.refreshFrom(msg.summaries)
		}
		return m, nil
	case startBgSyncMsg:
		cmd := m.list.startSyncing(msg.artist)
		return m, tea.Batch(m.startBgSync(msg.artist), cmd)
	case bgSyncDoneMsg:
		m.list.stopSyncing(msg.artist)
		if msg.err != nil {
			m.list.list.NewStatusMessage(errorStyle.Render("sync " + msg.artist + ": " + msg.err.Error()))
		} else {
			m.list.list.NewStatusMessage(localStyle.Render(msg.artist + " synced"))
		}
		return m, m.list.refreshCmd()
	case startSyncMsg:
		m.prevState = m.state
		m.sync = newSyncModel(m.db, m.mb, msg.artist)
		m.state = viewSyncing
		return m, m.sync.Init()
	case startBulkSyncMsg:
		m.prevState = m.state
		m.bulkSync = newBulkSyncModel(m.db, m.mb, msg.artists)
		m.state = viewBulkSyncing
		return m, m.bulkSync.Init()
	case startPruneMsg:
		m.prune = newPruneModel(m.db, msg.artists)
		m.state = viewPruning
		return m, nil
	}

	switch m.state {
	case viewScanning:
		if msg, ok := msg.(tea.KeyPressMsg); ok {
			if msg.String() == "q" || msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
		}

	case viewList:
		if msg, ok := msg.(tea.KeyPressMsg); ok && msg.String() == "?" && m.list.list.FilterState() != list.Filtering {
			m.prevHelpState = viewList
			m.state = viewHelp
			return m, nil
		}
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		// Check for view transitions
		switch msg := msg.(type) {
		case showDetailMsg:
			detail := msg
			m.detail = newDetailModel(m.db, m.mb, detail.artist)
			m.detail.height = m.height
			m.state = viewDetail
			return m, nil
		case showSortMsg:
			m.sort = newSortModel(m.list.sortMode)
			m.state = viewSortPicker
			return m, nil
		}
		return m, cmd

	case viewDetail:
		if msg, ok := msg.(tea.KeyPressMsg); ok && msg.String() == "?" {
			m.prevHelpState = viewDetail
			m.state = viewHelp
			return m, nil
		}
		var cmd tea.Cmd
		m.detail, cmd = m.detail.Update(msg)
		switch msg := msg.(type) {
		case backToListMsg:
			m.state = viewList
			if msg.reviewed {
				return m, m.list.refreshCmd()
			}
			return m, nil
		case showAlbumDetailMsg:
			m.albumDetail = newAlbumDetailModel(m.db, msg.artist, msg.albumTitle, msg.year)
			m.albumDetail.height = m.height
			m.state = viewAlbumDetail
			return m, nil
		}
		return m, cmd

	case viewAlbumDetail:
		if msg, ok := msg.(tea.KeyPressMsg); ok && msg.String() == "?" {
			m.prevHelpState = viewAlbumDetail
			m.state = viewHelp
			return m, nil
		}
		var cmd tea.Cmd
		m.albumDetail, cmd = m.albumDetail.Update(msg)
		if _, ok := msg.(backToDetailMsg); ok {
			m.state = viewDetail
			return m, nil
		}
		return m, cmd

	case viewHelp:
		if _, ok := msg.(tea.KeyPressMsg); ok {
			m.state = m.prevHelpState
			return m, nil
		}

	case viewSortPicker:
		var cmd tea.Cmd
		m.sort, cmd = m.sort.Update(msg)
		switch msg := msg.(type) {
		case sortChosenMsg:
			m.state = viewList
			chosen := msg
			m.list, cmd = m.list.Update(chosen)
			return m, cmd
		case sortCancelMsg:
			m.state = viewList
			return m, nil
		}
		return m, cmd

	case viewSyncing:
		// Allow quit during sync
		if msg, ok := msg.(tea.KeyPressMsg); ok {
			if msg.String() == "q" || msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
		}

		var cmd tea.Cmd
		m.sync, cmd = m.sync.Update(msg)

		if m.sync.done {
			if m.sync.err != nil {
				// On error, go back to where we were, show error in detail view
				m.detail = newDetailModel(m.db, m.mb, m.sync.artist)
				m.detail.height = m.height
				m.detail.fetchErr = m.sync.err
				m.state = viewDetail
				return m, nil
			}
			// On success, refresh list async and show detail view with results
			m.detail = newDetailModel(m.db, m.mb, m.sync.artist)
			m.detail.height = m.height
			m.state = viewDetail
			return m, m.list.refreshCmd()
		}

		return m, cmd

	case viewBulkSyncing:
		if msg, ok := msg.(tea.KeyPressMsg); ok {
			switch msg.String() {
			case "esc":
				m.bulkSync.cancel()
				m.bulkSync.cancelled = true
				return m, nil
			case "q", "ctrl+c":
				return m, tea.Quit
			}
		}

		var cmd tea.Cmd
		m.bulkSync, cmd = m.bulkSync.Update(msg)

		if m.bulkSync.done {
			m.state = viewList
			return m, m.list.refreshCmd()
		}

		return m, cmd

	case viewPruning:
		if msg, ok := msg.(tea.KeyPressMsg); ok {
			if m.prune.done {
				// Any key dismisses the result.
				m.state = viewList
				return m, m.list.refreshCmd()
			}
			if !m.prune.running {
				switch msg.String() {
				case "n", "esc":
					m.state = viewList
					return m, nil
				case "q", "ctrl+c":
					return m, tea.Quit
				}
			}
		}

		var cmd tea.Cmd
		m.prune, cmd = m.prune.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) View() tea.View {
	var s string
	switch m.state {
	case viewScanning:
		s = fmt.Sprintf("\n  %s\n\n  %s\n",
			titleStyle.Render("musup"),
			m.scanStatus,
		)
	case viewList:
		s = m.list.View()
	case viewDetail:
		s = m.detail.View()
	case viewAlbumDetail:
		s = m.albumDetail.View()
	case viewSortPicker:
		s = m.sort.View(m.width, m.height, m.list.View())
	case viewSyncing:
		bg := m.bgView()
		s = m.sync.View(m.width, m.height, bg)
	case viewBulkSyncing:
		s = m.bulkSync.View(m.width, m.height, m.list.View())
	case viewPruning:
		s = m.prune.View(m.width, m.height, m.list.View())
	case viewHelp:
		var bg string
		switch m.prevHelpState {
		case viewDetail:
			bg = m.detail.View()
		case viewAlbumDetail:
			bg = m.albumDetail.View()
		default:
			bg = m.list.View()
		}
		s = helpView(m.width, m.height, bg, m.prevHelpState)
	}

	return tea.NewView(s)
}

func (m Model) startBgSync(artist string) tea.Cmd {
	return func() tea.Msg {
		ch := make(chan tea.Msg, 1)
		go runSync(context.Background(), ch, m.mb, m.db, artist)
		// Drain channel until closed, capture final result.
		var syncErr error
		for msg := range ch {
			if done, ok := msg.(syncDoneMsg); ok {
				syncErr = done.err
			}
		}
		return bgSyncDoneMsg{artist: artist, err: syncErr}
	}
}

func (m Model) bgView() string {
	switch m.prevState {
	case viewDetail:
		return m.detail.View()
	case viewAlbumDetail:
		return m.albumDetail.View()
	default:
		return m.list.View()
	}
}
