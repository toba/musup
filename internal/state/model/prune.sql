-- name: DeleteUnfollowedTracks :execrows
DELETE FROM tracks WHERE album_id IN (
    SELECT al.id FROM albums al
    JOIN artists ar ON ar.id = al.artist_id
    WHERE ar.followed = 0
);

-- name: DeleteUnfollowedAlbums :execrows
DELETE FROM albums WHERE artist_id IN (
    SELECT id FROM artists WHERE followed = 0
);

-- name: DeleteUnfollowedArtists :execrows
DELETE FROM artists WHERE followed = 0;
