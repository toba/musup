---
# 4qr-gn0
title: Artist view shows track count but album view shows zero tracks
status: completed
type: bug
priority: normal
created_at: 2026-03-18T19:12:49Z
updated_at: 2026-03-18T19:35:16Z
sync:
    github:
        issue_number: "31"
        synced_at: "2026-03-18T19:45:07Z"
---

Artists like Amy Lee show `1/22` tracks in the artist list view, but when drilling into the album view, all albums show zero tracks.

This suggests the track count in `ArtistSummaries` counts differently (possibly across all artist_name variants or via a different join) than the per-album `Albums()` query which joins tracks by exact `artist_name`.

## Tasks
- [x] Write a failing test that reproduces the mismatch
- [x] Identify the discrepancy between ArtistSummaries track counting and Albums track counting
- [x] Fix the inconsistency (fixed by z1w-gqq integer PK refactor)
- [x] Verify with `go test ./...`


## Summary of Changes

This bug was fixed as a side effect of the integer PK refactor (z1w-gqq). The root cause was that `ArtistSummaries` joined tracks via `artist_norm` (finding all name variants) while `Albums()` joined via exact `(artist_name, album_title)` text keys (missing variants). With integer FKs, both paths join through `artist_id`/`album_id`, eliminating the mismatch.

Added `TestTrackCounts_ConsistentBetweenSummariesAndAlbums` regression test.
