package tui

import (
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/paginator"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/toba/musup/internal/db"
)

type keyMap struct {
	Up             key.Binding
	Down           key.Binding
	Left           key.Binding
	Right          key.Binding
	NextFollowed   key.Binding
	PrevFollowed   key.Binding
	Toggle         key.Binding
	NextPage       key.Binding
	PrevPage       key.Binding
	Detail         key.Binding
	Help           key.Binding
	FilterInactive key.Binding
	FilterFollowed key.Binding
	Quit           key.Binding
}

var keys = keyMap{
	Up:             key.NewBinding(key.WithKeys("up")),
	Down:           key.NewBinding(key.WithKeys("down")),
	Left:           key.NewBinding(key.WithKeys("left")),
	Right:          key.NewBinding(key.WithKeys("right")),
	NextFollowed:   key.NewBinding(key.WithKeys("shift+down")),
	PrevFollowed:   key.NewBinding(key.WithKeys("shift+up")),
	Toggle:         key.NewBinding(key.WithKeys("space")),
	NextPage:       key.NewBinding(key.WithKeys("pgdown")),
	PrevPage:       key.NewBinding(key.WithKeys("pgup")),
	Detail:         key.NewBinding(key.WithKeys("enter")),
	Help:           key.NewBinding(key.WithKeys("?")),
	FilterInactive: key.NewBinding(key.WithKeys(".")),
	FilterFollowed: key.NewBinding(key.WithKeys("/")),
	Quit:           key.NewBinding(key.WithKeys("esc", "ctrl+c")),
}

func buildHelpContent() string {
	entries := [][2]string{
		{"↑ ↓", "Move up/down"},
		{"⇧↑ ⇧↓", "Jump to followed"},
		{"← →", "Move left/right"},
		{"space", "Toggle follow"},
		{"enter", "Show albums/tracks"},
		{"p", "Pin discography modal"},
		{"pgdn/pgup", "Next/previous page"},
		{"a-z", "Jump to artist"},
		{"1-9", "Filter by release recency"},
		{"*", "Check for new releases"},
		{"/", "Show only followed"},
		{".", "Show inactive artists"},
		{"?", "Show this help"},
		{"esc", "Quit"},
	}
	const colWidth = 14
	var sb strings.Builder
	for i, e := range entries {
		if i > 0 {
			sb.WriteByte('\n')
		}
		styled := helpKeyStyle.Render(e[0])
		pad := max(colWidth-lipgloss.Width(e[0]), 1)
		sb.WriteString(styled + strings.Repeat(" ", pad) + e[1])
	}
	return sb.String()
}

var (
	checkStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	mutedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	selectedStyle = lipgloss.NewStyle().Bold(true).Reverse(true)
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	helpKeyStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("80"))
	searchStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	modalStyle    = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2)
	modalTitleStyle  = lipgloss.NewStyle().Bold(true)
	pinnedModalStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("214")).
				Padding(1, 2)
	albumStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("33"))
	newReleaseStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
)

type artist struct {
	id            int64
	name          string
	followed      bool
	inactive      bool
	latestRelease string
}

type modalTrack struct {
	path        string
	album       string
	releaseYear string
	line        string // e.g. "  3. Title"
}

type modalKind int

const (
	modalHelp modalKind = iota
	modalDiscography
	modalConfirmFetch
	modalNewReleases
)

type modalData struct {
	kind       modalKind
	artistName string
	followed   bool
	content    string       // pre-rendered for help/confirm modals
	tracks     []modalTrack // for discography modal
	cursor     int
	pinned     bool // pinned mode: modal stays open while navigating artists
}

type searchClearMsg int
type yearInputMsg int
type pinnedRefreshMsg int

// FetchInactiveFunc queries MusicBrainz for each artist's inactive status.
// It should call onProgress for each artist processed and check ctx for cancellation.
type FetchInactiveFunc func(ctx context.Context, d *db.DB, onProgress func(name string)) (map[int64]bool, error)

type fetchState struct {
	mu   sync.Mutex
	name string
}

func (fs *fetchState) set(name string) {
	fs.mu.Lock()
	fs.name = name
	fs.mu.Unlock()
}

func (fs *fetchState) get() string {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.name
}

type inactiveDataMsg struct {
	inactive map[int64]bool
	err      error
}

