---
# g95-gds
title: Slash key toggles un-followed artist filtering in artist list view
status: completed
type: feature
priority: normal
created_at: 2026-03-24T17:17:46Z
updated_at: 2026-03-24T17:21:19Z
sync:
    github:
        issue_number: "73"
        synced_at: "2026-03-24T17:23:26Z"
---

Pressing `/` in the artist list view should toggle filtering of un-followed artists.

## Behavior
- Default view: shows all artists (including un-followed)
- Press `/`: hides un-followed artists (shows only followed)
- Press `/` again: shows all artists again (back to default)

## Tasks
- [x] Add `/` key binding in artist list view
- [x] Implement toggle state for un-followed artist filtering
- [x] Default state shows un-followed artists
- [x] Ensure list updates immediately on toggle


## Summary of Changes

Added `/` key binding to toggle hiding un-followed artists in `internal/tui/tui.go`:
- New `FilterFollowed` key binding mapped to `/`
- New `hideUnfollowed` bool on `Model` (default false = shows all)
- Filter applied early in `applyFilter()` so it composes with inactive and recency filters
- Status bar dynamically shows active filter combination (e.g. `FOLLOWED + INACTIVE`)
- Help modal lists the new shortcut
