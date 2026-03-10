package musicbrainz

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

const defaultBaseURL = "https://musicbrainz.org/ws/2"

// Client is a rate-limited MusicBrainz WS2 JSON client.
type Client struct {
	base      string
	http      *http.Client
	userAgent string

	mu       sync.Mutex
	lastCall time.Time
}

// New creates a Client with the required User-Agent identification.
// Format per MB guidelines: "AppName/Version (contact-url-or-email)"
func New(appName, version, contact string) *Client {
	return NewWithBase(defaultBaseURL, appName, version, contact)
}

// NewWithBase creates a Client with a custom base URL (useful for testing).
func NewWithBase(base, appName, version, contact string) *Client {
	return &Client{
		base:      base,
		http:      &http.Client{Timeout: 15 * time.Second},
		userAgent: fmt.Sprintf("%s/%s ( %s )", appName, version, contact),
	}
}

// SearchArtists searches for artists by name.
func (c *Client) SearchArtists(ctx context.Context, name string, limit, offset int) (*ArtistSearchResult, error) {
	params := url.Values{
		"query":  {fmt.Sprintf(`artist:"%s"`, name)},
		"fmt":    {"json"},
		"limit":  {strconv.Itoa(limit)},
		"offset": {strconv.Itoa(offset)},
	}

	var result ArtistSearchResult
	if err := c.get(ctx, "/artist", params, &result); err != nil {
		return nil, fmt.Errorf("search artists: %w", err)
	}
	return &result, nil
}

// BrowseReleaseGroups returns release groups for an artist MBID.
// Use typeFilter to restrict results (e.g. "album", "ep", "single") or "" for all.
func (c *Client) BrowseReleaseGroups(ctx context.Context, artistMBID string, typeFilter string, limit, offset int) (*ReleaseGroupBrowseResult, error) {
	params := url.Values{
		"artist": {artistMBID},
		"fmt":    {"json"},
		"limit":  {strconv.Itoa(limit)},
		"offset": {strconv.Itoa(offset)},
	}
	if typeFilter != "" {
		params.Set("type", typeFilter)
	}

	var result ReleaseGroupBrowseResult
	if err := c.get(ctx, "/release-group", params, &result); err != nil {
		return nil, fmt.Errorf("browse release groups: %w", err)
	}
	return &result, nil
}

// AllReleaseGroups pages through all release groups for an artist.
func (c *Client) AllReleaseGroups(ctx context.Context, artistMBID string, typeFilter string) ([]ReleaseGroup, error) {
	const pageSize = 100
	var all []ReleaseGroup

	for offset := 0; ; offset += pageSize {
		page, err := c.BrowseReleaseGroups(ctx, artistMBID, typeFilter, pageSize, offset)
		if err != nil {
			return nil, err
		}
		all = append(all, page.ReleaseGroups...)
		if len(all) >= page.Count {
			break
		}
	}
	return all, nil
}

func (c *Client) get(ctx context.Context, path string, params url.Values, dest any) error {
	c.rateLimit()

	reqURL := c.base + path + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("musicbrainz: %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(dest)
}

// rateLimit enforces the 1 request/second limit.
func (c *Client) rateLimit() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elapsed := time.Since(c.lastCall); elapsed < time.Second {
		time.Sleep(time.Second - elapsed)
	}
	c.lastCall = time.Now()
}
