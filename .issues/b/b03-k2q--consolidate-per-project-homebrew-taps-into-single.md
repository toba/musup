---
# b03-k2q
title: Consolidate per-project Homebrew taps into single toba/homebrew-tap
status: completed
type: task
priority: normal
created_at: 2026-03-20T19:36:40Z
updated_at: 2026-03-20T19:41:15Z
sync:
    github:
        issue_number: "38"
        synced_at: "2026-03-20T19:57:53Z"
---

Replace per-project Homebrew tap repos with a single `toba/homebrew-tap` repo (like charmbracelet/homebrew-tap).

## Current state
Per-project taps: `homebrew-musup`, `homebrew-jig`, `homebrew-xc-mcp`, `homebrew-swiftiomatic`, `homebrew-go-bigq`

## Plan
- [x] Create `toba/homebrew-tap` repo on GitHub
- [x] Populate with all existing formulae (musup, jig, ja, xc-mcp, swiftiomatic, go-bigq)
- [x] Add README with install instructions
- [x] Update musup release workflow, README, and .jig.yaml to use `homebrew-tap`
- [x] Update other projects' release workflows (jig, xc-mcp, swiftiomatic, go-bigq) — pushed to main
- [x] Archive old per-project tap repos — all 5 archived


## Summary of Changes

Consolidated 5 per-project Homebrew tap repos into a single `toba/homebrew-tap`.

- Created `toba/homebrew-tap` with 6 formulae (musup, jig, ja, xc-mcp, swiftiomatic, go-bigq)
- Updated release workflows in all 5 source repos to push to `homebrew-tap`
- Updated READMEs and `.jig.yaml` companion references
- Archived: homebrew-musup, homebrew-jig, homebrew-xc-mcp, homebrew-swiftiomatic, homebrew-go-bigq
- Users install via `brew install toba/tap/<formula>`
