---
# cia-f3j
title: Add prune command to delete unfollowed artist catalog data
status: completed
type: feature
priority: normal
created_at: 2026-03-20T20:00:40Z
updated_at: 2026-03-20T20:04:36Z
sync:
    github:
        issue_number: "45"
        synced_at: "2026-03-20T21:31:30Z"
---

Add a `p` key in the artist list that prunes the database of all unfollowed artists' albums and track info, then vacuums the DB.

- [x] Add DB method to delete albums/tracks for unfollowed artists
- [x] Add DB method to vacuum
- [x] Add confirmation modal in TUI
- [x] Add `p` key handler in list view to trigger prune
- [x] Wire up prune execution after confirmation
- [x] Tests
- [x] Lint clean


## Summary of Changes

Added `p` key in artist list to prune unfollowed artists' catalog data (albums, tracks, artist records). Shows a confirmation modal listing affected artists, executes deletion + VACUUM async, then refreshes the list. Files are never affected.

- `PruneUnfollowed()` and `Vacuum()` DB methods
- `UnfollowedArtistNames()` for the confirmation list
- `pruneModel` TUI with confirm/spinner/result states
- `TestPruneUnfollowed` covering deletion counts and file preservation
