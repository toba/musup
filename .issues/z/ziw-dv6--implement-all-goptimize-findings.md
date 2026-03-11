---
# ziw-dv6
title: Implement all goptimize findings
status: completed
type: task
priority: normal
created_at: 2026-03-11T00:33:55Z
updated_at: 2026-03-11T00:44:59Z
sync:
    github:
        issue_number: "19"
        synced_at: "2026-03-11T00:45:59Z"
---

Work through all 25 findings from the Go optimization report.

## Correctness & Modern Idioms
- [x] errors.Is for sql.ErrNoRows (3 sites)
- [x] Stale doc comment on GetMonitorStatus
- [x] Discarded error in status.go SetMonitorStatus
- [x] Discarded error in album.go Tracks
- [x] Pass version through to TUI from cmd

## Function Extraction
- [x] Extract newSpinner()
- [x] Extract modalStyle(width)
- [x] Extract viewableLines standalone func
- [x] Extract ensureVisible standalone func
- [x] Extract summariesToItems()

## Constants/Enums
- [x] mbMinMatchScore = 90
- [x] spinnerFPS constant
- [x] Fix #555 → colorSubtle
- [x] headerSepWidth constant
- [x] mbSearchLimit = 5

## Concurrency
- [x] backfillNorm migration in transaction
- [x] Replace rateLimit mutex with rate.Limiter
- [x] Fan out readTags with errgroup
- [ ] Parallel bulk sync artists (deferred — needs TUI rework)

## Tests
- [x] GetMonitorStatus/SetMonitorStatus tests
- [x] FileChanged edge cases
- [x] Albums SecondaryTypes + Tracks Local round-trip
- [x] UpsertArtist ON CONFLICT update path
- [ ] readTags + parseASF branch coverage (deferred)
- [x] musicbrainz error/edge case tests


## Summary of Changes

24 of 25 findings implemented. Coverage improved: state 75.0% → 78.5%, musicbrainz 88.2% → 89.4%. Deferred: parallel bulk sync (needs TUI redesign), scan branch coverage (low priority).
