package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestLoad_NotExist(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Dotfiles != "" {
		t.Errorf("non-existent file should return zero value, got %+v", cfg)
	}
}

func TestLoad_DotfilesOnly(t *testing.T) {
	dir := t.TempDir()
	content := "dotfiles: ~/dotfiles\n"
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
}

func TestGet_KnownKeys(t *testing.T) {
	cfg := &Config{
		Dotfiles: "~/dotfiles",
	}
	cases := []struct{ key, want string }{
		{"dotfiles", "~/dotfiles"},
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
		Dotfiles: "~/dotfiles",
	}
	if err := Save(path, original); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Dotfiles != original.Dotfiles {
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

// TestConfig_OnlyDotfiles verifies that Keys contains exactly [dotfiles] and
// that removed fields (bin_dir, link_root, generated_dir) are rejected by
// strict YAML decoding.
func TestConfig_OnlyDotfiles(t *testing.T) {
	if len(Keys) != 1 || Keys[0] != KeyDotfiles {
		t.Fatalf("Keys = %v, want [dotfiles]", Keys)
	}
	for _, field := range []string{"bin_dir: /x\n", "link_root: ~/.config\n", "generated_dir: /g\n"} {
		if _, err := loadFrom(strings.NewReader(field)); err == nil {
			t.Errorf("expected strict-decode error for removed field: %q", field)
		}
	}
}

// TestLoadLenient_IgnoresUnknownFields verifies that LoadLenient tolerates a
// legacy config (removed fields), still reads `dotfiles`, and reports which
// fields it ignored — so the path preamble never hard-fails on stale config.
func TestLoadLenient_IgnoresUnknownFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	legacy := "dotfiles: ~/d\nbin_dir: /x\ngenerated_dir: /g\nlink_root: ~/.config\n"
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, unknown, err := LoadLenient(path)
	if err != nil {
		t.Fatalf("LoadLenient error = %v", err)
	}
	if cfg.Dotfiles != "~/d" {
		t.Errorf("Dotfiles = %q, want ~/d", cfg.Dotfiles)
	}
	want := []string{"bin_dir", "generated_dir", "link_root"}
	if !slices.Equal(unknown, want) {
		t.Errorf("unknown = %v, want %v", unknown, want)
	}
}

// TestLoadLenient_MissingFileIsZero verifies a missing file is not an error.
func TestLoadLenient_MissingFileIsZero(t *testing.T) {
	cfg, unknown, err := LoadLenient(filepath.Join(t.TempDir(), "absent.yaml"))
	if err != nil {
		t.Fatalf("LoadLenient(missing) error = %v", err)
	}
	if cfg.Dotfiles != "" || len(unknown) != 0 {
		t.Errorf("expected zero config and no unknowns, got %+v / %v", cfg, unknown)
	}
}

// TestLoadLenient_MalformedYAMLErrors verifies genuine syntax errors still surface.
func TestLoadLenient_MalformedYAMLErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("dotfiles: [unterminated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadLenient(path); err == nil {
		t.Fatal("expected error on malformed YAML")
	}
}
