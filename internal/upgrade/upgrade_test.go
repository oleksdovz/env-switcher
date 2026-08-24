package upgrade

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
)

type fakeReleaseSource struct {
	release Release
	err     error
}

func (f fakeReleaseSource) LatestStable(context.Context) (Release, error) { return f.release, f.err }

// zippedAsset builds a zip archive containing a single "env-switcher" entry with the given
// content, and returns the bytes plus their SHA-256 hex digest (what a real SHA256SUMS entry
// would contain for it).
func zippedAsset(t *testing.T, content string) (zipBytes []byte, digest string) {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	hdr := &zip.FileHeader{Name: "env-switcher", Method: zip.Deflate}
	hdr.SetMode(0o755)
	fw, err := w.CreateHeader(hdr)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(buf.Bytes())
	return buf.Bytes(), hex.EncodeToString(sum[:])
}

// upgradeFixture serves a release's zip asset and (optionally corrupted) checksum file over
// HTTP, and returns an Upgrader wired to fakes/that server — no live GitHub access.
type upgradeFixture struct {
	srv       *httptest.Server
	assetName string
	upgrader  *Upgrader
	destPath  string
}

func newUpgradeFixture(t *testing.T, content, checksumOverride string) upgradeFixture {
	t.Helper()
	zipBytes, digest := zippedAsset(t, content)
	if checksumOverride != "" {
		digest = checksumOverride
	}
	assetName := AssetName(CurrentPlatform())
	dir := t.TempDir()

	mux := http.NewServeMux()
	mux.HandleFunc("/asset.zip", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(zipBytes)
	})
	mux.HandleFunc("/SHA256SUMS", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s  %s\n", digest, assetName)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	release := Release{
		TagName: "v2.0.0",
		Assets: []Asset{
			{Name: assetName, DownloadURL: srv.URL + "/asset.zip", Size: int64(len(zipBytes))},
			{Name: ChecksumAssetName, DownloadURL: srv.URL + "/SHA256SUMS"},
		},
	}
	destPath := filepath.Join(dir, "bin", "env-switcher")
	u := &Upgrader{
		Source:        fakeReleaseSource{release: release},
		Checksums:     &GitHubChecksumSource{Client: srv.Client()},
		Client:        srv.Client(),
		Platform:      CurrentPlatform(),
		InstalledPath: func() (string, error) { return destPath, nil },
	}
	return upgradeFixture{srv: srv, assetName: assetName, upgrader: u, destPath: destPath}
}

func TestUpgradeInstallsNewerRelease(t *testing.T) {
	fx := newUpgradeFixture(t, "new-binary-content", "")
	result, err := fx.upgrader.Upgrade(t.Context(), "v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if result.AlreadyCurrent {
		t.Fatal("should not report already current")
	}
	if result.OldVersion != "v1.0.0" || result.NewVersion != "v2.0.0" {
		t.Fatalf("unexpected versions: %+v", result)
	}
	if result.InstalledPath != fx.destPath {
		t.Fatalf("installed path = %q, want %q", result.InstalledPath, fx.destPath)
	}
	got, err := os.ReadFile(fx.destPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new-binary-content" {
		t.Fatalf("installed content = %q", got)
	}
	info, err := os.Stat(fx.destPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("installed mode = %v, want 0700", info.Mode().Perm())
	}
	// No leftover temp files in the install directory.
	entries, err := os.ReadDir(filepath.Dir(fx.destPath))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "env-switcher" {
		t.Fatalf("unexpected leftover files: %v", entries)
	}
}

func TestUpgradeAlreadyCurrentInstallsNothing(t *testing.T) {
	fx := newUpgradeFixture(t, "new-binary-content", "")
	result, err := fx.upgrader.Upgrade(t.Context(), "v2.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if !result.AlreadyCurrent {
		t.Fatal("expected already-current")
	}
	if result.InstalledPath != "" {
		t.Fatalf("should not report an installed path, got %q", result.InstalledPath)
	}
	if _, err := os.Stat(fx.destPath); !os.IsNotExist(err) {
		t.Fatalf("nothing should have been installed, stat err=%v", err)
	}
}