type Model struct {
	allArtists      []artist // full list from DB
	artists         []artist // currently visible (filtered)
	cursor          int      // page-relative index
	cols            int
	db              *db.DB
	musicRoot       string
	fetchInactive   FetchInactiveFunc
	fetchCancel     context.CancelFunc
	fetchProgress   *fetchState
	paginator       paginator.Model
	modal           *modalData
	spinner         spinner.Model
	search          string
	searchGen       int
	filterInactive  bool
	hideUnfollowed  bool
	yearInput       string
	yearInputGen    int
	pinnedGen       int
	filterYears     int
	newReleases     map[int64][]db.FollowedNewerReleasesRow
	newReleasesMode bool
	fetching        bool
	width           int
	height          int
	err             error
}

func New(d *db.DB, musicRoot string, fetchInactive FetchInactiveFunc) Model {
	p := paginator.New()
	p.Type = paginator.Arabic
	p.PerPage = 1
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	return Model{db: d, musicRoot: musicRoot, fetchInactive: fetchInactive, spinner: sp, cols: 3, paginator: p}
}

// Err returns any error that caused the TUI to exit.
func (m Model) Err() error {
	return m.err
}

func (m Model) Init() tea.Cmd {
	return m.loadArtists
}

func (m Model) loadArtists() tea.Msg {
	rows, err := m.db.Q.AlbumArtists(context.Background())
	if err != nil {
		return errMsg{err}
	}
	artists := make([]artist, len(rows))
	for i, r := range rows {
		artists[i] = artist{id: r.ID, name: r.Name, followed: r.Followed == 1, inactive: r.Inactive == 1, latestRelease: r.LatestRelease}
	}
	var nr map[int64][]db.FollowedNewerReleasesRow
	if nrRows, nrErr := m.db.Q.FollowedNewerReleases(context.Background()); nrErr == nil && len(nrRows) > 0 {
		nr = make(map[int64][]db.FollowedNewerReleasesRow)
		for _, r := range nrRows {
			nr[r.ArtistID] = append(nr[r.ArtistID], r)
		}
	}
	return artistsMsg{artists: artists, newReleases: nr}
}

