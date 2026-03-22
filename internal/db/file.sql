-- name: UpsertFile :exec
INSERT INTO files (path, size, mod_time, artist, album, title, track_number, is_album_artist, scanned_at, title_norm, album_norm, artist_norm, artist_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(path) DO UPDATE SET
    size            = excluded.size,
    mod_time        = excluded.mod_time,
    artist          = excluded.artist,
    album           = excluded.album,
    title           = excluded.title,
    track_number    = excluded.track_number,
    is_album_artist = excluded.is_album_artist,
    scanned_at      = excluded.scanned_at,
    title_norm      = excluded.title_norm,
    album_norm      = excluded.album_norm,
    artist_norm     = excluded.artist_norm,
    artist_id       = excluded.artist_id;

-- name: AllFileMeta :many
SELECT path, size, mod_time, title FROM files;

-- name: AllFilePaths :many
SELECT path FROM files;

-- name: DeleteFileByPath :exec
DELETE FROM files WHERE path = ?;

-- name: DistinctArtistIDs :many
SELECT DISTINCT f.artist_id FROM files f
JOIN artists a ON a.id = f.artist_id
WHERE f.artist != '' AND f.is_album_artist = 1 AND f.artist_id != 0
  AND a.followed = 1;

-- name: ArtistLocalTracks :many
SELECT path, album, track_number, title
FROM files
WHERE artist_id = ? AND album != ''
ORDER BY album COLLATE NOCASE, track_number, title COLLATE NOCASE;
