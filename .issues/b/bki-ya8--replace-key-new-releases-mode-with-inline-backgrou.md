---
# bki-ya8
title: Replace * key new-releases mode with inline background sync
status: review
type: feature
priority: normal
created_at: 2026-03-24T21:22:42Z
updated_at: 2026-03-24T21:26:35Z
sync:
    github:
        issue_number: "81"
        synced_at: "2026-03-24T22:26:57Z"
---

- [x] Export SyncArtist from check package
- [x] Remove newReleasesMode from TUI (mode, modal, filter, help)
- [x] Add sync infrastructure (SyncArtistFunc, fields, messages)
- [x] Implement * key handler with toggle (start/cancel)
- [x] Handle syncArtistDoneMsg in Update
- [x] Update spinner tick, renderItem, help text
- [x] Inject SyncArtistFunc from cmd/root.go
- [x] Build, test, lint


## Summary of Changes

Replaced the `*` key new-releases mode with inline background sync. Pressing `*` now syncs all followed artists from MusicBrainz (skipping recently-checked). The checkmark next to each artist becomes a spinner while syncing. Pressing `*` again cancels. Removed the newReleasesMode filtered view, the new releases modal, and the unused openFile/searchWeb functions.