type artistsMsg struct {
	artists     []artist
	newReleases map[int64][]db.FollowedNewerReleasesRow
}
type errMsg struct{ err error }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updatePagination()
	case artistsMsg:
		m.allArtists = msg.artists
		m.newReleases = msg.newReleases
		m.applyFilter()
		m.cursor = 0
		m.updatePagination()
	case errMsg:
		m.err = msg.err
		return m, tea.Quit
	case searchClearMsg:
		if int(msg) == m.searchGen {
			m.search = ""
		}
	case yearInputMsg:
		if int(msg) == m.yearInputGen && m.yearInput != "" {
			n, err := strconv.Atoi(m.yearInput)
			if err == nil {
				m.filterYears = n
				m.applyFilter()
				m.cursor = 0
				m.paginator.Page = 0
				m.updatePagination()
			}
			m.yearInput = ""
		}
	case pinnedRefreshMsg:
		if m.modal != nil && m.modal.pinned && int(msg) == m.pinnedGen && len(m.artists) > 0 {
			a := m.artists[m.globalIndex()]
			newModal := m.buildDiscographyModal(a)
			newModal.pinned = true
			m.modal = newModal
		}
	case spinner.TickMsg:
		if m.fetching {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	case inactiveDataMsg:
		m.fetching = false
		m.fetchCancel = nil
		if msg.err != nil {
			if !strings.Contains(msg.err.Error(), "context canceled") {
				m.modal = &modalData{kind: modalHelp, artistName: "Error", content: fmt.Sprintf("Failed to fetch inactive status: %v", msg.err)}
			} else {
				m.modal = nil
			}
			return m, nil
		}
		for i := range m.allArtists {
			if inactive, ok := msg.inactive[m.allArtists[i].id]; ok {
				m.allArtists[i].inactive = inactive
			}
		}
		m.modal = nil
		m.filterInactive = true
		m.applyFilter()
		m.cursor = 0
		m.paginator.Page = 0
		m.updatePagination()
	case tea.KeyPressMsg:
		if m.modal != nil {
			return m.handleModalKey(msg)
		}
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleModalKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch m.modal.kind {
	case modalHelp:
		m.modal = nil

	case modalNewReleases:
		switch {
		case key.Matches(msg, keys.Up):
			if m.modal.cursor > 0 {
				m.modal.cursor--
			}
		case key.Matches(msg, keys.Down):
			if m.modal.cursor < len(m.modal.tracks)-1 {
				m.modal.cursor++
			}
		case key.Matches(msg, keys.Detail):
			if len(m.modal.tracks) > 0 {
				t := m.modal.tracks[m.modal.cursor]
				searchWeb(fmt.Sprintf("%q %s", m.modal.artistName, t.album))
			}
		case key.Matches(msg, keys.Quit):
			m.modal = nil
		}

	case modalConfirmFetch:
		if m.fetching {
			if key.Matches(msg, keys.Quit) && m.fetchCancel != nil {
				m.fetchCancel()
			}
			return m, nil
		}
		switch {
		case key.Matches(msg, keys.Detail):
			m.fetching = true
			m.modal = &modalData{
				kind:       modalConfirmFetch,
				artistName: "Fetching Inactive Status",
				content:    m.spinner.View() + " Starting...",
			}
			return m, tea.Batch(m.spinner.Tick, m.startFetch())
		case key.Matches(msg, keys.Quit):
			m.modal = nil
		}

	case modalDiscography:
		if m.modal.pinned {
			k := msg.Key()
			if k.Code == tea.KeyEscape {
				m.modal = nil
				return m, nil
			}
			if k.Text == "c" && k.Mod.Contains(tea.ModCtrl) {
				return m, tea.Quit
			}
			// Delegate all other keys to main handler.
			model, cmd := m.handleKey(msg)
			m = model.(Model) //nolint:errcheck // type is always Model
			// Debounce modal refresh: update title immediately, rebuild content after a short delay.
			if m.modal != nil && len(m.artists) > 0 {
				a := m.artists[m.globalIndex()]
				m.modal.artistName = a.name
				m.modal.followed = a.followed
				m.pinnedGen++
				gen := m.pinnedGen
				cmd = tea.Batch(cmd, tea.Tick(50*time.Millisecond, func(time.Time) tea.Msg {
					return pinnedRefreshMsg(gen)
				}))
			}
			return m, cmd
		}
		switch {
		case key.Matches(msg, keys.Up):
			if m.modal.cursor > 0 {
				m.modal.cursor--
			}
		case key.Matches(msg, keys.Down):
			if m.modal.cursor < len(m.modal.tracks)-1 {
				m.modal.cursor++
			}
		case key.Matches(msg, keys.Detail):
			if len(m.modal.tracks) > 0 {
				t := m.modal.tracks[m.modal.cursor]
				openFile(filepath.Join(m.musicRoot, t.path))
			}
		case msg.Key().Text == "p":
			m.modal.pinned = true
		case key.Matches(msg, keys.Quit):
			m.modal = nil
		}
	}
	return m, nil
}

func openFile(path string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	_ = cmd.Start()
}

func searchWeb(query string) {
	u := "https://www.google.com/search?q=" + url.QueryEscape(query)
	openFile(u)
}

func (m *Model) startFetch() tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background()) //nolint:gosec // cancel stored for user-initiated esc
	m.fetchCancel = cancel
	m.fetchProgress = &fetchState{}

	fs := m.fetchProgress
	fetchFn := m.fetchInactive
	db := m.db
	return func() tea.Msg {
		result, err := fetchFn(ctx, db, func(name string) {
			fs.set(name)
		})
		return inactiveDataMsg{inactive: result, err: err}
	}
}

func (m *Model) applyFilter() {
	source := m.allArtists

	if m.newReleasesMode && m.newReleases != nil {
		var filtered []artist
		for _, a := range source {
			if _, ok := m.newReleases[a.id]; ok {
				filtered = append(filtered, a)
			}
		}
		m.artists = filtered
		return
	}

	if m.hideUnfollowed {
		var filtered []artist
		for _, a := range source {
			if a.followed {
				filtered = append(filtered, a)
			}
		}
		source = filtered
	}

	if m.filterInactive {
		var filtered []artist
		for _, a := range source {
			if a.inactive {
				filtered = append(filtered, a)
			}
		}
		source = filtered
	}

	if m.filterYears > 0 {
		cutoff := time.Now().AddDate(-m.filterYears, 0, 0).Format("2006-01-02")
		var filtered []artist
		for _, a := range source {
			if a.latestRelease == "" || a.latestRelease < cutoff {
				filtered = append(filtered, a)
			}
		}
		source = filtered
	}

	m.artists = source
}

