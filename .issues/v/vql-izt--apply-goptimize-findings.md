---
# vql-izt
title: Apply goptimize findings
status: completed
type: task
priority: normal
created_at: 2026-03-24T21:51:04Z
updated_at: 2026-03-24T21:56:21Z
sync:
    github:
        issue_number: "78"
        synced_at: "2026-03-24T22:26:58Z"
---

- [x] Extract dateFormat constant
- [x] Extract filterArtists helper in tui.go
- [x] Extract appendReleaseSection closure in renderPane
- [x] Extract updateAllArtist helper in tui.go
- [x] Extract columnExists helper in db.go
- [x] Extract releaseTypeAlbum constant in check.go
- [x] Extract httpTimeout constant in musicbrainz client
- [x] Add check package tests (0% -> 59%)
- [ ] Add tui package tests (deferred — complex state machine)
- [x] Build, test, lint


## Summary of Changes

Constants: dateFormat, httpTimeout, releaseTypeAlbum. Extractions: filterArtists, updateAllArtist, columnExists, appendReleaseSection. Tests: 5 new check package tests covering SyncArtist, SyncAll staleness/cancellation, and composer tag handling. Check coverage 0% -> 59%.
