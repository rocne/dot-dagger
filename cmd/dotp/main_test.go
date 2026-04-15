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

func TestListNoRequirements(t *testing.T) {
	out, err := run(t, "list", "--files", t.TempDir())
	if err != nil {
		t.Fatalf("list error = %v", err)
	}
	if !strings.Contains(out, "no package requirements found") {
		t.Errorf("expected 'no package requirements found': %q", out)
	}
}

func TestCheckNoRequirements(t *testing.T) {
	out, err := run(t, "check", "--files", t.TempDir())
	if err != nil {
		t.Fatalf("check error = %v", err)
	}
	if !strings.Contains(out, "no package requirements found") {
		t.Errorf("expected 'no package requirements found': %q", out)
	}
}

func TestListWithRequirements(t *testing.T) {
	dotfiles := t.TempDir()
	scriptsDir := filepath.Join(dotfiles, "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "# @require fzf\n# @request ripgrep\nexport FOO=1\n"
	if err := os.WriteFile(filepath.Join(scriptsDir, "tools.sh"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "list", "--files", dotfiles)
	if err != nil {
		t.Fatalf("list error = %v", err)
	}
	if !strings.Contains(out, "fzf") {
		t.Errorf("expected fzf in list output: %q", out)
	}
	if !strings.Contains(out, "ripgrep") {
		t.Errorf("expected ripgrep in list output: %q", out)
	}
}

func TestListIncludesWhenGatedFiles(t *testing.T) {
	// Standalone: @when-gated files still included unconditionally.
	dotfiles := t.TempDir()
	scriptsDir := filepath.Join(dotfiles, "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "# @when os=never-true\n# @require fzf\n"
	if err := os.WriteFile(filepath.Join(scriptsDir, "tools.sh"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "list", "--files", dotfiles)
	if err != nil {
		t.Fatalf("list error = %v", err)
	}
	// Standalone: fzf listed regardless of @when.
	if !strings.Contains(out, "fzf") {
		t.Errorf("expected fzf listed unconditionally: %q", out)
	}
}

func TestInstallDryRunNoRequirements(t *testing.T) {
	_, err := run(t, "install", "--dry-run", "--files", t.TempDir())
	if err != nil {
		t.Fatalf("install --dry-run error = %v", err)
	}
}

func TestCheckWithRequirements(t *testing.T) {
	dotfiles := t.TempDir()

	pkgYaml := `package_managers:
  brew:
    install: brew install {package}
packages:
  fzf:
    brew: {}
`
	if err := os.WriteFile(filepath.Join(dotfiles, "packages.yaml"), []byte(pkgYaml), 0o644); err != nil {
		t.Fatal(err)
	}

	scriptsDir := filepath.Join(dotfiles, "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scriptsDir, "tools.sh"), []byte("# @require fzf\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "check", "--files", dotfiles)
	if err != nil {
		t.Fatalf("check error = %v", err)
	}
	if !strings.Contains(out, "fzf") {
		t.Errorf("expected fzf in check output: %q", out)
	}
}
