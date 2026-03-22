-- name: GetArtistByNameNorm :one
SELECT id, name, mbid, last_checked_at, not_found
FROM artists WHERE name_norm = ?;

-- name: InsertArtist :execlastid
INSERT INTO artists (name, name_norm) VALUES (?, ?);

-- name: UpdateArtistMeta :exec
UPDATE artists SET mbid = ?, last_checked_at = ? WHERE id = ?;

-- name: UpdateArtistFull :exec
UPDATE artists SET
    mbid = ?, last_checked_at = ?, not_found = ?
WHERE id = ?;

-- name: MarkArtistNotFound :exec
UPDATE artists SET not_found = 1 WHERE id = ?;

-- name: AlbumArtists :many
-- All album artists with their followed status, for the TUI.
SELECT ar.id, ar.name, ar.followed, ar.inactive,
       CAST(COALESCE((SELECT MAX(al.release_date) FROM albums al WHERE al.artist_id = ar.id), '') AS TEXT) AS latest_release
FROM artists ar
WHERE ar.id IN (
    SELECT artist_id FROM files
    WHERE artist != '' AND album != '' AND artist_id != 0
    GROUP BY artist_id
    HAVING MAX(is_album_artist) = 1
)
ORDER BY
  CASE
    WHEN ar.name LIKE 'The %' THEN SUBSTR(ar.name, 5)
    WHEN ar.name LIKE 'A %'   THEN SUBSTR(ar.name, 3)
    ELSE ar.name
  END COLLATE NOCASE;

-- name: GetArtistByID :one
SELECT id, name, name_norm, mbid, last_checked_at, not_found
FROM artists WHERE id = ?;

-- name: SetFollowed :exec
UPDATE artists SET followed = ? WHERE id = ?;

-- name: SetInactive :exec
UPDATE artists SET inactive = ? WHERE id = ?;
