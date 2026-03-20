package state

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// ensureArtist is a test helper that calls EnsureArtist and fails on error.
func ensureArtist(t *testing.T, db *DB, name string) int64 {
	t.Helper()
	id, err := db.EnsureArtist(name)
	if err != nil {
		t.Fatalf("EnsureArtist(%q): %v", name, err)
	}
	return id
}

// upsertAlbum is a test helper that calls UpsertAlbum and fails on error.
func upsertAlbum(t *testing.T, db *DB, artistID int64, a AlbumRecord) int64 {
	t.Helper()
	id, err := db.UpsertAlbum(artistID, a)
	if err != nil {
		t.Fatalf("UpsertAlbum(%q): %v", a.Title, err)
	}
	return id
}

// upsertTrack is a test helper that calls UpsertTrack and fails on error.
func upsertTrack(t *testing.T, db *DB, albumID int64, tr TrackRecord) {
	t.Helper()
	if err := db.UpsertTrack(albumID, tr); err != nil {
		t.Fatalf("UpsertTrack(%q): %v", tr.Title, err)
	}
}

func TestOpenClose(t *testing.T) {
	db := openTestDB(t)
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestAllFileMeta_Empty(t *testing.T) {
	db := openTestDB(t)

	m, err := db.AllFileMeta()
	if err != nil {
		t.Fatalf("AllFileMeta: %v", err)
	}
	if len(m) != 0 {
		t.Fatalf("expected empty map, got %d entries", len(m))
	}
}

func TestAllFileMeta_Unchanged(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().Truncate(time.Second)

	rec := FileRecord{
		Path:      "artist/album/song.flac",
		Size:      1000,
		ModTime:   now,
		Artist:    "Test",
		Album:     "Album",
		Title:     "Song",
		ScannedAt: now,
	}
	if err := db.UpsertFile(rec); err != nil {
		t.Fatalf("UpsertFile: %v", err)
	}

	m, err := db.AllFileMeta()
	if err != nil {
		t.Fatalf("AllFileMeta: %v", err)
	}
	fm, ok := m[rec.Path]
	if !ok {
		t.Fatal("expected file in map")
	}
	if fm.Size != rec.Size {
		t.Fatalf("size: got %d, want %d", fm.Size, rec.Size)
	}
	if fm.ModTime != rec.ModTime.Format(time.RFC3339) {
		t.Fatalf("mod_time: got %s, want %s", fm.ModTime, rec.ModTime.Format(time.RFC3339))
	}
	if fm.Title != "Song" {
		t.Fatalf("title: got %q, want %q", fm.Title, "Song")
	}
}

func TestAllFileMeta_MultipleFiles(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().Truncate(time.Second)

	for i := range 3 {
		rec := FileRecord{
			Path:      fmt.Sprintf("artist/album/song%d.flac", i),
			Size:      int64(1000 + i),
			ModTime:   now,
			Artist:    "Test",
			Album:     "Album",
			Title:     fmt.Sprintf("Song %d", i),
			ScannedAt: now,
		}
		if err := db.UpsertFile(rec); err != nil {
			t.Fatalf("UpsertFile: %v", err)
		}
	}

	m, err := db.AllFileMeta()
	if err != nil {
		t.Fatalf("AllFileMeta: %v", err)
	}
	if len(m) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(m))
	}
}

func TestUniqueArtists(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().Truncate(time.Second)

	files := []FileRecord{
		{Path: "a/1.flac", Size: 100, ModTime: now, Artist: "Zed", Album: "Z", ScannedAt: now},
		{Path: "b/2.flac", Size: 200, ModTime: now, Artist: "Alpha", Album: "A", ScannedAt: now},
		{Path: "c/3.flac", Size: 300, ModTime: now, Artist: "Alpha", Album: "B", ScannedAt: now},
		{Path: "d/4.flac", Size: 400, ModTime: now, Artist: "", Album: "", ScannedAt: now},
	}
	for _, f := range files {
		if err := db.UpsertFile(f); err != nil {
			t.Fatalf("UpsertFile: %v", err)
		}
	}

	artists, err := db.UniqueArtists()
	if err != nil {
		t.Fatalf("UniqueArtists: %v", err)
	}
	if len(artists) != 2 {
		t.Fatalf("expected 2 artists, got %d: %v", len(artists), artists)
	}
	if artists[0] != "Alpha" || artists[1] != "Zed" {
		t.Fatalf("unexpected order: %v", artists)
	}
}

func TestLocalAlbums(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().Truncate(time.Second)

	files := []FileRecord{
		{Path: "a/1.flac", Size: 100, ModTime: now, Artist: "A", Album: "X", ScannedAt: now},
		{Path: "a/2.flac", Size: 200, ModTime: now, Artist: "A", Album: "Y", ScannedAt: now},
		{Path: "a/3.flac", Size: 300, ModTime: now, Artist: "A", Album: "X", ScannedAt: now},
	}
	for _, f := range files {
		if err := db.UpsertFile(f); err != nil {
			t.Fatalf("UpsertFile: %v", err)
		}
	}

	albums, err := db.LocalAlbums("A")
	if err != nil {
		t.Fatalf("LocalAlbums: %v", err)
	}
	if len(albums) != 2 {
		t.Fatalf("expected 2 albums, got %d", len(albums))
	}
}

func TestUpsertArtistAndLookup(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().Truncate(time.Second)

	rec := ArtistRecord{
		Name:          "Radiohead",
		MBID:          "a74b1b7f-71a5-4011-9441-d0b5e4122711",
		LastCheckedAt: now,
		LatestRelease: "A Moon Shaped Pool",
		LatestDate:    "2016-05-08",
	}
	if err := db.UpsertArtist(rec); err != nil {
		t.Fatalf("UpsertArtist: %v", err)
	}

	got, err := db.Artist("Radiohead")
	if err != nil {
		t.Fatalf("Artist: %v", err)
	}
	if got == nil {
		t.Fatal("expected artist, got nil")
	}
	if got.MBID != rec.MBID {
		t.Fatalf("MBID mismatch: %q vs %q", got.MBID, rec.MBID)
	}
}

