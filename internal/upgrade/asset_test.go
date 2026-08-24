package upgrade

import (
	"strings"
	"testing"
)

func TestSelectAssetMatchesOSAndArch(t *testing.T) {
	release := Release{
		TagName: "v1.2.0",
		Assets: []Asset{
			{Name: "env-switcher_linux_amd64.zip"},
			{Name: "env-switcher_darwin_arm64.zip"},
			{Name: ChecksumAssetName},
		},
	}
	got, err := SelectAsset(release, Platform{OS: "darwin", Arch: "arm64"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "env-switcher_darwin_arm64.zip" {
		t.Fatalf("got %q", got.Name)
	}
}

func TestSelectAssetNoMatchIsActionable(t *testing.T) {
	release := Release{
		TagName: "v1.2.0",
		Assets: []Asset{
			{Name: "env-switcher_linux_amd64.zip"},
			{Name: "env-switcher_darwin_arm64.zip"},
			{Name: ChecksumAssetName},
		},
	}
	_, err := SelectAsset(release, Platform{OS: "windows", Arch: "amd64"})
	if err == nil {
		t.Fatal("expected an error for an unsupported platform")
	}
	msg := err.Error()
	for _, want := range []string{"windows/amd64", "env-switcher_windows_amd64.zip", "env-switcher_linux_amd64.zip", "env-switcher_darwin_arm64.zip"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q missing %q", msg, want)
		}
	}
	if strings.Contains(msg, ChecksumAssetName) {
		t.Errorf("error %q should not list the checksum file as an available build", msg)
	}
}

func TestSelectAssetNoPlatformAssetsAtAll(t *testing.T) {
	release := Release{TagName: "v1.2.0", Assets: []Asset{{Name: ChecksumAssetName}}}
	_, err := SelectAsset(release, Platform{OS: "linux", Arch: "amd64"})
	if err == nil {
		t.Fatal("expected an error")
	}
}
