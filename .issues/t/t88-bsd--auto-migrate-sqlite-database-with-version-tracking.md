---
# t88-bsd
title: Auto-migrate SQLite database with version tracking
status: completed
type: task
priority: normal
created_at: 2026-03-10T22:56:28Z
updated_at: 2026-03-10T22:56:28Z
sync:
    github:
        issue_number: "7"
        synced_at: "2026-03-10T22:56:42Z"
---

Replace ad-hoc `migrate()` with `PRAGMA user_version`-based versioned migrations so schema changes apply automatically and only run once.

- [x] Use `PRAGMA user_version` as version counter
- [x] Version 0→1: initial schema + historical fixups (idempotent)
- [x] Version 1→2: add `track_number` to `files`
- [x] Add `TestMigrationFromV0` and `TestMigrationIdempotent`

## Summary of Changes

Replaced `migrate()` body with version-checked `if version < N` blocks using SQLite's `PRAGMA user_version`. Existing helpers (`addColumnIfMissing`, `dropAlbumsLocalColumn`) kept as-is. Added two migration tests verifying upgrade from v0 and idempotent reopens.
