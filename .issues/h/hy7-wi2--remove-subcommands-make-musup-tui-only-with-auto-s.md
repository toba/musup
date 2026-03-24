---
# hy7-wi2
title: Remove subcommands, make musup TUI-only with auto-scan
status: review
type: feature
priority: normal
created_at: 2026-03-24T21:59:55Z
updated_at: 2026-03-24T22:08:48Z
sync:
    github:
        issue_number: "80"
        synced_at: "2026-03-24T22:26:57Z"
---

- [x] Remove cmd/scan.go and scan subcommand registration
- [x] Remove runCheck / CLI check mode from cmd/root.go
- [x] Simplify cmd/root.go to TUI-only with flag package, pass scan func to TUI
- [x] Add ScanFunc type and scan state to TUI model
- [x] Start scan on Init(), reload artists every 500ms during scan
- [x] Show scanning status in TUI (spinner + SCANNING in help bar, empty state message)
- [x] Handle scan completion, update empty-state message
- [x] Tests pass
- [x] Build, test, lint clean
- [x] Update README and CLAUDE.md


## Summary of Changes

Removed all subcommands (scan, check/years, version via Cobra). Replaced Cobra with stdlib flag. musup now runs only as a TUI. On launch it auto-scans the CWD recursively, populating the view in real-time (re-queries DB every 500ms during scan). Added `-` key for VACUUM. Updated README and CLAUDE.md.