func (m *Model) updatePagination() {
	perPage := max(m.rowsPerCol()*m.cols, 1)
	m.paginator.PerPage = perPage
	m.paginator.SetTotalPages(len(m.artists))
	m.clampCursor()
}

func (m *Model) clampCursor() {
	itemsOnPage := m.paginator.ItemsOnPage(len(m.artists))
	if m.cursor >= itemsOnPage {
		m.cursor = itemsOnPage - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *Model) globalIndex() int {
	return m.paginator.Page*m.paginator.PerPage + m.cursor
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	rows := m.rowsPerCol()

	switch {
	case key.Matches(msg, keys.Quit):
		if m.newReleasesMode {
			m.newReleasesMode = false
			m.applyFilter()
			m.cursor = 0
			m.paginator.Page = 0
			m.updatePagination()
			return m, nil
		}
		return m, tea.Quit

	case key.Matches(msg, keys.PrevFollowed):
		jumpToFollowed(&m, -1)

	case key.Matches(msg, keys.NextFollowed):
		jumpToFollowed(&m, 1)

	case key.Matches(msg, keys.Up):
		if m.cursor > 0 {
			m.cursor--
		} else if !m.paginator.OnFirstPage() {
			m.paginator.PrevPage()
			m.cursor = m.paginator.ItemsOnPage(len(m.artists)) - 1
		}

	case key.Matches(msg, keys.Down):
		itemsOnPage := m.paginator.ItemsOnPage(len(m.artists))
		if m.cursor < itemsOnPage-1 {
			m.cursor++
		} else if !m.paginator.OnLastPage() {
			m.paginator.NextPage()
			m.cursor = 0
		}

	case key.Matches(msg, keys.Left):
		if m.cursor >= rows {
			m.cursor -= rows
		} else if !m.paginator.OnFirstPage() {
			row := m.cursor
			m.paginator.PrevPage()
			itemsOnPage := m.paginator.ItemsOnPage(len(m.artists))
			lastCol := (itemsOnPage - 1) / rows
			m.cursor = lastCol*rows + row
			m.clampCursor()
		}

	case key.Matches(msg, keys.Right):
		next := m.cursor + rows
		itemsOnPage := m.paginator.ItemsOnPage(len(m.artists))
		if next < itemsOnPage {
			m.cursor = next
		} else if !m.paginator.OnLastPage() {
			row := m.cursor % rows
			m.paginator.NextPage()
			m.cursor = row
			m.clampCursor()
		}

	case key.Matches(msg, keys.NextPage):
		if !m.paginator.OnLastPage() {
			m.paginator.NextPage()
			m.cursor = 0
			m.clampCursor()
		}

	case key.Matches(msg, keys.PrevPage):
		if !m.paginator.OnFirstPage() {
			m.paginator.PrevPage()
			m.cursor = 0
		}

	case key.Matches(msg, keys.Toggle):
		if len(m.artists) > 0 {
			gi := m.globalIndex()
			a := &m.artists[gi]
			a.followed = !a.followed
			followed := int64(0)
			if a.followed {
				followed = 1
			}
			_ = m.db.Q.SetFollowed(context.Background(), followed, a.id)
			if !a.followed {
				_ = m.db.Q.DeleteAlbumsByArtist(context.Background(), a.id)
				delete(m.newReleases, a.id)
			}
			// Sync back to allArtists.
			for i := range m.allArtists {
				if m.allArtists[i].id == a.id {
					m.allArtists[i].followed = a.followed
					break
				}
			}

			itemsOnPage := m.paginator.ItemsOnPage(len(m.artists))
			if m.cursor < itemsOnPage-1 {
				m.cursor++
			} else if !m.paginator.OnLastPage() {
				m.paginator.NextPage()
				m.cursor = 0
			}
		}

	case key.Matches(msg, keys.Detail):
		if len(m.artists) > 0 {
			a := m.artists[m.globalIndex()]
			if m.newReleasesMode {
				m.modal = m.buildNewReleasesModal(a)
			} else {
				m.modal = m.buildDiscographyModal(a)
			}
		}

	case key.Matches(msg, keys.FilterFollowed):
		m.hideUnfollowed = !m.hideUnfollowed
		m.applyFilter()
		m.cursor = 0
		m.paginator.Page = 0
		m.updatePagination()

	case key.Matches(msg, keys.FilterInactive):
		if m.filterInactive {
			// Toggle off.
			m.filterInactive = false
			m.applyFilter()
			m.cursor = 0
			m.paginator.Page = 0
			m.updatePagination()
		} else {
			// Check if ended data exists.
			hasInactive := false
			for _, a := range m.allArtists {
				if a.inactive {
					hasInactive = true
					break
				}
			}
			if hasInactive {
				m.filterInactive = true
				m.applyFilter()
				m.cursor = 0
				m.paginator.Page = 0
				m.updatePagination()
			} else {
				m.modal = &modalData{
					kind:       modalConfirmFetch,
					artistName: "Inactive Dates Not Available",
					content: "Artist inactive (deceased or disbanded) dates have not yet been retrieved.\n\n" +
						helpKeyStyle.Render("enter") + "  Fetch from MusicBrainz (1 artist/sec)\n" +
						helpKeyStyle.Render("esc") + "    Cancel",
				}
			}
		}

	case key.Matches(msg, keys.Help):
		m.modal = &modalData{artistName: "Keyboard Shortcuts", content: buildHelpContent()}

	default:
		k := msg.Key()
		if len(k.Text) == 1 {
			r := rune(k.Text[0])
			if r == '*' {
				rows, qErr := m.db.Q.FollowedNewerReleases(context.Background())
				if qErr == nil && len(rows) > 0 {
					m.newReleases = make(map[int64][]db.FollowedNewerReleasesRow)
					for _, row := range rows {
						m.newReleases[row.ArtistID] = append(m.newReleases[row.ArtistID], row)
					}
					m.newReleasesMode = true
					m.applyFilter()
					m.cursor = 0
					m.paginator.Page = 0
					m.updatePagination()
				}
				return m, nil
			}
			if r >= '0' && r <= '9' {
				m.yearInput += string(r)
				m.yearInputGen++
				gen := m.yearInputGen
				return m, tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg {
					return yearInputMsg(gen)
				})
			}
			if m.yearInput != "" {
				// Non-digit while accumulating year input — cancel.
				m.yearInput = ""
				return m, nil
			}
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				m.search += strings.ToLower(string(r))
				m.searchGen++
				m.jumpToSearch()
				gen := m.searchGen
				return m, tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg {
					return searchClearMsg(gen)
				})
			}
		}
	}
	return m, nil
}

