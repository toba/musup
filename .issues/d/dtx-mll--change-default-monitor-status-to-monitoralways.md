---
# dtx-mll
title: Change default monitor status to MonitorAlways
status: completed
type: task
priority: normal
created_at: 2026-03-11T00:26:34Z
updated_at: 2026-03-11T00:28:39Z
sync:
    github:
        issue_number: "17"
        synced_at: "2026-03-11T00:45:59Z"
---

Change the default monitor status from 'sometimes' to 'monitor' (MonitorAlways) so that U bulk sync works out of the box.

- [x] Update column default in migration
- [x] Add migration to update existing 'sometimes' rows to 'monitor'
- [x] Update GetMonitorStatus fallback default
- [x] Run lint and tests


## Summary of Changes

Changed default monitor status from 'sometimes' to 'monitor' (MonitorAlways). Added migration v5→6 to update existing rows. Updated all fallback defaults in queries and GetMonitorStatus.
