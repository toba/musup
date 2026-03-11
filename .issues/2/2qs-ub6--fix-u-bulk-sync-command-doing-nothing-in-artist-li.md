---
# 2qs-ub6
title: Fix U (bulk sync) command doing nothing in artist list
status: completed
type: bug
priority: normal
created_at: 2026-03-11T00:21:38Z
updated_at: 2026-03-11T00:25:50Z
sync:
    github:
        issue_number: "18"
        synced_at: "2026-03-11T00:45:59Z"
---

The U command in the artist list only syncs artists with MonitorAlways status, but the default status is 'sometimes'. Since no artists default to MonitorAlways, pressing U does nothing. Fix to sync all non-ignored artists (MonitorAlways + MonitorSometimes).

- [x] Show status message when no MonitorAlways artists found
- [x] Update help text to clarify behavior
- [x] Run lint


## Summary of Changes

Added a status bar message when pressing U with no monitored artists, and clarified the help text from "sync all" to "sync monitored".
