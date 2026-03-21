-- name: UpsertFile :exec
INSERT INTO files (path, size, mod_time, artist, album, title, track_number, is_album_artist, scanned_at, title_norm, album_norm, artist_norm)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
    artist_norm     = excluded.artist_norm;

-- name: AllFileMeta :many
SELECT path, size, mod_time, title FROM files;

-- name: AllFilePaths :many
SELECT path FROM files;

-- name: DeleteFileByPath :exec
DELETE FROM files WHERE path = ?;

-- name: DistinctArtistNorms :many
SELECT DISTINCT artist_norm FROM files
WHERE artist != '' AND is_album_artist = 1 AND artist_norm != '';
