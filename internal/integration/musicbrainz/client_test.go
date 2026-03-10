package musicbrainz_test

import (
	"context"
	"encoding/json"
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
