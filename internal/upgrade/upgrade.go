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

// SourceURL is the human-readable location Check/Upgrade query, for callers that want to display
// it (e.g. "source: https://github.com/oleksdovz/env-switcher").
const SourceURL = "https://github.com/" + Owner + "/" + Repo

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

// CheckResult is the outcome of comparing the running version against the latest stable release —
// before any download happens, so a caller can show the user what was found and let them decide
// whether to Apply it.
type CheckResult struct {
	// CurrentLabel is the running version as it should be reported: the parsed/canonical form
	// when currentVersion (passed to Check) was a real release version, otherwise the raw string
	// as given (e.g. "dev" for a local build) — never blank.
	CurrentLabel string
	// CurrentIsRelease is false when the running version doesn't parse as a semantic version at
	// all (a local/dev build) — see Upgrade's doc comment for what that means for comparison.
	CurrentIsRelease bool
	// Release is the latest stable release found, kept so a subsequent Apply call doesn't need
	// to query the release source again.
	Release Release
	// NewVersion is Release's version, canonically formatted.
	NewVersion string
	// UpgradeAvailable is true when Release is newer than the running version (or the running
	// version isn't a release at all, so it's treated as definitely not current).
	UpgradeAvailable bool
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
	// Progress, if set, is called with a short human-readable stage description as Apply moves
	// through downloading, verifying, and installing — for a caller (the CLI) that wants to print
	// step-by-step output. Check does not call it: querying release metadata is a single network
	// round trip with nothing worth narrating in stages. Never called concurrently.
	Progress func(stage string)
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

func (u *Upgrader) progress(stage string) {
	if u.Progress != nil {
		u.Progress(stage)
	}
}

// Check queries the latest stable release and compares it against currentVersion — the running
// binary's own build-time version string (see internal/app.BuildInfo.Version) — without
// downloading anything. currentVersion not parsing as a semantic version (a local/dev build,
// e.g. the literal "dev") is treated as "definitely not current" rather than an error: there is
// no meaningful release version to compare against, so an upgrade is always considered available.
func (u *Upgrader) Check(ctx context.Context, currentVersion string) (CheckResult, error) {
	current, parseErr := ParseVersion(currentVersion)
	label := currentVersion
	if parseErr == nil {
		label = current.String()
	}

	release, err := u.Source.LatestStable(ctx)
	if err != nil {
		return CheckResult{}, fmt.Errorf("check for updates: %w", err)
	}
	latest, err := release.Version()
	if err != nil {
		return CheckResult{}, fmt.Errorf("latest release %q has an unparseable version: %w", release.TagName, err)
	}

	available := parseErr != nil || latest.NewerThan(current)
	return CheckResult{
		CurrentLabel:     label,
		CurrentIsRelease: parseErr == nil,
		Release:          release,
		NewVersion:       latest.String(),
		UpgradeAvailable: available,
	}, nil
}

// Apply downloads, verifies, and atomically installs release's asset for the running platform.
// currentLabel is only carried through into the returned Result.OldVersion for reporting; it does
// not affect what gets installed. The existing installed binary is left exactly as it was if any
// step — network, checksum, extraction, or install — fails.
func (u *Upgrader) Apply(ctx context.Context, release Release, currentLabel string) (Result, error) {
	latest, err := release.Version()
	if err != nil {
		return Result{}, fmt.Errorf("release %q has an unparseable version: %w", release.TagName, err)
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

	u.progress(fmt.Sprintf("downloading %s", asset.Name))
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

	u.progress("verifying checksum")
	if err := VerifyFile(downloadPath, wantSum); err != nil {
		return Result{}, fmt.Errorf("%s failed checksum verification: %w", asset.Name, err)
	}

	finalPath, err := reserveTempPath(destDir, "bin")
	if err != nil {
		return Result{}, err
	}
	defer os.Remove(finalPath)

	if strings.HasSuffix(asset.Name, ".zip") {
		u.progress("extracting " + executableAssetName)
		if err := ExtractExecutable(downloadPath, finalPath, executableAssetName); err != nil {
			return Result{}, fmt.Errorf("extract %s: %w", asset.Name, err)
		}
	} else {
		// Not an archive: the downloaded file is the executable itself.
		if err := copyFile(downloadPath, finalPath); err != nil {
			return Result{}, fmt.Errorf("stage %s: %w", asset.Name, err)
		}
	}

	u.progress("installing " + destPath)
	if err := fsatomic.Publish(finalPath, destPath, 0o700); err != nil {
		return Result{}, fmt.Errorf("install %s: %w", destPath, err)
	}

	return Result{OldVersion: currentLabel, NewVersion: latest.String(), InstalledPath: destPath}, nil
}

// Upgrade is Check then, if an upgrade is available, Apply — the single-call convenience API for
// a caller (the TUI's F6, which has its own confirmation dialog) that doesn't need to inspect or
// act on the check result before deciding whether to proceed. The CLI's "upgrade"/"--upgrade"
// command uses Check and Apply directly instead, so it can show what was found and ask first.
func (u *Upgrader) Upgrade(ctx context.Context, currentVersion string) (Result, error) {
	check, err := u.Check(ctx, currentVersion)
	if err != nil {
		return Result{}, err
	}
	if !check.UpgradeAvailable {
		return Result{OldVersion: check.CurrentLabel, NewVersion: check.NewVersion, AlreadyCurrent: true}, nil
	}
	return u.Apply(ctx, check.Release, check.CurrentLabel)
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
