package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// FileRecord represents a scanned music file.
type FileRecord struct {
	Path          string
	Size          int64
	ModTime       time.Time
	Artist        string
	Album         string
	Title         string
	TrackNumber   int
	IsAlbumArtist bool
	ScannedAt     time.Time
}

// AlbumRecord represents an album in the catalog (from MusicBrainz or local).
type AlbumRecord struct {
	ID             int64
	ArtistID       int64
	ArtistName     string
	Title          string
	MBID           string
	ReleaseDate    string
	PrimaryType    string
	SecondaryTypes string // comma-separated, e.g. "Compilation,Live"
	LocalTracks    int
	TotalTracks    int
}

// TrackRecord represents a track in an album.
type TrackRecord struct {
	ID         int64
	AlbumID    int64
	ArtistName string
	AlbumTitle string
	Title      string
	Position   int
	MBID       string
	LengthMS   int
	Local      bool
}

// ArtistRecord represents a tracked artist.
type ArtistRecord struct {
	ID            int64
	Name          string
	MBID          string
	LastCheckedAt time.Time
	LatestRelease string
	LatestDate    string
	NotFound      bool
}

// DB wraps a SQLite database for musup state.
type DB struct {
	db *sql.DB
	q  *Queries
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

var bg = context.Background()

// Open opens or creates the SQLite database at path and runs migrations.
func Open(path string) (*DB, error) {
	sqlDB, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	// SQLite only supports one writer at a time. Limit the pool to a single
	// connection so concurrent goroutines serialize through database/sql
	// instead of getting SQLITE_BUSY.
	sqlDB.SetMaxOpenConns(1)

	if _, err := sqlDB.Exec("PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000"); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("set pragmas: %w", err)
	}

	d := &DB{db: sqlDB, q: New(sqlDB)}
	if err := d.migrate(); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return d, nil
}

// Close closes the database.
func (d *DB) Close() error {
	return d.db.Close()
}