func (m *Model) jumpToSearch() {
	if m.search == "" || len(m.artists) == 0 {
		return
	}
	for i, a := range m.artists {
		if !strings.HasPrefix(strings.ToLower(a.name), m.search) {
			continue
		}
		perPage := m.paginator.PerPage
		m.paginator.Page = i / perPage
		m.cursor = i % perPage
		m.clampCursor()
		return
	}
}

func jumpToFollowed(m *Model, dir int) {
	gi := m.globalIndex()
	for i := gi + dir; i >= 0 && i < len(m.artists); i += dir {
		if !m.artists[i].followed {
			continue
		}
		perPage := m.paginator.PerPage
		m.paginator.Page = i / perPage
		m.cursor = i % perPage
		m.clampCursor()
		return
	}
}

type albumBlock struct {
	name        string
	releaseYear string
	tracks      []modalTrack
}

func (m Model) buildDiscographyModal(a artist) *modalData {
	rows, err := m.db.Q.ArtistLocalTracks(context.Background(), a.id)
	if err != nil {
		return &modalData{artistName: a.name, content: fmt.Sprintf("Error: %v", err)}
	}
	if len(rows) == 0 {
		return &modalData{artistName: a.name, content: "No local tracks found."}
	}

	// Build flat track list with album grouping info.
	var allTracks []modalTrack
	var currentAlbum string
	for _, r := range rows {
		var line string
		if r.TrackNumber > 0 {
			line = fmt.Sprintf("  %2d. %s", r.TrackNumber, r.Title)
		} else {
			line = "      " + r.Title
		}
		if r.Album != currentAlbum {
			currentAlbum = r.Album
		}
		year := ""
		if len(r.ReleaseDate) >= 4 {
			year = r.ReleaseDate[:4]
		}
		allTracks = append(allTracks, modalTrack{path: r.Path, album: r.Album, releaseYear: year, line: line})
	}

	return &modalData{
		kind:       modalDiscography,
		artistName: a.name,
		followed:   a.followed,
		tracks:     allTracks,
		cursor:     0,
	}
}

