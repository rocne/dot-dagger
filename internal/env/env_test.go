package env

import (
	"errors"
	"strings"
	"testing"
)

func TestResolveBuiltins(t *testing.T) {
	r := NewResolver()
	env, err := r.Resolve(nil)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	for _, key := range []string{"os", "shell"} {
		if env[key] == "" {
			t.Errorf("Resolve() key %q is empty", key)
		}
	}
}

func TestResolveOverride(t *testing.T) {
	r := NewResolver()
	// Replace detectors with stubs for determinism.
	r.Detectors["os"] = func() (string, error) { return "linux", nil }
	r.Detectors["shell"] = func() (string, error) { return "bash", nil }
	delete(r.Detectors, "distro")

	env, err := r.Resolve(map[string]string{"os": "macos"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if env["os"] != "macos" {
		t.Errorf("override: os = %q, want %q", env["os"], "macos")
	}
	if env["shell"] != "bash" {
		t.Errorf("detected: shell = %q, want %q", env["shell"], "bash")
	}
}

func TestResolveDetectorError(t *testing.T) {
	r := NewResolver()
	r.Detectors = map[string]Detector{
		"os":  func() (string, error) { return "linux", nil },
		"bad": func() (string, error) { return "", errors.New("fail") },
	}

	env, err := r.Resolve(nil)
	if err == nil {
		t.Fatal("Resolve() error = nil, want error for failing detector")
	}
	// Partial env still populated.
	if env["os"] != "linux" {
		t.Errorf("partial env: os = %q, want linux", env["os"])
	}
	if _, ok := env["bad"]; ok {
		t.Error("partial env: bad key should be absent")
	}
}

func TestRequireKeys(t *testing.T) {
	env := map[string]string{"os": "linux", "shell": "zsh"}

	if err := RequireKeys(env, "os", "shell"); err != nil {
		t.Errorf("RequireKeys() unexpected error: %v", err)
	}

	err := RequireKeys(env, "os", "context")
	if err == nil {
		t.Fatal("RequireKeys() error = nil, want error for missing key")
	}
	var mke *MissingKeysError
	if !errors.As(err, &mke) {
		t.Fatalf("RequireKeys() error type = %T, want *MissingKeysError", err)
	}
	if len(mke.Keys) != 1 || mke.Keys[0] != "context" {
		t.Errorf("MissingKeysError.Keys = %v, want [context]", mke.Keys)
	}
}

func TestLoadEnvFile(t *testing.T) {
	input := `
env:
  context: work
  role: desktop
dotfiles_repo: ~/dotfiles
`
	ef, err := LoadEnvFile(strings.NewReader(input))
	if err != nil {
		t.Fatalf("LoadEnvFile() error = %v", err)
	}
	if ef.Env["context"] != "work" {
		t.Errorf("context = %q, want work", ef.Env["context"])
	}
	if ef.DotfilesRepo != "~/dotfiles" {
		t.Errorf("dotfiles_repo = %q, want ~/dotfiles", ef.DotfilesRepo)
	}
}

func TestLoadEnvFileEmpty(t *testing.T) {
	ef, err := LoadEnvFile(strings.NewReader(""))
	if err != nil {
		t.Fatalf("LoadEnvFile() error = %v", err)
	}
	if ef.Env == nil {
		t.Error("Env map is nil for empty file")
	}
}

func TestLoadEnvFileFromPathMissing(t *testing.T) {
	ef, err := LoadEnvFileFromPath("/nonexistent/env.yaml")
	if err != nil {
		t.Fatalf("LoadEnvFileFromPath() error = %v for missing file", err)
	}
	if ef.Env == nil {
		t.Error("Env map is nil for missing file")
	}
}
