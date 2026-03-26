---
# 6kw-x0d
title: Test FollowedNewerReleases excludes local albums
status: completed
type: task
priority: normal
created_at: 2026-03-26T04:47:28Z
updated_at: 2026-03-26T04:48:20Z
sync:
    github:
        issue_number: "84"
        synced_at: "2026-03-26T04:50:49Z"
---

- [x] Add TestFollowedNewerReleases_ExcludesLocalAlbums
- [x] Add TestFollowedNewerReleases_ReviewedAlsoExcluded
- [x] Verify tests pass and lint clean

## Summary of Changes

Added two tests to `internal/db/db_test.go` verifying that `FollowedNewerReleases` correctly excludes albums once they exist locally. The existing query logic and scan→refresh flow already handle this correctly.
