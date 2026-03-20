---
# ppl-yfb
title: Show cached artist list instantly on startup; scan in background
status: completed
type: task
priority: normal
created_at: 2026-03-20T19:22:43Z
updated_at: 2026-03-20T19:26:22Z
sync:
    github:
        issue_number: "37"
        synced_at: "2026-03-20T19:57:53Z"
---

Load ArtistSummaries from DB immediately on startup so the TUI is interactive right away. Run the filesystem scan in the background and refresh the list when it completes.

- [x] Load cached summaries in New() or Init()
- [x] Start in viewList if cached data exists, viewScanning otherwise
- [x] Run scan as background tea.Cmd, refresh list on scanDoneMsg
- [x] Tests pass and lint clean


## Summary of Changes

Modified `New()` to load `ArtistSummaries` from the DB immediately. If cached data exists, the TUI starts in `viewList` right away. The filesystem scan still runs via `Init()` as a background `tea.Cmd`, and when it completes, `scanDoneMsg` refreshes the list in place instead of replacing it.
