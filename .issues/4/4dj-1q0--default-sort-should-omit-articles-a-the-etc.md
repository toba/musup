---
# 4dj-1q0
title: Default sort should omit articles (A, The, etc.)
status: review
type: bug
priority: normal
created_at: 2026-03-11T01:26:48Z
updated_at: 2026-03-11T01:30:18Z
sync:
    github:
        issue_number: "21"
        synced_at: "2026-03-11T01:32:09Z"
---

Artists starting with articles like "A" and "The" are sorted by the article instead of the next word. For example, "A Fine Frenzy" should sort under F, not A.

- [x] Find the sorting logic
- [x] Write a failing test
- [x] Fix the sort to strip leading articles
- [x] Run lint


## Summary of Changes

`newListModel` in `internal/tui/list.go` was not calling `sortArtists` on initial load, so artists appeared in database order (literal alphabetical). Added `sortArtists(items, sortByName)` before populating the list, so articles ("A", "The") are stripped on first render.

Also added `internal/tui/sort_test.go` with tests for both sort modes.
