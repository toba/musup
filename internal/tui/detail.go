package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/toba/musup/internal/integration/musicbrainz"
	"github.com/toba/musup/internal/state"
)

type detailModel struct {
	db     *state.DB
	mb     *musicbrainz.Client
	artist string
	albums []string // local-only albums (before catalog fetch)
	cursor int
	height int
	offset int
	err    error

	catalogAlbums []state.AlbumRecord
	fetchErr      error
}

func newDetailModel(db *state.DB, mb *musicbrainz.Client, artist string) detailModel {
	m := detailModel{db: db, mb: mb, artist: artist}

	// Check if we already have catalog albums stored
	catalog, err := db.Albums(artist)
	if err == nil && len(catalog) > 0 {
		m.catalogAlbums = catalog
	} else {
		// Fall back to local albums
		albums, err := db.LocalAlbums(artist)
		m.albums = albums
		m.err = err
	}
	return m
}

func (m detailModel) Update(msg tea.Msg) (detailModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "backspace", "left":
			return m, func() tea.Msg { return backToListMsg{} }
		case "q", "ctrl+c":
			return m, tea.Quit
		case "enter":
			if len(m.catalogAlbums) > 0 && m.cursor >= 0 && m.cursor < len(m.catalogAlbums) {
				a := m.catalogAlbums[m.cursor]
				year := ""
				if len(a.ReleaseDate) >= 4 {
					year = a.ReleaseDate[:4]
				}
				return m, func() tea.Msg {
					return showAlbumDetailMsg{artist: m.artist, albumTitle: a.Title, year: year}
				}
			}
		case "u":
			if m.mb != nil {
				return m, func() tea.Msg { return startSyncMsg{artist: m.artist} }
			}
		case "j", "down":
			if m.cursor < m.itemCount()-1 {
				m.cursor++
				m.ensureVisible()
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
				m.ensureVisible()
			}
		}
	}
	return m, nil
}

func (m detailModel) itemCount() int {
	if len(m.catalogAlbums) > 0 {
		return len(m.catalogAlbums)
	}
	return len(m.albums)
}

func (m *detailModel) ensureVisible() {
	viewable := m.viewableLines()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+viewable {
		m.offset = m.cursor - viewable + 1
	}
}

func (m detailModel) viewableLines() int {
	// header takes 4 lines, leave 1 for footer
	v := max(m.height-5, 1)
	return v
}

func (m detailModel) View() string {
	var b strings.Builder

	header := titleStyle.Render(m.artist)
	count := m.itemCount()
	noun := "albums"
	if count == 1 {
		noun = "album"
	}
	albumCount := mutedStyle.Render(fmt.Sprintf("%d %s", count, noun))
	b.WriteString(header + "  " + albumCount + "\n")
	b.WriteString(subtleStyle.Render(strings.Repeat("─", 40)) + "\n\n")

	if m.err != nil {
		b.WriteString(errorStyle.Render(m.err.Error()))
		return b.String()
	}

	if m.fetchErr != nil {
		b.WriteString(errorStyle.Render(m.fetchErr.Error()) + "\n\n")
	}

	if len(m.catalogAlbums) > 0 {
		m.renderCatalog(&b)
	} else if len(m.albums) > 0 {
		m.renderLocalAlbums(&b)
	} else {
		b.WriteString(mutedStyle.Render("No albums found."))
	}

	b.WriteString("\n" + subtleStyle.Render("esc: back · enter: tracks · u: update catalog · q: quit"))

	return b.String()
}

func (m detailModel) renderCatalog(b *strings.Builder) {
	viewable := m.viewableLines()
	end := min(m.offset+viewable, len(m.catalogAlbums))

	// Compute max title width for visible range
	maxTitleWidth := 0
	for i := m.offset; i < end; i++ {
		if len(m.catalogAlbums[i].Title) > maxTitleWidth {
			maxTitleWidth = len(m.catalogAlbums[i].Title)
		}
	}

	for i := m.offset; i < end; i++ {
		a := m.catalogAlbums[i]
		cursor := "  "
		style := lipgloss.NewStyle()
		if i == m.cursor {
			cursor = cursorStyle.Render("> ")
			style = style.Foreground(colorAccent)
		}

		year := "    "
		if len(a.ReleaseDate) >= 4 {
			year = a.ReleaseDate[:4]
		}

		title := a.Title
		padded := title + strings.Repeat(" ", max(0, maxTitleWidth-len(title)))

		ratio := strings.Repeat(" ", 5) // empty when no track info
		if a.TotalTracks > 0 {
			ratioStr := fmt.Sprintf("%d/%d", a.LocalTracks, a.TotalTracks)
			paddedRatio := fmt.Sprintf("%5s", ratioStr)
			if a.LocalTracks == a.TotalTracks {
				ratio = localStyle.Render(paddedRatio)
			} else {
				ratio = mutedStyle.Render(paddedRatio)
			}
		}

		b.WriteString(cursor + mutedStyle.Render(year) + "  " + style.Render(padded) + "  " + ratio + "\n")
	}
}

func (m detailModel) renderLocalAlbums(b *strings.Builder) {
	viewable := m.viewableLines()
	end := min(m.offset+viewable, len(m.albums))

	for i := m.offset; i < end; i++ {
		cursor := "  "
		style := lipgloss.NewStyle()
		if i == m.cursor {
			cursor = cursorStyle.Render("> ")
			style = style.Foreground(colorAccent)
		}
		b.WriteString(cursor + style.Render(m.albums[i]) + "\n")
	}
}

type backToListMsg struct{}

type showAlbumDetailMsg struct {
	artist     string
	albumTitle string
	year       string
}
