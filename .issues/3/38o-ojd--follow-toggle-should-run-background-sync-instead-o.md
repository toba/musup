---
# 38o-ojd
title: Follow toggle should run background sync instead of modal
status: completed
type: feature
priority: normal
created_at: 2026-03-20T20:38:13Z
updated_at: 2026-03-20T20:41:03Z
sync:
    github:
        issue_number: "43"
        synced_at: "2026-03-20T21:31:30Z"
---

Pressing f to toggle following ON should not show a modal and should not navigate to album list. Instead, start a background sync that appears in the top-right spinner widget.

- [x] Add startBgSyncMsg and bgSyncDoneMsg message types
- [x] Change list.go follow-ON to emit startBgSyncMsg instead of startSyncMsg
- [x] Change detail.go follow-ON to emit startBgSyncMsg instead of startSyncMsg  
- [x] Handle startBgSyncMsg in app.go: add bgTask, run sync in background
- [x] Handle bgSyncDoneMsg in app.go: remove bgTask, refresh list, show status
- [x] Verify: build, test, lint


## Summary of Changes

Changed follow-ON behavior to run sync as a background task instead of showing a modal overlay. The user stays on whatever view they're on (list or detail), the teal spinner shows "syncing <artist>" in the top-right, and a status message appears in the list when done.

**Files modified:**
- `internal/tui/app.go` — added `startBgSyncMsg`/`bgSyncDoneMsg` types, `startBgSync()` method, handlers in Update()
- `internal/tui/list.go` — follow-ON emits `startBgSyncMsg` instead of `startSyncMsg`
- `internal/tui/detail.go` — follow-ON emits `startBgSyncMsg` instead of `startSyncMsg`
