package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	_ "modernc.org/sqlite"
)

// DB wraps a SQLite database for musup state.
type DB struct {
	db *sql.DB
	Q  *Queries
}

// BoolToInt converts a bool to an int (1 or 0).
func BoolToInt(b bool) int {
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

	d := &DB{db: sqlDB, Q: New(sqlDB)}
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

	// Version 11 → 12: drop unused latest_release and latest_date columns from artists
	if version < 12 {
		if err := d.dropColumnIfExists("artists", "latest_release"); err != nil {
			return err
		}
		if err := d.dropColumnIfExists("artists", "latest_date"); err != nil {
			return err
		}
		version = 12
	}

	// Version 12 → 13: drop tracks table, remove followed/reviewed_at/monitor from artists
	if version < 13 {
		if _, err := d.db.Exec("DROP TABLE IF EXISTS tracks"); err != nil {
			return err
		}
		if err := d.dropColumnIfExists("artists", "followed"); err != nil {
			return err
		}
		if err := d.dropColumnIfExists("artists", "reviewed_at"); err != nil {
			return err
		}
		if err := d.dropColumnIfExists("artists", "monitor"); err != nil {
			return err
		}
		version = 13
	}

	// Version 13 → 14: re-add followed flag to artists (default 1 = followed)
	if version < 14 {
		if err := d.addColumnIfMissing("artists", "followed", "INTEGER NOT NULL DEFAULT 1"); err != nil {
			return err
		}
		version = 14
	}

	// Version 14 → 15: add artist_id FK to files
	if version < 15 {
		if err := d.addColumnIfMissing("files", "artist_id", "INTEGER NOT NULL DEFAULT 0"); err != nil {
			return err
		}
		if _, err := d.db.Exec(`
			UPDATE files SET artist_id = (
				SELECT id FROM artists WHERE name_norm = files.artist_norm
			) WHERE artist_norm != '' AND EXISTS (
				SELECT 1 FROM artists WHERE name_norm = files.artist_norm
			)
		`); err != nil {
			return err
		}
		if _, err := d.db.Exec("CREATE INDEX IF NOT EXISTS idx_files_artist_id ON files(artist_id)"); err != nil {
			return err
		}
		version = 15
	}

	// Version 15 → 16: add ended flag to artists (original name)
	if version < 16 {
		if err := d.addColumnIfMissing("artists", "ended", "INTEGER NOT NULL DEFAULT 0"); err != nil {
			return err
		}
		version = 16
	}

	// Version 16 → 17: rename ended → inactive
	if version < 17 {
		if err := d.addColumnIfMissing("artists", "inactive", "INTEGER NOT NULL DEFAULT 0"); err != nil {
			return err
		}
		if _, err := d.db.Exec("UPDATE artists SET inactive = ended WHERE ended != 0"); err != nil {
			return err
		}
		if err := d.dropColumnIfExists("artists", "ended"); err != nil {
			return err
		}
		version = 17
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
			not_found       INTEGER NOT NULL DEFAULT 0,
			monitor         TEXT NOT NULL DEFAULT 'monitor'
		);
		INSERT INTO artists_new (name, name_norm, mbid, last_checked_at, not_found, monitor)
		SELECT MAX(name), name_norm, MAX(mbid), MAX(last_checked_at), MAX(not_found), MAX(monitor)
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

func (d *DB) dropColumnIfExists(table, column string) error { //nolint:unparam // table varies across migration versions
	var count int
	err := d.db.QueryRow(
		fmt.Sprintf("SELECT COUNT(*) FROM pragma_table_info('%s') WHERE name = '%s'", table, column),
	).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		_, err = d.db.Exec(fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", table, column))
		if err != nil {
			return err
		}
	}
	return nil
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

// NormalizeFileParams fills in the TitleNorm, AlbumNorm, and ArtistNorm fields.
func NormalizeFileParams(p *UpsertFileParams) {
	p.TitleNorm = Normalize(p.Title)
	p.AlbumNorm = Normalize(p.Album)
	p.ArtistNorm = Normalize(p.Artist)
}

// NormalizeAlbumParams fills in the TitleNorm field.
func NormalizeAlbumParams(p *UpsertAlbumParams) {
	p.TitleNorm = Normalize(p.Title)
}

// EnsureArtist finds an artist by normalized name or creates one, returning the ID.
func (d *DB) EnsureArtist(name string) (int64, error) {
	norm := Normalize(name)
	row, err := d.Q.GetArtistByNameNorm(bg, norm)
	if err == nil {
		return row.ID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	return d.Q.InsertArtist(bg, name, norm)
}

// RemoveStaleFiles deletes file records whose paths are not in livePaths.
func (d *DB) RemoveStaleFiles(livePaths map[string]struct{}) (int64, error) {
	paths, err := d.Q.AllFilePaths(bg)
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

	qtx := d.Q.WithTx(tx)
	var removed int64
	for _, p := range stale {
		if err := qtx.DeleteFileByPath(bg, p); err != nil {
			return 0, err
		}
		removed++
	}
	return removed, tx.Commit()
}

// MarkArtistNotFound ensures an artist exists and sets not_found = 1.
func (d *DB) MarkArtistNotFound(name string) error {
	id, err := d.EnsureArtist(name)
	if err != nil {
		return err
	}
	return d.Q.MarkArtistNotFound(bg, id)
}

// Vacuum runs VACUUM on the database to reclaim space.
func (d *DB) Vacuum() error {
	_, err := d.db.Exec("VACUUM")
	return err
}
