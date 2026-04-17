package daggeryaml

import (
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	input := `
dotd:
  when: "os=macos"
  defaults:
    when: "os=linux"
link:
  link_root: ~/.config/nvim
`
	d, err := Load(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if d.Dotd.When != "os=macos" {
		t.Errorf("dotd.when = %q, want os=macos", d.Dotd.When)
	}
	if d.Dotd.Defaults.When != "os=linux" {
		t.Errorf("dotd.defaults.when = %q, want os=linux", d.Dotd.Defaults.When)
	}
	if d.Link.LinkRoot != "~/.config/nvim" {
		t.Errorf("link.link_root = %q, want ~/.config/nvim", d.Link.LinkRoot)
	}
}

func TestLoadEmpty(t *testing.T) {
	d, err := Load(strings.NewReader(""))
	if err != nil {
		t.Fatalf("Load() error = %v on empty input", err)
	}
	if d == nil {
		t.Fatal("Load() returned nil for empty input")
	}
}

func TestLoadFileMissing(t *testing.T) {
	d, err := LoadFile("/nonexistent/.dot-dagger.yaml")
	if err != nil {
		t.Fatalf("LoadFile() error = %v for missing file", err)
	}
	if d == nil {
		t.Fatal("LoadFile() returned nil for missing file")
	}
}
