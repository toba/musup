package state

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// FileRecord represents a scanned music file.
type FileRecord struct {
	Path        string
	Size        int64
	ModTime     time.Time
	Artist      string
	Album       string
	Title       string
	TrackNumber int
	ScannedAt   time.Time
}

// AlbumRecord represents an album in the catalog (from MusicBrainz or local).
type AlbumRecord struct {
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
}

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

	d := &DB{db: sqlDB}
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

	_, err := d.db.Exec(fmt.Sprintf("PRAGMA user_version = %d", version))
	return err
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
	// Recreate table without local column.
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
	// Backfill files
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

	// Backfill tracks
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

	// Write all updates in a single transaction.
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

	// files: backfill artist_norm from artist
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

	// artists: backfill name_norm from name
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

	// albums: backfill artist_norm from artist_name
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

	// tracks: backfill artist_norm from artist_name
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
	const q = `
	INSERT INTO files (path, size, mod_time, artist, album, title, track_number, scanned_at, title_norm, album_norm, artist_norm)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(path) DO UPDATE SET
		size         = excluded.size,
		mod_time     = excluded.mod_time,
		artist       = excluded.artist,
		album        = excluded.album,
		title        = excluded.title,
		track_number = excluded.track_number,
		scanned_at   = excluded.scanned_at,
		title_norm   = excluded.title_norm,
		album_norm   = excluded.album_norm,
		artist_norm  = excluded.artist_norm
	`
	_, err := d.db.Exec(q,
		f.Path, f.Size, f.ModTime.Format(time.RFC3339),
		f.Artist, f.Album, f.Title, f.TrackNumber, f.ScannedAt.Format(time.RFC3339),
		Normalize(f.Title), Normalize(f.Album), Normalize(f.Artist),
	)
	return err
}

// FileChanged reports whether a file at path needs re-scanning based on size and mtime.
// Returns true if the file is new or has changed.
func (d *DB) FileChanged(path string, size int64, modTime time.Time) (bool, error) {
	var dbSize int64
	var dbModTime string
	var title string

	err := d.db.QueryRow("SELECT size, mod_time, title FROM files WHERE path = ?", path).
		Scan(&dbSize, &dbModTime, &title)
	if errors.Is(err, sql.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return false, err
	}

	// Re-scan if metadata was previously missing (empty title means tags
	// weren't extracted, and we now have filename-based fallback).
	if title == "" {
		return true, nil
	}

	if dbSize != size || dbModTime != modTime.Format(time.RFC3339) {
		return true, nil
	}
	return false, nil
}

// ArtistSummary holds aggregate info for one artist.
type ArtistSummary struct {
	Name        string
	AlbumCount  int           // local albums (from files)
	NewestAlbum string        // kept for sort mode
	TrackCount  int           // local tracks (from files)
	TotalAlbums int           // catalog albums (from albums table, 0 if not synced)
	TotalTracks int           // catalog tracks (from tracks table, 0 if not synced)
	Synced      bool          // artist has MBID in artists table
	Monitor     MonitorStatus // monitor/sometimes/ignore
	HasNew      bool          // catalog has albums newer than latest local album
}

