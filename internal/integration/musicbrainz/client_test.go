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
	result, err := client.AllReleaseGroups(context.Background(), "abc-123", "album", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.ReleaseGroups) != 150 {
		t.Fatalf("expected 150 release groups, got %d", len(result.ReleaseGroups))
	}
	if result.Capped {
		t.Fatal("expected Capped=false")
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

func TestBrowseReleaseGroups_NoTypeFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := r.URL.Query()["type"]; ok {
			t.Errorf("unexpected type param in URL: %s", r.URL.RawQuery)
		}
		if got := r.URL.Query().Get("release-group-status"); got != "website-default" {
			t.Errorf("expected release-group-status=website-default, got %s", got)
		}

		json.NewEncoder(w).Encode(musicbrainz.ReleaseGroupBrowseResult{
			Count:         0,
			Offset:        0,
			ReleaseGroups: []musicbrainz.ReleaseGroup{},
		})
	}))
	defer srv.Close()

	client := musicbrainz.NewWithBase(srv.URL, "test", "0.1", "test@example.com")
	_, err := client.BrowseReleaseGroups(context.Background(), "abc-123", "", 25, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBrowseReleaseGroups_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := musicbrainz.NewWithBase(srv.URL, "test", "0.1", "test@example.com")
	_, err := client.BrowseReleaseGroups(context.Background(), "abc-123", "album", 25, 0)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestAllReleaseGroups_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(musicbrainz.ReleaseGroupBrowseResult{
			Count:         0,
			Offset:        0,
			ReleaseGroups: []musicbrainz.ReleaseGroup{},
		})
	}))
	defer srv.Close()

	client := musicbrainz.NewWithBase(srv.URL, "test", "0.1", "test@example.com")
	result, err := client.AllReleaseGroups(context.Background(), "abc-123", "album", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.ReleaseGroups) != 0 {
		t.Fatalf("expected 0 release groups, got %d", len(result.ReleaseGroups))
	}
}

func TestAllReleaseGroups_SinglePage(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		var rgs []musicbrainz.ReleaseGroup
		for i := range 5 {
			rgs = append(rgs, musicbrainz.ReleaseGroup{
				ID:    fmt.Sprintf("rg-%d", i),
				Title: fmt.Sprintf("Album %d", i),
			})
		}
		json.NewEncoder(w).Encode(musicbrainz.ReleaseGroupBrowseResult{
			Count:         5,
			Offset:        0,
			ReleaseGroups: rgs,
		})
	}))
	defer srv.Close()

	client := musicbrainz.NewWithBase(srv.URL, "test", "0.1", "test@example.com")
	result, err := client.AllReleaseGroups(context.Background(), "abc-123", "album", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.ReleaseGroups) != 5 {
		t.Fatalf("expected 5 release groups, got %d", len(result.ReleaseGroups))
	}
	if callCount != 1 {
		t.Fatalf("expected 1 API call, got %d", callCount)
	}
}

func TestAllReleaseGroups_Capped(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		var rgs []musicbrainz.ReleaseGroup
		for i := range 100 {
			rgs = append(rgs, musicbrainz.ReleaseGroup{
				ID:    fmt.Sprintf("rg-%d", i),
				Title: fmt.Sprintf("Album %d", i),
			})
		}
		json.NewEncoder(w).Encode(musicbrainz.ReleaseGroupBrowseResult{
			Count:         5000,
			Offset:        0,
			ReleaseGroups: rgs,
		})
	}))
	defer srv.Close()

	client := musicbrainz.NewWithBase(srv.URL, "test", "0.1", "test@example.com")
	result, err := client.AllReleaseGroups(context.Background(), "abc-123", "album", 500)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Capped {
		t.Fatal("expected Capped=true")
	}
	if result.TotalCount != 5000 {
		t.Fatalf("expected TotalCount=5000, got %d", result.TotalCount)
	}
	if len(result.ReleaseGroups) != 100 {
		t.Fatalf("expected 100 release groups, got %d", len(result.ReleaseGroups))
	}
	if callCount != 1 {
		t.Fatalf("expected 1 API call, got %d", callCount)
	}
}

func TestAllReleaseGroups_BelowCap(t *testing.T) {
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
	result, err := client.AllReleaseGroups(context.Background(), "abc-123", "album", 500)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Capped {
		t.Fatal("expected Capped=false")
	}
	if len(result.ReleaseGroups) != 150 {
		t.Fatalf("expected 150 release groups, got %d", len(result.ReleaseGroups))
	}
	if callCount != 2 {
		t.Fatalf("expected 2 API calls, got %d", callCount)
	}
}

func TestSearchArtists_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{not valid json`))
	}))
	defer srv.Close()

	client := musicbrainz.NewWithBase(srv.URL, "test", "0.1", "test@example.com")
	_, err := client.SearchArtists(context.Background(), "Test", 25, 0)
	if err == nil {
		t.Fatal("expected error for malformed JSON response")
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
		if got := r.URL.Query().Get("release-group-status"); got != "website-default" {
			t.Errorf("expected release-group-status=website-default, got %s", got)
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
