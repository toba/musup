-- name: UpsertAlbum :exec
INSERT INTO albums (artist_id, title, title_norm, mbid, release_date, primary_type, secondary_types)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(artist_id, title) DO UPDATE SET
    title_norm      = excluded.title_norm,
    mbid            = excluded.mbid,
    release_date    = excluded.release_date,
    primary_type    = excluded.primary_type,
    secondary_types = excluded.secondary_types;

-- name: NewerReleases :many
-- Artists with MB albums released since cutoff that aren't in local files.
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
)
SELECT dn.name AS artist_name,
       al.title AS album_title,
       al.release_date,
       al.primary_type,
       al.secondary_types
FROM local_artists la
JOIN display_names dn ON dn.artist_id = la.artist_id
JOIN albums al ON al.artist_id = la.artist_id
WHERE al.release_date >= ?
  AND al.title_norm NOT IN (
      SELECT DISTINCT album_norm FROM files
      WHERE artist_id = la.artist_id AND album_norm != ''
  )
ORDER BY dn.name, al.release_date DESC;
