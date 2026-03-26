---
# 0vz-pdu
title: 'Artists matching wrong albums: partial name matches (e.g. Bush matches Kate Bush albums)'
status: completed
type: bug
priority: normal
created_at: 2026-03-26T20:42:08Z
updated_at: 2026-03-26T20:48:21Z
sync:
    github:
        issue_number: "86"
        synced_at: "2026-03-26T20:55:20Z"
---

## Problem

Artists are matching the wrong albums in some cases. For example, the group "Bush" is matching albums for the singer "Kate Bush".

This suggests the MusicBrainz search or matching logic is using a partial/substring match instead of an exact artist name match, causing artists whose names are substrings of other artist names to pick up unrelated albums.

## TODO

- [x] Investigate how artist name matching works in the MusicBrainz integration
- [x] Write a failing test that reproduces the bug (e.g. searching for "Bush" should not return Kate Bush albums)
- [x] Fix the matching logic to use exact artist name matching
- [x] Verify fix with existing tests


## Summary of Changes

**Root cause:** `syncArtist()` in `check.go` blindly took `result.Artists[0]` from MusicBrainz search results without verifying the artist name matched. MusicBrainz returns results ranked by relevance score, and partial name matches (e.g. "Kate Bush" for a search of "Bush") can score higher than the exact match.

**Fix:** Added `bestArtistMatch()` which iterates through search results and selects the first artist whose normalized name exactly matches the search name (case-insensitive, punctuation-stripped). If no exact match exists, the artist is marked as not found. Also fixed the same issue in `FetchInactiveStatus()`.

**Files changed:**
- `internal/check/check.go` — added `bestArtistMatch()`, updated `syncArtist()` and `FetchInactiveStatus()` to use it
- `internal/check/check_test.go` — added 2 regression tests, fixed `fakeMB` helper to route by artist name query
