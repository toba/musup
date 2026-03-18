---
# 1lk-md3
title: Album detail shows 0 tracks despite artist summary showing track count
status: review
type: bug
priority: normal
created_at: 2026-03-18T19:58:06Z
updated_at: 2026-03-18T20:31:29Z
sync:
    github:
        issue_number: "32"
        synced_at: "2026-03-18T20:37:08Z"
---

ArtistSummaries reports 1 track for Amy Lee but Albums() shows 0/10 and 0/12. The track count in ArtistSummaries comes from the files table (local tracks) while Albums() shows catalog tracks. The mismatch is that the artist summary's TotalTracks count is correct but the per-album track counts from Albums() are zero — meaning tracks exist in the DB but the JOIN isn't finding them.

## Tasks
- [x] Reproduce with a failing test
- [x] Identify root cause
- [x] Fix
- [x] Verify with go test


## Summary of Changes

Root cause: `ArtistSummaries.TrackCount` counted local files (`COUNT(*)` from files table) while the album detail view counted `tracks.local` flags (set by `MarkLocalTracks`). When `MarkLocalTracks` couldn't match a file to a catalog track (different album name, etc.), the counts diverged.

Fix: For synced artists, `ArtistSummaries` now uses `SUM(tracks.local)` instead of file count, so the artist list and album detail view always show consistent local track numbers.