func TestArtist_NotFound(t *testing.T) {
	db := openTestDB(t)

	got, err := db.Artist("Nobody")
	if err != nil {
		t.Fatalf("Artist: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

func TestArtistSummaries(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().Truncate(time.Second)

	files := []FileRecord{
		{Path: "a/1.flac", Size: 100, ModTime: now, Artist: "Zed", Album: "Zebra", ScannedAt: now},
		{Path: "b/2.flac", Size: 200, ModTime: now, Artist: "Alpha", Album: "Apples", ScannedAt: now},
		{Path: "c/3.flac", Size: 300, ModTime: now, Artist: "Alpha", Album: "Bananas", ScannedAt: now},
		{Path: "d/4.flac", Size: 400, ModTime: now, Artist: "Alpha", Album: "Apples", ScannedAt: now},
		{Path: "e/5.flac", Size: 500, ModTime: now, Artist: "", Album: "", ScannedAt: now},
	}
	for _, f := range files {
		if err := db.UpsertFile(f); err != nil {
			t.Fatalf("UpsertFile: %v", err)
		}
	}

	summaries, err := db.ArtistSummaries()
	if err != nil {
		t.Fatalf("ArtistSummaries: %v", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("expected 2 summaries, got %d", len(summaries))
	}

	// Ordered by artist name
	if summaries[0].Name != "Alpha" {
		t.Fatalf("expected first artist Alpha, got %q", summaries[0].Name)
	}
	if summaries[0].AlbumCount != 2 {
		t.Fatalf("expected Alpha to have 2 albums, got %d", summaries[0].AlbumCount)
	}
	if summaries[0].NewestAlbum == "" {
		t.Fatal("expected Alpha to have a newest album")
	}

	if summaries[0].TrackCount != 3 {
		t.Fatalf("expected Alpha to have 3 tracks, got %d", summaries[0].TrackCount)
	}
	if summaries[0].Synced {
		t.Fatal("expected Alpha to not be synced")
	}

	if summaries[1].Name != "Zed" {
		t.Fatalf("expected second artist Zed, got %q", summaries[1].Name)
	}
	if summaries[1].AlbumCount != 1 {
		t.Fatalf("expected Zed to have 1 album, got %d", summaries[1].AlbumCount)
	}
	if summaries[1].TrackCount != 1 {
		t.Fatalf("expected Zed to have 1 track, got %d", summaries[1].TrackCount)
	}
	if summaries[1].Synced {
		t.Fatal("expected Zed to not be synced")
	}
}

func TestArtistSummaries_Synced(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().Truncate(time.Second)

	if err := db.UpsertFile(FileRecord{
		Path: "a/1.flac", Size: 100, ModTime: now, Artist: "Radiohead", Album: "OK Computer", ScannedAt: now,
	}); err != nil {
		t.Fatalf("UpsertFile: %v", err)
	}

	// Before artist record: not synced
	summaries, err := db.ArtistSummaries()
	if err != nil {
		t.Fatalf("ArtistSummaries: %v", err)
	}
	if summaries[0].Synced {
		t.Fatal("expected not synced before artist record")
	}

	// Insert artist with MBID
	if err := db.UpsertArtist(ArtistRecord{
		Name: "Radiohead", MBID: "abc-123", LastCheckedAt: now,
	}); err != nil {
		t.Fatalf("UpsertArtist: %v", err)
	}

	summaries, err = db.ArtistSummaries()
	if err != nil {
		t.Fatalf("ArtistSummaries: %v", err)
	}
	if !summaries[0].Synced {
		t.Fatal("expected synced after artist record with MBID")
	}
}

func TestArtistSummaries_Empty(t *testing.T) {
	db := openTestDB(t)

	summaries, err := db.ArtistSummaries()
	if err != nil {
		t.Fatalf("ArtistSummaries: %v", err)
	}
	if len(summaries) != 0 {
		t.Fatalf("expected 0 summaries, got %d", len(summaries))
	}
}

func TestConcurrentWrites(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().Truncate(time.Second)

	// Simulate concurrent writes from multiple goroutines, which triggers
	// SQLITE_BUSY if the connection pool has more than one connection.
	errc := make(chan error, 50)
	for i := range 50 {
		go func() {
			errc <- db.UpsertFile(FileRecord{
				Path:      fmt.Sprintf("artist/album/song%d.flac", i),
				Size:      int64(i * 100),
				ModTime:   now,
				Artist:    "Test",
				Album:     "Album",
				ScannedAt: now,
			})
		}()
	}
	for range 50 {
		if err := <-errc; err != nil {
			t.Fatalf("concurrent UpsertFile: %v", err)
		}
	}
}

func TestMarkArtistNotFound(t *testing.T) {
	db := openTestDB(t)

	if err := db.MarkArtistNotFound("Podcast Host"); err != nil {
		t.Fatalf("MarkArtistNotFound: %v", err)
	}

	got, err := db.Artist("Podcast Host")
	if err != nil {
		t.Fatalf("Artist: %v", err)
	}
	if got == nil {
		t.Fatal("expected artist record, got nil")
	}
	if !got.NotFound {
		t.Fatal("expected NotFound == true")
	}
}

func TestMarkArtistNotFound_ClearedByUpsert(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().Truncate(time.Second)

	if err := db.MarkArtistNotFound("Radiohead"); err != nil {
		t.Fatalf("MarkArtistNotFound: %v", err)
	}

	// Upserting a real artist record should clear not_found.
	rec := ArtistRecord{
		Name:          "Radiohead",
		MBID:          "a74b1b7f-71a5-4011-9441-d0b5e4122711",
		LastCheckedAt: now,
		LatestRelease: "A Moon Shaped Pool",
		LatestDate:    "2016-05-08",
		NotFound:      false,
	}
	if err := db.UpsertArtist(rec); err != nil {
		t.Fatalf("UpsertArtist: %v", err)
	}

	got, err := db.Artist("Radiohead")
	if err != nil {
		t.Fatalf("Artist: %v", err)
	}
	if got == nil {
		t.Fatal("expected artist record, got nil")
	}
	if got.NotFound {
		t.Fatal("expected NotFound == false after upsert")
	}
}

func TestUpsertAlbumAndQuery(t *testing.T) {
	db := openTestDB(t)
	artistID := ensureArtist(t, db, "Radiohead")

	albums := []AlbumRecord{
		{Title: "OK Computer", MBID: "aaa", ReleaseDate: "1997-05-21", PrimaryType: "Album"},
		{Title: "Kid A", MBID: "bbb", ReleaseDate: "2000-10-02", PrimaryType: "Album"},
		{Title: "A Moon Shaped Pool", MBID: "ccc", ReleaseDate: "2016-05-08", PrimaryType: "Album"},
	}
	for _, a := range albums {
		upsertAlbum(t, db, artistID, a)
	}

	got, err := db.Albums("Radiohead")
	if err != nil {
		t.Fatalf("Albums: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 albums, got %d", len(got))
	}
	// Oldest first (ASC)
	if got[0].Title != "OK Computer" {
		t.Fatalf("expected oldest album first, got %q", got[0].Title)
	}
	if got[2].Title != "A Moon Shaped Pool" {
		t.Fatalf("expected newest album last, got %q", got[2].Title)
	}
}

func TestMarkLocalTracks(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().Truncate(time.Second)

	// Insert local files with titles
	files := []FileRecord{
		{Path: "a/1.flac", Size: 100, ModTime: now, Artist: "Radiohead", Album: "OK Computer", Title: "Airbag", ScannedAt: now},
		{Path: "a/2.flac", Size: 200, ModTime: now, Artist: "Radiohead", Album: "OK Computer", Title: "Paranoid Android", ScannedAt: now},
		{Path: "a/3.flac", Size: 300, ModTime: now, Artist: "Radiohead", Album: "Kid A", Title: "Everything in Its Right Place", ScannedAt: now},
	}
	for _, f := range files {
		if err := db.UpsertFile(f); err != nil {
			t.Fatalf("UpsertFile: %v", err)
		}
	}

	artistID := ensureArtist(t, db, "Radiohead")
	okID := upsertAlbum(t, db, artistID, AlbumRecord{Title: "OK Computer"})
	kidID := upsertAlbum(t, db, artistID, AlbumRecord{Title: "Kid A"})
	amID := upsertAlbum(t, db, artistID, AlbumRecord{Title: "Amnesiac"})

	// Insert tracks
	tracks := []struct {
		albumID int64
		tr      TrackRecord
	}{
		{okID, TrackRecord{Title: "Airbag", Position: 1}},
		{okID, TrackRecord{Title: "Paranoid Android", Position: 2}},
		{okID, TrackRecord{Title: "Subterranean Homesick Alien", Position: 3}},
		{kidID, TrackRecord{Title: "Everything in Its Right Place", Position: 1}},
		{kidID, TrackRecord{Title: "Kid A", Position: 2}},
		{amID, TrackRecord{Title: "Packt Like Sardines in a Crushd Tin Box", Position: 1}},
	}
	for _, tt := range tracks {
		upsertTrack(t, db, tt.albumID, tt.tr)
	}

	if err := db.MarkLocalTracks("Radiohead"); err != nil {
		t.Fatalf("MarkLocalTracks: %v", err)
	}

	// Check OK Computer tracks: 2 of 3 should be local
	okTracks, err := db.Tracks("Radiohead", "OK Computer")
	if err != nil {
		t.Fatalf("Tracks: %v", err)
	}
	localCount := 0
	for _, tr := range okTracks {
		if tr.Local {
			localCount++
		}
	}
	if localCount != 2 {
		t.Fatalf("expected 2 local OK Computer tracks, got %d", localCount)
	}

	// Check Amnesiac: 0 should be local
	amTracks, err := db.Tracks("Radiohead", "Amnesiac")
	if err != nil {
		t.Fatalf("Tracks: %v", err)
	}
	for _, tr := range amTracks {
		if tr.Local {
			t.Fatal("Amnesiac track should not be local")
		}
	}
}

func TestMarkLocalTracks_FuzzyTitle(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().Truncate(time.Second)

	// Local files have canonical titles and track numbers from file tags
	files := []FileRecord{
		{Path: "a/1.flac", Size: 100, ModTime: now, Artist: "3 Doors Down", Album: "The Better Life", Title: "Kryptonite", TrackNumber: 3, ScannedAt: now},
		{Path: "a/2.flac", Size: 200, ModTime: now, Artist: "3 Doors Down", Album: "The Better Life", Title: "Loser", TrackNumber: 4, ScannedAt: now},
		{Path: "a/3.flac", Size: 300, ModTime: now, Artist: "3 Doors Down", Album: "The Better Life", Title: "Be Like That", TrackNumber: 7, ScannedAt: now},
	}
	for _, f := range files {
		if err := db.UpsertFile(f); err != nil {
			t.Fatalf("UpsertFile: %v", err)
		}
	}

	artistID := ensureArtist(t, db, "3 Doors Down")
	albumID := upsertAlbum(t, db, artistID, AlbumRecord{Title: "The Better Life"})

	// MusicBrainz tracks may have release-specific title variations
	tracks := []TrackRecord{
		{Title: "Kryptonite", Position: 3},         // exact match
		{Title: "Loser (radio edit)", Position: 4}, // title differs, same position
		{Title: "Be Like That", Position: 7},       // exact match
		{Title: "Duck and Run", Position: 5},       // not local
		{Title: "By My Side", Position: 6},         // not local
	}
	for _, tr := range tracks {
		upsertTrack(t, db, albumID, tr)
	}

	if err := db.MarkLocalTracks("3 Doors Down"); err != nil {
		t.Fatalf("MarkLocalTracks: %v", err)
	}

	got, err := db.Tracks("3 Doors Down", "The Better Life")
	if err != nil {
		t.Fatalf("Tracks: %v", err)
	}

	localByTitle := make(map[string]bool)
	for _, tr := range got {
		localByTitle[tr.Title] = tr.Local
	}

	// "Kryptonite" — exact title match
	if !localByTitle["Kryptonite"] {
		t.Error("expected Kryptonite to be local (exact match)")
	}
	// "Loser (radio edit)" — title differs but same artist+album+position as local "Loser"
	if !localByTitle["Loser (radio edit)"] {
		t.Error("expected 'Loser (radio edit)' to be local (position match)")
	}
	// "Be Like That" — exact match
	if !localByTitle["Be Like That"] {
		t.Error("expected Be Like That to be local (exact match)")
	}
	// "Duck and Run" — no local file
	if localByTitle["Duck and Run"] {
		t.Error("expected Duck and Run to NOT be local")
	}
	// "By My Side" — no local file
	if localByTitle["By My Side"] {
		t.Error("expected By My Side to NOT be local")
	}
}

func TestMarkLocalTracks_TrackNumberOnly(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().Truncate(time.Second)

	// Simulate files where tags had no title but scanner extracted track number
	files := []FileRecord{
		{Path: "a/06.flac", Size: 100, ModTime: now, Artist: "10,000 Maniacs", Album: "The Earth Pressed Flat", Title: "", TrackNumber: 6, ScannedAt: now},
		{Path: "a/11.flac", Size: 200, ModTime: now, Artist: "10,000 Maniacs", Album: "The Earth Pressed Flat", Title: "", TrackNumber: 11, ScannedAt: now},
	}
	for _, f := range files {
		if err := db.UpsertFile(f); err != nil {
			t.Fatalf("UpsertFile: %v", err)
		}
	}

	artistID := ensureArtist(t, db, "10,000 Maniacs")
	albumID := upsertAlbum(t, db, artistID, AlbumRecord{Title: "The Earth Pressed Flat"})

	tracks := []TrackRecord{
		{Title: "Somebody's Heaven", Position: 6},
		{Title: "Time Turns", Position: 11},
		{Title: "Ellen", Position: 2},
	}
	for _, tr := range tracks {
		upsertTrack(t, db, albumID, tr)
	}

	if err := db.MarkLocalTracks("10,000 Maniacs"); err != nil {
		t.Fatalf("MarkLocalTracks: %v", err)
	}

	got, err := db.Tracks("10,000 Maniacs", "The Earth Pressed Flat")
	if err != nil {
		t.Fatalf("Tracks: %v", err)
	}

	localCount := 0
	for _, tr := range got {
		if tr.Local {
			localCount++
		}
	}
	if localCount != 2 {
		t.Fatalf("expected 2 local tracks, got %d", localCount)
	}
}

func TestMarkLocalTracks_ClearsStale(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().Truncate(time.Second)

	// Insert a local file
	if err := db.UpsertFile(FileRecord{
		Path: "a/1.flac", Size: 100, ModTime: now, Artist: "A", Album: "X", Title: "Song", ScannedAt: now,
	}); err != nil {
		t.Fatalf("UpsertFile: %v", err)
	}

	artistID := ensureArtist(t, db, "A")
	albumID := upsertAlbum(t, db, artistID, AlbumRecord{Title: "X"})

	// Insert a track and mark it local
	upsertTrack(t, db, albumID, TrackRecord{Title: "Song", Position: 1, Local: true})

	// Remove the file (simulate library change)
	if _, err := db.RemoveStaleFiles(map[string]struct{}{}); err != nil {
		t.Fatalf("RemoveStaleFiles: %v", err)
	}

	// MarkLocalTracks should clear the stale local flag
	if err := db.MarkLocalTracks("A"); err != nil {
		t.Fatalf("MarkLocalTracks: %v", err)
	}

	tracks, err := db.Tracks("A", "X")
	if err != nil {
		t.Fatalf("Tracks: %v", err)
	}
	if len(tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(tracks))
	}
	if tracks[0].Local {
		t.Fatal("expected Local == false after file removed")
	}
}

func TestUpsertTrackAndQuery(t *testing.T) {
	db := openTestDB(t)

	artistID := ensureArtist(t, db, "Radiohead")
	albumID := upsertAlbum(t, db, artistID, AlbumRecord{Title: "OK Computer"})

	tracks := []TrackRecord{
		{Title: "Paranoid Android", Position: 2, MBID: "aaa", LengthMS: 383000},
		{Title: "Airbag", Position: 1, MBID: "bbb", LengthMS: 284000},
		{Title: "Lucky", Position: 3, MBID: "ccc", LengthMS: 258000},
	}
	for _, tr := range tracks {
		upsertTrack(t, db, albumID, tr)
	}

	got, err := db.Tracks("Radiohead", "OK Computer")
	if err != nil {
		t.Fatalf("Tracks: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 tracks, got %d", len(got))
	}
	// Should be ordered by position
	if got[0].Title != "Airbag" {
		t.Fatalf("expected first track Airbag, got %q", got[0].Title)
	}
	if got[1].Title != "Paranoid Android" {
		t.Fatalf("expected second track Paranoid Android, got %q", got[1].Title)
	}
	if got[2].Title != "Lucky" {
		t.Fatalf("expected third track Lucky, got %q", got[2].Title)
	}
}

func TestUpsertTrack_UpdatesOnConflict(t *testing.T) {
	db := openTestDB(t)

	artistID := ensureArtist(t, db, "Radiohead")
	albumID := upsertAlbum(t, db, artistID, AlbumRecord{Title: "OK Computer"})

	tr := TrackRecord{Title: "Airbag", Position: 1, MBID: "aaa", LengthMS: 284000}
	upsertTrack(t, db, albumID, tr)

	// Update position and length
	tr.Position = 5
	tr.LengthMS = 300000
	upsertTrack(t, db, albumID, tr)

	got, err := db.Tracks("Radiohead", "OK Computer")
	if err != nil {
		t.Fatalf("Tracks: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 track after upsert, got %d", len(got))
	}
	if got[0].Position != 5 {
		t.Fatalf("expected position 5, got %d", got[0].Position)
	}
	if got[0].LengthMS != 300000 {
		t.Fatalf("expected length 300000, got %d", got[0].LengthMS)
	}
}

func TestAlbumsWithTrackCounts(t *testing.T) {
	db := openTestDB(t)

	artistID := ensureArtist(t, db, "Radiohead")

	// Create albums
	okID := upsertAlbum(t, db, artistID, AlbumRecord{Title: "OK Computer", MBID: "aaa", ReleaseDate: "1997-05-21", PrimaryType: "Album"})
	upsertAlbum(t, db, artistID, AlbumRecord{Title: "Kid A", MBID: "bbb", ReleaseDate: "2000-10-02", PrimaryType: "Album"})

	// Add tracks to OK Computer (3 total, 2 local)
	upsertTrack(t, db, okID, TrackRecord{Title: "Airbag", Position: 1, Local: true})
	upsertTrack(t, db, okID, TrackRecord{Title: "Paranoid Android", Position: 2, Local: true})
	upsertTrack(t, db, okID, TrackRecord{Title: "Lucky", Position: 3})

	// Kid A has no tracks

	got, err := db.Albums("Radiohead")
	if err != nil {
		t.Fatalf("Albums: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 albums, got %d", len(got))
	}

	// OK Computer (1997) should be first (oldest, ASC)
	if got[0].Title != "OK Computer" {
		t.Fatalf("expected OK Computer first, got %q", got[0].Title)
	}
	if got[0].TotalTracks != 3 {
		t.Fatalf("expected 3 total tracks, got %d", got[0].TotalTracks)
	}
	if got[0].LocalTracks != 2 {
		t.Fatalf("expected 2 local tracks, got %d", got[0].LocalTracks)
	}

	// Kid A (2000) second
	if got[1].Title != "Kid A" {
		t.Fatalf("expected Kid A second, got %q", got[1].Title)
	}
	if got[1].TotalTracks != 0 || got[1].LocalTracks != 0 {
		t.Fatalf("Kid A should have 0 tracks, got total=%d local=%d", got[1].TotalTracks, got[1].LocalTracks)
	}
}

func TestFileTitleMigration(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().Truncate(time.Second)

	rec := FileRecord{
		Path:      "a/1.flac",
		Size:      100,
		ModTime:   now,
		Artist:    "Radiohead",
		Album:     "OK Computer",
		Title:     "Airbag",
		ScannedAt: now,
	}
	if err := db.UpsertFile(rec); err != nil {
		t.Fatalf("UpsertFile: %v", err)
	}

	// Read it back via a raw query to verify title round-trips
	var title string
	err := db.db.QueryRow("SELECT title FROM files WHERE path = ?", "a/1.flac").Scan(&title)
	if err != nil {
		t.Fatalf("query title: %v", err)
	}
	if title != "Airbag" {
		t.Fatalf("expected title Airbag, got %q", title)
	}
}

func TestMigrationFromV0(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")

	// Create a raw DB with the old schema (no track_number column).
	rawDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	const oldSchema = `
	CREATE TABLE files (
		path       TEXT PRIMARY KEY,
		size       INTEGER NOT NULL,
		mod_time   TEXT NOT NULL,
		artist     TEXT NOT NULL,
		album      TEXT NOT NULL DEFAULT '',
		title      TEXT NOT NULL DEFAULT '',
		scanned_at TEXT NOT NULL
	);
	CREATE TABLE albums (
		artist_name  TEXT NOT NULL,
		title        TEXT NOT NULL,
		mbid         TEXT NOT NULL DEFAULT '',
		release_date TEXT NOT NULL DEFAULT '',
		primary_type TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (artist_name, title)
	);
	CREATE TABLE tracks (
		artist_name TEXT NOT NULL,
		album_title TEXT NOT NULL,
		title       TEXT NOT NULL,
		position    INTEGER NOT NULL DEFAULT 0,
		mbid        TEXT NOT NULL DEFAULT '',
		length_ms   INTEGER NOT NULL DEFAULT 0,
		local       INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (artist_name, album_title, title)
	);
	CREATE TABLE artists (
		name            TEXT PRIMARY KEY,
		mbid            TEXT NOT NULL DEFAULT '',
		last_checked_at TEXT NOT NULL DEFAULT '',
		latest_release  TEXT NOT NULL DEFAULT '',
		latest_date     TEXT NOT NULL DEFAULT '',
		not_found       INTEGER NOT NULL DEFAULT 0
	);
	`
	if _, err := rawDB.Exec(oldSchema); err != nil {
		t.Fatalf("exec old schema: %v", err)
	}
	if err := rawDB.Close(); err != nil {
		t.Fatalf("close raw db: %v", err)
	}

	// Reopen via state.Open which triggers migrate().
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open after old schema: %v", err)
	}
	defer db.Close()

	// Verify the new integer PK schema exists.
	var colCount int
	err = db.db.QueryRow(
		"SELECT COUNT(*) FROM pragma_table_info('artists') WHERE name = 'id'",
	).Scan(&colCount)
	if err != nil {
		t.Fatalf("check id column: %v", err)
	}
	if colCount != 1 {
		t.Fatal("expected id column on artists after migration")
	}

	// Verify user_version is 8.
	var version int
	if err := db.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if version != 8 {
		t.Fatalf("expected user_version 8, got %d", version)
	}
}

func TestMigrationIdempotent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")

	// First open: creates fresh DB at latest version.
	db1, err := Open(dbPath)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	if err := db1.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}

	// Second open: migrate() should be a no-op.
	db2, err := Open(dbPath)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	defer db2.Close()

	var version int
	if err := db2.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if version != 8 {
		t.Fatalf("expected user_version 8, got %d", version)
	}
}

