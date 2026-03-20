---
# m7v-xee
title: Simplify monitor status to boolean followed
status: completed
type: task
priority: normal
created_at: 2026-03-20T19:05:49Z
updated_at: 2026-03-20T19:33:21Z
sync:
    github:
        issue_number: "34"
        synced_at: "2026-03-20T19:57:53Z"
---

- [x] Migration v8→v9: add followed column, migrate from monitor
- [x] Remove MonitorStatus type and related functions
- [x] Add IsFollowed/SetFollowed API
- [x] Update ArtistSummary and ArtistSummaries
- [x] Delete internal/tui/status.go
- [x] Update list.go (bool field, add f toggle key)
- [x] Update app.go (remove status picker)
- [x] Update helpers.go (field mapping)
- [x] Update db_test.go
- [x] Verify build, tests, lint


## Summary of Changes

- Added migration v8→v9: adds `followed` INTEGER column to artists, migrates from `monitor` text field
- Replaced `MonitorStatus` type/constants/methods with `IsFollowed`/`SetFollowed` boolean API
- Added `f` key in artist list and detail views to toggle followed status
- Removed status picker modal (`status.go`) and `viewStatusPicker` state
- Updated `ArtistSummary.Followed` bool field and query
- Removed unused `colorWarning` style
