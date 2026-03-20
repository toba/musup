---
# m7v-xee
title: Simplify monitor status to boolean followed
status: in-progress
type: task
priority: normal
created_at: 2026-03-20T19:05:49Z
updated_at: 2026-03-20T19:05:49Z
sync:
    github:
        issue_number: "34"
        synced_at: "2026-03-20T19:06:00Z"
---

- [ ] Migration v6→v7: add followed column, migrate data
- [ ] Remove MonitorStatus type and related functions
- [ ] Add IsFollowed/SetFollowed API
- [ ] Update ArtistSummary and ArtistSummaries
- [ ] Delete internal/tui/status.go
- [ ] Update list.go (bool field, toggle on 's')
- [ ] Update app.go (remove status picker)
- [ ] Update helpers.go (field mapping)
- [ ] Update db_test.go
- [ ] Verify build, tests, lint
