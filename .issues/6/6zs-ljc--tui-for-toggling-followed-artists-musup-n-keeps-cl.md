---
# 6zs-ljc
title: TUI for toggling followed artists; musup <N> keeps CLI mode
status: review
type: feature
priority: normal
created_at: 2026-03-22T16:49:43Z
updated_at: 2026-03-22T17:44:29Z
sync:
    github:
        issue_number: "61"
        synced_at: "2026-03-22T18:16:19Z"
---

## Overview

Change CLI behavior:
- `musup <number>` — same as today: show all artists and albums new within N years
- `musup` (no argument) — open a Bubble Tea TUI showing all album artists (using existing `is_album_artist` filtering) in 3 columns, navigable with arrow keys (up/down/across), with space bar to toggle "follows" checkmark

## Tasks

- [x] Add `followed` boolean column to `artists` table (new DB migration v14)
- [x] Add Bubble Tea + Lip Gloss + Bubbles dependencies to go.mod
- [x] Create TUI in `internal/tui/` package
  - 3-column layout of all album artists
  - Arrow key navigation (up, down, left, right across columns)
  - Space bar toggles followed status (standard checkmark display)
  - Persist follows to DB on toggle
- [x] Update `cmd/root.go`: if no args → launch TUI; if `<number>` arg → current CLI behavior
- [x] Add DB queries: list all album artists with followed status, toggle followed
- [x] Tests for new DB queries and migration


## Summary of Changes

- Added `followed` column to `artists` table (migration v14, default=1)
- Added `AlbumArtists` and `SetFollowed` sqlc queries
- Created `internal/tui/tui.go` with Bubble Tea v2 3-column artist list
- `musup` (no args) opens TUI; `musup <N>` keeps CLI behavior
- Added tests for new queries; all tests pass, lint clean



## Additional Changes (artist_id FK + modal)

- [x] Migration v15: add `artist_id` FK to `files` table, backfill from `artist_norm` join
- [x] Updated all queries (`NewerReleases`, `AlbumArtists`, `DistinctArtistIDs`) to use `artist_id`
- [x] Updated `scan.go` to call `EnsureArtist` and set `artist_id` on file upsert
- [x] Updated `check.go` to use `DistinctArtistIDs` and pass artist ID directly
- [x] Added `ArtistLocalTracks` query and `GetArtistByID` query
- [x] Added enter-key modal showing local albums/tracks for selected artist
- [x] All tests updated and passing, lint clean
