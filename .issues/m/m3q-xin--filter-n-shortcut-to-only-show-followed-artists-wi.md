---
# m3q-xin
title: Filter 'n' shortcut to only show followed artists with new releases
status: completed
type: bug
priority: normal
created_at: 2026-03-20T21:09:50Z
updated_at: 2026-03-20T21:10:18Z
sync:
    github:
        issue_number: "46"
        synced_at: "2026-03-20T21:31:30Z"
---

The 'n' toggle shows all artists with hasNew=true, but should only show followed artists with albums newer than last reviewed.


## Summary of Changes

Added `item.followed` check to the `showNew` filter in `applySort()` so the 'n' toggle only shows followed artists with catalog albums newer than their last review date.