// ArtistSummaries returns all artists with album counts and newest album name.
func (d *DB) ArtistSummaries() ([]ArtistSummary, error) {
	const q = `
	SELECT (SELECT artist FROM files WHERE artist_norm = f.artist_norm
	        GROUP BY artist ORDER BY COUNT(*) DESC LIMIT 1) AS display_name,
	       COUNT(DISTINCT f.album) AS album_cnt,
	       COALESCE(MAX(CASE WHEN f.album != '' THEN f.album END), '') AS newest,
	       COUNT(*) AS track_cnt,
	       COALESCE(a.mbid, '') AS mbid,
	       COALESCE(al.total_albums, 0) AS total_albums,
	       COALESCE(tr.total_tracks, 0) AS total_tracks,
	       COALESCE(a.monitor, 'monitor') AS monitor,
	       COALESCE((
	           SELECT 1 FROM albums a2
	           WHERE a2.artist_norm = f.artist_norm
	             AND a2.release_date > COALESCE((
	                 SELECT MAX(a3.release_date)
	                 FROM albums a3
	                 JOIN tracks t3 ON t3.artist_norm = a3.artist_norm
	                                AND t3.album_title = a3.title
	                 WHERE a3.artist_norm = f.artist_norm AND t3.local = 1
	             ), '')
	           LIMIT 1
	       ), 0) AS has_new
	FROM files f
	LEFT JOIN artists a ON a.name_norm = f.artist_norm
	LEFT JOIN (
	    SELECT artist_norm, COUNT(*) AS total_albums
	    FROM albums GROUP BY artist_norm
	) al ON al.artist_norm = f.artist_norm
	LEFT JOIN (
	    SELECT artist_norm, COUNT(*) AS total_tracks
	    FROM tracks GROUP BY artist_norm
	) tr ON tr.artist_norm = f.artist_norm
	WHERE f.artist != ''
	GROUP BY f.artist_norm
	ORDER BY f.artist_norm
	`
	rows, err := d.db.Query(q)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var summaries []ArtistSummary
	for rows.Next() {
		var s ArtistSummary
		var mbid, monitor string
		var hasNew int
		if err := rows.Scan(&s.Name, &s.AlbumCount, &s.NewestAlbum,
			&s.TrackCount, &mbid, &s.TotalAlbums, &s.TotalTracks, &monitor, &hasNew); err != nil {
			return nil, err
		}
		s.Synced = mbid != ""
		s.Monitor = MonitorStatus(monitor)
		s.HasNew = hasNew != 0
		summaries = append(summaries, s)
	}
	return summaries, rows.Err()
}

// UniqueArtists returns distinct artist names from the files table.
func (d *DB) UniqueArtists() ([]string, error) {
	rows, err := d.db.Query("SELECT DISTINCT artist FROM files WHERE artist != '' ORDER BY artist")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var artists []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		artists = append(artists, name)
	}
	return artists, rows.Err()
}

// LocalAlbums returns distinct album names for a given artist.
func (d *DB) LocalAlbums(artist string) ([]string, error) {
	rows, err := d.db.Query(
		"SELECT DISTINCT album FROM files WHERE artist_norm = ? AND album != '' ORDER BY album",
		Normalize(artist),
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var albums []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		albums = append(albums, name)
	}
	return albums, rows.Err()
}

// UpsertArtist inserts or updates an artist record.
func (d *DB) UpsertArtist(a ArtistRecord) error {
	const q = `
	INSERT INTO artists (name, mbid, last_checked_at, latest_release, latest_date, not_found, name_norm)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(name) DO UPDATE SET
		mbid             = excluded.mbid,
		last_checked_at  = excluded.last_checked_at,
		latest_release   = excluded.latest_release,
		latest_date      = excluded.latest_date,
		not_found        = excluded.not_found,
		name_norm        = excluded.name_norm
	`
	notFound := 0
	if a.NotFound {
		notFound = 1
	}
	_, err := d.db.Exec(q,
		a.Name, a.MBID, a.LastCheckedAt.Format(time.RFC3339),
		a.LatestRelease, a.LatestDate, notFound, Normalize(a.Name),
	)
	return err
}

// Artist retrieves an artist record by name. Returns nil if not found.
func (d *DB) Artist(name string) (*ArtistRecord, error) {
	var a ArtistRecord
	var lastChecked string
	var notFound int
	err := d.db.QueryRow(
		"SELECT name, mbid, last_checked_at, latest_release, latest_date, not_found FROM artists WHERE name_norm = ?",
		Normalize(name),
	).Scan(&a.Name, &a.MBID, &lastChecked, &a.LatestRelease, &a.LatestDate, &notFound)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if lastChecked != "" {
		a.LastCheckedAt, _ = time.Parse(time.RFC3339, lastChecked)
	}
	a.NotFound = notFound != 0
	return &a, nil
}

// MarkArtistNotFound upserts an artist with not_found = 1.
func (d *DB) MarkArtistNotFound(name string) error {
	const q = `
	INSERT INTO artists (name, not_found, name_norm)
	VALUES (?, 1, ?)
	ON CONFLICT(name) DO UPDATE SET
		not_found = 1,
		name_norm = excluded.name_norm
	`
	_, err := d.db.Exec(q, name, Normalize(name))
	return err
}

