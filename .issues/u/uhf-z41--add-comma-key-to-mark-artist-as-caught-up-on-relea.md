---
# uhf-z41
title: Add comma key to mark artist as caught up on releases
status: review
type: feature
priority: normal
created_at: 2026-03-24T21:43:00Z
updated_at: 2026-03-24T21:46:01Z
sync:
    github:
        issue_number: "77"
        synced_at: "2026-03-24T22:26:56Z"
---

- [x] Add reviewed_at column to schema.sql and migration v19
- [x] Add SetReviewedAt mutation and update AlbumArtists query
- [x] Regenerate sqlc
- [x] Add comma key handler to TUI
- [x] Update renderItem badge logic for reviewed_at
- [x] Update renderPane with NEWER RELEASES / REVIEWED split
- [x] Update help text
- [x] Update test version checks
- [x] Build, test, lint


## Summary of Changes

Added `reviewed_at` column to artists table (migration v19). Pressing `,` toggles the reviewed date: sets to today if empty, clears if set. The `*` badge only shows for releases newer than `reviewed_at`. The discography pane splits non-local releases into NEWER RELEASES (unreviewed) and REVIEWED (acknowledged) sections.
