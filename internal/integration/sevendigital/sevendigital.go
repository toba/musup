// Package sevendigital downloads purchased music from 7digital.com.
package sevendigital

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
)

// Release represents a downloadable release on a 7digital download page.
type Release struct {
	ReleaseID string
	FormatID  string
	Title     string
	Artist    string
	Format    string
}

// Client handles authenticated interactions with 7digital.com.
type Client struct {
	http  *http.Client
	bytes atomic.Int64 // total bytes downloaded across all concurrent releases
	done  atomic.Int32 // number of completed releases
}

// BytesDownloaded returns the total bytes downloaded across all concurrent releases.
func (c *Client) BytesDownloaded() int64 {
	return c.bytes.Load()
}

// ReleasesCompleted returns the number of releases that have finished downloading.
func (c *Client) ReleasesCompleted() int {
	return int(c.done.Load())
}

// Login authenticates with 7digital and returns a Client with session cookies.
func Login(ctx context.Context, email, password string) (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("creating cookie jar: %w", err)
	}
	c := &http.Client{
		Jar: jar,
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("too many redirects")
			}
			return nil
		},
	}

	form := url.Values{
		"referrer": {""},
		"email":    {email},
		"password": {password},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://us.7digital.com/signin", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("login request: %w", err)
	}
	_ = resp.Body.Close()

	// Verify we got session cookies.
	u, _ := url.Parse("https://us.7digital.com/")
	var hasSession bool
	for _, cookie := range jar.Cookies(u) {
		if cookie.Name == "session" || cookie.Name == "secureSession" {
			hasSession = true
		}
	}
	if !hasSession {
		return nil, errors.New("login failed: no session cookie received (check credentials)")
	}

	return &Client{http: c}, nil
}

var (
	formActionRe  = regexp.MustCompile(`<form[^>]*id="yourmusic-release-format-selector-(\d+)"[^>]*>`)
	formatInputRe = regexp.MustCompile(`<input[^>]*name="formatId"[^>]*value="(\d+)"[^>]*>`)
)

// releaseBlockRe matches each release form block.
var releaseBlockRe = regexp.MustCompile(`(?s)<form[^>]*id="yourmusic-release-format-selector-\d+"[^>]*>.*?</form>`)

// FetchReleases fetches the download page and returns the available releases.
func (c *Client) FetchReleases(ctx context.Context, orderID string) ([]Release, error) {
	reqURL := "https://us.7digital.com/download/" + orderID
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil) //nolint:gosec // orderID is user-provided download ID
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req) //nolint:gosec // same as above
	if err != nil {
		return nil, fmt.Errorf("fetching download page: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download page returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	html := string(body)

	blocks := releaseBlockRe.FindAllString(html, -1)
	var releases []Release
	for _, block := range blocks {
		m := formActionRe.FindStringSubmatch(block)
		if m == nil {
			continue
		}
		releaseID := m[1]

		fm := formatInputRe.FindStringSubmatch(block)
		if fm == nil {
			continue
		}
		formatID := fm[1]

		// Extract format label.
		formatLabel := ""
		if fl := regexp.MustCompile(`<span[^>]*class="release-format-label"[^>]*>([^<]+)</span>`).FindStringSubmatch(block); fl != nil {
			formatLabel = strings.TrimSpace(fl[1])
		}

		releases = append(releases, Release{
			ReleaseID: releaseID,
			FormatID:  formatID,
			Format:    formatLabel,
		})
	}

	// Extract artist and title from the surrounding HTML (outside forms).
	titleLinkRe := regexp.MustCompile(`<a[^>]*href="/yourmusic/artist/[^"]*"[^>]*>([^<]+)</a>`)
	artistLinkRe := regexp.MustCompile(`<a[^>]*href="/artist/[^"]*"[^>]*>([^<]+)</a>`)

	for i := range releases {
		formTag := fmt.Sprintf(`id="yourmusic-release-format-selector-%s"`, releases[i].ReleaseID)
		idx := strings.Index(html, formTag)
		if idx < 0 {
			continue
		}
		start := max(idx-500, 0)
		section := html[start:idx]

		if tm := titleLinkRe.FindStringSubmatch(section); tm != nil {
			releases[i].Title = strings.TrimSpace(tm[1])
		}
		if am := artistLinkRe.FindStringSubmatch(section); am != nil {
			releases[i].Artist = strings.TrimSpace(am[1])
		}
	}

	return releases, nil
}

// DownloadRelease downloads a release zip and extracts it to destDir.
// It calls onProgress with status messages.
func (c *Client) DownloadRelease(ctx context.Context, rel Release, destDir string, onProgress func(string)) error {
	reqURL := fmt.Sprintf("https://us.7digital.com/download/release/%s?formatId=%s", rel.ReleaseID, rel.FormatID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("download request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %d", resp.StatusCode)
	}

	// Get filename from Content-Disposition or synthesize one.
	filename := fmt.Sprintf("%s - %s.zip", rel.Artist, rel.Title)
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		if _, params, parseErr := mime.ParseMediaType(cd); parseErr == nil {
			if fn, ok := params["filename"]; ok {
				filename = fn
			}
		}
	}

	onProgress("Downloading " + filename)

	// Write to temp file, tracking bytes.
	c.bytes.Store(0)
	tmpFile, err := os.CreateTemp("", "musup-7d-*.zip")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	cw := &countWriter{w: tmpFile, n: &c.bytes}
	if _, err := io.Copy(cw, resp.Body); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("downloading zip: %w", err)
	}
	_ = tmpFile.Close()

	// Extract zip into artist subdirectory.
	if rel.Artist != "" {
		destDir = filepath.Join(destDir, rel.Artist)
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			return fmt.Errorf("creating artist dir: %w", err)
		}
	}
	onProgress("Extracting " + filename)
	if err := extractZip(tmpPath, destDir); err != nil {
		return err
	}
	c.done.Add(1)
	return nil
}

// DownloadAll downloads all releases concurrently and extracts to destDir.
// Returns the first error encountered, cancelling remaining downloads.
func (c *Client) DownloadAll(ctx context.Context, releases []Release, destDir string) error {
	c.bytes.Store(0)
	c.done.Store(0)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	errs := make(chan error, len(releases))

	for _, rel := range releases {
		wg.Add(1)
		go func(r Release) {
			defer wg.Done()
			if err := c.DownloadRelease(ctx, r, destDir, func(string) {}); err != nil {
				errs <- err
				cancel()
			}
		}(rel)
	}

	wg.Wait()
	close(errs)
	return <-errs // nil if channel is empty
}

type countWriter struct {
	w io.Writer
	n *atomic.Int64
}

func (cw *countWriter) Write(p []byte) (int, error) {
	n, err := cw.w.Write(p)
	cw.n.Add(int64(n))
	return n, err
}

func extractZip(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("opening zip: %w", err)
	}
	defer func() { _ = r.Close() }()

	for _, f := range r.File {
		// Skip directories and __MACOSX entries.
		if f.FileInfo().IsDir() || strings.HasPrefix(f.Name, "__MACOSX") {
			continue
		}
		destPath := filepath.Join(destDir, filepath.Base(f.Name))
		if err := extractZipFile(f, destPath); err != nil {
			return err
		}
	}
	return nil
}

func extractZipFile(f *zip.File, destPath string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer func() { _ = rc.Close() }()

	out, err := os.Create(destPath) //nolint:gosec // trusted zip from 7digital
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	_, err = io.Copy(out, io.LimitReader(rc, 1<<30)) //nolint:gosec // limit to 1GB per file
	return err
}
