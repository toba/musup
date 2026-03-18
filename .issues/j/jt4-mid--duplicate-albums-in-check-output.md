---
# jt4-mid
title: Duplicate albums in check output
status: completed
type: bug
priority: normal
created_at: 2026-03-18T19:03:17Z
updated_at: 2026-03-18T19:12:34Z
sync:
    github:
        issue_number: "30"
        synced_at: "2026-03-18T19:45:07Z"
---

Every album appears doubled in the `check` output for artists like Asaf Avidan. Each album row is listed twice with the same year and title but sometimes different track counts (e.g. `0/17` and `0/17`, or `1/13` and `2/13`).

Example from `musup check`:
```
2012  Avidan in a Box       0/17  [Liv…
2012  Avidan in a Box       0/17  [Liv…
2012  Different Pulses      0/11
2012  Different Pulses      5/11
2015  Gold Shadow           1/13
2015  Gold Shadow           2/13
…
```

## Expected
Each album should appear once with a single consolidated track count.

## Tasks
- [x] Write a failing test that reproduces the duplicate album rows
- [x] Identify the deduplication gap: `Albums()` query filtered by `artist_norm` but the PK `(artist_name, title)` allowed duplicate rows when the same artist was synced under different name variants
- [x] Fix deduplication logic: CTE deduplicates albums by `(artist_norm, title)` and joins tracks by `artist_norm`
- [x] Verify with `go test ./...`


## Summary of Changes

- `internal/state/db.go`: Rewrote `Albums()` query to deduplicate by `(artist_norm, title)` using a CTE, and join tracks by `artist_norm` instead of exact `artist_name`
- `internal/state/db_test.go`: Added `TestAlbums_DeduplicatesArtistNameVariants` that inserts the same album under two artist name casings and verifies only one row is returned with correct track counts
