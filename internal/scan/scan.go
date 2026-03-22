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
	"github.com/toba/musup/internal/db"
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

// Scan walks root for music files, reads metadata, and updates d.
func Scan(ctx context.Context, d *db.DB, root string) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve root: %w", err)
	}

	// Load all known file metadata up front for in-memory change detection
	// instead of issuing one SQLite query per file during the walk.
	metaRows, err := d.Q.AllFileMeta(context.Background())
	if err != nil {
		return fmt.Errorf("load file metadata: %w", err)
	}
	knownFiles := make(map[string]db.AllFileMetaRow, len(metaRows))
	for _, row := range metaRows {
		knownFiles[row.Path] = row
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

		// In-memory change detection: check against pre-loaded map.
		if fm, ok := knownFiles[relPath]; ok {
			if fm.Title != "" && fm.Size == info.Size() && fm.ModTime == info.ModTime().Format(time.RFC3339) {
				return nil
			}
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
		cf            changedFile
		artist        string
		album         string
		title         string
		trackNo       int
		isAlbumArtist bool
	}

	results := make([]tagResult, 0, len(changed))
	g, gctx := errgroup.WithContext(ctx)
	const tagReadParallelism = 8
	g.SetLimit(tagReadParallelism)
	var mu sync.Mutex

	for _, cf := range changed {
		g.Go(func() error {
			if gctx.Err() != nil {
				return gctx.Err()
			}
			artist, album, title, trackNo, isAlbumArtist := readTags(cf.absPath)
			mu.Lock()
			results = append(results, tagResult{cf: cf, artist: artist, album: album, title: title, trackNo: trackNo, isAlbumArtist: isAlbumArtist})
			mu.Unlock()
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}

	for _, r := range results {
		var artistID int64
		if r.artist != "" {
			id, err := d.EnsureArtist(r.artist)
			if err != nil {
				return fmt.Errorf("ensure artist %q: %w", r.artist, err)
			}
			artistID = id
		}
		if err := d.Q.UpsertFile(context.Background(), db.UpsertFileParams{
			Path:          r.cf.relPath,
			Size:          r.cf.size,
			ModTime:       r.cf.modTime.Format(time.RFC3339),
			Artist:        r.artist,
			Album:         r.album,
			Title:         r.title,
			TitleNorm:     db.Normalize(r.title),
			AlbumNorm:     db.Normalize(r.album),
			ArtistNorm:    db.Normalize(r.artist),
			ArtistID:      artistID,
			TrackNumber:   int64(r.trackNo),
			IsAlbumArtist: int64(db.BoolToInt(r.isAlbumArtist)),
			ScannedAt:     time.Now().Format(time.RFC3339),
		}); err != nil {
			return err
		}
	}

	_, err = d.RemoveStaleFiles(livePaths)
	if err != nil {
		return fmt.Errorf("remove stale files: %w", err)
	}

	return nil
}

func readTags(path string) (artist, album, title string, trackNumber int, isAlbumArtist bool) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".wma" {
		artist, album, title, trackNumber, isAlbumArtist = readASF(path)
	} else {
		f, err := os.Open(path)
		if err != nil {
			return "", "", "", 0, false
		}
		defer func() { _ = f.Close() }()

		m, err := tag.ReadFrom(f)
		if err != nil {
			return "", "", "", 0, false
		}

		artist = m.AlbumArtist()
		if artist != "" {
			isAlbumArtist = true
		} else {
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

	return artist, album, title, trackNumber, isAlbumArtist
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
