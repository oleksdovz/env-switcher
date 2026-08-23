package config

import (
	"bytes"
	"testing"
)

func FuzzParseDoesNotPanic(f *testing.F) {
	f.Add([]byte("version: 1\nenvs:\n  dev:\n    project: /tmp\n"))
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = Parse(bytes.NewReader(b))
	})
}
