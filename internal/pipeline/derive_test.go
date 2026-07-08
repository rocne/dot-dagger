package pipeline

import (
	"os"
	"path/filepath"
	"testing"
)

// writeFile creates path (and parents) with content.
func writeDeriveFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func deriveOpts(home string) ActOptions {
	return ActOptions{
		HomeDir:   home,
		ConfigDir: filepath.Join(home, ".config"),
		BinDir:    filepath.Join(home, ".local", "bin"),
	}
}

// The scaffold default: config/.dagger with link_root "$config" derives
// $config/.<name> for dot- prefixed files — the exact mechanics of issue #191.
func TestDerivedLinkDest_ScaffoldCascade(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	writeDeriveFile(t, filepath.Join(root, "config", ".dagger"),
		"link_root: \"$config\"\ndefaults:\n  actions:\n    - link\n")
	file := filepath.Join(root, "config", "dot-gitconfig")
	writeDeriveFile(t, file, "[user]\n")

	got, err := DerivedLinkDest(root, file, file, deriveOpts(home))
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".config", ".gitconfig")
	if got != want {
		t.Errorf("derived = %q, want %q", got, want)
	}
}

// An in-file @link annotation overrides the cascade default.
func TestDerivedLinkDest_AnnotationOverridesCascade(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	writeDeriveFile(t, filepath.Join(root, "config", ".dagger"),
		"link_root: \"$config\"\ndefaults:\n  actions:\n    - link\n")
	file := filepath.Join(root, "config", "dot-gitconfig")
	writeDeriveFile(t, file, "# @link(~/.gitconfig)\n[user]\n")

	got, err := DerivedLinkDest(root, file, file, deriveOpts(home))
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".gitconfig"); got != want {
		t.Errorf("derived = %q, want %q", got, want)
	}
}

// A files: dict entry in the containing .dagger wins and its actions are used
// verbatim — the shape adopt persists.
func TestDerivedLinkDest_FilesEntry(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	writeDeriveFile(t, filepath.Join(root, "config", ".dagger"),
		"link_root: \"$config\"\ndefaults:\n  actions:\n    - link\nfiles:\n  dot-gitconfig:\n    actions:\n      - link(~/.gitconfig)\n")
	file := filepath.Join(root, "config", "dot-gitconfig")
	writeDeriveFile(t, file, "[user]\n")

	got, err := DerivedLinkDest(root, file, file, deriveOpts(home))
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".gitconfig"); got != want {
		t.Errorf("derived = %q, want %q", got, want)
	}
}

// No link action anywhere → "".
func TestDerivedLinkDest_NoLink(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	writeDeriveFile(t, filepath.Join(root, "shellrc", ".dagger"),
		"defaults:\n  actions:\n    - source\n")
	file := filepath.Join(root, "shellrc", "aliases.sh")
	writeDeriveFile(t, file, "alias ll='ls -l'\n")

	got, err := DerivedLinkDest(root, file, file, deriveOpts(home))
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Errorf("derived = %q, want empty (no link action)", got)
	}
}

// contentPath lets a caller scan a file that has not been moved yet
// (adopt --dry-run): annotations come from contentPath, cascade from fileAbs.
func TestDerivedLinkDest_SeparateContentPath(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	writeDeriveFile(t, filepath.Join(root, "config", ".dagger"),
		"link_root: \"$config\"\ndefaults:\n  actions:\n    - link\n")
	// The file does not exist inside the repo yet.
	fileAbs := filepath.Join(root, "config", "dot-zshrc")
	content := filepath.Join(home, ".zshrc")
	writeDeriveFile(t, content, "# plain rc\n")

	got, err := DerivedLinkDest(root, fileAbs, content, deriveOpts(home))
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".config", ".zshrc"); got != want {
		t.Errorf("derived = %q, want %q", got, want)
	}
}
