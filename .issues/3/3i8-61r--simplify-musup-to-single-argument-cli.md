---
# 3i8-61r
title: Simplify musup to single-argument CLI
status: review
type: feature
priority: high
created_at: 2026-03-21T19:55:08Z
updated_at: 2026-03-21T20:23:04Z
sync:
    github:
        issue_number: "60"
        synced_at: "2026-03-21T20:38:39Z"
---

## Goal

Change musup from a multi-command CLI (scan, check, version) to a simple single-argument CLI:

```
musup <years>
```

Example: `musup 1` shows all artists with an album release in the last 1 year that is **newer** than what the user has in local files.

## Motivation

This substantially simplifies:
- What data needs to be stored (just artists + their latest local release date)
- What queries are needed for MusicBrainz (single query per artist: "any album in last N years?")
- The overall UX (one command, one argument)

## Tasks

- [x] Design the new simplified data model (what columns/tables are needed)
- [x] Remove Cobra subcommands (scan, check, version) in favor of a single root command
- [x] Accept a single positional argument: number of years (integer)
- [x] Combine scan + check into a single flow: scan local files, then check MusicBrainz for newer releases
- [x] Simplify DB schema to only store what's needed
- [x] Simplify MusicBrainz queries to only fetch what's needed
- [x] Update README and help text
- [x] Update tests


## Summary of Changes

Simplified musup from an interactive TUI app to a two-command CLI:

- **`musup scan [path]`** — scans music files into the database
- **`musup [years]`** — shows artists with new album releases in the last N years (default: 1) that aren't in the local library

### What changed:
- **DB schema v13**: Dropped `tracks` table, removed `followed`/`reviewed_at`/`monitor` columns from `artists`
- **New `NewerReleases` query**: Single SQL query finds MB albums released since cutoff that aren't in local files
- **New `internal/check` package**: Orchestrates MusicBrainz sync (search artist → fetch release groups → upsert albums)
- **Simplified MB client**: Removed `BrowseReleases` and all Release/Track/Recording types (no longer fetching individual tracks)
- **Deleted `internal/tui/`**: Removed all 11 Bubble Tea TUI files
- **Deleted track/prune queries**: Removed `track.sql`, `prune.sql` and generated code
- **Removed TUI dependencies**: `bubbletea`, `bubbles`, `lipgloss` dropped from go.mod

### Files deleted (16):
- `internal/tui/*` (11 files)
- `internal/db/track.sql`, `track.sql.go`, `prune.sql`, `prune.sql.go`

### Files created (3):
- `internal/check/check.go`
- `cmd/scan.go`

### All tests pass, lint clean.
