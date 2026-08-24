package upgrade

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseChecksumsValid(t *testing.T) {
	const digestA = "769b206a3f59eae2ed7455e4a9269121ce20f26534c06e32f9fa6085c7274a63"
	const digestB = "7b4875f447a821ed6920925775a6aa62791a87298e1637fd2ba8f4b9a0d72637"
	data := []byte(
		digestA + "  env-switcher_darwin_amd64.zip\n" +
			digestB + "  env-switcher_darwin_arm64.zip\n",
	)
	sums, err := ParseChecksums(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(sums) != 2 {
		t.Fatalf("got %d entries, want 2", len(sums))
	}
	if sums["env-switcher_darwin_amd64.zip"] != digestA {
		t.Fatalf("unexpected digest: %+v", sums)
	}
}

func TestParseChecksumsRejectsMalformed(t *testing.T) {
	cases := []string{
		"not-a-checksum-line\n",
		"deadbeef env-switcher_linux_amd64.zip\n", // digest too short
		"gg2d2fcd1ee0a9e27afc7538343e5db69c5629138358c817ffe8c75534bd9c0  env-switcher_linux_amd64.zip\n", // not hex
		"",
	}
	for _, c := range cases {
		if _, err := ParseChecksums([]byte(c)); err == nil {
			t.Errorf("ParseChecksums(%q): expected error", c)
		}
	}
}

func TestVerifyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "asset")
	if err := os.WriteFile(path, []byte("hello world"), 0o600); err != nil {
		t.Fatal(err)
	}
	// sha256("hello world")
	const want = "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	if err := VerifyFile(path, want); err != nil {
		t.Fatalf("expected match: %v", err)
	}
	if err := VerifyFile(path, strings.Repeat("0", 64)); err == nil {
		t.Fatal("expected a mismatch error")
	}
}

func TestGitHubChecksumSourceMissingAssetIsBlocker(t *testing.T) {
	src := &GitHubChecksumSource{}
	_, err := src.Fetch(t.Context(), Release{TagName: "v1.0.0"})
	if err != ErrChecksumsUnavailable {
		t.Fatalf("got %v, want ErrChecksumsUnavailable", err)
	}
}
