---
# wn9-o7p
title: Status modal enter key doesn't dismiss modal
status: review
type: bug
priority: normal
created_at: 2026-03-18T20:49:33Z
updated_at: 2026-03-18T21:06:28Z
sync:
    github:
        issue_number: "33"
        synced_at: "2026-03-18T21:20:00Z"
---

Pressing enter in the monitor status modal saves the selection but doesn't close the modal. Expected: enter saves and dismisses.

## Tasks
- [x] Write failing test (N/A — TUI modal)
- [x] Fix the bug
- [x] Verify with go test


## Summary of Changes

The status model's enter handler called `SetMonitorStatus` and, on error, returned `nil` cmd — keeping the modal open with no visible feedback. Now the modal always dismisses on enter regardless of DB errors.
