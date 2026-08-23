package shell

import "testing"

func FuzzQuote(f *testing.F) {
	f.Add("a'b;$HOME\n")
	f.Fuzz(func(t *testing.T, s string) { _, _ = Quote(s) })
}
