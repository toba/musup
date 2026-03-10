---
# gq1-g6w
title: 'Fix list view column spacing: use dynamic name column width and rune-aware padding'
status: completed
type: bug
priority: normal
created_at: 2026-03-10T23:00:11Z
updated_at: 2026-03-10T23:02:21Z
sync:
    github:
        issue_number: "11"
        synced_at: "2026-03-10T23:18:53Z"
---

- [x] Replace fixed nameCol=30 with dynamic width based on terminal width
- [x] Use rune-aware width calculation for multi-byte characters (e.g. Anúna)
- [x] Run lint


## Summary of Changes

- `list.go`: Replaced fixed `nameCol=30` with dynamic width calculated from terminal width; use `runewidth.StringWidth`/`runewidth.Truncate` for correct multi-byte character handling
- `detail.go`: Use `runewidth.StringWidth` for album title padding
- `album.go`: Use `runewidth.StringWidth` for track title padding
- Added direct dependency on `github.com/mattn/go-runewidth` (already a transitive dep)