func TestUpgradeAlreadyCurrentWhenInstalledIsNewer(t *testing.T) {
	fx := newUpgradeFixture(t, "new-binary-content", "")
	result, err := fx.upgrader.Upgrade(t.Context(), "v3.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if !result.AlreadyCurrent {
		t.Fatal("a newer-than-latest installed version must not be treated as an available upgrade")
	}
}

func TestUpgradeChecksumMismatchPreservesExistingBinary(t *testing.T) {
	fx := newUpgradeFixture(t, "new-binary-content", strings.Repeat("0", 64))
	// Pre-existing install this must not touch.
	if err := os.MkdirAll(filepath.Dir(fx.destPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fx.destPath, []byte("old-binary"), 0o700); err != nil {
		t.Fatal(err)
	}

	_, err := fx.upgrader.Upgrade(t.Context(), "v1.0.0")
	if err == nil {
		t.Fatal("expected a checksum verification error")
	}
	got, readErr := os.ReadFile(fx.destPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != "old-binary" {
		t.Fatalf("existing binary was modified after a failed upgrade: %q", got)
	}
	// No stray temp files left behind either.
	entries, err := os.ReadDir(filepath.Dir(fx.destPath))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "env-switcher" {
		t.Fatalf("temp files were not cleaned up: %v", entries)
	}
}

func TestUpgradeMissingChecksumEntryIsBlocker(t *testing.T) {
	zipBytes, _ := zippedAsset(t, "content")
	assetName := AssetName(CurrentPlatform())
	mux := http.NewServeMux()
	mux.HandleFunc("/asset.zip", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(zipBytes) })
	mux.HandleFunc("/SHA256SUMS", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s  a-different-asset.zip\n", strings.Repeat("0", 64))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	dir := t.TempDir()
	release := Release{TagName: "v2.0.0", Assets: []Asset{
		{Name: assetName, DownloadURL: srv.URL + "/asset.zip"},
		{Name: ChecksumAssetName, DownloadURL: srv.URL + "/SHA256SUMS"},
	}}
	u := &Upgrader{
		Source:        fakeReleaseSource{release: release},
		Checksums:     &GitHubChecksumSource{Client: srv.Client()},
		Client:        srv.Client(),
		Platform:      CurrentPlatform(),
		InstalledPath: func() (string, error) { return filepath.Join(dir, "bin", "env-switcher"), nil },
	}
	_, err := u.Upgrade(t.Context(), "v1.0.0")
	if err == nil {
		t.Fatal("expected an error when the checksum file doesn't list this asset")
	}
}

func TestUpgradeNoChecksumFilePublishedIsBlocker(t *testing.T) {
	zipBytes, _ := zippedAsset(t, "content")
	assetName := AssetName(CurrentPlatform())
	mux := http.NewServeMux()
	mux.HandleFunc("/asset.zip", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(zipBytes) })
	srv := httptest.NewServer(mux)
	defer srv.Close()

	dir := t.TempDir()
	release := Release{TagName: "v2.0.0", Assets: []Asset{
		{Name: assetName, DownloadURL: srv.URL + "/asset.zip"},
		// No SHA256SUMS asset at all.
	}}
	u := &Upgrader{
		Source:        fakeReleaseSource{release: release},
		Checksums:     &GitHubChecksumSource{Client: srv.Client()},
		Client:        srv.Client(),
		Platform:      CurrentPlatform(),
		InstalledPath: func() (string, error) { return filepath.Join(dir, "bin", "env-switcher"), nil },
	}
	_, err := u.Upgrade(t.Context(), "v1.0.0")
	if err != ErrChecksumsUnavailable {
		t.Fatalf("got %v, want ErrChecksumsUnavailable", err)
	}
}

func TestUpgradeUnsupportedPlatformIsActionable(t *testing.T) {
	dir := t.TempDir()
	release := Release{TagName: "v2.0.0", Assets: []Asset{{Name: "env-switcher_plan9_386.zip"}}}
	u := &Upgrader{
		Source:        fakeReleaseSource{release: release},
		Checksums:     &GitHubChecksumSource{},
		Client:        http.DefaultClient,
		Platform:      Platform{OS: "windows", Arch: "amd64"},
		InstalledPath: func() (string, error) { return filepath.Join(dir, "bin", "env-switcher"), nil },
	}
	_, err := u.Upgrade(t.Context(), "v1.0.0")
	if err == nil {
		t.Fatal("expected an error for an unsupported platform")
	}
}

func TestCheckReportsAvailableUpgradeWithoutDownloading(t *testing.T) {
	fx := newUpgradeFixture(t, "content", "")
	check, err := fx.upgrader.Check(t.Context(), "v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if !check.UpgradeAvailable || check.CurrentLabel != "v1.0.0" || check.NewVersion != "v2.0.0" || !check.CurrentIsRelease {
		t.Fatalf("unexpected check result: %+v", check)
	}
	if _, err := os.Stat(fx.destPath); !os.IsNotExist(err) {
		t.Fatalf("Check must not install anything, stat err=%v", err)
	}
}

func TestCheckReportsAlreadyCurrent(t *testing.T) {
	fx := newUpgradeFixture(t, "content", "")
	check, err := fx.upgrader.Check(t.Context(), "v2.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if check.UpgradeAvailable {
		t.Fatalf("v2.0.0 should already be current: %+v", check)
	}
}

func TestApplyUsesTheGivenReleaseWithoutRequeryingSource(t *testing.T) {
	fx := newUpgradeFixture(t, "content", "")
	check, err := fx.upgrader.Check(t.Context(), "v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	// Swap the source out after Check: Apply must not call LatestStable again, or this would
	// either error or (worse) silently pick a different release than what Check reported.
	fx.upgrader.Source = fakeReleaseSource{err: fmt.Errorf("Apply must not query the release source")}
	result, err := fx.upgrader.Apply(t.Context(), check.Release, check.CurrentLabel)
	if err != nil {
		t.Fatal(err)
	}
	if result.NewVersion != "v2.0.0" || result.OldVersion != "v1.0.0" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestApplyReportsProgress(t *testing.T) {
	fx := newUpgradeFixture(t, "content", "")
	check, err := fx.upgrader.Check(t.Context(), "v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	var stages []string
	fx.upgrader.Progress = func(stage string) { stages = append(stages, stage) }
	if _, err := fx.upgrader.Apply(t.Context(), check.Release, check.CurrentLabel); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(stages, " | ")
	for _, want := range []string{"download", "verif", "install"} {
		if !strings.Contains(joined, want) {
			t.Errorf("progress stages missing %q: %v", want, stages)
		}
	}
}

// TestUpgradeUnparseableCurrentVersionAlwaysInstallsLatest covers local/dev builds: main.go
// defaults BuildInfo.Version to the literal "dev" when it's not set via release ldflags, which
// doesn't parse as a semantic version. Rather than refuse to compare (and so refuse to upgrade
// at all — reported as a real bug, since it left a dev build with no way to reach a release),
// an unparseable current version is treated as "definitely not current": install latest.
func TestUpgradeUnparseableCurrentVersionAlwaysInstallsLatest(t *testing.T) {
	fx := newUpgradeFixture(t, "new-binary-content", "")
	for _, currentVersion := range []string{"dev", "not-a-version", ""} {
		t.Run(currentVersion, func(t *testing.T) {
			result, err := fx.upgrader.Upgrade(t.Context(), currentVersion)
			if err != nil {
				t.Fatal(err)
			}
			if result.AlreadyCurrent {
				t.Fatalf("%q should never be reported as already current", currentVersion)
			}
			if result.OldVersion != currentVersion {
				t.Fatalf("OldVersion = %q, want the raw %q preserved", result.OldVersion, currentVersion)
			}
			if result.NewVersion != "v2.0.0" || result.InstalledPath == "" {
				t.Fatalf("unexpected result: %+v", result)
			}
		})
	}
}
