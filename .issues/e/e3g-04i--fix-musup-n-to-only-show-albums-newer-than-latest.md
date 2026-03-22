---
# e3g-04i
title: Fix musup N to only show albums newer than latest local
status: completed
type: bug
priority: normal
created_at: 2026-03-22T21:00:32Z
updated_at: 2026-03-22T21:01:09Z
sync:
    github:
        issue_number: "71"
        synced_at: "2026-03-22T21:40:44Z"
---

- [x] Update NewerReleases SQL query with latest_local CTE\n- [x] Regenerate sqlc\n- [x] Verify build and tests


## Summary of Changes

Added `latest_local` CTE to `NewerReleases` query that computes each artist's newest local album release date by joining `files` to `albums` on normalized title. Added `AND al.release_date > COALESCE(ll.max_date, '')` filter so only albums newer than the latest local one are shown. Artists with no matched local album date still show all within the cutoff window.
