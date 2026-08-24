// Package upgrade implements the self-upgrade feature: find the latest compatible stable
// release of oleksdovz/env-switcher, verify it, and install it over the running binary.
//
// The pieces are deliberately separate and each independently testable against fakes/fixtures
// rather than live GitHub access:
//
//   - Platform (platform.go): the running GOOS/GOARCH.
//   - Version (version.go): semantic-version parsing and comparison.
//   - ReleaseSource (github.go): release metadata retrieval, filtering out drafts/prereleases.
//   - SelectAsset (asset.go): matching a release's assets to the current platform.
//   - ChecksumSource (checksum.go): fetching and parsing the release's checksum file.
//   - install.go: safe archive extraction and atomic installation.
//   - Upgrader (upgrade.go): orchestrates the above into the single Upgrade call used by both
//     the "upgrade"/"--upgrade" CLI command and the TUI's F6 shortcut, so neither duplicates the
//     other's logic — see internal/app/upgrade.go and internal/tui's Services.Upgrade.
package upgrade
