---
# c4d-vf6
title: Merge artists with names differing only by case or punctuation
status: completed
type: feature
priority: normal
created_at: 2026-03-18T18:23:44Z
updated_at: 2026-03-21T15:49:50Z
sync:
    github:
        issue_number: "27"
        synced_at: "2026-03-21T16:12:06Z"
---

Merge artists whose names differ only by upper/lower casing or basic punctuation (hyphen vs space, apostrophe vs none, etc.).

Examples:
- "Alice In Chains" and "Alice in Chains"
- "Guns N' Roses" and "Guns N Roses"

This could use the existing `Normalize()` function from `internal/state/norm.go` to detect duplicates during scan, then consolidate to a canonical form (e.g. the most common variant in the files).
