---
# jvw-ybz
title: 'Track count mismatch persists: artist list shows files but album detail shows 0 for guest/compilation artists'
status: completed
type: bug
priority: normal
created_at: 2026-03-20T21:22:50Z
updated_at: 2026-03-21T16:37:36Z
sync:
    github:
        issue_number: "51"
        synced_at: "2026-03-21T16:38:30Z"
---

## Problem

Artist list shows 1 track for artists like Caoimhín Ó Raghallaigh and Bruce Kaphan, but album detail view shows 0 local tracks on every album. This is the same symptom as #32 (1lk-md3) but a different underlying cause that wasn't fully resolved.

## Root Cause Analysis

There are **two independent track counting systems** that fundamentally cannot agree for certain artists:

1. **Artist list view** (`ArtistSummaries`): Counts rows in `files` table → always correct (file exists = counted)
2. **Album detail view** (`Albums`): Counts `tracks.local` flags set by `MarkLocalTracks()` fuzzy matching

`MarkLocalTracks()` (`db.go:1140-1172`) matches files to catalog tracks by:
- First: `files.artist_norm = ar.name_norm AND files.album_norm = al.title_norm AND (title OR track_number match)`
- Fallback: `files.artist_norm = ar.name_norm AND files.title_norm = tracks.title_norm` (title-only, ignoring album)

**Both clauses fail** when:
- The file's album doesn't exist in the artist's MusicBrainz discography
- AND the track title doesn't appear in any of the artist's catalog albums

### Why this happens

The user tagged per-track artists on compilation/guest tracks (e.g., a track saying "with Bruce Kaphan" gets artist="Bruce Kaphan" in the metadata). The scanner creates `files` records under that artist, but:

- **Caoimhín Ó Raghallaigh**: File is `Compilations/In the Echo: Field Recordings.../07 The Campanile.flac`. Album "In the Echo..." is NOT in this artist's MusicBrainz discography (it's a compilation). Track "The Campanile" doesn't appear in any of the artist's 10 catalog albums either.
- **Bruce Kaphan**: File is `Bruce Kaphan/Hybrid/01 Pohaka La.flac`. Album "Hybrid" is NOT in the catalog (catalog only has "Pelican Dreams" and "Slider"). Track "Pohaka La" isn't in those albums either.

### Why fix #32 didn't help

The previous fix added `max(fileCount, catalogMatchCount)` in `ArtistSummaries` Go code (db.go:821-824) so the artist list never drops below the file-based count. But this is a **band-aid at the summary level only** — the album detail view still reads `tracks.local` directly from the database, which is 0 because matching failed.

### History of this bug

This is the **5th issue** about track counting discrepancies:
- p63-7o4 (#10): 10,000 Maniacs — fixed normalization
- sx2-isj (#6): MusicBrainz title variations — fixed fuzzy matching
- 4qr-gn0 (#31): Artist view vs album view mismatch
- 1lk-md3 (#32): Amy Lee — added max() band-aid
- **This issue**: Fundamental mismatch for guest/compilation artists

## Possible Approaches (needs decision)

1. **Show "Unmatched Local Files" entry** in album detail — add a virtual row showing files that exist but couldn't match any catalog album
2. **Distinguish track artist vs album artist** at scan time — only count files under the album artist, not per-track guest artists
3. **Improve MarkLocalTracks matching** — more aggressive fuzzy matching (won't help when the track truly isn't in the catalog)

## Data for reproduction

```sql
-- Caoimhín Ó Raghallaigh: 1 file, 0 matched tracks
SELECT * FROM files WHERE artist_norm = 'caoimhín ó raghallaigh';
-- path: Compilations/In the Echo.../07 The Campanile.flac
-- album_norm: in the echo field recordings from earlsfort terrace

SELECT al.title_norm FROM albums al JOIN artists ar ON ar.id = al.artist_id WHERE ar.name_norm = 'caoimhín ó raghallaigh';
-- 10 albums, none matching "in the echo..."

-- Bruce Kaphan: 1 file, 0 matched tracks
SELECT * FROM files WHERE artist_norm = 'bruce kaphan';
-- path: Bruce Kaphan/Hybrid/01 Pohaka La.flac, album_norm: hybrid

SELECT al.title_norm FROM albums al JOIN artists ar ON ar.id = al.artist_id WHERE ar.name_norm = 'bruce kaphan';
-- pelican dreams, slider — no "hybrid"
```

## Summary of Changes

Added `is_album_artist` flag to the `files` table (migration v11). During scanning, `readTags()` now returns whether the artist came from the `AlbumArtist` tag (true) or fell back to the `Artist` tag (false). The `ArtistSummaries` query filters out artists who are only track artists (`HAVING MAX(is_album_artist) = 1`), so guest/compilation artists like Bruce Kaphan and Caoimhín Ó Raghallaigh no longer appear in the list.

Files changed:
- `internal/state/model/schema.sql` — v11 schema with `is_album_artist` column
- `internal/state/model/file.sql` — UpsertFile includes new column
- `internal/state/model/artist.sql` — HAVING clause in ArtistSummaries
- `internal/state/db.go` — migration v10→11, FileRecord struct, UpsertFile wrapper
- `internal/scan/scan.go` — readTags returns isAlbumArtist
- `internal/scan/asf.go` — readASF/parseASF return isAlbumArtist
