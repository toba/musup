---
# ebi-dss
title: Add background task spinner in top-right corner
status: completed
type: feature
priority: normal
created_at: 2026-03-20T20:29:24Z
updated_at: 2026-03-20T20:41:03Z
sync:
    github:
        issue_number: "49"
        synced_at: "2026-03-20T21:31:31Z"
---

Add a spinner in the top-right corner to show background activity during scans.

- [x] Add teal color and spinner style to styles.go
- [x] Add bgSpinner and bgTasks to Model in app.go
- [x] Start/stop spinner on cachedMsg/scanDoneMsg
- [x] Render spinner overlay in View()
- [x] Verify: build, test, lint


## Summary of Changes

Added a teal-colored braille spinner in the top-right corner that appears during background scans. Uses a `bgTasks []string` slice to track active tasks, supporting multiple concurrent background tasks with appropriate labeling (single task shows name, multiple shows count).

**Files modified:**
- `internal/tui/styles.go` — added `colorTeal` and `bgTaskStyle`
- `internal/tui/app.go` — added `bgSpinner`, `bgTasks`, tick forwarding, `bgTaskView()`, overlay in `View()`
