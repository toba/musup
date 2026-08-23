package tui

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/toba/musup-go/internal/db"

	_ "modernc.org/sqlite"
)

// syncTestDB builds a database with the given followed artists. Each artist
// gets one album-artist file row so AlbumArtists returns it.
func syncTestDB(t *testing.T, names ...string) *db.DB {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })

	for _, name := range names {
		id, err := d.EnsureArtist(name)
		if err != nil {
			t.Fatalf("ensure artist %q: %v", name, err)
		}
		if err := d.Q.SetFollowed(context.Background(), 1, id); err != nil {
			t.Fatalf("set followed: %v", err)
		}
		if err := d.Q.UpsertFile(context.Background(), db.UpsertFileParams{
			Path:          "/music/" + name + "/01.flac",
			Size:          1,
			ModTime:       "2026-01-01T00:00:00Z",
			Artist:        name,
			ArtistNorm:    db.Normalize(name),
			ArtistID:      id,
			Album:         "Album",
			AlbumNorm:     "album",
			Title:         "Track",
			TitleNorm:     "track",
			TrackNumber:   1,
			IsAlbumArtist: 1,
			ScannedAt:     "2026-01-01T00:00:00Z",
		}); err != nil {
			t.Fatalf("upsert file: %v", err)
		}
	}
	return d
}

// harness runs a model through a minimal Elm loop. It mirrors what Bubble Tea
// does: it runs each command in a goroutine and feeds the result back.
type harness struct {
	t     *testing.T
	model tea.Model
	msgs  chan tea.Msg
	done  chan struct{}
	once  sync.Once
}

func newHarness(t *testing.T, m tea.Model) *harness {
	t.Helper()
	h := &harness{t: t, model: m, msgs: make(chan tea.Msg, 64), done: make(chan struct{})}
	t.Cleanup(func() { h.once.Do(func() { close(h.done) }) })
	return h
}

func (h *harness) run(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	go func() {
		msg := cmd()
		select {
		case h.msgs <- msg:
		case <-h.done:
		}
	}()
}

// send queues a message for the loop.
func (h *harness) send(msg tea.Msg) {
	h.msgs <- msg
}

// pump processes messages until no message other than a spinner tick arrives
// for the given duration. Spinner ticks repeat forever, so they must not hold
// the loop open.
func (h *harness) pump(idle time.Duration) {
	deadline := time.Now().Add(idle)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return
		}
		select {
		case msg := <-h.msgs:
			if batch, ok := msg.(tea.BatchMsg); ok {
				for _, c := range batch {
					h.run(c)
				}
				continue
			}
			if _, isTick := msg.(spinner.TickMsg); !isTick {
				deadline = time.Now().Add(idle)
			}
			var cmd tea.Cmd
			h.model, cmd = h.model.Update(msg)
			h.run(cmd)
		case <-time.After(remaining):
			return
		}
	}
}

// TestReleaseSyncSurvivesModal proves that opening a modal during a release
// sync must not stop the sync. Both modals with a text input take their own
// branch in updateModalInputs. A branch that returns instead of falling
// through swallows syncArtistDoneMsg. Losing that message breaks the command
// chain: no command remains in flight, yet the model still reports a sync in
// progress.
func TestReleaseSyncSurvivesModal(t *testing.T) {
	tests := []struct {
		name string
		key  rune
	}{
		{"search URL modal", ':'},
		{"7digital download modal", '^'},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			names := []string{"Artist A", "Artist B", "Artist C", "Artist D"}
			d := syncTestDB(t, names...)

			var mu sync.Mutex
			var synced []int64
			// release gates each sync call so the test controls when the done
			// message reaches Update.
			release := make(chan struct{})
			syncArtist := func(_ context.Context, _ *db.DB, artistID int64) error {
				mu.Lock()
				synced = append(synced, artistID)
				mu.Unlock()
				<-release
				return nil
			}
			noScan := func(context.Context, *db.DB) error { return nil }
			noFetch := func(context.Context, *db.DB, func(string)) (map[int64]bool, error) {
				return nil, nil
			}

			m := New(d, t.TempDir(), noFetch, syncArtist, noScan)
			h := newHarness(t, m)

			h.send(tea.WindowSizeMsg{Width: 200, Height: 50})
			h.run(m.loadArtists)
			h.pump(200 * time.Millisecond)

			// Start the release sync. The first artist blocks inside syncArtist.
			h.send(tea.KeyPressMsg{Code: '*', Text: "*"})
			h.pump(100 * time.Millisecond)

			// Open the modal while the first artist is still in flight.
			h.send(tea.KeyPressMsg{Code: tt.key, Text: string(tt.key)})
			h.pump(100 * time.Millisecond)

			// Let the first artist finish. Its done message arrives while the
			// modal is open.
			close(release)
			h.pump(200 * time.Millisecond)

			// Close the modal and give the sync time to drain.
			h.send(tea.KeyPressMsg{Code: tea.KeyEscape})
			h.pump(500 * time.Millisecond)

			mu.Lock()
			got := len(synced)
			mu.Unlock()

			model, ok := h.model.(Model)
			if !ok {
				t.Fatalf("model has type %T, want Model", h.model)
			}
			if got != len(names) {
				t.Errorf("synced %d artists, want %d; syncCurrentID=%d queue=%d",
					got, len(names), model.syncCurrentID, len(model.syncQueue))
			}
			if model.syncing() {
				t.Errorf("model still reports a sync in progress: syncCurrentID=%d queue=%d",
					model.syncCurrentID, len(model.syncQueue))
			}
		})
	}
}
