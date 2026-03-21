-- name: UpsertTrack :exec
INSERT INTO tracks (album_id, title, title_norm, position, mbid, length_ms, local)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(album_id, title_norm) DO UPDATE SET
    title     = excluded.title,
    position  = excluded.position,
    mbid      = excluded.mbid,
    length_ms = excluded.length_ms,
    local     = excluded.local;

-- name: ListTracksByAlbum :many
SELECT ar.name AS artist_name, al.title AS album_title, t.title, t.position, t.mbid, t.length_ms, t.local
FROM tracks t
JOIN albums al ON al.id = t.album_id
JOIN artists ar ON ar.id = al.artist_id
WHERE ar.name_norm = ? AND al.title = ?
ORDER BY t.position ASC;

-- name: MarkLocalTracks :exec
UPDATE tracks SET local = (
    EXISTS (
        SELECT 1 FROM files
        JOIN albums al ON al.id = tracks.album_id
        JOIN artists ar ON ar.id = al.artist_id
        WHERE files.artist_norm = ar.name_norm
          AND files.album_norm = al.title_norm
          AND (
            files.title_norm = tracks.title_norm
            OR (files.track_number > 0 AND files.track_number = tracks.position)
          )
    )
    OR EXISTS (
        SELECT 1 FROM files
        JOIN albums al ON al.id = tracks.album_id
        JOIN artists ar ON ar.id = al.artist_id
        WHERE files.artist_norm = ar.name_norm
          AND files.title_norm != ''
          AND files.title_norm = tracks.title_norm
    )
)
WHERE album_id IN (
    SELECT al.id FROM albums al
    JOIN artists ar ON ar.id = al.artist_id
    WHERE ar.name_norm = ?
);