// UpsertAlbum inserts or updates an album record.
func (d *DB) UpsertAlbum(a AlbumRecord) error {
	const q = `
	INSERT INTO albums (artist_name, title, mbid, release_date, primary_type, secondary_types, artist_norm)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(artist_name, title) DO UPDATE SET
		mbid            = excluded.mbid,
		release_date    = excluded.release_date,
		primary_type    = excluded.primary_type,
		secondary_types = excluded.secondary_types,
		artist_norm     = excluded.artist_norm
	`
	_, err := d.db.Exec(q, a.ArtistName, a.Title, a.MBID, a.ReleaseDate, a.PrimaryType, a.SecondaryTypes, Normalize(a.ArtistName))
	return err
}

// Albums returns all albums for an artist with computed track counts,
// ordered by release_date ASC then title ASC.
func (d *DB) Albums(artistName string) ([]AlbumRecord, error) {
	const q = `
	SELECT a.artist_name, a.title, a.mbid, a.release_date, a.primary_type,
	       a.secondary_types, COALESCE(t.total, 0), COALESCE(t.local, 0)
	FROM albums a
	LEFT JOIN (
		SELECT artist_name, album_title,
		       COUNT(*) AS total,
		       SUM(local) AS local
		FROM tracks
		GROUP BY artist_name, album_title
	) t ON t.artist_name = a.artist_name AND t.album_title = a.title
	WHERE a.artist_norm = ?
	ORDER BY a.release_date ASC, a.title ASC
	`
	rows, err := d.db.Query(q, Normalize(artistName))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var albums []AlbumRecord
	for rows.Next() {
		var a AlbumRecord
		if err := rows.Scan(&a.ArtistName, &a.Title, &a.MBID, &a.ReleaseDate, &a.PrimaryType,
			&a.SecondaryTypes, &a.TotalTracks, &a.LocalTracks); err != nil {
			return nil, err
		}
		albums = append(albums, a)
	}
	return albums, rows.Err()
}

// UpsertTrack inserts or updates a track record.
func (d *DB) UpsertTrack(t TrackRecord) error {
	local := 0
	if t.Local {
		local = 1
	}
	const q = `
	INSERT INTO tracks (artist_name, album_title, title, position, mbid, length_ms, local, title_norm, album_norm, artist_norm)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(artist_name, album_title, title) DO UPDATE SET
		position    = excluded.position,
		mbid        = excluded.mbid,
		length_ms   = excluded.length_ms,
		local       = excluded.local,
		title_norm  = excluded.title_norm,
		album_norm  = excluded.album_norm,
		artist_norm = excluded.artist_norm
	`
	_, err := d.db.Exec(q, t.ArtistName, t.AlbumTitle, t.Title, t.Position, t.MBID, t.LengthMS, local,
		Normalize(t.Title), Normalize(t.AlbumTitle), Normalize(t.ArtistName))
	return err
}

// KnownAlbumMBIDs returns the set of album MBIDs for an artist that already
// have tracks in the database. This allows callers to skip fetching track
// listings from MusicBrainz for albums we already know about.
func (d *DB) KnownAlbumMBIDs(artistName string) (map[string]struct{}, error) {
	const q = `
	SELECT DISTINCT a.mbid
	FROM albums a
	JOIN tracks t ON t.artist_name = a.artist_name AND t.album_title = a.title
	WHERE a.artist_norm = ? AND a.mbid != ''
	`
	rows, err := d.db.Query(q, Normalize(artistName))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	known := make(map[string]struct{})
	for rows.Next() {
		var mbid string
		if err := rows.Scan(&mbid); err != nil {
			return nil, err
		}
		known[mbid] = struct{}{}
	}
	return known, rows.Err()
}

