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

func TestOpenClose(t *testing.T) {
	db := openTestDB(t)
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestFileChanged_NewFile(t *testing.T) {
	db := openTestDB(t)

	changed, err := db.FileChanged("artist/album/song.flac", 1000, time.Now())
	if err != nil {
		t.Fatalf("FileChanged: %v", err)
	}
	if !changed {
		t.Fatal("expected new file to be marked as changed")
	}
}

func TestFileChanged_Unchanged(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().Truncate(time.Second)

	rec := FileRecord{
		Path:      "artist/album/song.flac",
		Size:      1000,
		ModTime:   now,
		Artist:    "Test",
		Album:     "Album",
		ScannedAt: now,
	}
	if err := db.UpsertFile(rec); err != nil {
		t.Fatalf("UpsertFile: %v", err)
	}

	changed, err := db.FileChanged(rec.Path, rec.Size, rec.ModTime)
	if err != nil {
		t.Fatalf("FileChanged: %v", err)
	}
	if changed {
		t.Fatal("expected unchanged file not to be marked as changed")
	}
}

func TestFileChanged_Modified(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().Truncate(time.Second)

	rec := FileRecord{
		Path:      "artist/album/song.flac",
		Size:      1000,
		ModTime:   now,
		Artist:    "Test",
		Album:     "Album",
		ScannedAt: now,
	}
	if err := db.UpsertFile(rec); err != nil {
		t.Fatalf("UpsertFile: %v", err)
	}

	changed, err := db.FileChanged(rec.Path, rec.Size, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("FileChanged: %v", err)
	}
	if !changed {
		t.Fatal("expected modified file to be marked as changed")
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

	albums := []AlbumRecord{
		{ArtistName: "Radiohead", Title: "OK Computer", MBID: "aaa", ReleaseDate: "1997-05-21", PrimaryType: "Album"},
		{ArtistName: "Radiohead", Title: "Kid A", MBID: "bbb", ReleaseDate: "2000-10-02", PrimaryType: "Album"},
		{ArtistName: "Radiohead", Title: "A Moon Shaped Pool", MBID: "ccc", ReleaseDate: "2016-05-08", PrimaryType: "Album"},
	}
	for _, a := range albums {
		if err := db.UpsertAlbum(a); err != nil {
			t.Fatalf("UpsertAlbum: %v", err)
		}
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

	// Insert tracks
	tracks := []TrackRecord{
		{ArtistName: "Radiohead", AlbumTitle: "OK Computer", Title: "Airbag", Position: 1},
		{ArtistName: "Radiohead", AlbumTitle: "OK Computer", Title: "Paranoid Android", Position: 2},
		{ArtistName: "Radiohead", AlbumTitle: "OK Computer", Title: "Subterranean Homesick Alien", Position: 3},
		{ArtistName: "Radiohead", AlbumTitle: "Kid A", Title: "Everything in Its Right Place", Position: 1},
		{ArtistName: "Radiohead", AlbumTitle: "Kid A", Title: "Kid A", Position: 2},
		{ArtistName: "Radiohead", AlbumTitle: "Amnesiac", Title: "Packt Like Sardines in a Crushd Tin Box", Position: 1},
	}
	for _, tr := range tracks {
		if err := db.UpsertTrack(tr); err != nil {
			t.Fatalf("UpsertTrack: %v", err)
		}
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

	// MusicBrainz tracks may have release-specific title variations
	tracks := []TrackRecord{
		{ArtistName: "3 Doors Down", AlbumTitle: "The Better Life", Title: "Kryptonite", Position: 3},         // exact match
		{ArtistName: "3 Doors Down", AlbumTitle: "The Better Life", Title: "Loser (radio edit)", Position: 4}, // title differs, same position
		{ArtistName: "3 Doors Down", AlbumTitle: "The Better Life", Title: "Be Like That", Position: 7},       // exact match
		{ArtistName: "3 Doors Down", AlbumTitle: "The Better Life", Title: "Duck and Run", Position: 5},       // not local
		{ArtistName: "3 Doors Down", AlbumTitle: "The Better Life", Title: "By My Side", Position: 6},         // not local
	}
	for _, tr := range tracks {
		if err := db.UpsertTrack(tr); err != nil {
			t.Fatalf("UpsertTrack: %v", err)
		}
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

func TestMarkLocalTracks_ClearsStale(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().Truncate(time.Second)

	// Insert a local file
	if err := db.UpsertFile(FileRecord{
		Path: "a/1.flac", Size: 100, ModTime: now, Artist: "A", Album: "X", Title: "Song", ScannedAt: now,
	}); err != nil {
		t.Fatalf("UpsertFile: %v", err)
	}

	// Insert a track and mark it local
	if err := db.UpsertTrack(TrackRecord{
		ArtistName: "A", AlbumTitle: "X", Title: "Song", Position: 1, Local: true,
	}); err != nil {
		t.Fatalf("UpsertTrack: %v", err)
	}

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

	tracks := []TrackRecord{
		{ArtistName: "Radiohead", AlbumTitle: "OK Computer", Title: "Paranoid Android", Position: 2, MBID: "aaa", LengthMS: 383000},
		{ArtistName: "Radiohead", AlbumTitle: "OK Computer", Title: "Airbag", Position: 1, MBID: "bbb", LengthMS: 284000},
		{ArtistName: "Radiohead", AlbumTitle: "OK Computer", Title: "Lucky", Position: 3, MBID: "ccc", LengthMS: 258000},
	}
	for _, tr := range tracks {
		if err := db.UpsertTrack(tr); err != nil {
			t.Fatalf("UpsertTrack: %v", err)
		}
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

	tr := TrackRecord{
		ArtistName: "Radiohead", AlbumTitle: "OK Computer", Title: "Airbag",
		Position: 1, MBID: "aaa", LengthMS: 284000,
	}
	if err := db.UpsertTrack(tr); err != nil {
		t.Fatalf("UpsertTrack: %v", err)
	}

	// Update position and length
	tr.Position = 5
	tr.LengthMS = 300000
	if err := db.UpsertTrack(tr); err != nil {
		t.Fatalf("UpsertTrack update: %v", err)
	}

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

	// Create albums
	albums := []AlbumRecord{
		{ArtistName: "Radiohead", Title: "OK Computer", MBID: "aaa", ReleaseDate: "1997-05-21", PrimaryType: "Album"},
		{ArtistName: "Radiohead", Title: "Kid A", MBID: "bbb", ReleaseDate: "2000-10-02", PrimaryType: "Album"},
	}
	for _, a := range albums {
		if err := db.UpsertAlbum(a); err != nil {
			t.Fatalf("UpsertAlbum: %v", err)
		}
	}

	// Add tracks to OK Computer (3 total, 2 local)
	okTracks := []TrackRecord{
		{ArtistName: "Radiohead", AlbumTitle: "OK Computer", Title: "Airbag", Position: 1, Local: true},
		{ArtistName: "Radiohead", AlbumTitle: "OK Computer", Title: "Paranoid Android", Position: 2, Local: true},
		{ArtistName: "Radiohead", AlbumTitle: "OK Computer", Title: "Lucky", Position: 3},
	}
	for _, tr := range okTracks {
		if err := db.UpsertTrack(tr); err != nil {
			t.Fatalf("UpsertTrack: %v", err)
		}
	}

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

	// Verify track_number column exists.
	var colCount int
	err = db.db.QueryRow(
		"SELECT COUNT(*) FROM pragma_table_info('files') WHERE name = 'track_number'",
	).Scan(&colCount)
	if err != nil {
		t.Fatalf("check track_number column: %v", err)
	}
	if colCount != 1 {
		t.Fatal("expected track_number column to exist after migration")
	}

	// Verify user_version is 2.
	var version int
	if err := db.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if version != 2 {
		t.Fatalf("expected user_version 2, got %d", version)
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
	if version != 2 {
		t.Fatalf("expected user_version 2, got %d", version)
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
