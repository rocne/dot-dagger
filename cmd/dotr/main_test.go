package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := newRootCmd()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

func emptyEnvFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "env.yaml")
	if err := os.WriteFile(path, []byte("env: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCheckEmptyRepo(t *testing.T) {
	out, err := run(t, "check",
		"--env-file", emptyEnvFile(t),
		"--dotfiles", t.TempDir(),
	)
	if err != nil {
		t.Fatalf("check error = %v", err)
	}
	if !strings.Contains(out, "fileset:") {
		t.Errorf("expected 'fileset:' in output: %q", out)
	}
	if !strings.Contains(out, "scripts:") {
		t.Errorf("expected 'scripts:' in output: %q", out)
	}
	if !strings.Contains(out, "packages:") {
		t.Errorf("expected 'packages:' in output: %q", out)
	}
	if !strings.Contains(out, "symlinks:") {
		t.Errorf("expected 'symlinks:' in output: %q", out)
	}
}

func TestApplyDryRunEmptyRepo(t *testing.T) {
	dir := t.TempDir()
	initFile := filepath.Join(dir, "init.sh")

	out, err := run(t, "apply", "--dry-run",
		"--env-file", emptyEnvFile(t),
		"--dotfiles", t.TempDir(),
		"--init-file", initFile,
	)
	if err != nil {
		t.Fatalf("apply --dry-run error = %v", err)
	}
	if !strings.Contains(out, "would write") {
		t.Errorf("expected 'would write' in dry-run output: %q", out)
	}
	if _, err := os.Stat(initFile); !os.IsNotExist(err) {
		t.Error("dry-run must not create init.sh")
	}
}

func TestApplyDryRunWithScriptAndConf(t *testing.T) {
	dotfiles := t.TempDir()
	linkRoot := t.TempDir()

	if err := os.MkdirAll(filepath.Join(dotfiles, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dotfiles, "scripts", "base.sh"), []byte("export X=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dotfiles, "conf"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dotfiles, "conf", "dot-zshrc"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	initFile := filepath.Join(dir, "init.sh")

	out, err := run(t, "apply", "--dry-run",
		"--env-file", emptyEnvFile(t),
		"--dotfiles", dotfiles,
		"--link-root", linkRoot,
		"--init-file", initFile,
	)
	if err != nil {
		t.Fatalf("apply --dry-run error = %v", err)
	}
	if !strings.Contains(out, "base.sh") {
		t.Errorf("expected base.sh in dry-run output: %q", out)
	}
	if !strings.Contains(out, ".zshrc") {
		t.Errorf("expected .zshrc symlink in dry-run output: %q", out)
	}
}

func TestCheckWithMissingHardRequirement(t *testing.T) {
	dotfiles := t.TempDir()

	pkgYaml := `package_managers:
  brew:
    install: brew install {package}
packages:
  some-fake-tool-xyz:
    brew: {}
`
	if err := os.WriteFile(filepath.Join(dotfiles, "packages.yaml"), []byte(pkgYaml), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(dotfiles, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	// @require a package that won't be installed or installable (brew not on PATH in CI).
	if err := os.WriteFile(filepath.Join(dotfiles, "scripts", "tool.sh"), []byte("# @require some-fake-tool-xyz\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "check",
		"--env-file", emptyEnvFile(t),
		"--dotfiles", dotfiles,
	)
	// check should succeed (it reports state, doesn't error on missing requirements).
	if err != nil {
		t.Fatalf("check error = %v", err)
	}
	_ = out
}
