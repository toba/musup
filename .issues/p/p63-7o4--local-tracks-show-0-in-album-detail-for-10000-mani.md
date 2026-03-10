---
# p63-7o4
title: Local tracks show 0 in album detail for 10,000 Maniacs despite correct count in list view
status: completed
type: bug
priority: normal
created_at: 2026-03-10T23:05:56Z
updated_at: 2026-03-10T23:15:51Z
sync:
    github:
        issue_number: "10"
        synced_at: "2026-03-10T23:18:53Z"
---

- [x] Write failing test reproducing the bug
- [x] Investigate track matching logic in DB
- [x] Fix the root cause
- [x] Run lint


## Summary of Changes

### Bug fix: local tracks showing 0
- `scan.go`: Added `parseFilename()` fallback — when tag metadata is missing title/track number, extract from filename patterns like "06 Somebody's Heaven.flac"
- `db.go`: `FileChanged()` now re-scans files with empty titles so the new filename parsing takes effect
- Added `TestParseFilename` and `TestMarkLocalTracks_TrackNumberOnly` tests

### Enhancement: secondary types for albums
- Added `secondary-types` to MusicBrainz `ReleaseGroup` model
- DB migration v3: added `secondary_types` column to albums table
- Sync now stores secondary types (Compilation, Live, etc.)
- Detail view shows secondary type tags next to album entries
- User must re-sync artists to populate secondary types
