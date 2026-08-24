package upgrade

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/dolf/env-switcher/internal/fsatomic"
)

// Owner and Repo identify the GitHub repository this project publishes releases to. This is
// deliberately independent of the Go module path (github.com/dolf/env-switcher): the module path
// predates this repository's current home.
const (
	Owner = "oleksdovz"
	Repo  = "env-switcher"
)

// executableAssetName is the file this project's release workflow places inside every platform
// zip (see .github/workflows/release.yml: `zip ... env-switcher`).
const executableAssetName = "env-switcher"

// Result summarizes a finished upgrade check/attempt.
type Result struct {
	OldVersion string
	NewVersion string
	// InstalledPath is set only when a new binary was actually installed.
	InstalledPath string
	// AlreadyCurrent is true when the installed version is already the latest stable release
	// (NewVersion is still populated, so the caller can report what "current" means).
	AlreadyCurrent bool
}

// Upgrader finds and installs the latest compatible stable release. Its dependencies are all
// interfaces or plain function values so tests can substitute fixtures instead of live GitHub —
// see NewUpgrader for the real wiring and upgrade_test.go for the fakes.
type Upgrader struct {
	Source    ReleaseSource
	Checksums ChecksumSource
	Client    *http.Client
	Platform  Platform
	// InstalledPath returns the canonical install destination, e.g. via config.ExecutablePath.
	InstalledPath func() (string, error)
}

// NewUpgrader builds an Upgrader wired to the real oleksdovz/env-switcher GitHub repository.
func NewUpgrader(installedPath func() (string, error)) *Upgrader {
	client := newHTTPClient()
	return &Upgrader{
		Source:        &GitHubSource{Owner: Owner, Repo: Repo, BaseURL: "https://api.github.com", Client: client},
		Checksums:     &GitHubChecksumSource{Client: client},
		Client:        client,
		Platform:      CurrentPlatform(),
		InstalledPath: installedPath,
	}
}

// Upgrade checks the latest stable release against currentVersion and, if it's newer, downloads,
// verifies, and atomically installs it. currentVersion is the running binary's own build-time
// version string (see internal/app.BuildInfo.Version) — a local/dev build (unset by the release
// workflow's ldflags, e.g. the literal "dev") doesn't parse as a semantic version, and rather than
// refuse to compare, that's treated as "not a release, so not current": the latest stable release
// is always installed over it. The existing installed binary is left exactly as it was if any
// step — network, checksum, extraction, or install — fails.
func (u *Upgrader) Upgrade(ctx context.Context, currentVersion string) (Result, error) {
	current, parseErr := ParseVersion(currentVersion)
	// oldVersionLabel is what Result reports as "old": the parsed/canonical form when
	// currentVersion is a real release version, otherwise the raw string as given (e.g. "dev") —
	// never a blank zero-value Version.
	oldVersionLabel := currentVersion
	if parseErr == nil {
		oldVersionLabel = current.String()
	}

	release, err := u.Source.LatestStable(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("check for updates: %w", err)
	}
	latest, err := release.Version()
	if err != nil {
		return Result{}, fmt.Errorf("latest release %q has an unparseable version: %w", release.TagName, err)
	}
	if parseErr == nil && !latest.NewerThan(current) {
		return Result{OldVersion: oldVersionLabel, NewVersion: latest.String(), AlreadyCurrent: true}, nil
	}

	destPath, err := u.InstalledPath()
	if err != nil {
		return Result{}, fmt.Errorf("resolve install path: %w", err)
	}
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0o700); err != nil {
		return Result{}, fmt.Errorf("prepare %s: %w", destDir, err)
	}

	asset, err := SelectAsset(release, u.Platform)
	if err != nil {
		return Result{}, err
	}

	sums, err := u.Checksums.Fetch(ctx, release)
	if err != nil {
		return Result{}, err
	}
	wantSum, ok := sums[asset.Name]
	if !ok {
		return Result{}, fmt.Errorf("%s does not list a checksum for %s; refusing to install an unverified binary", ChecksumAssetName, asset.Name)
	}

	downloadPath, err := reserveTempPath(destDir, "download")
	if err != nil {
		return Result{}, err
	}
	defer os.Remove(downloadPath)

	limit := int64(maxResponseBytes)
	if asset.Size > 0 && asset.Size+(1<<20) < limit {
		limit = asset.Size + (1 << 20) // small slack over the size GitHub reports
	}
	if _, err := downloadToFile(ctx, u.Client, asset.DownloadURL, downloadPath, limit); err != nil {
		return Result{}, fmt.Errorf("download %s: %w", asset.Name, err)
	}
	if err := VerifyFile(downloadPath, wantSum); err != nil {
		return Result{}, fmt.Errorf("%s failed checksum verification: %w", asset.Name, err)
	}

	finalPath, err := reserveTempPath(destDir, "bin")
	if err != nil {
		return Result{}, err
	}
	defer os.Remove(finalPath)

	if strings.HasSuffix(asset.Name, ".zip") {
		if err := ExtractExecutable(downloadPath, finalPath, executableAssetName); err != nil {
			return Result{}, fmt.Errorf("extract %s: %w", asset.Name, err)
		}
	} else {
		// Not an archive: the downloaded file is the executable itself.
		if err := copyFile(downloadPath, finalPath); err != nil {
			return Result{}, fmt.Errorf("stage %s: %w", asset.Name, err)
		}
	}

	if err := fsatomic.Publish(finalPath, destPath, 0o700); err != nil {
		return Result{}, fmt.Errorf("install %s: %w", destPath, err)
	}

	return Result{OldVersion: oldVersionLabel, NewVersion: latest.String(), InstalledPath: destPath}, nil
}

// reserveTempPath returns a unique path in dir (created empty, so later writers can just
// truncate into it) for the given purpose, satisfying "download to a temporary file in the
// destination directory" — same filesystem as the eventual install target, so the final publish
// is a plain atomic rename.
func reserveTempPath(dir, purpose string) (string, error) {
	f, err := os.CreateTemp(dir, "env-switcher-upgrade-"+purpose+"-*.tmp")
	if err != nil {
		return "", err
	}
	path := f.Name()
	_ = f.Close()
	return path, nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o700)
}
