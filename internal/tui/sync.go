package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/toba/musup/internal/integration/musicbrainz"
	"github.com/toba/musup/internal/state"
)

type syncModel struct {
	spinner spinner.Model
	artist  string
	mb      *musicbrainz.Client
	db      *state.DB
	ch      <-chan tea.Msg

	// Progress state
	phase   string
	album   string
	current int
	total   int
	steps   []string // completed step summaries

	// Result
	done   bool
	albums []state.AlbumRecord
	err    error
}

type syncProgressMsg struct {
	phase   string
	album   string
	current int
	total   int
}

type syncDoneMsg struct {
	albums []state.AlbumRecord
	err    error
}

type startSyncMsg struct {
	artist string
}

func newSyncModel(db *state.DB, mb *musicbrainz.Client, artist string) syncModel {
	s := spinner.New()
	s.Spinner = spinner.Spinner{
		Frames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		FPS:    80 * time.Millisecond,
	}
	s.Style = lipgloss.NewStyle().Foreground(colorAccent)

	ch := make(chan tea.Msg, 1)

	m := syncModel{
		spinner: s,
		artist:  artist,
		mb:      mb,
		db:      db,
		ch:      ch,
		phase:   "Connecting to MusicBrainz...",
	}

	go runSync(ch, mb, db, artist)

	return m
}

func (m syncModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, listenForSync(m.ch))
}

func listenForSync(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

func (m syncModel) Update(msg tea.Msg) (syncModel, tea.Cmd) {
	switch msg := msg.(type) {
	case syncProgressMsg:
		// If moving to a new phase, record the old one as completed
		if msg.phase != m.phase && m.phase != "" {
			summary := m.phase
			if m.total > 0 {
				summary = fmt.Sprintf("%s (%d)", m.phase, m.total)
			}
			m.steps = append(m.steps, summary)
		}
		m.phase = msg.phase
		m.album = msg.album
		m.current = msg.current
		m.total = msg.total
		return m, listenForSync(m.ch)

	case syncDoneMsg:
		m.done = true
		m.albums = msg.albums
		m.err = msg.err
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m syncModel) View(width, height int, bg string) string {
	modal := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorAccent).
		Padding(1, 2).
		Width(44)

	var b strings.Builder

	b.WriteString(titleStyle.Render("Syncing "+m.artist) + "\n\n")

	// Completed steps
	for _, step := range m.steps {
		b.WriteString(localStyle.Render("  \u2713 ") + mutedStyle.Render(step) + "\n")
	}

	// Current phase
	if m.err != nil {
		b.WriteString(errorStyle.Render("  \u2717 "+m.err.Error()) + "\n")
	} else if !m.done {
		counter := ""
		if m.total > 0 {
			counter = mutedStyle.Render(fmt.Sprintf(" (%d/%d)", m.current, m.total))
		}
		b.WriteString("  " + m.spinner.View() + " " + m.phase + counter + "\n")
		if m.album != "" {
			b.WriteString("    " + subtleStyle.Render(m.album) + "\n")
		}
	}

	rendered := modal.Render(b.String())
	return placeOverlay(width, height, rendered, bg)
}

func runSync(ch chan<- tea.Msg, mb *musicbrainz.Client, db *state.DB, artist string) {
	defer close(ch)
	ctx := context.Background()

	// Step 1: Search for artist
	ch <- syncProgressMsg{phase: "Searching MusicBrainz..."}

	result, err := mb.SearchArtists(ctx, artist, 5, 0)
	if err != nil {
		ch <- syncDoneMsg{err: fmt.Errorf("search artist: %w", err)}
		return
	}

	if len(result.Artists) == 0 || result.Artists[0].Score < 90 {
		_ = db.MarkArtistNotFound(artist)
		ch <- syncDoneMsg{err: errors.New("artist not found on MusicBrainz")}
		return
	}

	mbArtist := result.Artists[0]

	// Step 2: Fetch release groups
	ch <- syncProgressMsg{phase: "Fetching albums..."}

	rgs, err := mb.AllReleaseGroups(ctx, mbArtist.ID, "album")
	if err != nil {
		ch <- syncDoneMsg{err: fmt.Errorf("fetch release groups: %w", err)}
		return
	}

	ch <- syncProgressMsg{
		phase: fmt.Sprintf("Found %d albums", len(rgs)),
	}

	// Step 3: For each release group, upsert album and fetch tracks
	for i, rg := range rgs {
		ch <- syncProgressMsg{
			phase:   "Fetching tracks",
			album:   rg.Title,
			current: i + 1,
			total:   len(rgs),
		}

		if err := db.UpsertAlbum(state.AlbumRecord{
			ArtistName:  artist,
			Title:       rg.Title,
			MBID:        rg.ID,
			ReleaseDate: rg.FirstReleaseDate,
			PrimaryType: rg.PrimaryType,
		}); err != nil {
			ch <- syncDoneMsg{err: fmt.Errorf("upsert album: %w", err)}
			return
		}

		relResult, err := mb.BrowseReleases(ctx, rg.ID, "recordings", 1, 0)
		if err != nil {
			ch <- syncDoneMsg{err: fmt.Errorf("browse releases: %w", err)}
			return
		}
		if len(relResult.Releases) > 0 {
			rel := relResult.Releases[0]
			for _, medium := range rel.Media {
				for _, track := range medium.Tracks {
					if err := db.UpsertTrack(state.TrackRecord{
						ArtistName: artist,
						AlbumTitle: rg.Title,
						Title:      track.Title,
						Position:   track.Position,
						MBID:       track.Recording.ID,
						LengthMS:   track.Recording.Length,
					}); err != nil {
						ch <- syncDoneMsg{err: fmt.Errorf("upsert track: %w", err)}
						return
					}
				}
			}
		}
	}

	// Step 4: Match local tracks
	ch <- syncProgressMsg{phase: "Matching local library..."}

	if err := db.MarkLocalTracks(artist); err != nil {
		ch <- syncDoneMsg{err: fmt.Errorf("mark local tracks: %w", err)}
		return
	}

	// Update artist record
	if err := db.UpsertArtist(state.ArtistRecord{
		Name: artist,
		MBID: mbArtist.ID,
	}); err != nil {
		ch <- syncDoneMsg{err: fmt.Errorf("upsert artist: %w", err)}
		return
	}

	// Read back the full catalog
	albums, err := db.Albums(artist)
	if err != nil {
		ch <- syncDoneMsg{err: fmt.Errorf("read catalog: %w", err)}
		return
	}

	ch <- syncDoneMsg{albums: albums}
}