func (m Model) buildNewReleasesModal(a artist) *modalData {
	releases := m.newReleases[a.id]
	if len(releases) == 0 {
		return &modalData{kind: modalNewReleases, artistName: a.name, followed: a.followed, content: "No new releases found."}
	}
	var tracks []modalTrack
	for _, r := range releases {
		line := albumStyle.Render(r.AlbumTitle) + "  " + mutedStyle.Render(r.ReleaseDate)
		if r.SecondaryTypes != "" {
			line += "  " + mutedStyle.Render("["+r.SecondaryTypes+"]")
		}
		tracks = append(tracks, modalTrack{album: r.AlbumTitle, line: line})
	}
	return &modalData{kind: modalNewReleases, artistName: a.name, followed: a.followed, tracks: tracks, cursor: 0}
}

func (m Model) renderDiscographyContent(md *modalData, showCursor bool, maxWidth int) string {
	// Content width inside modal: subtract border (2) + padding (4).
	colWidth := maxWidth - 6
	// Group tracks by album.
	var blocks []albumBlock
	var current albumBlock
	for _, t := range md.tracks {
		if t.album != current.name {
			if len(current.tracks) > 0 {
				blocks = append(blocks, current)
			}
			current = albumBlock{name: t.album, releaseYear: t.releaseYear}
		}
		current.tracks = append(current.tracks, t)
	}
	if len(current.tracks) > 0 {
		blocks = append(blocks, current)
	}

	// Calculate line counts for balanced split.
	type renderedBlock struct {
		lines     []string
		lineCount int
	}

	trackIdx := 0
	var rendered []renderedBlock
	for _, b := range blocks {
		var lines []string
		albumTitle := albumStyle.Render(b.name)
		if b.releaseYear != "" {
			albumTitle += " " + mutedStyle.Render(b.releaseYear)
		}
		lines = append(lines, albumTitle)
		for _, t := range b.tracks {
			line := t.line
			if showCursor && trackIdx == md.cursor {
				line = selectedStyle.Render(line)
			}
			lines = append(lines, line)
			trackIdx++
		}
		rendered = append(rendered, renderedBlock{lines: lines, lineCount: len(lines) + 1}) // +1 for spacing
	}

	totalLines := 0
	for _, r := range rendered {
		totalLines += r.lineCount
	}

	var leftLines, rightLines []string
	leftCount := 0
	half := totalLines / 2
	for _, r := range rendered {
		if leftCount <= half {
			leftLines = append(leftLines, r.lines...)
			leftLines = append(leftLines, "") // spacing between albums
			leftCount += r.lineCount
		} else {
			rightLines = append(rightLines, r.lines...)
			rightLines = append(rightLines, "")
		}
	}

	// Truncate lines to fit within modal content area.
	trackWidth := colWidth
	if len(rightLines) > 0 {
		trackWidth = (colWidth - 4) / 2 // 4-char gap between columns
	}
	if trackWidth > 0 {
		for i, l := range leftLines {
			leftLines[i] = truncate(l, trackWidth)
		}
		for i, l := range rightLines {
			rightLines[i] = truncate(l, trackWidth)
		}
	}

	leftCol := strings.Join(leftLines, "\n")
	if len(rightLines) == 0 {
		return leftCol
	}
	rightCol := strings.Join(rightLines, "\n")
	return lipgloss.JoinHorizontal(lipgloss.Top, leftCol, "    ", rightCol)
}

func (m Model) rowsPerCol() int {
	avail := max(
		// blank line + help bar + padding
		m.height-3, 1)
	return avail
}

