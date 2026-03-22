---
# k0b-fix
title: Fetching Inactive Status modal fails to disappear after fetch completes
status: completed
type: bug
priority: normal
created_at: 2026-03-22T19:27:08Z
updated_at: 2026-03-22T19:30:21Z
sync:
    github:
        issue_number: "62"
        synced_at: "2026-03-22T19:50:11Z"
---

The modal showing spinner + artist name during inactive status fetch does not dismiss when the fetch completes. The `inactiveDataMsg` handler sets `m.modal = nil` but the modal persists.

- [x] Reproduce and identify root cause
- [x] Fix
- [x] Verify


## Summary of Changes

The fetch goroutine was directly mutating `m.modal.content` on a stale Model copy, causing a data race and preventing the modal from updating. Fixed by introducing a `fetchState` struct with mutex-protected read/write, shared by pointer between the goroutine and `View()`. The spinner tick triggers re-renders which read the current artist name from `fetchProgress.get()`.