func TestRemoveStaleFiles(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().Truncate(time.Second)

	files := []FileRecord{
		{Path: "a/1.flac", Size: 100, ModTime: now, Artist: "A", Album: "X", ScannedAt: now},
		{Path: "b/2.flac", Size: 200, ModTime: now, Artist: "B", Album: "Y", ScannedAt: now},
		{Path: "c/3.flac", Size: 300, ModTime: now, Artist: "C", Album: "Z", ScannedAt: now},
	}
	for _, f := range files {
		if err := db.UpsertFile(f); err != nil {
			t.Fatalf("UpsertFile: %v", err)
		}
	}

	live := map[string]struct{}{
		"a/1.flac": {},
	}

	removed, err := db.RemoveStaleFiles(live)
	if err != nil {
		t.Fatalf("RemoveStaleFiles: %v", err)
	}
	if removed != 2 {
		t.Fatalf("expected 2 removed, got %d", removed)
	}

	artists, err := db.UniqueArtists()
	if err != nil {
		t.Fatalf("UniqueArtists: %v", err)
	}
	if len(artists) != 1 || artists[0] != "A" {
		t.Fatalf("expected [A], got %v", artists)
	}
}

func TestMarkLocalTracks_NormalizedTitle(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().Truncate(time.Second)

	// Local file has plain title; MB track has parenthetical suffix
	files := []FileRecord{
		{Path: "a/1.flac", Size: 100, ModTime: now, Artist: "Beck", Album: "Mellow Gold", Title: "Loser", TrackNumber: 1, ScannedAt: now},
	}
	for _, f := range files {
		if err := db.UpsertFile(f); err != nil {
			t.Fatalf("UpsertFile: %v", err)
		}
	}

	artistID := ensureArtist(t, db, "Beck")
	albumID := upsertAlbum(t, db, artistID, AlbumRecord{Title: "Mellow Gold"})

	tracks := []TrackRecord{
		{Title: "Loser (radio edit)", Position: 1},
		{Title: "Pay No Mind", Position: 2},
	}
	for _, tr := range tracks {
		upsertTrack(t, db, albumID, tr)
	}

	if err := db.MarkLocalTracks("Beck"); err != nil {
		t.Fatalf("MarkLocalTracks: %v", err)
	}

	got, err := db.Tracks("Beck", "Mellow Gold")
	if err != nil {
		t.Fatalf("Tracks: %v", err)
	}

	localByTitle := make(map[string]bool)
	for _, tr := range got {
		localByTitle[tr.Title] = tr.Local
	}

	if !localByTitle["Loser (radio edit)"] {
		t.Error("expected 'Loser (radio edit)' to match local 'Loser' via normalized title")
	}
	if localByTitle["Pay No Mind"] {
		t.Error("expected 'Pay No Mind' to NOT be local")
	}
}

