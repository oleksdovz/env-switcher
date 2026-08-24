package upgrade

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// userAgent identifies this client to GitHub's API and asset CDN; GitHub requires a descriptive
// User-Agent on API requests and will otherwise reject them.
const userAgent = "env-switcher-upgrade/1 (+https://github.com/oleksdovz/env-switcher)"

// requestTimeout bounds a single HTTP round trip (connect through body-read-start); the overall
// operation is additionally bounded by the caller's context.
const requestTimeout = 30 * time.Second

// maxRedirects caps how many hops a request may follow before giving up, independent of the
// host allowlist below.
const maxRedirects = 5

// maxResponseBytes bounds how much of any single response this client will read — the API JSON
// bodies are small, and release assets are checked against their own declared/expected size by
// the caller, but neither should be allowed to stream unboundedly.
const maxResponseBytes = 64 << 20 // 64MiB: comfortably above today's largest published asset

// trustedRedirectHosts are the only hosts a redirect may land on. GitHub serves release assets
// via a signed redirect from github.com to its blob storage front door, which has used a few
// different hostnames over time — all under githubusercontent.com — so that's matched by suffix;
// everything else (github.com, api.github.com) is matched exactly.
var trustedRedirectHosts = []string{"github.com", "api.github.com"}

func isTrustedHost(host string) bool {
	host = strings.ToLower(host)
	for _, h := range trustedRedirectHosts {
		if host == h {
			return true
		}
	}
	return strings.HasSuffix(host, ".githubusercontent.com")
}

// newHTTPClient builds the HTTP client used for both GitHub API calls and release asset/checksum
// downloads: HTTPS-only redirects limited to trusted GitHub hosts, and a hard cap on redirect
// hops.
func newHTTPClient() *http.Client {
	return &http.Client{
		Timeout: requestTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("too many redirects (>%d)", maxRedirects)
			}
			if req.URL.Scheme != "https" {
				return fmt.Errorf("refusing to follow redirect to non-HTTPS URL %q", req.URL.Redacted())
			}
			if !isTrustedHost(req.URL.Hostname()) {
				return fmt.Errorf("refusing to follow redirect to untrusted host %q", req.URL.Hostname())
			}
			return nil
		},
	}
}

func newRequest(ctx context.Context, method, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	return req, nil
}

// readLimited reads at most maxResponseBytes+1 from r, treating a full extra byte as "response
// too large" so truncated data is never silently accepted as complete.
func readLimited(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("response exceeded %d byte limit", limit)
	}
	return data, nil
}

// fetchBytes downloads url and returns its body, bounded by limit bytes.
func fetchBytes(ctx context.Context, client *http.Client, url string, limit int64) ([]byte, error) {
	req, err := newRequest(ctx, http.MethodGet, url)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := readLimited(resp.Body, 4096)
		return nil, fmt.Errorf("download %s: unexpected status %s: %s", url, resp.Status, strings.TrimSpace(string(body)))
	}
	return readLimited(resp.Body, limit)
}

// downloadToFile streams url's body into dest (created if absent, truncated if present — the
// caller is expected to have reserved a unique path, e.g. via os.CreateTemp), bounded by limit
// bytes, and returns the number of bytes written. dest is left in place (for the caller to
// remove) on any error.
func downloadToFile(ctx context.Context, client *http.Client, url, dest string, limit int64) (int64, error) {
	req, err := newRequest(ctx, http.MethodGet, url)
	if err != nil {
		return 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := readLimited(resp.Body, 4096)
		return 0, fmt.Errorf("download %s: unexpected status %s: %s", url, resp.Status, strings.TrimSpace(string(body)))
	}
	f, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	n, err := io.Copy(f, io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return 0, fmt.Errorf("download %s: %w", url, err)
	}
	if n > limit {
		return 0, fmt.Errorf("download %s exceeded %d byte limit", url, limit)
	}
	return n, nil
}

// githubAPIError formats a GitHub API error response, calling out rate limiting specifically
// since that's the actionable case ("try again later" / "authenticate") rather than a generic
// HTTP failure.
func githubAPIError(resp *http.Response, body []byte) error {
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		if remaining := resp.Header.Get("X-RateLimit-Remaining"); remaining == "0" {
			reset := resp.Header.Get("X-RateLimit-Reset")
			return fmt.Errorf("GitHub API rate limit exceeded (resets at unix time %s); try again later", reset)
		}
	}
	msg := strings.TrimSpace(string(body))
	if len(msg) > 300 {
		msg = msg[:300] + "…"
	}
	return fmt.Errorf("GitHub API request failed: %s: %s", resp.Status, msg)
}
