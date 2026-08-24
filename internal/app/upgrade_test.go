package app

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dolf/env-switcher/internal/upgrade"
)

// TestUpgradeDispatchRoutesBothSpellings proves "upgrade" and "--upgrade" both reach
// upgradeCommand rather than falling into the default "switch to a project named X" branch
// (config.Validate separately guards against a project actually being named "upgrade" — see
// internal/config's TestUpgradeIsAReservedProjectName). It never touches live GitHub: the
// context is canceled up front, so the HTTP client fails immediately with "context canceled"
// instead of making a request — enough to prove routing without any network dependency.
func TestUpgradeDispatchRoutesBothSpellings(t *testing.T) {
	for _, args := range [][]string{{"upgrade"}, {"--upgrade"}} {
		t.Run(strings.Join(args, ""), func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			a := New(BuildInfo{Version: "v1.0.0"})
			var stdout, stderr bytes.Buffer
			code := a.Run(ctx, args, nil, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("expected a non-zero exit with no network access; stdout=%s", stdout.String())
			}
			if !strings.Contains(stderr.String(), "upgrade:") {
				t.Fatalf("error does not look like it came from the upgrade command: %q", stderr.String())
			}
			if strings.Contains(stderr.String(), "is not configured") {
				t.Fatalf("upgrade fell through to the switch-to-project path: %q", stderr.String())
			}
		})
	}
}

type fakeReleaseSource struct{ release upgrade.Release }

func (f fakeReleaseSource) LatestStable(context.Context) (upgrade.Release, error) {
	return f.release, nil
}

// newFixtureUpgrader builds an *upgrade.Upgrader backed by an httptest server serving a single
// asset (zip-wrapping the given content) plus its matching SHA256SUMS — no live GitHub access —
// for testing upgradeCommand's own stdout formatting in isolation from the upgrade package's own
// (separately tested) network/verification logic.
func newFixtureUpgrader(t *testing.T, content string) *upgrade.Upgrader {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	hdr := &zip.FileHeader{Name: "env-switcher", Method: zip.Deflate}
	hdr.SetMode(0o755)
	fw, err := zw.CreateHeader(hdr)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(buf.Bytes())
	digest := hex.EncodeToString(sum[:])
	assetName := upgrade.AssetName(upgrade.CurrentPlatform())

	mux := http.NewServeMux()
	mux.HandleFunc("/asset.zip", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(buf.Bytes()) })
	mux.HandleFunc("/SHA256SUMS", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s  %s\n", digest, assetName)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	release := upgrade.Release{
		TagName: "v2.0.0",
		Assets: []upgrade.Asset{
			{Name: assetName, DownloadURL: srv.URL + "/asset.zip", Size: int64(buf.Len())},
			{Name: upgrade.ChecksumAssetName, DownloadURL: srv.URL + "/SHA256SUMS"},
		},
	}
	dir := t.TempDir()
	return &upgrade.Upgrader{
		Source:        fakeReleaseSource{release: release},
		Checksums:     &upgrade.GitHubChecksumSource{Client: srv.Client()},
		Client:        srv.Client(),
		Platform:      upgrade.CurrentPlatform(),
		InstalledPath: func() (string, error) { return filepath.Join(dir, "bin", "env-switcher"), nil },
	}
}

func TestUpgradeCommandReportsAlreadyCurrent(t *testing.T) {
	u := newFixtureUpgrader(t, "binary-content")
	var stdout, stderr bytes.Buffer
	if err := upgradeCommand(t.Context(), u, "v2.0.0", &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "already up to date") || !strings.Contains(stdout.String(), "v2.0.0") {
		t.Fatalf("unexpected output: %q", stdout.String())
	}
}

func TestUpgradeCommandReportsSuccess(t *testing.T) {
	u := newFixtureUpgrader(t, "binary-content")
	var stdout, stderr bytes.Buffer
	if err := upgradeCommand(t.Context(), u, "v1.0.0", &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	if !strings.Contains(out, "v1.0.0 -> v2.0.0") {
		t.Fatalf("output missing old/new version transition: %q", out)
	}
	if !strings.Contains(out, "installed at") {
		t.Fatalf("output missing installed path: %q", out)
	}
}
