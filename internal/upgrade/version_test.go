package upgrade

import "testing"

func TestParseVersion(t *testing.T) {
	cases := []struct {
		in                  string
		major, minor, patch int
		pre                 string
		wantErr             bool
	}{
		{in: "v1.2.3", major: 1, minor: 2, patch: 3},
		{in: "1.2.3", major: 1, minor: 2, patch: 3},
		{in: "v1.0.0-rc.1", major: 1, minor: 0, patch: 0, pre: "rc.1"},
		{in: "v1.0.0+build5", major: 1, minor: 0, patch: 0},
		{in: "v1.0.0-rc.1+build5", major: 1, minor: 0, patch: 0, pre: "rc.1"},
		{in: "not-a-version", wantErr: true},
		{in: "v1.2", wantErr: true},
		{in: "v1.2.3.4", wantErr: true},
		{in: "v1.x.3", wantErr: true},
		{in: "", wantErr: true},
	}
	for _, c := range cases {
		v, err := ParseVersion(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParseVersion(%q): expected error, got %+v", c.in, v)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseVersion(%q): unexpected error: %v", c.in, err)
			continue
		}
		if v.Major != c.major || v.Minor != c.minor || v.Patch != c.patch || v.Prerelease != c.pre {
			t.Errorf("ParseVersion(%q) = %+v, want major=%d minor=%d patch=%d pre=%q", c.in, v, c.major, c.minor, c.patch, c.pre)
		}
	}
}

func TestVersionCompare(t *testing.T) {
	// Each row must have strictly higher precedence than the last (semver.org's own example
	// ordering, restricted to what this project actually needs to compare).
	ordered := []string{
		"v1.0.0-alpha",
		"v1.0.0-alpha.1",
		"v1.0.0-alpha.beta",
		"v1.0.0-beta",
		"v1.0.0-beta.2",
		"v1.0.0-beta.11",
		"v1.0.0-rc.1",
		"v1.0.0",
		"v1.0.1",
		"v1.1.0",
		"v2.0.0",
	}
	parsed := make([]Version, len(ordered))
	for i, s := range ordered {
		v, err := ParseVersion(s)
		if err != nil {
			t.Fatalf("ParseVersion(%q): %v", s, err)
		}
		parsed[i] = v
	}
	for i := 1; i < len(parsed); i++ {
		if !parsed[i].NewerThan(parsed[i-1]) {
			t.Errorf("%s should be newer than %s", parsed[i], parsed[i-1])
		}
		if parsed[i-1].NewerThan(parsed[i]) {
			t.Errorf("%s should not be newer than %s", parsed[i-1], parsed[i])
		}
		if parsed[i].Compare(parsed[i]) != 0 {
			t.Errorf("%s should equal itself", parsed[i])
		}
	}
}

func TestVersionCompareEqualIgnoresBuildMetadata(t *testing.T) {
	a, _ := ParseVersion("v1.2.3+build1")
	b, _ := ParseVersion("v1.2.3+build2")
	if a.Compare(b) != 0 {
		t.Fatalf("build metadata should not affect precedence: %s vs %s", a, b)
	}
}