func TestMarkLocalTracks_NormalizedAlbum(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().Truncate(time.Second)

	// Local file has "Away From The Sun"; MB has "Away from the Sun"
	files := []FileRecord{
		{Path: "a/1.flac", Size: 100, ModTime: now, Artist: "3 Doors Down", Album: "Away From The Sun", Title: "When I'm Gone", TrackNumber: 1, ScannedAt: now},
	}
	for _, f := range files {
		if err := db.UpsertFile(f); err != nil {
			t.Fatalf("UpsertFile: %v", err)
		}
	}

	artistID := ensureArtist(t, db, "3 Doors Down")
	albumID := upsertAlbum(t, db, artistID, AlbumRecord{Title: "Away from the Sun"})

	tracks := []TrackRecord{
		{Title: "When I'm Gone", Position: 1},
	}
	for _, tr := range tracks {
		upsertTrack(t, db, albumID, tr)
	}

	if err := db.MarkLocalTracks("3 Doors Down"); err != nil {
		t.Fatalf("MarkLocalTracks: %v", err)
	}

	got, err := db.Tracks("3 Doors Down", "Away from the Sun")
	if err != nil {
		t.Fatalf("Tracks: %v", err)
	}
	if len(got) != 1 || !got[0].Local {
		t.Error("expected track to be local despite album casing difference")
	}
}

