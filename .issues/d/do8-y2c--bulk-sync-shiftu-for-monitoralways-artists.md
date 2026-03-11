---
# do8-y2c
title: Bulk Sync (Shift+U) for MonitorAlways Artists
status: completed
type: feature
priority: normal
created_at: 2026-03-10T23:55:15Z
updated_at: 2026-03-10T23:57:22Z
sync:
    github:
        issue_number: "16"
        synced_at: "2026-03-11T00:13:08Z"
---

Add U keybinding to bulk-sync all MonitorAlways artists sequentially with ESC cancellation support

## Summary of Changes

- **sync.go**: Added `context.Context` parameter to `runSync` with cancellation checks before each MusicBrainz API call and in the track fetch loop. Added `newSyncModelWithContext` helper.
- **bulksync.go** (new): `bulkSyncModel` that sequentially syncs MonitorAlways artists with a progress modal showing completed/current/remaining artists. Per-artist errors are non-fatal. ESC cancels via context.
- **list.go**: Added `U` keybinding that collects MonitorAlways artists and emits `startBulkSyncMsg`. Updated help text.
- **app.go**: Added `viewBulkSyncing` state, `bulkSync` field, message routing (ESC to cancel, forward sync messages), and View rendering.
