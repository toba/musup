---
# e5x-x7v
title: Truncate long track names in discography modal with ellipsis
status: completed
type: bug
priority: normal
created_at: 2026-03-22T20:21:17Z
updated_at: 2026-03-22T20:23:08Z
sync:
    github:
        issue_number: "68"
        synced_at: "2026-03-22T21:40:44Z"
---

Long track names like "Everything's Not Lost Includes Hidden Track 'Life Is For Living'" overflow the modal width. Truncate track lines to fit within the modal's available width.

- [x] Add truncation to track lines in renderDiscographyContent
- [x] Verify

## Summary of Changes

Added `maxWidth` parameter to `renderDiscographyContent`. After splitting albums into left/right columns, truncates all lines using the existing `truncate()` function — full content width for single-column, half (minus gap) for two-column layout.
