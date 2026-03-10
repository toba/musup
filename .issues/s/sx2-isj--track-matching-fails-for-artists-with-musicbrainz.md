---
# sx2-isj
title: Track matching fails for artists with MusicBrainz title variations
status: review
type: bug
priority: normal
created_at: 2026-03-10T22:40:01Z
updated_at: 2026-03-10T22:43:32Z
sync:
    github:
        issue_number: "6"
        synced_at: "2026-03-10T22:51:35Z"
---

## Problem
When syncing an artist like 3 Doors Down, 0 tracks match because:
1. `track.Title` (release-specific) is stored instead of `track.Recording.Title` (canonical)
2. `MarkLocalTracks` uses exact case-sensitive title comparison
3. `files` table doesn't store track number for position-based confirmation

## Fix
- [x] Write failing test
- [x] Add `track_number` to `FileRecord` and `files` table schema
- [x] Extract track number from metadata during scan
- [x] Use `track.Recording.Title` instead of `track.Title` in sync.go
- [x] Update `MarkLocalTracks` to fuzzy match: case-insensitive title OR same position within artist+album
- [x] Run lint


## Summary of Changes

Fixed track matching by:
1. Using `track.Recording.Title` (canonical) instead of `track.Title` (release-specific) when storing MusicBrainz data
2. Adding `track_number` to file scanning and storage
3. Updating `MarkLocalTracks` to match by case-insensitive title OR by track position within the same artist+album

Files changed: `internal/state/db.go`, `internal/scan/scan.go`, `internal/tui/sync.go`, `internal/state/db_test.go`
