package upgrade

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

// ErrNoStableRelease is returned when a repository has no release that is neither a draft nor a
// prerelease, or none whose tag parses as a semantic version.
var ErrNoStableRelease = errors.New("no stable release found")

// Asset is one downloadable file attached to a release.
type Asset struct {
	Name        string
	DownloadURL string
	Size        int64
}

// Release is the subset of GitHub release metadata this package needs.
type Release struct {
	TagName    string
	Name       string
	Draft      bool
	Prerelease bool
	Assets     []Asset
}

// Version parses the release's tag as a semantic version.
func (r Release) Version() (Version, error) { return ParseVersion(r.TagName) }

// Asset looks up one of the release's assets by exact name.
func (r Release) Asset(name string) (Asset, bool) {
	for _, a := range r.Assets {
		if a.Name == name {
			return a, true
		}
	}
	return Asset{}, false
}

// ReleaseSource retrieves release metadata. It exists so tests (and any future release host)
// never depend on live GitHub access — see githubSourceTest and the httptest-backed fixtures in
// upgrade_test.go.
type ReleaseSource interface {
	// LatestStable returns the highest-precedence release that is neither a draft nor a
	// prerelease and whose tag parses as a semantic version. It returns ErrNoStableRelease if
	// no such release exists.
	LatestStable(ctx context.Context) (Release, error)
}

// GitHubSource retrieves release metadata from the GitHub REST API.
type GitHubSource struct {
	Owner, Repo string
	// BaseURL defaults to https://api.github.com; overridden in tests to point at a local
	// httptest.Server fixture instead of live GitHub.
	BaseURL string
	Client  *http.Client
}

// NewGitHubSource returns a GitHubSource for owner/repo talking to the real GitHub API.
func NewGitHubSource(owner, repo string) *GitHubSource {
	return &GitHubSource{Owner: owner, Repo: repo, BaseURL: "https://api.github.com", Client: newHTTPClient()}
}

type githubRelease struct {
	TagName    string        `json:"tag_name"`
	Name       string        `json:"name"`
	Draft      bool          `json:"draft"`
	Prerelease bool          `json:"prerelease"`
	Assets     []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// LatestStable implements ReleaseSource. Draft and prerelease releases are ignored by
// construction here, not left to the caller, and the "latest" one is the highest by semantic
// version, not simply the most recently published — a late patch release for an older line
// should never be mistaken for a hotfix on top of a newer one.
func (s *GitHubSource) LatestStable(ctx context.Context) (Release, error) {
	client := s.Client
	if client == nil {
		client = newHTTPClient()
	}
	url := fmt.Sprintf("%s/repos/%s/%s/releases?per_page=30", s.BaseURL, s.Owner, s.Repo)
	req, err := newRequest(ctx, http.MethodGet, url)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := client.Do(req)
	if err != nil {
		return Release{}, fmt.Errorf("fetch releases: %w", err)
	}
	defer resp.Body.Close()
	body, err := readLimited(resp.Body, maxResponseBytes)
	if err != nil {
		return Release{}, fmt.Errorf("read releases response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return Release{}, githubAPIError(resp, body)
	}

	var raw []githubRelease
	if err := json.Unmarshal(body, &raw); err != nil {
		return Release{}, fmt.Errorf("parse releases response: %w", err)
	}

	var best Release
	var bestVersion Version
	found := false
	for _, r := range raw {
		if r.Draft || r.Prerelease {
			continue
		}
		v, err := ParseVersion(r.TagName)
		if err != nil {
			continue // non-semver tag (e.g. a manual "latest" alias); skip rather than fail
		}
		if found && !v.NewerThan(bestVersion) {
			continue
		}
		best = releaseFrom(r)
		bestVersion = v
		found = true
	}
	if !found {
		return Release{}, ErrNoStableRelease
	}
	return best, nil
}

func releaseFrom(r githubRelease) Release {
	assets := make([]Asset, len(r.Assets))
	for i, a := range r.Assets {
		assets[i] = Asset{Name: a.Name, DownloadURL: a.BrowserDownloadURL, Size: a.Size}
	}
	return Release{TagName: r.TagName, Name: r.Name, Draft: r.Draft, Prerelease: r.Prerelease, Assets: assets}
}
