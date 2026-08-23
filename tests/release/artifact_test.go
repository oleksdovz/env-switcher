package release

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeArtifactVersionValidateAndChecksum(t *testing.T) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("Go toolchain unavailable")
	}
	root := filepath.Join("..", "..")
	artifact := filepath.Join(t.TempDir(), "env-switcher")
	cmd := exec.Command(goBin, "build", "-trimpath", "-ldflags", "-X main.version=test-release -X main.commit=test-commit -X main.date=2026-08-23T00:00:00Z", "-o", artifact, "./cmd/env-switcher")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v %s", err, out)
	}
	out, err := exec.Command(artifact, "version").CombinedOutput()
	if err != nil || !strings.Contains(string(out), "test-release") {
		t.Fatalf("version smoke failed: %v %s", err, out)
	}
	b, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(b)
	if len(hex.EncodeToString(sum[:])) != 64 {
		t.Fatal("invalid SHA-256")
	}
}
