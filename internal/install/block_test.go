package install

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestReconcileIdempotentAndPreservesBytes(t *testing.T) {
	original := []byte("before\n")
	block := Block("fn() { :; }")
	once, changed, err := Reconcile(original, block)
	if err != nil || !changed {
		t.Fatal(err)
	}
	twice, changed, err := Reconcile(once, block)
	if err != nil || changed || !bytes.Equal(once, twice) {
		t.Fatal("not idempotent")
	}
	if !bytes.HasPrefix(twice, original) {
		t.Fatal("unrelated bytes changed")
	}
}

func TestMalformedGoldenProfilesFailClosed(t *testing.T) {
	for _, name := range []string{"duplicate-block.profile", "partial-block.profile", "reversed-block.profile", "nested-block.profile"} {
		b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "profiles", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := Reconcile(b, Block("wrapper")); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
}
func TestMalformedMarkersFail(t *testing.T) {
	if _, _, err := Reconcile([]byte(Begin+"\n"), Block("x")); err == nil {
		t.Fatal("malformed marker accepted")
	}
}
