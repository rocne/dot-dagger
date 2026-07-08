package dagger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setFileLink runs SetFileLink against a .dagger seeded with content
// (content == "" means no pre-existing file) and returns the resulting bytes.
func setFileLink(t *testing.T, content, name, dest string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".dagger")
	if content != "" {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := SetFileLink(path, name, dest); err != nil {
		t.Fatalf("SetFileLink: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// mustLoadEntry asserts the serialized .dagger content contains a files entry
// for name with exactly one link action to dest.
func mustLoadEntry(t *testing.T, content, name, dest string) {
	t.Helper()
	cfg, err := Load(strings.NewReader(content))
	if err != nil {
		t.Fatalf("Load after SetFileLink: %v\ncontent:\n%s", err, content)
	}
	entry, ok := cfg.Files[name]
	if !ok {
		t.Fatalf("files entry %q missing after SetFileLink\ncontent:\n%s", name, content)
	}
	want := "link(" + dest + ")"
	if len(entry.Actions) != 1 || entry.Actions[0] != want {
		t.Fatalf("entry actions = %v, want [%s]", entry.Actions, want)
	}
}

// --- tier 1: no .dagger yet ---

func TestSetFileLink_CreatesFreshFile(t *testing.T) {
	got := setFileLink(t, "", "dot-gitconfig", "~/.gitconfig")
	mustLoadEntry(t, got, "dot-gitconfig", "~/.gitconfig")
}

// --- tier 2: .dagger exists, no files: key → textual append ---

func TestSetFileLink_AppendPreservesContentVerbatim(t *testing.T) {
	orig := "# my config dir\n" +
		"# these comments must survive\n" +
		"link_root: \"$config\"\n" +
		"defaults:\n" +
		"  actions:\n" +
		"    - link # inline comment\n"
	got := setFileLink(t, orig, "dot-bashrc", "~/.bashrc")

	// Textual append: the original bytes must be an untouched prefix.
	if !strings.HasPrefix(got, orig) {
		t.Errorf("original content not preserved verbatim:\n%s", got)
	}
	mustLoadEntry(t, got, "dot-bashrc", "~/.bashrc")

	// The rest of the config must still decode as before.
	cfg, err := Load(strings.NewReader(got))
	if err != nil {
		t.Fatal(err)
	}
	if string(cfg.LinkRoot) != "$config" || len(cfg.Defaults.Actions) != 1 {
		t.Errorf("existing config damaged: %+v", cfg)
	}
}

func TestSetFileLink_AppendHandlesMissingTrailingNewline(t *testing.T) {
	got := setFileLink(t, "link_root: \"~\"", "dot-x", "~/.x")
	mustLoadEntry(t, got, "dot-x", "~/.x")
}

func TestSetFileLink_CommentsOnlyFilePreserved(t *testing.T) {
	orig := "# nothing but comments here\n"
	got := setFileLink(t, orig, "dot-x", "~/.x")
	if !strings.HasPrefix(got, orig) {
		t.Errorf("comments-only content not preserved:\n%s", got)
	}
	mustLoadEntry(t, got, "dot-x", "~/.x")
}

// --- tier 3: files: exists → yaml.Node surgery ---

func TestSetFileLink_SurgeryAddsEntryAndKeepsComments(t *testing.T) {
	orig := "# header comment\n" +
		"link_root: \"$config\" # keep me\n" +
		"files:\n" +
		"  # entry comment\n" +
		"  existing.json:\n" +
		"    actions:\n" +
		"      - link(~/.existing.json)\n"
	got := setFileLink(t, orig, "dot-bashrc", "~/.bashrc")

	mustLoadEntry(t, got, "dot-bashrc", "~/.bashrc")
	// Existing entry intact.
	mustLoadEntry(t, got, "existing.json", "~/.existing.json")
	// Comments preserved through the node round-trip.
	for _, want := range []string{"# header comment", "# keep me", "# entry comment"} {
		if !strings.Contains(got, want) {
			t.Errorf("comment %q lost in surgery:\n%s", want, got)
		}
	}
	cfg, err := Load(strings.NewReader(got))
	if err != nil {
		t.Fatal(err)
	}
	if string(cfg.LinkRoot) != "$config" {
		t.Errorf("link_root damaged: %q", cfg.LinkRoot)
	}
}

func TestSetFileLink_SurgeryReplacesNullFiles(t *testing.T) {
	// files: with no value is YAML null — must become a proper mapping.
	got := setFileLink(t, "files:\n", "dot-x", "~/.x")
	mustLoadEntry(t, got, "dot-x", "~/.x")
}

func TestSetFileLink_FlowStyleRootMapping(t *testing.T) {
	// A root of `{}` (flow style) cannot take a textual append; the surgery
	// path must still produce valid YAML.
	got := setFileLink(t, "{}\n", "dot-x", "~/.x")
	mustLoadEntry(t, got, "dot-x", "~/.x")
}

// --- refusals: caller falls back to printing the snippet ---

func TestSetFileLink_ExistingEntryRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".dagger")
	orig := "files:\n  dot-x:\n    actions:\n      - link(~/.other)\n"
	if err := os.WriteFile(path, []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}
	err := SetFileLink(path, "dot-x", "~/.x")
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("err = %v, want already-exists refusal", err)
	}
	// File untouched.
	data, _ := os.ReadFile(path)
	if string(data) != orig {
		t.Errorf("refusal must not modify the file:\n%s", data)
	}
}

func TestSetFileLink_InvalidYAMLRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".dagger")
	orig := "link_root: [unclosed\n"
	if err := os.WriteFile(path, []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SetFileLink(path, "dot-x", "~/.x"); err == nil {
		t.Fatal("expected error for invalid existing YAML")
	}
	data, _ := os.ReadFile(path)
	if string(data) != orig {
		t.Errorf("refusal must not modify the file:\n%s", data)
	}
}

func TestSetFileLink_NonMappingFilesRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".dagger")
	if err := os.WriteFile(path, []byte("files:\n  - not-a-mapping\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SetFileLink(path, "dot-x", "~/.x"); err == nil {
		t.Fatal("expected error when files: is a sequence")
	}
}

// --- snippet ---

func TestFileLinkSnippet_ParsesToExpectedEntry(t *testing.T) {
	snippet := FileLinkSnippet("dot-gitconfig", "~/.gitconfig")
	mustLoadEntry(t, snippet, "dot-gitconfig", "~/.gitconfig")
}
