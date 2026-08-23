package config

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestMaximumScaleParsesWithinTwoSeconds(t *testing.T) {
	var b strings.Builder
	b.WriteString("version: 1\nenvs:\n")
	for p := 0; p < 100; p++ {
		fmt.Fprintf(&b, "  project_%03d:\n    project: /tmp\n    env-vars:\n", p)
		for i := 0; i < 100; i++ {
			fmt.Fprintf(&b, "      VAR_%03d: value\n", i)
		}
		b.WriteString("    shell-functions:\n")
		for i := 0; i < 100; i++ {
			fmt.Fprintf(&b, "      fn_%03d: ': '\n", i)
		}
	}
	if b.Len() > MaxSettingsSize {
		t.Fatalf("fixture exceeds settings limit: %d", b.Len())
	}
	started := time.Now()
	if _, err := Parse(strings.NewReader(b.String())); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("maximum-scale reload took %s, limit is 2s", elapsed)
	} else {
		t.Logf("maximum-scale reload: %s", elapsed)
	}
}
