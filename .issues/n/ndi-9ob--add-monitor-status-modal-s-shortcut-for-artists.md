---
# ndi-9ob
title: Add monitor status modal ('s' shortcut) for artists
status: review
type: feature
priority: normal
created_at: 2026-03-10T23:31:59Z
updated_at: 2026-03-10T23:42:17Z
sync:
    github:
        issue_number: "12"
        synced_at: "2026-03-10T23:47:37Z"
---

Add a monitor status picker modal triggered by 's' key in the list view. Statuses: monitor (always check), sometimes (default), ignore (never check).

- [x] Add monitor column to artists table (migration v3→v4)
- [x] Add DB methods: GetMonitorStatus, SetMonitorStatus
- [x] Create status picker modal (statusModel) following sortModel pattern
- [x] Wire 's' key in list view to open modal
- [x] Persist chosen status to DB
- [x] Show monitor status indicator in list view
- [x] Update help bar with 's: status'


## Summary of Changes

Added monitor status feature for artists with three levels: Monitor (always check), Sometimes (default), and Ignore (never check).

- DB: migration v3→v4 adds `monitor` column to `artists` table; `GetMonitorStatus`/`SetMonitorStatus` methods; `ArtistSummary` includes monitor field
- TUI: new `status.go` with `statusModel` (modal picker following `sortModel` pattern); `s` key opens picker from list view; list shows indicators (▲ monitor, — ignore, · synced default)
- Files: `internal/state/db.go`, `internal/tui/status.go` (new), `internal/tui/list.go`, `internal/tui/app.go`, `internal/state/db_test.go`
