package musicbrainz_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/toba/musup/internal/integration/musicbrainz"
)

func TestSearchArtists(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/artist" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("fmt"); got != "json" {
			t.Errorf("expected fmt=json, got %s", got)
		}
		if ua := r.Header.Get("User-Agent"); ua == "" {
			t.Error("missing User-Agent header")
		}

		json.NewEncoder(w).Encode(musicbrainz.ArtistSearchResult{
			Count:  1,
			Offset: 0,
			Artists: []musicbrainz.Artist{
				{ID: "abc-123", Name: "Test Artist", Score: 100},
			},
		})
	}))
	defer srv.Close()

	client := musicbrainz.NewWithBase(srv.URL, "test", "0.1", "test@example.com")
	result, err := client.SearchArtists(context.Background(), "Test Artist", 25, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Artists) != 1 {
		t.Fatalf("expected 1 artist, got %d", len(result.Artists))
	}
	if result.Artists[0].Name != "Test Artist" {
		t.Errorf("expected 'Test Artist', got %q", result.Artists[0].Name)
	}
}

func TestBrowseReleases(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/release" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("release-group"); got != "rg-456" {
			t.Errorf("expected release-group=rg-456, got %s", got)
		}
		if got := r.URL.Query().Get("inc"); got != "recordings" {
			t.Errorf("expected inc=recordings, got %s", got)
		}

		json.NewEncoder(w).Encode(musicbrainz.ReleaseBrowseResult{
			Count:  1,
			Offset: 0,
			Releases: []musicbrainz.Release{
				{
					ID:    "rel-789",
					Title: "Test Album",
					Date:  "2025-01-15",
					Media: []musicbrainz.Medium{
						{
							Position:   1,
							TrackCount: 2,
							Tracks: []musicbrainz.Track{
								{ID: "t1", Title: "Track One", Position: 1, Recording: musicbrainz.Recording{ID: "r1", Title: "Track One", Length: 240000}},
								{ID: "t2", Title: "Track Two", Position: 2, Recording: musicbrainz.Recording{ID: "r2", Title: "Track Two", Length: 180000}},
							},
						},
					},
				},
			},
		})
	}))
	defer srv.Close()

	client := musicbrainz.NewWithBase(srv.URL, "test", "0.1", "test@example.com")
	result, err := client.BrowseReleases(context.Background(), "rg-456", "recordings", 1, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Releases) != 1 {
		t.Fatalf("expected 1 release, got %d", len(result.Releases))
	}
	rel := result.Releases[0]
	if rel.Title != "Test Album" {
		t.Errorf("expected 'Test Album', got %q", rel.Title)
	}
	if len(rel.Media) != 1 {
		t.Fatalf("expected 1 medium, got %d", len(rel.Media))
	}
	if len(rel.Media[0].Tracks) != 2 {
		t.Fatalf("expected 2 tracks, got %d", len(rel.Media[0].Tracks))
	}
	if rel.Media[0].Tracks[0].Recording.Length != 240000 {
		t.Errorf("expected track 1 length 240000, got %d", rel.Media[0].Tracks[0].Recording.Length)
	}
}

func TestAllReleaseGroups_Pagination(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		offset := r.URL.Query().Get("offset")

		var rgs []musicbrainz.ReleaseGroup
		if offset == "0" {
			for i := range 100 {
				rgs = append(rgs, musicbrainz.ReleaseGroup{
					ID:    fmt.Sprintf("rg-%d", i),
					Title: fmt.Sprintf("Album %d", i),
				})
			}
		} else {
			for i := range 50 {
				rgs = append(rgs, musicbrainz.ReleaseGroup{
					ID:    fmt.Sprintf("rg-%d", 100+i),
					Title: fmt.Sprintf("Album %d", 100+i),
				})
			}
		}

		json.NewEncoder(w).Encode(musicbrainz.ReleaseGroupBrowseResult{
			Count:         150,
			Offset:        0,
			ReleaseGroups: rgs,
		})
	}))
	defer srv.Close()

	client := musicbrainz.NewWithBase(srv.URL, "test", "0.1", "test@example.com")
	all, err := client.AllReleaseGroups(context.Background(), "abc-123", "album")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(all) != 150 {
		t.Fatalf("expected 150 release groups, got %d", len(all))
	}
	if callCount != 2 {
		t.Fatalf("expected 2 API calls, got %d", callCount)
	}
}

func TestSearchArtists_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := musicbrainz.NewWithBase(srv.URL, "test", "0.1", "test@example.com")
	_, err := client.SearchArtists(context.Background(), "Test", 25, 0)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestBrowseReleaseGroups(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/release-group" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("artist"); got != "abc-123" {
			t.Errorf("expected artist=abc-123, got %s", got)
		}
		if got := r.URL.Query().Get("type"); got != "album" {
			t.Errorf("expected type=album, got %s", got)
		}

		json.NewEncoder(w).Encode(musicbrainz.ReleaseGroupBrowseResult{
			Count:  1,
			Offset: 0,
			ReleaseGroups: []musicbrainz.ReleaseGroup{
				{ID: "rg-456", Title: "Test Album", PrimaryType: "Album", FirstReleaseDate: "2025-01-15"},
			},
		})
	}))
	defer srv.Close()

	client := musicbrainz.NewWithBase(srv.URL, "test", "0.1", "test@example.com")
	result, err := client.BrowseReleaseGroups(context.Background(), "abc-123", "album", 25, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.ReleaseGroups) != 1 {
		t.Fatalf("expected 1 release group, got %d", len(result.ReleaseGroups))
	}
	if result.ReleaseGroups[0].Title != "Test Album" {
		t.Errorf("expected 'Test Album', got %q", result.ReleaseGroups[0].Title)
	}
}
