package upgrade

import (
	"fmt"
	"sort"
	"strings"
)

// AssetName is the naming convention this project's release workflow uses (see
// .github/workflows/release.yml): "env-switcher_<GOOS>_<GOARCH>.zip".
func AssetName(p Platform) string {
	return fmt.Sprintf("env-switcher_%s_%s.zip", p.OS, p.Arch)
}

// ChecksumAssetName is the name of the single checksum file published alongside every release's
// platform archives.
const ChecksumAssetName = "SHA256SUMS"

// SelectAsset finds the release asset matching platform. If none matches, the returned error
// lists what the release does publish, so the failure is actionable instead of a bare "not
// found".
func SelectAsset(release Release, p Platform) (Asset, error) {
	want := AssetName(p)
	if a, ok := release.Asset(want); ok {
		return a, nil
	}
	var available []string
	for _, a := range release.Assets {
		if a.Name == ChecksumAssetName {
			continue
		}
		available = append(available, a.Name)
	}
	sort.Strings(available)
	if len(available) == 0 {
		return Asset{}, fmt.Errorf("release %s publishes no platform assets", release.TagName)
	}
	return Asset{}, fmt.Errorf(
		"no %s release build for %s (looked for %q); available builds: %s",
		release.TagName, p, want, strings.Join(available, ", "),
	)
}
