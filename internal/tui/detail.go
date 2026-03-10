package tui

import (
	"context"
	"errors"
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

	fetching      bool
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

type catalogFetchedMsg struct {
	albums []state.AlbumRecord
	err    error
}

func (m detailModel) fetchCatalog() tea.Msg {
	ctx := context.Background()

	// Search for artist MBID
	result, err := m.mb.SearchArtists(ctx, m.artist, 5, 0)
	if err != nil {
		return catalogFetchedMsg{err: fmt.Errorf("search artist: %w", err)}
	}

	if len(result.Artists) == 0 || result.Artists[0].Score < 90 {
		_ = m.db.MarkArtistNotFound(m.artist)
		return catalogFetchedMsg{err: errors.New("artist not found on MusicBrainz")}
	}

	mbArtist := result.Artists[0]

	// Fetch all album release groups
	rgs, err := m.mb.AllReleaseGroups(ctx, mbArtist.ID, "album")
	if err != nil {
		return catalogFetchedMsg{err: fmt.Errorf("fetch release groups: %w", err)}
	}

	// Upsert each release group as an album, then fetch tracks
	for _, rg := range rgs {
		if err := m.db.UpsertAlbum(state.AlbumRecord{
			ArtistName:  m.artist,
			Title:       rg.Title,
			MBID:        rg.ID,
			ReleaseDate: rg.FirstReleaseDate,
			PrimaryType: rg.PrimaryType,
		}); err != nil {
			return catalogFetchedMsg{err: fmt.Errorf("upsert album: %w", err)}
		}

		// Fetch first release to get track listing
		relResult, err := m.mb.BrowseReleases(ctx, rg.ID, "recordings", 1, 0)
		if err != nil {
			return catalogFetchedMsg{err: fmt.Errorf("browse releases: %w", err)}
		}
		if len(relResult.Releases) > 0 {
			rel := relResult.Releases[0]
			for _, medium := range rel.Media {
				for _, track := range medium.Tracks {
					if err := m.db.UpsertTrack(state.TrackRecord{
						ArtistName: m.artist,
						AlbumTitle: rg.Title,
						Title:      track.Title,
						Position:   track.Position,
						MBID:       track.Recording.ID,
						LengthMS:   track.Recording.Length,
					}); err != nil {
						return catalogFetchedMsg{err: fmt.Errorf("upsert track: %w", err)}
					}
				}
			}
		}
	}

	// Mark which tracks are in the local library
	if err := m.db.MarkLocalTracks(m.artist); err != nil {
		return catalogFetchedMsg{err: fmt.Errorf("mark local tracks: %w", err)}
	}

	// Update artist record
	if err := m.db.UpsertArtist(state.ArtistRecord{
		Name: m.artist,
		MBID: mbArtist.ID,
	}); err != nil {
		return catalogFetchedMsg{err: fmt.Errorf("upsert artist: %w", err)}
	}

	// Read back the full catalog
	albums, err := m.db.Albums(m.artist)
	if err != nil {
		return catalogFetchedMsg{err: fmt.Errorf("read catalog: %w", err)}
	}

	return catalogFetchedMsg{albums: albums}
}

func (m detailModel) Update(msg tea.Msg) (detailModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
	case catalogFetchedMsg:
		m.fetching = false
		if msg.err != nil {
			m.fetchErr = msg.err
		} else {
			m.catalogAlbums = msg.albums
			m.fetchErr = nil
			m.cursor = 0
			m.offset = 0
		}
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "backspace", "left":
			return m, func() tea.Msg { return backToListMsg{} }
		case "q", "ctrl+c":
			return m, tea.Quit
		case "u":
			if !m.fetching && m.mb != nil {
				m.fetching = true
				m.fetchErr = nil
				return m, m.fetchCatalog
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

	if m.fetching {
		b.WriteString(mutedStyle.Render("Fetching catalog..."))
		b.WriteString("\n\n" + subtleStyle.Render("esc: back · q: quit"))
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

	b.WriteString("\n" + subtleStyle.Render("esc: back · u: update catalog · q: quit"))

	return b.String()
}

func (m detailModel) renderCatalog(b *strings.Builder) {
	viewable := m.viewableLines()
	end := min(m.offset+viewable, len(m.catalogAlbums))

	for i := m.offset; i < end; i++ {
		a := m.catalogAlbums[i]
		cursor := "  "
		style := lipgloss.NewStyle()
		if i == m.cursor {
			cursor = cursorStyle.Render("> ")
			style = style.Foreground(colorAccent)
		}

		marker := "  "
		if a.TotalTracks > 0 && a.LocalTracks == a.TotalTracks {
			marker = localStyle.Render("\u2713 ")
		}

		year := ""
		if len(a.ReleaseDate) >= 4 {
			year = " (" + a.ReleaseDate[:4] + ")"
		}

		ratio := ""
		if a.TotalTracks > 0 {
			ratio = mutedStyle.Render(fmt.Sprintf("  [%d/%d]", a.LocalTracks, a.TotalTracks))
		}

		b.WriteString(cursor + marker + style.Render(a.Title+year) + ratio + "\n")
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