func (d *DB) migrate() error {
	var version int
	if err := d.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}

	// Version 0 → 1: initial schema + historical fixups
	if version < 1 {
		const schema = `
		CREATE TABLE IF NOT EXISTS files (
			path         TEXT PRIMARY KEY,
			size         INTEGER NOT NULL,
			mod_time     TEXT NOT NULL,
			artist       TEXT NOT NULL,
			album        TEXT NOT NULL DEFAULT '',
			title        TEXT NOT NULL DEFAULT '',
			track_number INTEGER NOT NULL DEFAULT 0,
			scanned_at   TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS albums (
			artist_name  TEXT NOT NULL,
			title        TEXT NOT NULL,
			mbid         TEXT NOT NULL DEFAULT '',
			release_date TEXT NOT NULL DEFAULT '',
			primary_type TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (artist_name, title)
		);

		CREATE TABLE IF NOT EXISTS tracks (
			artist_name  TEXT NOT NULL,
			album_title  TEXT NOT NULL,
			title        TEXT NOT NULL,
			position     INTEGER NOT NULL DEFAULT 0,
			mbid         TEXT NOT NULL DEFAULT '',
			length_ms    INTEGER NOT NULL DEFAULT 0,
			local        INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (artist_name, album_title, title)
		);

		CREATE TABLE IF NOT EXISTS artists (
			name             TEXT PRIMARY KEY,
			mbid             TEXT NOT NULL DEFAULT '',
			last_checked_at  TEXT NOT NULL DEFAULT '',
			latest_release   TEXT NOT NULL DEFAULT '',
			latest_date      TEXT NOT NULL DEFAULT '',
			not_found        INTEGER NOT NULL DEFAULT 0
		);
		`
		if _, err := d.db.Exec(schema); err != nil {
			return err
		}
		if err := d.addColumnIfMissing("artists", "not_found", "INTEGER NOT NULL DEFAULT 0"); err != nil {
			return err
		}
		if err := d.addColumnIfMissing("files", "title", "TEXT NOT NULL DEFAULT ''"); err != nil {
			return err
		}
		if err := d.dropAlbumsLocalColumn(); err != nil {
			return err
		}
		version = 1
	}

	// Version 1 → 2: add track_number to files
	if version < 2 {
		if err := d.addColumnIfMissing("files", "track_number", "INTEGER NOT NULL DEFAULT 0"); err != nil {
			return err
		}
		version = 2
	}

	// Version 2 → 3: add secondary_types to albums
	if version < 3 {
		if err := d.addColumnIfMissing("albums", "secondary_types", "TEXT NOT NULL DEFAULT ''"); err != nil {
			return err
		}
		version = 3
	}

	// Version 3 → 4: add monitor status to artists
	if version < 4 {
		if err := d.addColumnIfMissing("artists", "monitor", "TEXT NOT NULL DEFAULT 'monitor'"); err != nil {
			return err
		}
		version = 4
	}

	// Version 4 → 5: add normalized title/album columns for fuzzy matching
	if version < 5 {
		for _, stmt := range []struct{ table, col string }{
			{"files", "title_norm"},
			{"files", "album_norm"},
			{"tracks", "title_norm"},
			{"tracks", "album_norm"},
		} {
			if err := d.addColumnIfMissing(stmt.table, stmt.col, "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
		}
		if err := d.backfillNorm(); err != nil {
			return err
		}
		version = 5
	}

	// Version 5 → 6: change default monitor status from 'sometimes' to 'monitor'
	if version < 6 {
		if _, err := d.db.Exec("UPDATE artists SET monitor = 'monitor' WHERE monitor = 'sometimes'"); err != nil {
			return err
		}
		version = 6
	}

	// Version 6 → 7: add artist_norm / name_norm columns for case-insensitive matching
	if version < 7 {
		for _, stmt := range []struct{ table, col string }{
			{"files", "artist_norm"},
			{"artists", "name_norm"},
			{"albums", "artist_norm"},
			{"tracks", "artist_norm"},
		} {
			if err := d.addColumnIfMissing(stmt.table, stmt.col, "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
		}
		if err := d.backfillArtistNorm(); err != nil {
			return err
		}
		if _, err := d.db.Exec(`
			CREATE INDEX IF NOT EXISTS idx_files_artist_norm ON files(artist_norm);
			CREATE INDEX IF NOT EXISTS idx_artists_name_norm ON artists(name_norm);
		`); err != nil {
			return err
		}
		version = 7
	}

	// Version 7 → 8: switch to integer PKs with foreign keys
	if version < 8 {
		if err := d.migrateToIntegerPKs(); err != nil {
			return err
		}
		version = 8
	}

	// Version 8 → 9: replace monitor (text) with followed (boolean)
	if version < 9 {
		if err := d.addColumnIfMissing("artists", "followed", "INTEGER NOT NULL DEFAULT 1"); err != nil {
			return err
		}
		if _, err := d.db.Exec("UPDATE artists SET followed = CASE WHEN monitor = 'monitor' THEN 1 ELSE 0 END"); err != nil {
			return err
		}
		version = 9
	}

	// Version 9 → 10: add reviewed_at to artists for tracking reviewed-through point
	if version < 10 {
		if err := d.addColumnIfMissing("artists", "reviewed_at", "TEXT NOT NULL DEFAULT ''"); err != nil {
			return err
		}
		version = 10
	}

	// Version 10 → 11: add is_album_artist flag to files
	if version < 11 {
		if err := d.addColumnIfMissing("files", "is_album_artist", "INTEGER NOT NULL DEFAULT 1"); err != nil {
			return err
		}
		version = 11
	}

	_, err := d.db.Exec(fmt.Sprintf("PRAGMA user_version = %d", version))
	return err
}

func (d *DB) migrateToIntegerPKs() error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Step 1: Create artists_new, deduplicate by name_norm
	if _, err := tx.Exec(`
		CREATE TABLE artists_new (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			name            TEXT NOT NULL,
			name_norm       TEXT NOT NULL UNIQUE,
			mbid            TEXT NOT NULL DEFAULT '',
			last_checked_at TEXT NOT NULL DEFAULT '',
			latest_release  TEXT NOT NULL DEFAULT '',
			latest_date     TEXT NOT NULL DEFAULT '',
			not_found       INTEGER NOT NULL DEFAULT 0,
			monitor         TEXT NOT NULL DEFAULT 'monitor'
		);
		INSERT INTO artists_new (name, name_norm, mbid, last_checked_at, latest_release, latest_date, not_found, monitor)
		SELECT MAX(name), name_norm, MAX(mbid), MAX(last_checked_at), MAX(latest_release), MAX(latest_date), MAX(not_found), MAX(monitor)
		FROM artists
		WHERE name_norm != ''
		GROUP BY name_norm;
	`); err != nil {
		return fmt.Errorf("migrate artists: %w", err)
	}

	// Step 2: Create albums_new — need Go-level Normalize for title_norm
	if _, err := tx.Exec(`
		CREATE TABLE albums_new (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			artist_id       INTEGER NOT NULL REFERENCES artists_new(id),
			title           TEXT NOT NULL,
			title_norm      TEXT NOT NULL DEFAULT '',
			mbid            TEXT NOT NULL DEFAULT '',
			release_date    TEXT NOT NULL DEFAULT '',
			primary_type    TEXT NOT NULL DEFAULT '',
			secondary_types TEXT NOT NULL DEFAULT '',
			UNIQUE (artist_id, title)
		);
	`); err != nil {
		return fmt.Errorf("create albums_new: %w", err)
	}

	// Query old albums and insert into albums_new with computed title_norm
	aRows, err := tx.Query(`
		SELECT a.artist_norm, a.title, MAX(a.mbid), MAX(a.release_date), MAX(a.primary_type), MAX(a.secondary_types)
		FROM albums a
		WHERE a.artist_norm != ''
		GROUP BY a.artist_norm, a.title
	`)
	if err != nil {
		return fmt.Errorf("query old albums: %w", err)
	}
	type albumMigRow struct {
		artistNorm, title, mbid, releaseDate, primaryType, secondaryTypes string
	}
	var albumRows []albumMigRow
	for aRows.Next() {
		var r albumMigRow
		if err := aRows.Scan(&r.artistNorm, &r.title, &r.mbid, &r.releaseDate, &r.primaryType, &r.secondaryTypes); err != nil {
			_ = aRows.Close()
			return err
		}
		albumRows = append(albumRows, r)
	}
	_ = aRows.Close()
	if err := aRows.Err(); err != nil {
		return err
	}

	albumStmt, err := tx.Prepare(`
		INSERT OR IGNORE INTO albums_new (artist_id, title, title_norm, mbid, release_date, primary_type, secondary_types)
		SELECT an.id, ?, ?, ?, ?, ?, ?
		FROM artists_new an WHERE an.name_norm = ?
	`)
	if err != nil {
		return err
	}
	defer func() { _ = albumStmt.Close() }()

	for _, r := range albumRows {
		if _, err := albumStmt.Exec(r.title, Normalize(r.title), r.mbid, r.releaseDate, r.primaryType, r.secondaryTypes, r.artistNorm); err != nil {
			return fmt.Errorf("insert album %q: %w", r.title, err)
		}
	}

	// Step 3: Create tracks_new — need Go-level Normalize for title_norm
	if _, err := tx.Exec(`
		CREATE TABLE tracks_new (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			album_id   INTEGER NOT NULL REFERENCES albums_new(id),
			title      TEXT NOT NULL,
			title_norm TEXT NOT NULL DEFAULT '',
			position   INTEGER NOT NULL DEFAULT 0,
			mbid       TEXT NOT NULL DEFAULT '',
			length_ms  INTEGER NOT NULL DEFAULT 0,
			local      INTEGER NOT NULL DEFAULT 0,
			UNIQUE (album_id, title_norm)
		);
	`); err != nil {
		return fmt.Errorf("create tracks_new: %w", err)
	}

	// Query old tracks and insert into tracks_new
	tRows, err := tx.Query(`
		SELECT t.artist_norm, t.album_title, t.title, MAX(t.position), MAX(t.mbid), MAX(t.length_ms), MAX(t.local)
		FROM tracks t
		WHERE t.artist_norm != ''
		GROUP BY t.artist_norm, t.album_title, t.title
	`)
	if err != nil {
		return fmt.Errorf("query old tracks: %w", err)
	}
	type trackMigRow struct {
		artistNorm, albumTitle, title, mbid string
		position, lengthMS, local           int
	}
	var trackRows []trackMigRow
	for tRows.Next() {
		var r trackMigRow
		if err := tRows.Scan(&r.artistNorm, &r.albumTitle, &r.title, &r.position, &r.mbid, &r.lengthMS, &r.local); err != nil {
			_ = tRows.Close()
			return err
		}
		trackRows = append(trackRows, r)
	}
	_ = tRows.Close()
	if err := tRows.Err(); err != nil {
		return err
	}

	trackStmt, err := tx.Prepare(`
		INSERT OR IGNORE INTO tracks_new (album_id, title, title_norm, position, mbid, length_ms, local)
		SELECT aln.id, ?, ?, ?, ?, ?, ?
		FROM albums_new aln
		JOIN artists_new an ON an.id = aln.artist_id
		WHERE an.name_norm = ? AND aln.title = ?
	`)
	if err != nil {
		return err
	}
	defer func() { _ = trackStmt.Close() }()

	for _, r := range trackRows {
		titleNorm := Normalize(r.title)
		if _, err := trackStmt.Exec(r.title, titleNorm, r.position, r.mbid, r.lengthMS, r.local, r.artistNorm, r.albumTitle); err != nil {
			return fmt.Errorf("insert track %q: %w", r.title, err)
		}
	}

	// Step 4: Drop old tables, rename new ones
	if _, err := tx.Exec(`
		DROP TABLE tracks;
		DROP TABLE albums;
		DROP TABLE artists;
		ALTER TABLE artists_new RENAME TO artists;
		ALTER TABLE albums_new RENAME TO albums;
		ALTER TABLE tracks_new RENAME TO tracks;
		CREATE INDEX idx_albums_artist_id ON albums(artist_id);
	`); err != nil {
		return fmt.Errorf("swap tables: %w", err)
	}

	return tx.Commit()
}

