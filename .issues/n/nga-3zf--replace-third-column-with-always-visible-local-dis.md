---
# nga-3zf
title: Replace third column with always-visible local discography pane
status: review
type: feature
priority: normal
created_at: 2026-03-24T17:26:00Z
updated_at: 2026-03-24T17:43:45Z
sync:
    github:
        issue_number: "74"
        synced_at: "2026-03-24T17:44:23Z"
---

Replace the third artist-name column with a persistent local discography pane on the right side of the TUI. This pane shows the same content as the current Enter detail modal (local albums and tracks for the selected artist) and updates as the cursor moves.

## Requirements
- Right pane replaces the third artist column (grid becomes two columns + detail pane)
- Shows local discography for the currently selected artist (same data as Enter modal)
- Full-height border around the pane, even when content doesn't fill it
- Updates immediately on cursor navigation
- Enter key no longer needed to view discography (modal removed or repurposed)

## Tasks
- [x] Remove third artist column from grid layout
- [x] Add right-side discography pane with full-height border
- [x] Populate pane with local albums/tracks for selected artist
- [x] Update pane content on cursor movement
- [x] Reconcile with existing Enter modal and pinned modal behavior
- [x] Update help text


## Summary of Changes

Replaced the third artist column with an always-visible local discography pane on the right side of the TUI:

- Grid changed from 3 columns to 2 (left 2/3 of terminal)
- Right 1/3 is a bordered pane showing local albums/tracks for the selected artist
- Full-height rounded border via `paneStyle` with `Height()` enforced
- Pane updates on cursor movement with 50ms debounce (`discogRefreshMsg`/`discogGen`)
- Artist name + followed status update immediately; full content deferred
- Removed Enter discography modal, pinned modal (`p` key), and all related infrastructure
- Enter key now only opens new releases modal in `*` mode
- Updated help text to remove `enter` and `p` entries
