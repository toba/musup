---
# 48a-ols
title: Setting reviewed can fail to include newest albums
status: completed
type: bug
priority: normal
created_at: 2026-03-27T00:05:20Z
updated_at: 2026-03-27T00:08:32Z
sync:
    github:
        issue_number: "87"
        synced_at: "2026-03-27T00:09:37Z"
---

When pressing comma to mark an artist as reviewed, the reviewed timestamp doesn't always capture the most recent albums.

**Example:** AJR after pressing comma shows:
- **NEWER RELEASES**: 2026 Live from the Hollywood Bowl
- **REVIEWED**: 2023 The Maybe Man

The 2026 album should have been included in REVIEWED (or at least not shown as a newer release), since the user just marked the artist as reviewed.

## Reproduce

- [x] Investigate how the reviewed timestamp is set vs how newer releases are determined
- [x] Write a failing test that reproduces the bug
- [x] Fix the logic so that pressing comma captures all currently known releases
- [x] Verify fix passes


## Summary of Changes

**Root cause:** `reviewedAt` was set to `time.Now()` (today's date), but MusicBrainz albums can have future release dates (e.g., 2026-03-27 when today is 2026-03-26). The string comparison `releaseDate <= reviewedAt` meant future-dated albums stayed in NEWER RELEASES.

**Fix:** Added `reviewedDate()` helper that takes the max of today's date and the latest release date from the artist's newer releases. This ensures all currently known releases are covered when marking as reviewed.

**Files changed:**
- `internal/tui/tui.go` — extracted `reviewedDate()` function, updated comma handler to use it
- `internal/tui/reviewed_test.go` — new tests covering future releases, no releases, and past-only releases
