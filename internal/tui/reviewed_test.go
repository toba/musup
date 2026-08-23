package tui

import (
	"testing"
	"time"

	"github.com/toba/musup-go/internal/db"
)

func TestReviewedDateCoversNewestRelease(t *testing.T) {
	// Simulate: today is 2026-03-26, but an artist has a release dated 2026-03-27.
	// When the user presses comma, the reviewed date should cover that future release.
	today := "2026-03-26"
	releases := []db.FollowedNewerReleasesRow{
		{ReleaseDate: "2023-11-10", AlbumTitle: "The Maybe Man"},
		{ReleaseDate: "2026-03-27", AlbumTitle: "Live from the Hollywood Bowl"},
	}

	reviewedAt := reviewedDate(today, releases)

	// The reviewed date must be >= the newest release date so all releases
	// fall into the "reviewed" bucket (releaseDate <= reviewedAt).
	for _, r := range releases {
		if r.ReleaseDate > reviewedAt {
			t.Errorf("release %q (%s) not covered by reviewedAt %q", r.AlbumTitle, r.ReleaseDate, reviewedAt)
		}
	}
}

func TestReviewedDateNoReleases(t *testing.T) {
	today := time.Now().Format(dateFormat)
	reviewedAt := reviewedDate(today, nil)
	if reviewedAt != today {
		t.Errorf("expected %q with no releases, got %q", today, reviewedAt)
	}
}

func TestReviewedDateAllInPast(t *testing.T) {
	today := "2026-03-26"
	releases := []db.FollowedNewerReleasesRow{
		{ReleaseDate: "2023-11-10", AlbumTitle: "Past Album"},
	}
	reviewedAt := reviewedDate(today, releases)
	if reviewedAt != today {
		t.Errorf("expected %q when all releases are in the past, got %q", today, reviewedAt)
	}
}
