# Changelog

## Week of Mar 15 – Mar 21, 2026

### ✨ Features

- Merge artists with names differing only by case or punctuation
- Filter artist list to show only artists with newer albums than local
- Highlight entire row when navigating artist list
- Add background task spinner in top-right corner
- Update TUI footer help text; add help modal
- Auto-sync when following an artist
- Follow toggle runs background sync instead of modal
- Add prune command to delete unfollowed artist catalog data
- Mark artist as reviewed through a particular album

### 🐛 Fixes

- Help modal width causes descriptions to wrap
- Background spinner not visible when first line fills terminal width ([#44](https://github.com/toba/musup/issues/44))
- Startup hangs; `ArtistSummaries` query takes ~51s on large databases
- Duplicate albums in check output
- Artist view shows track count but album view shows zero tracks
- Album detail shows 0 tracks despite artist summary showing track count
- Status modal enter key doesn't dismiss modal
- Fix inconsistent local album count for synced artists
- Filter `n` shortcut only shows followed artists with new releases
- Fix column alignment in artist list

### 🗜️ Tweaks

- Simplify monitor status to boolean `followed`
- Show cached artist list instantly on startup; scan in background
- `scripts/install.sh`; build and install to brew location
- Consolidate per-project Homebrew taps into single `toba/homebrew-tap`
- Refactor schema to use integer PKs instead of name-based composite keys
- Speed up startup; in-memory file change detection
- Migrate TUI dependencies to v2 (`bubbletea`, `lipgloss`, `bubbles`)
- Extract shared TUI helpers; simplify scan mutex pattern; add `boolToInt`; deduplicate `bgView`

## Week of Mar 8 – Mar 14, 2026

### ✨ Features

- Cap album/track fetching for excessive MusicBrainz results
- Implement MusicBrainz client for release lookups ([#4](https://github.com/toba/musup/issues/4))
- Add monitor status modal (`s` shortcut) for artists ([#12](https://github.com/toba/musup/issues/12))
- Bulk sync (`Shift+U`) for `MonitorAlways` artists
- Add WMA file support via minimal ASF parser

### 🐛 Fixes

- Track matching fails for artists with MusicBrainz title variations ([#6](https://github.com/toba/musup/issues/6))
- Fix track matching; fuzzy matching via normalized titles ([#14](https://github.com/toba/musup/issues/14))
- Fix list view column spacing; use dynamic name column width and rune-aware padding ([#11](https://github.com/toba/musup/issues/11))
- Local tracks show 0 in album detail for 10,000 Maniacs despite correct count in list view ([#10](https://github.com/toba/musup/issues/10))
- Fix `U` (bulk sync) command doing nothing in artist list
- Default sort not stripping leading articles ("A", "The") on initial load

### 🗜️ Tweaks

- Find API source for artist album releases ([#5](https://github.com/toba/musup/issues/5))
- Find Go library to read audio file metadata ([#3](https://github.com/toba/musup/issues/3))
- Design and implement state management ([#2](https://github.com/toba/musup/issues/2))
- Auto-migrate SQLite database with version tracking ([#7](https://github.com/toba/musup/issues/7))
- Skip MusicBrainz track fetch for known albums ([#15](https://github.com/toba/musup/issues/15))
- Change default monitor status to `MonitorAlways`
- Implement all `goptimize` findings; extract shared TUI helpers; replace rate-limit mutex with `rate.Limiter`; parallelize `readTags` with `errgroup`