func TestKnownAlbumMBIDs(t *testing.T) {
	db := openTestDB(t)
	artistID := ensureArtist(t, db, "Radiohead")

	// Album with tracks → should be in known set
	okID := upsertAlbum(t, db, artistID, AlbumRecord{Title: "OK Computer", MBID: "rg-okc", ReleaseDate: "1997-05-21", PrimaryType: "Album"})
	upsertTrack(t, db, okID, TrackRecord{Title: "Airbag", Position: 1, MBID: "tr-1"})

	// Album without tracks → should NOT be in known set
	upsertAlbum(t, db, artistID, AlbumRecord{Title: "Kid A", MBID: "rg-kida", ReleaseDate: "2000-10-02", PrimaryType: "Album"})

	// Album with empty MBID → should NOT be in known set
	amID := upsertAlbum(t, db, artistID, AlbumRecord{Title: "Amnesiac", MBID: "", ReleaseDate: "2001-06-05", PrimaryType: "Album"})
	upsertTrack(t, db, amID, TrackRecord{Title: "Packt", Position: 1})

	// Different artist → should not appear
	beckID := ensureArtist(t, db, "Beck")
	mgID := upsertAlbum(t, db, beckID, AlbumRecord{Title: "Mellow Gold", MBID: "rg-mg", ReleaseDate: "1994-03-01", PrimaryType: "Album"})
	upsertTrack(t, db, mgID, TrackRecord{Title: "Loser", Position: 1, MBID: "tr-2"})

	known, err := db.KnownAlbumMBIDs("Radiohead")
	if err != nil {
		t.Fatalf("KnownAlbumMBIDs: %v", err)
	}

	if _, ok := known["rg-okc"]; !ok {
		t.Error("expected rg-okc (OK Computer with tracks) to be known")
	}
	if _, ok := known["rg-kida"]; ok {
		t.Error("expected rg-kida (Kid A without tracks) to NOT be known")
	}
	if _, ok := known["rg-mg"]; ok {
		t.Error("expected rg-mg (Beck album) to NOT appear for Radiohead")
	}
	if len(known) != 1 {
		t.Fatalf("expected 1 known MBID, got %d: %v", len(known), known)
	}
}

func TestMarkLocalTracks_CrossAlbum(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().Truncate(time.Second)

	// Local file on album "Greatest Hits"; MB track on album "Mellow Gold"
	files := []FileRecord{
		{Path: "a/1.flac", Size: 100, ModTime: now, Artist: "Beck", Album: "Greatest Hits", Title: "Loser", TrackNumber: 1, ScannedAt: now},
	}
	for _, f := range files {
		if err := db.UpsertFile(f); err != nil {
			t.Fatalf("UpsertFile: %v", err)
		}
	}

	artistID := ensureArtist(t, db, "Beck")
	albumID := upsertAlbum(t, db, artistID, AlbumRecord{Title: "Mellow Gold"})

	tracks := []TrackRecord{
		{Title: "Loser", Position: 1},
		{Title: "Pay No Mind", Position: 2},
	}
	for _, tr := range tracks {
		upsertTrack(t, db, albumID, tr)
	}

	if err := db.MarkLocalTracks("Beck"); err != nil {
		t.Fatalf("MarkLocalTracks: %v", err)
	}

	got, err := db.Tracks("Beck", "Mellow Gold")
	if err != nil {
		t.Fatalf("Tracks: %v", err)
	}

	localByTitle := make(map[string]bool)
	for _, tr := range got {
		localByTitle[tr.Title] = tr.Local
	}

	if !localByTitle["Loser"] {
		t.Error("expected 'Loser' to match cross-album via tier 2 (title-only)")
	}
	if localByTitle["Pay No Mind"] {
		t.Error("expected 'Pay No Mind' to NOT be local")
	}
}

