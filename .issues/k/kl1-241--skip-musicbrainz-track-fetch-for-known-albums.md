---
# kl1-241
title: Skip MusicBrainz track fetch for known albums
status: in-progress
type: task
priority: normal
created_at: 2026-03-10T23:45:58Z
updated_at: 2026-03-10T23:45:58Z
sync:
    github:
        issue_number: "15"
        synced_at: "2026-03-10T23:47:37Z"
---

During sync, `runSync` calls `mb.BrowseReleases()` for every album from MusicBrainz, even ones already stored in the DB with tracks. Each call costs ~1s due to rate limiting.

Fix: query the DB for albums that already have tracks, and only call MusicBrainz for new/unknown albums.

- [ ] Add `AlbumMBIDs(artist)` method to DB that returns set of known album MBIDs with tracks
- [ ] In `runSync`, skip `BrowseReleases` for albums already in DB
- [ ] Update progress counter to reflect skipped albums
- [ ] Add test for new DB method
