package scan

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dhowden/tag"
	"github.com/toba/musup/internal/state"
	"golang.org/x/sync/errgroup"
)

var supportedExts = map[string]struct{}{
	".flac": {},
	".mp3":  {},
	".m4a":  {},
	".mp4":  {},
	".aac":  {},
	".wma":  {},
}

// changedFile holds info for a file that needs tag reading.
type changedFile struct {
	absPath string
	relPath string
	size    int64
	modTime time.Time
}

// Scan walks root for music files, reads metadata, and updates db.
func Scan(ctx context.Context, db *state.DB, root string) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve root: %w", err)
	}

	livePaths := make(map[string]struct{})
	var changed []changedFile

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

		needsScan, err := db.FileChanged(relPath, info.Size(), info.ModTime())
		if err != nil {
			return fmt.Errorf("check changed %s: %w", relPath, err)
		}
		if !needsScan {
			return nil
		}

		changed = append(changed, changedFile{
			absPath: absPath,
			relPath: relPath,
			size:    info.Size(),
			modTime: info.ModTime(),
		})
		return nil
	})
	if err != nil {
		return err
	}

	// Read tags in parallel, then upsert sequentially (single DB connection).
	type tagResult struct {
		cf      changedFile
		artist  string
		album   string
		title   string
		trackNo int
	}

	results := make([]tagResult, len(changed))
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(8)
	var mu sync.Mutex
	idx := 0

	for _, cf := range changed {
		g.Go(func() error {
			if gctx.Err() != nil {
				return gctx.Err()
			}
			artist, album, title, trackNo := readTags(cf.absPath)
			mu.Lock()
			results[idx] = tagResult{cf: cf, artist: artist, album: album, title: title, trackNo: trackNo}
			idx++
			mu.Unlock()
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}

	for _, r := range results[:idx] {
		if err := db.UpsertFile(state.FileRecord{
			Path:        r.cf.relPath,
			Size:        r.cf.size,
			ModTime:     r.cf.modTime,
			Artist:      r.artist,
			Album:       r.album,
			Title:       r.title,
			TrackNumber: r.trackNo,
			ScannedAt:   time.Now(),
		}); err != nil {
			return err
		}
	}

	_, err = db.RemoveStaleFiles(livePaths)
	if err != nil {
		return fmt.Errorf("remove stale files: %w", err)
	}

	return nil
}

func readTags(path string) (artist, album, title string, trackNumber int) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".wma" {
		artist, album, title, trackNumber = readASF(path)
	} else {
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
		title = m.Title()
		trackNumber, _ = m.Track()
		album = m.Album()
	}

	// Fall back to filename parsing when tags are missing title/track number.
	// Supports patterns like "06 Somebody's Heaven.flac" or "06. Title.flac".
	if title == "" || trackNumber == 0 {
		fnTitle, fnTrack := parseFilename(filepath.Base(path))
		if title == "" {
			title = fnTitle
		}
		if trackNumber == 0 {
			trackNumber = fnTrack
		}
	}

	return artist, album, title, trackNumber
}

// parseFilename extracts track number and title from a filename like
// "06 Somebody's Heaven.flac" or "06. Title.flac".
func parseFilename(basename string) (title string, trackNumber int) {
	name := strings.TrimSuffix(basename, filepath.Ext(basename))
	if name == "" {
		return "", 0
	}

	// Try to split leading digits from the rest
	i := 0
	for i < len(name) && name[i] >= '0' && name[i] <= '9' {
		i++
	}
	if i == 0 {
		return name, 0
	}
	if i == len(name) {
		// Entire name is digits — treat as track number only
		num := 0
		for _, ch := range name[:i] {
			num = num*10 + int(ch-'0')
		}
		return "", num
	}

	num := 0
	for _, ch := range name[:i] {
		num = num*10 + int(ch-'0')
	}

	rest := name[i:]
	// Strip leading separators: space, dot, dash, underscore
	rest = strings.TrimLeft(rest, " .-_")
	if rest == "" {
		return "", num
	}

	return rest, num
}
