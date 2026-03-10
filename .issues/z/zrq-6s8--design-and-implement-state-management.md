---
# zrq-6s8
title: Design and implement state management
status: completed
type: task
priority: normal
created_at: 2026-03-10T19:20:40Z
updated_at: 2026-03-10T19:36:14Z
sync:
    github:
        issue_number: "2"
        synced_at: "2026-03-10T20:06:26Z"
---

musup needs persistent state to avoid re-scanning the entire music library on every run.

## State to track

### Per-artist
- Artist name (as seen in tags)
- MusicBrainz MBID (once resolved)
- Known album titles/IDs from local files
- Latest known release group ID from MusicBrainz
- Last checked timestamp

### Per-file (for incremental scanning)
- File path
- Modification time (mtime) — probably sufficient, avoids hashing thousands of files
- Artist/album extracted from tags

## Initial vs incremental scan

First run: walk entire music folder, read tags from every file, build unique artist list and their local albums. This is I/O heavy (potentially thousands of files).

Subsequent runs: only re-read files whose mtime changed since last scan. Skip unchanged files entirely.

## File hashing evaluation

Hashing (e.g. xxhash of first N bytes + size) would detect content changes even if mtime is preserved, but:
- mtime changes on any real-world tag edit or re-encode
- Hashing thousands of large audio files is slow (reading full content)
- Partial hashing (first 4KB + file size) is fast but fragile
- Not worth the complexity — mtime is the standard approach (same as git, rsync, make)

**Recommendation: use mtime + file size as change detection. Store state as a single JSON or YAML file (e.g. ~/.config/musup/state.json or .musup.json in the music folder).**

## Open questions
- State file location: XDG config dir vs alongside music folder?
- Support multiple music folders?


## Summary of Changes

Implemented SQLite-based state management using `modernc.org/sqlite` (pure-Go, no CGo).

### New packages
- **`internal/state/`** — SQLite state layer with `files` and `artists` tables, change detection via mtime+size, stale file pruning
- **`internal/scan/`** — Music file walker that reads tags (`.flac`, `.mp3`, `.m4a`, `.mp4`, `.aac`), stores relative paths, skips unchanged files on subsequent runs

### Modified files
- **`cmd/root.go`** — Added `--db` persistent flag
- **`cmd/scan.go`** — Wired up scanner: opens DB, calls `scan.Scan()`, prints summary
- **`cmd/check.go`** — Opens DB, lists unique artists (ready for MusicBrainz lookups)

### New dependencies
- `modernc.org/sqlite` — pure-Go SQLite driver
- `github.com/dhowden/tag` — audio metadata reader

### Tests
- 9 state tests (open/close, change detection, upsert, unique artists, local albums, stale removal)
- 3 scan tests (empty dir, unsupported extensions, supported ext set)
