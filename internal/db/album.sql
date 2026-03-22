-- name: UpsertAlbum :exec
INSERT INTO albums (artist_id, title, title_norm, mbid, release_date, primary_type, secondary_types)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(artist_id, title) DO UPDATE SET
    title_norm      = excluded.title_norm,
    mbid            = excluded.mbid,
    release_date    = excluded.release_date,
    primary_type    = excluded.primary_type,
    secondary_types = excluded.secondary_types;

-- name: DeleteAlbumsByArtist :exec
DELETE FROM albums WHERE artist_id = ?;

-- name: NewerReleases :many
-- Artists with MB albums released since cutoff that are newer than the latest local album.
WITH local_artists AS (
    SELECT artist_id
    FROM files
    WHERE artist != '' AND album != '' AND artist_id != 0
    GROUP BY artist_id
    HAVING MAX(is_album_artist) = 1
),
display_names AS (
    SELECT artist_id, artist AS name
    FROM (
        SELECT artist_id, artist,
               ROW_NUMBER() OVER (PARTITION BY artist_id ORDER BY COUNT(*) DESC) AS rn
        FROM files WHERE artist != '' AND artist_id != 0
        GROUP BY artist_id, artist
    ) WHERE rn = 1
),
latest_local AS (
    SELECT f.artist_id, MAX(al.release_date) AS max_date
    FROM files f
    JOIN albums al ON al.artist_id = f.artist_id AND al.title_norm = f.album_norm
    WHERE f.album_norm != ''
    GROUP BY f.artist_id
)
SELECT dn.name AS artist_name,
       al.title AS album_title,
       al.release_date,
       al.primary_type,
       al.secondary_types
FROM local_artists la
JOIN display_names dn ON dn.artist_id = la.artist_id
JOIN albums al ON al.artist_id = la.artist_id
LEFT JOIN latest_local ll ON ll.artist_id = la.artist_id
WHERE al.release_date >= ?
  AND al.release_date > COALESCE(ll.max_date, '')
  AND al.title_norm NOT IN (
      SELECT DISTINCT album_norm FROM files
      WHERE artist_id = la.artist_id AND album_norm != ''
  )
ORDER BY
  CASE
    WHEN dn.name LIKE 'The %' THEN SUBSTR(dn.name, 5)
    WHEN dn.name LIKE 'A %'   THEN SUBSTR(dn.name, 3)
    ELSE dn.name
  END COLLATE NOCASE, al.release_date DESC;

-- name: FollowedNewerReleases :many
-- Followed artists with MB albums newer than their latest local album.
WITH latest_local AS (
    SELECT f.artist_id, MAX(al.release_date) AS max_date
    FROM files f
    JOIN albums al ON al.artist_id = f.artist_id AND al.title_norm = f.album_norm
    WHERE f.album_norm != ''
    GROUP BY f.artist_id
)
SELECT ar.id AS artist_id,
       ar.name AS artist_name,
       al.title AS album_title,
       al.release_date,
       al.primary_type,
       al.secondary_types
FROM artists ar
JOIN albums al ON al.artist_id = ar.id
LEFT JOIN latest_local ll ON ll.artist_id = ar.id
WHERE ar.followed = 1
  AND al.release_date > COALESCE(ll.max_date, '')
  AND al.title_norm NOT IN (
      SELECT DISTINCT album_norm FROM files
      WHERE artist_id = ar.id AND album_norm != ''
  )
ORDER BY
  CASE
    WHEN ar.name LIKE 'The %' THEN SUBSTR(ar.name, 5)
    WHEN ar.name LIKE 'A %'   THEN SUBSTR(ar.name, 3)
    ELSE ar.name
  END COLLATE NOCASE, al.release_date DESC;
