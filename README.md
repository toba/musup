# musup

A CLI that lives in your music folder. It scans audio file metadata, catalogs your artists, and checks [MusicBrainz](https://musicbrainz.org/) for albums you might be missing. The MusicBrainz part is rate-limited to one request per second, so syncing a large library is a *meditative* experience. Bring a book.

Built on [dhowden/tag](https://github.com/dhowden/tag) for metadata parsing and the [Bubble Tea](https://charm.sh/) stack for the terminal UI. The MusicBrainz client is homegrown, inspired by API patterns in [michiwend/gomusicbrainz](https://github.com/michiwend/gomusicbrainz).

## Install

### Homebrew (macOS)

```
brew install toba/tap/musup
```

### Scoop (Windows)

```
scoop bucket add musup https://github.com/toba/scoop-musup
scoop install musup
```

### From source

```
go install github.com/toba/musup@latest
```

Requires Go 1.26+.

## Usage

### Browse artists

```
cd ~/Music
musup
```

Opens a three-column artist browser. All artists found via `AlbumArtist` tags are shown — guest and compilation-only artists are filtered out. Artists are sorted case-insensitively. You can follow/unfollow artists, view their local discography, open tracks in your default player, and filter by activity status.

### Check for new releases

```
musup 3
```

Syncs followed artists with MusicBrainz (skipping anyone checked in the last 7 days), then prints albums released in the last 3 years that you don't have locally. Only followed artists are synced — unfollowed artists are ignored entirely, which keeps the sync fast.

### Scan files

```
musup scan [path]
```

Walks the directory tree, reads tags, and upserts file metadata into the database. Incremental by default — only files whose size or mtime changed get re-read. Tag reading is parallelized across 8 workers.

### The `--db` flag

Overrides the database location if you don't want `.musup.db` in your music folder, though honestly it's a single file and SQLite is already in your life whether you know it or not.

## Keyboard shortcuts

The TUI has two contexts: the main artist grid and modal overlays.

### Artist grid

| Key | Action |
|-----|--------|
| `↑` `↓` | Move cursor up/down within a column |
| `←` `→` | Move cursor between columns; wraps across pages |
| `space` | Toggle follow status; advances cursor for rapid toggling |
| `enter` | Open discography modal showing local albums and tracks |
| `pgup` `pgdn` | Previous/next page |
| `a`–`z` | Type-to-search; jumps to the first matching artist name (debounced) |
| `1`–`9` | Filter to artists with no MB release in N years (debounced; `0` clears) |
| `.` | Filter to inactive (deceased/disbanded) artists |
| `?` | Help modal |
| `esc` | Quit |

### Discography modal

| Key | Action |
|-----|--------|
| `↑` `↓` | Select track |
| `enter` | Open selected track in default app (`open` on macOS, `xdg-open` on Linux) |
| `esc` | Close modal |

### Help and confirmation modals

| Key | Action |
|-----|--------|
| *any key* | Dismiss help modal |
| `enter` | Confirm action (e.g. fetch inactive status from MusicBrainz) |
| `esc` | Cancel / dismiss |

## How it works

1. **Scan** — walks the directory, reads ID3/Vorbis/MP4/ASF tags, stores metadata in SQLite. Each file is linked to an artist via integer foreign key. The `AlbumArtist` tag determines the primary artist; files with only an `Artist` tag are flagged as non-album-artist so they don't clutter the list.

2. **Browse** — a three-column paginated grid shows every album artist. Followed artists get a green `✓`; unfollowed are muted. Type letters to jump to an artist, press digits to filter by release recency, or `.` to show inactive artists. Press `enter` to see what you have locally — albums and tracks in a two-column modal with release years.

3. **Sync** — queries MusicBrainz for each followed artist's discography and stores albums locally. Results are cached; only artists past the stale window (7 days) get re-checked. The check command (`musup N`) then compares your local files against the MB catalog and shows what's new.

## Supported formats

| Extension | Format |
|-----------|--------|
| `.flac` | FLAC (Vorbis comments) |
| `.mp3` | MP3 (ID3v1/v2) |
| `.m4a` | AAC / Apple Lossless |
| `.mp4` | MPEG-4 audio |
| `.aac` | AAC |
| `.wma` | Windows Media Audio (ASF) |

## License

Apache-2.0
