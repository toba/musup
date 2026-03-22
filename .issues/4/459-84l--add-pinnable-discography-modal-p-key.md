---
# 459-84l
title: Add pinnable discography modal (p key)
status: completed
type: feature
priority: normal
created_at: 2026-03-22T20:07:49Z
updated_at: 2026-03-22T20:10:24Z
sync:
    github:
        issue_number: "64"
        synced_at: "2026-03-22T20:20:12Z"
---

- [x] Add pinned field to modalData struct
- [x] Add pinnedModalStyle with yellow border
- [x] Handle p key in discography modal to pin it
- [x] Delegate keys to handleKey when pinned, refresh modal after navigation
- [x] Use pinned style in View()
- [x] Update modal footer for pinned/unpinned states
- [x] Update help content
- [x] Build, test, lint

## Summary of Changes

Added pinnable discography modal feature to the TUI. When viewing an artist's discography (Enter), pressing 'p' pins the modal with a yellow border. While pinned, arrow keys navigate the artist list and the modal updates to show the selected artist's discography. Esc closes the pinned modal; Ctrl+C still quits.
