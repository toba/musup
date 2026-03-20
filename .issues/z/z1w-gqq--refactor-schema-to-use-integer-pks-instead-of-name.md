---
# z1w-gqq
title: Refactor schema to use integer PKs instead of name-based composite keys
status: completed
type: task
priority: normal
created_at: 2026-03-18T19:13:32Z
updated_at: 2026-03-20T20:41:03Z
sync:
    github:
        issue_number: "28"
        synced_at: "2026-03-20T21:31:30Z"
---

The current schema uses composite text PKs like `(artist_name, title)` for albums and `(artist_name, album_title, title)` for tracks. This causes bugs when the same entity is inserted under different name variants (e.g., different casing), since the PK treats them as distinct rows even though they normalize to the same value.

## Motivation

- The duplicate albums bug (jt4-mid) was a direct consequence of name-based PKs
- The track count mismatch bug (4qr-gn0) likely stems from the same root cause
- String-based joins are slower and more fragile than integer joins

## Proposed Schema

```sql
CREATE TABLE artists (
    id            INTEGER PRIMARY KEY,
    name          TEXT NOT NULL,        -- display name (most-used variant)
    name_norm     TEXT NOT NULL UNIQUE,  -- normalized for dedup
    mbid          TEXT NOT NULL DEFAULT '',
    last_checked_at TEXT NOT NULL DEFAULT '',
    latest_release  TEXT NOT NULL DEFAULT '',
    latest_date     TEXT NOT NULL DEFAULT '',
    not_found     INTEGER NOT NULL DEFAULT 0,
    monitor       TEXT NOT NULL DEFAULT 'monitor'
);

CREATE TABLE albums (
    id             INTEGER PRIMARY KEY,
    artist_id      INTEGER NOT NULL REFERENCES artists(id),
    title          TEXT NOT NULL,
    title_norm     TEXT NOT NULL,
    mbid           TEXT NOT NULL DEFAULT '',
    release_date   TEXT NOT NULL DEFAULT '',
    primary_type   TEXT NOT NULL DEFAULT '',
    secondary_types TEXT NOT NULL DEFAULT '',
    UNIQUE (artist_id, title_norm)
);

CREATE TABLE tracks (
    id            INTEGER PRIMARY KEY,
    album_id      INTEGER NOT NULL REFERENCES albums(id),
    title         TEXT NOT NULL,
    title_norm    TEXT NOT NULL,
    position      INTEGER NOT NULL DEFAULT 0,
    mbid          TEXT NOT NULL DEFAULT '',
    length_ms     INTEGER NOT NULL DEFAULT 0,
    local         INTEGER NOT NULL DEFAULT 0,
    UNIQUE (album_id, title_norm)
);

CREATE TABLE files (
    path          TEXT PRIMARY KEY,
    size          INTEGER NOT NULL,
    mod_time      TEXT NOT NULL,
    artist        TEXT NOT NULL,
    artist_norm   TEXT NOT NULL DEFAULT '',
    album         TEXT NOT NULL DEFAULT '',
    album_norm    TEXT NOT NULL DEFAULT '',
    title         TEXT NOT NULL DEFAULT '',
    title_norm    TEXT NOT NULL DEFAULT '',
    track_number  INTEGER NOT NULL DEFAULT 0,
    scanned_at    TEXT NOT NULL
);
```

## Tasks

- [ ] Design migration from current schema to integer PKs (version 8)
- [ ] Migrate data: deduplicate albums/tracks by normalized keys, assign integer IDs
- [ ] Update all DB methods (UpsertAlbum, UpsertTrack, Albums, Tracks, etc.) to use integer FKs
- [ ] Update sync logic to resolve artist/album by normalized name → ID before inserting
- [ ] Update MarkLocalTracks to join via IDs
- [ ] Update ArtistSummaries query
- [ ] Update KnownAlbumMBIDs query
- [ ] Run full test suite and fix any breakage
- [ ] Verify UI still works end-to-end

## Summary of Changes

- Added v7→v8 migration that creates new tables with integer PKs (AUTOINCREMENT), deduplicates artists/albums/tracks by normalized names, and drops old text-PK tables
- New schema: `artists` has `id INTEGER PRIMARY KEY`, `albums` references `artist_id`, `tracks` references `album_id`
- Added `EnsureArtist(name) → (id, error)` and `UpdateArtistMeta(id, mbid)` methods
- Changed `UpsertAlbum` to take `artistID int64` and return `(albumID int64, error)`
- Changed `UpsertTrack` to take `albumID int64`
- Rewrote all read queries (Albums, Tracks, ArtistSummaries, KnownAlbumMBIDs, MarkLocalTracks) to use JOIN on integer FKs
- Removed CTE dedup workaround from Albums() — schema now prevents duplicates
- Updated sync.go to use ID-based flow: EnsureArtist → UpsertAlbum → UpsertTrack → UpdateArtistMeta
- All 42 tests pass, lint clean
