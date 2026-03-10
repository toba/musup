package state

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"
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

	if summaries[1].Name != "Zed" {
		t.Fatalf("expected second artist Zed, got %q", summaries[1].Name)
	}
	if summaries[1].AlbumCount != 1 {
		t.Fatalf("expected Zed to have 1 album, got %d", summaries[1].AlbumCount)
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
