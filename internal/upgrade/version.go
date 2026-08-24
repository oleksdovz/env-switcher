package upgrade

import (
	"fmt"
	"strconv"
	"strings"
)

// Version is a parsed semantic version (semver.org), e.g. "v1.2.3-rc.1+build" — build metadata
// is retained for display but never affects comparison.
type Version struct {
	Major, Minor, Patch int
	Prerelease          string
	raw                 string
}

// ParseVersion parses a semantic version, tolerating a leading "v" (the tag/build format this
// project and its release workflow use, e.g. "v1.0.0").
func ParseVersion(s string) (Version, error) {
	raw := strings.TrimSpace(s)
	trimmed := strings.TrimPrefix(raw, "v")
	trimmed = strings.TrimPrefix(trimmed, "V")

	core := trimmed
	if i := strings.IndexByte(core, '+'); i >= 0 {
		core = core[:i] // build metadata: retained on raw, ignored for precedence
	}
	var pre string
	if i := strings.IndexByte(core, '-'); i >= 0 {
		pre = core[i+1:]
		core = core[:i]
	}

	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("invalid semantic version %q", raw)
	}
	nums := make([]int, 3)
	for i, p := range parts {
		if p == "" {
			return Version{}, fmt.Errorf("invalid semantic version %q", raw)
		}
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return Version{}, fmt.Errorf("invalid semantic version %q", raw)
		}
		nums[i] = n
	}
	return Version{Major: nums[0], Minor: nums[1], Patch: nums[2], Prerelease: pre, raw: raw}, nil
}

// String returns the version exactly as parsed (including any "v" prefix and build metadata).
func (v Version) String() string { return v.raw }

// Compare returns -1, 0, or +1 as v is less than, equal to, or greater than o, following semver
// precedence rules: major, then minor, then patch, then prerelease (a version with a prerelease
// has lower precedence than the same version without one).
func (v Version) Compare(o Version) int {
	if v.Major != o.Major {
		return cmpInt(v.Major, o.Major)
	}
	if v.Minor != o.Minor {
		return cmpInt(v.Minor, o.Minor)
	}
	if v.Patch != o.Patch {
		return cmpInt(v.Patch, o.Patch)
	}
	return comparePrerelease(v.Prerelease, o.Prerelease)
}

// NewerThan reports whether v has higher precedence than o.
func (v Version) NewerThan(o Version) bool { return v.Compare(o) > 0 }

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

// comparePrerelease implements semver.org's prerelease precedence: no prerelease outranks any
// prerelease; otherwise identifiers are compared left to right, numeric identifiers compare
// numerically and are always lower than alphanumeric ones, and a version with more identifiers
// outranks a version whose identifiers are all otherwise equal.
func comparePrerelease(a, b string) int {
	if a == "" && b == "" {
		return 0
	}
	if a == "" {
		return 1
	}
	if b == "" {
		return -1
	}
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	for i := 0; i < len(as) && i < len(bs); i++ {
		if c := compareIdentifier(as[i], bs[i]); c != 0 {
			return c
		}
	}
	return cmpInt(len(as), len(bs))
}

func compareIdentifier(a, b string) int {
	an, aErr := strconv.Atoi(a)
	bn, bErr := strconv.Atoi(b)
	aNum, bNum := aErr == nil, bErr == nil
	switch {
	case aNum && bNum:
		return cmpInt(an, bn)
	case aNum:
		return -1 // numeric identifiers always have lower precedence than alphanumeric ones
	case bNum:
		return 1
	default:
		return strings.Compare(a, b)
	}
}
