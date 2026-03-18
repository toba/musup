---
# uhk-fw7
title: Filter artist list to show only artists with newer albums than local
status: completed
type: feature
priority: normal
created_at: 2026-03-18T18:03:14Z
updated_at: 2026-03-18T18:23:44Z
sync:
    github:
        issue_number: "26"
        synced_at: "2026-03-18T18:40:07Z"
---

Add 'n' shortcut to toggle filtering the artist list to show only artists that have catalog albums with release dates newer than the latest album that has local tracks. Requires:
- [ ] Add HasNew field to ArtistSummary, computed via SQL
- [ ] Add showNew toggle and 'n' key handler to list view
- [ ] Filter displayed items when toggle is active
- [ ] Update help bar to show 'n' shortcut
- [ ] Add DB test for HasNew computation
- [ ] Run lint

## Summary of Changes

- Added `HasNew` field to `ArtistSummary` in `internal/state/db.go`, computed via SQL subquery comparing max catalog release date against max local album release date
- Added `hasNew` field to `artistItem` in `internal/tui/list.go`
- Added `showNew` toggle to `listModel` with `n` key handler
- `applySort()` now filters to only `hasNew` items when toggle is active
- Title bar updates to show "X artists with new releases" when filtered
- Status message confirms filter on/off
- Help bar updated to show `n: new` shortcut
- Added `TestArtistSummaries_HasNew` test covering: artist with newer catalog albums (true), artist with no newer albums (false), unsynced artist (false)
