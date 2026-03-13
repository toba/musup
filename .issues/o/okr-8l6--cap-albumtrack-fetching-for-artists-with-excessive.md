---
# okr-8l6
title: Cap album/track fetching for artists with excessive MusicBrainz results
status: in-progress
type: feature
priority: normal
created_at: 2026-03-13T21:44:15Z
updated_at: 2026-03-13T21:45:31Z
sync:
    github:
        issue_number: "23"
        synced_at: "2026-03-13T22:02:29Z"
---

## Problem

Some artists — particularly classical composers like Beethoven — have thousands of albums in MusicBrainz. This causes massive, slow fetches and bloats the database with data that isn't useful for the core use case (detecting *new* releases for artists in your library).

Current numbers from the live database:

| Artist | Albums | Tracks |
|---|---|---|
| Ludwig Van Beethoven | 4,813 | 142,361 |
| Bruce Springsteen | 1,849 | 45,464 |
| Bob Dylan | 975 | 23,147 |
| Elvis Presley | 965 | 32,793 |
| Metallica | 943 | 16,834 |

These are composers or legacy artists with massive back-catalogs of compilations, bootlegs, and reissues — not the kind of artists where you'd meaningfully track "new" releases.

## Possible approaches

- [ ] **Album count cap**: set an upper limit (e.g., 500) on albums fetched per artist; skip or truncate beyond that
- [ ] **Filter by release type**: skip compilations, bootlegs, DJ-mixes, etc. — only fetch studio albums and EPs (may already be partially done via `PrimaryType` filtering)
- [ ] **"Composer" vs "artist" distinction**: use MusicBrainz artist type metadata to detect composers and skip or handle them differently
- [ ] **Per-artist override**: allow marking an artist to skip sync entirely (beyond the existing monitor status)
- [ ] **Lazy/paginated fetch**: only fetch first N pages and stop, with a UI indicator that results are capped

## Notes

- The existing `monitor` status (`always`/`sometimes`/`never`) could be extended, or a separate mechanism could be added
- Classical composers are technically not "artists" on the local albums — they're the composer. The actual performing artist is different. MusicBrainz conflates these
- Should also consider deduplication of artist name variants (e.g., "Ludwig Van Beethoven" vs "Ludwig van Beethoven" have separate entries with 99k and 43k tracks respectively)
