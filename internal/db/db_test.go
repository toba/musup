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

// testUpsertAlbum is a test helper that calls UpsertAlbum and fails on error.
func testUpsertAlbum(t *testing.T, db *DB, artistID int64, p UpsertAlbumParams) int64 {
	t.Helper()
	p.ArtistID = artistID
	if p.TitleNorm == "" {
		p.TitleNorm = Normalize(p.Title)
	}
	id, err := db.UpsertAlbum(artistID, p)
	if err != nil {
		t.Fatalf("UpsertAlbum(%q): %v", p.Title, err)
	}
	return id
}

// testUpsertTrack is a test helper that calls Q.UpsertTrack and fails on error.
func testUpsertTrack(t *testing.T, db *DB, albumID int64, p UpsertTrackParams) {
	t.Helper()
	p.AlbumID = albumID
	if p.TitleNorm == "" {
		p.TitleNorm = Normalize(p.Title)
	}
	if err := db.Q.UpsertTrack(bg, p); err != nil {
		t.Fatalf("UpsertTrack(%q): %v", p.Title, err)
	}
}

// testUpsertArtist is a test helper that calls EnsureArtist + UpdateArtistFull.
func testUpsertArtist(t *testing.T, db *DB, name, mbid string, lastCheckedAt time.Time) {
	t.Helper()
	id, err := db.EnsureArtist(name)
	if err != nil {
		t.Fatalf("EnsureArtist(%q): %v", name, err)
	}
	err = db.Q.UpdateArtistFull(bg, mbid, lastCheckedAt.Format(time.RFC3339), 0, id)
	if err != nil {
		t.Fatalf("UpdateArtistFull(%q): %v", name, err)
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

func TestUniqueArtists(t *testing.T) {
	db := openTestDB(t)

	files := []UpsertFileParams{
		testFileParams("a/1.flac", "Zed", "Z", ""),
		testFileParams("b/2.flac", "Alpha", "A", ""),
		testFileParams("c/3.flac", "Alpha", "B", ""),
		testFileParams("d/4.flac", "", "", "", func(f *UpsertFileParams) { f.IsAlbumArtist = 0 }),
	}
	for _, f := range files {
		if err := db.Q.UpsertFile(bg, f); err != nil {
			t.Fatalf("UpsertFile: %v", err)
		}
	}

	artists, err := db.Q.UniqueArtists(bg)
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

	files := []UpsertFileParams{
		testFileParams("a/1.flac", "A", "X", ""),
		testFileParams("a/2.flac", "A", "Y", ""),
		testFileParams("a/3.flac", "A", "X", ""),
	}
	for _, f := range files {
		if err := db.Q.UpsertFile(bg, f); err != nil {
			t.Fatalf("UpsertFile: %v", err)
		}
	}

	albums, err := db.Q.LocalAlbums(bg, Normalize("A"))
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

func TestArtistSummaries(t *testing.T) {
	db := openTestDB(t)

	files := []UpsertFileParams{
		testFileParams("a/1.flac", "Zed", "Zebra", ""),
		testFileParams("b/2.flac", "Alpha", "Apples", ""),
		testFileParams("c/3.flac", "Alpha", "Bananas", ""),
		testFileParams("d/4.flac", "Alpha", "Apples", ""),
		testFileParams("e/5.flac", "", "", "", func(f *UpsertFileParams) { f.IsAlbumArtist = 0 }),
	}
	for _, f := range files {
		if err := db.Q.UpsertFile(bg, f); err != nil {
			t.Fatalf("UpsertFile: %v", err)
		}
	}

	summaries, err := db.Q.ArtistSummaries(bg)
	if err != nil {
		t.Fatalf("ArtistSummaries: %v", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("expected 2 summaries, got %d", len(summaries))
	}

	if summaries[0].Name != "Alpha" {
		t.Fatalf("expected first artist Alpha, got %q", summaries[0].Name)
	}
	if summaries[0].AlbumCnt != 2 {
		t.Fatalf("expected Alpha to have 2 albums, got %d", summaries[0].AlbumCnt)
	}
	if summaries[0].Newest == "" {
		t.Fatal("expected Alpha to have a newest album")
	}
	if summaries[0].TrackCnt != 3 {
		t.Fatalf("expected Alpha to have 3 tracks, got %d", summaries[0].TrackCnt)
	}
	if summaries[0].Mbid != "" {
		t.Fatal("expected Alpha to not be synced")
	}

	if summaries[1].Name != "Zed" {
		t.Fatalf("expected second artist Zed, got %q", summaries[1].Name)
	}
	if summaries[1].AlbumCnt != 1 {
		t.Fatalf("expected Zed to have 1 album, got %d", summaries[1].AlbumCnt)
	}
	if summaries[1].TrackCnt != 1 {
		t.Fatalf("expected Zed to have 1 track, got %d", summaries[1].TrackCnt)
	}
	if summaries[1].Mbid != "" {
		t.Fatal("expected Zed to not be synced")
	}
}

func TestArtistSummaries_Synced(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().Truncate(time.Second)

	p := testFileParams("a/1.flac", "Radiohead", "OK Computer", "")
	if err := db.Q.UpsertFile(bg, p); err != nil {
		t.Fatalf("UpsertFile: %v", err)
	}

	summaries, err := db.Q.ArtistSummaries(bg)
	if err != nil {
		t.Fatalf("ArtistSummaries: %v", err)
	}
	if summaries[0].Mbid != "" {
		t.Fatal("expected not synced before artist record")
	}

	testUpsertArtist(t, db, "Radiohead", "abc-123", now)

	summaries, err = db.Q.ArtistSummaries(bg)
	if err != nil {
		t.Fatalf("ArtistSummaries: %v", err)
	}
	if summaries[0].Mbid == "" {
		t.Fatal("expected synced after artist record with MBID")
	}
}

func TestArtistSummaries_Empty(t *testing.T) {
	db := openTestDB(t)

	summaries, err := db.Q.ArtistSummaries(bg)
	if err != nil {
		t.Fatalf("ArtistSummaries: %v", err)
	}
	if len(summaries) != 0 {
		t.Fatalf("expected 0 summaries, got %d", len(summaries))
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

	got, err := db.Q.ListAlbumsByArtist(bg, Normalize("Radiohead"))
	if err != nil {
		t.Fatalf("ListAlbumsByArtist: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 albums, got %d", len(got))
	}
	if got[0].Title != "OK Computer" {
		t.Fatalf("expected oldest album first, got %q", got[0].Title)
	}
	if got[2].Title != "A Moon Shaped Pool" {
		t.Fatalf("expected newest album last, got %q", got[2].Title)
	}
}

func TestMarkLocalTracks(t *testing.T) {
	db := openTestDB(t)

	files := []UpsertFileParams{
		testFileParams("a/1.flac", "Radiohead", "OK Computer", "Airbag"),
		testFileParams("a/2.flac", "Radiohead", "OK Computer", "Paranoid Android"),
		testFileParams("a/3.flac", "Radiohead", "Kid A", "Everything in Its Right Place"),
	}
	for _, f := range files {
		if err := db.Q.UpsertFile(bg, f); err != nil {
			t.Fatalf("UpsertFile: %v", err)
		}
	}

	artistID := ensureArtist(t, db, "Radiohead")
	okID := testUpsertAlbum(t, db, artistID, UpsertAlbumParams{Title: "OK Computer"})
	kidID := testUpsertAlbum(t, db, artistID, UpsertAlbumParams{Title: "Kid A"})
	amID := testUpsertAlbum(t, db, artistID, UpsertAlbumParams{Title: "Amnesiac"})

	tracks := []struct {
		albumID int64
		tr      UpsertTrackParams
	}{
		{okID, UpsertTrackParams{Title: "Airbag", Position: 1}},
		{okID, UpsertTrackParams{Title: "Paranoid Android", Position: 2}},
		{okID, UpsertTrackParams{Title: "Subterranean Homesick Alien", Position: 3}},
		{kidID, UpsertTrackParams{Title: "Everything in Its Right Place", Position: 1}},
		{kidID, UpsertTrackParams{Title: "Kid A", Position: 2}},
		{amID, UpsertTrackParams{Title: "Packt Like Sardines in a Crushd Tin Box", Position: 1}},
	}
	for _, tt := range tracks {
		testUpsertTrack(t, db, tt.albumID, tt.tr)
	}

	if err := db.MarkLocalTracks("Radiohead"); err != nil {
		t.Fatalf("MarkLocalTracks: %v", err)
	}

	okTracks, err := db.Q.ListTracksByAlbum(bg, Normalize("Radiohead"), "OK Computer")
	if err != nil {
		t.Fatalf("ListTracksByAlbum: %v", err)
	}
	localCount := 0
	for _, tr := range okTracks {
		if tr.Local == 1 {
			localCount++
		}
	}
	if localCount != 2 {
		t.Fatalf("expected 2 local OK Computer tracks, got %d", localCount)
	}

	amTracks, err := db.Q.ListTracksByAlbum(bg, Normalize("Radiohead"), "Amnesiac")
	if err != nil {
		t.Fatalf("ListTracksByAlbum: %v", err)
	}
	for _, tr := range amTracks {
		if tr.Local == 1 {
			t.Fatal("Amnesiac track should not be local")
		}
	}
}

func TestMarkLocalTracks_FuzzyTitle(t *testing.T) {
	db := openTestDB(t)

	files := []UpsertFileParams{
		testFileParams("a/1.flac", "3 Doors Down", "The Better Life", "Kryptonite", func(f *UpsertFileParams) { f.TrackNumber = 3 }),
		testFileParams("a/2.flac", "3 Doors Down", "The Better Life", "Loser", func(f *UpsertFileParams) { f.TrackNumber = 4 }),
		testFileParams("a/3.flac", "3 Doors Down", "The Better Life", "Be Like That", func(f *UpsertFileParams) { f.TrackNumber = 7 }),
	}
	for _, f := range files {
		if err := db.Q.UpsertFile(bg, f); err != nil {
			t.Fatalf("UpsertFile: %v", err)
		}
	}

	artistID := ensureArtist(t, db, "3 Doors Down")
	albumID := testUpsertAlbum(t, db, artistID, UpsertAlbumParams{Title: "The Better Life"})

	tracks := []UpsertTrackParams{
		{Title: "Kryptonite", Position: 3},
		{Title: "Loser (radio edit)", Position: 4},
		{Title: "Be Like That", Position: 7},
		{Title: "Duck and Run", Position: 5},
		{Title: "By My Side", Position: 6},
	}
	for _, tr := range tracks {
		testUpsertTrack(t, db, albumID, tr)
	}

	if err := db.MarkLocalTracks("3 Doors Down"); err != nil {
		t.Fatalf("MarkLocalTracks: %v", err)
	}

	got, err := db.Q.ListTracksByAlbum(bg, Normalize("3 Doors Down"), "The Better Life")
	if err != nil {
		t.Fatalf("ListTracksByAlbum: %v", err)
	}

	localByTitle := make(map[string]bool)
	for _, tr := range got {
		localByTitle[tr.Title] = tr.Local == 1
	}

	if !localByTitle["Kryptonite"] {
		t.Error("expected Kryptonite to be local (exact match)")
	}
	if !localByTitle["Loser (radio edit)"] {
		t.Error("expected 'Loser (radio edit)' to be local (position match)")
	}
	if !localByTitle["Be Like That"] {
		t.Error("expected Be Like That to be local (exact match)")
	}
	if localByTitle["Duck and Run"] {
		t.Error("expected Duck and Run to NOT be local")
	}
	if localByTitle["By My Side"] {
		t.Error("expected By My Side to NOT be local")
	}
}

func TestMarkLocalTracks_TrackNumberOnly(t *testing.T) {
	db := openTestDB(t)

	files := []UpsertFileParams{
		testFileParams("a/06.flac", "10,000 Maniacs", "The Earth Pressed Flat", "", func(f *UpsertFileParams) { f.TrackNumber = 6 }),
		testFileParams("a/11.flac", "10,000 Maniacs", "The Earth Pressed Flat", "", func(f *UpsertFileParams) { f.TrackNumber = 11 }),
	}
	for _, f := range files {
		if err := db.Q.UpsertFile(bg, f); err != nil {
			t.Fatalf("UpsertFile: %v", err)
		}
	}

	artistID := ensureArtist(t, db, "10,000 Maniacs")
	albumID := testUpsertAlbum(t, db, artistID, UpsertAlbumParams{Title: "The Earth Pressed Flat"})

	tracks := []UpsertTrackParams{
		{Title: "Somebody's Heaven", Position: 6},
		{Title: "Time Turns", Position: 11},
		{Title: "Ellen", Position: 2},
	}
	for _, tr := range tracks {
		testUpsertTrack(t, db, albumID, tr)
	}

	if err := db.MarkLocalTracks("10,000 Maniacs"); err != nil {
		t.Fatalf("MarkLocalTracks: %v", err)
	}

	got, err := db.Q.ListTracksByAlbum(bg, Normalize("10,000 Maniacs"), "The Earth Pressed Flat")
	if err != nil {
		t.Fatalf("ListTracksByAlbum: %v", err)
	}

	localCount := 0
	for _, tr := range got {
		if tr.Local == 1 {
			localCount++
		}
	}
	if localCount != 2 {
		t.Fatalf("expected 2 local tracks, got %d", localCount)
	}
}

func TestMarkLocalTracks_ClearsStale(t *testing.T) {
	db := openTestDB(t)

	p := testFileParams("a/1.flac", "A", "X", "Song")
	if err := db.Q.UpsertFile(bg, p); err != nil {
		t.Fatalf("UpsertFile: %v", err)
	}

	artistID := ensureArtist(t, db, "A")
	albumID := testUpsertAlbum(t, db, artistID, UpsertAlbumParams{Title: "X"})

	testUpsertTrack(t, db, albumID, UpsertTrackParams{Title: "Song", Position: 1, Local: 1})

	if _, err := db.RemoveStaleFiles(map[string]struct{}{}); err != nil {
		t.Fatalf("RemoveStaleFiles: %v", err)
	}

	if err := db.MarkLocalTracks("A"); err != nil {
		t.Fatalf("MarkLocalTracks: %v", err)
	}

	tracks, err := db.Q.ListTracksByAlbum(bg, Normalize("A"), "X")
	if err != nil {
		t.Fatalf("ListTracksByAlbum: %v", err)
	}
	if len(tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(tracks))
	}
	if tracks[0].Local == 1 {
		t.Fatal("expected Local == 0 after file removed")
	}
}

func TestUpsertTrackAndQuery(t *testing.T) {
	db := openTestDB(t)

	artistID := ensureArtist(t, db, "Radiohead")
	albumID := testUpsertAlbum(t, db, artistID, UpsertAlbumParams{Title: "OK Computer"})

	tracks := []UpsertTrackParams{
		{Title: "Paranoid Android", Position: 2, Mbid: "aaa", LengthMs: 383000},
		{Title: "Airbag", Position: 1, Mbid: "bbb", LengthMs: 284000},
		{Title: "Lucky", Position: 3, Mbid: "ccc", LengthMs: 258000},
	}
	for _, tr := range tracks {
		testUpsertTrack(t, db, albumID, tr)
	}

	got, err := db.Q.ListTracksByAlbum(bg, Normalize("Radiohead"), "OK Computer")
	if err != nil {
		t.Fatalf("ListTracksByAlbum: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 tracks, got %d", len(got))
	}
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
	albumID := testUpsertAlbum(t, db, artistID, UpsertAlbumParams{Title: "OK Computer"})

	tr := UpsertTrackParams{Title: "Airbag", Position: 1, Mbid: "aaa", LengthMs: 284000}
	testUpsertTrack(t, db, albumID, tr)

	tr.Position = 5
	tr.LengthMs = 300000
	testUpsertTrack(t, db, albumID, tr)

	got, err := db.Q.ListTracksByAlbum(bg, Normalize("Radiohead"), "OK Computer")
	if err != nil {
		t.Fatalf("ListTracksByAlbum: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 track after upsert, got %d", len(got))
	}
	if got[0].Position != 5 {
		t.Fatalf("expected position 5, got %d", got[0].Position)
	}
	if got[0].LengthMs != 300000 {
		t.Fatalf("expected length 300000, got %d", got[0].LengthMs)
	}
}

func TestAlbumsWithTrackCounts(t *testing.T) {
	db := openTestDB(t)

	artistID := ensureArtist(t, db, "Radiohead")

	okID := testUpsertAlbum(t, db, artistID, UpsertAlbumParams{Title: "OK Computer", Mbid: "aaa", ReleaseDate: "1997-05-21", PrimaryType: "Album"})
	testUpsertAlbum(t, db, artistID, UpsertAlbumParams{Title: "Kid A", Mbid: "bbb", ReleaseDate: "2000-10-02", PrimaryType: "Album"})

	testUpsertTrack(t, db, okID, UpsertTrackParams{Title: "Airbag", Position: 1, Local: 1})
	testUpsertTrack(t, db, okID, UpsertTrackParams{Title: "Paranoid Android", Position: 2, Local: 1})
	testUpsertTrack(t, db, okID, UpsertTrackParams{Title: "Lucky", Position: 3})

	got, err := db.Q.ListAlbumsByArtist(bg, Normalize("Radiohead"))
	if err != nil {
		t.Fatalf("ListAlbumsByArtist: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 albums, got %d", len(got))
	}

	if got[0].Title != "OK Computer" {
		t.Fatalf("expected OK Computer first, got %q", got[0].Title)
	}
	if got[0].TotalTracks != 3 {
		t.Fatalf("expected 3 total tracks, got %d", got[0].TotalTracks)
	}
	if got[0].LocalTracks != 2 {
		t.Fatalf("expected 2 local tracks, got %d", got[0].LocalTracks)
	}

	if got[1].Title != "Kid A" {
		t.Fatalf("expected Kid A second, got %q", got[1].Title)
	}
	if got[1].TotalTracks != 0 || got[1].LocalTracks != 0 {
		t.Fatalf("Kid A should have 0 tracks, got total=%d local=%d", got[1].TotalTracks, got[1].LocalTracks)
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

	var version int
	if err := db.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if version != 12 {
		t.Fatalf("expected user_version 12, got %d", version)
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
	if version != 12 {
		t.Fatalf("expected user_version 12, got %d", version)
	}
}

func TestRemoveStaleFiles(t *testing.T) {
	db := openTestDB(t)

	files := []UpsertFileParams{
		testFileParams("a/1.flac", "A", "X", ""),
		testFileParams("b/2.flac", "B", "Y", ""),
		testFileParams("c/3.flac", "C", "Z", ""),
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

	artists, err := db.Q.UniqueArtists(bg)
	if err != nil {
		t.Fatalf("UniqueArtists: %v", err)
	}
	if len(artists) != 1 || artists[0] != "A" {
		t.Fatalf("expected [A], got %v", artists)
	}
}

func TestMarkLocalTracks_NormalizedTitle(t *testing.T) {
	db := openTestDB(t)

	files := []UpsertFileParams{
		testFileParams("a/1.flac", "Beck", "Mellow Gold", "Loser", func(f *UpsertFileParams) { f.TrackNumber = 1 }),
	}
	for _, f := range files {
		if err := db.Q.UpsertFile(bg, f); err != nil {
			t.Fatalf("UpsertFile: %v", err)
		}
	}

	artistID := ensureArtist(t, db, "Beck")
	albumID := testUpsertAlbum(t, db, artistID, UpsertAlbumParams{Title: "Mellow Gold"})

	tracks := []UpsertTrackParams{
		{Title: "Loser (radio edit)", Position: 1},
		{Title: "Pay No Mind", Position: 2},
	}
	for _, tr := range tracks {
		testUpsertTrack(t, db, albumID, tr)
	}

	if err := db.MarkLocalTracks("Beck"); err != nil {
		t.Fatalf("MarkLocalTracks: %v", err)
	}

	got, err := db.Q.ListTracksByAlbum(bg, Normalize("Beck"), "Mellow Gold")
	if err != nil {
		t.Fatalf("ListTracksByAlbum: %v", err)
	}

	localByTitle := make(map[string]bool)
	for _, tr := range got {
		localByTitle[tr.Title] = tr.Local == 1
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

	files := []UpsertFileParams{
		testFileParams("a/1.flac", "3 Doors Down", "Away From The Sun", "When I'm Gone", func(f *UpsertFileParams) { f.TrackNumber = 1 }),
	}
	for _, f := range files {
		if err := db.Q.UpsertFile(bg, f); err != nil {
			t.Fatalf("UpsertFile: %v", err)
		}
	}

	artistID := ensureArtist(t, db, "3 Doors Down")
	albumID := testUpsertAlbum(t, db, artistID, UpsertAlbumParams{Title: "Away from the Sun"})

	tracks := []UpsertTrackParams{
		{Title: "When I'm Gone", Position: 1},
	}
	for _, tr := range tracks {
		testUpsertTrack(t, db, albumID, tr)
	}

	if err := db.MarkLocalTracks("3 Doors Down"); err != nil {
		t.Fatalf("MarkLocalTracks: %v", err)
	}

	got, err := db.Q.ListTracksByAlbum(bg, Normalize("3 Doors Down"), "Away from the Sun")
	if err != nil {
		t.Fatalf("ListTracksByAlbum: %v", err)
	}
	if len(got) != 1 || got[0].Local != 1 {
		t.Error("expected track to be local despite album casing difference")
	}
}

func TestKnownAlbumMBIDs(t *testing.T) {
	db := openTestDB(t)
	artistID := ensureArtist(t, db, "Radiohead")

	okID := testUpsertAlbum(t, db, artistID, UpsertAlbumParams{Title: "OK Computer", Mbid: "rg-okc", ReleaseDate: "1997-05-21", PrimaryType: "Album"})
	testUpsertTrack(t, db, okID, UpsertTrackParams{Title: "Airbag", Position: 1, Mbid: "tr-1"})

	testUpsertAlbum(t, db, artistID, UpsertAlbumParams{Title: "Kid A", Mbid: "rg-kida", ReleaseDate: "2000-10-02", PrimaryType: "Album"})

	amID := testUpsertAlbum(t, db, artistID, UpsertAlbumParams{Title: "Amnesiac", Mbid: "", ReleaseDate: "2001-06-05", PrimaryType: "Album"})
	testUpsertTrack(t, db, amID, UpsertTrackParams{Title: "Packt", Position: 1})

	beckID := ensureArtist(t, db, "Beck")
	mgID := testUpsertAlbum(t, db, beckID, UpsertAlbumParams{Title: "Mellow Gold", Mbid: "rg-mg", ReleaseDate: "1994-03-01", PrimaryType: "Album"})
	testUpsertTrack(t, db, mgID, UpsertTrackParams{Title: "Loser", Position: 1, Mbid: "tr-2"})

	mbids, err := db.Q.ListKnownAlbumMBIDs(bg, Normalize("Radiohead"))
	if err != nil {
		t.Fatalf("ListKnownAlbumMBIDs: %v", err)
	}
	known := make(map[string]struct{}, len(mbids))
	for _, m := range mbids {
		known[m] = struct{}{}
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

	files := []UpsertFileParams{
		testFileParams("a/1.flac", "Beck", "Greatest Hits", "Loser", func(f *UpsertFileParams) { f.TrackNumber = 1 }),
	}
	for _, f := range files {
		if err := db.Q.UpsertFile(bg, f); err != nil {
			t.Fatalf("UpsertFile: %v", err)
		}
	}

	artistID := ensureArtist(t, db, "Beck")
	albumID := testUpsertAlbum(t, db, artistID, UpsertAlbumParams{Title: "Mellow Gold"})

	tracks := []UpsertTrackParams{
		{Title: "Loser", Position: 1},
		{Title: "Pay No Mind", Position: 2},
	}
	for _, tr := range tracks {
		testUpsertTrack(t, db, albumID, tr)
	}

	if err := db.MarkLocalTracks("Beck"); err != nil {
		t.Fatalf("MarkLocalTracks: %v", err)
	}

	got, err := db.Q.ListTracksByAlbum(bg, Normalize("Beck"), "Mellow Gold")
	if err != nil {
		t.Fatalf("ListTracksByAlbum: %v", err)
	}

	localByTitle := make(map[string]bool)
	for _, tr := range got {
		localByTitle[tr.Title] = tr.Local == 1
	}

	if !localByTitle["Loser"] {
		t.Error("expected 'Loser' to match cross-album via tier 2 (title-only)")
	}
	if localByTitle["Pay No Mind"] {
		t.Error("expected 'Pay No Mind' to NOT be local")
	}
}

func TestGetSetFollowed(t *testing.T) {
	db := openTestDB(t)

	followed, err := db.IsFollowed("Unknown Artist")
	if err != nil {
		t.Fatalf("IsFollowed unknown: %v", err)
	}
	if !followed {
		t.Fatal("expected unknown artist to be followed by default")
	}

	if err := db.SetFollowed("Test Artist", false); err != nil {
		t.Fatalf("SetFollowed false: %v", err)
	}
	followed, err = db.IsFollowed("Test Artist")
	if err != nil {
		t.Fatalf("IsFollowed after set false: %v", err)
	}
	if followed {
		t.Fatal("expected not followed after SetFollowed(false)")
	}

	if err := db.SetFollowed("Test Artist", true); err != nil {
		t.Fatalf("SetFollowed true: %v", err)
	}
	followed, err = db.IsFollowed("Test Artist")
	if err != nil {
		t.Fatalf("IsFollowed after set true: %v", err)
	}
	if !followed {
		t.Fatal("expected followed after SetFollowed(true)")
	}

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

func TestAlbums_SecondaryTypes(t *testing.T) {
	db := openTestDB(t)

	artistID := ensureArtist(t, db, "Radiohead")
	testUpsertAlbum(t, db, artistID, UpsertAlbumParams{
		Title:          "OK Computer OKNOTOK",
		Mbid:           "aaa",
		ReleaseDate:    "2017-06-23",
		PrimaryType:    "Album",
		SecondaryTypes: "Compilation",
	})

	got, err := db.Q.ListAlbumsByArtist(bg, Normalize("Radiohead"))
	if err != nil {
		t.Fatalf("ListAlbumsByArtist: %v", err)
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

	p := testFileParams("a/1.flac", "Radiohead", "OK Computer", "Airbag", func(f *UpsertFileParams) { f.TrackNumber = 1 })
	if err := db.Q.UpsertFile(bg, p); err != nil {
		t.Fatalf("UpsertFile: %v", err)
	}

	artistID := ensureArtist(t, db, "Radiohead")
	albumID := testUpsertAlbum(t, db, artistID, UpsertAlbumParams{Title: "OK Computer"})

	testUpsertTrack(t, db, albumID, UpsertTrackParams{Title: "Airbag", Position: 1, Local: 1})

	if err := db.MarkLocalTracks("Radiohead"); err != nil {
		t.Fatalf("MarkLocalTracks: %v", err)
	}

	tracks, err := db.Q.ListTracksByAlbum(bg, Normalize("Radiohead"), "OK Computer")
	if err != nil {
		t.Fatalf("ListTracksByAlbum: %v", err)
	}
	if len(tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(tracks))
	}
	if tracks[0].Local != 1 {
		t.Fatal("expected Local == 1 for matched track")
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

func TestArtistSummaries_HasNew(t *testing.T) {
	db := openTestDB(t)

	for _, f := range []UpsertFileParams{
		testFileParams("a/1.flac", "Radiohead", "OK Computer", "Airbag"),
		testFileParams("b/1.flac", "Beck", "Mellow Gold", "Loser"),
		testFileParams("c/1.flac", "Bjork", "Debut", "Human Behaviour"),
	} {
		if err := db.Q.UpsertFile(bg, f); err != nil {
			t.Fatalf("UpsertFile: %v", err)
		}
	}

	rhID := ensureArtist(t, db, "Radiohead")
	okID := testUpsertAlbum(t, db, rhID, UpsertAlbumParams{Title: "OK Computer", Mbid: "aaa", ReleaseDate: "1997-05-21", PrimaryType: "Album"})
	testUpsertAlbum(t, db, rhID, UpsertAlbumParams{Title: "A Moon Shaped Pool", Mbid: "bbb", ReleaseDate: "2016-05-08", PrimaryType: "Album"})
	testUpsertTrack(t, db, okID, UpsertTrackParams{Title: "Airbag", Position: 1, Local: 1})

	beckID := ensureArtist(t, db, "Beck")
	mgID := testUpsertAlbum(t, db, beckID, UpsertAlbumParams{Title: "Mellow Gold", Mbid: "ccc", ReleaseDate: "1994-03-01", PrimaryType: "Album"})
	testUpsertTrack(t, db, mgID, UpsertTrackParams{Title: "Loser", Position: 1, Local: 1})

	summaries, err := db.Q.ArtistSummaries(bg)
	if err != nil {
		t.Fatalf("ArtistSummaries: %v", err)
	}
	if len(summaries) != 3 {
		t.Fatalf("expected 3 summaries, got %d", len(summaries))
	}

	byName := make(map[string]ArtistSummariesRow)
	for _, s := range summaries {
		byName[s.Name] = s
	}

	if byName["Radiohead"].HasNew != 1 {
		t.Error("expected Radiohead HasNew = 1 (catalog album 2016 > local 1997)")
	}
	if byName["Beck"].HasNew != 0 {
		t.Error("expected Beck HasNew = 0 (no catalog albums newer than local)")
	}
	if byName["Bjork"].HasNew != 0 {
		t.Error("expected Bjork HasNew = 0 (not synced)")
	}
}

func TestArtistSummaries_LatestDateFromAlbums(t *testing.T) {
	db := openTestDB(t)

	// Create a file so the artist appears in summaries.
	if err := db.Q.UpsertFile(bg, testFileParams("a/1.flac", "Radiohead", "OK Computer", "Airbag")); err != nil {
		t.Fatalf("UpsertFile: %v", err)
	}

	// Create an artist with synced albums — latest_date should be derived from albums.
	rhID := ensureArtist(t, db, "Radiohead")
	okID := testUpsertAlbum(t, db, rhID, UpsertAlbumParams{Title: "OK Computer", Mbid: "aaa", ReleaseDate: "1997-05-21", PrimaryType: "Album"})
	testUpsertAlbum(t, db, rhID, UpsertAlbumParams{Title: "A Moon Shaped Pool", Mbid: "bbb", ReleaseDate: "2016-05-08", PrimaryType: "Album"})
	testUpsertTrack(t, db, okID, UpsertTrackParams{Title: "Airbag", Position: 1, Local: 1})

	summaries, err := db.Q.ArtistSummaries(bg)
	if err != nil {
		t.Fatalf("ArtistSummaries: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}

	// The summary should still report 2016-05-08 derived from albums.release_date.
	if summaries[0].LatestDate != "2016-05-08" {
		t.Errorf("expected LatestDate %q, got %q", "2016-05-08", summaries[0].LatestDate)
	}
}

func TestArtistSummaries_FollowedField(t *testing.T) {
	db := openTestDB(t)

	p := testFileParams("a/1.flac", "Radiohead", "OK Computer", "")
	if err := db.Q.UpsertFile(bg, p); err != nil {
		t.Fatalf("UpsertFile: %v", err)
	}

	summaries, err := db.Q.ArtistSummaries(bg)
	if err != nil {
		t.Fatalf("ArtistSummaries (default): %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}
	if summaries[0].Followed != 1 {
		t.Fatal("expected Followed=1 by default")
	}

	if err := db.SetFollowed("Radiohead", false); err != nil {
		t.Fatalf("SetFollowed: %v", err)
	}
	summaries, err = db.Q.ArtistSummaries(bg)
	if err != nil {
		t.Fatalf("ArtistSummaries (after unfollow): %v", err)
	}
	if summaries[0].Followed != 0 {
		t.Fatal("expected Followed=0 after unfollow")
	}

	if err := db.SetFollowed("Radiohead", true); err != nil {
		t.Fatalf("SetFollowed: %v", err)
	}
	summaries, err = db.Q.ArtistSummaries(bg)
	if err != nil {
		t.Fatalf("ArtistSummaries (after re-follow): %v", err)
	}
	if summaries[0].Followed != 1 {
		t.Fatal("expected Followed=1 after re-follow")
	}
}

func TestAlbums_DeduplicatesArtistNameVariants(t *testing.T) {
	db := openTestDB(t)

	artistID := ensureArtist(t, db, "Asaf Avidan")

	gsID := testUpsertAlbum(t, db, artistID, UpsertAlbumParams{Title: "Gold Shadow", Mbid: "aaa", ReleaseDate: "2015-01-26", PrimaryType: "Album"})
	testUpsertAlbum(t, db, artistID, UpsertAlbumParams{Title: "Anagnorisis", Mbid: "bbb", ReleaseDate: "2020-07-03", PrimaryType: "Album"})

	artistID2 := ensureArtist(t, db, "asaf avidan")
	if artistID2 != artistID {
		t.Fatalf("expected same artist ID for different casing, got %d vs %d", artistID, artistID2)
	}

	testUpsertTrack(t, db, gsID, UpsertTrackParams{Title: "Over My Head", Position: 1, Local: 1})
	testUpsertTrack(t, db, gsID, UpsertTrackParams{Title: "My Old Pain", Position: 2})

	got, err := db.Q.ListAlbumsByArtist(bg, Normalize("Asaf Avidan"))
	if err != nil {
		t.Fatalf("ListAlbumsByArtist: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 unique albums, got %d", len(got))
	}

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

	files := []UpsertFileParams{
		testFileParams("a/1.flac", "Alice In Chains", "Dirt", "Rooster"),
		testFileParams("a/2.flac", "Alice In Chains", "Dirt", "Down in a Hole"),
		testFileParams("a/3.flac", "Alice In Chains", "Jar of Flies", "No Excuses"),
		testFileParams("b/1.flac", "Alice in Chains", "Facelift", "Man in the Box"),
	}
	for _, f := range files {
		if err := db.Q.UpsertFile(bg, f); err != nil {
			t.Fatalf("UpsertFile: %v", err)
		}
	}

	summaries, err := db.Q.ArtistSummaries(bg)
	if err != nil {
		t.Fatalf("ArtistSummaries: %v", err)
	}

	if len(summaries) != 1 {
		names := make([]string, len(summaries))
		for i, s := range summaries {
			names[i] = fmt.Sprintf("%q (tracks=%d)", s.Name, s.TrackCnt)
		}
		t.Fatalf("expected 1 merged artist, got %d: %v", len(summaries), names)
	}

	s := summaries[0]
	if s.Name != "Alice In Chains" {
		t.Errorf("expected canonical name %q, got %q", "Alice In Chains", s.Name)
	}
	if s.TrackCnt != 4 {
		t.Errorf("expected 4 tracks, got %d", s.TrackCnt)
	}
	if s.AlbumCnt != 3 {
		t.Errorf("expected 3 albums (Dirt, Jar of Flies, Facelift), got %d", s.AlbumCnt)
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

func TestTrackCounts_ConsistentBetweenSummariesAndAlbums(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().Truncate(time.Second)

	p := testFileParams("a/1.flac", "Amy Lee", "Recover", "Use My Voice")
	if err := db.Q.UpsertFile(bg, p); err != nil {
		t.Fatalf("UpsertFile: %v", err)
	}

	testUpsertArtist(t, db, "Amy Lee", "mbid-amylee", now)
	artistID := ensureArtist(t, db, "Amy Lee")

	a1ID := testUpsertAlbum(t, db, artistID, UpsertAlbumParams{
		Title: "Recover", Mbid: "aaa", ReleaseDate: "2023-04-14", PrimaryType: "Album",
	})
	for i, title := range []string{"Use My Voice", "Blind Faith", "Love Exists"} {
		testUpsertTrack(t, db, a1ID, UpsertTrackParams{Title: title, Position: int64(i + 1)})
	}

	a2ID := testUpsertAlbum(t, db, artistID, UpsertAlbumParams{
		Title: "Dream Too Much", Mbid: "bbb", ReleaseDate: "2016-09-30", PrimaryType: "Album",
	})
	for i, title := range []string{"I'm Not Tired", "Rubber Duckie"} {
		testUpsertTrack(t, db, a2ID, UpsertTrackParams{Title: title, Position: int64(i + 1)})
	}

	if err := db.MarkLocalTracks("Amy Lee"); err != nil {
		t.Fatalf("MarkLocalTracks: %v", err)
	}

	summaries, err := db.Q.ArtistSummaries(bg)
	if err != nil {
		t.Fatalf("ArtistSummaries: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}
	if summaries[0].TotalTracks != 5 {
		t.Fatalf("ArtistSummaries: expected 5 total tracks, got %d", summaries[0].TotalTracks)
	}

	albums, err := db.Q.ListAlbumsByArtist(bg, Normalize("Amy Lee"))
	if err != nil {
		t.Fatalf("ListAlbumsByArtist: %v", err)
	}

	var albumLocalSum int64
	for _, a := range albums {
		albumLocalSum += a.LocalTracks
	}

	s := summaries[0]
	effectiveTrackCount := s.TrackCnt
	if s.Mbid != "" && s.LocalTracks > effectiveTrackCount {
		effectiveTrackCount = s.LocalTracks
	}
	if effectiveTrackCount != albumLocalSum {
		t.Fatalf("effective TrackCount (%d) != sum of album LocalTracks (%d)",
			effectiveTrackCount, albumLocalSum)
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
	if version != 12 {
		t.Fatalf("expected version 12, got %d", version)
	}

	var artistCount int
	if err := db.db.QueryRow("SELECT COUNT(*) FROM artists").Scan(&artistCount); err != nil {
		t.Fatalf("count artists: %v", err)
	}
	if artistCount != 1 {
		t.Fatalf("expected 1 artist after dedup, got %d", artistCount)
	}

	albums, err := db.Q.ListAlbumsByArtist(bg, Normalize("Radiohead"))
	if err != nil {
		t.Fatalf("ListAlbumsByArtist: %v", err)
	}
	if len(albums) != 2 {
		t.Fatalf("expected 2 albums (OK Computer, Kid A), got %d", len(albums))
	}

	tracks, err := db.Q.ListTracksByAlbum(bg, Normalize("Radiohead"), "OK Computer")
	if err != nil {
		t.Fatalf("ListTracksByAlbum: %v", err)
	}
	if len(tracks) != 2 {
		t.Fatalf("expected 2 tracks, got %d", len(tracks))
	}
	if tracks[0].Local != 1 {
		t.Error("expected Airbag to remain local after migration")
	}
}

func TestPruneUnfollowed(t *testing.T) {
	db := openTestDB(t)

	for _, f := range []UpsertFileParams{
		testFileParams("a/1.flac", "Kept", "A", "T1"),
		testFileParams("b/1.flac", "Pruned", "B", "T2"),
	} {
		if err := db.Q.UpsertFile(bg, f); err != nil {
			t.Fatalf("UpsertFile: %v", err)
		}
	}

	keptID := ensureArtist(t, db, "Kept")
	prunedID := ensureArtist(t, db, "Pruned")

	kaID := testUpsertAlbum(t, db, keptID, UpsertAlbumParams{Title: "A", Mbid: "aaa"})
	testUpsertTrack(t, db, kaID, UpsertTrackParams{Title: "T1", Position: 1})

	paID := testUpsertAlbum(t, db, prunedID, UpsertAlbumParams{Title: "B", Mbid: "bbb"})
	testUpsertTrack(t, db, paID, UpsertTrackParams{Title: "T2", Position: 1})
	testUpsertTrack(t, db, paID, UpsertTrackParams{Title: "T3", Position: 2})

	if err := db.SetFollowed("Pruned", false); err != nil {
		t.Fatalf("SetFollowed: %v", err)
	}

	names, err := db.Q.ListUnfollowedArtistNames(bg)
	if err != nil {
		t.Fatalf("ListUnfollowedArtistNames: %v", err)
	}
	if len(names) != 1 || names[0] != "Pruned" {
		t.Fatalf("expected [Pruned], got %v", names)
	}

	result, err := db.PruneUnfollowed()
	if err != nil {
		t.Fatalf("PruneUnfollowed: %v", err)
	}
	if result.Artists != 1 {
		t.Errorf("expected 1 artist pruned, got %d", result.Artists)
	}
	if result.Albums != 1 {
		t.Errorf("expected 1 album pruned, got %d", result.Albums)
	}
	if result.Tracks != 2 {
		t.Errorf("expected 2 tracks pruned, got %d", result.Tracks)
	}

	keptAlbums, err := db.Q.ListAlbumsByArtist(bg, Normalize("Kept"))
	if err != nil {
		t.Fatalf("ListAlbumsByArtist: %v", err)
	}
	if len(keptAlbums) != 1 {
		t.Fatalf("expected 1 album for Kept, got %d", len(keptAlbums))
	}

	_, err = db.Q.GetArtistByNameNorm(bg, Normalize("Pruned"))
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected sql.ErrNoRows for Pruned artist, got %v", err)
	}

	meta, err := db.Q.AllFileMeta(bg)
	if err != nil {
		t.Fatalf("AllFileMeta: %v", err)
	}
	if len(meta) != 2 {
		t.Fatalf("expected 2 files preserved, got %d", len(meta))
	}
}

func TestMarkReviewed(t *testing.T) {
	db := openTestDB(t)

	p := testFileParams("a/1.flac", "Radiohead", "OK Computer", "Airbag")
	if err := db.Q.UpsertFile(bg, p); err != nil {
		t.Fatalf("UpsertFile: %v", err)
	}

	rhID := ensureArtist(t, db, "Radiohead")
	okID := testUpsertAlbum(t, db, rhID, UpsertAlbumParams{Title: "OK Computer", Mbid: "aaa", ReleaseDate: "1997-05-21", PrimaryType: "Album"})
	testUpsertAlbum(t, db, rhID, UpsertAlbumParams{Title: "A Moon Shaped Pool", Mbid: "bbb", ReleaseDate: "2016-05-08", PrimaryType: "Album"})
	testUpsertTrack(t, db, okID, UpsertTrackParams{Title: "Airbag", Position: 1, Local: 1})

	summaries, err := db.Q.ArtistSummaries(bg)
	if err != nil {
		t.Fatalf("ArtistSummaries: %v", err)
	}
	if summaries[0].HasNew != 1 {
		t.Fatal("expected HasNew=1 before MarkReviewed")
	}

	if err := db.MarkReviewed("Radiohead"); err != nil {
		t.Fatalf("MarkReviewed: %v", err)
	}

	summaries, err = db.Q.ArtistSummaries(bg)
	if err != nil {
		t.Fatalf("ArtistSummaries: %v", err)
	}
	if summaries[0].HasNew != 0 {
		t.Fatal("expected HasNew=0 after MarkReviewed")
	}

	testUpsertAlbum(t, db, rhID, UpsertAlbumParams{Title: "New Album", Mbid: "ccc", ReleaseDate: "2025-01-01", PrimaryType: "Album"})
	summaries, err = db.Q.ArtistSummaries(bg)
	if err != nil {
		t.Fatalf("ArtistSummaries: %v", err)
	}
	if summaries[0].HasNew != 1 {
		t.Fatal("expected HasNew=1 after adding album newer than reviewed_at")
	}
}

func TestArtistSummaries_LocalAlbumCountConsistentWithTracks(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().Truncate(time.Second)

	p := testFileParams("a/1.flac", "Alphabet Backwards", "Festivus", "Dearest Santa", func(f *UpsertFileParams) { f.TrackNumber = 6 })
	if err := db.Q.UpsertFile(bg, p); err != nil {
		t.Fatalf("UpsertFile: %v", err)
	}

	artistID := ensureArtist(t, db, "Alphabet Backwards")
	testUpsertArtist(t, db, "Alphabet Backwards", "mbid-ab", now)

	a1ID := testUpsertAlbum(t, db, artistID, UpsertAlbumParams{
		Title: "Little Victories", Mbid: "aaa", ReleaseDate: "2012-10-01", PrimaryType: "Album",
	})
	a2ID := testUpsertAlbum(t, db, artistID, UpsertAlbumParams{
		Title: "Friends, Lovers & Empty Beds", Mbid: "bbb", ReleaseDate: "2018", PrimaryType: "Album",
	})
	for i := range 12 {
		testUpsertTrack(t, db, a1ID, UpsertTrackParams{Title: fmt.Sprintf("Track %d", i+1), Position: int64(i + 1)})
	}
	for i := range 11 {
		testUpsertTrack(t, db, a2ID, UpsertTrackParams{Title: fmt.Sprintf("Track %d", i+13), Position: int64(i + 1)})
	}

	if err := db.MarkLocalTracks("Alphabet Backwards"); err != nil {
		t.Fatalf("MarkLocalTracks: %v", err)
	}

	summaries, err := db.Q.ArtistSummaries(bg)
	if err != nil {
		t.Fatalf("ArtistSummaries: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}

	s := summaries[0]
	effectiveTrackCount := s.TrackCnt
	if s.Mbid != "" && s.LocalTracks > effectiveTrackCount {
		effectiveTrackCount = s.LocalTracks
	}
	if effectiveTrackCount < 1 {
		t.Fatalf("expected at least 1 local track (from files), got %d", effectiveTrackCount)
	}
	effectiveAlbumCount := s.AlbumCnt
	if s.Mbid != "" && s.LocalAlbums > effectiveAlbumCount {
		effectiveAlbumCount = s.LocalAlbums
	}
	if effectiveAlbumCount < 1 {
		t.Fatalf("expected at least 1 local album (from files), got %d", effectiveAlbumCount)
	}
}
