package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
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
	viewStatusPicker
	viewSyncing
	viewBulkSyncing
)

type Model struct {
	db     *state.DB
	mb     *musicbrainz.Client
	root   string
	state  viewState
	width  int
	height int

	scanStatus string

	list        listModel
	detail      detailModel
	albumDetail albumDetailModel
	sort        sortModel
	status      statusModel
	sync        syncModel
	bulkSync    bulkSyncModel
	prevState   viewState // view behind the sync modal
}

func New(db *state.DB, root string) Model {
	mb := musicbrainz.New("musup", "0.1.0", "https://github.com/toba/musup")
	return Model{
		db:         db,
		mb:         mb,
		root:       root,
		state:      viewScanning,
		scanStatus: "Scanning music files...",
	}
}

func (m Model) Init() tea.Cmd {
	return m.startScan()
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

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case scanDoneMsg:
		if msg.err != nil {
			m.scanStatus = fmt.Sprintf("Error: %v", msg.err)
			return m, nil
		}
		if len(msg.summaries) == 0 {
			m.scanStatus = "No supported music files found in this directory."
			return m, nil
		}
		m.list = newListModel(m.db, msg.summaries, m.width, m.height)
		m.state = viewList
		return m, nil
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
	}

	switch m.state {
	case viewScanning:
		if msg, ok := msg.(tea.KeyMsg); ok {
			if msg.String() == "q" || msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
		}

	case viewList:
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
		case showStatusMsg:
			m.status = newStatusModel(m.db, msg.artist, msg.current)
			m.state = viewStatusPicker
			return m, nil
		}
		return m, cmd

	case viewDetail:
		var cmd tea.Cmd
		m.detail, cmd = m.detail.Update(msg)
		switch msg := msg.(type) {
		case backToListMsg:
			m.state = viewList
			return m, nil
		case showAlbumDetailMsg:
			m.albumDetail = newAlbumDetailModel(m.db, msg.artist, msg.albumTitle, msg.year)
			m.albumDetail.height = m.height
			m.state = viewAlbumDetail
			return m, nil
		}
		return m, cmd

	case viewAlbumDetail:
		var cmd tea.Cmd
		m.albumDetail, cmd = m.albumDetail.Update(msg)
		if _, ok := msg.(backToDetailMsg); ok {
			m.state = viewDetail
			return m, nil
		}
		return m, cmd

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

	case viewStatusPicker:
		var cmd tea.Cmd
		m.status, cmd = m.status.Update(msg)
		switch msg.(type) {
		case statusChosenMsg:
			m.list.refreshItems()
			m.state = viewList
			return m, nil
		case statusCancelMsg:
			m.state = viewList
			return m, nil
		}
		return m, cmd

	case viewSyncing:
		// Allow quit during sync
		if msg, ok := msg.(tea.KeyMsg); ok {
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
			// On success, refresh list and show detail view with results
			m.list.refreshItems()
			m.detail = newDetailModel(m.db, m.mb, m.sync.artist)
			m.detail.height = m.height
			m.state = viewDetail
			return m, nil
		}

		return m, cmd

	case viewBulkSyncing:
		if msg, ok := msg.(tea.KeyMsg); ok {
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
			m.list.refreshItems()
			m.state = viewList
			return m, nil
		}

		return m, cmd
	}

	return m, nil
}

func (m Model) View() string {
	switch m.state {
	case viewScanning:
		return fmt.Sprintf("\n  %s\n\n  %s\n",
			titleStyle.Render("musup"),
			m.scanStatus,
		)
	case viewList:
		return m.list.View()
	case viewDetail:
		return m.detail.View()
	case viewAlbumDetail:
		return m.albumDetail.View()
	case viewSortPicker:
		return m.sort.View(m.width, m.height, m.list.View())
	case viewStatusPicker:
		return m.status.View(m.width, m.height, m.list.View())
	case viewSyncing:
		bg := m.bgView()
		return m.sync.View(m.width, m.height, bg)
	case viewBulkSyncing:
		return m.bulkSync.View(m.width, m.height, m.list.View())
	}
	return ""
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
