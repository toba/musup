# musup

CLI that scans a folder of music files and reports which artists have released new albums.

## Rules

- ALWAYS write a failing test before fixing bugs
- Run `scripts/lint.sh` after editing Go files

## Build & Test

```bash
go build -o musup .
go test ./...
go vet ./...
scripts/lint.sh        # golangci-lint with auto-fix, then report remaining issues
```

## Architecture

- `cmd/` — Cobra commands
  - `scan` — scan a music folder and extract artist names from file metadata
  - `check` — check for new releases from artists in your library
  - `version` — print version info
- `internal/scan/` — music file scanning and metadata extraction
- `internal/releases/` — new release lookup (MusicBrainz, etc.)
