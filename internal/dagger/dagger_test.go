package dagger

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rocne/dot-dagger/internal/annotation"
)

func TestLoad_Empty(t *testing.T) {
	node, err := Load(strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	if node.When != "" || node.LinkRoot != "" || len(node.Actions) != 0 {
		t.Errorf("empty input should produce zero-value node, got %+v", node)
	}
}

func TestLoad_MultipleDocumentsRejected(t *testing.T) {
	input := "when: os=linux\n---\nwhen: os=macos\n"
	_, err := Load(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for multi-document .dagger file, got nil")
	}
	if !strings.Contains(err.Error(), "multiple YAML documents") {
		t.Errorf("expected multi-document error, got: %v", err)
	}
}

func TestLoad_BasicFields(t *testing.T) {
	input := `when: os=macos
link_root: ~/relative
actions:
  - source
  - link(~/.tmux.conf)
name: my.name`
	node, err := Load(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if node.When != "os=macos" {
		t.Errorf("When = %q, want %q", node.When, "os=macos")
	}
	if node.LinkRoot != "~/relative" {
		t.Errorf("LinkRoot = %q, want %q", node.LinkRoot, "~/relative")
	}
	if len(node.Actions) != 2 || node.Actions[0] != "source" || node.Actions[1] != "link(~/.tmux.conf)" {
		t.Errorf("Actions = %v", node.Actions)
	}
	if node.Name != "my.name" {
		t.Errorf("Name = %q, want %q", node.Name, "my.name")
	}
}

func TestLoad_Defaults(t *testing.T) {
	input := `defaults:
  when: context=work
  link_root: ~/apps
  actions:
    - no-source`
	node, err := Load(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if node.Defaults.When != "context=work" {
		t.Errorf("Defaults.When = %q, want %q", node.Defaults.When, "context=work")
	}
	if node.Defaults.LinkRoot != "~/apps" {
		t.Errorf("Defaults.LinkRoot = %q", node.Defaults.LinkRoot)
	}
	if len(node.Defaults.Actions) != 1 || node.Defaults.Actions[0] != "no-source" {
		t.Errorf("Defaults.Actions = %v", node.Defaults.Actions)
	}
}

func TestLoad_Composition(t *testing.T) {
	input := `composition:
  enabled: true`
	node, err := Load(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if !node.Composition.Enabled {
		t.Error("Composition.Enabled = false, want true")
	}
}

func TestLoad_Files(t *testing.T) {
	input := `files:
  settings.json:
    when: os=macos
    name: nvim.settings
    actions:
      - link(settings.json)
  dot-gitconfig-work:
    when: context=work
    actions:
      - link(~/.gitconfig)`
	node, err := Load(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(node.Files) != 2 {
		t.Fatalf("Files len = %d, want 2", len(node.Files))
	}
	s, ok := node.Files["settings.json"]
	if !ok {
		t.Fatal("Files[settings.json] not found")
	}
	if s.When != "os=macos" || s.Name != "nvim.settings" {
		t.Errorf("settings.json = %+v", s)
	}
	if len(s.Actions) != 1 || s.Actions[0] != "link(settings.json)" {
		t.Errorf("settings.json.Actions = %v", s.Actions)
	}
	g, ok := node.Files["dot-gitconfig-work"]
	if !ok {
		t.Fatal("Files[dot-gitconfig-work] not found")
	}
	if g.When != "context=work" {
		t.Errorf("dot-gitconfig-work.When = %q", g.When)
	}
}

func TestLoadFile_NotExist(t *testing.T) {
	node, err := LoadFile(filepath.Join(t.TempDir(), ".dagger"))
	if err != nil {
		t.Fatalf("LoadFile non-existent: %v", err)
	}
	if node.When != "" || node.Composition.Enabled {
		t.Errorf("non-existent file should return zero value, got %+v", node)
	}
}

func TestLoadFile_RealFile(t *testing.T) {
	dir := t.TempDir()
	content := "when: os=macos\ncomposition:\n  enabled: true\n"
	if err := os.WriteFile(filepath.Join(dir, ".dagger"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	node, err := LoadFile(filepath.Join(dir, ".dagger"))
	if err != nil {
		t.Fatal(err)
	}
	if node.When != "os=macos" || !node.Composition.Enabled {
		t.Errorf("got %+v", node)
	}
}

func TestLoad_Conventions(t *testing.T) {
	input := `conventions:
  shellrc: scripts
  bin: executables
  config: dotfiles`
	node, err := Load(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if node.Conventions.Shellrc != "scripts" {
		t.Errorf("Conventions.Shellrc = %q, want %q", node.Conventions.Shellrc, "scripts")
	}
	if node.Conventions.Bin != "executables" {
		t.Errorf("Conventions.Bin = %q, want %q", node.Conventions.Bin, "executables")
	}
	if node.Conventions.Config != "dotfiles" {
		t.Errorf("Conventions.Config = %q, want %q", node.Conventions.Config, "dotfiles")
	}
}

// TestBasicNodeYAMLTagsMatchKeys guards that BasicNode and NamedNode yaml struct
// tags stay in sync with the annotation.Key* vocabulary constants. A one-sided
// rename (tag but not const, or const but not tag) fails this test.
func TestBasicNodeYAMLTagsMatchKeys(t *testing.T) {
	basicTests := []struct {
		field   string
		wantTag string
	}{
		{"When", annotation.KeyWhen},
		{"After", annotation.KeyAfter},
		{"Require", annotation.KeyRequire},
		{"Request", annotation.KeyRequest},
		{"Disable", annotation.KeyDisable},
	}
	rt := reflect.TypeOf(BasicNode{})
	for _, tc := range basicTests {
		f, ok := rt.FieldByName(tc.field)
		if !ok {
			t.Errorf("BasicNode: missing field %s", tc.field)
			continue
		}
		if got := f.Tag.Get("yaml"); got != tc.wantTag {
			t.Errorf("BasicNode.%s yaml tag: got %q, want %q", tc.field, got, tc.wantTag)
		}
	}

	namedTests := []struct {
		field   string
		wantTag string
	}{
		{"Name", annotation.KeyName},
	}
	rtn := reflect.TypeOf(NamedNode{})
	for _, tc := range namedTests {
		f, ok := rtn.FieldByName(tc.field)
		if !ok {
			t.Errorf("NamedNode: missing field %s", tc.field)
			continue
		}
		if got := f.Tag.Get("yaml"); got != tc.wantTag {
			t.Errorf("NamedNode.%s yaml tag: got %q, want %q", tc.field, got, tc.wantTag)
		}
	}
}
