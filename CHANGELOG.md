# Changelog

## Week of Mar 8 – Mar 14, 2026

### ✨ Features

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
