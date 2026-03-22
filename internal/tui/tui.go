package tui

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/paginator"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/toba/musup/internal/db"
)

type keyMap struct {
	Up       key.Binding
	Down     key.Binding
	Left     key.Binding
	Right    key.Binding
	Toggle   key.Binding
	NextPage key.Binding
	PrevPage key.Binding
	Detail   key.Binding
	Help     key.Binding
	Quit     key.Binding
}

var keys = keyMap{
	Up:       key.NewBinding(key.WithKeys("up")),
	Down:     key.NewBinding(key.WithKeys("down")),
	Left:     key.NewBinding(key.WithKeys("left")),
	Right:    key.NewBinding(key.WithKeys("right")),
	Toggle:   key.NewBinding(key.WithKeys("space")),
	NextPage: key.NewBinding(key.WithKeys("pgdown")),
	PrevPage: key.NewBinding(key.WithKeys("pgup")),
	Detail:   key.NewBinding(key.WithKeys("enter")),
	Help:     key.NewBinding(key.WithKeys("?")),
	Quit:     key.NewBinding(key.WithKeys("esc", "ctrl+c")),
}

func buildHelpContent() string {
	entries := [][2]string{
		{"↑ ↓", "Move up/down"},
		{"← →", "Move left/right"},
		{"space", "Toggle follow"},
		{"enter", "Show albums/tracks"},
		{"pgdn/pgup", "Next/previous page"},
		{"a-z", "Jump to artist"},
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
		pad := colWidth - lipgloss.Width(e[0])
		if pad < 1 {
			pad = 1
		}
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
	albumStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("33"))
)

type artist struct {
	id       int64
	name     string
	followed bool
}

type modalTrack struct {
	path  string
	album string
	line  string // e.g. "  3. Title"
}

type modalData struct {
	artistName string
	content    string       // pre-rendered for help modal
	tracks     []modalTrack // nil for help modal
	cursor     int
}

func (md *modalData) interactive() bool {
	return md.tracks != nil
}

type searchClearMsg int

type Model struct {
	artists   []artist
	cursor    int // page-relative index
	cols      int
	db        *db.DB
	musicRoot string
	paginator paginator.Model
	modal     *modalData
	search    string
	searchGen int
	width     int
	height    int
	err       error
}

func New(d *db.DB, musicRoot string) Model {
	p := paginator.New()
	p.Type = paginator.Arabic
	p.PerPage = 1
	return Model{db: d, musicRoot: musicRoot, cols: 3, paginator: p}
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
		artists[i] = artist{id: r.ID, name: r.Name, followed: r.Followed == 1}
	}
	return artistsMsg(artists)
}

type artistsMsg []artist
type errMsg struct{ err error }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updatePagination()
	case artistsMsg:
		m.artists = []artist(msg)
		m.cursor = 0
		m.updatePagination()
	case errMsg:
		m.err = msg.err
		return m, tea.Quit
	case searchClearMsg:
		if int(msg) == m.searchGen {
			m.search = ""
		}
	case tea.KeyPressMsg:
		if m.modal != nil {
			return m.handleModalKey(msg)
		}
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleModalKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if !m.modal.interactive() {
		// Help modal: any key dismisses.
		m.modal = nil
		return m, nil
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
	case key.Matches(msg, keys.Quit):
		m.modal = nil
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

func (m *Model) updatePagination() {
	perPage := m.rowsPerCol() * m.cols
	if perPage < 1 {
		perPage = 1
	}
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
		return m, tea.Quit

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
			m.modal = m.buildDiscographyModal(a)
		}

	case key.Matches(msg, keys.Help):
		m.modal = &modalData{artistName: "Keyboard Shortcuts", content: buildHelpContent()}

	default:
		k := msg.Key()
		if len(k.Text) == 1 {
			r := rune(k.Text[0])
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

type albumBlock struct {
	name   string
	tracks []modalTrack
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
		allTracks = append(allTracks, modalTrack{path: r.Path, album: r.Album, line: line})
	}

	return &modalData{
		artistName: a.name,
		tracks:     allTracks,
		cursor:     0,
	}
}

func (m Model) renderDiscographyContent(md *modalData) string {
	// Group tracks by album.
	var blocks []albumBlock
	var current albumBlock
	for _, t := range md.tracks {
		if t.album != current.name {
			if len(current.tracks) > 0 {
				blocks = append(blocks, current)
			}
			current = albumBlock{name: t.album}
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
		lines = append(lines, albumStyle.Render(b.name))
		for _, t := range b.tracks {
			line := t.line
			if trackIdx == md.cursor {
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

	leftCol := strings.Join(leftLines, "\n")
	if len(rightLines) == 0 {
		return leftCol
	}
	rightCol := strings.Join(rightLines, "\n")
	return lipgloss.JoinHorizontal(lipgloss.Top, leftCol, "    ", rightCol)
}

func (m Model) rowsPerCol() int {
	avail := m.height - 3 // blank line + help bar + padding
	if avail < 1 {
		avail = 1
	}
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
	colWidth := m.width / m.cols
	if colWidth < 20 {
		colWidth = 20
	}

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
	if m.search != "" {
		helpText = helpStyle.Render("search: ") + searchStyle.Render(m.search) + helpStyle.Render("          "+m.paginator.View())
	} else {
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
		title := modalTitleStyle.Render(m.modal.artistName)
		if m.modal.interactive() {
			body := m.renderDiscographyContent(m.modal)
			footer := helpKeyStyle.Render("↑↓") + helpStyle.Render(" select | ") +
				helpKeyStyle.Render("enter") + helpStyle.Render(" open | ") +
				helpKeyStyle.Render("esc") + helpStyle.Render(" close")
			inner = title + "\n\n" + body + "\n" + footer
		} else {
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
	if a.followed {
		prefix = checkStyle.Render("✓")
		name = a.name
	} else {
		prefix = mutedStyle.Render("·")
		name = mutedStyle.Render(a.name)
	}

	nameWidth := width - 3
	line := prefix + " " + truncate(name, nameWidth)

	if selected {
		plain := renderPlain(a, width)
		line = selectedStyle.Render(plain)
	}

	return padToWidth(line, width)
}

func renderPlain(a artist, width int) string {
	prefix := "✓"
	if !a.followed {
		prefix = "·"
	}
	nameWidth := width - 3
	return prefix + " " + truncate(a.name, nameWidth)
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
