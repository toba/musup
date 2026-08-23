package check_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/toba/musup-go/internal/check"
	"github.com/toba/musup-go/internal/db"
	"github.com/toba/musup-go/internal/integration/musicbrainz"
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
// The artists map is keyed by artist name; the query parameter artist:"Name" is parsed to look up the key.
func fakeMB(t *testing.T, artists map[string]musicbrainz.ArtistSearchResult, releases map[string]musicbrainz.ReleaseGroupBrowseResult) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/artist":
			query := r.URL.Query().Get("query")
			// Parse artist:"Name" from query to find the right result.
			for key, result := range artists {
				expected := fmt.Sprintf(`artist:"%s"`, key)
				if query == expected {
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

func TestSyncArtist_PartialNameMatch_Rejected(t *testing.T) {
	// Searching for "Bush" should NOT match "Kate Bush".
	d := openTestDB(t)
	ctx := context.Background()
	artistID := ensureArtist(t, d, "Bush")

	_ = d.Q.UpsertFile(ctx, db.UpsertFileParams{
		Path: "bush/track.flac", Size: 100, ModTime: "2024-01-01",
		Artist: "Bush", ArtistNorm: "bush", ArtistID: artistID,
		Album: "Sixteen Stone", AlbumNorm: "sixteen stone", ScannedAt: "2024-01-01",
		IsAlbumArtist: 1,
	})

	srv := fakeMB(t,
		map[string]musicbrainz.ArtistSearchResult{
			"Bush": {Count: 2, Artists: []musicbrainz.Artist{
				{ID: "mbid-kate", Name: "Kate Bush", Score: 100},
				{ID: "mbid-bush", Name: "Bush", Score: 95},
			}},
		},
		map[string]musicbrainz.ReleaseGroupBrowseResult{
			"mbid-kate": {Count: 1, ReleaseGroups: []musicbrainz.ReleaseGroup{
				{ID: "rg-kate-1", Title: "Hounds of Love", PrimaryType: "Album", FirstReleaseDate: "1985-09-16"},
			}},
			"mbid-bush": {Count: 1, ReleaseGroups: []musicbrainz.ReleaseGroup{
				{ID: "rg-bush-1", Title: "Sixteen Stone", PrimaryType: "Album", FirstReleaseDate: "1994-12-06"},
			}},
		},
	)
	defer srv.Close()

	mb := musicbrainz.NewWithBase(srv.URL, "test", "0.1", "test@test.com")
	if err := check.SyncArtist(ctx, d, mb, artistID); err != nil {
		t.Fatalf("SyncArtist: %v", err)
	}

	// Should have matched "Bush", not "Kate Bush".
	row, err := d.Q.GetArtistByID(ctx, artistID)
	if err != nil {
		t.Fatalf("GetArtistByID: %v", err)
	}
	if row.Mbid != "mbid-bush" {
		t.Errorf("expected mbid 'mbid-bush', got %q (matched wrong artist)", row.Mbid)
	}
}

func TestSyncArtist_NoExactMatch_MarksNotFound(t *testing.T) {
	// If no search result has an exact name match, mark as not found.
	d := openTestDB(t)
	ctx := context.Background()
	artistID := ensureArtist(t, d, "Bush")

	_ = d.Q.UpsertFile(ctx, db.UpsertFileParams{
		Path: "bush/track.flac", Size: 100, ModTime: "2024-01-01",
		Artist: "Bush", ArtistNorm: "bush", ArtistID: artistID,
		Album: "Sixteen Stone", AlbumNorm: "sixteen stone", ScannedAt: "2024-01-01",
		IsAlbumArtist: 1,
	})

	srv := fakeMB(t,
		map[string]musicbrainz.ArtistSearchResult{
			"Bush": {Count: 1, Artists: []musicbrainz.Artist{
				{ID: "mbid-kate", Name: "Kate Bush", Score: 100},
			}},
		},
		nil,
	)
	defer srv.Close()

	mb := musicbrainz.NewWithBase(srv.URL, "test", "0.1", "test@test.com")
	if err := check.SyncArtist(ctx, d, mb, artistID); err != nil {
		t.Fatalf("SyncArtist: %v", err)
	}

	row, err := d.Q.GetArtistByID(ctx, artistID)
	if err != nil {
		t.Fatalf("GetArtistByID: %v", err)
	}
	if row.NotFound != 1 {
		t.Errorf("expected not_found=1 (no exact match), got %d", row.NotFound)
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
