---
# 643-10e
title: Clear stale album data after website-default filter change
status: completed
type: task
priority: normal
created_at: 2026-03-24T21:02:55Z
updated_at: 2026-03-24T21:06:01Z
sync:
    github:
        issue_number: "79"
        synced_at: "2026-03-24T22:26:58Z"
---

Add DB migration (v17→18) to delete all albums and reset last_checked_at so next check re-fetches with the new release-group-status=website-default filter.

- [x] Add migration v18 to db.go
- [x] Update migration test
- [x] Run lint


## Summary of Changes

Added DB migration v17→18 that deletes all albums and clears `last_checked_at` on all artists. On next `check` run, albums will be re-fetched with the new `release-group-status=website-default` filter, excluding release-groups with only promo/bootleg/pseudo-release status.

Release status (Official/Promo/Bootleg/Pseudo-Release) lives on individual releases within a release group and is not returned in the release-group browse response, so server-side filtering + re-fetch is the correct approach.
