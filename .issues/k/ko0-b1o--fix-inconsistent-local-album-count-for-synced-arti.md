---
# ko0-b1o
title: Fix inconsistent local album count for synced artists
status: completed
type: bug
priority: normal
created_at: 2026-03-20T20:49:13Z
updated_at: 2026-03-20T20:53:42Z
sync:
    github:
        issue_number: "48"
        synced_at: "2026-03-20T21:31:31Z"
---

## Bug

For synced artists, the local track count comes from `tracks.local` (catalog-matched),
but the local album count comes from `COUNT(DISTINCT album)` in the `files` table.
This means an artist can show 1 local album but 0 local tracks when the local file
is on a non-catalog album (e.g. a compilation like "Festivus").

Example: Alphabet Backwards shows `0/23 tracks  1/2 albums` — the 1 local album is
"Festivus" (a compilation not in the catalog), so no tracks matched.

## Fix

- [x] For synced artists, derive local album count from catalog data (albums with ≥1 local track) instead of the files table


## Summary of Changes

- Added `local_albums` (`COUNT(DISTINCT CASE WHEN t.local = 1 THEN al.id END)`) to the `track_counts` CTE in `ArtistSummaries()`
- For synced artists, `AlbumCount` now uses catalog-derived `local_albums` instead of `files` table count
- Added `TestArtistSummaries_LocalAlbumCountConsistentWithTracks` reproducing the bug
