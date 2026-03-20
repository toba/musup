---
# 3b1-zdh
title: Auto-sync when following an artist
status: completed
type: feature
priority: normal
created_at: 2026-03-20T20:06:06Z
updated_at: 2026-03-20T20:06:58Z
sync:
    github:
        issue_number: "42"
        synced_at: "2026-03-20T21:31:30Z"
---

When following is toggled ON via `f`, automatically start a sync (MusicBrainz discography fetch) for that artist.

- [x] List view: after `f` toggles followed=true, emit `startSyncMsg`
- [x] Detail view: same behavior
- [x] Lint clean


## Summary of Changes

When `f` toggles an artist to followed, both the list and detail views now emit `startSyncMsg` to automatically fetch the full MusicBrainz discography. Unfollowing still just refreshes the list.
