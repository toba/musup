---
# o1s-71b
title: Implement goptimize findings
status: completed
type: task
priority: normal
created_at: 2026-03-21T16:03:58Z
updated_at: 2026-03-21T16:11:03Z
sync:
    github:
        issue_number: "55"
        synced_at: "2026-03-21T16:12:07Z"
---

## Findings

### TUI Package
- [x] Extract `accentStyle` to styles.go (used 4+ times as inline `lipgloss.NewStyle().Foreground(colorAccent)`)
- [x] Extract cursor/selection helper: repeated cursor+style pattern in detail.go, album.go, sort.go, list.go
- [x] Extract `pluralize` helper (detail.go:119, album.go:72)
- [x] Move `listenForSync` to helpers.go (used in sync.go + bulksync.go)
- [x] Deduplicate `bgView()` logic with help-view bg logic in app.go
- [x] Extract `boolToInt` helper in state/db.go (3 occurrences)
- [x] Add `const tagReadParallelism = 8` in scan.go
- [x] Fix scan.go mutex pattern: use append instead of index tracking
- [x] Add `const maxVisibleBulkResults = 5` in bulksync.go


## Summary of Changes

All 9 findings implemented. Tests pass, zero lint issues.
