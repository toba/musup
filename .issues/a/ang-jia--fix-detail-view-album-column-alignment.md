---
# ang-jia
title: Fix detail view album column alignment
status: in-progress
type: bug
priority: normal
created_at: 2026-03-10T23:26:53Z
updated_at: 2026-03-10T23:26:53Z
sync:
    github:
        issue_number: "13"
        synced_at: "2026-03-10T23:47:37Z"
---

The detail view (album list for an artist) pads all titles to the longest title in the visible range, pushing the ratio column far to the right. Need to use terminal width to set a dynamic title column and truncate long titles, like the list view already does.
