# musup-go

TUI that scans a folder of music files, catalogs artists, and checks MusicBrainz for new releases.

The binary is `musup-go` on macOS and Linux. The binary stays `musup` on Windows. The database file stays `.musup.db`.

## Rules

- ALWAYS create a `jig todo` issue FIRST before starting any work (bug fix, feature, task, refactor). Set it to `in-progress` and keep it updated as you work
- For bugs: ALWAYS write a failing test FIRST that reproduces the bug, then fix the code to make it pass
- Run `scripts/lint.sh` after editing Go files
- NEVER commit unless the user explicitly asks you to

## Testing with real data

The real music library and database live at:
- Music root: `/Users/jason/Library/Mobile Documents/com~apple~CloudDocs/Music`
- DB: `/Users/jason/Library/Mobile Documents/com~apple~CloudDocs/Music/.musup.db`

**Always copy the DB before mutating it** when running benchmarks or experiments:
```bash
cp "/Users/jason/Library/Mobile Documents/com~apple~CloudDocs/Music/.musup.db" /tmp/musup-test.db
```

## Build & Test

```bash
go build -o musup-go .
go test ./...
go vet ./...
scripts/lint.sh        # golangci-lint with auto-fix, then report remaining issues
```

## Architecture

- `cmd/` — CLI entry point (flag parsing, dependency wiring)
- `internal/tui/` — Bubble Tea TUI (artist browser, discography pane, sync/scan UI)
- `internal/scan/` — music file scanning and metadata extraction
- `internal/check/` — MusicBrainz release sync and inactive status lookup
- `internal/db/` — SQLite database, migrations, sqlc-generated queries
- `internal/integration/musicbrainz/` — rate-limited MusicBrainz WS2 client
