---
# 0in-mro
title: Number key shortcuts (1-9) to filter artist list by recent release years
status: completed
type: feature
priority: normal
created_at: 2026-03-21T18:06:56Z
updated_at: 2026-03-21T18:21:51Z
sync:
    github:
        issue_number: "57"
        synced_at: "2026-03-21T18:36:22Z"
---

## Description

Pressing 1-9 in the artist list should filter to only artists with albums released in the last N years (where N is the key pressed). This filter applies irrespective of whether the artist is followed.

For example:
- Pressing `1` → show only artists with albums released in the last 1 year
- Pressing `5` → show only artists with albums released in the last 5 years
- Pressing `9` → show only artists with albums released in the last 9 years

Pressing the same number again (or `0`) should clear the filter.

## Tasks

- [x] Add key bindings for 1-9 in the artist list view
- [x] Implement year-based filtering logic (compare album release dates against current year minus N)
- [x] Show active filter in the UI (e.g. status bar or help text)
- [x] Filter should ignore follow status — show all matching artists
- [x] Pressing the active filter key again or pressing 0 clears the filter


## Summary of Changes

- Added `latest_date` to `ArtistSummaries` SQL query (already on `artists` table, just not selected)
- Added `latestDate` field to `artistItem` struct and mapped it in `summariesToItems`
- Added `recentYears` filter to `listModel` with key handlers for 1-9 (toggle) and 0 (clear)
- Year filter stacks with existing `showNew` filter; unsynced artists are excluded when year filter is active
- Updated title bar to show active filter (e.g. "last 3 years"), status bar hints, and help modal
