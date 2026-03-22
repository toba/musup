---
# gv3-8lg
title: Pinned discography modal should not show track selection or arrow navigation
status: completed
type: bug
priority: normal
created_at: 2026-03-22T20:16:24Z
updated_at: 2026-03-22T20:17:01Z
sync:
    github:
        issue_number: "63"
        synced_at: "2026-03-22T20:20:12Z"
---

When the artist detail modal is pinned, it still shows the track cursor (selection highlight) and the footer mentions arrow navigation. In pinned mode, the modal is read-only for tracks — arrows navigate artists instead — so the cursor highlight and arrow mention should be removed.

- [x] Remove track cursor rendering when pinned
- [x] Change footer to omit arrow mention
- [x] Verify

## Summary of Changes

In `renderDiscographyContent`, added a `showCursor` parameter — when false (pinned mode), the track selection highlight is not rendered. Updated the pinned footer to show only "esc close" instead of mentioning arrow navigation.