func TestGetSetMonitorStatus(t *testing.T) {
	db := openTestDB(t)

	// Unknown artist defaults to MonitorAlways.
	status, err := db.GetMonitorStatus("Unknown Artist")
	if err != nil {
		t.Fatalf("GetMonitorStatus unknown: %v", err)
	}
	if status != MonitorAlways {
		t.Fatalf("expected MonitorAlways for unknown artist, got %q", status)
	}

	// Round-trip each of the 3 statuses.
	cases := []MonitorStatus{MonitorAlways, MonitorSometimes, MonitorIgnore}
	for _, want := range cases {
		if err := db.SetMonitorStatus("Test Artist", want); err != nil {
			t.Fatalf("SetMonitorStatus %q: %v", want, err)
		}
		got, err := db.GetMonitorStatus("Test Artist")
		if err != nil {
			t.Fatalf("GetMonitorStatus after Set %q: %v", want, err)
		}
		if got != want {
			t.Fatalf("expected %q, got %q", want, got)
		}
	}

	// Calling SetMonitorStatus twice on the same artist updates (not duplicates).
	if err := db.SetMonitorStatus("Test Artist", MonitorIgnore); err != nil {
		t.Fatalf("SetMonitorStatus second call: %v", err)
	}
	if err := db.SetMonitorStatus("Test Artist", MonitorSometimes); err != nil {
		t.Fatalf("SetMonitorStatus third call: %v", err)
	}
	got, err := db.GetMonitorStatus("Test Artist")
	if err != nil {
		t.Fatalf("GetMonitorStatus after double set: %v", err)
	}
	if got != MonitorSometimes {
		t.Fatalf("expected MonitorSometimes after update, got %q", got)
	}

	// Verify no duplicate rows exist.
	var rowCount int
	if err := db.db.QueryRow("SELECT COUNT(*) FROM artists WHERE name_norm = ?", Normalize("Test Artist")).Scan(&rowCount); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if rowCount != 1 {
		t.Fatalf("expected 1 row, got %d", rowCount)
	}
}

func TestAllFileMeta_EmptyTitle(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().Truncate(time.Second)

	rec := FileRecord{
		Path:      "artist/album/song.flac",
		Size:      1000,
		ModTime:   now,
		Artist:    "Test",
		Album:     "Album",
		Title:     "", // empty — signals tags were not read
		ScannedAt: now,
	}
	if err := db.UpsertFile(rec); err != nil {
		t.Fatalf("UpsertFile: %v", err)
	}

	m, err := db.AllFileMeta()
	if err != nil {
		t.Fatalf("AllFileMeta: %v", err)
	}
	fm := m[rec.Path]
	if fm.Title != "" {
		t.Fatalf("expected empty title, got %q", fm.Title)
	}
}

func TestAlbums_SecondaryTypes(t *testing.T) {
	db := openTestDB(t)

	artistID := ensureArtist(t, db, "Radiohead")
	upsertAlbum(t, db, artistID, AlbumRecord{
		Title:          "OK Computer OKNOTOK",
		MBID:           "aaa",
		ReleaseDate:    "2017-06-23",
		PrimaryType:    "Album",
		SecondaryTypes: "Compilation",
	})

	got, err := db.Albums("Radiohead")
	if err != nil {
		t.Fatalf("Albums: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 album, got %d", len(got))
	}
	if got[0].SecondaryTypes != "Compilation" {
		t.Fatalf("expected SecondaryTypes %q, got %q", "Compilation", got[0].SecondaryTypes)
	}
}

func TestTracks_LocalRoundTrip(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().Truncate(time.Second)

	// Insert local file so MarkLocalTracks can match it.
	if err := db.UpsertFile(FileRecord{
		Path: "a/1.flac", Size: 100, ModTime: now,
		Artist: "Radiohead", Album: "OK Computer", Title: "Airbag",
		TrackNumber: 1, ScannedAt: now,
	}); err != nil {
		t.Fatalf("UpsertFile: %v", err)
	}

	artistID := ensureArtist(t, db, "Radiohead")
	albumID := upsertAlbum(t, db, artistID, AlbumRecord{Title: "OK Computer"})

	// Insert track with Local=true explicitly.
	upsertTrack(t, db, albumID, TrackRecord{Title: "Airbag", Position: 1, Local: true})

	if err := db.MarkLocalTracks("Radiohead"); err != nil {
		t.Fatalf("MarkLocalTracks: %v", err)
	}

	tracks, err := db.Tracks("Radiohead", "OK Computer")
	if err != nil {
		t.Fatalf("Tracks: %v", err)
	}
	if len(tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(tracks))
	}
	if !tracks[0].Local {
		t.Fatal("expected Local == true for matched track")
	}
}

func TestUpsertArtist_UpdateFields(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().Truncate(time.Second)

	first := ArtistRecord{
		Name:          "Radiohead",
		MBID:          "mbid-v1",
		LastCheckedAt: now,
		LatestRelease: "Pablo Honey",
		LatestDate:    "1993-02-22",
	}
	if err := db.UpsertArtist(first); err != nil {
		t.Fatalf("UpsertArtist first: %v", err)
	}

	later := now.Add(time.Hour)
	second := ArtistRecord{
		Name:          "Radiohead",
		MBID:          "mbid-v2",
		LastCheckedAt: later,
		LatestRelease: "A Moon Shaped Pool",
		LatestDate:    "2016-05-08",
	}
	if err := db.UpsertArtist(second); err != nil {
		t.Fatalf("UpsertArtist second: %v", err)
	}

	got, err := db.Artist("Radiohead")
	if err != nil {
		t.Fatalf("Artist: %v", err)
	}
	if got == nil {
		t.Fatal("expected artist record, got nil")
	}
	if got.MBID != "mbid-v2" {
		t.Fatalf("expected MBID %q, got %q", "mbid-v2", got.MBID)
	}
	if got.LatestRelease != "A Moon Shaped Pool" {
		t.Fatalf("expected LatestRelease %q, got %q", "A Moon Shaped Pool", got.LatestRelease)
	}
	if got.LatestDate != "2016-05-08" {
		t.Fatalf("expected LatestDate %q, got %q", "2016-05-08", got.LatestDate)
	}
}

func TestArtistSummaries_HasNew(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().Truncate(time.Second)

	// Seed local files for two artists.
	for _, f := range []FileRecord{
		{Path: "a/1.flac", Size: 100, ModTime: now, Artist: "Radiohead", Album: "OK Computer", Title: "Airbag", ScannedAt: now},
		{Path: "b/1.flac", Size: 100, ModTime: now, Artist: "Beck", Album: "Mellow Gold", Title: "Loser", ScannedAt: now},
		{Path: "c/1.flac", Size: 100, ModTime: now, Artist: "Bjork", Album: "Debut", Title: "Human Behaviour", ScannedAt: now},
	} {
		if err := db.UpsertFile(f); err != nil {
			t.Fatalf("UpsertFile: %v", err)
		}
	}

	// Radiohead: catalog has albums from 1997 and 2016; local tracks on 1997 album only → HasNew = true
	rhID := ensureArtist(t, db, "Radiohead")
	okID := upsertAlbum(t, db, rhID, AlbumRecord{Title: "OK Computer", MBID: "aaa", ReleaseDate: "1997-05-21", PrimaryType: "Album"})
	upsertAlbum(t, db, rhID, AlbumRecord{Title: "A Moon Shaped Pool", MBID: "bbb", ReleaseDate: "2016-05-08", PrimaryType: "Album"})
	upsertTrack(t, db, okID, TrackRecord{Title: "Airbag", Position: 1, Local: true})

	// Beck: catalog only has the album the user already has locally → HasNew = false
	beckID := ensureArtist(t, db, "Beck")
	mgID := upsertAlbum(t, db, beckID, AlbumRecord{Title: "Mellow Gold", MBID: "ccc", ReleaseDate: "1994-03-01", PrimaryType: "Album"})
	upsertTrack(t, db, mgID, TrackRecord{Title: "Loser", Position: 1, Local: true})

	// Bjork: not synced (no albums/tracks in catalog) → HasNew = false

	summaries, err := db.ArtistSummaries()
	if err != nil {
		t.Fatalf("ArtistSummaries: %v", err)
	}
	if len(summaries) != 3 {
		t.Fatalf("expected 3 summaries, got %d", len(summaries))
	}

	byName := make(map[string]ArtistSummary)
	for _, s := range summaries {
		byName[s.Name] = s
	}

	if !byName["Radiohead"].HasNew {
		t.Error("expected Radiohead HasNew = true (catalog album 2016 > local 1997)")
	}
	if byName["Beck"].HasNew {
		t.Error("expected Beck HasNew = false (no catalog albums newer than local)")
	}
	if byName["Bjork"].HasNew {
		t.Error("expected Bjork HasNew = false (not synced)")
	}
}

