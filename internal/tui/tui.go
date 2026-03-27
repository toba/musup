package tui

import (
	"cmp"
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/paginator"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
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
		{"space", "un/follow"},
		{"pgdn/pgup", "Next/previous page"},
		{"a-z", "Jump to artist"},
		{"1-9", "Filter by release recency"},
		{"enter", "Search artist"},
		{"=", "Toggle tracks"},
		{"*", "Sync releases"},
		{",", "Mark caught up"},
		{"/", "Show only followed"},
		{"-", "Vacuum database"},
		{".", "Show inactive artists"},
		{":", "Edit search URL"},
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
	modalTitleStyle = lipgloss.NewStyle().Bold(true)
	paneStyle       = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62"))
	albumStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("33"))
	newReleaseStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	datePrefixRe    = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}[:\s]\s*`)
)

const dateFormat = "2006-01-02"

type artist struct {
	id            int64
	name          string
	followed      bool
	inactive      bool
	latestRelease string
	reviewedAt    string
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
	modalSearchURL
)

type modalData struct {
	kind       modalKind
	artistName string
	followed   bool
	content    string       // pre-rendered for help/confirm modals
	tracks     []modalTrack // for discography modal
	cursor     int
}

type searchClearMsg int
type yearInputMsg int
type discogRefreshMsg int

// FetchInactiveFunc queries MusicBrainz for each artist's inactive status.
// It should call onProgress for each artist processed and check ctx for cancellation.
type FetchInactiveFunc func(ctx context.Context, d *db.DB, onProgress func(name string)) (map[int64]bool, error)

// SyncArtistFunc syncs one artist's releases from MusicBrainz.
type SyncArtistFunc func(ctx context.Context, d *db.DB, artistID int64) error

// ScanFunc scans the music root for files and updates the database.
type ScanFunc func(ctx context.Context, d *db.DB) error

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

type syncArtistDoneMsg struct {
	artistID int64
	err      error
}

type scanDoneMsg struct{ err error }
type scanRefreshMsg int
type vacuumDoneMsg struct{ err error }

type Model struct {
	allArtists     []artist // full list from DB
	artists        []artist // currently visible (filtered)
	cursor         int      // page-relative index
	cols           int
	db             *db.DB
	musicRoot      string
	fetchInactive  FetchInactiveFunc
	fetchCancel    context.CancelFunc
	fetchProgress  *fetchState
	syncArtistFn   SyncArtistFunc
	scanFn         ScanFunc
	scanning       bool
	scanGen        int
	syncCtx        context.Context //nolint:containedctx // needed for cancellation across tea.Cmd chain
	syncCancel     context.CancelFunc
	syncQueue      []int64 // artist IDs remaining to sync
	syncCurrentID  int64   // artist being synced right now (0 = none)
	syncTotal      int
	syncDone       int
	paginator      paginator.Model
	modal          *modalData
	spinner        spinner.Model
	search         string
	searchGen      int
	filterInactive bool
	hideUnfollowed bool
	yearInput      string
	yearInputGen   int
	discog         *modalData
	discogGen      int
	showTracks     bool
	filterYears    int
	newReleases    map[int64][]db.FollowedNewerReleasesRow
	searchURL      string
	searchURLInput textinput.Model
	searchURLErr   string
	fetching       bool
	width          int
	height         int
	err            error
}

func New(d *db.DB, musicRoot string, fetchInactive FetchInactiveFunc, syncArtist SyncArtistFunc, scanFn ScanFunc) Model {
	p := paginator.New()
	p.Type = paginator.Arabic
	p.PerPage = 1
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	ti := textinput.New()
	ti.SetWidth(60)
	return Model{db: d, musicRoot: musicRoot, fetchInactive: fetchInactive, syncArtistFn: syncArtist, scanFn: scanFn, scanning: true, spinner: sp, cols: 2, paginator: p, searchURLInput: ti}
}

// Err returns any error that caused the TUI to exit.
func (m Model) Err() error {
	return m.err
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.loadArtists, m.startScan(), m.spinner.Tick)
}

func (m *Model) startScan() tea.Cmd {
	scanFn := m.scanFn
	d := m.db
	return func() tea.Msg {
		err := scanFn(context.Background(), d)
		return scanDoneMsg{err: err}
	}
}

func (m *Model) scheduleScanRefresh() tea.Cmd {
	m.scanGen++
	gen := m.scanGen
	return tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg {
		return scanRefreshMsg(gen)
	})
}

func (m Model) loadArtists() tea.Msg {
	rows, err := m.db.Q.AlbumArtists(context.Background())
	if err != nil {
		return errMsg{err}
	}
	artists := make([]artist, len(rows))
	for i, r := range rows {
		artists[i] = artist{id: r.ID, name: r.Name, followed: r.Followed == 1, inactive: r.Inactive == 1, latestRelease: r.LatestRelease, reviewedAt: r.ReviewedAt}
	}
	var nr map[int64][]db.FollowedNewerReleasesRow
	if nrRows, nrErr := m.db.Q.FollowedNewerReleases(context.Background()); nrErr == nil && len(nrRows) > 0 {
		nr = make(map[int64][]db.FollowedNewerReleasesRow)
		for _, r := range nrRows {
			nr[r.ArtistID] = append(nr[r.ArtistID], r)
		}
	}
	searchURL, _ := m.db.Q.GetSetting(context.Background(), "search_url")
	return artistsMsg{artists: artists, newReleases: nr, searchURL: searchURL}
}

type artistsMsg struct {
	artists     []artist
	newReleases map[int64][]db.FollowedNewerReleasesRow
	searchURL   string
}
type errMsg struct{ err error }

func (m *Model) syncing() bool {
	return m.syncCurrentID != 0 || len(m.syncQueue) > 0
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.modal != nil && m.modal.kind == modalSearchURL {
		if _, isKey := msg.(tea.KeyPressMsg); !isKey {
			var cmd tea.Cmd
			m.searchURLInput, cmd = m.searchURLInput.Update(msg)
			return m, cmd
		}
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updatePagination()
	case artistsMsg:
		prevLen := len(m.allArtists)
		m.allArtists = msg.artists
		m.newReleases = msg.newReleases
		if msg.searchURL != "" {
			m.searchURL = msg.searchURL
		}
		hasFollowed := false
		for _, a := range m.allArtists {
			if a.followed {
				hasFollowed = true
				break
			}
		}
		m.hideUnfollowed = hasFollowed
		m.applyFilter()
		if prevLen == 0 {
			m.cursor = 0
		}
		m.updatePagination()
		if len(m.artists) > 0 {
			m.discog = m.buildDiscographyModal(m.artists[m.globalIndex()])
		}
		var cmds []tea.Cmd
		if m.scanning {
			cmds = append(cmds, m.scheduleScanRefresh())
		}
		return m, tea.Batch(cmds...)
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
	case discogRefreshMsg:
		if int(msg) == m.discogGen && len(m.artists) > 0 {
			a := m.artists[m.globalIndex()]
			m.discog = m.buildDiscographyModal(a)
		}
	case scanRefreshMsg:
		if m.scanning && int(msg) == m.scanGen {
			return m, m.loadArtists
		}
	case scanDoneMsg:
		m.scanning = false
		return m, m.loadArtists
	case vacuumDoneMsg:
		// Vacuum runs silently; nothing to update.
	case spinner.TickMsg:
		if m.fetching || m.syncCurrentID != 0 || m.scanning {
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
		if len(m.artists) > 0 {
			m.discog = m.buildDiscographyModal(m.artists[0])
		}
	case syncArtistDoneMsg:
		if !m.syncing() {
			return m, nil
		}
		m.syncDone++
		m.syncCurrentID = 0
		m.refreshNewReleases()
		if len(m.syncQueue) > 0 {
			return m, m.syncNextArtist()
		}
		// All done.
		m.syncCancel = nil
		m.syncCtx = nil
		if len(m.artists) > 0 {
			m.discog = m.buildDiscographyModal(m.artists[m.globalIndex()])
		}
		return m, nil
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

	case modalConfirmFetch:
		if m.fetching {
			if key.Matches(msg, keys.Quit) && m.fetchCancel != nil {
				m.fetchCancel()
			}
			return m, nil
		}
		switch {
		case msg.Key().Code == tea.KeyEnter:
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

	case modalSearchURL:
		switch {
		case msg.Key().Code == tea.KeyEnter:
			val := strings.TrimSpace(m.searchURLInput.Value())
			if !strings.HasPrefix(val, "http://") && !strings.HasPrefix(val, "https://") {
				m.searchURLErr = "URL must start with http:// or https://"
			} else if !strings.Contains(val, "%s") {
				m.searchURLErr = "URL must contain %s"
			} else {
				_ = m.db.Q.SetSetting(context.Background(), "search_url", val)
				m.searchURL = val
				m.searchURLInput.Blur()
				m.searchURLErr = ""
				m.modal = nil
			}
		case key.Matches(msg, keys.Quit):
			m.searchURLInput.Blur()
			m.searchURLErr = ""
			m.modal = nil
		default:
			var cmd tea.Cmd
			m.searchURLInput, cmd = m.searchURLInput.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func openSearchURL(urlTemplate, artistName string) {
	u := fmt.Sprintf(urlTemplate, url.PathEscape(artistName))
	var c *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		c = exec.Command("open", u)
	case "windows":
		c = exec.Command("cmd", "/c", "start", "", u)
	default:
		c = exec.Command("xdg-open", u)
	}
	_ = c.Start()
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

func (m *Model) startReleaseSync() tea.Cmd {
	staleAfter := 7 * 24 * time.Hour
	var queue []int64
	for _, a := range m.allArtists {
		if !a.followed {
			continue
		}
		row, err := m.db.Q.GetArtistByID(context.Background(), a.id)
		if err != nil || row.NotFound != 0 {
			continue
		}
		if row.Mbid != "" && row.LastCheckedAt != "" {
			checked, parseErr := time.Parse(time.RFC3339, row.LastCheckedAt)
			if parseErr == nil && time.Since(checked) < staleAfter {
				continue
			}
		}
		queue = append(queue, a.id)
	}
	if len(queue) == 0 {
		m.refreshNewReleases()
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background()) //nolint:gosec // cancel stored for user-initiated *
	m.syncCtx = ctx
	m.syncCancel = cancel
	m.syncQueue = queue
	m.syncTotal = len(queue)
	m.syncDone = 0
	return tea.Batch(m.spinner.Tick, m.syncNextArtist())
}

func (m *Model) syncNextArtist() tea.Cmd {
	if len(m.syncQueue) == 0 {
		return nil
	}
	id := m.syncQueue[0]
	m.syncQueue = m.syncQueue[1:]
	m.syncCurrentID = id

	ctx := m.syncCtx
	syncFn := m.syncArtistFn
	d := m.db
	return func() tea.Msg {
		err := syncFn(ctx, d, id)
		return syncArtistDoneMsg{artistID: id, err: err}
	}
}

func (m *Model) cancelSync() {
	if m.syncCancel != nil {
		m.syncCancel()
	}
	m.syncCancel = nil
	m.syncCtx = nil
	m.syncQueue = nil
	m.syncCurrentID = 0
}

func (m *Model) refreshNewReleases() {
	nrRows, err := m.db.Q.FollowedNewerReleases(context.Background())
	if err != nil {
		return
	}
	m.newReleases = make(map[int64][]db.FollowedNewerReleasesRow)
	for _, r := range nrRows {
		m.newReleases[r.ArtistID] = append(m.newReleases[r.ArtistID], r)
	}
}

func (m *Model) updateAllArtist(id int64, fn func(*artist)) {
	for i := range m.allArtists {
		if m.allArtists[i].id == id {
			fn(&m.allArtists[i])
			return
		}
	}
}

// reviewedDate returns the date to store as reviewed_at. It uses today's date
// unless the artist has newer releases dated in the future, in which case it
// returns the latest release date so all known releases are covered.
func reviewedDate(today string, releases []db.FollowedNewerReleasesRow) string {
	best := today
	for _, r := range releases {
		if r.ReleaseDate > best {
			best = r.ReleaseDate
		}
	}
	return best
}

func filterArtists(source []artist, pred func(artist) bool) []artist {
	var out []artist
	for _, a := range source {
		if pred(a) {
			out = append(out, a)
		}
	}
	return out
}

func (m *Model) applyFilter() {
	source := m.allArtists

	if m.hideUnfollowed {
		source = filterArtists(source, func(a artist) bool { return a.followed })
	}
	if m.filterInactive {
		source = filterArtists(source, func(a artist) bool { return a.inactive })
	}
	if m.filterYears > 0 {
		cutoff := time.Now().AddDate(-m.filterYears, 0, 0).Format(dateFormat)
		source = filterArtists(source, func(a artist) bool {
			return a.latestRelease == "" || a.latestRelease < cutoff
		})
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
	var cmd tea.Cmd

	switch {
	case key.Matches(msg, keys.Quit):
		if m.syncing() {
			m.cancelSync()
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
			m.updateAllArtist(a.id, func(aa *artist) { aa.followed = a.followed })

			itemsOnPage := m.paginator.ItemsOnPage(len(m.artists))
			if m.cursor < itemsOnPage-1 {
				m.cursor++
			} else if !m.paginator.OnLastPage() {
				m.paginator.NextPage()
				m.cursor = 0
			}
		}

	case key.Matches(msg, keys.FilterFollowed):
		m.hideUnfollowed = !m.hideUnfollowed
		m.applyFilter()
		m.cursor = 0
		m.paginator.Page = 0
		m.updatePagination()

	case msg.Key().Code == tea.KeyEnter:
		if len(m.artists) > 0 && m.searchURL != "" {
			a := m.artists[m.globalIndex()]
			openSearchURL(m.searchURL, a.name)
		}

	case key.Matches(msg, keys.FilterInactive):
		if m.syncing() {
			break
		}
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
			if r == '=' {
				m.showTracks = !m.showTracks
			} else if r == '*' {
				if m.syncing() {
					m.cancelSync()
				} else if !m.fetching {
					cmd = m.startReleaseSync()
				}
			} else if r == '-' {
				d := m.db
				cmd = func() tea.Msg {
					return vacuumDoneMsg{err: d.Vacuum()}
				}
			} else if r == ',' {
				if len(m.artists) > 0 {
					gi := m.globalIndex()
					a := &m.artists[gi]
					if a.reviewedAt != "" {
						a.reviewedAt = ""
					} else {
						a.reviewedAt = reviewedDate(time.Now().Format(dateFormat), m.newReleases[a.id])
					}
					_ = m.db.Q.SetReviewedAt(context.Background(), a.reviewedAt, a.id)
					m.updateAllArtist(a.id, func(aa *artist) { aa.reviewedAt = a.reviewedAt })
				}
			} else if r == ':' {
				m.searchURLInput.SetValue(m.searchURL)
				m.searchURLInput.CursorEnd()
				m.searchURLErr = ""
				cmd = m.searchURLInput.Focus()
				m.modal = &modalData{kind: modalSearchURL, artistName: "Search URL"}
			} else if r >= '0' && r <= '9' {
				m.yearInput += string(r)
				m.yearInputGen++
				gen := m.yearInputGen
				cmd = tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg {
					return yearInputMsg(gen)
				})
			} else if m.yearInput != "" {
				// Non-digit while accumulating year input — cancel.
				m.yearInput = ""
			} else if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				m.search += strings.ToLower(string(r))
				m.searchGen++
				m.jumpToSearch()
				gen := m.searchGen
				cmd = tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg {
					return searchClearMsg(gen)
				})
			}
		}
	}

	// Update discography pane title immediately; schedule full content refresh.
	if len(m.artists) > 0 {
		a := m.artists[m.globalIndex()]
		if m.discog != nil {
			m.discog.artistName = a.name
			m.discog.followed = a.followed
		}
	}
	refreshCmd := m.scheduleDiscogRefresh()
	return m, tea.Batch(cmd, refreshCmd)
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
			line = fmt.Sprintf(" %2d. %s", r.TrackNumber, r.Title)
		} else {
			line = "     " + r.Title
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

func (m Model) rowsPerCol() int {
	avail := max(
		// blank line + help bar + padding
		m.height-3, 1)
	return avail
}

func (m *Model) scheduleDiscogRefresh() tea.Cmd {
	m.discogGen++
	gen := m.discogGen
	return tea.Tick(50*time.Millisecond, func(time.Time) tea.Msg {
		return discogRefreshMsg(gen)
	})
}

func (m Model) renderPane(width, height int) string {
	// Width() sets total width including border. Content area = width - 2 (rounded border).
	contentWidth := width - 2
	if contentWidth < 10 {
		return ""
	}

	renderAlbumLine := func(year, title string) string {
		title = datePrefixRe.ReplaceAllString(title, "")
		if year != "" {
			maxTitle := contentWidth - len(year) - 3 // 1 leading space + 1 space after year + 1 safety
			return " " + mutedStyle.Render(year) + " " + albumStyle.Render(truncate(title, maxTitle))
		}
		return " " + albumStyle.Render(truncate(title, contentWidth-1))
	}

	var lines []string
	if m.discog != nil && len(m.discog.tracks) > 0 {
		type albumInfo struct {
			name string
			year string
		}
		seen := map[string]bool{}
		var albums []albumInfo
		tracksByAlbum := map[string][]modalTrack{}
		for _, t := range m.discog.tracks {
			if !seen[t.album] {
				seen[t.album] = true
				albums = append(albums, albumInfo{t.album, t.releaseYear})
			}
			tracksByAlbum[t.album] = append(tracksByAlbum[t.album], t)
		}
		slices.SortFunc(albums, func(a, b albumInfo) int {
			return cmp.Compare(b.year, a.year)
		})
		for _, a := range albums {
			lines = append(lines, renderAlbumLine(a.year, a.name))
			if m.showTracks {
				for _, t := range tracksByAlbum[a.name] {
					lines = append(lines, truncate(t.line, contentWidth))
				}
			}
		}
	}

	if len(m.artists) > 0 {
		a := m.artists[m.globalIndex()]
		if releases, ok := m.newReleases[a.id]; ok && len(releases) > 0 {
			var newer, reviewed []db.FollowedNewerReleasesRow
			for _, r := range releases {
				if a.reviewedAt != "" && r.ReleaseDate <= a.reviewedAt {
					reviewed = append(reviewed, r)
				} else {
					newer = append(newer, r)
				}
			}
			appendReleaseSection := func(heading string, style lipgloss.Style, releases []db.FollowedNewerReleasesRow) {
				if len(releases) == 0 {
					return
				}
				if len(lines) > 0 {
					lines = append(lines, "")
				}
				lines = append(lines, " "+style.Render(heading))
				for _, r := range releases {
					year := ""
					if len(r.ReleaseDate) >= 4 {
						year = r.ReleaseDate[:4]
					}
					lines = append(lines, renderAlbumLine(year, r.AlbumTitle))
				}
			}
			appendReleaseSection("NEWER RELEASES", newReleaseStyle, newer)
			appendReleaseSection("REVIEWED", mutedStyle, reviewed)
		}
	}

	inner := strings.Join(lines, "\n")

	return paneStyle.
		Width(width).
		Height(height).
		Render(inner)
}

func (m Model) View() tea.View {
	var v tea.View
	v.AltScreen = true

	if m.err != nil {
		v.Content = fmt.Sprintf("Error: %v\n", m.err)
		return v
	}
	if len(m.artists) == 0 {
		if m.scanning {
			v.Content = m.spinner.View() + " Scanning your music files…\n"
		} else {
			v.Content = "No music files found in this directory.\n"
		}
		return v
	}

	rows := m.rowsPerCol()
	artistAreaWidth := m.width*2/3 - 12
	paneWidth := m.width - artistAreaWidth
	colWidth := max(artistAreaWidth/m.cols, 20)

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

	pane := m.renderPane(paneWidth, rows)
	body := lipgloss.JoinHorizontal(lipgloss.Top, colStrs[0], colStrs[1], pane)

	var helpText string
	switch {
	case m.syncCurrentID != 0:
		progress := fmt.Sprintf("%d/%d", m.syncDone, m.syncTotal)
		helpText = m.spinner.View() + " " + searchStyle.Render("SYNCING RELEASES") +
			helpStyle.Render(" "+progress+"  ") +
			helpKeyStyle.Render("esc") + helpStyle.Render("/") +
			helpKeyStyle.Render("*") + helpStyle.Render(" cancel  "+m.paginator.View())
	case m.yearInput != "":
		helpText = helpStyle.Render("years: ") + searchStyle.Render(m.yearInput) + helpStyle.Render("          "+m.paginator.View())
	case m.search != "":
		helpText = helpStyle.Render("search: ") + searchStyle.Render(m.search) + helpStyle.Render("          "+m.paginator.View())
	case m.filterInactive || m.filterYears > 0:
		var parts []string
		var clearHints []string
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
		var scanPrefix string
		if m.scanning {
			scanPrefix = m.spinner.View() + " " + searchStyle.Render("SCANNING") + helpStyle.Render(" | ")
		}
		slashLabel := " all"
		if !m.hideUnfollowed {
			slashLabel = " followed"
		}
		helpText = scanPrefix + helpKeyStyle.Render("space") + helpStyle.Render(" un/follow | ") +
			helpKeyStyle.Render("/") + helpStyle.Render(slashLabel+" | ") +
			helpKeyStyle.Render("=") + helpStyle.Render(" tracks | ") +
			helpKeyStyle.Render("*") + helpStyle.Render(" sync | ") +
			helpKeyStyle.Render(",") + helpStyle.Render(" caught up | ") +
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
		title := modalTitleStyle.Render(m.modal.artistName)
		switch m.modal.kind {
		case modalConfirmFetch:
			content := m.modal.content
			if m.fetching && m.fetchProgress != nil {
				content = m.spinner.View() + " " + m.fetchProgress.get()
			}
			footer := "\n" + helpKeyStyle.Render("esc") + helpStyle.Render(" cancel")
			inner = title + "\n\n" + content + footer
		case modalSearchURL:
			inputView := m.searchURLInput.View()
			hint := helpStyle.Render("%s must be included to represent the artist name")
			var errLine string
			if m.searchURLErr != "" {
				errLine = "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render(m.searchURLErr)
			}
			footer := "\n\n" + helpKeyStyle.Render("enter") + helpStyle.Render(" save  ") +
				helpKeyStyle.Render("esc") + helpStyle.Render(" cancel")
			inner = title + "\n\n" + inputView + "\n" + hint + errLine + footer
		default:
			footer := helpStyle.Render("Press any key to close")
			inner = title + "\n\n" + m.modal.content + "\n" + footer
		}

		box := modalStyle.MaxWidth(maxW).MaxHeight(maxH).Render(inner)
		content = placeOverlay(m.width, m.height, box, content)
	}

	v.Content = content
	return v
}

func (m Model) renderItem(a artist, selected bool, width int) string {
	var prefix, name string
	if m.syncCurrentID != 0 && a.id == m.syncCurrentID {
		prefix = newReleaseStyle.Render(strings.TrimRight(m.spinner.View(), " "))
	} else if a.followed {
		prefix = checkStyle.Render("✓")
	} else {
		prefix = mutedStyle.Render("·")
	}
	if a.followed {
		name = a.name
	} else {
		name = mutedStyle.Render(a.name)
	}
	if selected {
		name = selectedStyle.Render(name)
	}

	nameWidth := width - 3
	hasNewRelease := false
	if releases, ok := m.newReleases[a.id]; ok {
		for _, r := range releases {
			if a.reviewedAt == "" || r.ReleaseDate > a.reviewedAt {
				hasNewRelease = true
				break
			}
		}
	}
	if hasNewRelease {
		nameWidth--
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
