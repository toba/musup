# musup

A TUI that lives in your music folder. Point it at a directory of `.flac`, `.mp3`, `.m4a`, `.mp4`, or `.aac` files and it scans metadata, catalogs your artists, and — eventually — tells you when they've released something new. The "eventually" part leans on [MusicBrainz](https://musicbrainz.org/), which is both wonderful and rate-limited to one request per second. *Patience is a virtue.*

No subcommands. Just `cd` into your music folder and run `musup`. It opens a local SQLite database (`.musup.db`), incrementally scans for new or changed files, then drops you into an interactive artist browser. Second run is fast — only changed files get re-read.

## Install

### Homebrew (macOS, Apple Silicon)

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

## Usage

```
cd ~/Music
musup
```

That's it. The TUI takes over from there:

- `/` — filter artists
- `o` — sort (name, newest album, album count)
- `Enter` — view an artist's albums
- `Esc` — back
- `q` — quit

The `--db` flag overrides the database location if you don't want `.musup.db` cluttering your music folder, though honestly it's a single file and SQLite is already in your life whether you know it or not.

## How it works

1. **Scan** — walks the directory tree, reads ID3/Vorbis/MP4 tags via [dhowden/tag](https://github.com/dhowden/tag), stores artist and album metadata in SQLite. Incremental by default; skips files whose size and mtime haven't changed.

2. **Browse** — a [Bubble Tea](https://github.com/charmbracelet/bubbletea) TUI lists every artist with album counts and newest album title. Filter, sort, drill into album lists.

3. **Check** *(coming soon)* — queries [MusicBrainz](https://musicbrainz.org/) for each artist's discography and flags releases you don't have locally. This is the whole *raison d'être* but it's not wired up yet. One thing at a time.

## Supported formats

| Extension | Format |
|-----------|--------|
| `.flac` | FLAC |
| `.mp3` | MP3 (ID3v1/v2) |
| `.m4a` | AAC / Apple Lossless |
| `.mp4` | MPEG-4 audio |
| `.aac` | AAC |

## License

Apache-2.0
