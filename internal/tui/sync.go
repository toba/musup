package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/toba/musup/internal/db"
	"github.com/toba/musup/internal/integration/musicbrainz"
)

type syncModel struct {
	spinner spinner.Model
	artist  string
	mb      *musicbrainz.Client
	db      *db.DB
	ch      <-chan tea.Msg

	// Progress state
	phase   string
	album   string
	current int
	total   int
	steps   []string // completed step summaries

	// Result
	done   bool
	albums []db.ListAlbumsByArtistRow
	err    error
}

type syncProgressMsg struct {
	phase   string
	album   string
	current int
	total   int
}

type syncDoneMsg struct {
	albums []db.ListAlbumsByArtistRow
	err    error
}

type startSyncMsg struct {
	artist string
}

func newSyncModel(d *db.DB, mb *musicbrainz.Client, artist string) syncModel {
	return newSyncModelWithContext(context.Background(), d, mb, artist)
}

func newSyncModelWithContext(ctx context.Context, d *db.DB, mb *musicbrainz.Client, artist string) syncModel {
	ch := make(chan tea.Msg, 1)

	m := syncModel{
		spinner: newSpinner(),
		artist:  artist,
		mb:      mb,
		db:      d,
		ch:      ch,
		phase:   "Connecting to MusicBrainz...",
	}

	go runSync(ctx, ch, mb, d, artist)

	return m
}

func (m syncModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, listenForSync(m.ch))
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
	modal := modalStyle(44)

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

func hasComposerTag(artist musicbrainz.Artist) bool {
	for _, tag := range artist.Tags {
		if tag.Name == "composer" {
			return true
		}
	}
	return false
}

func runSync(ctx context.Context, ch chan<- tea.Msg, mb *musicbrainz.Client, d *db.DB, artist string) {
	defer close(ch)

	// Step 1: Search for artist
	if ctx.Err() != nil {
		ch <- syncDoneMsg{err: ctx.Err()}
		return
	}
	ch <- syncProgressMsg{phase: "Searching MusicBrainz..."}

	result, err := mb.SearchArtists(ctx, artist, mbSearchLimit, 0)
	if err != nil {
		ch <- syncDoneMsg{err: fmt.Errorf("search artist: %w", err)}
		return
	}

	if len(result.Artists) == 0 || result.Artists[0].Score < mbMinMatchScore {
		_ = d.MarkArtistNotFound(artist)
		ch <- syncDoneMsg{err: errors.New("artist not found on MusicBrainz")}
		return
	}

	mbArtist := result.Artists[0]

	// Resolve artist → ID
	artistID, err := d.EnsureArtist(artist)
	if err != nil {
		ch <- syncDoneMsg{err: fmt.Errorf("ensure artist: %w", err)}
		return
	}

	// Step 2: Fetch release groups
	if ctx.Err() != nil {
		ch <- syncDoneMsg{err: ctx.Err()}
		return
	}
	ch <- syncProgressMsg{phase: "Fetching albums..."}

	rgCap := mbMaxReleaseGroups
	if hasComposerTag(mbArtist) {
		rgCap = mbMaxReleaseGroupsComposer
	}

	rgResult, err := mb.AllReleaseGroups(ctx, mbArtist.ID, "album", rgCap)
	if err != nil {
		ch <- syncDoneMsg{err: fmt.Errorf("fetch release groups: %w", err)}
		return
	}

	rgs := rgResult.ReleaseGroups
	capped := rgResult.Capped

	if capped {
		ch <- syncProgressMsg{
			phase: fmt.Sprintf("Too many albums (%d) — stored first %d, skipping tracks", rgResult.TotalCount, len(rgs)),
		}
	} else {
		ch <- syncProgressMsg{
			phase: fmt.Sprintf("Found %d albums", len(rgs)),
		}
	}

	// Build set of albums we already have tracks for so we can skip
	// fetching them from MusicBrainz again.
	knownList, err := d.Q.ListKnownAlbumMBIDs(ctx, db.Normalize(artist))
	if err != nil {
		ch <- syncDoneMsg{err: fmt.Errorf("known albums: %w", err)}
		return
	}
	knownMBIDs := make(map[string]struct{}, len(knownList))
	for _, mbid := range knownList {
		knownMBIDs[mbid] = struct{}{}
	}

	// Step 3: For each release group, upsert album and fetch tracks
	fetched := 0
	for _, rg := range rgs {
		// Always upsert the album metadata (cheap, keeps it fresh).
		albumID, err := d.UpsertAlbum(artistID, db.UpsertAlbumParams{
			ArtistID:       artistID,
			Title:          rg.Title,
			TitleNorm:      db.Normalize(rg.Title),
			Mbid:           rg.ID,
			ReleaseDate:    rg.FirstReleaseDate,
			PrimaryType:    rg.PrimaryType,
			SecondaryTypes: strings.Join(rg.SecondaryTypes, ","),
		})
		if err != nil {
			ch <- syncDoneMsg{err: fmt.Errorf("upsert album: %w", err)}
			return
		}

		// When capped, skip all track fetching — we only store album metadata.
		if capped {
			continue
		}

		// Skip the expensive BrowseReleases call if we already have tracks.
		if _, known := knownMBIDs[rg.ID]; known {
			continue
		}

		if ctx.Err() != nil {
			ch <- syncDoneMsg{err: ctx.Err()}
			return
		}

		fetched++
		ch <- syncProgressMsg{
			phase:   "Fetching tracks",
			album:   rg.Title,
			current: fetched,
			total:   len(rgs) - len(knownMBIDs),
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
					if err := d.Q.UpsertTrack(ctx, db.UpsertTrackParams{
						AlbumID:   albumID,
						Title:     track.Recording.Title,
						TitleNorm: db.Normalize(track.Recording.Title),
						Position:  int64(track.Position),
						Mbid:      track.Recording.ID,
						LengthMs:  int64(track.Recording.Length),
						Local:     0,
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

	if err := d.MarkLocalTracks(artist); err != nil {
		ch <- syncDoneMsg{err: fmt.Errorf("mark local tracks: %w", err)}
		return
	}

	// Update artist record
	if err := d.Q.UpdateArtistMeta(ctx, mbArtist.ID, time.Now().Format(time.RFC3339), artistID); err != nil {
		ch <- syncDoneMsg{err: fmt.Errorf("update artist: %w", err)}
		return
	}

	// Read back the full catalog
	albums, err := d.Q.ListAlbumsByArtist(ctx, db.Normalize(artist))
	if err != nil {
		ch <- syncDoneMsg{err: fmt.Errorf("read catalog: %w", err)}
		return
	}

	ch <- syncDoneMsg{albums: albums}
}
