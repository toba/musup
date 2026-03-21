-- name: UpsertAlbum :execlastid
INSERT INTO albums (artist_id, title, title_norm, mbid, release_date, primary_type, secondary_types)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(artist_id, title) DO UPDATE SET
    title_norm      = excluded.title_norm,
    mbid            = excluded.mbid,
    release_date    = excluded.release_date,
    primary_type    = excluded.primary_type,
    secondary_types = excluded.secondary_types;

-- name: GetAlbumID :one
SELECT id FROM albums WHERE artist_id = ? AND title = ?;

-- name: ListAlbumsByArtist :many
SELECT ar.name, al.title, al.mbid, al.release_date, al.primary_type,
       al.secondary_types, CAST(COALESCE(t.total, 0) AS INTEGER) AS total_tracks, CAST(COALESCE(t.local, 0) AS INTEGER) AS local_tracks
FROM albums al
JOIN artists ar ON ar.id = al.artist_id
LEFT JOIN (
    SELECT album_id,
           COUNT(*) AS total,
           SUM(local) AS local
    FROM tracks
    GROUP BY album_id
) t ON t.album_id = al.id
WHERE ar.name_norm = ?
ORDER BY al.release_date ASC, al.title ASC;

-- name: ListKnownAlbumMBIDs :many
SELECT DISTINCT al.mbid
FROM albums al
JOIN artists ar ON ar.id = al.artist_id
JOIN tracks t ON t.album_id = al.id
WHERE ar.name_norm = ? AND al.mbid != '';
