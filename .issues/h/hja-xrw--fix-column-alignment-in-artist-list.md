---
# hja-xrw
title: Fix column alignment in artist list
status: completed
type: bug
priority: normal
created_at: 2026-03-20T20:53:33Z
updated_at: 2026-03-20T21:08:06Z
sync:
    github:
        issue_number: "50"
        synced_at: "2026-03-20T21:31:31Z"
---

numWidth=7 is too small for numbers like 433/1010. Compute max width dynamically from data instead of using a fixed constant.


## Summary of Changes

- Changed artistDelegate to use a shared *colWidths pointer instead of a hardcoded numWidth=7 constant
- Column widths are computed dynamically from actual data via computeColumnWidths()
- Widths recompute on refresh so updates propagate without reconstructing the list
