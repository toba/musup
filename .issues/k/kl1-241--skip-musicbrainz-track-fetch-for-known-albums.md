---
# kl1-241
title: Skip MusicBrainz track fetch for known albums
status: completed
type: task
priority: normal
created_at: 2026-03-10T23:45:58Z
updated_at: 2026-03-10T23:50:44Z
sync:
    github:
        issue_number: "15"
        synced_at: "2026-03-11T00:13:08Z"
---

During sync, `runSync` calls `mb.BrowseReleases()` for every album from MusicBrainz, even ones already stored in the DB with tracks. Each call costs ~1s due to rate limiting.

Fix: query the DB for albums that already have tracks, and only call MusicBrainz for new/unknown albums.

- [x] Add `KnownAlbumMBIDs(artist)` method to DB that returns set of known album MBIDs with tracks
- [x] In `runSync`, skip `BrowseReleases` for albums already in DB
- [x] Update progress counter to reflect skipped albums
- [x] Add test for new DB method


## Summary of Changes

Added `KnownAlbumMBIDs()` to DB layer which queries for albums that already have tracks stored. Modified `runSync` to skip the expensive `BrowseReleases` API call for these albums, only fetching tracks for genuinely new albums. Progress counter now reflects only the albums being fetched. Album metadata is still always upserted to keep it fresh.
