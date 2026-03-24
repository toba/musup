---
# 05j-b9x
title: Filter MB browse to official releases only
status: completed
type: task
priority: normal
created_at: 2026-03-24T19:53:40Z
updated_at: 2026-03-24T19:54:38Z
sync:
    github:
        issue_number: "75"
        synced_at: "2026-03-24T19:55:13Z"
---

Add release-group-status=website-default to MusicBrainz release-group browse requests to exclude bootleg/promo/pseudo-release-only groups, reducing noise and API calls.

- [x] Add releaseGroupStatusDefault constant to client.go
- [x] Add release-group-status param to BrowseReleaseGroups
- [x] Update TestBrowseReleaseGroups assertion
- [x] Update TestBrowseReleaseGroups_NoTypeFilter assertion
- [x] Run tests and lint


## Summary of Changes

Added `release-group-status=website-default` query parameter to `BrowseReleaseGroups` in `client.go`. This server-side filter excludes release-groups that contain only promotional, bootleg, or pseudo-release status releases, matching the MusicBrainz website default view. Reduces noise and API calls. Updated two tests to assert the new parameter.
