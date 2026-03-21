---
# f8m-tfi
title: Implement sqlc for type-safe database queries
status: review
type: task
priority: normal
created_at: 2026-03-21T16:12:50Z
updated_at: 2026-03-21T16:23:22Z
sync:
    github:
        issue_number: "56"
        synced_at: "2026-03-21T16:38:30Z"
---

Replace hand-written SQL in `internal/state/db.go` (1,293 lines, ~33 methods) with sqlc-generated type-safe Go code, following the pattern established in pacer/core.

## Motivation

- db.go has inline SQL string constants with manual `rows.Scan()` — error-prone (column order mismatches, missing scan targets)
- sqlc validates queries against the schema at generation time, catching bugs before runtime
- Generated code eliminates boilerplate scan/bind code

## Tasks

- [x] Create `config/sqlc.yaml` (sqlite engine, query/schema paths)
- [x] Create `internal/state/model/schema.sql` (v10 final schema)
- [x] Extract queries into `.sql` files organized by entity (file, artist, album, track, prune)
- [x] Run `sqlc generate` successfully
- [x] Refactor `db.go` to use generated query functions
- [x] Keep migrations, RemoveStaleFiles, and Vacuum as hand-written (dynamic DDL / Go logic)
- [x] Verify all existing tests pass (45/45)
- [ ] Run against real library to confirm behavior (user to verify)


## Summary of Changes

- Created `config/sqlc.yaml` for SQLite engine
- Created `internal/state/model/schema.sql` with v10 final schema
- Extracted 22 queries into 5 `.sql` files organized by entity
- Generated type-safe Go code via `sqlc generate`
- Refactored `db.go` to delegate to `model.Queries` instead of hand-written SQL
- Kept migrations, `RemoveStaleFiles`, and `Vacuum` as hand-written (dynamic DDL / Go logic)
- All 45 existing tests pass, 0 lint issues
