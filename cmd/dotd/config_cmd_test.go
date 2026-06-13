package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	dotcfg "github.com/rocne/dot-dagger/internal/config"
)

// writeConfigYAML writes a config.yaml with the given content and returns its path.
func writeConfigYAML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// --- dotd config show ---

func TestConfigShow_Empty(t *testing.T) {
	configPath := writeConfigYAML(t, "{}\n")

	out, err := run(t, "config", "show",
		"--config", configPath,
		"--env-file", emptyEnvFile(t),
		"--files", emptyDotfiles(t),
	)
	if err != nil {
		t.Fatalf("config show error = %v", err)
	}
	// All known keys should appear in output even if empty.
	for _, key := range []string{"dotfiles", "bin_dir", "generated_dir", "link_root"} {
		if !strings.Contains(out, key+"=") {
			t.Errorf("expected key %q in output: %q", key, out)
		}
	}
}

func TestConfigShow_PopulatedKeys(t *testing.T) {
	configPath := writeConfigYAML(t, "dotfiles: /home/user/dotfiles\nlink_root: /home/user\n")

	out, err := run(t, "config", "show",
		"--config", configPath,
		"--env-file", emptyEnvFile(t),
		"--files", emptyDotfiles(t),
	)
	if err != nil {
		t.Fatalf("config show error = %v", err)
	}
	if !strings.Contains(out, "dotfiles=/home/user/dotfiles") {
		t.Errorf("expected dotfiles value in output: %q", out)
	}
	if !strings.Contains(out, "link_root=/home/user") {
		t.Errorf("expected link_root value in output: %q", out)
	}
}

// --- dotd config get ---

func TestConfigGet_ExistingKey(t *testing.T) {
	configPath := writeConfigYAML(t, "dotfiles: /tmp/myfiles\n")

	out, err := run(t, "config", "get", "dotfiles",
		"--config", configPath,
		"--env-file", emptyEnvFile(t),
		"--files", emptyDotfiles(t),
	)
	if err != nil {
		t.Fatalf("config get error = %v", err)
	}
	if strings.TrimSpace(out) != "/tmp/myfiles" {
		t.Errorf("config get dotfiles = %q, want /tmp/myfiles", strings.TrimSpace(out))
	}
}

func TestConfigGet_MissingKey(t *testing.T) {
	configPath := writeConfigYAML(t, "{}\n")

	_, err := run(t, "config", "get", "no_such_key",
		"--config", configPath,
		"--env-file", emptyEnvFile(t),
		"--files", emptyDotfiles(t),
	)
	if err == nil {
		t.Error("expected error for unknown key, got nil")
	}
}

func TestConfigGet_EmptyValue(t *testing.T) {
	configPath := writeConfigYAML(t, "{}\n")

	out, err := run(t, "config", "get", "dotfiles",
		"--config", configPath,
		"--env-file", emptyEnvFile(t),
		"--files", emptyDotfiles(t),
	)
	if err != nil {
		t.Fatalf("config get dotfiles (empty) error = %v", err)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("expected empty value, got %q", strings.TrimSpace(out))
	}
}

// --- dotd config set ---

func TestConfigSet_WritesValue(t *testing.T) {
	configPath := writeConfigYAML(t, "{}\n")

	_, err := run(t, "config", "set", "dotfiles", "/new/path",
		"--config", configPath,
		"--env-file", emptyEnvFile(t),
		"--files", emptyDotfiles(t),
	)
	if err != nil {
		t.Fatalf("config set error = %v", err)
	}

	// Read it back.
	out, err := run(t, "config", "get", "dotfiles",
		"--config", configPath,
		"--env-file", emptyEnvFile(t),
		"--files", emptyDotfiles(t),
	)
	if err != nil {
		t.Fatalf("config get after set error = %v", err)
	}
	if strings.TrimSpace(out) != "/new/path" {
		t.Errorf("config get dotfiles = %q, want /new/path", strings.TrimSpace(out))
	}
}

func TestConfigSet_UpdatesExistingValue(t *testing.T) {
	configPath := writeConfigYAML(t, "dotfiles: /old/path\n")

	_, err := run(t, "config", "set", "dotfiles", "/updated/path",
		"--config", configPath,
		"--env-file", emptyEnvFile(t),
		"--files", emptyDotfiles(t),
	)
	if err != nil {
		t.Fatalf("config set error = %v", err)
	}

	out, err := run(t, "config", "get", "dotfiles",
		"--config", configPath,
		"--env-file", emptyEnvFile(t),
		"--files", emptyDotfiles(t),
	)
	if err != nil {
		t.Fatalf("config get error = %v", err)
	}
	if strings.TrimSpace(out) != "/updated/path" {
		t.Errorf("config get dotfiles = %q, want /updated/path", strings.TrimSpace(out))
	}
}

