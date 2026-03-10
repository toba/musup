---
# 8q8-wm3
title: 'Fix track matching: fuzzy matching via normalized titles'
status: completed
type: bug
priority: normal
created_at: 2026-03-10T23:37:15Z
updated_at: 2026-03-10T23:44:04Z
sync:
    github:
        issue_number: "14"
        synced_at: "2026-03-10T23:47:37Z"
---

MarkLocalTracks() uses exact string matching, so 'Loser' won't match 'Loser (radio edit)'. Add normalized title/album columns and match on those.

- [x] Create `internal/state/norm.go` with Normalize function
- [x] Create `internal/state/norm_test.go`
- [x] Add migration v5 with norm columns
- [x] Backfill existing rows
- [x] Update UpsertFile/UpsertTrack to populate norm columns
- [x] Update MarkLocalTracks SQL to use norm columns
- [x] Update/add tests in db_test.go
- [x] Run tests and lint


## Summary of Changes

Added fuzzy track matching via normalized title/album columns. Normalize function lowercases, strips parentheticals/brackets, removes punctuation, and collapses whitespace. Two-tier matching: tier 1 matches on artist + normalized album + (normalized title OR track position), tier 2 matches on artist + normalized title across albums. Migration v5 adds columns and backfills existing data. All 30 tests pass, lint clean.
