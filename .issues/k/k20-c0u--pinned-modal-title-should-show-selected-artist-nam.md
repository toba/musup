---
# k20-c0u
title: Use marker instead of reverse-video for selected artist in grid
status: completed
type: bug
priority: normal
created_at: 2026-03-22T20:23:29Z
updated_at: 2026-03-22T20:28:38Z
sync:
    github:
        issue_number: "72"
        synced_at: "2026-03-22T21:40:44Z"
---

When the pinned discography modal overlaps the selected artist in the grid, the selection highlight is hidden behind the modal. The artist name is already shown as the modal title, but it could be more visually prominent to compensate.

- [x] Replace reverse-video selection with a colored ▸ marker that survives overlay compositing
- [x] Revert the selectedStyle title change from previous attempt
- [x] Verify

## Summary of Changes

Replaced reverse-video `selectedStyle` for the selected artist in the grid with a yellow `▸` marker prefix. This survives overlay compositing from the pinned modal since it's a single character with a foreground color rather than a full-line reverse-video effect. Removed unused `renderPlain` function.