func TestConfigSet_InvalidKey(t *testing.T) {
	configPath := writeConfigYAML(t, "{}\n")

	_, err := run(t, "config", "set", "no_such_key", "val",
		"--config", configPath,
		"--env-file", emptyEnvFile(t),
		"--files", emptyDotfiles(t),
	)
	if err == nil {
		t.Error("expected error for unknown key")
	}
}

// --- dotd config edit ---

// TestConfigEdit_InvokesEditor verifies that config edit passes the config.yaml
// path to $EDITOR. A fake editor script writes its first argument to a marker
// file so we can assert the path without needing an interactive terminal.
func TestConfigEdit_InvokesEditor(t *testing.T) {
	configPath := writeConfigYAML(t, "dotfiles: /tmp/df\n")

	scriptDir := t.TempDir()
	markerDir := t.TempDir()
	markerFile := filepath.Join(markerDir, "marker")
	editorScript := filepath.Join(scriptDir, "fakeedit.sh")

	if err := os.WriteFile(editorScript,
		[]byte(fmt.Sprintf("#!/bin/sh\necho \"$1\" > %s\n", markerFile)),
		0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("EDITOR", editorScript)

	_, err := run(t, "config", "edit",
		"--config", configPath,
		"--env-file", emptyEnvFile(t),
		"--files", emptyDotfiles(t),
	)
	if err != nil {
		t.Fatalf("config edit error = %v", err)
	}

	got, readErr := os.ReadFile(markerFile)
	if readErr != nil {
		t.Fatalf("marker file not created — editor was not invoked: %v", readErr)
	}
	if !strings.Contains(strings.TrimSpace(string(got)), configPath) {
		t.Errorf("editor received path %q, want %q", strings.TrimSpace(string(got)), configPath)
	}
}

// --- dotd env edit ---

// TestEnvEdit_InvokesEditor verifies that env edit passes the env.yaml path
// to $EDITOR using the same fake-editor pattern as TestConfigEdit_InvokesEditor.
func TestEnvEdit_InvokesEditor(t *testing.T) {
	envFile := emptyEnvFile(t)

	scriptDir := t.TempDir()
	markerDir := t.TempDir()
	markerFile := filepath.Join(markerDir, "marker")
	editorScript := filepath.Join(scriptDir, "fakeedit.sh")

	if err := os.WriteFile(editorScript,
		[]byte(fmt.Sprintf("#!/bin/sh\necho \"$1\" > %s\n", markerFile)),
		0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("EDITOR", editorScript)

	_, err := run(t, "env", "edit",
		"--env-file", envFile,
		"--files", emptyDotfiles(t),
	)
	if err != nil {
		t.Fatalf("env edit error = %v", err)
	}

	got, readErr := os.ReadFile(markerFile)
	if readErr != nil {
		t.Fatalf("marker file not created — editor was not invoked: %v", readErr)
	}
	if !strings.Contains(strings.TrimSpace(string(got)), envFile) {
		t.Errorf("editor received path %q, want %q", strings.TrimSpace(string(got)), envFile)
	}
}

// --- loadConfig ---

func TestLoadConfig_ReturnsZeroOnMissing(t *testing.T) {
	cfg, err := dotcfg.Load(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	if err != nil {
		t.Fatalf("loadConfig nonexistent = %v, want nil error", err)
	}
	if cfg.Dotfiles != "" || cfg.BinDir != "" || cfg.LinkRoot != "" {
		t.Errorf("expected zero Config for missing file, got %+v", cfg)
	}
}

func TestLoadConfig_ReadsFields(t *testing.T) {
	configPath := writeConfigYAML(t, "dotfiles: /df\nbin_dir: /bin\n")

	cfg, err := dotcfg.Load(configPath)
	if err != nil {
		t.Fatalf("loadConfig error = %v", err)
	}
	if cfg.Dotfiles != "/df" {
		t.Errorf("Dotfiles = %q, want /df", cfg.Dotfiles)
	}
	if cfg.BinDir != "/bin" {
		t.Errorf("BinDir = %q, want /bin", cfg.BinDir)
	}
}
