---
# 88e-3s0
title: Implement MusicBrainz client for release lookups
status: completed
type: task
priority: normal
created_at: 2026-03-10T19:09:25Z
updated_at: 2026-03-10T19:14:18Z
sync:
    github:
        issue_number: "4"
        synced_at: "2026-03-10T20:06:26Z"
---

Build our own MusicBrainz API client in internal/musicbrainz/.

Requirements:
- Artist search by name
- Browse releases by artist MBID (with pagination)
- Filter by release type (album, EP, single)
- Proper User-Agent header per MB API guidelines
- Respect 1 req/sec rate limit
- net/http based, no external client library

Reference michiwend/gomusicbrainz (cited) for API patterns and response models but implement from scratch.
