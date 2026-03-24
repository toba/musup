---
# otu-hy3
title: Add settings table and Enter key to open artist search URL
status: review
type: feature
priority: normal
created_at: 2026-03-24T22:20:25Z
updated_at: 2026-03-24T22:23:23Z
sync:
    github:
        issue_number: "76"
        synced_at: "2026-03-24T22:26:56Z"
---

- [x] Add settings table to schema.sql and migration v20
- [x] Create settings.sql with GetSetting/SetSetting queries
- [x] Regenerate sqlc
- [x] Add Enter key handler and openSearchURL in TUI
- [x] Load search_url via loadArtists into Model
- [x] Update help text
- [x] Update test version checks
- [x] Build, test, lint


## Summary of Changes

Added `settings` table (key-value) with migration v20. Seeded with `search_url` = `https://www.allmusic.com/search/artists/%s`. Press Enter on an artist to open their allmusic search page in the default browser. Checked for orphan code — modalConfirmFetch is still used by the `.` key inactive status fetch.
