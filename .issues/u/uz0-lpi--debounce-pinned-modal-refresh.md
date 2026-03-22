---
# uz0-lpi
title: Debounce pinned modal refresh
status: completed
type: task
priority: normal
created_at: 2026-03-22T20:39:25Z
updated_at: 2026-03-22T20:40:06Z
sync:
    github:
        issue_number: "67"
        synced_at: "2026-03-22T21:40:44Z"
---

- [x] Add pinnedGen counter + pinnedRefreshMsg to Model\n- [x] On pinned navigation, show lightweight placeholder and schedule tick\n- [x] On tick, rebuild full discography modal if gen matches


## Summary of Changes

Added 50ms debounce to pinned modal refresh using generation counter pattern (matching existing searchGen/yearInputGen). On each navigation keystroke, the artist name updates immediately but the full discography rebuild is deferred. Rapid keystrokes cancel pending rebuilds so only the final position triggers a DB query.
