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
    SELECT artist_norm
    FROM files
    WHERE artist != '' AND album != ''
    GROUP BY artist_norm
    HAVING MAX(is_album_artist) = 1
),
display_names AS (
    SELECT artist_norm, artist AS name
    FROM (
        SELECT artist_norm, artist,
               ROW_NUMBER() OVER (PARTITION BY artist_norm ORDER BY COUNT(*) DESC) AS rn
        FROM files WHERE artist != ''
        GROUP BY artist_norm, artist
    ) WHERE rn = 1
)
SELECT dn.name AS artist_name,
       al.title AS album_title,
       al.release_date,
       al.primary_type,
       al.secondary_types
FROM local_artists la
JOIN display_names dn ON dn.artist_norm = la.artist_norm
JOIN artists ar ON ar.name_norm = la.artist_norm
JOIN albums al ON al.artist_id = ar.id
WHERE al.release_date >= ?
  AND al.title_norm NOT IN (
      SELECT DISTINCT album_norm FROM files
      WHERE artist_norm = la.artist_norm AND album_norm != ''
  )
ORDER BY dn.name, al.release_date DESC;
