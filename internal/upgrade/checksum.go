package upgrade

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// ErrChecksumsUnavailable means a release publishes no checksum file. This is a hard blocker,
// never a silent skip: see ChecksumSource's doc comment.
var ErrChecksumsUnavailable = errors.New(
	"release does not publish a " + ChecksumAssetName + " checksum file; " +
		"refusing to install an unverified binary. This requires a release-workflow change: " +
		"add a step that runs `sha256sum *.zip > " + ChecksumAssetName + "` and uploads it " +
		"as a release asset (see .github/workflows/release.yml, which already does this — " +
		"if this error appears, the release in question predates that step or omitted it)",
)

// Checksums maps an asset's exact file name to its lowercase hex-encoded SHA-256 digest.
type Checksums map[string]string

// ParseChecksums parses a sha256sum(1)-style file: one "<64-hex-digest>  <filename>" line per
// asset (the format the project's release workflow produces via `sha256sum *.zip`).
func ParseChecksums(data []byte) (Checksums, error) {
	sums := Checksums{}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, fmt.Errorf("malformed checksum line %q", line)
		}
		digest, name := strings.ToLower(fields[0]), fields[1]
		name = strings.TrimPrefix(name, "*") // sha256sum's binary-mode marker
		if len(digest) != sha256.Size*2 {
			return nil, fmt.Errorf("malformed checksum for %q: not a 64-character SHA-256 hex digest", name)
		}
		if _, err := hex.DecodeString(digest); err != nil {
			return nil, fmt.Errorf("malformed checksum for %q: %w", name, err)
		}
		sums[name] = digest
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(sums) == 0 {
		return nil, fmt.Errorf("checksum file had no entries")
	}
	return sums, nil
}

// ChecksumSource retrieves the published checksums for a release's assets.
type ChecksumSource interface {
	Fetch(ctx context.Context, release Release) (Checksums, error)
}

// GitHubChecksumSource fetches and parses the release's SHA256SUMS asset.
type GitHubChecksumSource struct {
	Client *http.Client
}

func (s *GitHubChecksumSource) Fetch(ctx context.Context, release Release) (Checksums, error) {
	asset, ok := release.Asset(ChecksumAssetName)
	if !ok {
		return nil, ErrChecksumsUnavailable
	}
	client := s.Client
	if client == nil {
		client = newHTTPClient()
	}
	data, err := fetchBytes(ctx, client, asset.DownloadURL, maxResponseBytes)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", ChecksumAssetName, err)
	}
	sums, err := ParseChecksums(data)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", ChecksumAssetName, err)
	}
	return sums, nil
}

// VerifyFile checks that path's SHA-256 digest matches want (a lowercase hex digest).
func VerifyFile(path, want string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("checksum mismatch: got %s, want %s", got, want)
	}
	return nil
}