// Tracks returns all tracks for an album, ordered by position.
func (d *DB) Tracks(artistName, albumTitle string) ([]TrackRecord, error) {
	const q = `
	SELECT artist_name, album_title, title, position, mbid, length_ms, local
	FROM tracks
	WHERE artist_norm = ? AND album_title = ?
	ORDER BY position ASC
	`
	rows, err := d.db.Query(q, Normalize(artistName), albumTitle)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var tracks []TrackRecord
	for rows.Next() {
		var t TrackRecord
		var local int
		if err := rows.Scan(&t.ArtistName, &t.AlbumTitle, &t.Title, &t.Position, &t.MBID, &t.LengthMS, &local); err != nil {
			return nil, err
		}
		t.Local = local != 0
		tracks = append(tracks, t)
	}
	return tracks, rows.Err()
}

// MonitorStatus represents how frequently to check an artist for new releases.
type MonitorStatus string

const (
	MonitorAlways    MonitorStatus = "monitor"
	MonitorSometimes MonitorStatus = "sometimes"
	MonitorIgnore    MonitorStatus = "ignore"
)

// MonitorStatuses is the ordered list of valid monitor statuses.
var MonitorStatuses = []MonitorStatus{MonitorAlways, MonitorSometimes, MonitorIgnore}

// MonitorLabels maps each status to its display label.
var MonitorLabels = map[MonitorStatus]string{
	MonitorAlways:    "Monitor — always check",
	MonitorSometimes: "Sometimes — check occasionally",
	MonitorIgnore:    "Ignore — never check",
}

// GetMonitorStatus returns the monitor status for an artist, defaulting to MonitorAlways.
func (d *DB) GetMonitorStatus(artist string) (MonitorStatus, error) {
	var status string
	err := d.db.QueryRow("SELECT monitor FROM artists WHERE name_norm = ?", Normalize(artist)).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return MonitorAlways, nil
	}
	if err != nil {
		return MonitorAlways, err
	}
	return MonitorStatus(status), nil
}

// SetMonitorStatus sets the monitor status for an artist, upserting the artists row.
func (d *DB) SetMonitorStatus(artist string, status MonitorStatus) error {
	const q = `
	INSERT INTO artists (name, monitor, name_norm) VALUES (?, ?, ?)
	ON CONFLICT(name) DO UPDATE SET
		monitor   = excluded.monitor,
		name_norm = excluded.name_norm
	`
	_, err := d.db.Exec(q, artist, string(status), Normalize(artist))
	return err
}

// MarkLocalTracks cross-references the files table to set local flag on tracks.
// Uses normalized titles/albums for fuzzy matching, with two-tier logic:
// Tier 1: same artist + normalized album + (normalized title OR track position)
// Tier 2: same artist + normalized title (cross-album fallback)
func (d *DB) MarkLocalTracks(artistName string) error {
	const q = `
	UPDATE tracks SET local = (
		EXISTS (
			SELECT 1 FROM files
			WHERE files.artist_norm = tracks.artist_norm
			  AND files.album_norm = tracks.album_norm
			  AND (
			    files.title_norm = tracks.title_norm
			    OR (files.track_number > 0 AND files.track_number = tracks.position)
			  )
		)
		OR EXISTS (
			SELECT 1 FROM files
			WHERE files.artist_norm = tracks.artist_norm
			  AND files.title_norm != ''
			  AND files.title_norm = tracks.title_norm
		)
	)
	WHERE artist_norm = ?
	`
	_, err := d.db.Exec(q, Normalize(artistName))
	return err
}

// RemoveStaleFiles deletes file records whose paths are not in livePaths.
func (d *DB) RemoveStaleFiles(livePaths map[string]struct{}) (int64, error) {
	rows, err := d.db.Query("SELECT path FROM files")
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()

	var stale []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return 0, err
		}
		if _, ok := livePaths[p]; !ok {
			stale = append(stale, p)
		}
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	if len(stale) == 0 {
		return 0, nil
	}

	tx, err := d.db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare("DELETE FROM files WHERE path = ?")
	if err != nil {
		return 0, err
	}
	defer func() { _ = stmt.Close() }()

	var removed int64
	for _, p := range stale {
		res, err := stmt.Exec(p)
		if err != nil {
			return 0, err
		}
		n, _ := res.RowsAffected()
		removed += n
	}
	return removed, tx.Commit()
}
