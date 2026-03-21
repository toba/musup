---
# f8p-w79
title: Drop artists.latest_date column
status: completed
type: task
priority: normal
created_at: 2026-03-21T18:26:20Z
updated_at: 2026-03-21T18:34:55Z
sync:
    github:
        issue_number: "58"
        synced_at: "2026-03-21T18:36:22Z"
---

- [x] Remove latest_date from schema in db.go
- [x] Remove from migrations in db.go
- [x] Remove from GetArtistByNameNorm query
- [x] Remove from UpdateArtistFull query
- [x] Regenerate sqlc
- [x] Update tests
- [x] Run tests and lint



## Summary of Changes

Dropped `latest_release` and `latest_date` columns from the `artists` table. These were never populated by the sync flow and are now derived from `albums.release_date` in the `ArtistSummaries` query. Added migration v12 to drop the columns from existing databases.
