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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dolf/env-switcher/internal/testutil"
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
	if err := upgradeCommand(t.Context(), u, "v2.0.0", []string{"--yes"}, nil, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "already up to date") || !strings.Contains(stdout.String(), "v2.0.0") {
		t.Fatalf("unexpected output: %q", stdout.String())
	}
}

func TestUpgradeCommandReportsSuccess(t *testing.T) {
	u := newFixtureUpgrader(t, "binary-content")
	var stdout, stderr bytes.Buffer
	if err := upgradeCommand(t.Context(), u, "v1.0.0", []string{"--yes"}, nil, &stdout, &stderr); err != nil {
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

// TestUpgradeCommandShowsDetailBeforeActing proves the CLI narrates what it found — current
// version, release source, and the latest version — before doing anything, as requested: not
// just a terse "already up to date" or "upgraded".
func TestUpgradeCommandShowsDetailBeforeActing(t *testing.T) {
	u := newFixtureUpgrader(t, "binary-content")
	var stdout, stderr bytes.Buffer
	if err := upgradeCommand(t.Context(), u, "v1.0.0", []string{"--yes"}, nil, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	out := stderr.String()
	for _, want := range []string{"v1.0.0", "v2.0.0", upgrade.SourceURL} {
		if !strings.Contains(out, want) {
			t.Errorf("stderr missing %q: %s", want, out)
		}
	}
}

// TestUpgradeCommandPromptsAndSkipsOnDecline proves declining the confirmation (the default
// without --yes) does not install anything, and is reported as a skip, not a hard failure.
func TestUpgradeCommandPromptsAndSkipsOnDecline(t *testing.T) {
	u := newFixtureUpgrader(t, "binary-content")
	var stdout, stderr bytes.Buffer
	err := upgradeCommand(t.Context(), u, "v1.0.0", nil, strings.NewReader("n\n"), &stdout, &stderr)
	if err == nil {
		t.Fatal("expected a cancellation error when the upgrade is declined")
	}
	if !strings.Contains(err.Error(), "skip") {
		t.Fatalf("error does not read as a skip: %v", err)
	}
	if strings.Contains(stdout.String(), "upgraded") {
		t.Fatalf("declined upgrade still reported success: %q", stdout.String())
	}
	// Apply's os.MkdirAll(destDir, ...) is the first thing that would ever create bin/ — a
	// decline must never reach that, so it must not exist at all.
	binDir := filepath.Dir(mustInstalledPath(t, u))
	if _, statErr := os.Stat(binDir); !os.IsNotExist(statErr) {
		t.Fatalf("declined upgrade touched the install directory: stat err=%v", statErr)
	}
}

// TestUpgradeCommandPromptsAndProceedsOnAccept proves accepting via stdin ("y"), without --yes,
// installs exactly like the --yes path does.
func TestUpgradeCommandPromptsAndProceedsOnAccept(t *testing.T) {
	u := newFixtureUpgrader(t, "binary-content")
	var stdout, stderr bytes.Buffer
	if err := upgradeCommand(t.Context(), u, "v1.0.0", nil, strings.NewReader("y\n"), &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "v1.0.0 -> v2.0.0") {
		t.Fatalf("accepted upgrade did not report success: %q", stdout.String())
	}
}

// TestUpgradeCommandNarratesProgress proves each stage of the actual install is reported, not
// just a single terse final line.
func TestUpgradeCommandNarratesProgress(t *testing.T) {
	u := newFixtureUpgrader(t, "binary-content")
	var stdout, stderr bytes.Buffer
	if err := upgradeCommand(t.Context(), u, "v1.0.0", []string{"--yes"}, nil, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	out := stderr.String()
	for _, want := range []string{"downloading", "verifying", "installing"} {
		if !strings.Contains(out, want) {
			t.Errorf("stderr missing progress stage %q: %s", want, out)
		}
	}
}

func mustInstalledPath(t *testing.T, u *upgrade.Upgrader) string {
	t.Helper()
	p, err := u.InstalledPath()
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// TestUpgradeCommandNeverTouchesCurrentEnv addresses a second report alongside the "dev" version
// bug: after running `env-switcher upgrade`, a previously-switched project appeared reactivated
// (its shell-cmd hooks visibly re-ran). upgradeCommand and the internal/upgrade package it calls
// never reference current-env or switchCommand at all (grep confirms it) — reactivation after a
// non-switch command is exactly the wrapper bug fixed in TestWrapperDoesNotReactivateOnNonSwitch
// Commands and TestHelpDoesNotReactivateProject; this proves the Go side of `upgrade` specifically
// carries no such risk, so a real report of it recurring means the *installed* shell wrapper on
// that machine predates the fix (env-switcher install/upgrade re-syncs it) — not a code regression
// here.
func TestUpgradeCommandNeverTouchesCurrentEnv(t *testing.T) {
	home := testutil.IsolatedHome(t)
	dir := filepath.Join(home, ".env-switcher")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	currentEnv := filepath.Join(dir, "current-env")
	preexisting := "export SOME_VAR='from-a-previous-switch'\n"
	if err := os.WriteFile(currentEnv, []byte(preexisting), 0o600); err != nil {
		t.Fatal(err)
	}

	u := newFixtureUpgrader(t, "binary-content")
	var stdout, stderr bytes.Buffer
	if err := upgradeCommand(t.Context(), u, "v1.0.0", []string{"--yes"}, nil, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(currentEnv)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != preexisting {
		t.Fatalf("upgrade modified current-env: got %q, want unchanged %q", got, preexisting)
	}
}

// TestUpgradeCommandWorksForLocalDevBuild reproduces the reported bug: a locally built binary
// (main.go's default BuildInfo.Version, "dev") could never upgrade at all, since "dev" isn't a
// semantic version to compare against. It must succeed, installing whatever the latest stable
// release is.
func TestUpgradeCommandWorksForLocalDevBuild(t *testing.T) {
	u := newFixtureUpgrader(t, "binary-content")
	var stdout, stderr bytes.Buffer
	if err := upgradeCommand(t.Context(), u, "dev", []string{"--yes"}, nil, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	out := stdout.String()
	if !strings.Contains(out, "dev -> v2.0.0") {
		t.Fatalf("output missing old/new version transition: %q", out)
	}
	if !strings.Contains(out, "installed at") {
		t.Fatalf("output missing installed path: %q", out)
	}
}
