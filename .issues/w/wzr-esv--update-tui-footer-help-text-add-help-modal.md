---
# wzr-esv
title: Update TUI footer help text & add help modal
status: completed
type: feature
priority: normal
created_at: 2026-03-20T20:45:41Z
updated_at: 2026-03-20T20:48:30Z
sync:
    github:
        issue_number: "53"
        synced_at: "2026-03-20T21:31:32Z"
---

- [x] Update footer text in list.go, detail.go, album.go
- [x] Add viewHelp state and prevHelpState to app.go
- [x] Add help modal rendering function in helpers.go
- [x] Handle ? key in all views and help view dismissal
- [x] Build and lint

## Summary of Changes

Updated footer help text in all three views to be shorter and added `?: help` shortcut. Added a help modal (`viewHelp` state) that shows context-sensitive keyboard shortcuts per view, dismissed by any keypress.
