package cmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/toba/musup-go/internal/check"
	"github.com/toba/musup-go/internal/db"
	"github.com/toba/musup-go/internal/integration/musicbrainz"
	"github.com/toba/musup-go/internal/scan"
	"github.com/toba/musup-go/internal/tui"
)

var (
	ver    = "dev"
	commit = "none"
	date   = "unknown"
)

func Execute() {
	dbFlag := flag.String("db", "", "path to state database (default: .musup.db in current dir)")
	versionFlag := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("musup %s (%s) built %s\n", ver, commit, date)
		return
	}

	if err := run(*dbFlag); err != nil {
		fmt.Fprintf(os.Stderr, "musup: %v\n", err)
		os.Exit(1)
	}
}

func run(dbPath string) error {
	d, dp, err := openDB(dbPath)
	if err != nil {
		return err
	}
	defer func() { _ = d.Close() }()

	musicRoot := filepath.Dir(dp)
	mb := musicbrainz.New("musup", ver, "https://github.com/toba/musup-go")

	fetchInactive := func(ctx context.Context, d *db.DB, onProgress func(string)) (map[int64]bool, error) {
		return check.FetchInactiveStatus(ctx, d, mb, onProgress)
	}
	syncArtist := func(ctx context.Context, d *db.DB, artistID int64) error {
		return check.SyncArtist(ctx, d, mb, artistID)
	}
	scanFn := func(ctx context.Context, d *db.DB) error {
		return scan.Scan(ctx, d, musicRoot)
	}

	p := tea.NewProgram(tui.New(d, musicRoot, fetchInactive, syncArtist, scanFn))
	finalModel, err := p.Run()
	if err != nil {
		return err
	}
	if m, ok := finalModel.(tui.Model); ok && m.Err() != nil {
		return m.Err()
	}
	return nil
}

func openDB(dbPath string) (*db.DB, string, error) {
	dp := dbPath
	if dp == "" {
		root, err := os.Getwd()
		if err != nil {
			return nil, "", fmt.Errorf("get working directory: %w", err)
		}
		dp = filepath.Join(root, ".musup.db")
	}
	d, err := db.Open(dp)
	if err != nil {
		return nil, "", fmt.Errorf("open db: %w", err)
	}
	return d, dp, nil
}
