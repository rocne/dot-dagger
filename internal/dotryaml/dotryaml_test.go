package dotryaml

import (
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	input := `
dote:
  package_managers:
    priority: [brew, apt]
dotd:
  when: "os=macos"
  defaults:
    when: "os=linux"
dotl:
  link_root: ~/.config/nvim
`
	d, err := Load(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if d.Dote.PackageManagers.Priority[0] != "brew" {
		t.Errorf("priority[0] = %q, want brew", d.Dote.PackageManagers.Priority[0])
	}
	if d.Dotd.When != "os=macos" {
		t.Errorf("dotd.when = %q, want os=macos", d.Dotd.When)
	}
	if d.Dotd.Defaults.When != "os=linux" {
		t.Errorf("dotd.defaults.when = %q, want os=linux", d.Dotd.Defaults.When)
	}
	if d.Dotl.LinkRoot != "~/.config/nvim" {
		t.Errorf("dotl.link_root = %q, want ~/.config/nvim", d.Dotl.LinkRoot)
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
	d, err := LoadFile("/nonexistent/.dotr.yaml")
	if err != nil {
		t.Fatalf("LoadFile() error = %v for missing file", err)
	}
	if d == nil {
		t.Fatal("LoadFile() returned nil for missing file")
	}
}