func (d *DB) addColumnIfMissing(table, column, colDef string) error {
	var count int
	err := d.db.QueryRow(
		fmt.Sprintf("SELECT COUNT(*) FROM pragma_table_info('%s') WHERE name = '%s'", table, column),
	).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		_, err = d.db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, colDef))
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *DB) dropAlbumsLocalColumn() error {
	var count int
	err := d.db.QueryRow(
		"SELECT COUNT(*) FROM pragma_table_info('albums') WHERE name = 'local'",
	).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		return nil
	}
	const migration = `
	CREATE TABLE albums_new (
		artist_name  TEXT NOT NULL,
		title        TEXT NOT NULL,
		mbid         TEXT NOT NULL DEFAULT '',
		release_date TEXT NOT NULL DEFAULT '',
		primary_type TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (artist_name, title)
	);
	INSERT INTO albums_new (artist_name, title, mbid, release_date, primary_type)
		SELECT artist_name, title, mbid, release_date, primary_type FROM albums;
	DROP TABLE albums;
	ALTER TABLE albums_new RENAME TO albums;
	`
	_, err = d.db.Exec(migration)
	return err
}

func (d *DB) backfillNorm() error {
	rows, err := d.db.Query("SELECT path, title, album FROM files")
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	type fileNorm struct {
		path, titleNorm, albumNorm string
	}
	var fileUpdates []fileNorm
	for rows.Next() {
		var path, title, album string
		if err := rows.Scan(&path, &title, &album); err != nil {
			return err
		}
		fileUpdates = append(fileUpdates, fileNorm{path, Normalize(title), Normalize(album)})
	}
	if err := rows.Err(); err != nil {
		return err
	}

	trows, err := d.db.Query("SELECT artist_name, album_title, title FROM tracks")
	if err != nil {
		return err
	}
	defer func() { _ = trows.Close() }()

	type trackNorm struct {
		artist, album, title, titleNorm, albumNorm string
	}
	var trackUpdates []trackNorm
	for trows.Next() {
		var artist, album, title string
		if err := trows.Scan(&artist, &album, &title); err != nil {
			return err
		}
		trackUpdates = append(trackUpdates, trackNorm{artist, album, title, Normalize(title), Normalize(album)})
	}
	if err := trows.Err(); err != nil {
		return err
	}

	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, u := range fileUpdates {
		if _, err := tx.Exec("UPDATE files SET title_norm = ?, album_norm = ? WHERE path = ?",
			u.titleNorm, u.albumNorm, u.path); err != nil {
			return err
		}
	}
	for _, u := range trackUpdates {
		if _, err := tx.Exec("UPDATE tracks SET title_norm = ?, album_norm = ? WHERE artist_name = ? AND album_title = ? AND title = ?",
			u.titleNorm, u.albumNorm, u.artist, u.album, u.title); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (d *DB) backfillArtistNorm() error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	fRows, err := tx.Query("SELECT path, artist FROM files")
	if err != nil {
		return err
	}
	type pathArtist struct{ path, norm string }
	var fUpdates []pathArtist
	for fRows.Next() {
		var path, artist string
		if err := fRows.Scan(&path, &artist); err != nil {
			_ = fRows.Close()
			return err
		}
		fUpdates = append(fUpdates, pathArtist{path, Normalize(artist)})
	}
	_ = fRows.Close()
	if err := fRows.Err(); err != nil {
		return err
	}
	for _, u := range fUpdates {
		if _, err := tx.Exec("UPDATE files SET artist_norm = ? WHERE path = ?", u.norm, u.path); err != nil {
			return err
		}
	}

	aRows, err := tx.Query("SELECT name FROM artists")
	if err != nil {
		return err
	}
	type nameNorm struct{ name, norm string }
	var aUpdates []nameNorm
	for aRows.Next() {
		var name string
		if err := aRows.Scan(&name); err != nil {
			_ = aRows.Close()
			return err
		}
		aUpdates = append(aUpdates, nameNorm{name, Normalize(name)})
	}
	_ = aRows.Close()
	if err := aRows.Err(); err != nil {
		return err
	}
	for _, u := range aUpdates {
		if _, err := tx.Exec("UPDATE artists SET name_norm = ? WHERE name = ?", u.norm, u.name); err != nil {
			return err
		}
	}

	alRows, err := tx.Query("SELECT artist_name, title FROM albums")
	if err != nil {
		return err
	}
	type albumKey struct{ artist, title, norm string }
	var alUpdates []albumKey
	for alRows.Next() {
		var artist, title string
		if err := alRows.Scan(&artist, &title); err != nil {
			_ = alRows.Close()
			return err
		}
		alUpdates = append(alUpdates, albumKey{artist, title, Normalize(artist)})
	}
	_ = alRows.Close()
	if err := alRows.Err(); err != nil {
		return err
	}
	for _, u := range alUpdates {
		if _, err := tx.Exec("UPDATE albums SET artist_norm = ? WHERE artist_name = ? AND title = ?", u.norm, u.artist, u.title); err != nil {
			return err
		}
	}

	tRows, err := tx.Query("SELECT artist_name, album_title, title FROM tracks")
	if err != nil {
		return err
	}
	type trackKey struct{ artist, album, title, norm string }
	var tUpdates []trackKey
	for tRows.Next() {
		var artist, album, title string
		if err := tRows.Scan(&artist, &album, &title); err != nil {
			_ = tRows.Close()
			return err
		}
		tUpdates = append(tUpdates, trackKey{artist, album, title, Normalize(artist)})
	}
	_ = tRows.Close()
	if err := tRows.Err(); err != nil {
		return err
	}
	for _, u := range tUpdates {
		if _, err := tx.Exec("UPDATE tracks SET artist_norm = ? WHERE artist_name = ? AND album_title = ? AND title = ?", u.norm, u.artist, u.album, u.title); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// UpsertFile inserts or updates a file record.
func (d *DB) UpsertFile(f FileRecord) error {
	return d.q.UpsertFile(bg, UpsertFileParams{
		Path:          f.Path,
		Size:          f.Size,
		ModTime:       f.ModTime.Format(time.RFC3339),
		Artist:        f.Artist,
		Album:         f.Album,
		Title:         f.Title,
		TrackNumber:   int64(f.TrackNumber),
		IsAlbumArtist: int64(boolToInt(f.IsAlbumArtist)),
		ScannedAt:     f.ScannedAt.Format(time.RFC3339),
		TitleNorm:     Normalize(f.Title),
		AlbumNorm:     Normalize(f.Album),
		ArtistNorm:    Normalize(f.Artist),
	})
}

// FileMeta holds the stored metadata used for change detection.
type FileMeta struct {
	Size    int64
	ModTime string
	Title   string
}

// AllFileMeta loads all file records into a map keyed by path for fast
// in-memory change detection.
func (d *DB) AllFileMeta() (map[string]FileMeta, error) {
	rows, err := d.q.AllFileMeta(bg)
	if err != nil {
		return nil, err
	}
	m := make(map[string]FileMeta, len(rows))
	for _, r := range rows {
		m[r.Path] = FileMeta{Size: r.Size, ModTime: r.ModTime, Title: r.Title}
	}
	return m, nil
}

// ArtistSummary holds aggregate info for one artist.
type ArtistSummary struct {
	Name        string
	AlbumCount  int    // local albums (from files)
	NewestAlbum string // kept for sort mode
	TrackCount  int    // local tracks — from tracks.local when synced, else file count
	TotalAlbums int    // catalog albums (from albums table, 0 if not synced)
	TotalTracks int    // catalog tracks (from tracks table, 0 if not synced)
	Synced      bool   // artist has MBID in artists table
	Followed    bool   // artist is followed for sync
	HasNew      bool   // catalog has albums newer than latest local album
}

// ArtistSummaries returns all artists with album counts and newest album name.
func (d *DB) ArtistSummaries() ([]ArtistSummary, error) {
	rows, err := d.q.ArtistSummaries(bg)
	if err != nil {
		return nil, err
	}
	summaries := make([]ArtistSummary, 0, len(rows))
	for _, r := range rows {
		s := ArtistSummary{
			Name:        r.Name,
			AlbumCount:  int(r.AlbumCnt),
			NewestAlbum: r.Newest,
			TrackCount:  int(r.TrackCnt),
			TotalAlbums: int(r.TotalAlbums),
			TotalTracks: int(r.TotalTracks),
			Synced:      r.Mbid != "",
			Followed:    r.Followed != 0,
			HasNew:      r.HasNew != 0,
		}
		// For synced artists, prefer catalog-matched local counts when they
		// are higher (accounts for fuzzy matching), but never go below the
		// file-based counts — the files table proves those tracks exist.
		if s.Synced && s.TotalTracks > 0 {
			s.TrackCount = max(s.TrackCount, int(r.LocalTracks))
			s.AlbumCount = max(s.AlbumCount, int(r.LocalAlbums))
		}
		summaries = append(summaries, s)
	}
	return summaries, nil
}

// UniqueArtists returns distinct artist names from the files table.
func (d *DB) UniqueArtists() ([]string, error) {
	return d.q.UniqueArtists(bg)
}

// LocalAlbums returns distinct album names for a given artist.
func (d *DB) LocalAlbums(artist string) ([]string, error) {
	return d.q.LocalAlbums(bg, Normalize(artist))
}

// EnsureArtist finds an artist by normalized name or creates one, returning the ID.
func (d *DB) EnsureArtist(name string) (int64, error) {
	norm := Normalize(name)
	row, err := d.q.GetArtistByNameNorm(bg, norm)
	if err == nil {
		return row.ID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	return d.q.InsertArtist(bg, name, norm)
}

// UpdateArtistMeta updates the MBID and last_checked_at for an artist.
func (d *DB) UpdateArtistMeta(id int64, mbid string) error {
	return d.q.UpdateArtistMeta(bg, mbid, time.Now().Format(time.RFC3339), id)
}

// Artist retrieves an artist record by name. Returns nil if not found.
func (d *DB) Artist(name string) (*ArtistRecord, error) {
	row, err := d.q.GetArtistByNameNorm(bg, Normalize(name))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	a := &ArtistRecord{
		ID:            row.ID,
		Name:          row.Name,
		MBID:          row.Mbid,
		LatestRelease: row.LatestRelease,
		LatestDate:    row.LatestDate,
		NotFound:      row.NotFound != 0,
	}
	if row.LastCheckedAt != "" {
		a.LastCheckedAt, _ = time.Parse(time.RFC3339, row.LastCheckedAt)
	}
	return a, nil
}

// MarkArtistNotFound ensures an artist exists and sets not_found = 1.
func (d *DB) MarkArtistNotFound(name string) error {
	id, err := d.EnsureArtist(name)
	if err != nil {
		return err
	}
	return d.q.MarkArtistNotFound(bg, id)
}

// UpsertArtist inserts or updates an artist record (kept for backward compat with tests).
func (d *DB) UpsertArtist(a ArtistRecord) error {
	id, err := d.EnsureArtist(a.Name)
	if err != nil {
		return err
	}
	return d.q.UpdateArtistFull(bg, UpdateArtistFullParams{
		Mbid:          a.MBID,
		LastCheckedAt: a.LastCheckedAt.Format(time.RFC3339),
		LatestRelease: a.LatestRelease,
		LatestDate:    a.LatestDate,
		NotFound:      int64(boolToInt(a.NotFound)),
		ID:            id,
	})
}

// UpsertAlbum inserts or updates an album record. Returns the album ID.
func (d *DB) UpsertAlbum(artistID int64, a AlbumRecord) (int64, error) {
	albumID, err := d.q.UpsertAlbum(bg, UpsertAlbumParams{
		ArtistID:       artistID,
		Title:          a.Title,
		TitleNorm:      Normalize(a.Title),
		Mbid:           a.MBID,
		ReleaseDate:    a.ReleaseDate,
		PrimaryType:    a.PrimaryType,
		SecondaryTypes: a.SecondaryTypes,
	})
	if err != nil {
		return 0, err
	}
	// LastInsertId returns 0 on UPDATE (no new row). Look up the existing ID.
	if albumID == 0 {
		albumID, err = d.q.GetAlbumID(bg, artistID, a.Title)
		if err != nil {
			return 0, err
		}
	}
	return albumID, nil
}

// Albums returns all albums for an artist with computed track counts,
// ordered by release_date ASC then title ASC.
func (d *DB) Albums(artistName string) ([]AlbumRecord, error) {
	rows, err := d.q.ListAlbumsByArtist(bg, Normalize(artistName))
	if err != nil {
		return nil, err
	}
	albums := make([]AlbumRecord, 0, len(rows))
	for _, r := range rows {
		albums = append(albums, AlbumRecord{
			ArtistName:     r.Name,
			Title:          r.Title,
			MBID:           r.Mbid,
			ReleaseDate:    r.ReleaseDate,
			PrimaryType:    r.PrimaryType,
			SecondaryTypes: r.SecondaryTypes,
			TotalTracks:    int(r.TotalTracks),
			LocalTracks:    int(r.LocalTracks),
		})
	}
	return albums, nil
}

// UpsertTrack inserts or updates a track record.
func (d *DB) UpsertTrack(albumID int64, t TrackRecord) error {
	return d.q.UpsertTrack(bg, UpsertTrackParams{
		AlbumID:   albumID,
		Title:     t.Title,
		TitleNorm: Normalize(t.Title),
		Position:  int64(t.Position),
		Mbid:      t.MBID,
		LengthMs:  int64(t.LengthMS),
		Local:     int64(boolToInt(t.Local)),
	})
}

// KnownAlbumMBIDs returns the set of album MBIDs for an artist that already
// have tracks in the database.
func (d *DB) KnownAlbumMBIDs(artistName string) (map[string]struct{}, error) {
	mbids, err := d.q.ListKnownAlbumMBIDs(bg, Normalize(artistName))
	if err != nil {
		return nil, err
	}
	known := make(map[string]struct{}, len(mbids))
	for _, mbid := range mbids {
		known[mbid] = struct{}{}
	}
	return known, nil
}

// Tracks returns all tracks for an album, ordered by position.
func (d *DB) Tracks(artistName, albumTitle string) ([]TrackRecord, error) {
	rows, err := d.q.ListTracksByAlbum(bg, Normalize(artistName), albumTitle)
	if err != nil {
		return nil, err
	}
	tracks := make([]TrackRecord, 0, len(rows))
	for _, r := range rows {
		tracks = append(tracks, TrackRecord{
			ArtistName: r.ArtistName,
			AlbumTitle: r.AlbumTitle,
			Title:      r.Title,
			Position:   int(r.Position),
			MBID:       r.Mbid,
			LengthMS:   int(r.LengthMs),
			Local:      r.Local != 0,
		})
	}
	return tracks, nil
}

// IsFollowed returns whether an artist is followed, defaulting to true if no row exists.
func (d *DB) IsFollowed(artist string) (bool, error) {
	followed, err := d.q.GetFollowed(bg, Normalize(artist))
	if errors.Is(err, sql.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return true, err
	}
	return followed != 0, nil
}

// SetFollowed sets the followed status for an artist.
func (d *DB) SetFollowed(artist string, followed bool) error {
	id, err := d.EnsureArtist(artist)
	if err != nil {
		return err
	}
	return d.q.SetFollowed(bg, int64(boolToInt(followed)), id)
}

// MarkReviewed sets the reviewed_at date for an artist to the latest album
// release date in the catalog. This marks all current albums as "seen."
func (d *DB) MarkReviewed(artistName string) error {
	id, err := d.EnsureArtist(artistName)
	if err != nil {
		return err
	}
	return d.q.MarkReviewed(bg, id)
}

// MarkLocalTracks cross-references the files table to set local flag on tracks.
func (d *DB) MarkLocalTracks(artistName string) error {
	return d.q.MarkLocalTracks(bg, Normalize(artistName))
}

// RemoveStaleFiles deletes file records whose paths are not in livePaths.
func (d *DB) RemoveStaleFiles(livePaths map[string]struct{}) (int64, error) {
	paths, err := d.q.AllFilePaths(bg)
	if err != nil {
		return 0, err
	}

	var stale []string
	for _, p := range paths {
		if _, ok := livePaths[p]; !ok {
			stale = append(stale, p)
		}
	}

	if len(stale) == 0 {
		return 0, nil
	}

	tx, err := d.db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	qtx := d.q.WithTx(tx)
	var removed int64
	for _, p := range stale {
		if err := qtx.DeleteFileByPath(bg, p); err != nil {
			return 0, err
		}
		removed++
	}
	return removed, tx.Commit()
}

// PruneResult holds counts from a prune operation.
type PruneResult struct {
	Artists int64
	Albums  int64
	Tracks  int64
}

// UnfollowedArtistNames returns the names of all unfollowed artists.
func (d *DB) UnfollowedArtistNames() ([]string, error) {
	return d.q.ListUnfollowedArtistNames(bg)
}

// PruneUnfollowed deletes albums, tracks, and artist records for all
// unfollowed artists. Files are not affected.
func (d *DB) PruneUnfollowed() (PruneResult, error) {
	var r PruneResult

	tx, err := d.db.Begin()
	if err != nil {
		return r, err
	}
	defer func() { _ = tx.Rollback() }()

	qtx := d.q.WithTx(tx)

	r.Tracks, err = qtx.DeleteUnfollowedTracks(bg)
	if err != nil {
		return r, err
	}

	r.Albums, err = qtx.DeleteUnfollowedAlbums(bg)
	if err != nil {
		return r, err
	}

	r.Artists, err = qtx.DeleteUnfollowedArtists(bg)
	if err != nil {
		return r, err
	}

	return r, tx.Commit()
}

// Vacuum runs VACUUM on the database to reclaim space.
func (d *DB) Vacuum() error {
	_, err := d.db.Exec("VACUUM")
	return err
}
