# musup

A TUI that lives in your music folder. Point it at a directory of audio files and it scans metadata, catalogs your artists, and — when you ask — checks [MusicBrainz](https://musicbrainz.org/) for albums you might be missing. The MusicBrainz part is rate-limited to one request per second, so syncing a large library is a *meditative* experience. Bring a book.

No subcommands. Just `cd` into your music folder and run `musup`. It opens a local SQLite database (`.musup.db`), incrementally scans for new or changed files, then drops you into an interactive artist browser. Second run is fast — only changed files get re-read.

Built on [dhowden/tag](https://github.com/dhowden/tag) for metadata parsing and inspired by the API patterns in [michiwend/gomusicbrainz](https://github.com/michiwend/gomusicbrainz), though the MusicBrainz client is homegrown.

## Install

### Homebrew (macOS)

```
brew install toba/musup/musup
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

```
cd ~/Music
musup
```

That's it. The TUI takes over from there.

### Key bindings

| Key | Action |
|-----|--------|
| `/` | Filter artists by name |
| `o` | Sort — name, newest album, album count |
| `s` | Set monitor status — *monitor*, *sometimes*, or *ignore* |
| `Enter` | View an artist's albums; drill into track listing |
| `u` | Sync selected artist with MusicBrainz |
| `U` | Bulk sync all *monitored* artists |
| `Esc` | Back |
| `q` | Quit |

### Monitor status

Each artist has a monitor level that controls whether `U` (bulk sync) includes them:

- **Monitor** (default) — always included in bulk sync
- **Sometimes** — manual sync only (`u`)
- **Ignore** — never synced

Press `s` on any artist to change their status.

### The `--db` flag

Overrides the database location if you don't want `.musup.db` cluttering your music folder, though honestly it's a single file and SQLite is already in your life whether you know it or not.

## How it works

1. **Scan** — walks the directory tree, reads ID3/Vorbis/MP4/ASF tags, stores artist and album metadata in SQLite. Incremental by default; skips files whose size and mtime haven't changed. Tag reading is parallelized across 8 workers because life is short.

2. **Browse** — a [Bubble Tea](https://github.com/charmbracelet/bubbletea) TUI lists every artist with album counts, track ratios, and newest album title. Filter, sort, drill into album track listings. Local tracks are matched against the MusicBrainz catalog using fuzzy normalized titles — so *"Loser"* matches *"Loser (radio edit)"* without you losing sleep over it.

3. **Sync** — queries [MusicBrainz](https://musicbrainz.org/) for an artist's full discography, stores albums and tracks locally, and shows you what you have versus what exists. Supports single-artist sync (`u`) or bulk sync of all monitored artists (`U`). Already-fetched albums are skipped on subsequent syncs.

## Supported formats

| Extension | Format |
|-----------|--------|
| `.flac` | FLAC (Vorbis comments) |
| `.mp3` | MP3 (ID3v1/v2) |
| `.m4a` | AAC / Apple Lossless |
| `.mp4` | MPEG-4 audio |
| `.aac` | AAC |
| `.wma` | Windows Media Audio (ASF) |

WMA support uses a minimal built-in ASF header parser — no external dependencies, just enough to pull artist, album, title, and track number from the metadata objects.

## License

Apache-2.0
