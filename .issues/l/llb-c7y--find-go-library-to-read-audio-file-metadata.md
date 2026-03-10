---
# llb-c7y
title: Find Go library to read audio file metadata
status: completed
type: task
priority: normal
created_at: 2026-03-10T19:05:48Z
updated_at: 2026-03-10T19:18:25Z
sync:
    github:
        issue_number: "3"
        synced_at: "2026-03-10T20:06:26Z"
---

Find a Go library that can read ID3/tag metadata (artist, album, year) from music files.

Required format support:
- MP3 (ID3v2)
- FLAC (Vorbis comments)
- ALAC / M4A (MP4 atoms)
- AAC / MP4A

Evaluate dhowden/tag, mikkyang/id3-go, and any other maintained options.
