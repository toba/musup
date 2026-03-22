---
# 8p6-10b
title: Style discography modal title to match artist list
status: completed
type: task
priority: normal
created_at: 2026-03-22T20:46:16Z
updated_at: 2026-03-22T20:46:56Z
sync:
    github:
        issue_number: "70"
        synced_at: "2026-03-22T21:40:44Z"
---

- [x] Add followed field to modalData\n- [x] Set followed in buildDiscographyModal and pinned update path\n- [x] Render styled title with checkmark/muted dot


## Summary of Changes

Discography modal title now shows green ✓ + bold name for followed artists and muted · + muted name for unfollowed, matching the main list. The followed state is stored in modalData and updated immediately during pinned navigation (before debounce), so toggling follow or switching artists reflects instantly in the title.
