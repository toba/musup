package scan

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dhowden/tag"
	"github.com/toba/musup/internal/state"
)

var supportedExts = map[string]struct{}{
	".flac": {},
	".mp3":  {},
	".m4a":  {},
	".mp4":  {},
	".aac":  {},
}

// Scan walks root for music files, reads metadata, and updates db.
func Scan(ctx context.Context, db *state.DB, root string) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve root: %w", err)
	}

	livePaths := make(map[string]struct{})

	err = filepath.WalkDir(root, func(absPath string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil //nolint:nilerr // skip unreadable entries
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if d.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(absPath))
		if _, ok := supportedExts[ext]; !ok {
			return nil
		}

		relPath, relErr := filepath.Rel(root, absPath)
		if relErr != nil {
			return nil //nolint:nilerr // skip files we can't make relative
		}
		livePaths[relPath] = struct{}{}

		info, infoErr := d.Info()
		if infoErr != nil {
			return nil //nolint:nilerr // skip files we can't stat
		}

		changed, err := db.FileChanged(relPath, info.Size(), info.ModTime())
		if err != nil {
			return fmt.Errorf("check changed %s: %w", relPath, err)
		}
		if !changed {
			return nil
		}

		artist, album, title, trackNum := readTags(absPath)

		return db.UpsertFile(state.FileRecord{
			Path:        relPath,
			Size:        info.Size(),
			ModTime:     info.ModTime(),
			Artist:      artist,
			Album:       album,
			Title:       title,
			TrackNumber: trackNum,
			ScannedAt:   time.Now(),
		})
	})
	if err != nil {
		return err
	}

	_, err = db.RemoveStaleFiles(livePaths)
	if err != nil {
		return fmt.Errorf("remove stale files: %w", err)
	}

	return nil
}

func readTags(path string) (artist, album, title string, trackNumber int) {
	f, err := os.Open(path)
	if err != nil {
		return "", "", "", 0
	}
	defer func() { _ = f.Close() }()

	m, err := tag.ReadFrom(f)
	if err != nil {
		return "", "", "", 0
	}

	artist = m.AlbumArtist()
	if artist == "" {
		artist = m.Artist()
	}
	trackNumber, _ = m.Track()
	return artist, m.Album(), m.Title(), trackNumber
}
