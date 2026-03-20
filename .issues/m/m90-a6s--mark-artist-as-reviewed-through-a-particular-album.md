---
# m90-a6s
title: Mark artist as reviewed through a particular album
status: review
type: feature
priority: normal
created_at: 2026-03-20T20:33:53Z
updated_at: 2026-03-20T20:44:35Z
sync:
    github:
        issue_number: "52"
        synced_at: "2026-03-20T21:31:31Z"
---

## Goal

When pressing "n" to see new albums, only show **unreviewed** new albums. Users should be able to mark an artist as "reviewed through" a particular album, meaning all albums up to and including that one have been seen.

## Requirements

- [x] Add a DB column or table to track the "reviewed through" album for each artist (by release date of latest fetched album)
- [x] Add "r" keybinding in both artist list and album list to mark the selected artist as reviewed through their latest fetched album
- [x] When showing new albums ("n"), filter out artists whose newest album is not newer than their reviewed-through point
- [x] Show visual indicator for artists that have unreviewed new albums (existing HasNew indicator now respects reviewed_at)
- [x] Allow re-pressing "r" to update the reviewed-through point to the current latest album


## Summary of Changes

- Added `reviewed_at` column to `artists` table (migration v9→v10)
- Added `MarkReviewed(artistName)` DB method that sets `reviewed_at` to the max album release date
- Updated `has_new` CTE in `ArtistSummaries()` to compare against `MAX(reviewed_at, max_local_date)`
- Added "r" keybinding in both artist list and album detail views
- Added test `TestMarkReviewed` covering the full lifecycle
