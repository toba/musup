package db

import (
	"database/sql"
	"errors"
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

// testUpsertAlbum is a test helper that calls Q.UpsertAlbum and fails on error.
func testUpsertAlbum(t *testing.T, db *DB, artistID int64, p UpsertAlbumParams) {
	t.Helper()
	p.ArtistID = artistID
	if p.TitleNorm == "" {
		p.TitleNorm = Normalize(p.Title)
	}
	if err := db.Q.UpsertAlbum(bg, p); err != nil {
		t.Fatalf("UpsertAlbum(%q): %v", p.Title, err)
	}
}

// testFileParams builds a UpsertFileParams with sensible defaults.
func testFileParams(path, artist, album, title string, opts ...func(*UpsertFileParams)) UpsertFileParams {
	now := time.Now().Truncate(time.Second).Format(time.RFC3339)
	p := UpsertFileParams{
		Path: path, Size: 100, ModTime: now, Artist: artist, Album: album, Title: title,
		IsAlbumArtist: 1, ScannedAt: now,
		TitleNorm: Normalize(title), AlbumNorm: Normalize(album), ArtistNorm: Normalize(artist),
	}
	for _, o := range opts {
		o(&p)
	}
	return p
}

func TestOpenClose(t *testing.T) {
	db := openTestDB(t)
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestAllFileMeta_Empty(t *testing.T) {
	db := openTestDB(t)

	rows, err := db.Q.AllFileMeta(bg)
	if err != nil {
		t.Fatalf("AllFileMeta: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected empty result, got %d entries", len(rows))
	}
}

func TestAllFileMeta_Unchanged(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().Truncate(time.Second)

	p := UpsertFileParams{
		Path:       "artist/album/song.flac",
		Size:       1000,
		ModTime:    now.Format(time.RFC3339),
		Artist:     "Test",
		Album:      "Album",
		Title:      "Song",
		ScannedAt:  now.Format(time.RFC3339),
		TitleNorm:  Normalize("Song"),
		AlbumNorm:  Normalize("Album"),
		ArtistNorm: Normalize("Test"),
	}
	if err := db.Q.UpsertFile(bg, p); err != nil {
		t.Fatalf("UpsertFile: %v", err)
	}

	rows, err := db.Q.AllFileMeta(bg)
	if err != nil {
		t.Fatalf("AllFileMeta: %v", err)
	}
	m := make(map[string]AllFileMetaRow, len(rows))
	for _, r := range rows {
		m[r.Path] = r
	}
	fm, ok := m[p.Path]
	if !ok {
		t.Fatal("expected file in map")
	}
	if fm.Size != p.Size {
		t.Fatalf("size: got %d, want %d", fm.Size, p.Size)
	}
	if fm.ModTime != now.Format(time.RFC3339) {
		t.Fatalf("mod_time: got %s, want %s", fm.ModTime, now.Format(time.RFC3339))
	}
	if fm.Title != "Song" {
		t.Fatalf("title: got %q, want %q", fm.Title, "Song")
	}
}

func TestAllFileMeta_MultipleFiles(t *testing.T) {
	db := openTestDB(t)

	for i := range 3 {
		p := testFileParams(
			fmt.Sprintf("artist/album/song%d.flac", i),
			"Test", "Album", fmt.Sprintf("Song %d", i),
			func(f *UpsertFileParams) { f.Size = int64(1000 + i) },
		)
		if err := db.Q.UpsertFile(bg, p); err != nil {
			t.Fatalf("UpsertFile: %v", err)
		}
	}

	rows, err := db.Q.AllFileMeta(bg)
	if err != nil {
		t.Fatalf("AllFileMeta: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(rows))
	}
}

func TestDistinctArtistIDs(t *testing.T) {
	db := openTestDB(t)

	zedID := ensureArtist(t, db, "Zed")
	alphaID := ensureArtist(t, db, "Alpha")
	guestID := ensureArtist(t, db, "Guest")

	files := []UpsertFileParams{
		testFileParams("a/1.flac", "Zed", "Z", "", func(f *UpsertFileParams) { f.ArtistID = zedID }),
		testFileParams("b/2.flac", "Alpha", "A", "", func(f *UpsertFileParams) { f.ArtistID = alphaID }),
		testFileParams("c/3.flac", "Alpha", "B", "", func(f *UpsertFileParams) { f.ArtistID = alphaID }),
		testFileParams("d/4.flac", "Guest", "", "", func(f *UpsertFileParams) { f.IsAlbumArtist = 0; f.ArtistID = guestID }),
	}
	for _, f := range files {
		if err := db.Q.UpsertFile(bg, f); err != nil {
			t.Fatalf("UpsertFile: %v", err)
		}
	}

	ids, err := db.Q.DistinctArtistIDs(bg)
	if err != nil {
		t.Fatalf("DistinctArtistIDs: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 artist IDs, got %d: %v", len(ids), ids)
	}
}

func TestUpsertArtistAndLookup(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().Truncate(time.Second)

	id, err := db.EnsureArtist("Radiohead")
	if err != nil {
		t.Fatalf("EnsureArtist: %v", err)
	}
	err = db.Q.UpdateArtistFull(bg, "a74b1b7f-71a5-4011-9441-d0b5e4122711", now.Format(time.RFC3339), 0, id)
	if err != nil {
		t.Fatalf("UpdateArtistFull: %v", err)
	}

	got, err := db.Q.GetArtistByNameNorm(bg, Normalize("Radiohead"))
	if err != nil {
		t.Fatalf("GetArtistByNameNorm: %v", err)
	}
	if got.Mbid != "a74b1b7f-71a5-4011-9441-d0b5e4122711" {
		t.Fatalf("MBID mismatch: %q vs %q", got.Mbid, "a74b1b7f-71a5-4011-9441-d0b5e4122711")
	}
}

func TestArtist_NotFound(t *testing.T) {
	db := openTestDB(t)

	_, err := db.Q.GetArtistByNameNorm(bg, Normalize("Nobody"))
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestConcurrentWrites(t *testing.T) {
	db := openTestDB(t)

	errc := make(chan error, 50)
	for i := range 50 {
		go func() {
			p := testFileParams(
				fmt.Sprintf("artist/album/song%d.flac", i),
				"Test", "Album", "",
				func(f *UpsertFileParams) { f.Size = int64(i * 100) },
			)
			errc <- db.Q.UpsertFile(bg, p)
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

	got, err := db.Q.GetArtistByNameNorm(bg, Normalize("Podcast Host"))
	if err != nil {
		t.Fatalf("GetArtistByNameNorm: %v", err)
	}
	if got.NotFound != 1 {
		t.Fatal("expected NotFound == 1")
	}
}

func TestMarkArtistNotFound_ClearedByUpsert(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().Truncate(time.Second)

	if err := db.MarkArtistNotFound("Radiohead"); err != nil {
		t.Fatalf("MarkArtistNotFound: %v", err)
	}

	id, err := db.EnsureArtist("Radiohead")
	if err != nil {
		t.Fatalf("EnsureArtist: %v", err)
	}
	err = db.Q.UpdateArtistFull(bg, "a74b1b7f-71a5-4011-9441-d0b5e4122711", now.Format(time.RFC3339), 0, id)
	if err != nil {
		t.Fatalf("UpdateArtistFull: %v", err)
	}

	got, err := db.Q.GetArtistByNameNorm(bg, Normalize("Radiohead"))
	if err != nil {
		t.Fatalf("GetArtistByNameNorm: %v", err)
	}
	if got.NotFound != 0 {
		t.Fatal("expected NotFound == 0 after upsert")
	}
}

func TestUpsertAlbumAndQuery(t *testing.T) {
	db := openTestDB(t)
	artistID := ensureArtist(t, db, "Radiohead")

	albums := []UpsertAlbumParams{
		{Title: "OK Computer", Mbid: "aaa", ReleaseDate: "1997-05-21", PrimaryType: "Album"},
		{Title: "Kid A", Mbid: "bbb", ReleaseDate: "2000-10-02", PrimaryType: "Album"},
		{Title: "A Moon Shaped Pool", Mbid: "ccc", ReleaseDate: "2016-05-08", PrimaryType: "Album"},
	}
	for _, a := range albums {
		testUpsertAlbum(t, db, artistID, a)
	}

	// Verify albums exist via a simple count query.
	var count int
	err := db.db.QueryRow("SELECT COUNT(*) FROM albums WHERE artist_id = ?", artistID).Scan(&count)
	if err != nil {
		t.Fatalf("count albums: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected 3 albums, got %d", count)
	}
}

func TestFileTitleMigration(t *testing.T) {
	db := openTestDB(t)

	p := testFileParams("a/1.flac", "Radiohead", "OK Computer", "Airbag")
	if err := db.Q.UpsertFile(bg, p); err != nil {
		t.Fatalf("UpsertFile: %v", err)
	}

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

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open after old schema: %v", err)
	}
	defer db.Close()

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

	// Tracks table should be dropped by v13.
	var tableCount int
	err = db.db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='tracks'",
	).Scan(&tableCount)
	if err != nil {
		t.Fatalf("check tracks table: %v", err)
	}
	if tableCount != 0 {
		t.Fatal("expected tracks table to be dropped after migration")
	}

	var version int
	if err := db.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if version != 17 {
		t.Fatalf("expected user_version 16, got %d", version)
	}
}

func TestMigrationIdempotent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")

	db1, err := Open(dbPath)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	if err := db1.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}

	db2, err := Open(dbPath)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	defer db2.Close()

	var version int
	if err := db2.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if version != 17 {
		t.Fatalf("expected user_version 16, got %d", version)
	}
}

func TestRemoveStaleFiles(t *testing.T) {
	db := openTestDB(t)

	aID := ensureArtist(t, db, "A")
	bID := ensureArtist(t, db, "B")
	cID := ensureArtist(t, db, "C")

	files := []UpsertFileParams{
		testFileParams("a/1.flac", "A", "X", "", func(p *UpsertFileParams) { p.ArtistID = aID }),
		testFileParams("b/2.flac", "B", "Y", "", func(p *UpsertFileParams) { p.ArtistID = bID }),
		testFileParams("c/3.flac", "C", "Z", "", func(p *UpsertFileParams) { p.ArtistID = cID }),
	}
	for _, f := range files {
		if err := db.Q.UpsertFile(bg, f); err != nil {
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

	ids, err := db.Q.DistinctArtistIDs(bg)
	if err != nil {
		t.Fatalf("DistinctArtistIDs: %v", err)
	}
	if len(ids) != 1 {
		t.Fatalf("expected 1 artist ID, got %d: %v", len(ids), ids)
	}
}

func TestAllFileMeta_EmptyTitle(t *testing.T) {
	db := openTestDB(t)

	p := testFileParams("artist/album/song.flac", "Test", "Album", "")
	p.Size = 1000
	if err := db.Q.UpsertFile(bg, p); err != nil {
		t.Fatalf("UpsertFile: %v", err)
	}

	rows, err := db.Q.AllFileMeta(bg)
	if err != nil {
		t.Fatalf("AllFileMeta: %v", err)
	}
	m := make(map[string]AllFileMetaRow, len(rows))
	for _, r := range rows {
		m[r.Path] = r
	}
	fm := m[p.Path]
	if fm.Title != "" {
		t.Fatalf("expected empty title, got %q", fm.Title)
	}
}

func TestUpsertArtist_UpdateFields(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().Truncate(time.Second)

	id, err := db.EnsureArtist("Radiohead")
	if err != nil {
		t.Fatalf("EnsureArtist: %v", err)
	}
	err = db.Q.UpdateArtistFull(bg, "mbid-v1", now.Format(time.RFC3339), 0, id)
	if err != nil {
		t.Fatalf("UpdateArtistFull first: %v", err)
	}

	later := now.Add(time.Hour)
	err = db.Q.UpdateArtistFull(bg, "mbid-v2", later.Format(time.RFC3339), 0, id)
	if err != nil {
		t.Fatalf("UpdateArtistFull second: %v", err)
	}

	got, err := db.Q.GetArtistByNameNorm(bg, Normalize("Radiohead"))
	if err != nil {
		t.Fatalf("GetArtistByNameNorm: %v", err)
	}
	if got.Mbid != "mbid-v2" {
		t.Fatalf("expected MBID %q, got %q", "mbid-v2", got.Mbid)
	}
}

func TestEnsureArtist(t *testing.T) {
	db := openTestDB(t)

	id1, err := db.EnsureArtist("Radiohead")
	if err != nil {
		t.Fatalf("EnsureArtist: %v", err)
	}
	if id1 == 0 {
		t.Fatal("expected non-zero ID")
	}

	id2, err := db.EnsureArtist("Radiohead")
	if err != nil {
		t.Fatalf("EnsureArtist: %v", err)
	}
	if id2 != id1 {
		t.Fatalf("expected same ID %d, got %d", id1, id2)
	}

	id3, err := db.EnsureArtist("radiohead")
	if err != nil {
		t.Fatalf("EnsureArtist: %v", err)
	}
	if id3 != id1 {
		t.Fatalf("expected same ID %d for different casing, got %d", id1, id3)
	}
}

func TestMigrationV7toV8_WithData(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")

	rawDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}

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

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	var version int
	if err := db.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("read version: %v", err)
	}
	if version != 17 {
		t.Fatalf("expected version 15, got %d", version)
	}

	var artistCount int
	if err := db.db.QueryRow("SELECT COUNT(*) FROM artists").Scan(&artistCount); err != nil {
		t.Fatalf("count artists: %v", err)
	}
	if artistCount != 1 {
		t.Fatalf("expected 1 artist after dedup, got %d", artistCount)
	}

	var albumCount int
	if err := db.db.QueryRow("SELECT COUNT(*) FROM albums").Scan(&albumCount); err != nil {
		t.Fatalf("count albums: %v", err)
	}
	if albumCount != 2 {
		t.Fatalf("expected 2 albums (OK Computer, Kid A), got %d", albumCount)
	}

	// Tracks table should be dropped by v13.
	var tableCount int
	err = db.db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='tracks'",
	).Scan(&tableCount)
	if err != nil {
		t.Fatalf("check tracks table: %v", err)
	}
	if tableCount != 0 {
		t.Fatal("expected tracks table to be dropped after migration")
	}
}

func TestNewerReleases(t *testing.T) {
	db := openTestDB(t)

	rhID := ensureArtist(t, db, "Radiohead")
	beckID := ensureArtist(t, db, "Beck")

	// Set up local files for two artists.
	for _, f := range []UpsertFileParams{
		testFileParams("a/1.flac", "Radiohead", "OK Computer", "Airbag", func(p *UpsertFileParams) { p.ArtistID = rhID }),
		testFileParams("b/1.flac", "Beck", "Mellow Gold", "Loser", func(p *UpsertFileParams) { p.ArtistID = beckID }),
	} {
		if err := db.Q.UpsertFile(bg, f); err != nil {
			t.Fatalf("UpsertFile: %v", err)
		}
	}

	// Set up MB data: Radiohead has a newer album, Beck does not.
	testUpsertAlbum(t, db, rhID, UpsertAlbumParams{
		Title: "OK Computer", Mbid: "aaa", ReleaseDate: "1997-05-21", PrimaryType: "Album",
	})
	testUpsertAlbum(t, db, rhID, UpsertAlbumParams{
		Title: "A Moon Shaped Pool", Mbid: "bbb", ReleaseDate: "2016-05-08", PrimaryType: "Album",
	})
	testUpsertAlbum(t, db, beckID, UpsertAlbumParams{
		Title: "Mellow Gold", Mbid: "ccc", ReleaseDate: "1994-03-01", PrimaryType: "Album",
	})

	// Query for albums released in the last 20 years.
	releases, err := db.Q.NewerReleases(bg, "2000-01-01")
	if err != nil {
		t.Fatalf("NewerReleases: %v", err)
	}

	if len(releases) != 1 {
		t.Fatalf("expected 1 newer release, got %d", len(releases))
	}
	if releases[0].ArtistName != "Radiohead" {
		t.Errorf("expected artist Radiohead, got %q", releases[0].ArtistName)
	}
	if releases[0].AlbumTitle != "A Moon Shaped Pool" {
		t.Errorf("expected album 'A Moon Shaped Pool', got %q", releases[0].AlbumTitle)
	}
}

func TestNewerReleases_ExcludesLocalAlbums(t *testing.T) {
	db := openTestDB(t)

	rhID := ensureArtist(t, db, "Radiohead")

	// User has "A Moon Shaped Pool" locally.
	for _, f := range []UpsertFileParams{
		testFileParams("a/1.flac", "Radiohead", "OK Computer", "Airbag", func(p *UpsertFileParams) { p.ArtistID = rhID }),
		testFileParams("a/2.flac", "Radiohead", "A Moon Shaped Pool", "Burn the Witch", func(p *UpsertFileParams) { p.ArtistID = rhID }),
	} {
		if err := db.Q.UpsertFile(bg, f); err != nil {
			t.Fatalf("UpsertFile: %v", err)
		}
	}

	testUpsertAlbum(t, db, rhID, UpsertAlbumParams{
		Title: "OK Computer", Mbid: "aaa", ReleaseDate: "1997-05-21", PrimaryType: "Album",
	})
	testUpsertAlbum(t, db, rhID, UpsertAlbumParams{
		Title: "A Moon Shaped Pool", Mbid: "bbb", ReleaseDate: "2016-05-08", PrimaryType: "Album",
	})

	releases, err := db.Q.NewerReleases(bg, "2000-01-01")
	if err != nil {
		t.Fatalf("NewerReleases: %v", err)
	}
	if len(releases) != 0 {
		t.Fatalf("expected 0 releases (user already has it), got %d", len(releases))
	}
}

func TestAlbumArtists(t *testing.T) {
	db := openTestDB(t)

	rhID := ensureArtist(t, db, "Radiohead")
	beckID := ensureArtist(t, db, "Beck")
	guestID := ensureArtist(t, db, "Guest")

	for _, f := range []UpsertFileParams{
		testFileParams("a/1.flac", "Radiohead", "OK Computer", "Airbag", func(p *UpsertFileParams) { p.ArtistID = rhID }),
		testFileParams("b/1.flac", "Beck", "Mellow Gold", "Loser", func(p *UpsertFileParams) { p.ArtistID = beckID }),
		testFileParams("c/1.flac", "Guest", "Various", "Track", func(p *UpsertFileParams) { p.IsAlbumArtist = 0; p.ArtistID = guestID }),
	} {
		if err := db.Q.UpsertFile(bg, f); err != nil {
			t.Fatalf("UpsertFile: %v", err)
		}
	}

	artists, err := db.Q.AlbumArtists(bg)
	if err != nil {
		t.Fatalf("AlbumArtists: %v", err)
	}
	if len(artists) != 2 {
		t.Fatalf("expected 2 album artists, got %d", len(artists))
	}
	if artists[0].Name != "Beck" {
		t.Errorf("expected first artist Beck, got %q", artists[0].Name)
	}
	if artists[1].Name != "Radiohead" {
		t.Errorf("expected second artist Radiohead, got %q", artists[1].Name)
	}
	if artists[0].Followed != 1 {
		t.Errorf("expected Beck followed=1, got %d", artists[0].Followed)
	}
}

func TestSetFollowed(t *testing.T) {
	db := openTestDB(t)

	id := ensureArtist(t, db, "Radiohead")
	if err := db.Q.UpsertFile(bg, testFileParams("a/1.flac", "Radiohead", "OK Computer", "Airbag", func(p *UpsertFileParams) { p.ArtistID = id })); err != nil {
		t.Fatalf("UpsertFile: %v", err)
	}

	// Unfollow.
	if err := db.Q.SetFollowed(bg, 0, id); err != nil {
		t.Fatalf("SetFollowed(0): %v", err)
	}

	artists, err := db.Q.AlbumArtists(bg)
	if err != nil {
		t.Fatalf("AlbumArtists: %v", err)
	}
	if len(artists) != 1 {
		t.Fatalf("expected 1 artist, got %d", len(artists))
	}
	if artists[0].Followed != 0 {
		t.Fatalf("expected followed=0 after unfollow, got %d", artists[0].Followed)
	}

	// Re-follow.
	if err := db.Q.SetFollowed(bg, 1, id); err != nil {
		t.Fatalf("SetFollowed(1): %v", err)
	}
	artists, err = db.Q.AlbumArtists(bg)
	if err != nil {
		t.Fatalf("AlbumArtists: %v", err)
	}
	if artists[0].Followed != 1 {
		t.Fatalf("expected followed=1 after re-follow, got %d", artists[0].Followed)
	}
}

func TestArtistLocalTracks_NoDuplicatesFromMultipleAlbums(t *testing.T) {
	db := openTestDB(t)
	artistID := ensureArtist(t, db, "Alanis Morissette")

	// Insert a file with album "Jagged Little Pill".
	fp := testFileParams("a/track1.flac", "Alanis Morissette", "Jagged Little Pill", "All I Really Want",
		func(p *UpsertFileParams) { p.TrackNumber = 1; p.ArtistID = artistID })
	if err := db.Q.UpsertFile(bg, fp); err != nil {
		t.Fatalf("UpsertFile: %v", err)
	}

	// Insert multiple albums that normalize to the same title_norm.
	albumNorm := Normalize("Jagged Little Pill")
	for _, title := range []string{"Jagged Little Pill", "Jagged Little Pill (acoustic)", "Jagged Little Pill (Original Broadway Cast Recording)"} {
		testUpsertAlbum(t, db, artistID, UpsertAlbumParams{
			Title: title, TitleNorm: albumNorm, ReleaseDate: "1995-06-09",
		})
	}

	rows, err := db.Q.ArtistLocalTracks(bg, artistID)
	if err != nil {
		t.Fatalf("ArtistLocalTracks: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 track, got %d (duplicate rows from JOIN)", len(rows))
	}
	if rows[0].Title != "All I Really Want" {
		t.Fatalf("unexpected title: %s", rows[0].Title)
	}
}
