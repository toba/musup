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
	viewSortPicker
)

type Model struct {
	db     *state.DB
	mb     *musicbrainz.Client
	root   string
	state  viewState
	width  int
	height int

	scanStatus string

	list   listModel
	detail detailModel
	sort   sortModel
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
		}
		return m, cmd

	case viewDetail:
		var cmd tea.Cmd
		m.detail, cmd = m.detail.Update(msg)
		if _, ok := msg.(backToListMsg); ok {
			m.state = viewList
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
	case viewSortPicker:
		return m.sort.View(m.width, m.height, m.list.View())
	}
	return ""
}
