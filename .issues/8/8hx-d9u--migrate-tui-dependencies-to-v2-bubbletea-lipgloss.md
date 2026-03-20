---
# 8hx-d9u
title: Migrate TUI dependencies to v2 (bubbletea, lipgloss, bubbles)
status: completed
type: task
priority: normal
created_at: 2026-03-20T20:23:52Z
updated_at: 2026-03-20T20:25:41Z
sync:
    github:
        issue_number: "47"
        synced_at: "2026-03-20T21:31:30Z"
---

Migrate from Charmbracelet v1 to v2:

- [x] Update module paths in go.mod (`charm.land/*/v2`)
- [x] `View() string` → `View() tea.View` with `tea.NewView(s)`
- [x] `tea.KeyMsg` → `tea.KeyPressMsg` everywhere
- [x] Remove `tea.WithAltScreen()` (handled automatically in v2)
- [x] Update all import paths across TUI package
- [x] Run `go mod tidy` and verify build
- [x] Run tests and lint


## Summary of Changes

Migrated from Charmbracelet v1 to v2:
- bubbletea v1.3.10 → charm.land/bubbletea/v2 v2.0.2
- lipgloss v1.1.0 → charm.land/lipgloss/v2 v2.0.2
- bubbles v1.0.0 → charm.land/bubbles/v2 v2.0.0

API changes:
- `View() string` → `View() tea.View` with `tea.NewView(s)`
- `tea.KeyMsg` → `tea.KeyPressMsg` (11 occurrences)
- Removed `tea.WithAltScreen()` program option (automatic in v2)
- Removed `FilterPrompt`/`FilterCursor` styles (not in v2 list.Styles)
