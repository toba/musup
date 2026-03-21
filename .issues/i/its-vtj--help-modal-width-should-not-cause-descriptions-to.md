---
# its-vtj
title: Help modal width should not cause descriptions to wrap
status: completed
type: bug
priority: normal
created_at: 2026-03-21T15:54:58Z
updated_at: 2026-03-21T15:58:00Z
sync:
    github:
        issue_number: "54"
        synced_at: "2026-03-21T16:12:07Z"
---

The help modal (keyboard shortcuts overlay) is too narrow, causing description text to wrap awkwardly. For example, 'Toggle new releases filter' wraps mid-phrase, and 'Sync all followed artists' wraps. The modal should be wider so descriptions fit on a single line.

## Summary of Changes\n\nChanged help modal width from 34 to 44 in `internal/tui/helpers.go:107` so descriptions no longer wrap.
