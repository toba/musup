---
# c0c-gez
title: Highlight entire row when navigating artist list
status: completed
type: feature
priority: normal
created_at: 2026-03-18T19:12:49Z
updated_at: 2026-03-20T20:41:03Z
sync:
    github:
        issue_number: "29"
        synced_at: "2026-03-20T21:31:29Z"
---

When navigating the artist list, only the artist name text is highlighted. The entire row (including track count, album count columns) should be color-highlighted for better visibility.

## Tasks
- [x] Update artist list rendering to apply highlight style to the full row, not just the name
- [ ] Verify visual appearance


## Summary of Changes

When a row is selected, the track count and album count columns now render in the accent color instead of muted gray. The artist name remains bold+accent; the stat columns use accent (non-bold) to visually unify the row while keeping the name prominent.
