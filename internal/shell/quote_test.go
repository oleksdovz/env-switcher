package shell

import "testing"

func TestQuote(t *testing.T) {
	tests := map[string]string{"": "''", "a b": "'a b'", "$HOME; x": "'$HOME; x'", "a'b": "'a'\\''b'", "line\nnext": "'line\nnext'"}
	for in, want := range tests {
		got, err := Quote(in)
		if err != nil || got != want {
			t.Fatalf("Quote(%q)=%q,%v want %q", in, got, err, want)
		}
	}
	if _, err := Quote("x\x00y"); err == nil {
		t.Fatal("NUL accepted")
	}
}
