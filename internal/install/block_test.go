package install

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestReconcileIdempotentAndPreservesBytes(t *testing.T) {
	// 7 lines: enough that some of it (the first two) lands genuinely before the tail-preserving
	// insertion point, and the rest (5 lines) after — see TestReconcileInsertsBeforeTrailingLines
	// for the placement itself.
	original := []byte("one\ntwo\nthree\nfour\nfive\nsix\nseven\n")
	block := Block("fn() { :; }")
	once, changed, err := Reconcile(original, block)
	if err != nil || !changed {
		t.Fatal(err)
	}
	twice, changed, err := Reconcile(once, block)
	if err != nil || changed || !bytes.Equal(once, twice) {
		t.Fatal("not idempotent")
	}
	if !bytes.HasPrefix(twice, []byte("one\ntwo\n")) || !bytes.HasSuffix(twice, []byte("three\nfour\nfive\nsix\nseven\n")) {
		t.Fatal("unrelated bytes changed")
	}
}

// TestReconcileInsertsBeforeTrailingLines proves a fresh install doesn't just append at the very
// end of the file: it leaves the file's own last tailLines lines running after the managed block,
// so something the user deliberately put last (a final plugin/tool loader, say) keeps running
// after this block, not before it.
func TestReconcileInsertsBeforeTrailingLines(t *testing.T) {
	original := []byte("one\ntwo\nthree\nfour\nfive\nsix\nseven\n")
	out, changed, err := Reconcile(original, Block("fn() { :; }"))
	if err != nil || !changed {
		t.Fatal(err)
	}
	wantTail := "three\nfour\nfive\nsix\nseven\n"
	if !bytes.HasSuffix(out, []byte(wantTail)) {
		t.Fatalf("last %d lines were not preserved as the tail: %s", tailLines, out)
	}
	if !bytes.HasPrefix(out, []byte("one\ntwo\n"+Begin)) {
		t.Fatalf("block was not inserted right after the first two lines: %s", out)
	}
}

// TestReconcileShortProfileInsertsAtStart proves a profile with fewer than tailLines lines gets
// the block at the very beginning — there's nothing meaningfully "before the tail" to insert
// after — rather than, say, panicking on the short input or silently falling back to the old
// "always append at the end" behavior.
func TestReconcileShortProfileInsertsAtStart(t *testing.T) {
	original := []byte("only one line\n")
	out, changed, err := Reconcile(original, Block("fn() { :; }"))
	if err != nil || !changed {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(out, []byte(Begin)) {
		t.Fatalf("block was not inserted at the start of a short profile: %s", out)
	}
	if !bytes.HasSuffix(out, original) {
		t.Fatalf("original short profile content was not preserved as the tail: %s", out)
	}
}

// TestReconcileEmptyProfile proves inserting into a brand-new/empty profile still just produces
// the block, with no panic and no spurious leading blank line.
func TestReconcileEmptyProfile(t *testing.T) {
	out, changed, err := Reconcile(nil, Block("fn() { :; }"))
	if err != nil || !changed {
		t.Fatal(err)
	}
	if !bytes.Equal(out, Block("fn() { :; }")) {
		t.Fatalf("unexpected result for an empty profile: %s", out)
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
