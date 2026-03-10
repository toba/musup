---
# gri-d4p
title: Interactive artist browser TUI
status: draft
type: feature
priority: normal
created_at: 2026-03-10T19:43:34Z
updated_at: 2026-03-10T19:43:34Z
sync:
    github:
        issue_number: "1"
        synced_at: "2026-03-10T20:06:26Z"
---

Add a `musup browse` command that opens a Bubbletea-powered interactive list of all artists (and their albums) from the local state DB.

## UX

### Layout
- Full-terminal list view showing artists from the `.musup.db` state
- Each row: artist name, album count, newest album title, newest album date
- Responsive columns (hide album title on narrow terminals)

### Filtering
- `/` to enter filter mode (substring match on artist name, case-insensitive)
- `//` for deep search (also matches album names)
- `Esc` or `Backspace` to clear filter
- Filter prompt styled with accent color

### Sorting
- `o` to open sort picker modal
- Sort modes:
  - **Name** (default) — alphabetical by artist name
  - **Newest release** — most recent local album date first
  - **Album count** — most albums first
- Current sort indicated in picker

### Detail view
- `Enter` on an artist to expand/show album list inline or in a detail pane
- Albums listed with title and year
- `Esc` to go back to artist list

### Key bindings
- `j`/`k` or arrows — navigate
- `Enter` — view artist detail
- `/` — filter
- `o` — sort picker
- `q` / `Ctrl+C` — quit
- `?` — help overlay

## Dependencies
- `github.com/charmbracelet/bubbletea`
- `github.com/charmbracelet/bubbles`
- `github.com/charmbracelet/lipgloss`

## New packages
- `internal/tui/` — Bubbletea app, models, views, key bindings, styles

## Files to create/modify
| File | Action |
|------|--------|
| `internal/tui/app.go` | Create — top-level model, Init/Update/View |
| `internal/tui/list.go` | Create — artist list model, filtering, rendering |
| `internal/tui/detail.go` | Create — artist detail view (album list) |
| `internal/tui/sort.go` | Create — sort picker modal |
| `internal/tui/keys.go` | Create — key bindings |
| `internal/tui/styles.go` | Create — lipgloss styles, color palette |
| `cmd/browse.go` | Create — `browse` command, opens DB + launches TUI |
| `go.mod` / `go.sum` | Modified by `go get` |

## Data source
All data comes from `state.DB`:
- `UniqueArtists()` for the artist list
- `LocalAlbums(artist)` for album detail
- May need a new `state.DB` method: `ArtistSummaries() ([]ArtistSummary, error)` that returns artist name, album count, newest album, and newest album year in a single query for efficient list population

## Out of scope
- MusicBrainz integration (that's the `check` command)
- Editing or modifying state from the TUI
- Multi-select / batch operations
