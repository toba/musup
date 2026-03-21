-- name: GetArtistByNameNorm :one
SELECT id, name, mbid, last_checked_at, latest_release, latest_date, not_found
FROM artists WHERE name_norm = ?;

-- name: InsertArtist :execlastid
INSERT INTO artists (name, name_norm) VALUES (?, ?);

-- name: UpdateArtistMeta :exec
UPDATE artists SET mbid = ?, last_checked_at = ? WHERE id = ?;

-- name: UpdateArtistFull :exec
UPDATE artists SET
    mbid = ?, last_checked_at = ?, latest_release = ?, latest_date = ?, not_found = ?
WHERE id = ?;

-- name: MarkArtistNotFound :exec
UPDATE artists SET not_found = 1 WHERE id = ?;

-- name: GetFollowed :one
SELECT followed FROM artists WHERE name_norm = ?;

-- name: SetFollowed :exec
UPDATE artists SET followed = ? WHERE id = ?;

-- name: MarkReviewed :exec
UPDATE artists SET reviewed_at = COALESCE(
    (SELECT MAX(al.release_date) FROM albums al WHERE al.artist_id = sqlc.arg(artist_id)),
    ''
) WHERE artists.id = sqlc.arg(artist_id);

-- name: ListUnfollowedArtistNames :many
SELECT name FROM artists WHERE followed = 0;

-- name: ArtistSummaries :many
WITH
  file_stats AS (
    SELECT artist_norm,
           COUNT(DISTINCT album) AS album_cnt,
           COALESCE(MAX(CASE WHEN album != '' THEN album END), '') AS newest,
           COUNT(*) AS track_cnt
    FROM files
    WHERE artist != ''
    GROUP BY artist_norm
    HAVING MAX(is_album_artist) = 1
  ),
  display_names AS (
    SELECT artist_norm, artist AS name
    FROM (
      SELECT artist_norm, artist,
             ROW_NUMBER() OVER (PARTITION BY artist_norm ORDER BY COUNT(*) DESC) AS rn
      FROM files
      WHERE artist != ''
      GROUP BY artist_norm, artist
    )
    WHERE rn = 1
  ),
  album_counts AS (
    SELECT ar.name_norm, COUNT(*) AS total_albums
    FROM albums al JOIN artists ar ON ar.id = al.artist_id
    GROUP BY ar.name_norm
  ),
  track_counts AS (
    SELECT ar.name_norm,
           COUNT(*) AS total_tracks,
           SUM(t.local) AS local_tracks,
           COUNT(DISTINCT CASE WHEN t.local = 1 THEN al.id END) AS local_albums
    FROM tracks t
    JOIN albums al ON al.id = t.album_id
    JOIN artists ar ON ar.id = al.artist_id
    GROUP BY ar.name_norm
  ),
  max_local_date AS (
    SELECT ar.name_norm, MAX(al.release_date) AS max_date
    FROM albums al
    JOIN artists ar ON ar.id = al.artist_id
    JOIN tracks t ON t.album_id = al.id
    WHERE t.local = 1
    GROUP BY ar.name_norm
  ),
  has_new AS (
    SELECT ar.name_norm
    FROM albums al
    JOIN artists ar ON ar.id = al.artist_id
    LEFT JOIN max_local_date mld ON mld.name_norm = ar.name_norm
    WHERE al.release_date > MAX(COALESCE(mld.max_date, ''), COALESCE(ar.reviewed_at, ''))
    GROUP BY ar.name_norm
  )
SELECT dn.name,
       CAST(fs.album_cnt AS INTEGER) AS album_cnt,
       CAST(fs.newest AS TEXT) AS newest,
       CAST(fs.track_cnt AS INTEGER) AS track_cnt,
       CAST(COALESCE(a.mbid, '') AS TEXT) AS mbid,
       CAST(COALESCE(ac.total_albums, 0) AS INTEGER) AS total_albums,
       CAST(COALESCE(tc.total_tracks, 0) AS INTEGER) AS total_tracks,
       CAST(COALESCE(a.followed, 1) AS INTEGER) AS followed,
       CAST(CASE WHEN hn.name_norm IS NOT NULL THEN 1 ELSE 0 END AS INTEGER) AS has_new,
       CAST(COALESCE(tc.local_tracks, 0) AS INTEGER) AS local_tracks,
       CAST(COALESCE(tc.local_albums, 0) AS INTEGER) AS local_albums,
       CAST(COALESCE(a.latest_date, '') AS TEXT) AS latest_date
FROM file_stats fs
JOIN display_names dn ON dn.artist_norm = fs.artist_norm
LEFT JOIN artists a ON a.name_norm = fs.artist_norm
LEFT JOIN album_counts ac ON ac.name_norm = fs.artist_norm
LEFT JOIN track_counts tc ON tc.name_norm = fs.artist_norm
LEFT JOIN has_new hn ON hn.name_norm = fs.artist_norm
ORDER BY fs.artist_norm;
