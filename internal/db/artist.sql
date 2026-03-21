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
