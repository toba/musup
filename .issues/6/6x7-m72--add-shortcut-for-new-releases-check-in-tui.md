---
# 6x7-m72
title: Add * shortcut for new releases check in TUI
status: completed
type: feature
priority: normal
created_at: 2026-03-22T21:07:52Z
updated_at: 2026-03-22T21:15:34Z
sync:
    github:
        issue_number: "66"
        synced_at: "2026-03-22T21:40:44Z"
---

- [x] Add FollowedNewerReleases SQL query + sqlc generate\n- [x] Add newReleases map, newReleasesMode, modalNewReleases to Model\n- [x] Handle * key to query and enter new releases mode\n- [x] Filter artists in new releases mode\n- [x] Yellow asterisk badge in normal view\n- [x] Enter opens new releases modal (album list with dates)\n- [x] Esc exits new releases mode\n- [x] Help bar and help content updates


## Summary of Changes

Added `*` shortcut that queries `FollowedNewerReleases` (new SQL query for followed artists with albums newer than latest local). Enters a filtered view showing only those artists. Enter opens a modal listing newer albums with dates and types. Yellow `*` badge appears after artist names in normal view. Esc returns to normal view.
