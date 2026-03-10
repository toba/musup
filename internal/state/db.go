package state

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// FileRecord represents a scanned music file.
type FileRecord struct {
	Path      string
	Size      int64
	ModTime   time.Time
	Artist    string
	Album     string
	ScannedAt time.Time
}

// ArtistRecord represents a tracked artist.
type ArtistRecord struct {
	Name          string
	MBID          string
	LastCheckedAt time.Time
	LatestRelease string
	LatestDate    string
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

	if _, err := sqlDB.Exec("PRAGMA journal_mode=WAL"); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
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
	const schema = `
	CREATE TABLE IF NOT EXISTS files (
		path        TEXT PRIMARY KEY,
		size        INTEGER NOT NULL,
		mod_time    TEXT NOT NULL,
		artist      TEXT NOT NULL,
		album       TEXT NOT NULL DEFAULT '',
		scanned_at  TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS artists (
		name             TEXT PRIMARY KEY,
		mbid             TEXT NOT NULL DEFAULT '',
		last_checked_at  TEXT NOT NULL DEFAULT '',
		latest_release   TEXT NOT NULL DEFAULT '',
		latest_date      TEXT NOT NULL DEFAULT ''
	);
	`
	_, err := d.db.Exec(schema)
	return err
}

// UpsertFile inserts or updates a file record.
func (d *DB) UpsertFile(f FileRecord) error {
	const q = `
	INSERT INTO files (path, size, mod_time, artist, album, scanned_at)
	VALUES (?, ?, ?, ?, ?, ?)
	ON CONFLICT(path) DO UPDATE SET
		size       = excluded.size,
		mod_time   = excluded.mod_time,
		artist     = excluded.artist,
		album      = excluded.album,
		scanned_at = excluded.scanned_at
	`
	_, err := d.db.Exec(q,
		f.Path, f.Size, f.ModTime.Format(time.RFC3339),
		f.Artist, f.Album, f.ScannedAt.Format(time.RFC3339),
	)
	return err
}

// FileChanged reports whether a file at path needs re-scanning based on size and mtime.
// Returns true if the file is new or has changed.
func (d *DB) FileChanged(path string, size int64, modTime time.Time) (bool, error) {
	var dbSize int64
	var dbModTime string

	err := d.db.QueryRow("SELECT size, mod_time FROM files WHERE path = ?", path).
		Scan(&dbSize, &dbModTime)
	if err == sql.ErrNoRows {
		return true, nil
	}
	if err != nil {
		return false, err
	}

	if dbSize != size || dbModTime != modTime.Format(time.RFC3339) {
		return true, nil
	}
	return false, nil
}

// ArtistSummary holds aggregate info for one artist.
type ArtistSummary struct {
	Name        string
	AlbumCount  int
	NewestAlbum string
}

// ArtistSummaries returns all artists with album counts and newest album name.
func (d *DB) ArtistSummaries() ([]ArtistSummary, error) {
	const q = `
	SELECT artist, COUNT(DISTINCT album) AS cnt,
	       COALESCE(MAX(CASE WHEN album != '' THEN album END), '') AS newest
	FROM files
	WHERE artist != ''
	GROUP BY artist
	ORDER BY artist
	`
	rows, err := d.db.Query(q)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var summaries []ArtistSummary
	for rows.Next() {
		var s ArtistSummary
		if err := rows.Scan(&s.Name, &s.AlbumCount, &s.NewestAlbum); err != nil {
			return nil, err
		}
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
		"SELECT DISTINCT album FROM files WHERE artist = ? AND album != '' ORDER BY album",
		artist,
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
	INSERT INTO artists (name, mbid, last_checked_at, latest_release, latest_date)
	VALUES (?, ?, ?, ?, ?)
	ON CONFLICT(name) DO UPDATE SET
		mbid             = excluded.mbid,
		last_checked_at  = excluded.last_checked_at,
		latest_release   = excluded.latest_release,
		latest_date      = excluded.latest_date
	`
	_, err := d.db.Exec(q,
		a.Name, a.MBID, a.LastCheckedAt.Format(time.RFC3339),
		a.LatestRelease, a.LatestDate,
	)
	return err
}

// Artist retrieves an artist record by name. Returns nil if not found.
func (d *DB) Artist(name string) (*ArtistRecord, error) {
	var a ArtistRecord
	var lastChecked string
	err := d.db.QueryRow(
		"SELECT name, mbid, last_checked_at, latest_release, latest_date FROM artists WHERE name = ?",
		name,
	).Scan(&a.Name, &a.MBID, &lastChecked, &a.LatestRelease, &a.LatestDate)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if lastChecked != "" {
		a.LastCheckedAt, _ = time.Parse(time.RFC3339, lastChecked)
	}
	return &a, nil
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
