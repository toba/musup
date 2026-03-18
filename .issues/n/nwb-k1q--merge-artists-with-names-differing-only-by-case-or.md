---
# nwb-k1q
title: Merge artists with names differing only by case or punctuation
status: completed
type: feature
priority: normal
created_at: 2026-03-18T18:32:22Z
updated_at: 2026-03-18T18:35:57Z
sync:
    github:
        issue_number: "25"
        synced_at: "2026-03-18T18:40:06Z"
---

Add artist_norm/name_norm columns to all four tables, backfill with Normalize(), and update queries to group/match on normalized values.

## Summary of Changes

- Added migration v7: `artist_norm` column on files/albums/tracks, `name_norm` on artists, with backfill and indexes
- Updated all upsert functions (UpsertFile, UpsertArtist, UpsertAlbum, UpsertTrack, MarkArtistNotFound, SetMonitorStatus) to populate norm columns
- Updated all queries (ArtistSummaries, Albums, Tracks, Artist, LocalAlbums, KnownAlbumMBIDs, MarkLocalTracks, GetMonitorStatus) to match on normalized columns
- ArtistSummaries now groups by `artist_norm` and picks canonical display name by most-used variant
- Added `TestArtistSummaries_MergesByNorm` test verifying case-variant artists merge into one
