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

func TestCheckEmptyRepo(t *testing.T) {
	out, err := run(t, "check", "--dotfiles", t.TempDir())
	if err != nil {
		t.Fatalf("check error = %v", err)
	}
	if !strings.Contains(out, "0 ok") {
		t.Errorf("expected '0 ok' in output: %q", out)
	}
}

func TestCheckReportsSymlinks(t *testing.T) {
	dotfiles := t.TempDir()
	linkRoot := t.TempDir()

	confDir := filepath.Join(dotfiles, "conf")
	if err := os.MkdirAll(confDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, "dot-zshrc"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "check", "--dotfiles", dotfiles, "--link-root", linkRoot)
	if err != nil {
		t.Fatalf("check error = %v", err)
	}
	if !strings.Contains(out, "1 missing") {
		t.Errorf("expected '1 missing': %q", out)
	}
}

func TestCheckIncludesWhenGatedFiles(t *testing.T) {
	// Standalone: @when-gated files still included unconditionally.
	dotfiles := t.TempDir()
	linkRoot := t.TempDir()

	confDir := filepath.Join(dotfiles, "conf")
	if err := os.MkdirAll(confDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// This file would be filtered out in orchestrated mode (os=never-true).
	content := "# @when os=never-true\n"
	if err := os.WriteFile(filepath.Join(confDir, "dot-zshrc"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "check", "--dotfiles", dotfiles, "--link-root", linkRoot)
	if err != nil {
		t.Fatalf("check error = %v", err)
	}
	// Standalone: file included regardless of @when → shows as missing symlink.
	if !strings.Contains(out, "1 missing") {
		t.Errorf("expected file included unconditionally: %q", out)
	}
}

func TestApplyDryRunEmptyRepo(t *testing.T) {
	_, err := run(t, "apply", "--dry-run", "--dotfiles", t.TempDir())
	if err != nil {
		t.Fatalf("apply --dry-run error = %v", err)
	}
}

func TestApplyDryRunWithConfFile(t *testing.T) {
	dotfiles := t.TempDir()
	linkRoot := t.TempDir()

	confDir := filepath.Join(dotfiles, "conf")
	if err := os.MkdirAll(confDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, "dot-zshrc"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "apply", "--dry-run", "--dotfiles", dotfiles, "--link-root", linkRoot)
	if err != nil {
		t.Fatalf("apply --dry-run error = %v", err)
	}
	if !strings.Contains(out, ".zshrc") {
		t.Errorf("expected .zshrc in dry-run output: %q", out)
	}
}

func TestRemoveDryRun(t *testing.T) {
	dotfiles := t.TempDir()
	linkRoot := t.TempDir()

	confDir := filepath.Join(dotfiles, "conf")
	if err := os.MkdirAll(confDir, 0o755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(confDir, "dot-zshrc")
	if err := os.WriteFile(src, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(linkRoot, ".zshrc")
	if err := os.Symlink(src, dst); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "remove", "--dry-run", "--dotfiles", dotfiles, "--link-root", linkRoot)
	if err != nil {
		t.Fatalf("remove --dry-run error = %v", err)
	}
	if !strings.Contains(out, ".zshrc") {
		t.Errorf("expected .zshrc in dry-run remove output: %q", out)
	}
	if _, err := os.Lstat(dst); err != nil {
		t.Error("dry-run must not remove symlink")
	}
}
