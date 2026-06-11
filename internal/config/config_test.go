package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_NotExist(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Dotfiles != "" || cfg.BinDir != "" {
		t.Errorf("non-existent file should return zero value, got %+v", cfg)
	}
}

func TestLoad_AllFields(t *testing.T) {
	dir := t.TempDir()
	content := "dotfiles: ~/dotfiles\nbin_dir: ~/bin\ngenerated_dir: ~/.config/dot-dagger/generated\nlink_root: \"~\"\n"
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Dotfiles != "~/dotfiles" {
		t.Errorf("Dotfiles = %q", cfg.Dotfiles)
	}
	if cfg.BinDir != "~/bin" {
		t.Errorf("BinDir = %q", cfg.BinDir)
	}
	if cfg.GeneratedDir != "~/.config/dot-dagger/generated" {
		t.Errorf("GeneratedDir = %q", cfg.GeneratedDir)
	}
	if cfg.LinkRoot != "~" {
		t.Errorf("LinkRoot = %q", cfg.LinkRoot)
	}
}

func TestGet_KnownKeys(t *testing.T) {
	cfg := &Config{
		Dotfiles:     "~/dotfiles",
		BinDir:       "~/bin",
		GeneratedDir: "~/.config/dot-dagger/generated",
		LinkRoot:     "~",
	}
	cases := []struct{ key, want string }{
		{"dotfiles", "~/dotfiles"},
		{"bin_dir", "~/bin"},
		{"generated_dir", "~/.config/dot-dagger/generated"},
		{"link_root", "~"},
	}
	for _, c := range cases {
		got, err := cfg.Get(c.key)
		if err != nil {
			t.Errorf("Get(%q) error: %v", c.key, err)
			continue
		}
		if got != c.want {
			t.Errorf("Get(%q) = %q, want %q", c.key, got, c.want)
		}
	}
}

func TestGet_UnknownKey(t *testing.T) {
	cfg := &Config{}
	_, err := cfg.Get("unknown_key")
	if err == nil {
		t.Error("Get(unknown) should return error")
	}
}

func TestSet_KnownKeys(t *testing.T) {
	cfg := &Config{}
	if err := cfg.Set("dotfiles", "~/mydots"); err != nil {
		t.Fatal(err)
	}
	if cfg.Dotfiles != "~/mydots" {
		t.Errorf("Set dotfiles: got %q", cfg.Dotfiles)
	}
	if err := cfg.Set("bin_dir", "~/bin"); err != nil {
		t.Fatal(err)
	}
	if cfg.BinDir != "~/bin" {
		t.Errorf("Set bin_dir: got %q", cfg.BinDir)
	}
}

func TestSet_UnknownKey(t *testing.T) {
	cfg := &Config{}
	if err := cfg.Set("nope", "val"); err == nil {
		t.Error("Set(unknown) should return error")
	}
}

func TestSave_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	original := &Config{
		Dotfiles:     "~/dotfiles",
		BinDir:       "~/bin",
		GeneratedDir: "~/.config/dot-dagger/generated",
		LinkRoot:     "~",
	}
	if err := Save(path, original); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Dotfiles != original.Dotfiles || got.BinDir != original.BinDir {
		t.Errorf("round-trip failed: got %+v", got)
	}
}

// TestSave_UnwritableDirErrors verifies Save returns an error wrapped with
// "config:" when the destination dir cannot be written.
func TestSave_UnwritableDirErrors(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	path := filepath.Join(dir, "subdir", "config.yaml")
	err := Save(path, &Config{Dotfiles: "~/d"})
	if err == nil {
		t.Fatal("expected Save to fail on unwritable dir")
	}
	if !strings.Contains(err.Error(), "config:") {
		t.Errorf("error not prefixed with config:, got %q", err.Error())
	}
}

// TestLoadFrom_RejectsUnknownField verifies KnownFields(true) makes loadFrom
// error out when the YAML has a field not in the Config struct.
func TestLoadFrom_RejectsUnknownField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("dotfiles: ~/d\nunknown_field: yes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected Load to reject unknown field via KnownFields(true)")
	}
}
