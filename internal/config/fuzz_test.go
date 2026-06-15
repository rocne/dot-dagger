package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FuzzLoadFrom asserts strict config.yaml decoding never panics on arbitrary
// content (KnownFields(true) must reject unknowns with an error, not a crash).
func FuzzLoadFrom(f *testing.F) {
	seeds := []string{
		"",
		"dotfiles: /home/me/dotfiles\n",
		"dotfiles: ~\n",
		"unknown: 1\n",
		"dotfiles:\n  nested: x\n", // wrong shape
		"- a\n",
		":\n",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, input string) {
		_, _ = loadFrom(strings.NewReader(input)) // must not panic
	})
}

// FuzzLoadLenient asserts the lenient loader — which decodes into map[string]any
// to diff unknown keys before decoding into the struct — never panics. This is
// the path every command runs during path resolution, so hostile config content
// must degrade, not crash.
func FuzzLoadLenient(f *testing.F) {
	seeds := []string{
		"dotfiles: /x\nlegacy_field: y\n",
		"dotfiles: /x\n",
		"a: 1\nb: 2\nc: 3\n",
		"",
		"deeply:\n  nested:\n    map: value\n",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	dir := f.TempDir()
	f.Fuzz(func(t *testing.T, input string) {
		path := filepath.Join(dir, "config.yaml")
		if err := os.WriteFile(path, []byte(input), 0o644); err != nil {
			t.Skip()
		}
		_, _, _ = LoadLenient(path) // must not panic
	})
}
