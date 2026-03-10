# musup

CLI that scans a folder of music files and reports which artists have released new albums.

## Rules

- ALWAYS create a `jig todo` issue FIRST before starting any work (bug fix, feature, task, refactor). Set it to `in-progress` and keep it updated as you work
- For bugs: ALWAYS write a failing test FIRST that reproduces the bug, then fix the code to make it pass
- Run `scripts/lint.sh` after editing Go files
- NEVER commit unless the user explicitly asks you to

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
