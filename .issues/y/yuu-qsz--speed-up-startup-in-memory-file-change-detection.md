---
# yuu-qsz
title: 'Speed up startup: in-memory file change detection'
status: completed
type: task
priority: normal
created_at: 2026-03-20T18:55:32Z
updated_at: 2026-03-20T18:59:52Z
sync:
    github:
        issue_number: "35"
        synced_at: "2026-03-20T19:06:01Z"
---

Replace per-file FileChanged SQLite queries during scan with a single bulk load into a map, then check in-memory during the directory walk.

- [x] Add DB method to bulk-load file metadata
- [x] Modify scan.Scan to use in-memory map
- [x] Remove now-unused FileChanged method
- [x] Run tests


## Summary of Changes

Replaced per-file `FileChanged` SQLite queries with `AllFileMeta()` that bulk-loads all file metadata into an in-memory map at scan start. The directory walk now checks against this map instead of issuing N individual queries. Tests updated accordingly.