func TestArtistSummaries_MonitorField(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().Truncate(time.Second)

	// Seed a file so the artist appears in ArtistSummaries.
	if err := db.UpsertFile(FileRecord{
		Path: "a/1.flac", Size: 100, ModTime: now,
		Artist: "Radiohead", Album: "OK Computer", ScannedAt: now,
	}); err != nil {
		t.Fatalf("UpsertFile: %v", err)
	}

	// Default (no artists row) should be MonitorAlways.
	summaries, err := db.ArtistSummaries()
	if err != nil {
		t.Fatalf("ArtistSummaries (default): %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}
	if summaries[0].Monitor != MonitorAlways {
		t.Fatalf("expected MonitorAlways by default, got %q", summaries[0].Monitor)
	}

	// Set to Ignore and verify.
	if err := db.SetMonitorStatus("Radiohead", MonitorIgnore); err != nil {
		t.Fatalf("SetMonitorStatus: %v", err)
	}
	summaries, err = db.ArtistSummaries()
	if err != nil {
		t.Fatalf("ArtistSummaries (after set): %v", err)
	}
	if summaries[0].Monitor != MonitorIgnore {
		t.Fatalf("expected MonitorIgnore, got %q", summaries[0].Monitor)
	}

	// Set to Sometimes and verify.
	if err := db.SetMonitorStatus("Radiohead", MonitorSometimes); err != nil {
		t.Fatalf("SetMonitorStatus: %v", err)
	}
	summaries, err = db.ArtistSummaries()
	if err != nil {
		t.Fatalf("ArtistSummaries (after sometimes): %v", err)
	}
	if summaries[0].Monitor != MonitorSometimes {
		t.Fatalf("expected MonitorSometimes, got %q", summaries[0].Monitor)
	}
}

func TestAlbums_DeduplicatesArtistNameVariants(t *testing.T) {
	db := openTestDB(t)

	// With integer PKs, both name variants resolve to the same artist_id,
	// so there's no duplication at the album level.
	artistID := ensureArtist(t, db, "Asaf Avidan")

	gsID := upsertAlbum(t, db, artistID, AlbumRecord{Title: "Gold Shadow", MBID: "aaa", ReleaseDate: "2015-01-26", PrimaryType: "Album"})
	upsertAlbum(t, db, artistID, AlbumRecord{Title: "Anagnorisis", MBID: "bbb", ReleaseDate: "2020-07-03", PrimaryType: "Album"})

	// Calling EnsureArtist with a different casing returns the same ID
	artistID2 := ensureArtist(t, db, "asaf avidan")
	if artistID2 != artistID {
		t.Fatalf("expected same artist ID for different casing, got %d vs %d", artistID, artistID2)
	}

	// Add tracks
	upsertTrack(t, db, gsID, TrackRecord{Title: "Over My Head", Position: 1, Local: true})
	upsertTrack(t, db, gsID, TrackRecord{Title: "My Old Pain", Position: 2})

	got, err := db.Albums("Asaf Avidan")
	if err != nil {
		t.Fatalf("Albums: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 unique albums, got %d", len(got))
	}

	// Verify track counts are correct (not doubled)
	for _, a := range got {
		if a.Title == "Gold Shadow" {
			if a.TotalTracks != 2 {
				t.Errorf("Gold Shadow: expected 2 total tracks, got %d", a.TotalTracks)
			}
			if a.LocalTracks != 1 {
				t.Errorf("Gold Shadow: expected 1 local track, got %d", a.LocalTracks)
			}
		}
	}
}

func TestArtistSummaries_MergesByNorm(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().Truncate(time.Second)

	// Insert files with different casings of the same artist name.
	// "Alice In Chains" (3 files) vs "Alice in Chains" (1 file) — should merge.
	files := []FileRecord{
		{Path: "a/1.flac", Size: 100, ModTime: now, Artist: "Alice In Chains", Album: "Dirt", Title: "Rooster", ScannedAt: now},
		{Path: "a/2.flac", Size: 100, ModTime: now, Artist: "Alice In Chains", Album: "Dirt", Title: "Down in a Hole", ScannedAt: now},
		{Path: "a/3.flac", Size: 100, ModTime: now, Artist: "Alice In Chains", Album: "Jar of Flies", Title: "No Excuses", ScannedAt: now},
		{Path: "b/1.flac", Size: 100, ModTime: now, Artist: "Alice in Chains", Album: "Facelift", Title: "Man in the Box", ScannedAt: now},
	}
	for _, f := range files {
		if err := db.UpsertFile(f); err != nil {
			t.Fatalf("UpsertFile: %v", err)
		}
	}

	summaries, err := db.ArtistSummaries()
	if err != nil {
		t.Fatalf("ArtistSummaries: %v", err)
	}

	if len(summaries) != 1 {
		names := make([]string, len(summaries))
		for i, s := range summaries {
			names[i] = fmt.Sprintf("%q (tracks=%d)", s.Name, s.TrackCount)
		}
		t.Fatalf("expected 1 merged artist, got %d: %v", len(summaries), names)
	}

	s := summaries[0]
	// Canonical name should be the most-used variant.
	if s.Name != "Alice In Chains" {
		t.Errorf("expected canonical name %q, got %q", "Alice In Chains", s.Name)
	}
	if s.TrackCount != 4 {
		t.Errorf("expected 4 tracks, got %d", s.TrackCount)
	}
	if s.AlbumCount != 3 {
		t.Errorf("expected 3 albums (Dirt, Jar of Flies, Facelift), got %d", s.AlbumCount)
	}
}

func TestEnsureArtist(t *testing.T) {
	db := openTestDB(t)

	// First call creates
	id1, err := db.EnsureArtist("Radiohead")
	if err != nil {
		t.Fatalf("EnsureArtist: %v", err)
	}
	if id1 == 0 {
		t.Fatal("expected non-zero ID")
	}

	// Second call with same name returns same ID
	id2, err := db.EnsureArtist("Radiohead")
	if err != nil {
		t.Fatalf("EnsureArtist: %v", err)
	}
	if id2 != id1 {
		t.Fatalf("expected same ID %d, got %d", id1, id2)
	}

	// Different casing returns same ID
	id3, err := db.EnsureArtist("radiohead")
	if err != nil {
		t.Fatalf("EnsureArtist: %v", err)
	}
	if id3 != id1 {
		t.Fatalf("expected same ID %d for different casing, got %d", id1, id3)
	}
}

// TestTrackCounts_ConsistentBetweenSummariesAndAlbums verifies that the track
// counts reported by ArtistSummaries match the per-album counts from Albums().
// Regression test for #4qr-gn0 / #1lk-md3.
func TestTrackCounts_ConsistentBetweenSummariesAndAlbums(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().Truncate(time.Second)

	// Seed a local file so the artist appears in summaries.
	if err := db.UpsertFile(FileRecord{
		Path: "a/1.flac", Size: 100, ModTime: now,
		Artist: "Amy Lee", Album: "Recover", Title: "Use My Voice",
		ScannedAt: now,
	}); err != nil {
		t.Fatalf("UpsertFile: %v", err)
	}

	// Create artist with MBID (synced), album, and tracks.
	if err := db.UpsertArtist(ArtistRecord{
		Name: "Amy Lee", MBID: "mbid-amylee", LastCheckedAt: now,
	}); err != nil {
		t.Fatalf("UpsertArtist: %v", err)
	}
	artistID := ensureArtist(t, db, "Amy Lee")

	// Album 1: has a track that matches local file
	a1ID := upsertAlbum(t, db, artistID, AlbumRecord{
		Title: "Recover", MBID: "aaa", ReleaseDate: "2023-04-14", PrimaryType: "Album",
	})
	for i, title := range []string{"Use My Voice", "Blind Faith", "Love Exists"} {
		upsertTrack(t, db, a1ID, TrackRecord{Title: title, Position: i + 1})
	}

	// Album 2: no matching local files at all
	a2ID := upsertAlbum(t, db, artistID, AlbumRecord{
		Title: "Dream Too Much", MBID: "bbb", ReleaseDate: "2016-09-30", PrimaryType: "Album",
	})
	for i, title := range []string{"I'm Not Tired", "Rubber Duckie"} {
		upsertTrack(t, db, a2ID, TrackRecord{Title: title, Position: i + 1})
	}

	// Run MarkLocalTracks to set local flags
	if err := db.MarkLocalTracks("Amy Lee"); err != nil {
		t.Fatalf("MarkLocalTracks: %v", err)
	}

	// ArtistSummaries local track count must equal sum of per-album local counts.
	summaries, err := db.ArtistSummaries()
	if err != nil {
		t.Fatalf("ArtistSummaries: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}
	if summaries[0].TotalTracks != 5 {
		t.Fatalf("ArtistSummaries: expected 5 total tracks, got %d", summaries[0].TotalTracks)
	}

	albums, err := db.Albums("Amy Lee")
	if err != nil {
		t.Fatalf("Albums: %v", err)
	}

	albumLocalSum := 0
	for _, a := range albums {
		albumLocalSum += a.LocalTracks
	}

	if summaries[0].TrackCount != albumLocalSum {
		t.Fatalf("ArtistSummaries.TrackCount (%d) != sum of album LocalTracks (%d)",
			summaries[0].TrackCount, albumLocalSum)
	}
}

func TestMigrationV7toV8_WithData(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")

	// Create a v7 database with data to test the migration
	rawDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}

	// Set up v7 schema
	const v7Schema = `
	PRAGMA user_version = 7;
	CREATE TABLE files (
		path         TEXT PRIMARY KEY,
		size         INTEGER NOT NULL,
		mod_time     TEXT NOT NULL,
		artist       TEXT NOT NULL,
		album        TEXT NOT NULL DEFAULT '',
		title        TEXT NOT NULL DEFAULT '',
		track_number INTEGER NOT NULL DEFAULT 0,
		scanned_at   TEXT NOT NULL,
		title_norm   TEXT NOT NULL DEFAULT '',
		album_norm   TEXT NOT NULL DEFAULT '',
		artist_norm  TEXT NOT NULL DEFAULT ''
	);
	CREATE TABLE artists (
		name            TEXT PRIMARY KEY,
		mbid            TEXT NOT NULL DEFAULT '',
		last_checked_at TEXT NOT NULL DEFAULT '',
		latest_release  TEXT NOT NULL DEFAULT '',
		latest_date     TEXT NOT NULL DEFAULT '',
		not_found       INTEGER NOT NULL DEFAULT 0,
		monitor         TEXT NOT NULL DEFAULT 'monitor',
		name_norm       TEXT NOT NULL DEFAULT ''
	);
	CREATE TABLE albums (
		artist_name     TEXT NOT NULL,
		title           TEXT NOT NULL,
		mbid            TEXT NOT NULL DEFAULT '',
		release_date    TEXT NOT NULL DEFAULT '',
		primary_type    TEXT NOT NULL DEFAULT '',
		secondary_types TEXT NOT NULL DEFAULT '',
		artist_norm     TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (artist_name, title)
	);
	CREATE TABLE tracks (
		artist_name TEXT NOT NULL,
		album_title TEXT NOT NULL,
		title       TEXT NOT NULL,
		position    INTEGER NOT NULL DEFAULT 0,
		mbid        TEXT NOT NULL DEFAULT '',
		length_ms   INTEGER NOT NULL DEFAULT 0,
		local       INTEGER NOT NULL DEFAULT 0,
		title_norm  TEXT NOT NULL DEFAULT '',
		album_norm  TEXT NOT NULL DEFAULT '',
		artist_norm TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (artist_name, album_title, title)
	);
	`
	if _, err := rawDB.Exec(v7Schema); err != nil {
		t.Fatalf("exec v7 schema: %v", err)
	}

	// Insert test data with name variants
	if _, err := rawDB.Exec(`
		INSERT INTO artists (name, mbid, name_norm, last_checked_at) VALUES
			('Radiohead', 'mbid-rh', 'radiohead', '2024-01-01T00:00:00Z'),
			('radiohead', '', 'radiohead', '');
		INSERT INTO albums (artist_name, title, mbid, release_date, primary_type, artist_norm) VALUES
			('Radiohead', 'OK Computer', 'aaa', '1997-05-21', 'Album', 'radiohead'),
			('radiohead', 'OK Computer', 'aaa', '1997-05-21', 'Album', 'radiohead'),
			('Radiohead', 'Kid A', 'bbb', '2000-10-02', 'Album', 'radiohead');
		INSERT INTO tracks (artist_name, album_title, title, position, mbid, local, title_norm, album_norm, artist_norm) VALUES
			('Radiohead', 'OK Computer', 'Airbag', 1, 'tr-1', 1, 'airbag', 'ok computer', 'radiohead'),
			('Radiohead', 'OK Computer', 'Paranoid Android', 2, 'tr-2', 0, 'paranoid android', 'ok computer', 'radiohead');
	`); err != nil {
		t.Fatalf("insert v7 data: %v", err)
	}
	if err := rawDB.Close(); err != nil {
		t.Fatalf("close raw db: %v", err)
	}

	// Open with migration
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	// Verify version
	var version int
	if err := db.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("read version: %v", err)
	}
	if version != 8 {
		t.Fatalf("expected version 8, got %d", version)
	}

	// Verify deduplicated artists (both variants → 1 row)
	var artistCount int
	if err := db.db.QueryRow("SELECT COUNT(*) FROM artists").Scan(&artistCount); err != nil {
		t.Fatalf("count artists: %v", err)
	}
	if artistCount != 1 {
		t.Fatalf("expected 1 artist after dedup, got %d", artistCount)
	}

	// Verify deduplicated albums (duplicate "OK Computer" → 1 row)
	albums, err := db.Albums("Radiohead")
	if err != nil {
		t.Fatalf("Albums: %v", err)
	}
	if len(albums) != 2 {
		t.Fatalf("expected 2 albums (OK Computer, Kid A), got %d", len(albums))
	}

	// Verify tracks survived
	tracks, err := db.Tracks("Radiohead", "OK Computer")
	if err != nil {
		t.Fatalf("Tracks: %v", err)
	}
	if len(tracks) != 2 {
		t.Fatalf("expected 2 tracks, got %d", len(tracks))
	}
	if !tracks[0].Local {
		t.Error("expected Airbag to remain local after migration")
	}
}
