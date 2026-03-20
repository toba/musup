---
# wuv-xju
title: 'Startup hangs: ArtistSummaries takes ~51s'
status: completed
type: bug
priority: normal
created_at: 2026-03-20T19:44:37Z
updated_at: 2026-03-20T19:55:43Z
sync:
    github:
        issue_number: "40"
        synced_at: "2026-03-20T19:57:56Z"
---

## Problem
App appears frozen on startup because `ArtistSummaries()` takes ~51 seconds on a real 435MB database with 861 artists and 7343 files.

## Benchmark results
- DB open + migrate: 3ms
- ArtistSummaries: **51s** ← the bottleneck
- AllFileMeta: 7ms
- Full scan: 316ms

## Root cause
The `ArtistSummaries` query has expensive correlated subqueries (especially `has_new`) that scan the entire albums/tracks tables per artist group.

## Fix
- [x] Make ArtistSummaries async — show spinner on startup instead of hanging
- [x] Optimize the ArtistSummaries SQL query to sub-second (51s → 0.6s)
- [x] Make all refreshItems calls async — never block the event loop with DB queries


## Summary of Changes

**Root cause**: `ArtistSummaries()` query took ~51s due to correlated subqueries. It was also called synchronously in the bubbletea `Update()` handler (via `refreshItems()`), blocking the event loop and freezing keyboard input for ~600ms even after the query optimization.

**Fixes**:
1. Rewrote `ArtistSummaries` query using CTEs instead of correlated subqueries (51s → 0.6s)
2. Moved initial `ArtistSummaries` call from `tui.New()` (blocking) to `Init()` (async via `loadCached` command)
3. Replaced synchronous `refreshItems()` with async `refreshCmd()` (returns `tea.Cmd`) and `refreshFrom()` (uses pre-fetched data). No DB queries ever run inside `Update()` now.
4. Added `BenchmarkArtistSummaries` and `BenchmarkAllFileMeta` against real DB snapshot in `testdata/large.db`
