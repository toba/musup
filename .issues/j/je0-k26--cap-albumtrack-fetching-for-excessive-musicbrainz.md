---
# je0-k26
title: Cap album/track fetching for excessive MusicBrainz results
status: completed
type: feature
priority: normal
created_at: 2026-03-13T21:58:38Z
updated_at: 2026-03-13T22:01:07Z
sync:
    github:
        issue_number: "24"
        synced_at: "2026-03-13T22:02:29Z"
---

- [x] Add Tag and ReleaseGroupResult structs to models.go
- [x] Add Tags field to Artist struct
- [x] Modify AllReleaseGroups signature and add cap logic
- [x] Add constants to helpers.go
- [x] Add hasComposerTag helper and update runSync in sync.go
- [x] Update existing tests and add 2 new tests


## Summary of Changes

Added two-tier count-based cap for MusicBrainz album fetching. Composer-tagged artists are capped at 100 albums, all others at 500. When capped, only the first page of album metadata is stored and track fetching is skipped entirely. This prevents runaway API calls for artists like Beethoven (4,813 albums).
