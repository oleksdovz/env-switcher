package upgrade

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func releasesServer(t *testing.T, releases []githubRelease) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/releases") {
			http.NotFound(w, r)
			return
		}
		if got := r.Header.Get("User-Agent"); got == "" {
			t.Errorf("request missing User-Agent")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(releases)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestLatestStableIgnoresDraftsAndPrereleasesAndPicksHighestSemver(t *testing.T) {
	srv := releasesServer(t, []githubRelease{
		{TagName: "v1.4.0", Draft: true},
		{TagName: "v1.3.0-rc.1", Prerelease: true},
		{TagName: "v1.2.0", Assets: []githubAsset{{Name: "env-switcher_linux_amd64.zip"}}},
		{TagName: "v1.1.0"},
		{TagName: "latest"}, // non-semver tag: must be skipped, not fatal
	})
	src := &GitHubSource{Owner: "o", Repo: "r", BaseURL: srv.URL, Client: srv.Client()}
	release, err := src.LatestStable(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if release.TagName != "v1.2.0" {
		t.Fatalf("got %q, want v1.2.0 (highest non-draft, non-prerelease, semver-parseable)", release.TagName)
	}
	if len(release.Assets) != 1 || release.Assets[0].Name != "env-switcher_linux_amd64.zip" {
		t.Fatalf("assets not carried through: %+v", release.Assets)
	}
}

func TestLatestStableNoneQualifies(t *testing.T) {
	srv := releasesServer(t, []githubRelease{
		{TagName: "v2.0.0", Draft: true},
		{TagName: "v2.0.0-beta", Prerelease: true},
	})
	src := &GitHubSource{Owner: "o", Repo: "r", BaseURL: srv.URL, Client: srv.Client()}
	_, err := src.LatestStable(t.Context())
	if err != ErrNoStableRelease {
		t.Fatalf("got %v, want ErrNoStableRelease", err)
	}
}

func TestLatestStableSurfacesRateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", "1700000000")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"API rate limit exceeded"}`))
	}))
	defer srv.Close()
	src := &GitHubSource{Owner: "o", Repo: "r", BaseURL: srv.URL, Client: srv.Client()}
	_, err := src.LatestStable(t.Context())
	if err == nil || !strings.Contains(err.Error(), "rate limit") {
		t.Fatalf("got %v, want a rate-limit error", err)
	}
}

func TestLatestStableSurfacesGenericAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()
	src := &GitHubSource{Owner: "o", Repo: "r", BaseURL: srv.URL, Client: srv.Client()}
	_, err := src.LatestStable(t.Context())
	if err == nil {
		t.Fatal("expected an error")
	}
}
