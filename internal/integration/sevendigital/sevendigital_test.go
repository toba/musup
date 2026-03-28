package sevendigital

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"os"
	"testing"
)

func TestDownloadRelease(t *testing.T) {
	email := os.Getenv("SD_USER")
	pass := os.Getenv("SD_PASS")
	orderID := os.Getenv("SD_ORDER")
	if email == "" || pass == "" || orderID == "" {
		t.Skip("set SD_USER, SD_PASS, SD_ORDER to run")
	}

	ctx := context.Background()
	client, err := Login(ctx, email, pass)
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	releases, err := client.FetchReleases(ctx, orderID)
	if err != nil {
		t.Fatalf("FetchReleases: %v", err)
	}
	if len(releases) == 0 {
		t.Fatal("no releases")
	}

	destDir := t.TempDir()
	t.Logf("Downloading all %d releases concurrently to %s", len(releases), destDir)
	err = client.DownloadAll(ctx, releases, destDir)
	if err != nil {
		t.Fatalf("DownloadAll: %v", err)
	}
	t.Logf("Done: %d/%d releases, %d bytes", client.ReleasesCompleted(), len(releases), client.BytesDownloaded())

	entries, _ := os.ReadDir(destDir)
	t.Logf("Extracted %d files:", len(entries))
	for _, e := range entries {
		info, _ := e.Info()
		t.Logf("  %s (%d bytes)", e.Name(), info.Size())
	}
	if len(entries) == 0 {
		t.Fatal("no files extracted")
	}
}

func TestDownloadFlow(t *testing.T) {
	email := os.Getenv("SD_USER")
	pass := os.Getenv("SD_PASS")
	orderID := os.Getenv("SD_ORDER")
	if email == "" || pass == "" || orderID == "" {
		t.Skip("set SD_USER, SD_PASS, SD_ORDER to run")
	}

	ctx := context.Background()

	t.Log("Logging in...")
	client, err := Login(ctx, email, pass)
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	t.Log("Login OK")

	t.Log("Fetching releases...")
	releases, err := client.FetchReleases(ctx, orderID)
	if err != nil {
		t.Fatalf("FetchReleases failed: %v", err)
	}
	t.Logf("Found %d releases:", len(releases))
	for i, r := range releases {
		t.Logf("  %d: %s - %s (release=%s format=%s %s)", i, r.Artist, r.Title, r.ReleaseID, r.FormatID, r.Format)
	}

	if len(releases) == 0 {
		t.Fatal("No releases found")
	}

	// Test downloading the first release - just check the redirect chain, don't download the full file.
	rel := releases[0]
	reqURL := fmt.Sprintf("https://us.7digital.com/download/release/%s?formatId=%s", rel.ReleaseID, rel.FormatID)
	t.Logf("Testing download URL: %s", reqURL)

	// Don't follow redirects - inspect the chain manually.
	noRedirectClient := *client.http
	noRedirectClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := noRedirectClient.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	dump, _ := httputil.DumpResponse(resp, false)
	t.Logf("Response (step 1):\n%s", dump)

	if resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusTemporaryRedirect {
		loc := resp.Header.Get("Location")
		t.Logf("Redirect to: %s", loc)

		// Follow as HTTP (OAuth signature is computed for http:// URL).
		req2, _ := http.NewRequestWithContext(ctx, http.MethodGet, loc, nil)
		resp2, err := noRedirectClient.Do(req2)
		if err != nil {
			t.Fatalf("Redirect request failed: %v", err)
		}
		defer func() { _ = resp2.Body.Close() }()

		dump2, _ := httputil.DumpResponse(resp2, false)
		t.Logf("Response (step 2, http):\n%s", dump2)

		// If we get another redirect, follow it too.
		if resp2.StatusCode == http.StatusFound || resp2.StatusCode == http.StatusTemporaryRedirect {
			loc2 := resp2.Header.Get("Location")
			t.Logf("Second redirect to: %s", loc2)
			req3, _ := http.NewRequestWithContext(ctx, http.MethodGet, loc2, nil)
			resp3, err := noRedirectClient.Do(req3)
			if err != nil {
				t.Fatalf("Second redirect request failed: %v", err)
			}
			defer func() { _ = resp3.Body.Close() }()
			dump3, _ := httputil.DumpResponse(resp3, false)
			t.Logf("Response (step 3):\n%s", dump3)
		}
	}
}
