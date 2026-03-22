package check

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/toba/musup/internal/db"
	"github.com/toba/musup/internal/integration/musicbrainz"
)

const (
	mbSearchLimit              = 5
	mbMinMatchScore            = 90
	mbMaxReleaseGroups         = 500
	mbMaxReleaseGroupsComposer = 100
)

// Progress is sent via the callback to report sync progress.
type Progress struct {
	Current int
	Total   int
	Artist  string
}

// SyncAll checks MusicBrainz for all artists in the local library that
// haven't been checked recently. staleAfter controls how old a check can
// be before re-syncing (0 means never re-check).
func SyncAll(ctx context.Context, d *db.DB, mb *musicbrainz.Client, staleAfter time.Duration, onProgress func(Progress)) error {
	artistIDs, err := d.Q.DistinctArtistIDs(ctx)
	if err != nil {
		return fmt.Errorf("list artists: %w", err)
	}

	// Build a list of artists that need checking.
	type artistWork struct {
		id   int64
		name string
	}
	var work []artistWork

	for _, id := range artistIDs {
		row, err := d.Q.GetArtistByID(ctx, id)
		if err != nil {
			continue
		}
		if row.NotFound != 0 {
			continue
		}
		if row.Mbid != "" && staleAfter > 0 && row.LastCheckedAt != "" {
			checked, parseErr := time.Parse(time.RFC3339, row.LastCheckedAt)
			if parseErr == nil && time.Since(checked) < staleAfter {
				continue
			}
		}
		work = append(work, artistWork{id: row.ID, name: row.Name})
	}

	for i, w := range work {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if onProgress != nil {
			onProgress(Progress{Current: i + 1, Total: len(work), Artist: w.name})
		}

		if err := syncArtist(ctx, d, mb, w.id); err != nil {
			if errors.Is(err, context.Canceled) {
				return err
			}
			continue
		}
	}

	return nil
}

func syncArtist(ctx context.Context, d *db.DB, mb *musicbrainz.Client, artistID int64) error {
	row, err := d.Q.GetArtistByID(ctx, artistID)
	if err != nil {
		return fmt.Errorf("get artist: %w", err)
	}

	// If we already have an MBID, skip the search step.
	mbid := row.Mbid
	if mbid == "" {
		result, err := mb.SearchArtists(ctx, row.Name, mbSearchLimit, 0)
		if err != nil {
			return fmt.Errorf("search artist: %w", err)
		}
		if len(result.Artists) == 0 || result.Artists[0].Score < mbMinMatchScore {
			return d.Q.MarkArtistNotFound(ctx, artistID)
		}
		mbid = result.Artists[0].ID

		rgCap := mbMaxReleaseGroups
		if hasComposerTag(result.Artists[0]) {
			rgCap = mbMaxReleaseGroupsComposer
		}

		if err := fetchAndStoreAlbums(ctx, d, mb, artistID, mbid, rgCap); err != nil {
			return err
		}
	} else {
		rgCap := mbMaxReleaseGroups
		if err := fetchAndStoreAlbums(ctx, d, mb, artistID, mbid, rgCap); err != nil {
			return err
		}
	}

	return d.Q.UpdateArtistMeta(ctx, mbid, time.Now().Format(time.RFC3339), artistID)
}

func fetchAndStoreAlbums(ctx context.Context, d *db.DB, mb *musicbrainz.Client, artistID int64, mbid string, rgCap int) error {
	rgResult, err := mb.AllReleaseGroups(ctx, mbid, "album", rgCap)
	if err != nil {
		return fmt.Errorf("fetch release groups: %w", err)
	}

	for _, rg := range rgResult.ReleaseGroups {
		if err := d.Q.UpsertAlbum(ctx, db.UpsertAlbumParams{
			ArtistID:       artistID,
			Title:          rg.Title,
			TitleNorm:      db.Normalize(rg.Title),
			Mbid:           rg.ID,
			ReleaseDate:    rg.FirstReleaseDate,
			PrimaryType:    rg.PrimaryType,
			SecondaryTypes: strings.Join(rg.SecondaryTypes, ","),
		}); err != nil {
			return fmt.Errorf("upsert album %q: %w", rg.Title, err)
		}
	}

	return nil
}

func hasComposerTag(artist musicbrainz.Artist) bool {
	for _, tag := range artist.Tags {
		if tag.Name == "composer" {
			return true
		}
	}
	return false
}
