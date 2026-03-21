---
# 9we-9i9
title: Background spinner not visible when first line fills terminal width
status: completed
type: bug
priority: normal
created_at: 2026-03-20T21:11:51Z
updated_at: 2026-03-21T15:49:38Z
sync:
    github:
        issue_number: "44"
        synced_at: "2026-03-21T16:12:06Z"
---

When pressing 'f' to follow an artist, the background task spinner in the top right doesn't appear. The overlay logic in app.go skips rendering when firstWidth >= m.width, but the bubbles list title bar always fills the full width, so the condition is never met.

## Tasks
- [ ] Fix overlay logic to truncate first line when needed to make room for spinner
- [ ] Verify spinner appears when following an artist
