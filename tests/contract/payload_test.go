package contract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dolf/env-switcher/internal/environment"
	"github.com/dolf/env-switcher/internal/shell"
)

func TestRenderIsDeterministicAndMatchesGolden(t *testing.T) {
	for _, name := range []string{"bash", "zsh"} {
		e := &environment.Effective{Project: "dev", Shell: name, Variables: []environment.Variable{{Name: "VALUE", Value: "a'b"}}}
		a, err := shell.Render(e)
		if err != nil {
			t.Fatal(err)
		}
		b, err := shell.Render(e)
		if err != nil {
			t.Fatal(err)
		}
		if a != b {
			t.Fatal("render is not deterministic")
		}
		goldenPath := filepath.Join("..", "..", "testdata", "payload", name, "effective.golden")
		if os.Getenv("UPDATE_GOLDEN") == "1" {
			if err := os.WriteFile(goldenPath, []byte(a), 0o600); err != nil {
				t.Fatal(err)
			}
		} else {
			golden, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatal(err)
			}
			if a != string(golden) {
				t.Fatalf("%s render differs from golden fixture", name)
			}
		}
		if !strings.Contains(a, "VALUE='a'\\''b'") {
			t.Fatal("quoting contract missing")
		}
	}
}

func TestPayloadRejectsNULBeforeRendering(t *testing.T) {
	e := &environment.Effective{Project: "dev", Shell: "bash", Variables: []environment.Variable{{Name: "VALUE", Value: "bad\x00value"}}}
	if _, err := shell.Render(e); err == nil {
		t.Fatal("NUL payload rendered")
	}
}
