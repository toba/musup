---
# muc-g7c
title: Add search URL edit modal (colon key)
status: review
type: feature
priority: normal
created_at: 2026-03-26T00:58:57Z
updated_at: 2026-03-26T01:00:53Z
sync:
    github:
        issue_number: "83"
        synced_at: "2026-03-26T01:02:18Z"
---

- [x] Add textinput import
- [x] Add modalSearchURL enum value
- [x] Add searchURLInput and searchURLErr fields to Model
- [x] Initialize textinput in New()
- [x] Route non-key messages to textinput when modal active
- [x] Handle : key in handleKey()
- [x] Handle Enter/Esc/typing in handleModalKey()
- [x] Render modal in View()
- [x] Add : to help content
- [x] Build, test, lint


## Summary of Changes

Added search URL edit modal to TUI, triggered by `:` key. Uses bubbles/v2 textinput with validation (must be http(s) URL containing %s). Persists to DB via existing SetSetting.
