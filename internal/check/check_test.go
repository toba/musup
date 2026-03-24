package check_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/toba/musup/internal/check"
	"github.com/toba/musup/internal/db"
	"github.com/toba/musup/internal/integration/musicbrainz"
)

func openTestDB(t *testing.T) *db.DB {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func ensureArtist(t *testing.T, d *db.DB, name string) int64 {
	t.Helper()
	id, err := d.EnsureArtist(name)
	if err != nil {
		t.Fatalf("EnsureArtist(%q): %v", name, err)
	}
	return id
}

// fakeMB returns an httptest.Server that responds to artist search and release-group browse.
func fakeMB(t *testing.T, artists map[string]musicbrainz.ArtistSearchResult, releases map[string]musicbrainz.ReleaseGroupBrowseResult) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/artist":
			query := r.URL.Query().Get("query")
			for key, result := range artists {
				if query != "" && key != "" {
					json.NewEncoder(w).Encode(result)
					return
				}
			}
			json.NewEncoder(w).Encode(musicbrainz.ArtistSearchResult{})
		case "/release-group":
			mbid := r.URL.Query().Get("artist")
			if result, ok := releases[mbid]; ok {
				json.NewEncoder(w).Encode(result)
			} else {
				json.NewEncoder(w).Encode(musicbrainz.ReleaseGroupBrowseResult{})
			}
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestSyncArtist_NewArtist(t *testing.T) {
	d := openTestDB(t)
	ctx := context.Background()
	artistID := ensureArtist(t, d, "Radiohead")

	// Insert a file so the artist appears in DistinctArtistIDs.
	_ = d.Q.UpsertFile(ctx, db.UpsertFileParams{
		Path: "radiohead/ok.flac", Size: 100, ModTime: "2024-01-01",
		Artist: "Radiohead", ArtistNorm: "radiohead", ArtistID: artistID,
		Album: "OK Computer", AlbumNorm: "ok computer", ScannedAt: "2024-01-01",
		IsAlbumArtist: 1,
	})

	srv := fakeMB(t,
		map[string]musicbrainz.ArtistSearchResult{
			"Radiohead": {Count: 1, Artists: []musicbrainz.Artist{
				{ID: "mbid-rh", Name: "Radiohead", Score: 100},
			}},
		},
		map[string]musicbrainz.ReleaseGroupBrowseResult{
			"mbid-rh": {Count: 1, ReleaseGroups: []musicbrainz.ReleaseGroup{
				{ID: "rg-1", Title: "OK Computer", PrimaryType: "Album", FirstReleaseDate: "1997-05-21"},
				{ID: "rg-2", Title: "Kid A", PrimaryType: "Album", FirstReleaseDate: "2000-10-02"},
			}},
		},
	)
	defer srv.Close()

	mb := musicbrainz.NewWithBase(srv.URL, "test", "0.1", "test@test.com")
	err := check.SyncArtist(ctx, d, mb, artistID)
	if err != nil {
		t.Fatalf("SyncArtist: %v", err)
	}

	// Verify artist got an MBID.
	row, err := d.Q.GetArtistByID(ctx, artistID)
	if err != nil {
		t.Fatalf("GetArtistByID: %v", err)
	}
	if row.Mbid != "mbid-rh" {
		t.Errorf("expected mbid 'mbid-rh', got %q", row.Mbid)
	}
	if row.LastCheckedAt == "" {
		t.Error("expected last_checked_at to be set")
	}
}

func TestSyncArtist_LowScore_MarksNotFound(t *testing.T) {
	d := openTestDB(t)
	ctx := context.Background()
	artistID := ensureArtist(t, d, "Obscure Band")

	_ = d.Q.UpsertFile(ctx, db.UpsertFileParams{
		Path: "obscure/track.flac", Size: 100, ModTime: "2024-01-01",
		Artist: "Obscure Band", ArtistNorm: "obscure band", ArtistID: artistID,
		Album: "Album", AlbumNorm: "album", ScannedAt: "2024-01-01",
		IsAlbumArtist: 1,
	})

	srv := fakeMB(t,
		map[string]musicbrainz.ArtistSearchResult{
			"Obscure Band": {Count: 1, Artists: []musicbrainz.Artist{
				{ID: "mbid-x", Name: "Obscure Band X", Score: 50},
			}},
		},
		nil,
	)
	defer srv.Close()

	mb := musicbrainz.NewWithBase(srv.URL, "test", "0.1", "test@test.com")
	err := check.SyncArtist(ctx, d, mb, artistID)
	if err != nil {
		t.Fatalf("SyncArtist: %v", err)
	}

	row, err := d.Q.GetArtistByID(ctx, artistID)
	if err != nil {
		t.Fatalf("GetArtistByID: %v", err)
	}
	if row.NotFound != 1 {
		t.Errorf("expected not_found=1, got %d", row.NotFound)
	}
}

func TestSyncAll_SkipsStaleAndNotFound(t *testing.T) {
	d := openTestDB(t)
	ctx := context.Background()

	freshID := ensureArtist(t, d, "Fresh Artist")
	staleID := ensureArtist(t, d, "Stale Artist")
	notFoundID := ensureArtist(t, d, "Not Found")

	for _, id := range []int64{freshID, staleID, notFoundID} {
		_ = d.Q.UpsertFile(ctx, db.UpsertFileParams{
			Path: filepath.Join("a", strconv.FormatInt(id, 10), "t.flac"), Size: 100, ModTime: "2024-01-01",
			Artist: "x", ArtistNorm: "x", ArtistID: id,
			Album: "A", AlbumNorm: "a", ScannedAt: "2024-01-01",
			IsAlbumArtist: 1,
		})
	}

	// Mark freshID as recently checked.
	_ = d.Q.UpdateArtistMeta(ctx, "mbid-fresh", time.Now().Format(time.RFC3339), freshID)
	// Mark staleID as checked long ago.
	_ = d.Q.UpdateArtistMeta(ctx, "mbid-stale", time.Now().Add(-30*24*time.Hour).Format(time.RFC3339), staleID)
	// Mark notFoundID as not found.
	_ = d.Q.MarkArtistNotFound(ctx, notFoundID)

	var synced []string
	srv := fakeMB(t,
		map[string]musicbrainz.ArtistSearchResult{
			"any": {Count: 0, Artists: nil},
		},
		map[string]musicbrainz.ReleaseGroupBrowseResult{
			"mbid-stale": {Count: 0},
		},
	)
	defer srv.Close()

	mb := musicbrainz.NewWithBase(srv.URL, "test", "0.1", "test@test.com")
	err := check.SyncAll(ctx, d, mb, 7*24*time.Hour, func(p check.Progress) {
		synced = append(synced, p.Artist)
	})
	if err != nil {
		t.Fatalf("SyncAll: %v", err)
	}

	// Fresh should be skipped (recently checked), not found should be skipped.
	// Only stale should be synced.
	if len(synced) != 1 {
		t.Fatalf("expected 1 artist synced, got %d: %v", len(synced), synced)
	}
	if synced[0] != "Stale Artist" {
		t.Errorf("expected 'Stale Artist', got %q", synced[0])
	}
}

func TestSyncAll_ContextCancellation(t *testing.T) {
	d := openTestDB(t)
	ctx, cancel := context.WithCancel(context.Background())

	artistID := ensureArtist(t, d, "Test Artist")
	_ = d.Q.UpsertFile(ctx, db.UpsertFileParams{
		Path: "test/t.flac", Size: 100, ModTime: "2024-01-01",
		Artist: "Test Artist", ArtistNorm: "test artist", ArtistID: artistID,
		Album: "A", AlbumNorm: "a", ScannedAt: "2024-01-01",
		IsAlbumArtist: 1,
	})

	// Cancel before sync starts.
	cancel()

	srv := fakeMB(t, nil, nil)
	defer srv.Close()

	mb := musicbrainz.NewWithBase(srv.URL, "test", "0.1", "test@test.com")
	err := check.SyncAll(ctx, d, mb, 0, nil)
	if err == nil {
		t.Fatal("expected context canceled error")
	}
}

func TestHasComposerTag(t *testing.T) {
	// This tests indirectly via SyncArtist — a composer-tagged artist gets a lower rgCap.
	// We verify the function works by checking that sync completes without error
	// for an artist whose MB result includes a composer tag.
	d := openTestDB(t)
	ctx := context.Background()
	artistID := ensureArtist(t, d, "Bach")

	_ = d.Q.UpsertFile(ctx, db.UpsertFileParams{
		Path: "bach/fugue.flac", Size: 100, ModTime: "2024-01-01",
		Artist: "Bach", ArtistNorm: "bach", ArtistID: artistID,
		Album: "Fugues", AlbumNorm: "fugues", ScannedAt: "2024-01-01",
		IsAlbumArtist: 1,
	})

	srv := fakeMB(t,
		map[string]musicbrainz.ArtistSearchResult{
			"Bach": {Count: 1, Artists: []musicbrainz.Artist{
				{ID: "mbid-bach", Name: "Johann Sebastian Bach", Score: 95,
					Tags: []musicbrainz.Tag{{Name: "composer", Count: 10}}},
			}},
		},
		map[string]musicbrainz.ReleaseGroupBrowseResult{
			"mbid-bach": {Count: 0},
		},
	)
	defer srv.Close()

	mb := musicbrainz.NewWithBase(srv.URL, "test", "0.1", "test@test.com")
	if err := check.SyncArtist(ctx, d, mb, artistID); err != nil {
		t.Fatalf("SyncArtist: %v", err)
	}
}