func (m Model) View() tea.View {
	var v tea.View
	v.AltScreen = true

	if m.err != nil {
		v.Content = fmt.Sprintf("Error: %v\n", m.err)
		return v
	}
	if len(m.artists) == 0 {
		v.Content = "No artists found. Run 'musup scan <path>' first.\n"
		return v
	}

	rows := m.rowsPerCol()
	colWidth := max(m.width/m.cols, 20)

	start, end := m.paginator.GetSliceBounds(len(m.artists))
	pageArtists := m.artists[start:end]

	colStrs := make([]string, m.cols)
	for col := range m.cols {
		var lines []string
		for row := range rows {
			idx := col*rows + row
			if idx >= len(pageArtists) {
				lines = append(lines, strings.Repeat(" ", colWidth))
				continue
			}
			a := pageArtists[idx]
			line := m.renderItem(a, idx == m.cursor, colWidth)
			lines = append(lines, line)
		}
		colStrs[col] = lipgloss.JoinVertical(lipgloss.Left, lines...)
	}

	body := lipgloss.JoinHorizontal(lipgloss.Top, colStrs...)

	var helpText string
	switch {
	case m.newReleasesMode:
		helpText = searchStyle.Render("NEW RELEASES") +
			helpStyle.Render(" (esc to return)  "+m.paginator.View())
	case m.yearInput != "":
		helpText = helpStyle.Render("years: ") + searchStyle.Render(m.yearInput) + helpStyle.Render("          "+m.paginator.View())
	case m.search != "":
		helpText = helpStyle.Render("search: ") + searchStyle.Render(m.search) + helpStyle.Render("          "+m.paginator.View())
	case m.hideUnfollowed || m.filterInactive || m.filterYears > 0:
		var parts []string
		var clearHints []string
		if m.hideUnfollowed {
			parts = append(parts, "FOLLOWED")
			clearHints = append(clearHints, "/")
		}
		if m.filterInactive {
			parts = append(parts, "INACTIVE")
			clearHints = append(clearHints, ".")
		}
		if m.filterYears > 0 {
			parts = append(parts, fmt.Sprintf("NO RELEASE %dY", m.filterYears))
			clearHints = append(clearHints, "0")
		}
		helpText = searchStyle.Render(strings.Join(parts, " + ")) +
			helpStyle.Render(" ("+strings.Join(clearHints, " / ")+" to clear)  "+m.paginator.View())
	default:
		helpText = helpKeyStyle.Render("↑↓←→") + helpStyle.Render(" navigate | ") +
			helpKeyStyle.Render("space") + helpStyle.Render(" toggle | ") +
			helpKeyStyle.Render("enter") + helpStyle.Render(" detail | ") +
			helpKeyStyle.Render("pgup/dn") + helpStyle.Render(" page | ") +
			helpKeyStyle.Render("?") + helpStyle.Render(" help | ") +
			helpKeyStyle.Render("esc") + helpStyle.Render(" quit  "+m.paginator.View())
	}
	content := body + "\n\n" + helpText

	if m.modal != nil {
		maxW := m.width - 4
		maxH := m.height - 4
		if maxW < 20 {
			maxW = 20
		}
		if maxH < 5 {
			maxH = 5
		}

		var inner string
		var title string
		if m.modal.kind == modalDiscography {
			if m.modal.followed {
				title = checkStyle.Render("✓") + " " + modalTitleStyle.Render(m.modal.artistName)
			} else {
				title = mutedStyle.Render("· " + m.modal.artistName)
			}
		} else {
			title = modalTitleStyle.Render(m.modal.artistName)
		}
		switch m.modal.kind {
		case modalDiscography:
			body := m.renderDiscographyContent(m.modal, !m.modal.pinned, maxW)
			var footer string
			if m.modal.pinned {
				footer = helpKeyStyle.Render("esc") + helpStyle.Render(" close")
			} else {
				footer = helpKeyStyle.Render("↑↓") + helpStyle.Render(" select | ") +
					helpKeyStyle.Render("enter") + helpStyle.Render(" open | ") +
					helpKeyStyle.Render("p") + helpStyle.Render(" pin | ") +
					helpKeyStyle.Render("esc") + helpStyle.Render(" close")
			}
			inner = title + "\n\n" + body + "\n" + footer
		case modalNewReleases:
			var body string
			if len(m.modal.tracks) > 0 {
				var lines []string
				for i, t := range m.modal.tracks {
					line := t.line
					if i == m.modal.cursor {
						line = selectedStyle.Render(line)
					}
					lines = append(lines, line)
				}
				body = strings.Join(lines, "\n")
			} else {
				body = m.modal.content
			}
			footer := helpKeyStyle.Render("↑↓") + helpStyle.Render(" select | ") +
				helpKeyStyle.Render("enter") + helpStyle.Render(" search | ") +
				helpKeyStyle.Render("esc") + helpStyle.Render(" close")
			inner = title + "\n\n" + body + "\n" + footer
		case modalConfirmFetch:
			content := m.modal.content
			if m.fetching && m.fetchProgress != nil {
				content = m.spinner.View() + " " + m.fetchProgress.get()
			}
			footer := "\n" + helpKeyStyle.Render("esc") + helpStyle.Render(" cancel")
			inner = title + "\n\n" + content + footer
		default:
			footer := helpStyle.Render("Press any key to close")
			inner = title + "\n\n" + m.modal.content + "\n" + footer
		}

		style := modalStyle
		if m.modal.pinned {
			style = pinnedModalStyle
		}
		box := style.MaxWidth(maxW).MaxHeight(maxH).Render(inner)
		content = placeOverlay(m.width, m.height, box, content)
	}

	v.Content = content
	return v
}

var cursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))

func (m Model) renderItem(a artist, selected bool, width int) string {
	var prefix, name string
	if m.newReleasesMode {
		if selected {
			prefix = cursorStyle.Render("▸")
		} else {
			prefix = " "
		}
		name = a.name
	} else if selected {
		prefix = cursorStyle.Render("▸")
		if a.followed {
			name = a.name
		} else {
			name = mutedStyle.Render(a.name)
		}
	} else if a.followed {
		prefix = checkStyle.Render("✓")
		name = a.name
	} else {
		prefix = mutedStyle.Render("·")
		name = mutedStyle.Render(a.name)
	}

	nameWidth := width - 3
	hasNewRelease := !m.newReleasesMode && m.newReleases != nil
	if hasNewRelease {
		if _, ok := m.newReleases[a.id]; ok {
			nameWidth--
		} else {
			hasNewRelease = false
		}
	}
	line := prefix + " " + truncate(name, nameWidth)
	if hasNewRelease {
		line += newReleaseStyle.Render("*")
	}

	return padToWidth(line, width)
}

func truncate(s string, max int) string {
	w := lipgloss.Width(s)
	if w <= max {
		return s
	}
	runes := []rune(s)
	for len(runes) > 0 && lipgloss.Width(string(runes)) > max-1 {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}

func padToWidth(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

// placeOverlay composites fg centered over bg, preserving bg content around the edges.
func placeOverlay(width, height int, fg, bg string) string {
	bgLines := strings.Split(bg, "\n")
	fgLines := strings.Split(fg, "\n")

	for len(bgLines) < height {
		bgLines = append(bgLines, "")
	}

	fgW := lipgloss.Width(fg)
	fgH := len(fgLines)

	startRow := (height - fgH) / 2
	startCol := (width - fgW) / 2
	if startRow < 0 {
		startRow = 0
	}
	if startCol < 0 {
		startCol = 0
	}

	for i, fgLine := range fgLines {
		bgRow := startRow + i
		if bgRow >= len(bgLines) {
			break
		}

		bgLine := bgLines[bgRow]
		bgW := lipgloss.Width(bgLine)
		if bgW < width {
			bgLine += strings.Repeat(" ", width-bgW)
		}

		left := ansiTruncate(bgLine, startCol)
		fgLineW := lipgloss.Width(fgLine)
		rightStart := startCol + fgLineW
		right := ""
		if rightStart < width {
			right = ansiCutLeft(bgLine, rightStart)
		}

		bgLines[bgRow] = left + fgLine + right
	}

	return strings.Join(bgLines[:height], "\n")
}

func ansiTruncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	var result strings.Builder
	visible := 0
	inEsc := false
	for _, r := range s {
		if r == '\x1b' {
			inEsc = true
			result.WriteRune(r)
			continue
		}
		if inEsc {
			result.WriteRune(r)
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inEsc = false
			}
			continue
		}
		if visible >= n {
			break
		}
		result.WriteRune(r)
		visible++
	}
	for visible < n {
		result.WriteByte(' ')
		visible++
	}
	return result.String()
}

func ansiCutLeft(s string, n int) string {
	if n <= 0 {
		return s
	}
	var result strings.Builder
	visible := 0
	inEsc := false
	started := false
	for _, r := range s {
		if r == '\x1b' {
			inEsc = true
			if started {
				result.WriteRune(r)
			}
			continue
		}
		if inEsc {
			if started {
				result.WriteRune(r)
			}
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inEsc = false
			}
			continue
		}
		if visible >= n {
			started = true
			result.WriteRune(r)
		}
		visible++
	}
	return result.String()
}
