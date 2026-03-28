# Changelog

## Week of Mar 22 – Mar 28, 2026

### ✨ Features

- TUI for toggling followed artists; `musup <N>` keeps CLI mode ([#61](https://github.com/toba/musup/issues/61))
- ~~Pinnable discography modal (`p` key)~~ ([#64](https://github.com/toba/musup/issues/64))
- Shift+Up/Down to jump to next followed artist ([#69](https://github.com/toba/musup/issues/69))
- Add `*` shortcut for new releases check in TUI ([#66](https://github.com/toba/musup/issues/66))
- `/` key toggles un-followed artist filtering in artist list view ([#73](https://github.com/toba/musup/issues/73))
- Replace third column with always-visible local discography pane ([#74](https://github.com/toba/musup/issues/74))
- Replace `*` key new-releases mode with inline background sync ([#81](https://github.com/toba/musup/issues/81))
- `,` key marks artist as caught up on releases ([#77](https://github.com/toba/musup/issues/77))
- Remove subcommands; TUI-only with auto-scan on startup ([#80](https://github.com/toba/musup/issues/80))
- `enter` opens artist search URL; settings table for configuration ([#76](https://github.com/toba/musup/issues/76))
- Add search URL edit modal; `:` key opens `textinput` with validation ([#83](https://github.com/toba/musup/issues/83))
- Hidden `^` command to download 7digital zip files; concurrent downloads with progress ([#88](https://github.com/toba/musup/issues/88))
- Create GitHub Pages static site ([#82](https://github.com/toba/musup/issues/82))

### 🐞 Fixes

- Setting reviewed can fail to include newest albums with future release dates ([#87](https://github.com/toba/musup/issues/87))
- Artists matching wrong albums; partial name matches (e.g. Bush matches Kate Bush albums) ([#86](https://github.com/toba/musup/issues/86))
- Fetching Inactive Status modal fails to disappear after fetch completes ([#62](https://github.com/toba/musup/issues/62))
- Discography modal shows duplicate tracks due to JOIN on albums with same `title_norm` ([#65](https://github.com/toba/musup/issues/65))
- Pinned discography modal should not show track selection or arrow navigation ([#63](https://github.com/toba/musup/issues/63))
- Truncate long track names in discography modal with ellipsis ([#68](https://github.com/toba/musup/issues/68))
- Use marker instead of reverse-video for selected artist in grid ([#72](https://github.com/toba/musup/issues/72))
- Fix `musup N` to only show albums newer than latest local ([#71](https://github.com/toba/musup/issues/71))

### 🗜️ Tweaks

- Debounce pinned modal refresh ([#67](https://github.com/toba/musup/issues/67))
- Style discography modal title to match artist list ([#70](https://github.com/toba/musup/issues/70))
- Filter MB browse to official releases only ([#75](https://github.com/toba/musup/issues/75))
- Clear stale album data after `website-default` filter change ([#79](https://github.com/toba/musup/issues/79))
- Apply `goptimize` findings; extract helpers and constants; add check package tests ([#78](https://github.com/toba/musup/issues/78))
- Test `FollowedNewerReleases` excludes local albums ([#84](https://github.com/toba/musup/issues/84))
- Evaluate Zed GPUI as alternative to Bubble Tea TUI ([#85](https://github.com/toba/musup/issues/85))

## Week of Mar 15 – Mar 21, 2026

### ✨ Features

- ~~Simplify to single-argument CLI; `musup [years]` replaces interactive TUI~~ ([#60](https://github.com/toba/musup/issues/60))
- Merge artists with names differing only by case or punctuation
- Filter artist list to show only artists with newer albums than local
- ~~Highlight entire row when navigating artist list~~
- Add number key shortcuts (1-9) to filter artist list by release recency ([#57](https://github.com/toba/musup/issues/57))
- ~~Add background task spinner in top-right corner~~
- Update TUI footer help text; add help modal
- Auto-sync when following an artist
- Follow toggle runs background sync instead of modal
- Add prune command to delete unfollowed artist catalog data
- ~~Mark artist as reviewed through a particular album~~

### 🐞 Fixes

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
- 0 artists shown when filtering by release recency; `artists.latest_date` was never populated ([#59](https://github.com/toba/musup/issues/59))
- Hide guest/compilation artists who aren't album artists on any files ([#51](https://github.com/toba/musup/issues/51))

### 🗜️ Tweaks

- Simplify monitor status to boolean `followed`
- Show cached artist list instantly on startup; scan in background
- `scripts/install.sh`; build and install to brew location
- Consolidate per-project Homebrew taps into single `toba/homebrew-tap`
- Refactor schema to use integer PKs instead of name-based composite keys
- Speed up startup; in-memory file change detection
- Migrate TUI dependencies to v2 (`bubbletea`, `lipgloss`, `bubbles`)
- Extract shared TUI helpers; simplify scan mutex pattern; add `boolToInt`; deduplicate `bgView`
- Drop unused `artists.latest_date` and `artists.latest_release` columns; derive from `albums.release_date` ([#58](https://github.com/toba/musup/issues/58))
- Implement `sqlc` for type-safe database queries; flatten `internal/state/model` into `internal/db` ([#56](https://github.com/toba/musup/issues/56))

## Week of Mar 8 – Mar 14, 2026

### ✨ Features

- Cap album/track fetching for excessive MusicBrainz results
- Implement MusicBrainz client for release lookups ([#4](https://github.com/toba/musup/issues/4))
- ~~Add monitor status modal (`s` shortcut) for artists~~ ([#12](https://github.com/toba/musup/issues/12))
- ~~Bulk sync (`Shift+U`) for `MonitorAlways` artists~~
- Add WMA file support via minimal ASF parser

### 🐞 Fixes

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
