-- musup schema v17 (final state after all migrations)

CREATE TABLE files (
    path            TEXT PRIMARY KEY,
    size            INTEGER NOT NULL,
    mod_time        TEXT NOT NULL,
    artist          TEXT NOT NULL,
    artist_norm     TEXT NOT NULL DEFAULT '',
    artist_id       INTEGER NOT NULL DEFAULT 0 REFERENCES artists(id),
    album           TEXT NOT NULL DEFAULT '',
    album_norm      TEXT NOT NULL DEFAULT '',
    title           TEXT NOT NULL DEFAULT '',
    title_norm      TEXT NOT NULL DEFAULT '',
    track_number    INTEGER NOT NULL DEFAULT 0,
    is_album_artist INTEGER NOT NULL DEFAULT 1,
    scanned_at      TEXT NOT NULL
);

CREATE TABLE artists (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    name            TEXT NOT NULL,
    name_norm       TEXT NOT NULL UNIQUE,
    mbid            TEXT NOT NULL DEFAULT '',
    last_checked_at TEXT NOT NULL DEFAULT '',
    not_found       INTEGER NOT NULL DEFAULT 0,
    followed        INTEGER NOT NULL DEFAULT 1,
    inactive        INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE albums (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    artist_id       INTEGER NOT NULL REFERENCES artists(id),
    title           TEXT NOT NULL,
    title_norm      TEXT NOT NULL DEFAULT '',
    mbid            TEXT NOT NULL DEFAULT '',
    release_date    TEXT NOT NULL DEFAULT '',
    primary_type    TEXT NOT NULL DEFAULT '',
    secondary_types TEXT NOT NULL DEFAULT '',
    UNIQUE (artist_id, title)
);

CREATE INDEX idx_albums_artist_id ON albums(artist_id);
CREATE INDEX idx_files_artist_norm ON files(artist_norm);
CREATE INDEX idx_files_artist_id ON files(artist_id);
CREATE INDEX idx_artists_name_norm ON artists(name_norm);
