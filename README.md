# musup-go

A terminal app that lives in your music folder. It scans audio file metadata, catalogs your artists, and checks [MusicBrainz](https://musicbrainz.org/) for albums you might be missing. The MusicBrainz part is rate-limited to one request per second, so syncing a large library is a *meditative* experience. Bring a book.

Built on [dhowden/tag](https://github.com/dhowden/tag) for metadata parsing and the [Bubble Tea](https://charm.sh/) stack for the terminal UI. The MusicBrainz client is homegrown, inspired by API patterns in [michiwend/gomusicbrainz](https://github.com/michiwend/gomusicbrainz).

## Install

### Homebrew (macOS)

```
brew install toba/tap/musup-go
```

The command is `musup-go` on macOS and Linux.

### Scoop (Windows)

```
scoop bucket add toba https://github.com/toba/scoop-bucket
scoop install musup
```

The command is `musup` on Windows.

### From source

```
go install github.com/toba/musup-go@latest
```

Requires Go 1.26+.

## Usage

```
cd ~/Music
musup-go
```

That's it. On launch, musup scans the current directory for music files and populates the TUI in real-time as artists are discovered. Subsequent launches re-scan automatically — only files whose size or mtime changed get re-read, so it's fast after the first run.

The `--db` flag overrides the database location if you don't want `.musup.db` in your music folder, though honestly it's a single file and SQLite is already in your life whether you know it or not.

## Keyboard shortcuts

| Key | Action |
|-----|--------|
| `↑` `↓` | Move cursor up/down within a column |
| `←` `→` | Move cursor between columns; wraps across pages |
| `⇧↑` `⇧↓` | Jump to next/previous followed artist |
| `space` | Toggle follow status; advances cursor for rapid toggling |
| `pgup` `pgdn` | Previous/next page |
| `a`–`z` | Type-to-search; jumps to the first matching artist name |
| `1`–`9` | Filter to artists with no MB release in N years (`0` clears) |
| `enter` | Open artist search in default browser (allmusic.com) |
| `=` | Toggle track listing in the discography pane |
| `*` | Sync releases from MusicBrainz for all followed artists |
| `,` | Mark artist as caught up — dismisses the `*` badge and moves newer releases to a "reviewed" section in the pane. Press again to undo. |
| `-` | Vacuum the database |
| `/` | Toggle between followed artists only and all artists |
| `.` | Filter to inactive (deceased/disbanded) artists; fetches from MB if needed |
| `?` | Help overlay |
| `esc` | Cancel sync (if running) or quit |

## How it works

1. **Scan** — on startup, walks the current directory for music files, reads ID3/Vorbis/MP4/ASF tags, and stores metadata in SQLite. Each file is linked to an artist via integer foreign key. The `AlbumArtist` tag determines the primary artist; files with only an `Artist` tag are flagged as non-album-artist so they don't clutter the list. The TUI populates in real-time as files are processed.

2. **Browse** — a two-column paginated grid shows every album artist with an always-visible discography pane on the right. Followed artists get a green `✓`; unfollowed are muted. Type letters to jump to an artist, press digits to filter by release recency, or `.` to show inactive artists. The pane shows your local albums — press `=` to expand individual tracks. Press `enter` to look up the selected artist on allmusic.com.

3. **Sync** — press `*` to query MusicBrainz for each followed artist's discography. Results are cached; only artists past the stale window (7 days) get re-checked. Artists with new albums you don't own get a yellow `*` badge, and their releases appear under "NEWER RELEASES" in the discography pane.

4. **Review** — press `,` to mark an artist as caught up. Their `*` badge disappears and the releases move to a "REVIEWED" section in the pane. If new albums appear after a future sync, the badge comes back. Press `,` again to undo.

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
